package httpserver

import (
	"errors"
	"net/http"

	"goisekai/internal/database"
	"goisekai/internal/hostnet"
	"goisekai/pkg/types"
)

// apiMangaDetailResponse is the JSON shape for GET /manga/{pluginID}/{mangaID}.
type apiMangaDetailResponse struct {
	Title       string              `json:"title"`
	Status      string              `json:"status"`
	Description string              `json:"description"`
	CoverURL    string              `json:"cover_url"`
	InLibrary   bool                `json:"in_library"`
	AltTitles   []map[string]string `json:"alt_titles"`
	Chapters    []apiChapterItem    `json:"chapters"`
	Continue    *apiContinuePoint   `json:"continue"`
}

// apiChapterItem is the per-chapter JSON shape inside manga detail.
type apiChapterItem struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	ChapterNum float64 `json:"chapter_num"`
	IsRead     bool    `json:"is_read"`
	LastPage   int     `json:"last_page"`
	TotalPages int     `json:"total_pages"`
}

// apiContinuePoint names where the Continue button should resume.
type apiContinuePoint struct {
	ChapterID string  `json:"chapter_id"`
	ChapterN  float64 `json:"chapter_num"`
	Page      int     `json:"page"`
	Started   bool    `json:"started"` // any read history: label "Continue", not "Start Reading"
}

// apiMangaDetail mirrors viewMangaDetail: manga metadata + chapters with progress.
func (s *Server) apiMangaDetail(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	// Opening the detail clears the [New] badge.
	_ = s.service.ClearMangaNew(pluginID, mangaID)
	manga, chapters, err := s.service.GetMangaDetails(pluginID, mangaID)
	if err != nil {
		if _, ok := errors.AsType[*hostnet.ChallengeError](err); ok {
			writeErr(w, http.StatusForbidden, "source requires verification")
			return
		}
		s.logger.Error("api manga detail", "error", err, "plugin", pluginID, "manga", mangaID)
		writeErr(w, http.StatusNotFound, "manga not found")
		return
	}
	progress, err := s.service.GetChapterProgresses(pluginID, mangaID)
	if err != nil {
		s.logger.Warn("api chapter progress", "error", err)
		progress = map[string]database.ChapterProgress{}
	}
	// Chapters arrive newest-first (desc) — keep that order per D7.
	apiChapters := make([]apiChapterItem, 0, len(chapters))
	for _, c := range chapters {
		ac := apiChapterItem{ID: c.ID, Title: c.Title, ChapterNum: c.ChapterNum}
		if p, ok := progress[c.ID]; ok {
			ac.IsRead = p.Done
			ac.LastPage = p.LastPageRead
			ac.TotalPages = p.TotalPages
		}
		apiChapters = append(apiChapters, ac)
	}
	// Compute continue point.
	continuePt := computeContinueAPI(chapters, progress)
	altRows, altErr := s.service.ListAltTitles(pluginID, mangaID)
	if altErr != nil {
		s.logger.Warn("api alt titles", "error", altErr)
	}
	writeJSON(w, http.StatusOK, apiMangaDetailResponse{
		AltTitles:   altTitleRowsPayload(altRows),
		Title:       manga.Title,
		Status:      manga.Status,
		Description: manga.Description,
		CoverURL:    manga.CoverURL,
		InLibrary:   s.service.IsInLibrary(pluginID, mangaID),
		Chapters:    apiChapters,
		Continue:    continuePt,
	})
}

// computeContinueAPI picks the resume target for the JSON API.
func computeContinueAPI(chapters []types.Chapter, progress map[string]database.ChapterProgress) *apiContinuePoint {
	started := false
	for _, p := range progress {
		if p.LastPageRead > 0 || p.IsRead || p.Done {
			started = true
			break
		}
	}
	var firstUnread *apiContinuePoint
	for _, c := range chapters {
		p, ok := progress[c.ID]
		if ok && p.LastPageRead > 0 {
			if p.TotalPages == 0 || p.LastPageRead < p.TotalPages {
				return &apiContinuePoint{ChapterID: c.ID, ChapterN: c.ChapterNum, Page: p.LastPageRead, Started: true}
			}
			continue
		}
		firstUnread = &apiContinuePoint{ChapterID: c.ID, ChapterN: c.ChapterNum, Page: 1}
	}
	if firstUnread != nil {
		firstUnread.Started = started
	}
	return firstUnread
}
