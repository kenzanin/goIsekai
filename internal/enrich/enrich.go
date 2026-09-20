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
	"fmt"
	"net/http"
	"slices"
	"sort"
	"sync"

	"goisekai/internal/logger"
)

// Kind is an enrichment category.
type Kind string

const (
	KindTitles     Kind = "titles"
	KindSummaries  Kind = "summaries"
	KindCategories Kind = "categories"
	KindRelated    Kind = "related"
	KindAuthors    Kind = "authors"
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

// Registry holds all known enrichment providers.
type Registry struct {
	mu     sync.RWMutex
	byID   map[string]Provider // keyed by Provider.ID()
	byKind map[Kind][]Provider // for filtering
	order  []string            // registration order, so the catalog is stable
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		byID:   make(map[string]Provider),
		byKind: make(map[Kind][]Provider),
	}
}

// Register adds a provider to the registry. If a provider with the same ID
// already exists, the new one is ignored (the first plugin to claim an ID
// wins, so the catalog stays stable across reloads).
// Disabled providers are skipped entirely.
func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, dup := r.byID[p.ID()]; dup {
		return
	}
	if !p.Enabled() {
		return
	}
	r.byID[p.ID()] = p
	r.order = append(r.order, p.ID())
	sort.Slice(r.order, func(i, j int) bool {
		pi := r.byID[r.order[i]]
		pj := r.byID[r.order[j]]
		if pi.Precedence() != pj.Precedence() {
			return pi.Precedence() < pj.Precedence()
		}
		return r.order[i] < r.order[j]
	})
	for _, k := range p.Kinds() {
		r.byKind[k] = append(r.byKind[k], p)
	}
}

// Catalog returns all registered sources, optionally filtered by kind.
// Results are sorted by precedence (ascending), with source ID as tiebreaker.
// Disabled sources are excluded.
func (r *Registry) Catalog(kind Kind) []CatalogEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var entries []CatalogEntry
	if kind != "" {
		for _, p := range r.byKind[kind] {
			if !p.Enabled() {
				continue
			}
			entries = append(entries, CatalogEntry{
				ID:         p.ID(),
				Name:       p.Name(),
				Kinds:      p.Kinds(),
				Precedence: p.Precedence(),
				Enabled:    p.Enabled(),
			})
		}
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].Precedence != entries[j].Precedence {
				return entries[i].Precedence < entries[j].Precedence
			}
			return entries[i].ID < entries[j].ID
		})
	} else {
		for _, id := range r.order {
			p := r.byID[id]
			entries = append(entries, CatalogEntry{
				ID:         p.ID(),
				Name:       p.Name(),
				Kinds:      p.Kinds(),
				Precedence: p.Precedence(),
				Enabled:    p.Enabled(),
			})
		}
	}
	return entries
}

// Resolve returns the provider for a given source ID.
func (r *Registry) Resolve(source string) Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byID[source]
}

// SupportsKind reports whether the provider at source supports kind k.
func (r *Registry) SupportsKind(source string, k Kind) bool {
	p := r.Resolve(source)
	if p == nil {
		return false
	}
	return slices.Contains(p.Kinds(), k)
}

// Fetch calls the provider at source for the given kind, returning items
// tagged with the provider's source label.
func (r *Registry) Fetch(ctx context.Context, httpc *http.Client, source string, title string, k Kind) ([]Item, error) {
	p := r.Resolve(source)
	if p == nil {
		return nil, fmt.Errorf("enrich: unknown source %q", source)
	}
	if !slices.Contains(p.Kinds(), k) {
		return nil, fmt.Errorf("enrich: source %q does not support kind %q", source, k)
	}
	items, err := p.Fetch(ctx, httpc, title, k)
	if err != nil {
		return nil, fmt.Errorf("enrich: fetch %s from %s: %w", k, source, err)
	}
	for i := range items {
		items[i].Source = source
	}
	return items, nil
}

// FetchFirst fetches each kind from the first source that returns anything for
// it, so one source owns a kind instead of every source merging into it. The
// source order is the precedence order; a source that errors or comes back
// empty passes the kind on to the next one.
func (r *Registry) FetchFirst(ctx context.Context, httpc *http.Client, title string, sources []string) map[Kind][]Item {
	out := make(map[Kind][]Item)
	for _, source := range sources {
		p := r.Resolve(source)
		if p == nil {
			continue
		}
		for _, kind := range p.Kinds() {
			if len(out[kind]) > 0 {
				continue
			}
			items, err := p.Fetch(ctx, httpc, title, kind)
			if err != nil {
				logger.Debug("enrich fetch failed", "source", source, "kind", string(kind), "error", err)
				continue
			}
			if len(items) == 0 {
				logger.Debug("enrich fetch empty", "source", source, "kind", string(kind))
				continue
			}
			for i := range items {
				items[i].Source = source
			}
			logger.Debug("enrich fetch ok", "source", source, "kind", string(kind), "count", len(items))
			out[kind] = items
		}
	}
	return out
}
