package templates

import (
	"bytes"
	"strings"
	"testing"

	"goisekai/internal/database"
)

func mustEngine(t *testing.T) *Engine {
	t.Helper()
	e, err := New(false)
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
