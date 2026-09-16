package bridge

import (
	"strings"

	"goisekai/internal/config"
)

// genreIndex resolves plugin genre spellings to canonical names using the
// alias map from config. Built once at construction: the map is static, and
// this runs per manga detail, so it should not rebuild per call.
type genreIndex struct {
	// canonical maps a normalized spelling to the name that gets displayed
	// and stored. It holds both the canonical names themselves and every
	// alias pointing at one.
	canonical map[string]string
}

// newGenreIndex builds a lookup from the configured alias map. Each canonical
// name is registered under its own spelling too, so a plugin already sending
// "Sci-Fi" resolves without an alias entry.
func newGenreIndex(aliases map[string][]string) *genreIndex {
	idx := &genreIndex{canonical: make(map[string]string, len(aliases)*2)}
	for name, variants := range aliases {
		key := genreKey(name)
		if key == "" {
			continue
		}
		idx.canonical[key] = name
		for _, v := range variants {
			if vk := genreKey(v); vk != "" {
				idx.canonical[vk] = name
			}
		}
	}
	return idx
}

// indexGenreAliases builds the index from the config defaults, for callers that
// have no loaded config (tests, or a bridge constructed without one).
func indexGenreAliases() *genreIndex {
	return newGenreIndex(config.DefaultGenreAlias())
}

// normalize rewrites plugin-supplied genre names to their canonical spelling.
// Names with no matching alias are kept verbatim so no information is lost,
// duplicates are dropped, and the original order holds.
func (g *genreIndex) normalize(genres []string) []string {
	if len(genres) == 0 || g == nil {
		return genres
	}
	out := make([]string, 0, len(genres))
	seen := make(map[string]bool, len(genres))
	for _, raw := range genres {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		key := genreKey(name)
		if canonical, ok := g.canonical[key]; ok {
			name = canonical
			key = genreKey(canonical)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, name)
	}
	return out
}

// genreKey lowercases a genre name and drops every separator plugins vary on
// (spaces, hyphens, underscores, apostrophes, dots, slashes), so "Sci-Fi",
// "sci fi", "SCI_FI" and "scifi" all share one key. Names are short and none
// are distinguished by punctuation alone, so collapsing is safe.
func genreKey(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range strings.ToLower(name) {
		switch r {
		case ' ', '\t', '-', '_', '\'', '\u2019', '.', '/':
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
