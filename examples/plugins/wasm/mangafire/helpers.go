//go:build wasip1

package main

import "strings"

// vrfURL builds the full API URL: path + url-encoded params + vrf param.
func vrfURL(apiPath string, params map[string]string) string {
	sig := vrf.Sign(apiPath, params)
	u := apiBase + apiPath
	if len(params) == 0 {
		return u + "?vrf=" + sig
	}
	q := neturl.Values{}
	for k, v := range params {
		q.Set(k, v)
	}
	return u + "?" + q.Encode() + "&vrf=" + sig
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// stripHTML removes HTML tags from a synopsis HTML string.
func stripHTML(s string) string {
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	re := strings.NewReplacer(
		"&quot;", "\"", "&#039;", "'", "&amp;", "&", "&lt;", "<", "&gt;", ">",
	)
	s = re.Replace(s)
	var b strings.Builder
	inTag := false
	for _, c := range s {
		if c == '<' {
			inTag = true
			continue
		}
		if c == '>' {
			inTag = false
			continue
		}
		if !inTag {
			b.WriteRune(c)
		}
	}
	return strings.TrimSpace(b.String())
}

// normalizeStatus maps MangaFire's native status codes to human-readable labels.
// MangaFire returns e.g. "releasing" / "finished" / "on_hold" / "discontinued" /
// "not_published"; these read awkwardly next to other plugins' "Ongoing" etc.
func normalizeStatus(s string) string {
	switch s {
	case "releasing":
		return "Ongoing"
	case "finished":
		return "Completed"
	case "on_hold":
		return "Hiatus"
	case "discontinued":
		return "Dropped"
	case "not_published", "upcoming":
		return "Upcoming"
	default:
		return s
	}
}

// sanitizeTitle strips MangaFire's HTML-escaped title entities.
func sanitizeTitle(s string) string {
	s = strings.ReplaceAll(s, "&#039;", "'")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&amp;", "&")
	return strings.TrimSpace(s)
}
