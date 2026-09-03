package bridge

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"goisekai/internal/logger"
	"goisekai/pkg/types"
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

// completeCSVName is the marker file written into a chapter's cache dir once
// every page is cached on disk, enabling a fully-offline CBZ export.
const completeCSVName = "complete.csv"

// chapterCacheDir returns the on-disk cache directory for a chapter's pages.
func (s *AppService) chapterCacheDir(pluginID, mangaID, chapterID string) string {
	return filepath.Join(s.cacheDir, "images", pluginID, mangaID, chapterID)
}

// writeCompleteCSV records a chapter's ordered page URLs so an offline export
// can rebuild the CBZ from the disk cache alone (no plugin/network call).
func (s *AppService) writeCompleteCSV(pluginID, mangaID, chapterID string, pages []types.Page) error {
	dir := s.chapterCacheDir(pluginID, mangaID, chapterID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(dir, completeCSVName))
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	for _, p := range pages {
		_ = w.Write([]string{p.URL})
	}
	w.Flush()
	return w.Error()
}

// readCompleteCSV returns the ordered page URLs recorded in complete.csv, or
// nil when the marker is absent or unreadable.
func (s *AppService) readCompleteCSV(pluginID, mangaID, chapterID string) []string {
	f, err := os.Open(filepath.Join(s.chapterCacheDir(pluginID, mangaID, chapterID), completeCSVName))
	if err != nil {
		return nil
	}
	defer f.Close()
	recs, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil
	}
	urls := make([]string, 0, len(recs))
	for _, r := range recs {
		if len(r) > 0 && r[0] != "" {
			urls = append(urls, r[0])
		}
	}
	return urls
}

// readCachedImage reads a page's bytes from the L2 disk cache only — it never
// touches the network. ok is false when the page is not cached on disk.
func (s *AppService) readCachedImage(pluginID, mangaID, chapterID, url string) ([]byte, bool) {
	base := s.diskCachePath(pluginID, mangaID, chapterID, url)
	if base == "" {
		return nil, false
	}
	for _, ext := range []string{".webp", ".img"} {
		if data, err := os.ReadFile(base + ext); err == nil && validateImageFast(data) {
			return data, true
		}
	}
	return nil, false
}

// zipImages writes ordered image byte slices into a .cbz at path. Entries are
// zero-padded to 4 digits so lexicographic order == reading order.
func zipImages(path string, images [][]byte) (int, error) {
	out, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	zw := zip.NewWriter(out)
	n := 0
	for i, data := range images {
		entry := fmt.Sprintf("%04d%s", i+1, imageExt(data))
		w, werr := zw.Create(entry)
		if werr != nil {
			_ = zw.Close()
			_ = out.Close()
			return 0, fmt.Errorf("zip entry %s: %w", entry, werr)
		}
		if _, werr = w.Write(data); werr != nil {
			_ = zw.Close()
			_ = out.Close()
			return 0, fmt.Errorf("write %s: %w", entry, werr)
		}
		n++
	}
	if err := zw.Close(); err != nil {
		_ = out.Close()
		return 0, err
	}
	if err := out.Close(); err != nil {
		return 0, err
	}
	return n, nil
}

// MarkChapterComplete writes complete.csv when a chapter is fully cached. It
// is best-effort: called once the last page has been read (all pages fetched).
func (s *AppService) MarkChapterComplete(pluginID, mangaID, chapterID string) error {
	pages, err := s.GetPageList(pluginID, chapterID)
	if err != nil {
		return err
	}
	if len(pages) == 0 {
		return nil
	}
	if s.countCachedPages(pluginID, mangaID, chapterID) < len(pages) {
		return nil // not fully cached yet
	}
	return s.writeCompleteCSV(pluginID, mangaID, chapterID, pages)
}

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
