package pluginmanager

import (
	"context"
	"encoding/json"
	"fmt"
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
	pluginID string
	id       string
	name     string
	kinds    []enrich.Kind
	fetch    enrichmentFetcher
}

func (p *pluginProvider) ID() string           { return p.id }
func (p *pluginProvider) Name() string         { return p.name }
func (p *pluginProvider) Kinds() []enrich.Kind { return p.kinds }

// Fetch calls the plugin's GetEnrichment export.
func (p *pluginProvider) Fetch(_ context.Context, _ *http.Client, title string, k enrich.Kind) ([]enrich.Item, error) {
	if p.fetch == nil {
		return nil, fmt.Errorf("enrichment: plugin provider not yet wired to manager")
	}
	return p.fetch.GetEnrichment(p.pluginID, title, string(k), p.id)
}

// AltTitleServerEntry is one row in the aggregated server list: the plugin
// that provides the server, the server id, and the human-readable name.
type AltTitleServerEntry struct {
	ProviderPluginID string
	ServerID         string
	Name             string
	Kind             string
}

// ensureMetaLoaded instantiates every registered plugin that is not yet
// loaded so its metadata (AltTitleServers and friends) is visible. Plugin
// load is lazy elsewhere; the provider-lookup paths need full visibility to
// find which plugin serves a given server after a cold start.
func (m *Manager) ensureMetaLoaded() {
	m.mu.RLock()
	var ids []string
	for id, p := range m.plugins {
		if !p.loaded {
			ids = append(ids, id)
		}
	}
	m.mu.RUnlock()
	for _, id := range ids {
		if err := m.ensureLoaded(id); err != nil {
			logger.Warn("meta load", "id", id, "error", err)
		}
	}
}

// AltTitleServers iterates all discovered plugins and returns every declared
// alt-title server. Deferred plugins are loaded on demand so their metadata
// is visible even after a cold start.
func (m *Manager) AltTitleServers() []AltTitleServerEntry {
	// Ensure every deferred plugin exposes its metadata.
	m.ensureMetaLoaded()
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []AltTitleServerEntry
	for id, p := range m.plugins {
		for _, s := range p.meta.AltTitleServers {
			out = append(out, AltTitleServerEntry{
				ProviderPluginID: id,
				ServerID:         s.ID,
				Name:             s.Name,
				Kind:             s.Kind,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ProviderPluginID != out[j].ProviderPluginID {
			return out[i].ProviderPluginID < out[j].ProviderPluginID
		}
		return out[i].ServerID < out[j].ServerID
	})
	return out
}

// titlesKind reports whether a server kind is titles-capable (empty,
// "titles", or "both").
func titlesKind(kind string) bool {
	return kind == "" || kind == "titles" || kind == "both"
}

// summariesKind reports whether a server kind is summaries-capable
// ("summaries" or "both").
func summariesKind(kind string) bool {
	return kind == "summaries" || kind == "both"
}

// GetAltTitles calls the provider plugin that advertises the given server to
// resolve alternative titles. The input JSON sent to the plugin is
// {"title":..., "server":...} per the ABI contract.
func (m *Manager) GetAltTitles(title, server string) (types.AltTitlesResult, error) {
	// Find the provider plugin for this server (must be titles-capable).
	m.ensureMetaLoaded()
	m.mu.RLock()
	var providerID string
	for id, p := range m.plugins {
		for _, s := range p.meta.AltTitleServers {
			if s.ID == server && titlesKind(s.Kind) {
				providerID = id
				break
			}
		}
		if providerID != "" {
			break
		}
	}
	m.mu.RUnlock()

	if providerID == "" {
		return types.AltTitlesResult{}, fmt.Errorf("server %q not found in any provider", server)
	}

	p, err := m.get(providerID)
	if err != nil {
		return types.AltTitlesResult{}, err
	}

	input, err := json.Marshal(map[string]string{"title": title, "server": server})
	if err != nil {
		return types.AltTitlesResult{}, err
	}
	out, err := m.call(p, types.GetAltTitlesFunc, string(input))
	if err != nil {
		return types.AltTitlesResult{}, err
	}

	var res types.AltTitlesResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		return types.AltTitlesResult{}, fmt.Errorf("alt-titles decode: %w", err)
	}
	if res.Source == "" {
		res.Source = providerID
	}
	return res, nil
}

// GetAltSummaries calls the provider plugin that advertises the given server
// to resolve alternative summaries. The input JSON sent to the plugin is
// {"title":..., "server":...} per the ABI contract.
func (m *Manager) GetAltSummaries(title, server string) (types.AltSummaryResult, error) {
	// Find the provider plugin for this server (must be summaries-capable).
	m.ensureMetaLoaded()
	m.mu.RLock()
	var providerID string
	for id, p := range m.plugins {
		for _, s := range p.meta.AltTitleServers {
			if s.ID == server && summariesKind(s.Kind) {
				providerID = id
				break
			}
		}
		if providerID != "" {
			break
		}
	}
	m.mu.RUnlock()

	if providerID == "" {
		return types.AltSummaryResult{}, fmt.Errorf("server %q not found in any summaries provider", server)
	}

	p, err := m.get(providerID)
	if err != nil {
		return types.AltSummaryResult{}, err
	}

	input, err := json.Marshal(map[string]string{"title": title, "server": server})
	if err != nil {
		return types.AltSummaryResult{}, err
	}
	out, err := m.call(p, types.GetAltSummaryFunc, string(input))
	if err != nil {
		return types.AltSummaryResult{}, err
	}

	var res types.AltSummaryResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		return types.AltSummaryResult{}, fmt.Errorf("alt-summaries decode: %w", err)
	}
	if res.Source == "" {
		res.Source = providerID
	}
	return res, nil
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
