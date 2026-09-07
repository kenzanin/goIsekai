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

// PluginProfile returns the plugin's pinned TLS profile ("" when unpinned) and
// the selectable profile names, for the plugins-page profile UI.
func (s *AppService) PluginProfile(pluginID string) (pin string, available []string) {
	return s.proxy.PinnedProfile(pluginID), s.proxy.AvailableProfiles()
}

// TestProfile runs a single GET against url (defaulting to the plugin's site
// URL) using an explicit TLS profile or "stdlib", returning the HTTP status.
// It does not change the plugin's pinned profile.
func (s *AppService) TestProfile(pluginID, profile, url string) (int, error) {
	if url == "" {
		url = s.PluginMeta(pluginID).SiteURL
	}
	if url == "" {
		return 0, fmt.Errorf("bridge: no site URL for plugin %s", pluginID)
	}
	return s.proxy.TestProfile(pluginID, profile, url)
}

// ResetProfile clears a plugin's pinned TLS profile so it returns to the
// default and re-probes on the next challenge.
func (s *AppService) ResetProfile(pluginID string) {
	s.proxy.ClearPin(pluginID)
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
