package database

import (
	"strings"
	"unicode"
)

// normalizeTitle lowercases, trims, strips non-alphanumeric characters
// (keeping letters, digits, and spaces), and collapses whitespace runs.
func normalizeTitle(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevSpace = false
		} else if !prevSpace { // non-alnum → treat as space separator
			b.WriteByte(' ')
			prevSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

// spansPlugins reports whether the member indices come from at least two
// different plugin IDs.
func spansPlugins(members map[int]struct{}, all []Manga) bool {
	var seen string
	for idx := range members {
		if seen == "" {
			seen = all[idx].PluginID
		} else if seen != all[idx].PluginID {
			return true
		}
	}
	return false
}
