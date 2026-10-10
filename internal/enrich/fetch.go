package enrich

import (
	"context"
	"fmt"
	"net/http"
	"slices"

	"goisekai/internal/logger"
)

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
// empty passes the kind on to the next one. When sources is empty, every
// enabled provider walks in precedence order.
func (r *Registry) FetchFirst(ctx context.Context, httpc *http.Client, title string, sources []string) map[Kind][]Item {
	if len(sources) == 0 {
		for _, p := range r.OrderedProviders() {
			sources = append(sources, p.ID())
		}
	}
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

// FetchAll fetches items for a kind from every enabled provider that supports it,
// returning a map from provider source to items. Unlike FetchFirst, this gathers
// from all sources and doesn't stop at the first winner. Errors are logged but
// don't prevent other providers from being queried.
func (r *Registry) FetchAll(ctx context.Context, httpc *http.Client, title string, kinds []Kind) map[Kind][]Item {
	out := make(map[Kind][]Item)
	r.mu.RLock()
	for _, id := range r.order {
		p := r.byID[id]
		if p == nil || !p.Enabled() {
			continue
		}
		for _, kind := range p.Kinds() {
			// Check if this kind was requested
			found := slices.Contains(kinds, kind)
			if !found {
				continue
			}
			items, err := p.Fetch(ctx, httpc, title, kind)
			if err != nil {
				logger.Debug("enrich fetch failed", "source", id, "kind", string(kind), "error", err)
				continue
			}
			if len(items) == 0 {
				logger.Debug("enrich fetch empty", "source", id, "kind", string(kind))
				continue
			}
			for i := range items {
				items[i].Source = id
			}
			out[kind] = append(out[kind], items...)
			logger.Debug("enrich fetch ok", "source", id, "kind", string(kind), "count", len(items))
		}
	}
	r.mu.RUnlock()
	return out
}
