package bridge

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"goisekai/internal/logger"
)

// imageExt sniffs an image's magic bytes and returns a filename extension.
// WebP conversion means cached pages are usually .webp; gifs pass through.
func imageExt(data []byte) string {
	switch {
	case len(data) >= 4 && bytes.HasPrefix(data, []byte("GIF8")):
		return ".gif"
	case len(data) >= 12 && bytes.HasPrefix(data, []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")):
		return ".webp"
	case len(data) >= 8 && bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		return ".png"
	case len(data) >= 3 && bytes.Equal(data[:3], []byte{0xFF, 0xD8, 0xFF}):
		return ".jpg"
	default:
		return ".img"
	}
}

// sanitizeFilename strips characters that are unsafe in a filesystem name.
func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_",
		"?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return replacer.Replace(name)
}

// exportDir returns the directory CBZ files are written to. It lives outside
// images/ so ClearAllCache (which removes images/) never deletes exports.
func (s *AppService) exportDir() string {
	return filepath.Join(s.cacheDir, "exports")
}

// ExportCBZ builds a .cbz archive of one chapter's pages in reading order and
// returns the path to the written file. Images come from the cache first
// (GetImage), so an already-read chapter exports with no network traffic; any
// missing page is fetched and cached on the way.
func (s *AppService) ExportCBZ(pluginID, mangaID, chapterID, title string) (string, error) {
	pages, err := s.GetPageList(pluginID, chapterID)
	if err != nil {
		return "", err
	}
	if len(pages) == 0 {
		return "", fmt.Errorf("bridge: export cbz: no pages for chapter %s", chapterID)
	}

	dir := filepath.Join(s.exportDir(), pluginID, mangaID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("bridge: export cbz: mkdir: %w", err)
	}
	name := sanitizeFilename(title) + ".cbz"
	path := filepath.Join(dir, name)

	out, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("bridge: export cbz: create: %w", err)
	}
	zw := zip.NewWriter(out)

	failed := 0
	for i, p := range pages {
		data, imgErr := s.GetImage(pluginID, p.URL, p.Headers, mangaID, chapterID)
		if imgErr != nil {
			logger.Warn("export cbz: skip page", "page", i+1, "error", imgErr)
			failed++
			continue
		}
		// Zero-padded to 4 digits so lexicographic order == reading order.
		entry := fmt.Sprintf("%04d%s", i+1, imageExt(data))
		w, werr := zw.Create(entry)
		if werr != nil {
			_ = zw.Close()
			_ = out.Close()
			return "", fmt.Errorf("bridge: export cbz: zip entry %s: %w", entry, werr)
		}
		if _, werr = w.Write(data); werr != nil {
			_ = zw.Close()
			_ = out.Close()
			return "", fmt.Errorf("bridge: export cbz: write %s: %w", entry, werr)
		}
	}
	if err := zw.Close(); err != nil {
		_ = out.Close()
		return "", fmt.Errorf("bridge: export cbz: close zip: %w", err)
	}
	if err := out.Close(); err != nil {
		return "", fmt.Errorf("bridge: export cbz: close file: %w", err)
	}
	if failed == len(pages) {
		_ = os.Remove(path)
		return "", fmt.Errorf("bridge: export cbz: all %d pages failed", failed)
	}
	logger.Info("exported cbz", "path", path, "pages", len(pages), "skipped", failed)
	return path, nil
}

// ClearMangaCache removes every cached image file for one manga. It frees disk
// only; the in-memory L1 cache is left untouched (bounded and short-lived).
func (s *AppService) ClearMangaCache(pluginID, mangaID string) error {
	if s.cacheDir == "" {
		return nil
	}
	dir := filepath.Join(s.cacheDir, "images", pluginID, mangaID)
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("bridge: clear manga cache: %w", err)
	}
	logger.Info("cleared manga cache", "plugin", pluginID, "manga", mangaID)
	return nil
}

// ClearAllCache removes the entire image cache directory and drops the L1 map.
func (s *AppService) ClearAllCache() error {
	if s.cacheDir == "" {
		return nil
	}
	dir := filepath.Join(s.cacheDir, "images")
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("bridge: clear all cache: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("bridge: clear all cache: mkdir: %w", err)
	}
	s.imageMu.Lock()
	s.imageCache = make(map[string][]byte)
	s.imageMu.Unlock()
	logger.Info("cleared all image cache")
	return nil
}

// CacheSize returns the total bytes of all cached image files on disk.
func (s *AppService) CacheSize() (int64, error) {
	if s.cacheDir == "" {
		return 0, nil
	}
	dir := filepath.Join(s.cacheDir, "images")
	var total int64
	err := filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("bridge: cache size: %w", err)
	}
	return total, nil
}
