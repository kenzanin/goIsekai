package pluginutil

// DefaultStatusMap returns the canonical host status vocabulary as a map
// from lowercase upstream values to the canonical labels (Ongoing, Completed,
// Hiatus, Dropped, Upcoming). Plugins may extend this map with site-specific
// keys before passing it to NormalizeStatus.
func DefaultStatusMap() map[string]string {
	return map[string]string{
		// Ongoing
		"ongoing":    "Ongoing",
		"on-going":   "Ongoing",
		"on going":   "Ongoing",
		"on_going":   "Ongoing",
		"releasing":  "Ongoing",
		"release":    "Ongoing",
		"publishing": "Ongoing",
		"published":  "Ongoing",
		"publish":    "Ongoing",
		// Completed
		"completed":   "Completed",
		"complete":    "Completed",
		"finished":    "Completed",
		"finish":      "Completed",
		"finished_up": "Completed",
		// Hiatus
		"hiatus":  "Hiatus",
		"onhold":  "Hiatus",
		"on hold": "Hiatus",
		"on_hold": "Hiatus",
		// Dropped
		"dropped":      "Dropped",
		"drop":         "Dropped",
		"cancelled":    "Dropped",
		"canceled":     "Dropped",
		"cancel":       "Dropped",
		"discontinued": "Dropped",
		// Upcoming
		"upcoming":          "Upcoming",
		"not_yet_published": "Upcoming",
		"not_published":     "Upcoming",
		"not yet published": "Upcoming",
	}
}

// NormalizeStatus maps rawStatus to a canonical status using the provided
// mapping. Keys are matched case-insensitively. When the map is nil,
// DefaultStatusMap is used. The raw value is returned unchanged when it
// matches no key.
func NormalizeStatus(m map[string]string, raw string) string {
	if m == nil {
		m = DefaultStatusMap()
	}
	if mapped, ok := m[toLowerASCII(raw)]; ok {
		return mapped
	}
	return raw
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
