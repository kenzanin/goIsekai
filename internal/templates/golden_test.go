package templates

import (
	"flag"
	"hash/fnv"
	"os"
	"strings"
	"testing"
)

// Golden-render harness for the Lua template splits.
//
// A view is split by moving blocks of its return-concatenation into partials
// that the view requires. That is safe only if the split is a pure move: same
// blocks, same order, same data keys. The golden string is the view's full
// byte output BEFORE the split; after it, RenderAndGolden fails on the first
// byte that drifted, which locates the broken move.
//
// Goldens live on disk (the golden/ dir next to this file), not inline, so a
// re-baseline is a copy, not a paste: `go test -update` rewrites them from the
// current render and the diff shows exactly what changed. A green test without
// -update means the split produced byte-identical output.
const goldenDir = "golden"

// goldenName derives the golden file for (kind, name): "view:views/detail" ->
// golden/detail.view.lua.golden. The kind prefix keeps a view and a partial
// of the same base name from colliding.
func goldenName(kind, name string) string {
	base := strings.TrimPrefix(name, "views/")
	base = strings.TrimPrefix(base, "partials/")
	base = strings.TrimSuffix(base, ".lua")
	// Two views can share a base (detail view vs detail_chapters partial do
	// not, but the view-vs-partial direction would collide on "detail"), so
	// prefix the kind.
	return goldenDir + "/" + kind + "." + base + ".golden"
}

// goldenFixture returns a data map rich enough to make most template branches
// take the non-empty path. Empty values only exercise the empty-state markup,
// which every split keeps verbatim anyway, so they cannot mask a broken move.
func goldenFixture(name string) map[string]any {
	manga := map[string]any{
		"ID": "s1", "Title": "Golden Manga", "CoverURL": "https://cdn.test/c.jpg",
		"Description": "A description long enough to render the synopsis block.",
		"Status": "ongoing", "Author": "Some Author",
		"Genres": []any{"Action", "Comedy", "Drama"},
	}
	chapters := []any{
		map[string]any{"ID": "c1", "SourceChapterID": "c1", "Title": "Ch 1", "ChapterNum": 1.0},
		map[string]any{"ID": "c2", "SourceChapterID": "c2", "Title": "Ch 2", "ChapterNum": 2.0},
	}
	cards := []any{
		map[string]any{"ID": 1, "Title": "One", "CoverURL": "https://cdn.test/1.jpg",
			"PluginID": "p", "SourceMangaID": "s", "InLibrary": true},
		map[string]any{"ID": 2, "Title": "Two", "CoverURL": "https://cdn.test/2.jpg",
			"PluginID": "p", "SourceMangaID": "t"},
	}
	plugins := []any{
		map[string]any{"ID": "p", "Name": "Plugin P", "Version": "1", "IsActive": true},
	}

	switch {
	case strings.Contains(name, "detail_alt"):
		return map[string]any{
			"PluginID": "md", "MangaID": "s1", "CurrentTitle": "Golden Manga",
			"AltTitles":     []any{map[string]any{"Title": "Alt One", "Source": "MangaDex"}},
			"AltSummaries":  []any{map[string]any{"Description": "Alt synopsis.", "Source": "MU"}},
			"Categories":    []any{map[string]any{"Value": "Action", "Source": "plugin"}},
			"PluginGenres":  []any{"Action"},
		}
	case strings.Contains(name, "detail_chapters"):
		return map[string]any{
			"PluginID": "md", "MangaID": "s1", "Chapters": chapters,
			"Progress": map[string]any{"c1": map[string]any{"LastPageRead": 3}},
			"ChCurrentPage": 1, "ChTotalPages": 1,
		}
	case strings.Contains(name, "detail"):
		return map[string]any{
			"PluginID": "md", "PluginName": "MangaDex", "PluginIcon": "",
			"MangaID": "s1", "Manga": manga, "CurrentTitle": "Golden Manga",
			"AltTitles":  []any{map[string]any{"Title": "Alt One", "Source": "MangaDex"}},
			"AltSummaries": []any{map[string]any{"Description": "Alt synopsis."}},
			"Chapters": chapters, "Progress": map[string]any{"c1": map[string]any{"LastPageRead": 3}},
			"Continue": map[string]any{"ChapterID": "c1"}, "InLibrary": true,
			"Categories": []any{map[string]any{"Value": "Action"}},
			"Related":    []any{map[string]any{"Value": "Other", "URL": "https://x"}},
			"AltCovers":  []any{map[string]any{"URL": "https://cdn.test/alt.jpg"}},
			"ChCurrentPage": 1, "ChTotalPages": 1,
		}
	case strings.Contains(name, "library_sidebar"):
		return map[string]any{
			"Mangas": cards, "Stats": map[string]any{"TotalTitles": 2},
			"CategoryCounts": map[string]any{"Action": 3},
			"PluginCounts": []any{map[string]any{"PluginID": "p", "Name": "Plugin P", "Count": 2}},
			"DuplicateCount": 0,
		}
	case strings.Contains(name, "plugin_cards"):
		return map[string]any{
			"Plugins": plugins, "Mangas": cards,
		}
	case strings.Contains(name, "search"):
		return map[string]any{
			"Q": "golden", "Plugins": plugins, "Results": cards, "Mangas": cards,
			"Page": 1, "HasNext": false, "HasPrev": false, "TotalPages": 1,
			"CategoryCounts": map[string]any{"Action": 3},
			"Selected":       map[string]any{"p": true},
		}
	case strings.Contains(name, "logs"):
		return map[string]any{
			"Lines": []any{"line one", "line two"}, "Level": "debug",
			"Plugins": plugins, "Q": "",
		}
	case strings.Contains(name, "migrate"):
		return map[string]any{
			"PluginID": "md", "MangaID": "s1", "Manga": manga,
			"Plugins": plugins, "Q": "golden", "SourceChapterCount": 2,
			"Candidates": []any{map[string]any{
				"PluginID": "p", "PluginName": "Plugin P", "SourceMangaID": "s",
				"Title": "One", "CoverURL": "https://cdn.test/1.jpg",
				"IsExactMatch": true, "ChapterCount": 2, "SourceChapterCount": 2, "Fewer": false,
			}},
			"Page": 1, "HasNext": false, "Failures": []any{}, "SearchError": "",
		}
	default: // library and anything else
		return map[string]any{
			"Mangas": cards, "Plugins": plugins, "Q": "",
			"Stats": map[string]any{"TotalTitles": 2}, "Page": 1, "TotalPages": 1,
			"HasNext": false, "HasPrev": false, "Sort": "updated", "Status": "all",
			"CategoryCounts": map[string]any{"Action": 3},
			"DupColors": map[string]any{} , "DupRank": map[string]any{},
			"DuplicateCount": 0, "Tag": "", "View": "",
		}
	}
}

// RenderAndGolden renders view/partial `name` with goldenFixture and compares
// byte-for-byte against the golden file. With -update it writes the file
// instead, so the operator can eyeball the diff.
//
// The name is passed the way the Go side renders it ("views/detail",
// "partials/cover_modal"): views use Render, partials use RenderPartial.
func RenderAndGolden(t *testing.T, e *Engine, kind, name string) {
	t.Helper()
	data := goldenFixture(name)
	var buf strings.Builder
	if kind == "partial" {
		if err := e.RenderPartial(&buf, name, data); err != nil {
			t.Fatalf("RenderPartial %s: %v", name, err)
		}
	} else {
		if err := e.Render(&buf, name, data); err != nil {
			t.Fatalf("Render %s: %v", name, err)
		}
	}
	out := buf.String()

	path := goldenName(kind, name)
	if *goldenUpdate {
		if err := os.MkdirAll(goldenDir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", goldenDir, err)
		}
		if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
			t.Fatalf("write golden %s: %v", path, err)
		}
		t.Logf("wrote %s (%d bytes)", path, len(out))
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden %s missing — run with -update to capture it: %v", path, err)
	}
	if out != string(want) {
		gotHash, wantHash := fnvHash(out), fnvHash(string(want))
		diffAt, diffCtx := firstByteDiff(out, string(want))
		t.Errorf("%s render drifted from golden (fnv1a got=%08x want=%08x, first diff at byte %d):\n...got:  %q\n...want: %q\nre-baseline with: go test -update ./internal/templates",
			name, gotHash, wantHash, diffAt, diffCtx, diffCtx)
	}
}

// fnvHash gives both sides of a diff a cheap identity so a log line is
// searchable in a 200 KB golden without printing it.
func fnvHash(s string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return h.Sum32()
}

// firstByteDiff reports where two strings diverge plus ~30 bytes around it,
// which is how a move that reorders one tag shows up.
func firstByteDiff(a, b string) (int, string) {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			lo, hi := i-30, i+30
			if lo < 0 {
				lo = 0
			}
			if hi > len(a) {
				hi = len(a)
			}
			return i, "[" + a[lo:hi] + "]"
		}
	}
	if len(a) != len(b) {
		lo := len(a) - 30
		if lo < 0 {
			lo = 0
		}
		return len(a), "tail [" + a[lo:] + "]"
	}
	return -1, ""
}

// goldenUpdate is set by the -update flag so RenderAndGolden re-baselines
// instead of asserting. Custom Go test flags live on the test binary's flag
// set, not os.Args, so register one.
var goldenUpdate = flag.Bool("update", false, "re-baseline the golden template renders instead of asserting against them")



// goldenTargets is the set of files the split touches. Each entry is rendered
// before and after its split; a green run means the move was byte-identical.
func goldenTargets() []struct{ kind, name string } {
	return []struct{ kind, name string }{
		{"view", "views/detail"},
		{"view", "views/search"},
		{"view", "views/logs"},
		{"view", "views/library"},
		{"view", "views/migrate"},
		{"partial", "partials/detail_chapters"},
		{"partial", "partials/library_sidebar"},
		{"partial", "partials/plugin_cards"},
		{"partial", "partials/detail_alt"},
	}
}

// TestGoldenTemplateRenders pins the rendered bytes of every file the split
// touches. Run `go test ./internal/templates -run TestGoldenTemplateRenders
// -update` to (re)capture; a plain run asserts.
func TestGoldenTemplateRenders(t *testing.T) {
	e := mustEngine(t)
	for _, tc := range goldenTargets() {
		t.Run(strings.ReplaceAll(tc.name, "/", "_"), func(t *testing.T) {
			RenderAndGolden(t, e, tc.kind, tc.name)
		})
	}
}
