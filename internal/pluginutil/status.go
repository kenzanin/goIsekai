package pluginutil

// Statuses returns the canonical host status vocabulary. These are the only
// status values the host accepts; a plugin reporting a site-specific word
// (publishing, finished, on hiatus, ...) must translate it to one of these
// before returning it.
//
// The list is a contract, not configuration: library statistics compare stored
// values against these labels directly, so they stay literals in one place.
func Statuses() []string {
	return []string{"Ongoing", "Completed", "Hiatus", "Cancelled", "Upcoming"}
}

// NormalizeStatus returns the canonical status label matching raw
// case-insensitively, or the empty string when raw is not part of the
// canonical vocabulary. The host never emits, echoes, or invents a
// non-canonical status.
//
// Matching is exact rather than substring-based: a substring match would let
// "not yet published" resolve through "published" and manufacture a status
// from a word no plugin meant as one.
func NormalizeStatus(raw string) string {
	key := toLowerASCII(raw)
	for _, status := range Statuses() {
		if toLowerASCII(status) == key {
			return status
		}
	}
	return ""
}

// toLowerASCII returns a lowercase copy of s using only ASCII folding.
// Status values from upstream sites are ASCII, so unicode.ToLower is not needed.
func toLowerASCII(s string) string {
	var out []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			out = append(out, c+32)
		} else {
			out = append(out, c)
		}
	}
	return string(out)
}
