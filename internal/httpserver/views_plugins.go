package httpserver

import (
	"goisekai/internal/config"
	"goisekai/internal/database"
	"net/http"
	"strings"
)

// PluginView enriches a database plugin with runtime verify metadata and the
// declared thumbnail ratio for the plugins page.
type PluginView struct {
	database.Plugin
	Loaded            bool // true once the runtime has been instantiated
	VerifyURL         string
	NeedsHumanVerify  bool
	VerifyCookies     string
	VerifyUserAgent   string
	ThumbRatio        float64 // runtime meta; shadows database.Plugin.ThumbRatio (0 for bridge-installed plugins)
	SiteURL           string
	PinnedProfile     string   // current pinned TLS profile name, "" = auto
	AvailableProfiles []string // selectable profile names for dropdown
}

// viewPlugins renders the plugin manager page.
func (s *Server) viewPlugins(w http.ResponseWriter, r *http.Request) {
	plugins, err := s.service.ListPlugins()
	if err != nil {
		s.logger.Error("plugin list", "error", err)
	}
	metas := s.service.PluginMetas()
	views := make([]PluginView, 0, len(plugins))
	for _, p := range plugins {
		v := PluginView{Plugin: p}
		if m, ok := metas[p.ID]; ok {
			v.Loaded = m.Loaded
			v.VerifyURL = m.VerifyURL
			v.NeedsHumanVerify = m.NeedsHumanVerify
			v.ThumbRatio = m.ThumbRatio
			v.SiteURL = m.SiteURL
			if m.Name != "" {
				v.Name = m.Name // override DB name if plugin declares one
			}
			if m.Logo != "" {
				v.IconURL = resolveLogoURL(m.Logo, p.ID)
			}
		}
		if row, ok, err := s.service.GetPluginVerifyState(p.ID); err == nil && ok {
			v.VerifyCookies = row.Cookies
			v.VerifyUserAgent = row.UserAgent
		}
		v.PinnedProfile, v.AvailableProfiles = s.service.PluginProfile(p.ID)
		views = append(views, v)
	}
	s.renderPage(w, r, "views/plugins", "plugins", map[string]any{"Plugins": views})
}

// viewSettings renders the current goisekai.ini values.
func (s *Server) viewSettings(w http.ResponseWriter, r *http.Request) {
	path := s.service.GetConfigPath()
	cfg, err := config.Load(path)
	if err != nil {
		s.logger.Error("config load", "error", err, "path", path)
	}
	cacheBytes, _ := s.service.CacheSize()
	s.renderPage(w, r, "views/settings", "settings", map[string]any{
		"Config":     cfg,
		"Path":       path,
		"CacheBytes": cacheBytes,
	})
}

// resolveLogoURL maps a plugin's Logo field to a URL. Bare filenames become
// /plugin-static/{id}/{file}; absolute URLs and data URIs pass through as-is.
func resolveLogoURL(logo, pluginID string) string {
	if strings.HasPrefix(logo, "http://") || strings.HasPrefix(logo, "https://") || strings.HasPrefix(logo, "data:") {
		return logo
	}
	return "/plugin-static/" + pluginID + "/" + logo
}
