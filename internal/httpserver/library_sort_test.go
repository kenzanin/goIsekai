package httpserver

import (
	"strconv"
	"testing"

	"goisekai/internal/database"
)

// The library grid shows one page of 24 at a time, so these run over the whole
// list before slicing. The read/total numbers come from ListLibraryWithProgress,
// whose MangaID is the mangas row-ID - keying the lookup any other way makes
// every entry look unread, which is what the status filter then reports.

func manga(id int64, plugin, title string) database.Manga {
	return database.Manga{ID: id, PluginID: plugin, SourceMangaID: "s" + strconv.Itoa(int(id)), Title: title}
}

func stats(m database.Manga, read, total int) database.LibraryMangaStats {
	return database.LibraryMangaStats{
		MangaID:       strconv.FormatInt(m.ID, 10),
		ReadChapters:  read,
		TotalChapters: total,
	}
}

func titles(mangas []database.Manga) []string {
	out := make([]string, len(mangas))
	for i, m := range mangas {
		out[i] = m.Title
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// A manga with no chapters at all cannot honestly be called finished or unread:
// there is nothing to have read. Counting it as "done" would put a dead series
// at the top of exactly the list a migration is driven from.
func TestStatusFilterExcludesSeriesWithNoChapters(t *testing.T) {
	a := manga(1, "p", "empty")
	b := manga(2, "p", "half")
	c := manga(3, "p", "finished")
	list := []database.Manga{a, b, c}
	st := map[string]database.LibraryMangaStats{
		strconv.FormatInt(b.ID, 10): stats(b, 3, 10),
		strconv.FormatInt(c.ID, 10): stats(c, 10, 10),
	}

	for _, tc := range []struct {
		status string
		want   []string
	}{
		{"all", []string{"empty", "half", "finished"}},
		{"reading", []string{"half"}},
		{"done", []string{"finished"}},
		{"unread", []string{"empty"}},
	} {
		got := filterLibrary(append([]database.Manga(nil), list...), st, nil, tc.status, "")
		if !equal(titles(got), tc.want) {
			t.Errorf("status=%s: got %v, want %v", tc.status, titles(got), tc.want)
		}
	}
}

func TestStatusFilterTreatsFullyReadLastPageAsDone(t *testing.T) {
	// The grid's own rule: a chapter read to its last page counts as read even
	// when is_read was never set. The filter has to use the same definition, or
	// a finished series shows up as "in progress".
	m := manga(1, "p", "finished by paging")
	st := map[string]database.LibraryMangaStats{strconv.FormatInt(m.ID, 10): stats(m, 8, 8)}

	got := filterLibrary([]database.Manga{m}, st, nil, "done", "")
	if len(got) != 1 {
		t.Errorf("a fully read series is not counted as done: %v", titles(got))
	}
}

// "Most read" and "most finished" are different questions: 315 of 328 is a
// nearly-finished long series, 3 of 3 is a short finished one. Progress must
// rank by completion, not by raw count.
func TestSortReadCountsChaptersAndProgressRanksCompletion(t *testing.T) {
	long := manga(1, "p", "long nearly done")
	short := manga(2, "p", "short finished")
	tiny := manga(3, "p", "barely started")
	list := []database.Manga{tiny, long, short}
	st := map[string]database.LibraryMangaStats{
		strconv.FormatInt(long.ID, 10):  stats(long, 315, 328),
		strconv.FormatInt(short.ID, 10): stats(short, 3, 3),
		strconv.FormatInt(tiny.ID, 10):  stats(tiny, 2, 400),
	}

	byRead := append([]database.Manga(nil), list...)
	sortLibrary(byRead, st, "read")
	if got := titles(byRead)[0]; got != "long nearly done" {
		t.Errorf("read sort put %q first, want the 315-chapter entry", got)
	}

	byProgress := append([]database.Manga(nil), list...)
	sortLibrary(byProgress, st, "progress")
	if got := titles(byProgress); !equal(got, []string{"short finished", "long nearly done", "barely started"}) {
		t.Errorf("progress sort: got %v", got)
	}
}

func TestSortTitleIsCaseInsensitive(t *testing.T) {
	b := manga(1, "p", "banana")
	a := manga(2, "p", "Apple")
	sortLibrary([]database.Manga{b, a}, nil, "title")
	if got := titles([]database.Manga{b, a}); got[0] != "banana" {
		t.Errorf("expected insertion order untouched by the caller's slice, got %v", got)
	}

	list := []database.Manga{b, a}
	sortLibrary(list, nil, "title")
	if got := titles(list); !equal(got, []string{"Apple", "banana"}) {
		t.Errorf("title sort: got %v, want Apple before banana", got)
	}
}

// ?sort= is a preference in a URL bar, so an unknown value must fall back to the
// existing order rather than reshuffle the library or 400.
func TestUnknownSortKeepsGivenOrder(t *testing.T) {
	a := manga(1, "p", "a")
	b := manga(2, "p", "b")
	list := []database.Manga{b, a}
	sortLibrary(list, nil, "nonsense")
	if got := titles(list); !equal(got, []string{"b", "a"}) {
		t.Errorf("unknown sort reordered the library: %v", got)
	}
}

func TestTagFilterMatchesCaseInsensitively(t *testing.T) {
	a := manga(1, "p", "isekai one")
	b := manga(2, "p", "fantasy one")
	cats := map[string][]string{
		strconv.FormatInt(a.ID, 10): {"Isekai", "Fantasy"},
		strconv.FormatInt(b.ID, 10): {"Fantasy"},
	}
	list := []database.Manga{a, b}

	got := filterLibrary(append([]database.Manga(nil), list...), nil, cats, "all", "isekai")
	if !equal(titles(got), []string{"isekai one"}) {
		t.Errorf("lowercase tag did not match: %v", titles(got))
	}
	got = filterLibrary(append([]database.Manga(nil), list...), nil, cats, "all", " Fantasy ")
	if !equal(titles(got), []string{"isekai one", "fantasy one"}) {
		t.Errorf("untrimmed tag did not match both: %v", titles(got))
	}
}

// Sorting and filtering compose: a migration looks for one source among
// finished entries, which is both a sort and a filter at once.
func TestSortAndFilterCompose(t *testing.T) {
	a := manga(1, "kaliscan", "done short")
	b := manga(2, "kaliscan", "reading long")
	c := manga(3, "1manga", "done other source")
	st := map[string]database.LibraryMangaStats{
		strconv.FormatInt(a.ID, 10): stats(a, 3, 3),
		strconv.FormatInt(b.ID, 10): stats(b, 315, 328),
		strconv.FormatInt(c.ID, 10): stats(c, 9, 9),
	}
	list := []database.Manga{b, c, a}

	kept := filterLibrary(append([]database.Manga(nil), list...), st, nil, "done", "")
	sortLibrary(kept, st, "read")
	if !equal(titles(kept), []string{"done other source", "done short"}) {
		t.Errorf("got %v", titles(kept))
	}
}
