package bridge

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"goisekai/internal/database"
	"goisekai/internal/enrich"
	"goisekai/internal/hostnet"
)

// The enrichment panel's writes go through these three service methods. Each
// has a rule that is easy to lose in a refactor: SetMainTitle refuses a title
// the page never offered, SetMainSummary refuses an empty one, and
// FetchEnrichment must never search with an empty title (upstream answers that
// with unrelated trending series, which then get stored as this manga's data).

// newEnrichService wires an AppService with a single mock provider.
func newEnrichService(t *testing.T, provider *enrichMockProvider) *AppService {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "enrich.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	reg := enrich.NewRegistry()
	reg.Register(provider)
	return NewAppService(db, nil, hostnet.NewProxy(), "", "", reg)
}

func seedMangaRow(t *testing.T, s *AppService, id, pluginID, sourceID, title, desc string) {
	t.Helper()
	if err := s.db.UpsertManga(database.Manga{
		ID:            id,
		PluginID:      pluginID,
		SourceMangaID: sourceID,
		Title:         title,
		Description:   desc,
	}); err != nil {
		t.Fatalf("upsert manga %s: %v", id, err)
	}
}

func TestFetchEnrichmentBlankTitleUsesStoredTitle(t *testing.T) {
	provider := &enrichMockProvider{
		id: "p1", name: "P1",
		kinds: []enrich.Kind{enrich.KindTitles},
		items: map[enrich.Kind][]enrich.Item{
			enrich.KindTitles: {{Value: "Alt Title", Source: "p1"}},
		},
	}
	s := newEnrichService(t, provider)
	seedMangaRow(t, s, "p1|m1", "p1", "m1", "Stored Title", "")

	for _, blank := range []string{"", "   ", "\t"} {
		provider.titles = nil
		if err := s.FetchEnrichment("p1", "m1", blank, []string{"p1"}); err != nil {
			t.Fatalf("FetchEnrichment(%q): %v", blank, err)
		}
		if len(provider.titles) == 0 {
			t.Fatalf("FetchEnrichment(%q) never queried the provider", blank)
		}
		for _, got := range provider.titles {
			if got != "Stored Title" {
				t.Errorf("FetchEnrichment(%q) searched for %q, want the stored title", blank, got)
			}
		}
	}
}

func TestFetchEnrichmentExplicitTitleWinsOverStored(t *testing.T) {
	provider := &enrichMockProvider{id: "p1", name: "P1", kinds: []enrich.Kind{enrich.KindTitles}}
	s := newEnrichService(t, provider)
	seedMangaRow(t, s, "p1|m1", "p1", "m1", "Stored Title", "")

	if err := s.FetchEnrichment("p1", "m1", "Posted Title", []string{"p1"}); err != nil {
		t.Fatalf("FetchEnrichment: %v", err)
	}
	if len(provider.titles) == 0 || provider.titles[0] != "Posted Title" {
		t.Errorf("searched for %v, want the posted title", provider.titles)
	}
}

func TestFetchEnrichmentWithoutAnyTitleErrors(t *testing.T) {
	provider := &enrichMockProvider{id: "p1", name: "P1", kinds: []enrich.Kind{enrich.KindTitles}}
	s := newEnrichService(t, provider)
	seedMangaRow(t, s, "p1|m1", "p1", "m1", "", "")

	if err := s.FetchEnrichment("p1", "m1", "", []string{"p1"}); err == nil {
		t.Fatal("expected an error when neither the form nor the DB has a title")
	}
	if len(provider.titles) != 0 {
		t.Errorf("provider was queried with %v despite having no title", provider.titles)
	}
}

func TestSetMainTitleUnknownTitleErrorNamesIt(t *testing.T) {
	s := newTestService(t)
	seedMangaRow(t, s, "p2|s2", "p2", "s2", "Original", "")
	if _, err := s.db.AddAltTitles("p2|s2", []string{"Known Alt"}, "src"); err != nil {
		t.Fatalf("add alt: %v", err)
	}

	err := s.SetMainTitle("p2", "s2", "Nonexistent")
	if err == nil {
		t.Fatal("expected an error for an unknown title")
	}
	if !strings.Contains(err.Error(), `"Nonexistent"`) {
		t.Errorf("error %q should name the rejected title", err)
	}
	got, err2 := s.db.MangaTitle("p2", "s2")
	if err2 != nil {
		t.Fatalf("MangaTitle: %v", err2)
	}
	if got != "Original" {
		t.Errorf("title = %q, want the original untouched", got)
	}
}

func TestSetMainTitleRejectsBlank(t *testing.T) {
	s := newTestService(t)
	seedMangaRow(t, s, "p2|s2", "p2", "s2", "Original", "")
	if _, err := s.db.AddAltTitles("p2|s2", []string{"Known Alt"}, "src"); err != nil {
		t.Fatalf("add alt: %v", err)
	}

	// A form posted with an empty field must not blank the main title.
	if err := s.SetMainTitle("p2", "s2", ""); err == nil {
		t.Error("expected an error when promoting an empty title")
	}
	if got, _ := s.db.MangaTitle("p2", "s2"); got != "Original" {
		t.Errorf("title = %q, want the original untouched", got)
	}
}

func TestSetMainSummarySwapsAndDemotesOld(t *testing.T) {
	s := newTestService(t)
	seedMangaRow(t, s, "p2|s2", "p2", "s2", "Title", "Original synopsis.")
	if _, err := s.db.AddAltDescriptions("p2|s2", []string{"Better synopsis."}, "src"); err != nil {
		t.Fatalf("add alt descriptions: %v", err)
	}

	if err := s.SetMainSummary("p2", "s2", "Better synopsis."); err != nil {
		t.Fatalf("SetMainSummary: %v", err)
	}
	desc, custom, err := s.db.MangaDescriptionIfCustom("p2", "s2")
	if err != nil {
		t.Fatalf("MangaDescriptionIfCustom: %v", err)
	}
	if desc != "Better synopsis." {
		t.Errorf("description = %q, want Better synopsis.", desc)
	}
	if !custom {
		t.Error("description was not locked from plugin overwrites")
	}
	sums, err := s.db.ListAltDescriptions("p2|s2")
	if err != nil {
		t.Fatalf("ListAltDescriptions: %v", err)
	}
	var texts []string
	for _, r := range sums {
		texts = append(texts, r.Description)
	}
	if !containsStr(texts, "Original synopsis.") {
		t.Errorf("the old main synopsis was not demoted: %v", texts)
	}
	if containsStr(texts, "Better synopsis.") {
		t.Errorf("the promoted synopsis is still an alternative: %v", texts)
	}
}

func TestSetMainSummaryRejectsBlank(t *testing.T) {
	s := newTestService(t)
	seedMangaRow(t, s, "p2|s2", "p2", "s2", "Title", "Original synopsis.")

	for _, in := range []string{"", "   ", "\n\t"} {
		if err := s.SetMainSummary("p2", "s2", in); err == nil {
			t.Errorf("SetMainSummary(%q) succeeded, want an error", in)
		}
	}
	desc, _, err := s.db.MangaDescriptionIfCustom("p2", "s2")
	if err != nil {
		t.Fatalf("MangaDescriptionIfCustom: %v", err)
	}
	if desc != "Original synopsis." {
		t.Errorf("description = %q, want the original preserved", desc)
	}
}

// TestSetMainSummaryAcceptsRoundTrippedText pins a deliberate loosening: the
// page renders a description out of the DB and posts it back, and HTML entity
// round-tripping can make the text differ byte-for-byte. The strict
// membership check rejected those valid submissions, so only emptiness is
// rejected now.
func TestSetMainSummaryAcceptsRoundTrippedText(t *testing.T) {
	s := newTestService(t)
	seedMangaRow(t, s, "p2|s2", "p2", "s2", "Title", "Original synopsis.")
	if _, err := s.db.AddAltDescriptions("p2|s2", []string{`Tom & Jerry's "best"`}, "src"); err != nil {
		t.Fatalf("add alt descriptions: %v", err)
	}

	if err := s.SetMainSummary("p2", "s2", `Tom &amp; Jerry&#39;s &quot;best&quot;`); err != nil {
		t.Fatalf("SetMainSummary of an entity-encoded description: %v", err)
	}
	desc, custom, err := s.db.MangaDescriptionIfCustom("p2", "s2")
	if err != nil {
		t.Fatalf("MangaDescriptionIfCustom: %v", err)
	}
	if desc != `Tom &amp; Jerry&#39;s &quot;best&quot;` {
		t.Errorf("description = %q, want the submitted text stored verbatim", desc)
	}
	if !custom {
		t.Error("description was not locked from plugin overwrites")
	}
}

// TestRemoveAltSummaryDropsRow: the panel's only way to shrink the synopsis
// list. Without it a bad provider result can never be cleared.
func TestRemoveAltSummaryDropsRow(t *testing.T) {
	s := newTestService(t)
	seedMangaRow(t, s, "p2|s2", "p2", "s2", "Title", "Original synopsis.")
	if _, err := s.db.AddAltDescriptions("p2|s2", []string{"Alt synopsis"}, "src"); err != nil {
		t.Fatalf("add alt descriptions: %v", err)
	}

	if err := s.RemoveAltSummary("p2", "s2", "Alt synopsis"); err != nil {
		t.Fatalf("RemoveAltSummary: %v", err)
	}
	sums, err := s.db.ListAltDescriptions("p2|s2")
	if err != nil {
		t.Fatalf("ListAltDescriptions: %v", err)
	}
	if len(sums) != 0 {
		t.Errorf("alt synopses after removal = %v, want none", sums)
	}
}

func containsStr(haystack []string, needle string) bool {
	return slices.Contains(haystack, needle)
}
