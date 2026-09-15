package templates

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"testing/fstest"

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

// TestFormatHelpers covers the template-visible formatters through the Lua
// bindings the templates actually call, not a Go-side duplicate.
func TestFormatHelpers(t *testing.T) {
	tmplFS := fstest.MapFS{
		"views/fmt.lua": &fstest.MapFile{
			Data: []byte(`return function(data)
				return table.concat({
					formatDate(""),
					formatDate("2024-01-02T15:04:05Z"),
					formatChapterNum(nil),
					formatChapterNum(5.0),
					formatChapterNum(5.5),
				}, "|")
			end`),
		},
	}
	engine, err := NewLuaEngine(tmplFS, false)
	if err != nil {
		t.Fatalf("NewLuaEngine: %v", err)
	}
	var buf bytes.Buffer
	if err := engine.Render(&buf, "views/fmt", nil); err != nil {
		t.Fatalf("Render: %v", err)
	}
	const want = "\u2014|Jan 2, 2024|\u2014|5|5.5"
	if got := strings.TrimSpace(buf.String()); got != want {
		t.Errorf("formatters = %q, want %q", got, want)
	}
}
