package pluginmanager

import (
	"context"
	"fmt"
	"github.com/goccy/go-json"
	"goisekai/internal/enrich"
	"goisekai/internal/logger"
	"goisekai/pkg/types"
	"net/http"
	"sort"
)

// enrichmentFetcher is the interface the Manager implements so that
// pluginProvider can call plugin GetEnrichment exports at fetch time.
type enrichmentFetcher interface {
	GetEnrichment(pluginID, title, kind, source string) ([]enrich.Item, error)
}

// pluginProvider bridges the enrich.Provider interface to a plugin's
// GetEnrichment export.
type pluginProvider struct {
	pluginID   string
	id         string
	name       string
	kinds      []enrich.Kind
	fetch      enrichmentFetcher
	precedence int
	enabled    bool
}

func (p *pluginProvider) ID() string           { return p.id }
func (p *pluginProvider) Name() string         { return p.name }
func (p *pluginProvider) Kinds() []enrich.Kind { return p.kinds }
func (p *pluginProvider) Precedence() int      { return p.precedence }
func (p *pluginProvider) Enabled() bool        { return p.enabled }

// Fetch calls the plugin's GetEnrichment export.
func (p *pluginProvider) Fetch(_ context.Context, _ *http.Client, title string, k enrich.Kind) ([]enrich.Item, error) {
	if p.fetch == nil {
		return nil, fmt.Errorf("enrichment: plugin provider not yet wired to manager")
	}
	return p.fetch.GetEnrichment(p.pluginID, title, string(k), p.id)
}

// ensureInfoLoaded instantiates every enrichment script that has not been
// loaded yet, so the providers they declare appear in the catalog. Scraper
// plugins are left alone: reading the catalog must not pay for every source VM.
func (m *Manager) ensureInfoLoaded() {
	m.mu.RLock()
	var ids []string
	for id, p := range m.plugins {
		if p.infoOnly && !p.loaded {
			ids = append(ids, id)
		}
	}
	m.mu.RUnlock()
	sort.Strings(ids)
	for _, id := range ids {
		if err := m.ensureLoaded(id); err != nil {
			logger.Warn("info script load", "id", id, "error", err)
		}
	}
}

// LoadEnrichmentProviders makes every discovered enrichment script declare its
// providers. Call it before reading the enrichment catalog.
func (m *Manager) LoadEnrichmentProviders() {
	m.ensureInfoLoaded()
}

// GetEnrichment calls the plugin's GetEnrichment export and returns
// enrichment items for the given kind. The source parameter identifies
// which enrichment provider within the plugin to use.
func (m *Manager) GetEnrichment(pluginID, title, kind, source string) ([]enrich.Item, error) {
	p, err := m.get(pluginID)
	if err != nil {
		return nil, err
	}

	input, err := json.Marshal(map[string]string{
		"title":  title,
		"kind":   kind,
		"source": source,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal enrichment request: %w", err)
	}

	out, err := m.call(p, types.GetEnrichmentFunc, string(input))
	if err != nil {
		return nil, fmt.Errorf("plugin enrichment call: %w", err)
	}

	var items []enrich.Item
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		return nil, fmt.Errorf("enrichment decode: %w", err)
	}
	for i := range items {
		items[i].Source = source
	}
	return items, nil
}
