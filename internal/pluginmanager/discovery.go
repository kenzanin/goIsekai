package pluginmanager

import (
	"encoding/json"
	"os"
	"strings"
	"net/url"
	"goisekai/internal/logger"
	"path/filepath"
)

// Discover scans pluginsDir and registers every folder containing main.lua,
// main.js, or main.go, WITHOUT instantiating any runtime. Plugins are lazily
// instantiated on first use via ensureLoaded. A folder that collides with an
// already-registered id is logged and skipped rather than aborting discovery.
func (m *Manager) Discover() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Lua plugins: one folder per plugin, main.lua entry, folder name = id.
	luaMatches, err := filepath.Glob(filepath.Join(m.pluginsDir, "*", "main.lua"))
	if err != nil {
		return err
	}
	for _, path := range luaMatches {
		id := filepath.Base(filepath.Dir(path))
		if _, dup := m.plugins[id]; dup {
			logger.Error("plugin id collision, skipping", "id", id, "kind", "lua")
			continue
		}
		m.plugins[id] = &loadedPlugin{
			id:       id,
			wasmPath: filepath.Dir(path),
			kind:     "lua",
		}
		logger.Info("lua plugin registered", "id", id, "path", path)
	}

	// JS plugins: one folder per plugin, main.js entry, folder name = id.
	jsMatches, err := filepath.Glob(filepath.Join(m.pluginsDir, "*", "main.js"))
	if err != nil {
		return err
	}
	for _, path := range jsMatches {
		id := filepath.Base(filepath.Dir(path))
		if _, dup := m.plugins[id]; dup {
			logger.Error("plugin id collision, skipping", "id", id, "kind", "js")
			continue
		}
		m.plugins[id] = &loadedPlugin{
			id:       id,
			wasmPath: filepath.Dir(path),
			kind:     "js",
		}
		logger.Info("js plugin registered", "id", id, "path", path)
	}

	// Yaegi plugins: one folder per plugin, main.go entry, folder name = id.
	yaegiMatches, err := filepath.Glob(filepath.Join(m.pluginsDir, "*", "main.go"))
	if err != nil {
		return err
	}
	for _, path := range yaegiMatches {
		id := filepath.Base(filepath.Dir(path))
		if _, dup := m.plugins[id]; dup {
			logger.Error("plugin id collision, skipping", "id", id, "kind", "yaegi")
			continue
		}
		m.plugins[id] = &loadedPlugin{
			id:       id,
			wasmPath: filepath.Dir(path),
			kind:     "yaegi",
		}
		logger.Info("yaegi plugin registered", "id", id, "path", path)
	}

	// Go native plugins: one .so file per plugin, filename (minus .so) = id.
	if err := m.discoverGo(); err != nil {
		return err
	}
	return nil
}

// trackKnownHosts extracts hosts from plugin metadata and writes them to known_hosts.json.
func (m *Manager) TrackKnownHosts() {
	if m.pluginsDir == "" {
		return
	}
	
	hosts := make(map[string]bool)
	
	// Scan plugin directories for site_url in metadata
	// Lua/JS plugins: try to read plugin.json
	luaMatches, _ := filepath.Glob(filepath.Join(m.pluginsDir, "*", "plugin.json"))
	for _, path := range luaMatches {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var meta struct {
			SiteURL string `json:"site_url"`
		}
		if err := json.Unmarshal(data, &meta); err == nil && meta.SiteURL != "" {
			if u, err := url.Parse(meta.SiteURL); err == nil && u.Host != "" {
				hosts[u.Host] = true
			}
		}
	}
	
	// JS plugins: parse main.js for PLUGIN object
	jsMatches, _ := filepath.Glob(filepath.Join(m.pluginsDir, "*", "main.js"))
	for _, path := range jsMatches {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		// Extract site_url from PLUGIN JSON
		if strings.Contains(string(data), "site_url") {
			if u, err := url.Parse(string(data)); err == nil && u.Host != "" {
				hosts[u.Host] = true
			}
		}
	}
	
	// Save to known_hosts.json
	data, _ := json.Marshal(func() []string { r := make([]string, 0, len(hosts)); for h := range hosts { r = append(r, h) }; return r }())
	_ = os.WriteFile(filepath.Join(os.Getenv("HOME"), "app_data", "known_hosts.json"), data, 0644)
}

// GetKnownHosts returns the list of known hosts from known_hosts.json
func (m *Manager) GetKnownHosts() []string {
	data, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), "app_data", "known_hosts.json"))
	if err != nil {
		return nil
	}
	var hosts []string
	_ = json.Unmarshal(data, &hosts)
	return hosts
}
