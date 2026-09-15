package templates

import (
	"strings"
	"testing"

	"goisekai/internal/database"
)

// The enrichment panel is pure server-rendered markup: a chip does nothing
// unless its form posts to the right /action endpoint with the right hidden
// field. Nothing else in the stack checks that pairing — the JS handler just
// submits "the closest form". These tests pin the markup contract that the
// action handlers on the other side rely on.

// enrichmentData is a realistic panel payload, reusing the exact row types
// buildMangaDetailData passes in.
func enrichmentData() map[string]any {
	return map[string]any{
		"PluginID":     "demo",
		"MangaID":      "m1",
		"CurrentTitle": "Main Title",
		"AltTitles": []database.AltTitleRow{
			{Title: "Alt One", Source: "MangaDex"},
			{Title: "Alt Two", Source: ""},
		},
		"AltSummaries": []database.AltDescriptionRow{
			{Description: "An alternative synopsis.", Source: "MangaUpdates"},
		},
		"Categories": []database.EnrichmentRow{
			{Value: "Action", Source: "plugin"},
			{Value: "Shounen", Source: "plugin"},
		},
		"PluginGenres": []string{"Action"},
	}
}

func renderEnrichment(t *testing.T, data map[string]any) string {
	t.Helper()
	return renderPartial(t, mustEngine(t), "partials/detail_alt", data)
}

// TestDetailAltChipFormsTargetActions is the whole contract in one table: for
// each chip, the endpoint it posts to and the hidden field carrying its value.
func TestDetailAltChipFormsTargetActions(t *testing.T) {
	out := renderEnrichment(t, enrichmentData())
	cases := []struct {
		name   string
		action string
		field  string
		value  string
	}{
		{"alt title", "/action/set-title/demo/m1", "title", "Alt One"},
		{"alt title server", "/action/set-title/demo/m1", "title", "Alt Two"},
		{"remove alt title", "/action/remove-alt-title/demo/m1", "title", "Alt One"},
		{"alt synopsis", "/action/set-summary/demo/m1", "description", "An alternative synopsis."},
		{"remove alt synopsis", "/action/remove-alt-summary/demo/m1", "description", "An alternative synopsis."},
		{"add category", "/action/add-category/demo/m1", "category", "Shounen"},
		{"remove category", "/action/remove-category/demo/m1", "category", "Action"},
		{"fetch", "/action/fetch-enrichment/demo/m1", "manga_title", "Main Title"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			forms := formsPosting(t, out, tc.action)
			for _, form := range forms {
				if strings.Contains(form, `name="`+tc.field+`" value="`+tc.value+`"`) {
					return
				}
			}
			t.Fatalf("no form posting to %s carries %s=%q:\n%s", tc.action, tc.field, tc.value, strings.Join(forms, "\n"))
		})
	}
}

// formsPosting returns every <form ...> ... </form> block whose action matches.
func formsPosting(t *testing.T, html, action string) []string {
	t.Helper()
	var out []string
	marker := `action="` + action + `"`
	for pos := 0; ; {
		i := strings.Index(html[pos:], marker)
		if i < 0 {
			return out
		}
		i += pos
		start := strings.LastIndex(html[:i], "<form")
		end := strings.Index(html[i:], "</form>")
		if start < 0 || end < 0 {
			t.Fatalf("malformed form markup for %s", action)
		}
		out = append(out, html[start:i+end+len("</form>")])
		pos = i + end + len("</form>")
	}
}

// TestDetailAltCategoryTogglesDirection guards the one place the panel branch:
// a category already in the plugin's raw genres must offer removal, a new one
// must offer addition. Reversing it would make every chip a no-op.
func TestDetailAltCategoryTogglesDirection(t *testing.T) {
	out := renderEnrichment(t, enrichmentData())
	if !strings.Contains(out, `action="/action/remove-category/demo/m1"`) {
		t.Error("active category (in PluginGenres) should offer /action/remove-category/")
	}
	if !strings.Contains(out, `action="/action/add-category/demo/m1"`) {
		t.Error("non-active category should offer /action/add-category/")
	}
	if !strings.Contains(formsPosting(t, out, "/action/remove-category/demo/m1")[0], `value="Action"`) {
		t.Error("remove-category form is not the Action chip")
	}
	if !strings.Contains(formsPosting(t, out, "/action/add-category/demo/m1")[0], `value="Shounen"`) {
		t.Error("add-category form is not the Shounen chip")
	}
}

// TestDetailAltChipsAreClickable pins the handler that turns a chip into a
// submit. Without it the chip is inert markup.
func TestDetailAltChipsAreClickable(t *testing.T) {
	out := renderEnrichment(t, enrichmentData())
	form := formsPosting(t, out, "/action/set-title/demo/m1")[0]
	if !strings.Contains(form, "submitForm(") {
		t.Errorf("alt title chip has no submitForm() handler:\n%s", form)
	}
	if n := strings.Count(out, "submitForm("); n < 4 {
		t.Errorf("only %d chips are clickable, want >= 4 (titles, synopses, categories)", n)
	}
}

// TestDetailAltEscapesUntrustedValues: alt titles and synopses come from third
// party sites, so they land in value="..." attributes and text nodes. A title
// containing a quote must not be able to break out of the attribute.
func TestDetailAltEscapesUntrustedValues(t *testing.T) {
	nasty := `He said "hi" <script>alert(1)</script>`
	data := enrichmentData()
	data["AltTitles"] = []database.AltTitleRow{{Title: nasty, Source: "MangaDex"}}
	data["AltSummaries"] = []database.AltDescriptionRow{{Description: nasty}}
	data["Categories"] = []database.EnrichmentRow{{Value: nasty}}

	out := renderEnrichment(t, data)
	if strings.Contains(out, "<script>") {
		t.Errorf("a title was rendered as live markup:\n%s", out)
	}
	if !strings.Contains(out, "&lt;script&gt;") {
		t.Errorf("a title was not HTML-escaped:\n%s", out)
	}
	if strings.Contains(out, `value="He said "hi"`) {
		t.Errorf("a quote broke out of a value attribute:\n%s", out)
	}
}

// TestDetailAltOmitsEmptySections: with no enrichment rows the panel must still
// render the Fetch/Reset controls and nothing else clickable.
func TestDetailAltOmitsEmptySections(t *testing.T) {
	out := renderEnrichment(t, map[string]any{
		"PluginID":     "demo",
		"MangaID":      "m1",
		"CurrentTitle": "Only Title",
	})
	if !strings.Contains(out, `action="/action/fetch-enrichment/demo/m1"`) {
		t.Error("Fetch Details form missing when there is nothing to show yet")
	}
	if !strings.Contains(out, `action="/action/reset-enrichment/demo/m1"`) {
		t.Error("Reset form missing")
	}
	for _, gone := range []string{"set-title", "set-summary", "add-category", "remove-category"} {
		if strings.Contains(out, "/action/"+gone+"/") {
			t.Errorf("empty panel rendered an %s form", gone)
		}
	}
}

// TestDetailAltFetchCarriesCurrentTitle: FetchDetails falls back to searching
// the stored title when the posted one is blank, so the hidden field must
// always hold the title the user is looking at.
func TestDetailAltFetchCarriesCurrentTitle(t *testing.T) {
	data := enrichmentData()
	data["CurrentTitle"] = `Weird "Title" & Co`
	out := renderEnrichment(t, data)
	form := formsPosting(t, out, "/action/fetch-enrichment/demo/m1")[0]
	if !strings.Contains(form, `name="manga_title" value="Weird &#34;Title&#34; &amp; Co"`) {
		t.Errorf("fetch form does not carry the escaped current title:\n%s", form)
	}
}
