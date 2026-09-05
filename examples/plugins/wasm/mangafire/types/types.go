// Package types defines the JSON contract shared between the goIsekai host
// and its manga source plugins. This is a standalone copy of the core
// pkg/types contract so this plugin builds without importing the repo root;
// the JSON tags must match the host exactly.
package types

import "time"

// Manga is a single manga series.
type Manga struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	CoverURL    string   `json:"cover_url"`
	Author      string   `json:"author,omitempty"`
	Description string   `json:"description,omitempty"`
	Status      string   `json:"status,omitempty"`
	Genres      []string `json:"genres,omitempty"`
}

// Chapter is a single chapter of a manga.
type Chapter struct {
	ID         string    `json:"id"`
	MangaID    string    `json:"manga_id"`
	Title      string    `json:"title"`
	ChapterNum float64   `json:"chapter_num"`
	VolumeNum  float64   `json:"volume_num,omitempty"`
	ReleasedAt time.Time `json:"released_at"`
	URL        string    `json:"url"`
}

// Page is a single image page within a chapter. Headers holds optional
// per-page overrides (e.g. a custom Referer or User-Agent) required to fetch
// the image.
type Page struct {
	Index   int               `json:"index"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

// SearchFilter is a search request passed to a plugin's Search function.
type SearchFilter struct {
	Query  string   `json:"query"`
	Page   int      `json:"page"`
	Genres []string `json:"genres,omitempty"`
	SortBy string   `json:"sort_by,omitempty"`
}

// PluginMeta is the metadata a plugin optionally declares in its Init response.
type PluginMeta struct {
	Name             string  `json:"name,omitempty"`
	SiteURL          string  `json:"site_url,omitempty"`
	Logo             string  `json:"logo,omitempty"`
	VerifyURL        string  `json:"verify_url,omitempty"`
	NeedsHumanVerify bool    `json:"needs_human_verify,omitempty"`
	ThumbRatio       float64 `json:"thumb_ratio,omitempty"`
	NeedsJS          bool    `json:"needs_js,omitempty"`
	SearchPageSize   int     `json:"search_page_size,omitempty"`
}

// HTTPRequest is the request payload a plugin passes to the host-imported
// host_http_request function.
type HTTPRequest struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
}

// HTTPResponse is the response payload the host returns from
// host_http_request.
type HTTPResponse struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
}
