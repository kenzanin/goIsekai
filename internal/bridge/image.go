package bridge

import (
	"fmt"
	"net/http"

	"goisekai/pkg/types"
)

// GetImage fetches image bytes for pluginID from url (with optional per-request
// headers) through the hostnet proxy, caching successful results by URL so a
// repeat lookup returns from memory without a network round-trip. Non-2xx
// responses and transport errors are returned and never cached.
func (s *AppService) GetImage(pluginID, url string, headers map[string]string) ([]byte, error) {
	s.imageMu.RLock()
	if cached, ok := s.imageCache[url]; ok {
		s.imageMu.RUnlock()
		return cached, nil
	}
	s.imageMu.RUnlock()

	resp, err := s.proxy.Request(pluginID, types.HTTPRequest{
		Method:  http.MethodGet,
		URL:     url,
		Headers: headers,
	})
	if err != nil {
		return nil, fmt.Errorf("bridge: get image %s: %w", url, err)
	}
	if resp.Status < 200 || resp.Status >= 300 {
		return nil, fmt.Errorf("bridge: get image %s: unexpected status %d", url, resp.Status)
	}
	body := []byte(resp.Body)

	s.imageMu.Lock()
	s.imageCache[url] = body
	s.imageMu.Unlock()

	return body, nil
}
