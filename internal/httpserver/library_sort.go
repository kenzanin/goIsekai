package httpserver

import (
	"sort"
	"strconv"
	"strings"

	"goisekai/internal/database"
)

// The library grid renders one page at a time, so sorting has to happen over
// the whole list before it is sliced - sorting the rendered page would only
// reorder the 24 cards the user happens to be looking at.

// librarySorts are the accepted values of ?sort=. Anything else keeps the
// previous behaviour (most recently updated first) rather than erroring, since
// this is a preference in a URL bar.
var librarySorts = []string{"updated", "title", "read", "progress"}

// libraryStatuses are the accepted values of ?status=, which answers "which of
// these did I actually read" - the question a source migration starts from.
var libraryStatuses = []string{"all", "reading", "unread", "done"}

// statsByManga flattens the row-ID-keyed progress stats into the
// pluginID:sourceMangaID key the manga list uses, so sorting and filtering can
// read a manga's numbers without a second lookup.
func statsByManga(mangas []database.Manga, stats []database.LibraryMangaStats) map[string]database.LibraryMangaStats {
	byKey := make(map[string]database.LibraryMangaStats, len(stats))
	for _, st := range stats {
		byKey[st.MangaID] = st
	}
	return byKey
}

// readState is the read/total pair a card shows, which is also what "reading"
// and "done" are defined against.
func readState(m database.Manga, stats map[string]database.LibraryMangaStats) (read, total int) {
	// LibraryMangaStats.MangaID is the mangas row-ID, not pluginID:sourceMangaID -
	// reading it the other way is why every manga looked unread.
	st, ok := stats[strconv.FormatInt(m.ID, 10)]
	if !ok {
		return 0, 0
	}
	return st.ReadChapters, st.TotalChapters
}

// sortLibrary orders the library in place. It keeps the incoming order for ties,
// so an unknown sort key leaves the caller's ordering untouched.
func sortLibrary(mangas []database.Manga, stats map[string]database.LibraryMangaStats, key string) {
	switch key {
	case "title":
		sort.SliceStable(mangas, func(i, j int) bool {
			return strings.ToLower(mangas[i].Title) < strings.ToLower(mangas[j].Title)
		})
	case "read":
		// Most chapters read first. A manga with 300 chapters read outranks one
		// with 10 read, which is what "I was actually reading this" means.
		sort.SliceStable(mangas, func(i, j int) bool {
			ri, _ := readState(mangas[i], stats)
			rj, _ := readState(mangas[j], stats)
			return ri > rj
		})
	case "progress":
		// Highest completion first, so a nearly-finished series sorts above a
		// long one that was barely started. Denominator 0 is treated as
		// unfinished rather than dividing by it.
		sort.SliceStable(mangas, func(i, j int) bool {
			ri, ti := readState(mangas[i], stats)
			rj, tj := readState(mangas[j], stats)
			return ratio(ri, ti) > ratio(rj, tj)
		})
	}
}

func ratio(read, total int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(read) / float64(total)
}

// filterLibrary keeps the mangas matching status and tag. Empty means no filter.
// A manga with no chapters is only ever "all": calling an empty series finished
// or unread would be a claim the data cannot support.
func filterLibrary(mangas []database.Manga, stats map[string]database.LibraryMangaStats, categories map[string][]string, status, tag string) []database.Manga {
	keep := make([]database.Manga, 0, len(mangas))
	for _, m := range mangas {
		read, total := readState(m, stats)

		switch status {
		case "reading":
			if read == 0 || total == 0 || read >= total {
				continue
			}
		case "unread":
			if read != 0 {
				continue
			}
		case "done":
			if total == 0 || read < total {
				continue
			}
		}

		if tag != "" && !hasCategory(categories, m, tag) {
			continue
		}
		keep = append(keep, m)
	}
	return keep
}

// hasCategory matches a tag case-insensitively, since the tag comes from a URL
// the user may have typed or picked from a different casing than the stored one.
func hasCategory(categories map[string][]string, m database.Manga, tag string) bool {
	want := strings.ToLower(strings.TrimSpace(tag))
	for _, c := range categories[strconv.FormatInt(m.ID, 10)] {
		if strings.ToLower(c) == want {
			return true
		}
	}
	return false
}
