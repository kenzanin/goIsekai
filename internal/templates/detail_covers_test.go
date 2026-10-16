package templates

import (
	"strings"
	"testing"

	"goisekai/internal/database"
)

// The Clear Covers / Refresh Covers buttons live in the cover picker modal.
// Their form actions must carry both the pluginID and the mangaID segments
// that the /action/{clear,fetch}-covers/{pluginID}/{mangaID} routes register;
// omitting the mangaID used to 404 (error_cover.har). These pin both URLs.

func coversData() map[string]any {
	return map[string]any{
		"PluginID":     "atsumaru",
		"PluginName":   "Atsumaru",
		"MangaID":      "puUQg",
		"CurrentTitle": "Test Manga",
		"Manga":        map[string]any{"Title": "Test Manga", "CoverURL": "https://example.com/cover.jpg"},
		"Chapters":     []any{},
		"Progress":     map[string]any{},
		"Categories":   []database.EnrichmentRow{},
		"Related":      []database.EnrichmentRow{},
	}
}

func TestCoversModalFormsCarryMangaID(t *testing.T) {
	e := mustEngine(t)
	out := renderPartial(t, e, "views/detail", coversData())
	for _, want := range []string{
		`action="/action/clear-covers/atsumaru/puUQg"`,
		`action="/action/fetch-covers/atsumaru/puUQg"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("views/detail missing %q form action", want)
		}
	}
	// The buggy variant dropped the /{mangaID} segment; make sure it's not
	// back.
	if strings.Contains(out, `action="/action/clear-covers/atsumaru"`) ||
		strings.Contains(out, `action="/action/fetch-covers/atsumaru"`) {
		t.Error("a covers form action is missing its /{mangaID} segment")
	}
}
