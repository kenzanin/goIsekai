package pluginmanager

import (
	"encoding/json"
	"fmt"
	"goisekai/internal/logger"
	"goisekai/pkg/types"
	"sort"
)

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
