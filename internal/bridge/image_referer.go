package bridge

import (
	"maps"
	neturl "net/url"
	"strings"
)

// hasHeader reports whether headers already carry the named key (case-insensitive).
func hasHeader(headers map[string]string, name string) bool {
	for k := range headers {
		if strings.EqualFold(k, name) {
			return true
		}
	}
	return false
}

// withReferer returns a copy of headers carrying the given Referer (input never
// mutated). An empty ref leaves headers untouched.
func withReferer(headers map[string]string, ref string) map[string]string {
	if ref == "" {
		return headers
	}
	out := make(map[string]string, len(headers)+1)
	maps.Copy(out, headers)
	out["Referer"] = ref
	return out
}

// memorizedReferer returns the Referer that previously succeeded for this
// image host, so gated CDNs (rx.resmk.org) pay the extra 403 round-trip only
// once per host instead of once per image.
func (s *AppService) memorizedReferer(host string) (string, bool) {
	s.imgRefMu.Lock()
	defer s.imgRefMu.Unlock()
	if s.imgRef == nil {
		s.imgRef = make(map[string]string)
	}
	ref, ok := s.imgRef[host]
	return ref, ok
}

// memorizeReferer records the Referer that succeeded for an image host.
func (s *AppService) memorizeReferer(host, ref string) {
	s.imgRefMu.Lock()
	defer s.imgRefMu.Unlock()
	if s.imgRef == nil {
		s.imgRef = make(map[string]string)
	}
	s.imgRef[host] = ref
}

// refererCandidates are the Referer values to try for one image fetch, in
// order, when the caller set none (explicit Referer always wins, untouched).
// The image's own origin leads: a CDN that serves its own origin (uploads.
// mangadex.org) returns the real image there, while a foreign Referer (e.g.
// the plugin's site) silently serves a placeholder — a 200 with wrong bytes,
// undetectable by status. The plugin's siteURL follows as fallback because
// gated CDNs (rx.resmk.org) reject the origin and accept only the plugin
// site. A failure of the first candidate costs one extra request, only for
// CDNs that gate on the plugin site.
func refererCandidates(imageURL, siteURL string) []string {
	var out []string
	if u, err := neturl.Parse(imageURL); err == nil && u.Scheme != "" && u.Host != "" {
		out = append(out, u.Scheme+"://"+u.Host+"/")
	}
	if siteURL != "" && (len(out) == 0 || out[0] != siteURL) {
		out = append(out, siteURL)
	}
	// Last-resort: some CDNs (comix.to's lov.*.site) reject ANY Referer and
	// serve only with none. Working plugins memoize at an earlier candidate and
	// never reach this one, so appending it is a no-op for them. Without it, a
	// host that 403s on both image-origin and plugin-site Referers has no
	// no-Referer attempt and every image fails.
	out = append(out, "")
	return out
}
