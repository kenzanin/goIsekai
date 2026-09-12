// Package enrich provides a registry of enrichment providers that fetch
// metadata for library manga — titles, summaries, categories, and related
// manga — from external sources. Built-in providers (MangaDex, MangaUpdates)
// are shipped with the host; plugins may also declare custom providers.
//
// Providers are resolved by a string source identifier (e.g. "mangadex").
// The registry dispatches to the built-in provider or a plugin-declared one.
// The host never makes network calls directly — every provider must use
// internal/hostnet so the host's TLS fingerprinting, cookie jar, and pacing
// apply uniformly.
package enrich

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"sync"
)

// Kind is an enrichment category.
type Kind string

const (
	KindTitles     Kind = "titles"
	KindSummaries  Kind = "summaries"
	KindCategories Kind = "categories"
	KindRelated    Kind = "related"
)

// Item is one enrichment record. All four kinds share this shape because
// they are all small text/graph records.
type Item struct {
	Value    string // primary text (title/summary/category name)
	URL      string // link to the source page
	CoverURL string // cover image URL (for related manga)
	Source   string // provider source label (set at fetch time)
}

// Provider is the interface every enrichment provider implements.
type Provider interface {
	// ID returns the stable source identifier (e.g. "mangadex").
	ID() string
	// Name returns the display label shown in the UI (e.g. "MangaDex").
	Name() string
	// Kinds returns the subset of enrichment kinds this provider supports.
	Kinds() []Kind
	// Fetch fetches items for the given kind by searching with the title.
	// The ctx may carry a timeout. The HTTP client passed in is already
	// configured with the host's TLS fingerprinting, cookies, and pacing.
	Fetch(ctx context.Context, httpc *http.Client, title string, k Kind) ([]Item, error)
}

// CatalogEntry is one source visible in the enrichment catalog.
type CatalogEntry struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Kinds []Kind `json:"kinds"`
}

// Registry holds all known enrichment providers.
type Registry struct {
	mu     sync.RWMutex
	byID   map[string]Provider // keyed by Provider.ID()
	byKind map[Kind][]Provider // for filtering
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		byID:   make(map[string]Provider),
		byKind: make(map[Kind][]Provider),
	}
}

// Register adds a provider to the registry. If a provider with the same ID
// already exists, the new one is ignored (built-ins registered first take
// precedence).
func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, dup := r.byID[p.ID()]; dup {
		return
	}
	r.byID[p.ID()] = p
	for _, k := range p.Kinds() {
		r.byKind[k] = append(r.byKind[k], p)
	}
}

// Catalog returns all registered sources, optionally filtered by kind.
func (r *Registry) Catalog(kind Kind) []CatalogEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var entries []CatalogEntry
	if kind != "" {
		for _, p := range r.byKind[kind] {
			entries = append(entries, CatalogEntry{
				ID:    p.ID(),
				Name:  p.Name(),
				Kinds: p.Kinds(),
			})
		}
	} else {
		for _, p := range r.byID {
			entries = append(entries, CatalogEntry{
				ID:    p.ID(),
				Name:  p.Name(),
				Kinds: p.Kinds(),
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
	for _, kind := range p.Kinds() {
		if kind == k {
			items, err := p.Fetch(ctx, httpc, title, k)
			if err != nil {
				return nil, fmt.Errorf("enrich: fetch %s from %s: %w", k, source, err)
			}
			for i := range items {
				items[i].Source = source
			}
			return items, nil
		}
	}
	return nil, fmt.Errorf("enrich: source %q does not support kind %q", source, k)
}

// FetchAll fetches every kind supported by the given sources for the title.
// Returns a map[Kind][]Item. Items that fail to fetch are skipped (best-effort).
// Titles/summaries are excluded since those are handled by the existing alt-titles system.
func (r *Registry) FetchAll(ctx context.Context, httpc *http.Client, title string, sources []string) map[Kind][]Item {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make(map[Kind][]Item)
	for _, source := range sources {
		p := r.byID[source]
		if p == nil {
			continue
		}
		for _, kind := range p.Kinds() {
			if kind == KindTitles || kind == KindSummaries {
				continue
			}
			items, err := p.Fetch(ctx, httpc, title, kind)
			if err != nil {
				continue
			}
			for i := range items {
				items[i].Source = source
			}
			out[kind] = append(out[kind], items...)
		}
	}
	return out
}

// normalizeTitle strips common noise from a manga title before sending to
// upstream search APIs. Both MangaDex and MangaUpdates search tolerate
// extra whitespace and parentheses, but stripping the "( manga )" suffix
// (common on mirror sites) improves matching.
var mangaSuffixRe = regexp.MustCompile(`\s*\([^)]*[Mm][Aa][Nn][Gg][Aa][^)]*\)$`)

func normalizeTitle(t string) string {
	t = strings.TrimSpace(t)
	for {
		before := mangaSuffixRe.ReplaceAllString(t, "")
		if before == t {
			break
		}
		t = before
	}
	return strings.TrimSpace(t)
}
