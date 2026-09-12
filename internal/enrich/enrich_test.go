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

func Test_normalizeTitle(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"Solo Leveling", "Solo Leveling"},
		{"  Solo Leveling  ", "Solo Leveling"},
		{"Solo Leveling (manga)", "Solo Leveling"},
		{"Solo Leveling ( MANGA )", "Solo Leveling"},
		{"Solo Leveling (manga) (manga)", "Solo Leveling"},
	}
	for _, tt := range tests {
		got := normalizeTitle(tt.in)
		if got != tt.want {
			t.Errorf("normalizeTitle(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
