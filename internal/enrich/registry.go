package enrich

import (
	"slices"
	"sort"
	"sync"
)

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

// OrderedProviders returns all enabled providers sorted by precedence (ascending),
// then by ID for determinism.
func (r *Registry) OrderedProviders() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providers := make([]Provider, 0, len(r.order))
	for _, id := range r.order {
		if p := r.byID[id]; p != nil && p.Enabled() {
			providers = append(providers, p)
		}
	}
	return providers
}
