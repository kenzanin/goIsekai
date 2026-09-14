package enrich

import (
	"context"
	"fmt"
	"github.com/goccy/go-json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMangaDexProvider_FetchTitles(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/manga", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("title") != "Solo Leveling" {
			t.Errorf("unexpected title param: %s", r.URL.Query().Get("title"))
		}
		resp := mangaDexSearchResponse{
			Data: []mangaDexManga{
				{
					ID: "md1",
					Attributes: mangaDexMangaAttrs{
						Title: map[string]string{"ko": "나 혼자만 레벨업"},
						AltTitles: []map[string]string{
							{"ko": "나 혼자만 레벨업"},
							{"ja": "独りだけレベルアップ"},
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	origBase := mangaDexBase
	mangaDexBase = srv.URL

	p := NewMangaDexProvider()
	httpc := srv.Client()

	items, err := p.Fetch(context.Background(), httpc, "Solo Leveling", KindTitles)
	mangaDexBase = origBase
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected titles, got none")
	}
	// Should have the alt title first.
	if items[0].Value != "나 혼자만 레벨업" {
		t.Errorf("first title = %q, want 나 혼자만 레벨업", items[0].Value)
	}
}

func TestMangaDexProvider_FetchCategories(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/manga", func(w http.ResponseWriter, r *http.Request) {
		resp := mangaDexSearchResponse{
			Data: []mangaDexManga{
				{
					ID: "md1",
					Attributes: mangaDexMangaAttrs{
						Title: map[string]string{"en": "Solo Leveling"},
						Tags: []mangaDexTag{
							{ID: "1", Type: "tag", Attributes: mangaDexTagAttrs{Name: map[string]string{"en": "Action"}, Group: "genre"}},
							{ID: "2", Type: "tag", Attributes: mangaDexTagAttrs{Name: map[string]string{"en": "Fantasy"}, Group: "genre"}},
							{ID: "3", Type: "tag", Attributes: mangaDexTagAttrs{Name: map[string]string{"en": "Action"}, Group: "genre"}},      // dup
							{ID: "4", Type: "tag", Attributes: mangaDexTagAttrs{Name: map[string]string{"en": "Long Strip"}, Group: "format"}}, // skip
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	origBase := mangaDexBase
	mangaDexBase = srv.URL

	p := NewMangaDexProvider()
	httpc := srv.Client()

	items, err := p.Fetch(context.Background(), httpc, "Solo Leveling", KindCategories)
	mangaDexBase = origBase
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(items))
	}
}

func TestMangaDexProvider_FetchRelated(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/manga", func(w http.ResponseWriter, r *http.Request) {
		relAttr, _ := json.Marshal(map[string]string{"title": "The Beginning After the End"})
		resp := mangaDexSearchResponse{
			Data: []mangaDexManga{
				{
					ID: "md1",
					Attributes: mangaDexMangaAttrs{
						Title: map[string]string{"en": "Solo Leveling"},
					},
					Relationships: []mangaDexRelation{
						{ID: "md2", Type: "manga", Attributes: relAttr},
						{ID: "md3", Type: "anime", Attributes: relAttr}, // should be skipped
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	origBase := mangaDexBase
	mangaDexBase = srv.URL

	p := NewMangaDexProvider()
	httpc := srv.Client()

	items, err := p.Fetch(context.Background(), httpc, "Solo Leveling", KindRelated)
	mangaDexBase = origBase
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 related, got %d", len(items))
	}
	if items[0].Value != "The Beginning After the End" {
		t.Errorf("related = %q, want The Beginning After the End", items[0].Value)
	}
}

func TestMangaDexProvider_FetchEmpty(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/manga", func(w http.ResponseWriter, r *http.Request) {
		resp := mangaDexSearchResponse{Data: []mangaDexManga{}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	origBase := mangaDexBase
	mangaDexBase = srv.URL

	p := NewMangaDexProvider()
	httpc := srv.Client()

	for _, k := range []Kind{KindTitles, KindCategories, KindRelated} {
		items, err := p.Fetch(context.Background(), httpc, "Not Found", k)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", k, err)
		}
		if len(items) != 0 {
			t.Errorf("%s: expected empty, got %d items", k, len(items))
		}
	}
	mangaDexBase = origBase
}

func TestMangaDexProvider_Kinds(t *testing.T) {
	p := NewMangaDexProvider()
	if len(p.Kinds()) != 3 {
		t.Fatalf("expected 3 kinds, got %d", len(p.Kinds()))
	}
}

func TestMangaDexProvider_FetchError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/manga", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	origBase := mangaDexBase
	mangaDexBase = srv.URL

	p := NewMangaDexProvider()
	httpc := srv.Client()

	_, err := p.Fetch(context.Background(), httpc, "Solo Leveling", KindTitles)
	mangaDexBase = origBase
	if err == nil {
		t.Fatal("expected error, got none")
	}
}

// brokenProvider always fails its Fetch.
type brokenProvider struct {
	id string
}

func (b *brokenProvider) ID() string    { return b.id }
func (b *brokenProvider) Name() string  { return "Broken" }
func (b *brokenProvider) Kinds() []Kind { return []Kind{KindCategories} }
func (b *brokenProvider) Fetch(_ context.Context, _ *http.Client, _ string, _ Kind) ([]Item, error) {
	return nil, fmt.Errorf("broken: plugin exploded")
}

func TestErrorIsolation(t *testing.T) {
	// Build a registry with a broken provider and a working one.
	reg := NewRegistry()
	reg.Register(&brokenProvider{id: "broken"})
	reg.Register(NewMangaDexProvider())

	mux := http.NewServeMux()
	mux.HandleFunc("/manga", func(w http.ResponseWriter, r *http.Request) {
		resp := mangaDexSearchResponse{
			Data: []mangaDexManga{
				{
					ID: "md1",
					Attributes: mangaDexMangaAttrs{
						Title: map[string]string{"en": "Solo Leveling"},
						Tags:  []mangaDexTag{{ID: "1", Type: "tag", Attributes: mangaDexTagAttrs{Name: map[string]string{"en": "Action"}, Group: "genre"}}},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	origBase := mangaDexBase
	mangaDexBase = srv.URL
	defer func() { mangaDexBase = origBase }()

	// Broken provider fails.
	_, err := reg.Fetch(context.Background(), srv.Client(), "broken", "Solo Leveling", KindCategories)
	if err == nil {
		t.Fatal("expected error from broken provider")
	}

	// Built-in provider works independently.
	items, err := reg.Fetch(context.Background(), srv.Client(), "mangadex", "Solo Leveling", KindCategories)
	if err != nil {
		t.Fatalf("mangadex fetch failed: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected categories from mangadex, got none")
	}
}

func TestMangaUpdatesProvider_FetchTitles(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/series/search", func(w http.ResponseWriter, r *http.Request) {
		// Verify POST body
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"search":"Solo Leveling"}` {
			t.Errorf("unexpected body: %s", string(body))
		}
		resp := muSearchResponse{
			Results: []muResultItem{
				{Record: muSeriesRecord{ID: 42, Title: "Solo Leveling"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	// Alt titles live on the series detail endpoint's associated[] list.
	mux.HandleFunc("/v1/series/42", func(w http.ResponseWriter, r *http.Request) {
		resp := muSeriesDetail{
			Associated: []muAssociatedTitle{
				{Title: "Solo Leveling"},
				{Title: "Only I Level Up"},
				{Title: "Solo Leveling"}, // dup
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	orig := muBase
	muBase = srv.URL

	p := NewMangaUpdatesProvider()
	httpc := srv.Client()

	items, err := p.Fetch(context.Background(), httpc, "Solo Leveling", KindTitles)
	muBase = orig
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 deduped titles, got %d: %v", len(items), items)
	}
	if items[0].Value != "Solo Leveling" || items[1].Value != "Only I Level Up" {
		t.Errorf("titles = %v, want [Solo Leveling, Only I Level Up]", items)
	}
}

func TestMangaUpdatesProvider_FetchSummaries(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/series/search", func(w http.ResponseWriter, r *http.Request) {
		resp := muSearchResponse{
			Results: []muResultItem{
				{Record: muSeriesRecord{Description: "A longer description."}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	orig := muBase
	muBase = srv.URL

	p := NewMangaUpdatesProvider()
	httpc := srv.Client()

	items, err := p.Fetch(context.Background(), httpc, "Test", KindSummaries)
	muBase = orig
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(items))
	}
	if items[0].Value != "A longer description." {
		t.Errorf("summary = %q, want A longer description.", items[0].Value)
	}
}

func TestMangaUpdatesProvider_FetchCategories(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/series/search", func(w http.ResponseWriter, r *http.Request) {
		resp := muSearchResponse{
			Results: []muResultItem{
				{Record: muSeriesRecord{
					Genres: []muGenreEntry{
						{Genre: "Action"},
						{Genre: "Fantasy"},
						{Genre: "Action"}, // dup
					},
				}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	orig := muBase
	muBase = srv.URL

	p := NewMangaUpdatesProvider()
	httpc := srv.Client()

	items, err := p.Fetch(context.Background(), httpc, "Test", KindCategories)
	muBase = orig
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(items))
	}
}

func TestMangaUpdatesProvider_FetchEmpty(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/series/search", func(w http.ResponseWriter, r *http.Request) {
		resp := muSearchResponse{Results: []muResultItem{}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	orig := muBase
	muBase = srv.URL

	p := NewMangaUpdatesProvider()
	httpc := srv.Client()

	for _, k := range []Kind{KindTitles, KindSummaries, KindCategories} {
		items, err := p.Fetch(context.Background(), httpc, "Not Found", k)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", k, err)
		}
		if len(items) != 0 {
			t.Errorf("%s: expected empty, got %d items", k, len(items))
		}
	}
	muBase = orig
}

func TestMangaUpdatesProvider_Kinds(t *testing.T) {
	p := NewMangaUpdatesProvider()
	if len(p.Kinds()) != 4 {
		t.Fatalf("expected 4 kinds, got %d", len(p.Kinds()))
	}
}

func TestMangaUpdatesProvider_FetchError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/series/search", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	orig := muBase
	muBase = srv.URL

	p := NewMangaUpdatesProvider()
	httpc := srv.Client()

	_, err := p.Fetch(context.Background(), httpc, "Test", KindTitles)
	muBase = orig
	if err == nil {
		t.Fatal("expected error, got none")
	}
}

// TestMangaUpdatesProvider_OnlyBestMatch guards against merging the metadata of
// unrelated series: the search endpoint returns up to 25 fuzzy matches, and
// only the first (most relevant) one describes the queried series.
func TestMangaUpdatesProvider_OnlyBestMatch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/series/search", func(w http.ResponseWriter, r *http.Request) {
		resp := muSearchResponse{
			Results: []muResultItem{
				{Record: muSeriesRecord{
					ID:          100,
					Title:       "The Right Series",
					Description: "The right synopsis.",
					Genres:      []muGenreEntry{{Genre: "Action"}},
				}},
				{Record: muSeriesRecord{
					ID:          200,
					Title:       "An Unrelated Series",
					Description: "An unrelated synopsis.",
					Genres:      []muGenreEntry{{Genre: "Romance"}},
				}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	// Only the best match (series 100) may be queried for its alt titles.
	// related_series is served on purpose: KindRelated must ignore it and read
	// category_recommendations instead.
	mux.HandleFunc("/v1/series/100", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"associated":               []map[string]any{{"title": "The Right Series"}},
			"related_series":           []map[string]any{{"related_series_name": "Plot Relation"}},
			"category_recommendations": []map[string]any{{"series_name": "Recommended Series", "series_url": "http://rec"}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/v1/series/200", func(w http.ResponseWriter, r *http.Request) {
		t.Error("fetched detail for an unrelated series")
		w.WriteHeader(http.StatusNotFound)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	orig := muBase
	muBase = srv.URL

	p := NewMangaUpdatesProvider()
	httpc := srv.Client()

	for _, tc := range []struct {
		kind    Kind
		want    string
		wantURL string
	}{
		{KindTitles, "The Right Series", ""},
		{KindSummaries, "The right synopsis.", ""},
		{KindCategories, "Action", ""},
		{KindRelated, "Recommended Series", "http://rec"},
	} {
		items, err := p.Fetch(context.Background(), httpc, "The Right Series", tc.kind)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.kind, err)
		}
		if len(items) != 1 {
			t.Fatalf("%s: expected 1 item from best match, got %d: %v", tc.kind, len(items), items)
		}
		if items[0].Value != tc.want {
			t.Errorf("%s: got %q, want %q", tc.kind, items[0].Value, tc.want)
		}
		if tc.wantURL != "" && items[0].URL != tc.wantURL {
			t.Errorf("%s: url = %q, want %q", tc.kind, items[0].URL, tc.wantURL)
		}
	}
	muBase = orig
}

// TestMangaDexProvider_OnlyBestMatch guards against merging titles and genres
// across the multiple fuzzy matches the MangaDex search endpoint returns.
func TestMangaDexProvider_OnlyBestMatch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/manga", func(w http.ResponseWriter, r *http.Request) {
		resp := mangaDexSearchResponse{
			Data: []mangaDexManga{
				{
					Attributes: mangaDexMangaAttrs{
						Title:     map[string]string{"en": "The Right Series"},
						AltTitles: []map[string]string{{"en": "Right Alt"}},
						Tags: []mangaDexTag{
							{Attributes: mangaDexTagAttrs{Group: "genre", Name: map[string]string{"en": "Action"}}},
						},
					},
				},
				{
					Attributes: mangaDexMangaAttrs{
						Title:     map[string]string{"en": "An Unrelated Series"},
						AltTitles: []map[string]string{{"en": "Unrelated Alt"}},
						Tags: []mangaDexTag{
							{Attributes: mangaDexTagAttrs{Group: "genre", Name: map[string]string{"en": "Romance"}}},
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	orig := mangaDexBase
	mangaDexBase = srv.URL

	p := NewMangaDexProvider()
	httpc := srv.Client()

	titles, err := p.Fetch(context.Background(), httpc, "The Right Series", KindTitles)
	if err != nil {
		t.Fatalf("titles: unexpected error: %v", err)
	}
	if len(titles) != 1 || titles[0].Value != "Right Alt" {
		t.Errorf("titles = %v, want [Right Alt]", titles)
	}

	cats, err := p.Fetch(context.Background(), httpc, "The Right Series", KindCategories)
	if err != nil {
		t.Fatalf("categories: unexpected error: %v", err)
	}
	if len(cats) != 1 || cats[0].Value != "Action" {
		t.Errorf("categories = %v, want [Action]", cats)
	}
	mangaDexBase = orig
}
