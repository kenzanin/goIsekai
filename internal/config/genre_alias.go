package config

import "strings"

// defaultGenreAlias lists the canonical genre names and the alternate
// spellings plugins are known to send, so a scraped "scifi" or "sci fi" lands
// as "Sci-Fi" instead of a separate genre. The keys double as the canonical
// vocabulary; a name that matches nothing is stored verbatim.
//
// The canonical set mirrors the `genre` tag group MangaDex publishes, so a
// library scraped from any plugin lines up with that vocabulary. Extend it in
// goisekai.ini with `genre_alias.<Name> = variant, variant` lines.
func DefaultGenreAlias() map[string][]string {
	return map[string][]string{
		"Action":        {},
		"Adventure":     {},
		"Boys' Love":    {"bl", "yaoi"},
		"Comedy":        {},
		"Crime":         {},
		"Demons":        {"demon", "devils", "devil"},
		"Drama":         {},
		"Fantasy":       {},
		"Girls' Love":   {"gl", "yuri"},
		"Historical":    {},
		"Horror":        {},
		"Isekai":        {},
		"Magical Girls": {"magical girl", "mahou shoujo"},
		"Martial Arts":  {"martial art"},
		"Mecha":         {},
		"Medical":       {},
		"Mystery":       {},
		"Philosophical": {},
		"Psychological": {},
		"Romance":       {"romantic"},
		"Sci-Fi":        {"scifi", "sci fi", "science fiction"},
		"Slice of Life": {"sol", "daily life"},
		"Sports":        {},
		"Superhero":     {"super heroes", "super hero"},
		"Thriller":      {"suspense"},
		"Tragedy":       {"tragic"},
		"Wuxia":         {},
	}
}

// addGenreAlias records one `[genre]` line.
func (c *Config) addGenreAlias(canonical, variants string) {
	c.GenreAlias = addAliasLine(c.GenreAlias, &c.aliasTouched, canonical, variants)
}

// DefaultStatusAlias lists the publication statuses the UI distinguishes and
// the spellings plugins send for them. A status matching nothing is stored as
// "Unknown" rather than as the raw plugin word.
func DefaultStatusAlias() map[string][]string {
	return map[string][]string{
		"Ongoing":   {"ongoing", "publishing", "releasing", "on going"},
		"Completed": {"completed", "complete", "finished", "ended"},
		"Hiatus":    {"hiatus", "uncertain", "on hold", "paused"},
		"Cancelled": {"cancelled", "canceled", "dropped", "axed"},
		"Unknown":   {},
	}
}

// addStatusAlias records one `[status]` line the same way addGenreAlias does
// for `[genre]`.
func (c *Config) addStatusAlias(canonical, variants string) {
	c.StatusAlias = addAliasLine(c.StatusAlias, &c.aliasTouched, canonical, variants)
}

// addAliasLine registers one canonical name and its variant list, replacing the
// built-in variants the first time a name appears in a config file.
func addAliasLine(m map[string][]string, touched *map[string]bool, canonical, variants string) map[string][]string {
	canonical = strings.TrimSpace(canonical)
	if canonical == "" {
		return m
	}
	if m == nil {
		m = map[string][]string{}
	}
	if *touched == nil {
		*touched = map[string]bool{}
	}
	var list []string
	if !(*touched)[canonical] {
		(*touched)[canonical] = true
	} else {
		list = m[canonical]
	}
	for v := range strings.SplitSeq(variants, ",") {
		if v = strings.TrimSpace(v); v != "" {
			list = append(list, v)
		}
	}
	if list == nil {
		list = []string{}
	}
	m[canonical] = list
	return m
}
