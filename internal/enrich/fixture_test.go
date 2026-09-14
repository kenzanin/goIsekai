package enrich

import (
	"context"
	"github.com/goccy/go-json"
	"net/http"
	"os"
	"testing"
)

func TestMangaDexProvider_FetchFromLiveFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/mangadex_solo_leveling.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var resp mangaDexSearchResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Data) == 0 {
		t.Fatal("fixture has no data")
	}

	p := NewMangaDexProvider()

	// Titles: should have alt titles from the fixture
	items := p.fetchTitles(resp)
	if len(items) == 0 {
		t.Fatal("expected titles from fixture, got none")
	}
	// Should find English alt titles
	foundEN := false
	for _, it := range items {
		if it.Value == "Solo Leveling: Ragnarok" {
			foundEN = true
		}
	}
	if !foundEN {
		t.Errorf("expected English alt title 'Solo Leveling: Ragnarok' in titles, got: %v", items)
	}

	// Categories: should have genre tags
	items = p.fetchCategories(resp)
	if len(items) == 0 {
		t.Fatal("expected categories from fixture, got none")
	}
	// Should include "Action" genre
	foundAction := false
	for _, it := range items {
		if it.Value == "Action" {
			foundAction = true
		}
	}
	if !foundAction {
		t.Errorf("expected 'Action' genre in categories, got: %v", items)
	}

	// Related: should parse manga relationships
	items = p.fetchRelated(resp)
	// Solo Leveling: Ragnarok has related manga relationships
	t.Logf("related items: %d items", len(items))
	for _, it := range items {
		t.Logf("  related: %s", it.Value)
	}
}

func TestMangaUpdatesProvider_FetchFromLiveFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/mangaupdates_solo_leveling.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var resp muSearchResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Results) == 0 {
		t.Fatal("fixture has no results")
	}

	p := NewMangaUpdatesProvider()

	// Categories
	items := p.fetchCategories(resp.Results)
	if len(items) == 0 {
		t.Fatal("expected categories from fixture, got none")
	}
	t.Logf("categories: %v", items)
}

func TestMangaDexProvider_FetchFromLive(t *testing.T) {
	p := NewMangaDexProvider()

	// Test with a manga we know exists
	items, err := p.Fetch(context.Background(), http.DefaultClient, "Berserk", KindTitles)
	if err != nil {
		t.Skipf("live API unavailable: %v", err)
	}
	if len(items) == 0 {
		t.Log("no results (expected for some searches)")
		return
	}
	t.Logf("live titles: %v", items)
}

func TestMangaUpdatesProvider_FetchFromLive(t *testing.T) {
	p := NewMangaUpdatesProvider()

	items, err := p.Fetch(context.Background(), http.DefaultClient, "Berserk", KindTitles)
	if err != nil {
		t.Skipf("live API unavailable: %v", err)
	}
	if len(items) == 0 {
		t.Log("no results (expected for some searches)")
		return
	}
	t.Logf("live titles: %v", items)
}
