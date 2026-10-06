package httpserver

import (
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"

	"goisekai/internal/database"
	"goisekai/internal/templates"
)

// The library toolbar changes one filter at a time while keeping the others.
// Building those links by appending to a base URL emits the parameter being
// changed a second time - "?status=reading&status=done" - and Go reads the first,
// so the chip looks right and clicking it does nothing. This pins the invariant
// that makes that impossible: a parameter never appears twice.

func renderLibraryPage(t *testing.T, data map[string]any) string {
	t.Helper()
	engine, err := templates.New(os.DirFS("../templates"), false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	var buf strings.Builder
	if err := engine.Render(&buf, "views/library", data); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

func libraryPageData() map[string]any {
	return map[string]any{
		"Mangas":       []database.Manga{},
		"Q":            "isekai",
		"PluginID":     "kaliscan",
		"PluginName":   "Kaliscan",
		"Sort":         "read",
		"Status":       "reading",
		"Tag":          "Fantasy",
		"LibraryStats": map[string]map[string]any{},
		"Categories":   map[string][]string{},
		"CategoryCounts": map[string]int{
			"Fantasy": 123,
			"Isekai":  64,
		},
	}
}

var libraryLinkRe = regexp.MustCompile(`/view/library\?[^"'\s]*`)

func TestLibraryLinksNeverRepeatAParameter(t *testing.T) {
	html := renderLibraryPage(t, libraryPageData())

	links := libraryLinkRe.FindAllString(html, -1)
	if len(links) < 4 {
		t.Fatalf("expected the toolbar links to be present, found %d", len(links))
	}
	for _, raw := range links {
		parsed, err := url.Parse(raw)
		if err != nil {
			t.Errorf("unparseable link %q: %v", raw, err)
			continue
		}
		seen := map[string]int{}
		for key := range parsed.Query() {
			seen[key]++
			if seen[key] > 1 {
				t.Errorf("link repeats %q, so the first value wins and the change is ignored: %s", key, raw)
			}
		}
	}
}

// Every chip has to change the filter it names, so following it from a page
// where that filter is already active must actually change it.
func TestStatusChipsCarryTheStatusTheyName(t *testing.T) {
	html := renderLibraryPage(t, libraryPageData())

	for _, want := range []string{"all", "reading", "unread", "done"} {
		found := false
		for _, raw := range libraryLinkRe.FindAllString(html, -1) {
			parsed, err := url.Parse(raw)
			if err != nil {
				continue
			}
			q := parsed.Query()
			// "all" is the default, so its chip drops the parameter entirely.
			if want == "all" {
				found = q.Get("status") == "" && q.Get("q") == "isekai"
				if found {
					break
				}
				continue
			}
			if q.Get("status") == want {
				found = true
				if q.Get("q") != "isekai" {
					t.Errorf("status=%s chip dropped the search text: %s", want, raw)
				}
				break
			}
		}
		if !found {
			t.Errorf("no chip links to status=%s while one other status is active", want)
		}
	}
}

// A hidden input carrying the same name as a control in its own form makes the
// server read the hidden one and ignore the control the user just used.
func TestLibraryFormsDoNotShadowTheirOwnControls(t *testing.T) {
	html := renderLibraryPage(t, libraryPageData())

	for _, block := range regexp.MustCompile(`(?s)<form[^>]*>(.*?)</form>`).FindAllStringSubmatch(html, -1) {
		form := block[1]
		if !strings.Contains(form, `/view/library`) {
			continue
		}
		hidden := map[string]bool{}
		for _, m := range regexp.MustCompile(`<input type="hidden" name="([a-zA-Z]+)"`).FindAllStringSubmatch(form, -1) {
			hidden[m[1]] = true
		}
		for _, m := range regexp.MustCompile(`<(?:select|input type="(?:search|text)")[^>]*name="([a-zA-Z]+)"`).FindAllStringSubmatch(form, -1) {
			if hidden[m[1]] {
				t.Errorf("form has a hidden %q next to a %q control, so the hidden value wins", m[1], m[1])
			}
		}
	}
}

// The sort select must show the sort in effect, or the control reads as unset
// while the grid is sorted.
func TestSortSelectShowsTheActiveSort(t *testing.T) {
	html := renderLibraryPage(t, libraryPageData())

	block := regexp.MustCompile(`(?s)<select name="sort".*?</select>`).FindString(html)
	if block == "" {
		t.Fatal("no sort select rendered")
	}
	if !regexp.MustCompile(`value="read" selected`).MatchString(block) {
		t.Errorf("sort select does not mark read as selected:\n%s", block)
	}
}
