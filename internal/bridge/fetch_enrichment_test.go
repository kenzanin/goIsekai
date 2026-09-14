package bridge

import (
	"context"
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

	if err := db.UpsertManga(database.Manga{
		ID: "p1|m1", PluginID: "p1", SourceMangaID: "m1", Title: "Title",
	}); err != nil {
		t.Fatalf("upsert manga: %v", err)
	}

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
}
