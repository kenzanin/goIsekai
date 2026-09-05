// Package types defines the shared, versioned ABI contract between the host
// application and every manga source plugin. Plugins communicate with the host
// using JSON strings passed over WASM memory; these types define the wire shape
// of that JSON. Plugin authors may import this package to guarantee contract
// compatibility.
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

// AltTitleServer describes a single lookup server offered by a plugin.
type AltTitleServer struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// AltTitlesResult is what a GetAltTitles provider returns: the provider's own
// display name (reported by the plugin, never hardcoded by the host) plus the
// alternative title list.
type AltTitlesResult struct {
	Source string   `json:"source"`
	Titles []string `json:"titles"`
}

// PluginMeta is the metadata a plugin optionally declares in its Init response.
// All fields are optional; plugins without Init (or an empty response)
// contribute zero values and the host falls back to defaults (0 thumb_ratio →
// 2:3 aspect ratio in the UI).
type PluginMeta struct {
	// VerifyURL is where the user must go to solve a challenge for this source.
	VerifyURL string `json:"verify_url,omitempty"`
	// NeedsHumanVerify reports that this source's site shows a challenge that
	// requires a one-time manual verification.
	NeedsHumanVerify bool `json:"needs_human_verify,omitempty"`
	// ThumbRatio is the cover width/height ratio (e.g. 0.703) used by the UI
	// to reserve cover space before images load. 0 means use the 2:3 default.
	ThumbRatio float64 `json:"thumb_ratio,omitempty"`
	// NeedsJS reports that this source's site renders client-side and should be
	// routed through the browser engine (when configured) instead of the fast
	// path. Analogous to NeedsHumanVerify but for JS-capable engines.
	NeedsJS bool `json:"needs_js,omitempty"`
	// SearchPageSize is the number of results a single search page returns.
	// The host uses it to decide whether a "Next" pagination link should be
	// shown (a full page implies more results). 0 falls back to 24.
	SearchPageSize int `json:"search_page_size,omitempty"`
	// AltTitleServers lists the lookup servers this plugin provides for
	// alternative-title resolution. A non-empty list signals enricher
	// capability — the host also expects a GetAltTitles export.
	AltTitleServers []AltTitleServer `json:"alt_title_servers,omitempty"`
}

// HTTPRequest is the request payload a plugin passes to the host-imported
// host_http_request function.
type HTTPRequest struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
}

// HTTPResponse is the response payload the host returns to a plugin from
// host_http_request.
type HTTPResponse struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
}
