package templates

import (
	"strings"
	"testing"

	"goisekai/internal/database"
)

// The "Related / Recommended" section is rendered twice from the same
// data.Related: as plain tags in the detail column (views/detail) and as chips
// inside the enrichment panel (partials/detail_alt). Either one silently
// renders nothing when the list is empty, so a regression in one of the two
// places is invisible — no error, no log line. These tests pin both.

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

func TestRelatedSectionRendersInBothPlaces(t *testing.T) {
	e := mustEngine(t)
	for _, name := range []string{"views/detail", "partials/detail_alt"} {
		t.Run(name, func(t *testing.T) {
			out := renderPartial(t, e, name, detailData())
			if !strings.Contains(out, "Related / Recommended") {
				t.Fatalf("%s dropped the Related / Recommended heading", name)
			}
			if !strings.Contains(out, `action="/action/remove-related/demo/m1"`) {
				t.Errorf("%s has no remove-related form", name)
			}
			for _, r := range relatedRows() {
				if !strings.Contains(out, r.Value) {
					t.Errorf("%s is missing related entry %q", name, r.Value)
				}
			}
		})
	}
}

func TestRelatedSectionHiddenWhenEmpty(t *testing.T) {
	e := mustEngine(t)
	for _, name := range []string{"views/detail", "partials/detail_alt"} {
		t.Run(name, func(t *testing.T) {
			data := detailData()
			data["Related"] = []database.EnrichmentRow{}
			out := renderPartial(t, e, name, data)
			if strings.Contains(out, "Related / Recommended") {
				t.Errorf("%s rendered the Related / Recommended heading with no related manga", name)
			}
		})
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
