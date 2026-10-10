package pluginmanager

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

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
	data, _ := json.Marshal(func() []string {
		r := make([]string, 0, len(hosts))
		for h := range hosts {
			r = append(r, h)
		}
		return r
	}())
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
