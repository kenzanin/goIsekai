package pluginmanager

import (
	"sort"
	"strconv"
	"strings"

	"goisekai/internal/hostnet"
	"goisekai/pkg/types"
)

// LoadedPlugin is metadata about a currently-registered plugin.
type LoadedPlugin struct {
	ID               string
	Kind             string // runtime: "lua", "js", "go" or "yaegi"
	Version          string // ABI contract version (e.g. "1")
	Loaded           bool   // true when the runtime is instantiated
	WasmPath         string // path to the plugin directory or entry file
	VerifyURL        string // from the plugin's declared metadata
	NeedsHumanVerify bool
	ThumbRatio       float64
	NeedsJS          bool
	Name             string
	SiteURL          string
	BuiltWith        string // toolchain + version stamped at build time, "" = unknown
	Logo             string
	// Paginates reports that the plugin pages the remote source itself and
	// returns just the requested page, so the host must not slice again.
	Paginates bool
}

// LoadedPlugins returns metadata for every plugin currently registered,
// sorted by id.
func (m *Manager) LoadedPlugins() []LoadedPlugin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]LoadedPlugin, 0, len(m.plugins))
	for _, p := range m.plugins {
		if p.infoOnly {
			// An enrichment script is not a manga source: keep it out of the
			// plugin list the UI and search read.
			continue
		}
		ver := p.contractVersion
		if ver == 0 {
			// ponytail: lazy plugin not loaded yet — CheckVersion rejects
			// anything but ContractVersion at load, so 0 is never a real
			// declaration. Upgrade: persist declared version on first load.
			ver = types.ContractVersion
		}
		out = append(out, LoadedPlugin{
			ID:               p.id,
			Kind:             p.kind,
			Version:          strconv.Itoa(int(ver)),
			Loaded:           p.loaded,
			WasmPath:         p.wasmPath,
			VerifyURL:        p.meta.VerifyURL,
			NeedsHumanVerify: p.meta.NeedsHumanVerify,
			ThumbRatio:       p.meta.ThumbRatio,
			NeedsJS:          p.meta.NeedsJS,
			Name:             p.meta.Name,
			SiteURL:          p.meta.SiteURL,
			BuiltWith:        p.meta.BuiltWith,
			Logo:             p.meta.Logo,
			Paginates:        p.meta.Paginates,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// InfoPlugins returns metadata for the enrichment scripts discovered under
// infoDir, sorted by id. They stay out of LoadedPlugins (they are not manga
// sources, so search and verify must not see them), but the plugins page lists
// them separately so a user can tell which enrichment sources are installed and
// which runtime runs each one. The id is returned without the info: namespace.
func (m *Manager) InfoPlugins() []LoadedPlugin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]LoadedPlugin, 0, 4)
	for _, p := range m.plugins {
		if !p.infoOnly {
			continue
		}
		out = append(out, LoadedPlugin{
			ID:        strings.TrimPrefix(p.id, infoPrefix),
			Kind:      p.kind,
			Loaded:    p.loaded,
			WasmPath:  p.wasmPath,
			Name:      p.meta.Name,
			SiteURL:   p.meta.SiteURL,
			BuiltWith: p.meta.BuiltWith,
			Logo:      p.meta.Logo,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Proxy returns the internal proxy (for testing/debugging only)
func (m *Manager) Proxy() *hostnet.Proxy {
	return m.proxy
}

// SiteURL returns the plugin's declared site_url, or "" when the plugin is
// unknown or declared none. Nil-safe: test services run without a manager.
func (m *Manager) SiteURL(id string) string {
	if m == nil {
		return ""
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if p, ok := m.plugins[id]; ok {
		return p.meta.SiteURL
	}
	return ""
}
