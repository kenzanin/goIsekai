package bridge

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	"goisekai/internal/database"
	"goisekai/internal/enrich"
	"goisekai/internal/hostnet"
)

// enrichMockProvider is a test provider returning predefined items per kind.
// It also records the title each Fetch was asked for, so tests can assert
// which title the caller fell back to.
type enrichMockProvider struct {
	id     string
	name   string
	kinds  []enrich.Kind
	items  map[enrich.Kind][]enrich.Item
	titles []string
}

func (m *enrichMockProvider) ID() string   { return m.id }
func (m *enrichMockProvider) Name() string { return m.name }
func (m *enrichMockProvider) Kinds() []enrich.Kind {
	out := make([]enrich.Kind, len(m.kinds))
	copy(out, m.kinds)
	return out
}
func (m *enrichMockProvider) Precedence() int { return 0 }
func (m *enrichMockProvider) Enabled() bool   { return true }

func (m *enrichMockProvider) Fetch(_ context.Context, _ *http.Client, title string, k enrich.Kind) ([]enrich.Item, error) {
	m.titles = append(m.titles, title)
	return m.items[k], nil
}

func TestFetchEnrichmentStoresAllKinds(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "enrich.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	mangaID, err := db.UpsertManga(database.Manga{
		PluginID: "p1", SourceMangaID: "m1", Title: "Title",
	})
	if err != nil {
		t.Fatalf("upsert manga: %v", err)
	}
	_ = mangaID

	reg := enrich.NewRegistry()
	reg.Register(&enrichMockProvider{
		id:   "p1",
		name: "P1",
		kinds: []enrich.Kind{
			enrich.KindTitles, enrich.KindSummaries,
			enrich.KindCategories, enrich.KindRelated,
		},
		items: map[enrich.Kind][]enrich.Item{
			enrich.KindTitles:     {{Value: "Alt Title", Source: "p1"}},
			enrich.KindSummaries:  {{Value: "Alt Summary", Source: "p1"}},
			enrich.KindCategories: {{Value: "Action", Source: "p1"}},
			enrich.KindRelated:    {{Value: "Related Manga", URL: "http://x", Source: "p1"}},
		},
	})
	s := NewAppService(db, nil, hostnet.NewProxy(), "", "", reg)

	if err := s.FetchEnrichment("p1", "m1", "Title", []string{"p1"}); err != nil {
		t.Fatalf("FetchEnrichment: %v", err)
	}

	titles, _ := s.ListAltTitles("p1", "m1")
	if len(titles) != 1 || titles[0].Title != "Alt Title" {
		t.Errorf("titles = %+v, want 1 'Alt Title'", titles)
	}
	summs, _ := s.ListAltSummaries("p1", "m1")
	if len(summs) != 1 || summs[0].Description != "Alt Summary" {
		t.Errorf("summaries = %+v, want 1 'Alt Summary'", summs)
	}
	// Categories and related feed the detail page's Related / Recommended
	// section, which the templates drop silently when the list is empty.
	cats, _ := s.ListCategories("p1", "m1")
	if len(cats) != 1 || cats[0].Value != "Action" {
		t.Errorf("categories = %+v, want 1 'Action'", cats)
	}
	rels, _ := s.ListRelated("p1", "m1")
	if len(rels) != 1 || rels[0].Value != "Related Manga" || rels[0].URL != "http://x" {
		t.Errorf("related = %+v, want 1 'Related Manga' with its URL", rels)
	}
}

// TestFetchEnrichmentPrefersTheFirstSource: the source list is a precedence
// order, not a merge list. A kind the first source answers is final, and a
// kind it leaves empty falls through to the next source, so one source owns
// each section instead of every source piling into all of them.
func TestFetchEnrichmentPrefersTheFirstSource(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "enrich.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	mangaID, err := db.UpsertManga(database.Manga{
		PluginID: "p1", SourceMangaID: "m1", Title: "Title",
	})
	if err != nil {
		t.Fatalf("upsert manga: %v", err)
	}
	_ = mangaID

	primary := &enrichMockProvider{
		id:   "mangadex",
		name: "MangaDex",
		kinds: []enrich.Kind{
			enrich.KindCategories, enrich.KindRelated,
		},
		items: map[enrich.Kind][]enrich.Item{
			enrich.KindCategories: {{Value: "Action", Source: "mangadex"}},
		},
	}
	fallback := &enrichMockProvider{
		id:   "mangaupdates",
		name: "MangaUpdates",
		kinds: []enrich.Kind{
			enrich.KindCategories, enrich.KindRelated,
		},
		items: map[enrich.Kind][]enrich.Item{
			enrich.KindCategories: {{Value: "Shounen", Source: "mangaupdates"}},
			enrich.KindRelated:    {{Value: "Related Manga", URL: "http://x", Source: "mangaupdates"}},
		},
	}
	reg := enrich.NewRegistry()
	reg.Register(primary)
	reg.Register(fallback)
	s := NewAppService(db, nil, hostnet.NewProxy(), "", "", reg)

	if err := s.FetchEnrichment("p1", "m1", "Title", []string{"mangadex", "mangaupdates"}); err != nil {
		t.Fatalf("FetchEnrichment: %v", err)
	}

	cats, _ := s.ListCategories("p1", "m1")
	if len(cats) != 1 || cats[0].Value != "Action" || cats[0].Source != "mangadex" {
		t.Errorf("categories = %+v, want only the first source's 'Action'", cats)
	}
	rels, _ := s.ListRelated("p1", "m1")
	if len(rels) != 1 || rels[0].Value != "Related Manga" || rels[0].Source != "mangaupdates" {
		t.Errorf("related = %+v, want the fallback source's entry", rels)
	}
	// The first source answered categories, so the second was never asked.
	if n := len(fallback.titles); n != 1 {
		t.Errorf("the fallback source was consulted %d times, want 1 (related only)", n)
	}
}

// TestGetEnrichmentReturnsEveryStoredSection: the API response mirrors the
// detail page, and alt titles/summaries live in their own tables rather than
// in the enrichment kinds, so they need their own lookup path.
func TestGetEnrichmentReturnsEveryStoredSection(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "enrich.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	mangaID, err := db.UpsertManga(database.Manga{
		PluginID: "p1", SourceMangaID: "m1", Title: "Title",
	})
	if err != nil {
		t.Fatalf("upsert manga: %v", err)
	}
	rowID := fmt.Sprint(mangaID)
	if _, err := db.AddAltTitles(rowID, []string{"Alt Title"}, "p1"); err != nil {
		t.Fatalf("add alt titles: %v", err)
	}
	if _, err := db.AddAltDescriptions(rowID, []string{"Alt Summary"}, "p1"); err != nil {
		t.Fatalf("add alt summaries: %v", err)
	}
	if _, err := db.AddCategories(rowID, []string{"Action"}, "p1"); err != nil {
		t.Fatalf("add categories: %v", err)
	}
	if _, err := db.AddRelated(rowID, []database.RelatedRow{{Title: "Related Manga", URL: "http://x"}}, "p1"); err != nil {
		t.Fatalf("add related: %v", err)
	}

	s := NewAppService(db, nil, hostnet.NewProxy(), "", "", nil)
	got, err := s.GetEnrichment("p1", "m1")
	if err != nil {
		t.Fatalf("GetEnrichment: %v", err)
	}
	if len(got.AltTitles) != 1 || got.AltTitles[0].Value != "Alt Title" {
		t.Errorf("AltTitles = %+v, want 1 'Alt Title'", got.AltTitles)
	}
	if len(got.AltSummaries) != 1 || got.AltSummaries[0].Value != "Alt Summary" {
		t.Errorf("AltSummaries = %+v, want 1 'Alt Summary'", got.AltSummaries)
	}
	if len(got.Categories) != 1 || got.Categories[0].Value != "Action" {
		t.Errorf("Categories = %+v, want 1 'Action'", got.Categories)
	}
	if len(got.Related) != 1 || got.Related[0].Value != "Related Manga" {
		t.Errorf("Related = %+v, want 1 'Related Manga'", got.Related)
	}
}

// TestFetchEnrichmentMultiSource: when sources is empty, fetch from all enabled providers.
func TestFetchEnrichmentMultiSource(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "enrich.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	mangaID, err := db.UpsertManga(database.Manga{
		PluginID: "p1", SourceMangaID: "m1", Title: "Title",
	})
	if err != nil {
		t.Fatalf("upsert manga: %v", err)
	}
	_ = mangaID

	primary := &enrichMockProvider{
		id:   "mangadex",
		name: "MangaDex",
		kinds: []enrich.Kind{
			enrich.KindCategories,
		},
		items: map[enrich.Kind][]enrich.Item{
			enrich.KindCategories: {{Value: "Action", Source: "mangadex"}},
		},
	}
	fallback := &enrichMockProvider{
		id:   "mangaupdates",
		name: "MangaUpdates",
		kinds: []enrich.Kind{
			enrich.KindCategories,
		},
		items: map[enrich.Kind][]enrich.Item{
			enrich.KindCategories: {{Value: "Adventure", Source: "mangaupdates"}},
		},
	}
	reg := enrich.NewRegistry()
	reg.Register(primary)
	reg.Register(fallback)
	s := NewAppService(db, nil, hostnet.NewProxy(), "", "", reg)

	// Multi-source fetch (empty sources slice)
	if err := s.FetchEnrichment("p1", "m1", "Title", nil); err != nil {
		t.Fatalf("FetchEnrichment: %v", err)
	}

	// Both sources should have contributed categories
	cats, _ := s.ListCategories("p1", "m1")
	if len(cats) < 2 {
		t.Errorf("expected categories from both sources, got %d items: %+v", len(cats), cats)
	}
}
