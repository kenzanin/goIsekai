package httpserver

import (
	"goisekai/internal/database"
	"goisekai/pkg/types"
)

// chapterNeighbors returns the chapter IDs before/after chapterID in the
// plugin's chapter list (empty string when at either end), skipping any
// chapters the user has marked as skipped.
// chapterNeighbors resolves prev/next in READING order. Chapters arrive
// newest-first (desc), so the older chapter (prev) sits at i+1 and the
// newer chapter (next) at i-1.
func chapterNeighbors(chapters []types.Chapter, progress map[string]database.ChapterProgress, chapterID string) (prev, next string) {
	for i, c := range chapters {
		if c.ID != chapterID {
			continue
		}
		for j := i + 1; j < len(chapters); j++ {
			if progress[chapters[j].ID].IsSkipped {
				continue
			}
			prev = chapters[j].ID
			break
		}
		for j := i - 1; j >= 0; j-- {
			if progress[chapters[j].ID].IsSkipped {
				continue
			}
			next = chapters[j].ID
			break
		}
		return prev, next
	}
	return "", ""
}

// readerChapters resolves the manga and its chapter list for the reader. The
// persisted copy is preferred: it is the same list the detail page shows, and a
// live plugin fetch can come back partial, which would leave navigation with no
// next chapter even though the chapter exists. A manga that was never synced
// falls back to the plugin.
func (s *Server) readerChapters(pluginID, mangaID string) (types.Manga, []types.Chapter, error) {
	manga, chapters, err := s.service.CachedMangaAndChapters(pluginID, mangaID)
	if err == nil && len(chapters) > 0 {
		return manga, chapters, nil
	}

	manga, chapters, err = s.service.GetMangaDetails(pluginID, mangaID)
	if err != nil {
		return manga, chapters, err
	}
	if len(chapters) == 0 {
		live, liveErr := s.service.GetChapterList(pluginID, mangaID)
		if liveErr != nil {
			s.logger.Error("reader chapter list fallback", "error", liveErr, "plugin", pluginID, "manga", mangaID)
			return manga, chapters, nil
		}
		chapters = live
	}
	return manga, chapters, nil
}
