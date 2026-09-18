package httpserver

import (
	"errors"
	"goisekai/internal/database"
	"goisekai/internal/hostnet"
	"goisekai/pkg/types"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// buildMangaDetailData assembles the data map for a manga detail view.
// Used by both the GET handler and action handlers rendering inline.
func (s *Server) buildMangaDetailData(r *http.Request, pluginID, mangaID string) map[string]any {
	// Opening the detail page clears the library card's [New] badge.
	_ = s.service.ClearMangaNew(pluginID, mangaID)
	manga, chapters, err := s.service.GetMangaDetails(pluginID, mangaID)
	challenge := false
	if err != nil {
		if _, ok := errors.AsType[*hostnet.ChallengeError](err); ok {
			challenge = true
			s.logger.Warn("manga detail blocked by challenge", "plugin", pluginID, "manga", mangaID)
		} else {
			s.logger.Error("manga detail", "error", err, "plugin", pluginID, "manga", mangaID)
			return nil
		}
	}
	// Source plugins rarely supply an author; fall back to the one captured by
	// an enrichment provider, if any.
	if strings.TrimSpace(manga.Author) == "" {
		if author, ok := s.service.StoredAuthor(pluginID, mangaID); ok {
			manga.Author = author
		}
	}
	progress, err := s.service.GetChapterProgresses(pluginID, mangaID)
	if err != nil {
		s.logger.Warn("chapter progress", "error", err, "manga", mangaID)
		progress = map[string]database.ChapterProgress{}
	}
	continueTo := computeContinue(chapters, progress)
	if lastCont := s.continueFromHistory(pluginID, mangaID, chapters, progress); lastCont != nil {
		continueTo = lastCont
	}
	inLibrary := s.service.IsInLibrary(pluginID, mangaID)
	lastSynced, _ := s.service.LibrarySyncState(pluginID, mangaID, time.Now())

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
	altTitles, _ := s.service.ListAltTitles(pluginID, mangaID)
	altSummaries, _ := s.service.ListAltSummaries(pluginID, mangaID)
	cats, _ := s.service.ListCategories(pluginID, mangaID)
	rels, _ := s.service.ListRelated(pluginID, mangaID)
	s.logger.Debug("enrichment cache", "plugin", pluginID, "manga", mangaID, "categories", len(cats), "related", len(rels))
	overrideGenres, _, _ := s.service.GetMangaGenres(pluginID, mangaID)

	const chapterPageSize = 50
	chPage, _ := strconv.Atoi(r.URL.Query().Get("ChPage"))
	if chPage < 1 {
		chPage = 1
	}
	chTotal := len(chapters)
	chStart := min((chPage-1)*chapterPageSize, chTotal)
	chEnd := min(chStart+chapterPageSize, chTotal)

	return map[string]any{
		"PluginID":       pluginID,
		"PluginName":     pluginName,
		"PluginIcon":     pluginIcon,
		"MangaID":        mangaID,
		"Manga":          manga,
		"AltTitles":      altTitles,
		"AltSummaries":   altSummaries,
		"CurrentTitle":   manga.Title,
		"Chapters":       chapters[chStart:chEnd],
		"Progress":       progress,
		"Continue":       continueTo,
		"InLibrary":      inLibrary,
		"Challenge":      challenge,
		"ChCurrentPage":  chPage,
		"ChTotalPages":   max((chTotal+chapterPageSize-1)/chapterPageSize, 1),
		"ChHasNext":      chEnd < chTotal,
		"ChHasPrev":      chPage > 1,
		"Categories":     cats,
		"Related":        rels,
		"PluginGenres":   manga.RawGenres,
		"OverrideGenres": overrideGenres,
		"Genres":         manga.Genres,
		"CoverDim":       manga.CoverDim,
		"LastSynced":     lastSynced,
	}
}

// viewMangaDetail renders a manga's info plus its chapter list.
func (s *Server) viewMangaDetail(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	data := s.buildMangaDetailData(r, pluginID, mangaID)
	if data == nil {
		http.Error(w, "failed to load manga details", http.StatusBadGateway)
		return
	}
	// Action handlers (library toggle, genre chips, enrichment) redirect back
	// here, and the browser follows that 303 with X-Partial still set: render
	// the partial so the handler can swap it into #content.
	if r.Header.Get("X-Partial") == "true" {
		s.renderPage(w, r, "views/detail", "", data)
		return
	}
	s.renderPage(w, r, "views/detail", "", data)
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
		if ok && p.IsSkipped {
			continue // user explicitly skipped this chapter
		}
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
	lastChID, lastPage, ok := s.service.LastReadChapter(pluginID, mangaID)
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
