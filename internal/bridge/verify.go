package bridge

import (
	"fmt"
	"net/url"

	"goisekai/internal/database"
	"goisekai/internal/pluginmanager"
)

// SavePluginVerify stores pasted verification cookies and an optional
// User-Agent for a plugin: the hostnet client for the plugin's verify domain
// is re-seeded immediately, and the credentials persist in the database so
// they survive a restart. A plugin without a declared verify URL skips the
// client seeding but still stores the row.
func (s *AppService) SavePluginVerify(pluginID, cookies, userAgent string) error {
	meta := s.PluginMeta(pluginID)
	if domain := verifyHost(meta.VerifyURL); domain != "" {
		if err := s.proxy.SetVerifyCookies(pluginID, domain, cookies, userAgent); err != nil {
			return fmt.Errorf("bridge: save plugin verify: %w", err)
		}
	}
	if err := s.db.UpsertPluginVerify(database.PluginVerifyRow{
		PluginID:  pluginID,
		VerifyURL: meta.VerifyURL,
		Cookies:   cookies,
		UserAgent: userAgent,
	}); err != nil {
		return fmt.Errorf("bridge: save plugin verify: %w", err)
	}
	return nil
}

// GetPluginVerifyState returns the stored verification row for a plugin.
func (s *AppService) GetPluginVerifyState(pluginID string) (database.PluginVerifyRow, bool, error) {
	return s.db.GetPluginVerify(pluginID)
}

// PluginMeta returns runtime metadata (verify url, needs-human-verify, thumb
// ratio) for pluginID, or the zero value when the plugin isn't loaded.
func (s *AppService) PluginMeta(pluginID string) pluginmanager.LoadedPlugin {
	for _, m := range s.mgr.LoadedPlugins() {
		if m.ID == pluginID {
			return m
		}
	}
	return pluginmanager.LoadedPlugin{}
}

// PluginMetas returns runtime metadata for every loaded plugin keyed by id.
func (s *AppService) PluginMetas() map[string]pluginmanager.LoadedPlugin {
	out := make(map[string]pluginmanager.LoadedPlugin)
	for _, m := range s.mgr.LoadedPlugins() {
		out[m.ID] = m
	}
	return out
}

// verifyHost extracts the host (e.g. "example.com") from a verify URL, or ""
// when the URL is empty or unparsable.
func verifyHost(verifyURL string) string {
	u, err := url.Parse(verifyURL)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Host
}
