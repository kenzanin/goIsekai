package bridge

import (
	"fmt"
	"os"
	"path/filepath"

	"goisekai/internal/logger"
)

// ExportCBZ builds a .cbz archive of one chapter's pages in reading order and
// returns the path to the written file. It prefers an offline path — when
// complete.csv exists and every page is still on disk, no plugin/network call
// is made. Otherwise it fetches (cache-first) via the plugin and records
// complete.csv for future offline exports.
func (s *AppService) ExportCBZ(pluginID, mangaID, chapterID, title string) (string, error) {
	dir := filepath.Join(s.exportDir(), pluginID, mangaID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("bridge: export cbz: mkdir: %w", err)
	}
	name := sanitizeFilename(title) + ".cbz"
	path := filepath.Join(dir, name)

	// Offline path: complete.csv + disk cache, no plugin/network needed.
	if urls := s.readCompleteCSV(pluginID, mangaID, chapterID); len(urls) > 0 {
		images := make([][]byte, 0, len(urls))
		all := true
		for _, u := range urls {
			data, ok := s.readCachedImage(pluginID, mangaID, chapterID, u)
			if !ok {
				all = false
				break
			}
			images = append(images, data)
		}
		if all {
			n, err := zipImages(path, images)
			if err != nil {
				return "", fmt.Errorf("bridge: export cbz (offline): %w", err)
			}
			logger.Info("exported cbz (offline)", "path", path, "pages", n)
			return path, nil
		}
		// Cache incomplete — fall through to the online path.
	}

	pages, err := s.GetPageList(pluginID, chapterID)
	if err != nil {
		return "", err
	}
	if len(pages) == 0 {
		return "", fmt.Errorf("bridge: export cbz: no pages for chapter %s", chapterID)
	}

	images := make([][]byte, 0, len(pages))
	failed := 0
	for _, p := range pages {
		data, imgErr := s.GetImage(pluginID, p.URL, p.Headers, mangaID, chapterID)
		if imgErr != nil {
			logger.Warn("export cbz: skip page", "error", imgErr)
			failed++
			continue
		}
		images = append(images, data)
	}
	if len(images) == 0 {
		return "", fmt.Errorf("bridge: export cbz: all %d pages failed", failed)
	}
	if _, err := zipImages(path, images); err != nil {
		return "", fmt.Errorf("bridge: export cbz: %w", err)
	}
	if failed == 0 {
		if werr := s.writeCompleteCSV(pluginID, mangaID, chapterID, pages); werr != nil {
			logger.Warn("export cbz: write complete.csv", "error", werr)
		}
	}
	logger.Info("exported cbz", "path", path, "pages", len(images), "skipped", failed)
	return path, nil
}
