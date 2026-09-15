package templates

import (
	"strings"
	"testing"

	"goisekai/internal/database"
)

// The "Related / Recommended" section renders in the detail column and only
// there. The enrichment panel used to repeat the same list from the same
// data.Related, so every entry appeared twice on an opened page. Rendering
// nothing when the list is empty is silent, so these tests pin both the
// placement and the de-duplication.

func relatedRows() []database.EnrichmentRow {
	return []database.EnrichmentRow{
		{Value: "Other Manga", URL: "https://example.com/other", Source: "mangaupdates"},
		{Value: "Second Pick", Source: "mangaupdates"},
	}
}

func detailData() map[string]any {
	return map[string]any{
		"PluginID":     "demo",
		"PluginName":   "Demo",
		"MangaID":      "m1",
		"CurrentTitle": "Main Title",
		"Manga":        map[string]any{"Title": "Main Title"},
		"Chapters":     []any{},
		"Progress":     map[string]any{},
		"Categories":   []database.EnrichmentRow{{Value: "Action", Source: "mangaupdates"}},
		"Related":      relatedRows(),
	}
}

func TestRelatedSectionRendersInDetailColumnOnly(t *testing.T) {
	e := mustEngine(t)
	out := renderPartial(t, e, "views/detail", detailData())
	if !strings.Contains(out, "Related / Recommended") {
		t.Fatal("views/detail dropped the Related / Recommended heading")
	}
	if n := strings.Count(out, `action="/action/remove-related/demo/m1"`); n != len(relatedRows()) {
		t.Errorf("views/detail rendered %d related chips, want %d", n, len(relatedRows()))
	}
	if !strings.Contains(out, `href="https://example.com/other"`) {
		t.Error("views/detail did not link a related entry to its source page")
	}

	panel := renderPartial(t, e, "partials/detail_alt", detailData())
	if strings.Contains(panel, "Related / Recommended") {
		t.Error("the enrichment panel repeats the related list the detail column already shows")
	}
}

// TestRelatedSectionHiddenWhenEmpty: with no rows the heading must not appear,
// so an empty enrichment fetch does not leave a bare title behind.
func TestRelatedSectionHiddenWhenEmpty(t *testing.T) {
	e := mustEngine(t)
	data := detailData()
	data["Related"] = []database.EnrichmentRow{}
	out := renderPartial(t, e, "views/detail", data)
	if strings.Contains(out, "Related / Recommended") {
		t.Error("rendered the Related / Recommended heading with no related manga")
	}
}

// TestRelatedDuplicatesCollapse: two rows carrying the same title are one
// chip, whatever mix of sources produced them.
func TestRelatedDuplicatesCollapse(t *testing.T) {
	e := mustEngine(t)
	data := detailData()
	data["Related"] = []database.EnrichmentRow{
		{Value: "Same Title", URL: "https://example.com/a", Source: "mangadex"},
		{Value: "Same Title", URL: "https://example.com/b", Source: "mangaupdates"},
		{Value: "", Source: "mangadex"},
	}
	out := renderPartial(t, e, "views/detail", data)
	if n := strings.Count(out, `action="/action/remove-related/demo/m1"`); n != 1 {
		t.Errorf("three related rows rendered %d chips, want 1", n)
	}
	if strings.Count(out, `href="https://example.com/a"`) != 1 {
		t.Error("the first row's link should be the one kept")
	}
}

// TestRelatedLinksAreEscaped: related titles and URLs come from a third-party
// site, so the tag link must not be able to inject markup.
func TestRelatedLinksAreEscaped(t *testing.T) {
	e := mustEngine(t)
	data := detailData()
	data["Related"] = []database.EnrichmentRow{
		{Value: `<script>alert(1)</script>`, URL: `javascript:alert(1)"`},
	}
	out := renderPartial(t, e, "views/detail", data)
	if strings.Contains(out, "<script>alert(1)</script>") {
		t.Error("related title rendered as live markup")
	}
	if !strings.Contains(out, "&lt;script&gt;") {
		t.Error("related title was not HTML-escaped")
	}
}
