package bridge

import (
	"os"

	"goisekai/internal/logger"
)

// cachedImage returns the bytes for url from the in-memory or disk cache.
//
// It is a function rather than inline code because GetImage has to run it twice:
// once before joining the singleflight, and once after winning it. The second
// probe closes a window that is otherwise a duplicate upstream fetch - see the
// comment at that call site.
func (s *AppService) cachedImage(pluginID, mangaID, chapterID, url string) ([]byte, bool) {
	// L1: in-memory cache.
	s.imageMu.RLock()
	cached, ok := s.imageCache[url]
	s.imageMu.RUnlock()
	if ok {
		logger.Debug("image cache: L1 hit", "url", url)
		return cached, true
	}

	// L2: disk cache. Converted images are stored as <key>.<format>, anything
	// that kept its original bytes (gif passthrough, undecodable, "original"
	// mode) as <key>.img. Older builds used ".webp"/".img" only, so every
	// format we can write is tried before declaring a miss. JXL comes last:
	// when a format switch left several copies behind, the renderable one wins.
	base := s.diskCachePath(pluginID, mangaID, chapterID, url)
	if base == "" {
		return nil, false
	}
	for _, ext := range []string{
		"." + string(FormatAVIF), "." + string(FormatWebP), ".img",
		"." + string(FormatJXL),
	} {
		data, err := os.ReadFile(base + ext)
		if err != nil {
			continue
		}
		if !validateImageFast(data) {
			// Invalid cached image: delete stale file and treat as miss.
			_ = os.Remove(base + ext)
			continue
		}
		s.imageMu.Lock()
		s.imageCache[url] = data
		s.imageMu.Unlock()
		logger.Debug("image cache: L2 hit", "url", url, "ext", ext)
		return data, true
	}
	return nil, false
}
