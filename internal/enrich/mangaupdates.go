package enrich

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// MangaUpdatesProvider fetches enrichment data from MangaUpdates (api.mangaupdates.com).
type MangaUpdatesProvider struct{}

// NewMangaUpdatesProvider returns a new MangaUpdatesProvider.
func NewMangaUpdatesProvider() *MangaUpdatesProvider {
	return &MangaUpdatesProvider{}
}

func (p *MangaUpdatesProvider) ID() string   { return "mangaupdates" }
func (p *MangaUpdatesProvider) Name() string { return "MangaUpdates" }
func (p *MangaUpdatesProvider) Kinds() []Kind {
	return []Kind{KindTitles, KindSummaries, KindCategories}
}

func (p *MangaUpdatesProvider) Fetch(ctx context.Context, httpc *http.Client, title string, k Kind) ([]Item, error) {
	normalized := normalizeTitle(title)
	// Search endpoint: POST /v1/series/search with JSON body {"title":"..."}
	url := fmt.Sprintf("%s/v1/series/search", muBase)

	body := bytes.NewReader([]byte(fmt.Sprintf(`{"title":"%s"}`, normalized)))
	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("mangaupdates: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mangaupdates: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mangaupdates: status %d", resp.StatusCode)
	}

	var searchResp muSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("mangaupdates: decode: %w", err)
	}

	switch k {
	case KindTitles:
		return p.fetchTitles(searchResp.Results), nil
	case KindSummaries:
		return p.fetchSummaries(searchResp.Results), nil
	case KindCategories:
		return p.fetchCategories(searchResp.Results), nil
	default:
		return nil, fmt.Errorf("mangaupdates: unknown kind %q", k)
	}
}

// muSearchResponse is the top-level shape of the MangaUpdates search API.
type muSearchResponse struct {
	TotalHits int              `json:"total_hits"`
	Results   []muResultItem   `json:"results"`
}

type muResultItem struct {
	Record muSeriesRecord `json:"record"`
}

// muSeriesRecord mirrors the series shape in a MangaUpdates search result.
// The MU API search endpoint returns title, genres, description but NOT
// recommendations or related-series data. Those require a separate API call
// not exposed by the public search endpoint.
type muSeriesRecord struct {
	ID            int              `json:"series_id"`
	Title         string           `json:"title"`
	URL           string           `json:"url"`
	Genres        []muGenreEntry   `json:"genres"`
	Description   string           `json:"description"`
	Image         json.RawMessage  `json:"image"`
	ImageURL      string           `json:"-"`
	Type          string           `json:"type"`
	Year          string           `json:"year"`
	BayesianRating float64         `json:"bayesian_rating"`
	RatingVotes   int              `json:"rating_votes"`
	LastUpdated   json.RawMessage  `json:"last_updated"`
}

type muGenreEntry struct {
	Genre string `json:"genre"`
}

// ponytail: If MU adds a /v1/series/{id}/recommendations endpoint, add
// KindRelated support here. Ceiling: MU public API capability.

func (p *MangaUpdatesProvider) fetchTitles(results []muResultItem) []Item {
	var items []Item
	for _, r := range results {
		if r.Record.Title != "" {
			items = append(items, Item{Value: r.Record.Title})
		}
	}
	return items
}

func (p *MangaUpdatesProvider) fetchSummaries(results []muResultItem) []Item {
	var items []Item
	for _, r := range results {
		if r.Record.Description != "" {
			items = append(items, Item{Value: r.Record.Description})
		}
	}
	return items
}

func (p *MangaUpdatesProvider) fetchCategories(results []muResultItem) []Item {
	seen := make(map[string]bool)
	var items []Item
	for _, r := range results {
		for _, g := range r.Record.Genres {
			if g.Genre != "" && !seen[g.Genre] {
				seen[g.Genre] = true
				items = append(items, Item{Value: g.Genre})
			}
		}
	}
	return items
}
