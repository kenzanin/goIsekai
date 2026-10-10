package httpserver

import (
	"goisekai/internal/database"
	"goisekai/pkg/types"
)

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
			// TotalPages == 0 means the page count was never recorded, not that the
			// chapter is finished. computeContinue already treats that case as
			// in-progress; this used to read it as fully read and skip ahead to a
			// newer, unread chapter, which is why Continue could land somewhere the
			// reader had never opened.
			unfinished := !hasProgress || p.TotalPages == 0 || p.LastPageRead < p.TotalPages
			if unfinished {
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
