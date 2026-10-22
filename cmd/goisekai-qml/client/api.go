// Package client is the goisekai-qml HTTP layer: it turns the server's JSON
// into the shapes the QML views render.
//
// It deliberately lives in its own package with no cgo and no Qt import, so it
// can be built and tested under CGO_ENABLED=0 even though the binary that uses
// it needs both. Anything Qt-specific belongs in ../main.go.
package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// apiManga mirrors the JSON of GET /api/library.
type apiManga struct {
	Title         string `json:"title"`
	PluginID      string `json:"plugin_id"`
	SourceMangaID string `json:"source_manga_id"`
	CoverURL      string `json:"cover_url"`
	TotalChapters int    `json:"total_chapters"`
	ReadChapters  int    `json:"read_chapters"`
	HasNew        bool   `json:"has_new"`
	UpdatedAt     string `json:"updated_at"`
	LastReadAt    string `json:"last_read_at"`
	DupGroup      int    `json:"dup_group"`
}

// Overview mirrors the overview block of GET /api/library/facets.
type Overview struct {
	TotalTitles    int    `json:"total_titles"`
	StatusDone     int    `json:"status_done"`
	StatusOngoing  int    `json:"status_ongoing"`
	StatusUnknown  int    `json:"status_unknown"`
	FullyRead      int    `json:"fully_read"`
	StartedReading int    `json:"started_reading"`
	HasUpdates     int    `json:"has_updates"`
	PagesRead      int    `json:"pages_read"`
	MostTitle      string `json:"most_title"`
	MostCount      int    `json:"most_count"`
	MostDup        int    `json:"most_dup"`
	FewestTitle    string `json:"fewest_title"`
	FewestCount    int    `json:"fewest_count"`
	FewestDup      int    `json:"fewest_dup"`
}

// FacetRow is one plugin or one category in the sidebar.
type FacetRow struct {
	PluginID string `json:"plugin_id"`
	Name     string `json:"name"`
	Icon     string `json:"icon"`
	Count    int    `json:"count"`
}

// Facets mirrors GET /api/library/facets.
type Facets struct {
	Overview       Overview            `json:"overview"`
	Plugins        []FacetRow          `json:"plugins"`
	Categories     []FacetRow          `json:"categories"`
	Tags           map[string][]string `json:"tags"`
	DuplicateCount int                 `json:"duplicate_count"`
}

// EmptyFacets is what a failed facets fetch degrades to: valid shapes with zero
// values, so the sidebar renders empty instead of erroring on a nil map.
func EmptyFacets() Facets {
	return Facets{
		Plugins:    []FacetRow{},
		Categories: []FacetRow{},
		Tags:       map[string][]string{},
	}
}

// getJSON fetches url and decodes it into out. The API key is optional because
// the server exposes /api/* unauthenticated unless goisekai.ini sets api_key.
func getJSON(base, path, apiKey string, out any) error {
	req, err := http.NewRequest(http.MethodGet, base+path, nil)
	if err != nil {
		return err
	}
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("query %s%s: %w", base, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return fmt.Errorf("query %s%s: %s: %s", base, path, resp.Status, body)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

// FetchLibrary returns the library as QML-ready items.
func FetchLibrary(base, apiKey string) ([]Item, error) {
	var rows []apiManga
	if err := getJSON(base, "/api/library", apiKey, &rows); err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(rows))
	for _, m := range rows {
		items = append(items, Item{
			Title:      m.Title,
			Cover:      coverProxyURL(base, m.PluginID, m.CoverURL),
			Read:       m.ReadChapters,
			Total:      m.TotalChapters,
			HasNew:     m.HasNew,
			UpdatedAt:  m.UpdatedAt,
			LastReadAt: m.LastReadAt,
			DupGroup:   m.DupGroup,
			Href:       mangaHref(base, m.PluginID, m.SourceMangaID),
			Key:        m.PluginID + ":" + m.SourceMangaID,
			PluginID:   m.PluginID,
		})
	}
	return items, nil
}

// FetchFacets returns the sidebar payload. A failure is not fatal: the caller
// gets empty-but-valid facets so the grid still works.
func FetchFacets(base, apiKey string) (Facets, error) {
	f := EmptyFacets()
	if err := getJSON(base, "/api/library/facets", apiKey, &f); err != nil {
		return EmptyFacets(), err
	}
	// Defensive: the server always sends these, but a null would become a nil
	// slice and break QML's .length.
	if f.Plugins == nil {
		f.Plugins = []FacetRow{}
	}
	if f.Categories == nil {
		f.Categories = []FacetRow{}
	}
	if f.Tags == nil {
		f.Tags = map[string][]string{}
	}
	return f, nil
}

// ErrNoServer marks transport failures, which the UI words differently from a
// malformed response.
var ErrNoServer = errors.New("server unreachable")
