package bridge

import (
	"strings"
)

// CanonicalGenres returns the 26 MangaDex `genre` group names. The list is
// fixed at build time and does not include demographic groups (Shounen, Seinen,
// Josei) or non-MangaDex genres such as Ecchi, Manhwa.
var CanonicalGenres = []string{
	"Action",
	"Adventure",
	"Boys' Love",
	"Comedy",
	"Crime",
	"Demons",
	"Drama",
	"Fantasy",
	"Girls' Love",
	"Historical",
	"Horror",
	"Isekai",
	"Magical Girls",
	"Martial Arts",
	"Mecha",
	"Medical",
	"Mystery",
	"Philosophical",
	"Psychological",
	"Romance",
	"Sci-Fi",
	"Slice of Life",
	"Sports",
	"Superhero",
	"Thriller",
	"Tragedy",
	"Wuxia",
}

// genreIndex resolves plugin genre spellings to canonical names. Built once at
// construction: the list is static, and this runs per manga detail, so it
// should not rebuild per call.
type genreIndex struct {
	// canonical maps a normalized spelling to the name that gets displayed
	// and stored. Only the canonical names are registered: a spelling that
	// differs by case or separator resolves, anything else stays verbatim.
	canonical map[string]string
}

// newGenreIndex builds a lookup from the canonical genre list. Each canonical
// name is registered under its own spelling too, so a plugin already sending
// "Sci-Fi" resolves without an alias entry.
func newGenreIndex(names []string) *genreIndex {
	idx := &genreIndex{canonical: make(map[string]string, len(names)*2)}
	for _, name := range names {
		key := genreKey(name)
		if key == "" {
			continue
		}
		idx.canonical[key] = name
	}
	return idx
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
