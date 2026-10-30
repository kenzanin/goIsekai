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
var librarySorts = []string{"updated", "title", "read", "progress", "recent_read", "recent_added", "duplicates"}

// libraryStatuses are the accepted values of ?status=, which answers "which of
// these did I actually read" - the question a source migration starts from.
// "new" is special: it filters to manga with an unread [New] badge
// (HasNew from sync), not a reading-progress state.
var libraryStatuses = []string{"all", "reading", "unread", "done", "new"}

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
// so an unknown sort key leaves the caller's ordering untouched. dupRank maps a
// manga row-ID to its duplicate-group position; only the "duplicates" sort reads
// it, and it may be nil when callers have no duplicate data.
func sortLibrary(mangas []database.Manga, stats map[string]database.LibraryMangaStats, key string, dupRank map[string]int) {
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
	case "recent_added":
		// Most recently added to the library first. Manga never added
		// (zero AddedAt) sinks to the bottom, insertion order kept via
		// the stable sort.
		sort.SliceStable(mangas, func(i, j int) bool {
			ai, aj := mangas[i].AddedAt, mangas[j].AddedAt
			if ai.IsZero() && aj.IsZero() {
				return false
			}
			if ai.IsZero() {
				return false
			}
			if aj.IsZero() {
				return true
			}
			return ai.After(aj)
		})
	case "recent_read":
		// Most recently read first. Never-read sinks to the bottom (nil
		// LastReadAt sorts after any timestamp), keeping insertion order
		// among unread via the stable sort.
		sort.SliceStable(mangas, func(i, j int) bool {
			si := stats[strconv.FormatInt(mangas[i].ID, 10)].LastReadAt
			sj := stats[strconv.FormatInt(mangas[j].ID, 10)].LastReadAt
			if si == nil && sj == nil {
				return false
			}
			if si == nil {
				return false
			}
			if sj == nil {
				return true
			}
			return si.After(*sj)
		})
	case "duplicates":
		// Group members first, each group's cards adjacent, so a colour band
		// reads as one block instead of being split across pages. Manga with no
		// duplicate group sink to the bottom in their existing order.
		sort.SliceStable(mangas, func(i, j int) bool {
			ri, iDup := dupRank[strconv.FormatInt(mangas[i].ID, 10)]
			rj, jDup := dupRank[strconv.FormatInt(mangas[j].ID, 10)]
			if iDup != jDup {
				return iDup
			}
			return ri < rj
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
// "new" keeps only manga with the [New] badge (HasNew from a recent sync).
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
		case "new":
			key := strconv.FormatInt(m.ID, 10)
			if !stats[key].HasNew {
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
