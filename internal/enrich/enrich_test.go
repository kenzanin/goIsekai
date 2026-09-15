package enrich

import (
	"context"
	"net/http"
	"reflect"
	"testing"
)

// mockProvider is a test provider that returns predefined items.
type mockProvider struct {
	id    string
	name  string
	kinds []Kind
	items []Item
	err   error
}

func (m *mockProvider) ID() string   { return m.id }
func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) Kinds() []Kind {
	out := make([]Kind, len(m.kinds))
	copy(out, m.kinds)
	return out
}
func (m *mockProvider) Fetch(_ context.Context, _ *http.Client, _ string, _ Kind) ([]Item, error) {
	return m.items, m.err
}

func TestRegister_DeduplicatesByID(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockProvider{id: "a", name: "A", kinds: []Kind{KindTitles}})
	r.Register(&mockProvider{id: "a", name: "A-dup", kinds: []Kind{KindTitles}})

	entries := r.Catalog("")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Name != "A" {
		t.Fatalf("expected name A, got %q", entries[0].Name)
	}
}

func TestCatalog_FiltersByKind(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockProvider{id: "a", name: "A", kinds: []Kind{KindTitles}})
	r.Register(&mockProvider{id: "b", name: "B", kinds: []Kind{KindTitles, KindCategories}})

	// Filter by titles: both A and B support it.
	titles := r.Catalog(KindTitles)
	if len(titles) != 2 {
		t.Fatalf("expected 2 entries for titles, got %d", len(titles))
	}

	// Filter by categories: only B supports it.
	cats := r.Catalog(KindCategories)
	if len(cats) != 1 {
		t.Fatalf("expected 1 entry for categories, got %d", len(cats))
	}
	if cats[0].ID != "b" {
		t.Fatalf("expected ID b, got %q", cats[0].ID)
	}

	// Unknown kind returns empty.
	unknown := r.Catalog("nonesuch")
	if len(unknown) != 0 {
		t.Fatalf("expected 0 for unknown kind, got %d", len(unknown))
	}
}

func TestResolve_Unknown(t *testing.T) {
	r := NewRegistry()
	if p := r.Resolve("nope"); p != nil {
		t.Fatalf("expected nil for unknown source, got %v", p)
	}
}

func TestFetch_UnknownSource(t *testing.T) {
	r := NewRegistry()
	_, err := r.Fetch(context.Background(), nil, "nope", "Title", KindTitles)
	if err == nil {
		t.Fatal("expected error for unknown source")
	}
}

func TestFetch_UnknownKind(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockProvider{id: "a", name: "A", kinds: []Kind{KindTitles}})
	_, err := r.Fetch(context.Background(), nil, "a", "Title", KindCategories)
	if err == nil {
		t.Fatal("expected error for unsupported kind")
	}
}

func TestFetch_ReturnsItems(t *testing.T) {
	r := NewRegistry()
	want := []Item{{Value: "Alt Title"}}
	r.Register(&mockProvider{id: "a", name: "A", kinds: []Kind{KindTitles}, items: want})

	got, err := r.Fetch(context.Background(), nil, "a", "Title", KindTitles)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestSupportsKind(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockProvider{id: "a", name: "A", kinds: []Kind{KindTitles}})

	if !r.SupportsKind("a", KindTitles) {
		t.Fatal("expected SupportsKind(a, titles) to be true")
	}
	if r.SupportsKind("a", KindCategories) {
		t.Fatal("expected SupportsKind(a, categories) to be false")
	}
	if r.SupportsKind("nope", KindTitles) {
		t.Fatal("expected SupportsKind(nope, titles) to be false")
	}
}

// kindMock answers per kind and records which kinds it was asked for.
type kindMock struct {
	id     string
	kinds  []Kind
	byKind map[Kind][]Item
	calls  []Kind
}

func (m *kindMock) ID() string { return m.id }
func (m *kindMock) Name() string {
	return m.id
}
func (m *kindMock) Kinds() []Kind {
	out := make([]Kind, len(m.kinds))
	copy(out, m.kinds)
	return out
}
func (m *kindMock) Fetch(_ context.Context, _ *http.Client, _ string, k Kind) ([]Item, error) {
	m.calls = append(m.calls, k)
	return m.byKind[k], nil
}

// TestCatalog_KeepsRegistrationOrder: the catalog is the caller's source
// precedence, so walking it must not shuffle the sources (a map walk did).
func TestCatalog_KeepsRegistrationOrder(t *testing.T) {
	r := NewRegistry()
	for _, id := range []string{"mangadex", "mangaupdates", "anilist"} {
		r.Register(&kindMock{id: id, kinds: []Kind{KindTitles}})
	}

	var got []string
	for _, e := range r.Catalog("") {
		got = append(got, e.ID)
	}
	want := []string{"mangadex", "mangaupdates", "anilist"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("catalog order = %v, want %v", got, want)
	}
}

// TestFetchFirst_FirstSourceOwnsAKind: whatever the first source answers is
// final for that kind, and a later source is only asked for the kinds the
// earlier ones left empty.
func TestFetchFirst_FirstSourceOwnsAKind(t *testing.T) {
	r := NewRegistry()
	first := &kindMock{id: "mangadex", kinds: []Kind{KindCategories, KindRelated}, byKind: map[Kind][]Item{
		KindCategories: {{Value: "Action"}},
	}}
	second := &kindMock{id: "mangaupdates", kinds: []Kind{KindCategories, KindRelated}, byKind: map[Kind][]Item{
		KindCategories: {{Value: "Shounen"}},
		KindRelated:    {{Value: "Related Manga"}},
	}}
	r.Register(first)
	r.Register(second)

	got := r.FetchFirst(context.Background(), nil, "Title", []string{"mangadex", "mangaupdates"})

	if len(got[KindCategories]) != 1 || got[KindCategories][0].Value != "Action" {
		t.Fatalf("categories = %+v, want the first source's single 'Action'", got[KindCategories])
	}
	if got[KindCategories][0].Source != "mangadex" {
		t.Errorf("categories source = %q, want mangadex", got[KindCategories][0].Source)
	}
	if len(got[KindRelated]) != 1 || got[KindRelated][0].Value != "Related Manga" {
		t.Fatalf("related = %+v, want the first source's empty kind filled by the second", got[KindRelated])
	}
	if got[KindRelated][0].Source != "mangaupdates" {
		t.Errorf("related source = %q, want mangaupdates", got[KindRelated][0].Source)
	}
	if len(second.calls) != 1 || second.calls[0] != KindRelated {
		t.Errorf("second source was asked for %v, want only related", second.calls)
	}
}
