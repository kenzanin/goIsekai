package templates

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"goisekai/internal/database"
)

func mustEngine(t *testing.T) *Engine {
	t.Helper()
	e, err := New(os.DirFS("."), false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return e
}

func TestRenderHome(t *testing.T) {
	e := mustEngine(t)
	var buf bytes.Buffer
	if err := e.Render(&buf, "views/library", map[string]any{"Mangas": []database.Manga{}}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "<!DOCTYPE html>") {
		t.Errorf("missing DOCTYPE")
	}
	if !strings.Contains(out, "goIsekai") {
		t.Errorf("missing title goIsekai")
	}
	if !strings.Contains(out, "Your library is empty") {
		t.Errorf("missing empty-library state")
	}
	if !strings.Contains(out, `href="/view/search"`) {
		t.Errorf("nav missing Search link")
	}
}

func TestRenderActiveTab(t *testing.T) {
	e := mustEngine(t)
	var buf bytes.Buffer
	if err := e.Render(&buf, "views/library", map[string]any{"active": "search", "Mangas": []database.Manga{}}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `href="/view/search"`) || !strings.Contains(out, "bg-indigo-500") {
		t.Errorf("expected Search tab highlighted, got:\n%s", out)
	}
}

func TestRenderEnrichmentNoNestedForms(t *testing.T) {
	e := mustEngine(t)
	data := map[string]any{
		"PluginID":     "demo",
		"MangaID":      "m1",
		"CurrentTitle": "Main Title",
		"AltTitles": []map[string]any{
			{"Title": "Alt One", "Source": "MangaDex"},
			{"Title": "Alt Two", "Source": ""},
		},
		"AltSummaries": []map[string]any{
			{"Description": "Some synopsis", "Source": "MangaUpdates"},
		},
		"Categories": []map[string]any{
			{"Value": "Action"},
			{"Value": "Comedy"},
		},
		"Related": []map[string]any{
			{"Value": "Other Manga"},
		},
		"Genres": []string{"Action"},
	}
	if got := formNestingDepth(`<form><form></form></form>`); got != 2 {
		t.Fatalf("formNestingDepth self-check = %d, want 2", got)
	}
	out := renderPartial(t, e, "partials/detail_alt", data)
	if got := formNestingDepth(out); got > 1 {
		t.Fatalf("nested <form> depth = %d, want <= 1\n%s", got, out)
	}
}

func renderPartial(t *testing.T, e *Engine, name string, data map[string]any) string {
	t.Helper()
	var buf bytes.Buffer
	if err := e.RenderPartial(&buf, name, data); err != nil {
		t.Fatalf("RenderPartial %s: %v", name, err)
	}
	return buf.String()
}

// formNestingDepth returns the maximum simultaneous open <form> elements.
func formNestingDepth(html string) int {
	depth, max := 0, 0
	for {
		open := strings.Index(html, "<form")
		close := strings.Index(html, "</form")
		switch {
		case open == -1 && close == -1:
			return max
		case close == -1 || (open != -1 && open < close):
			depth++
			if depth > max {
				max = depth
			}
			html = html[open+len("<form"):]
		default:
			depth--
			html = html[close+len("</form"):]
		}
	}
}

func TestHelpers(t *testing.T) {
	if got := formatDate(""); got != "—" {
		t.Errorf(`formatDate("") = %q, want —`, got)
	}
	if got := formatDate("2024-01-02T15:04:05Z"); got != "Jan 2, 2024" {
		t.Errorf("formatDate = %q, want Jan 2, 2024", got)
	}
	if got := formatChapterNum(nil); got != "—" {
		t.Errorf("formatChapterNum(nil) = %q, want —", got)
	}
	if got := formatChapterNum(5.0); got != "5" {
		t.Errorf("formatChapterNum(5.0) = %q, want 5", got)
	}
	if got := formatChapterNum(5.5); got != "5.5" {
		t.Errorf("formatChapterNum(5.5) = %q, want 5.5", got)
	}

}
