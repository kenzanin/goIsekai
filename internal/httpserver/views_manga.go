package httpserver

import (
	"errors"
	"goisekai/internal/database"
	"goisekai/internal/hostnet"
	"goisekai/internal/pluginmanager"
	"goisekai/pkg/types"
	"net/http"
	"strconv"
)

// viewMangaDetail renders a manga's info plus its chapter list.
func (s *Server) viewMangaDetail(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	// Opening the detail page clears the library card's [New] badge.
	if err := s.service.ClearMangaNew(pluginID, mangaID); err != nil {
		s.logger.Warn("clear new badge", "plugin", pluginID, "manga", mangaID, "error", err)
	}
	manga, chapters, err := s.service.GetMangaDetails(pluginID, mangaID)
	challenge := false
	if err != nil {
		if _, ok := errors.AsType[*hostnet.ChallengeError](err); ok {
			challenge = true
			s.logger.Warn("manga detail blocked by challenge", "plugin", pluginID, "manga", mangaID)
		} else {
			s.logger.Error("manga detail", "error", err, "plugin", pluginID, "manga", mangaID)
			http.Error(w, "failed to load manga: "+err.Error(), http.StatusBadGateway)
			return
		}
	}
	progress, err := s.service.GetChapterProgresses(pluginID, mangaID)
	if err != nil {
		s.logger.Warn("chapter progress", "error", err, "manga", mangaID)
		progress = map[string]database.ChapterProgress{}
	}
	continueTo := computeContinue(chapters, progress)
	// Prefer most-recently-read chapter from history if it exists and isn't fully read
	if lastCont := s.continueFromHistory(pluginID, mangaID, chapters, progress); lastCont != nil {
		continueTo = lastCont
	}
	inLibrary := s.service.IsInLibrary(pluginID, mangaID)

	// Plugin identity for the header badge: display name + small logo.
	pluginName := pluginID
	pluginIcon := ""
	if m, ok := s.service.PluginMetas()[pluginID]; ok {
		if m.Name != "" {
			pluginName = m.Name
		}
		if m.Logo != "" {
			pluginIcon = resolveLogoURL(m.Logo, pluginID)
		}
	}
	altTitles, altErr := s.service.ListAltTitles(pluginID, mangaID)
	if altErr != nil {
		s.logger.Warn("alt titles", "error", altErr, "manga", mangaID)
		altTitles = nil
	}
	altSummaries, altSumErr := s.service.ListAltSummaries(pluginID, mangaID)
	if altSumErr != nil {
		s.logger.Warn("alt summaries", "error", altSumErr, "manga", mangaID)
		altSummaries = nil
	}
	allServers := s.service.AltTitleServers()
	var altTitleServers []pluginmanager.AltTitleServerEntry
	var altSummaryServers []pluginmanager.AltTitleServerEntry
	for _, srv := range allServers {
		if srv.ProviderPluginID == pluginID {
			if srv.Kind == "" || srv.Kind == "titles" || srv.Kind == "both" {
				altTitleServers = append(altTitleServers, srv)
			}
			if srv.Kind == "summaries" || srv.Kind == "both" {
				altSummaryServers = append(altSummaryServers, srv)
			}
		}
	}
	// Host-side chapter pagination: slice the full chapter list (newest-first)
	// so the detail page renders one page of chapters at a time.
	const chapterPageSize = 50
	chPage, _ := strconv.Atoi(r.URL.Query().Get("ChPage"))
	if chPage < 1 {
		chPage = 1
	}
	chTotal := len(chapters)
	chStart := min((chPage-1)*chapterPageSize, chTotal)
	chEnd := min(chStart+chapterPageSize, chTotal)
	s.renderPage(w, r, "views/detail", "", map[string]any{
		"PluginID":          pluginID,
		"PluginName":        pluginName,
		"PluginIcon":        pluginIcon,
		"MangaID":           mangaID,
		"Manga":             manga,
		"AltTitles":         altTitles,
		"AltSummaries":      altSummaries,
		"CurrentTitle":      manga.Title,
		"AltTitleServers":   altTitleServers,
		"AltSummaryServers": altSummaryServers,
		"Chapters":          chapters[chStart:chEnd],
		"Progress":          progress,
		"Continue":          continueTo,
		"InLibrary":         inLibrary,
		"Challenge":         challenge,
		"ChCurrentPage":     chPage,
		"ChTotalPages":      max((chTotal+chapterPageSize-1)/chapterPageSize, 1),
		"ChHasNext":         chEnd < chTotal,
		"ChHasPrev":         chPage > 1,
	})
}

// ContinuePoint names where the Continue button should resume.
type ContinuePoint struct {
	ChapterID string
	ChapterN  float64
	Page      int
	Started   bool // manga already has read history: label "Continue", not "Start Reading"
}

// computeContinue picks the resume target: the first in-progress chapter,
// else the first unread chapter, else nil when everything is finished.
func computeContinue(chapters []types.Chapter, progress map[string]database.ChapterProgress) *ContinuePoint {
	started := false
	for _, p := range progress {
		if p.LastPageRead > 0 || p.IsRead || p.Done {
			started = true
			break
		}
	}
	var firstUnread *ContinuePoint
	for _, c := range chapters {
		p, ok := progress[c.ID]
		if ok && p.LastPageRead > 0 {
			if p.TotalPages == 0 || p.LastPageRead < p.TotalPages {
				return &ContinuePoint{ChapterID: c.ID, ChapterN: c.ChapterNum, Page: p.LastPageRead, Started: true}
			}
			continue // fully read
		}
		// Chapters arrive newest-first; keep the LAST unread seen so the
		// fallback start point is the numerically lowest chapter.
		firstUnread = &ContinuePoint{ChapterID: c.ID, ChapterN: c.ChapterNum, Page: 1}
	}
	if firstUnread != nil {
		firstUnread.Started = started
	}
	return firstUnread
}

// continueFromHistory checks read_history for the most recently read chapter
// and returns it as the resume point if it's not fully read yet.
func (s *Server) continueFromHistory(pluginID, mangaID string, chapters []types.Chapter, progress map[string]database.ChapterProgress) *ContinuePoint {
	mangaRow := pluginID + "|" + mangaID
	lastChID, lastPage, ok := s.service.LastReadChapter(mangaRow)
	if !ok {
		return nil
	}
	for i, c := range chapters {
		if c.ID == lastChID {
			p, hasProgress := progress[c.ID]
			if !hasProgress || p.LastPageRead < p.TotalPages {
				return &ContinuePoint{ChapterID: c.ID, ChapterN: c.ChapterNum, Page: lastPage, Started: true}
			}
			// Fully read — advance to the next chapter (higher number = earlier in the slice).
			if i > 0 {
				next := chapters[i-1]
				return &ContinuePoint{ChapterID: next.ID, ChapterN: next.ChapterNum, Page: 1, Started: true}
			}
			return nil
		}
	}
	return nil
}
