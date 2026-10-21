package pluginmanager

import "goisekai/pkg/types"

// params.go holds the values a plugin declared in the "params" object of its
// own plugin.json, so a plugin can read the facts that change (the site URL,
// a CDN host, a user agent) without editing its code.
//
// The registry is a LEAF lock, for the same reason challengeMu is: a plugin's
// own code runs while the manager's mu is held (Install and discovery both
// load a plugin under it), and sync.RWMutex is not reentrant, so a lookup
// that took mu here would self-deadlock the host on every install.

// setParams records the params a plugin declared in its plugin.json, with the
// metadata the plugin already carried as the base: a plugin reading
// host.plugin.param("site_url") gets the one site_url from plugin.json, so a
// URL is declared once and not twice (a second copy would be a second fact to
// keep in sync, which is how a plugin ends up pointing at the wrong host).
func (m *Manager) setParams(id string, meta types.PluginMeta) {
	params := make(types.PluginParams, len(meta.Params)+1)
	for k, v := range meta.Params {
		params[k] = v
	}
	if meta.SiteURL != "" {
		if _, ok := params["site_url"]; !ok {
			params["site_url"] = meta.SiteURL
		}
	}
	m.paramsMu.Lock()
	defer m.paramsMu.Unlock()
	if m.params == nil {
		m.params = make(map[string]types.PluginParams)
	}
	m.params[id] = params
}

// loadParams reads the plugin's own plugin.json and records what it declared.
// It runs before the plugin's code, so a plugin can read a param at load time
// as well as during a call.
func (m *Manager) loadParams(id, dir string) {
	m.setParams(id, readPluginJSONMeta(dir))
}

// pluginParam returns the value the plugin declared under name, or "" when it
// declared none. An unknown name is not an error: a plugin that asks for a
// param its own plugin.json does not carry gets an empty string, the same as
// a host helper that returns "" for no result.
func (m *Manager) pluginParam(id, name string) string {
	m.paramsMu.Lock()
	defer m.paramsMu.Unlock()
	return m.params[id][name]
}
