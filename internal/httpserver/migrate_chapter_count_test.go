package httpserver

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"goisekai/internal/database"
	"goisekai/internal/templates"
	"goisekai/pkg/types"
)

// Migrating replaces the entry's chapter list with the target's and drops the
// surplus, unrecoverably. Picking a target without seeing its chapter count is
// how an 8-chapter stub silently replaces a 55-chapter entry, so the count has
// to be on the card, and the shortfall has to be stated on the card.
func renderMigratePage(t *testing.T, srcCount int, cands []any) string {
	t.Helper()
	engine, err := templates.New(os.DirFS("../templates"), false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	var buf bytes.Buffer
	data := map[string]any{
		"PluginID":           "kaliscan",
		"MangaID":            "m1",
		"Manga":              types.Manga{ID: "m1", Title: "Ascendance of a Bookworm"},
		"Plugins":            []database.Plugin{{ID: "kaliscan", Name: "Kaliscan", IsActive: true}},
		"Q":                  "Ascendance",
		"SourceChapterCount": srcCount,
		"Candidates":         cands,
		"Page":               1,
		"HasNext":            false,
	}
	if err := engine.Render(&buf, "views/migrate", data); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

func candidate(pluginID, id, title string, count int) any {
	return map[string]any{
		"PluginID": pluginID, "PluginName": pluginID, "SourceMangaID": id,
		"Title": title, "CoverURL": "", "IsExactMatch": true,
		"ChapterCount": count, "SourceChapterCount": 55, "Fewer": count >= 0 && count < 55,
	}
}

func TestMigrateCardWarnsWhenTheTargetHasFewerChapters(t *testing.T) {
	html := renderMigratePage(t, 55, []any{candidate("mangafire", "ascendance", "Ascendance of a Bookworm", 8)})

	if !strings.Contains(html, "55 chapters in this entry") {
		t.Error("the source card does not show the count being compared against")
	}
	if !strings.Contains(html, "8 / 55") {
		t.Errorf("a shorter target is not flagged:\n%s", excerpt(html, "ascendance"))
	}
	if !strings.Contains(html, "the surplus is lost") {
		t.Error("the shortfall does not say what happens to the missing chapters")
	}
}

func TestMigrateCardDoesNotWarnWhenTheTargetCoversMore(t *testing.T) {
	html := renderMigratePage(t, 55, []any{candidate("mangafire", "ascendance", "Ascendance of a Bookworm", 120)})

	if strings.Contains(html, "the surplus is lost") {
		t.Error("a longer target was warned about a surplus it does not have")
	}
	if !strings.Contains(html, "120 chapters") {
		t.Error("the target card does not show its chapter count")
	}
}

func TestMigrateCardSaysUnknownRatherThanZero(t *testing.T) {
	// -1 is "the source could not be reached". Rendering that as 0 chapters would
	// read as an empty series and look like the worst possible target.
	html := renderMigratePage(t, 55, []any{candidate("fanfox", "x", "Ascendance of a Bookworm", -1)})

	if !strings.Contains(html, "chapter count unavailable") {
		t.Errorf("an unreachable source is not reported as unknown:\n%s", excerpt(html, "Ascendance of a Bookworm"))
	}
	if strings.Contains(html, "0 / 55") {
		t.Error("an unknown count rendered as zero chapters")
	}
}

func excerpt(html, needle string) string {
	i := strings.Index(html, needle)
	if i < 0 {
		return "(card not found)"
	}
	lo := max(0, i-200)
	hi := min(len(html), i+400)
	return html[lo:hi]
}
