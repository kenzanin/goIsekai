// Package enrich provides a registry of enrichment providers that fetch
// metadata for library manga — titles, summaries, categories, authors, and
// related manga — from external sources.
//
// Providers are resolved by a string source identifier (e.g. "mangadex").
// Every provider is plugin-declared: a plugin advertises its sources in
// PLUGIN.enrichment_providers and exports GetEnrichment. The host ships no
// built-in provider, so adding or changing a source is a plugin edit rather
// than a host rebuild.
//
// The host never makes network calls directly — a provider runs inside a
// plugin VM, whose host.http.* helpers route through internal/hostnet so the
// host's TLS fingerprinting, cookie jar, and pacing apply uniformly.
package enrich

import (
	"context"
	"net/http"
)

// Kind is an enrichment category.
type Kind string

const (
	KindTitles     Kind = "titles"
	KindSummaries  Kind = "summaries"
	KindCategories Kind = "categories"
	KindRelated    Kind = "related"
	KindAuthors    Kind = "authors"
	// KindCovers carries alternative cover image URLs as Item.Value.
	KindCovers Kind = "covers"
)

// Item is one enrichment record. Every kind shares this shape because they
// are all small text/graph records. The JSON tags are the plugin-facing wire
// contract for a GetEnrichment response.
type Item struct {
	Value    string `json:"value"`     // primary text (title/summary/category/author name)
	URL      string `json:"url"`       // link to the source page
	CoverURL string `json:"cover_url"` // cover image URL (for related manga)
	Source   string `json:"source"`    // provider source label (also set by the host)
}

// Provider is the interface every enrichment provider implements.
type Provider interface {
	// ID returns the stable source identifier (e.g. "mangadex").
	ID() string
	// Name returns the display label shown in the UI (e.g. "MangaDex").
	Name() string
	// Kinds returns the subset of enrichment kinds this provider supports.
	Kinds() []Kind
	// Precedence indicates the order this source runs; lower values run first.
	// Default to highest value (runs last) when not specified.
	Precedence() int
	// Enabled indicates whether this source is active.
	Enabled() bool
	// Fetch fetches items for the given kind by searching with the title.
	// The ctx may carry a timeout.
	Fetch(ctx context.Context, httpc *http.Client, title string, k Kind) ([]Item, error)
}

// CatalogEntry is one source visible in the enrichment catalog.
type CatalogEntry struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Kinds      []Kind `json:"kinds"`
	Precedence int    `json:"precedence,omitempty"`
	Enabled    bool   `json:"enabled"`
}
