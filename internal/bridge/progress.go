package bridge

import (
	"fmt"
	"os"
	"path/filepath"

	"goisekai/internal/database"
	"goisekai/internal/logger"
)

// countCachedPages returns the number of page-image files already on disk for
// a chapter, or 0 when the cache dir is unset or the chapter dir is absent.
// Layout matches diskCachePath: images/<pluginID>/<mangaID>/<chapterID>/<hash8>.
func (s *AppService) countCachedPages(pluginID, mangaID, chapterID string) int {
	if s.cacheDir == "" {
		return 0
	}
	if pluginID == "" || mangaID == "" || chapterID == "" {
		return 0
	}
	dir := filepath.Join(s.cacheDir, "images", pluginID, mangaID, chapterID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() && e.Name() != completeCSVName {
			n++
		}
	}
	return n
}

// RecordRead appends a read-history entry for a chapter's current page,
// addressed by source identifiers.
func (s *AppService) RecordRead(pluginID, mangaID, chapterID string, pageNum int) error {
	chapterIntID, err := s.db.ResolveChapterIntID(pluginID, mangaID, chapterID)
	if err != nil {
		return fmt.Errorf("bridge: resolve chapter: %w", err)
	}
	if err := s.db.RecordRead(chapterIntID, pageNum); err != nil {
		return fmt.Errorf("bridge: record read: %w", err)
	}
	return nil
}

// SetChapterProgress records a chapter's last page read, addressed by source
// identifiers.
func (s *AppService) SetChapterProgress(pluginID, mangaID, chapterID string, lastPage int) error {
	chapterIntID, err := s.db.ResolveChapterIntID(pluginID, mangaID, chapterID)
	if err != nil {
		return fmt.Errorf("bridge: resolve chapter: %w", err)
	}
	if err := s.db.SetChapterProgress(chapterIntID, lastPage); err != nil {
		return fmt.Errorf("bridge: set chapter progress: %w", err)
	}
	// Reaching the final page means the whole chapter is cached — write
	// complete.csv so a later CBZ export can run fully offline. Best-effort.
	if total, err := s.db.GetChapterTotalPages(chapterIntID); err == nil && total > 0 && lastPage >= total {
		go func() {
			if err := s.MarkChapterComplete(pluginID, mangaID, chapterID); err != nil {
				logger.Warn("mark chapter complete", "error", err, "chapter", chapterID)
			}
		}()
	}
	return nil
}

// MarkChapterComplete records the chapter's page URLs in complete.csv so a
// later CBZ export can run fully offline.
func (s *AppService) MarkChapterComplete(pluginID, mangaID, chapterID string) error {
	pages, err := s.GetPageList(pluginID, chapterID)
	if err != nil {
		return fmt.Errorf("mark chapter complete: %w", err)
	}
	if err := s.writeCompleteCSV(pluginID, mangaID, chapterID, pages); err != nil {
		return fmt.Errorf("mark chapter complete: %w", err)
	}
	return nil
}

// GetChapterProgresses returns read progress keyed by source chapter id, with
// each chapter's CachedPages filled from the disk cache.
func (s *AppService) GetChapterProgresses(pluginID, mangaID string) (map[string]database.ChapterProgress, error) {
	mangaIntID, err := s.db.ResolveMangaIntID(pluginID, mangaID)
	if err != nil {
		return nil, fmt.Errorf("bridge: resolve manga: %w", err)
	}
	rows, err := s.db.GetChapterProgressForManga(mangaIntID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]database.ChapterProgress, len(rows))
	for _, r := range rows {
		r.CachedPages = s.countCachedPages(pluginID, mangaID, r.SourceChapterID)
		out[r.SourceChapterID] = r
	}
	return out, nil
}

// RefetchCover forces a fresh download of a manga's cover: L1/L2 cache
// entries for the current cover URL are dropped, the plugin is asked for the
// detail again (signed CDNs rotate cover URLs), and the cover is re-fetched
// and validated. A new URL overwrites the stored one so the next page render
// uses it. The re-downloaded bytes are cached as usual.
func (s *AppService) RefetchCover(pluginID, mangaID string) error {
	cached, err := s.db.GetMangaCached(pluginID, mangaID)
	if err != nil {
		return fmt.Errorf("bridge: refetch cover: %w", err)
	}
	if cached.CoverURL == "" {
		return fmt.Errorf("bridge: refetch cover: %s/%s has no cover URL", pluginID, mangaID)
	}
	// Evict caches for the current URL so the /image proxy refetches.
	s.imageMu.Lock()
	delete(s.imageCache, cached.CoverURL)
	s.imageMu.Unlock()
	if base := s.diskCachePath(pluginID, "", "", cached.CoverURL); base != "" {
		for _, ext := range []string{
			"." + string(FormatAVIF), "." + string(FormatWebP), ".img",
			"." + string(FormatJXL),
		} {
			_ = os.Remove(base + ext)
		}
	}
	// A signed CDN may have rotated the URL; refresh it from the plugin.
	if s.mgr != nil {
		if detail, err := s.mgr.GetMangaDetail(pluginID, mangaID); err == nil && detail.CoverURL != "" && detail.CoverURL != cached.CoverURL {
			if _, err := s.db.UpsertManga(database.Manga{
				PluginID:      pluginID,
				SourceMangaID: mangaID,
				Title:         cached.Title,
				CoverURL:      detail.CoverURL,
				Description:   cached.Description,
				Status:        cached.Status,
				InLibrary:     cached.InLibrary,
			}); err != nil {
				return fmt.Errorf("bridge: refetch cover update url: %w", err)
			}
			cached.CoverURL = detail.CoverURL
		}
	}
	if _, err := s.GetImage(pluginID, cached.CoverURL, nil, "", "", PrioHigh); err != nil {
		return fmt.Errorf("bridge: refetch cover download: %w", err)
	}
	return nil
}

// ToggleCoverDim toggles the cover dim overlay flag on the manga.
func (s *AppService) ToggleCoverDim(pluginID, mangaID string) error {
	mangaIntID, err := s.db.ResolveMangaIntID(pluginID, mangaID)
	if err != nil {
		return fmt.Errorf("bridge: resolve manga: %w", err)
	}
	dim, _, err := s.db.GetMangaCoverDim(mangaIntID)
	if err != nil {
		return fmt.Errorf("bridge: get cover_dim: %w", err)
	}
	if err := s.db.SetMangaCoverDim(mangaIntID, 1-dim); err != nil {
		return fmt.Errorf("bridge: set cover_dim: %w", err)
	}
	return nil
}

// SetChaptersSkip sets the skip flag for the given source chapters of a manga.
func (s *AppService) SetChaptersSkip(pluginID, mangaID string, chapterIDs []string, skip bool) error {
	mangaIntID, err := s.db.ResolveMangaIntID(pluginID, mangaID)
	if err != nil {
		return fmt.Errorf("bridge: resolve manga: %w", err)
	}
	if err := s.db.SetChaptersSkip(mangaIntID, chapterIDs, skip); err != nil {
		return fmt.Errorf("bridge: set chapters skip: %w", err)
	}
	return nil
}
