package bridge

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	defer func() { _ = f.Close() }()
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
	defer func() { _ = f.Close() }()
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
