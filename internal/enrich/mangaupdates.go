package enrich

import (
	"bytes"
	"context"
	"fmt"
	"github.com/goccy/go-json"
	"io"
	"net/http"

	"goisekai/internal/logger"
	"goisekai/internal/pluginutil"
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
	return []Kind{KindTitles, KindSummaries, KindCategories, KindRelated}
}

// muUserAgent is sent with every MangaUpdates request.
// ponytail: read from config when providers gain access to AppService.
const muUserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36"

func (p *MangaUpdatesProvider) Fetch(ctx context.Context, httpc *http.Client, title string, k Kind) ([]Item, error) {
	normalized := normalizeTitle(title)
	// Search endpoint: POST /v1/series/search with JSON body {"search":"..."}
	url := fmt.Sprintf("%s/v1/series/search", muBase)

	bodyJSON := []byte(fmt.Sprintf(`{"search":"%s"}`, normalized))
	body := bytes.NewReader(bodyJSON)
	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("mangaupdates: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", muUserAgent)

	resp, err := httpc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mangaupdates: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		logger.Debug("mangaupdates: search failed", "status", resp.StatusCode, "body", string(respBody), "title", title)
		return nil, fmt.Errorf("mangaupdates: status %d", resp.StatusCode)
	}

	var searchResp muSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("mangaupdates: decode: %w", err)
	}

	switch k {
	case KindTitles:
		if len(searchResp.Results) == 0 {
			return nil, nil
		}
		return p.fetchAltTitles(ctx, httpc, searchResp.Results[0].Record.ID)
	case KindSummaries:
		return p.fetchSummaries(searchResp.Results), nil
	case KindCategories:
		return p.fetchCategories(searchResp.Results), nil
	case KindRelated:
		if len(searchResp.Results) == 0 {
			return nil, nil
		}
		return p.fetchRelated(ctx, httpc, searchResp.Results[0].Record.ID)
	default:
		return nil, fmt.Errorf("mangaupdates: unknown kind %q", k)
	}
}

// muSearchResponse is the top-level shape of the MangaUpdates search API.
type muSearchResponse struct {
	TotalHits int            `json:"total_hits"`
	Results   []muResultItem `json:"results"`
}

type muResultItem struct {
	Record muSeriesRecord `json:"record"`
}

// muSeriesRecord mirrors the series shape in a MangaUpdates search result.
// The MU API search endpoint returns title, genres, description but NOT
// recommendations or related-series data. Those require a separate API call
// not exposed by the public search endpoint.
type muSeriesRecord struct {
	ID             int             `json:"series_id"`
	Title          string          `json:"title"`
	URL            string          `json:"url"`
	Genres         []muGenreEntry  `json:"genres"`
	Description    string          `json:"description"`
	Image          json.RawMessage `json:"image"`
	ImageURL       string          `json:"-"`
	Type           string          `json:"type"`
	Year           string          `json:"year"`
	BayesianRating float64         `json:"bayesian_rating"`
	RatingVotes    int             `json:"rating_votes"`
	LastUpdated    json.RawMessage `json:"last_updated"`
}

type muGenreEntry struct {
	Genre string `json:"genre"`
}

// muRelatedSeries is a related-series entry from the MangaUpdates series detail API.
type muRelatedSeries struct {
	RelationType      string `json:"relation_type"`
	RelatedSeriesID   int    `json:"related_series_id"`
	RelatedSeriesName string `json:"related_series_name"`
	RelatedSeriesURL  string `json:"related_series_url"`
}

// muAssociatedTitle is one alternative-title entry from the series detail API.
type muAssociatedTitle struct {
	Title string `json:"title"`
}

// muSeriesDetail is the full series response from GET /v1/series/{id}.
type muSeriesDetail struct {
	Description   string              `json:"description"`
	RelatedSeries []muRelatedSeries   `json:"related_series"`
	Associated    []muAssociatedTitle `json:"associated"`
}

// fetchDetail loads the full series record from GET /v1/series/{id}.
func (p *MangaUpdatesProvider) fetchDetail(ctx context.Context, httpc *http.Client, seriesID int) (*muSeriesDetail, error) {
	url := fmt.Sprintf("%s/v1/series/%d", muBase, seriesID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("mangaupdates: build detail request: %w", err)
	}
	req.Header.Set("User-Agent", muUserAgent)
	resp, err := httpc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mangaupdates: detail request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mangaupdates: detail status %d", resp.StatusCode)
	}
	var detail muSeriesDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return nil, fmt.Errorf("mangaupdates: detail decode: %w", err)
	}
	return &detail, nil
}

func (p *MangaUpdatesProvider) fetchRelated(ctx context.Context, httpc *http.Client, seriesID int) ([]Item, error) {
	detail, err := p.fetchDetail(ctx, httpc, seriesID)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var items []Item
	for _, rs := range detail.RelatedSeries {
		name := rs.RelatedSeriesName
		if name != "" && !seen[name] {
			seen[name] = true
			items = append(items, Item{Value: name, URL: rs.RelatedSeriesURL})
		}
	}
	return items, nil
}

// fetchAltTitles returns the series' alternative titles (including translated
// ones) from the detail API's associated[] list. The search endpoint only
// exposes the main title, so the detail call is required. Only the first
// (best-matching) search result is used; the rest are unrelated series.
func (p *MangaUpdatesProvider) fetchAltTitles(ctx context.Context, httpc *http.Client, seriesID int) ([]Item, error) {
	detail, err := p.fetchDetail(ctx, httpc, seriesID)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var items []Item
	for _, a := range detail.Associated {
		if a.Title != "" && !seen[a.Title] {
			seen[a.Title] = true
			items = append(items, Item{Value: a.Title})
		}
	}
	return items, nil
}

// fetchSummaries returns the synopsis of the best-matching series. Using every
// result merges the descriptions of unrelated series into one entry. The
// description arrives as markdown (e.g. an "[Official Web Raw](url)" prefix),
// so it is stripped to plain text like the plugin path does.
func (p *MangaUpdatesProvider) fetchSummaries(results []muResultItem) []Item {
	if len(results) == 0 || results[0].Record.Description == "" {
		return nil
	}
	return []Item{{Value: pluginutil.StripMarkdown(results[0].Record.Description)}}
}

// fetchCategories returns the deduped genres of the best-matching series.
func (p *MangaUpdatesProvider) fetchCategories(results []muResultItem) []Item {
	if len(results) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	var items []Item
	for _, g := range results[0].Record.Genres {
		if g.Genre != "" && !seen[g.Genre] {
			seen[g.Genre] = true
			items = append(items, Item{Value: g.Genre})
		}
	}
	return items
}
