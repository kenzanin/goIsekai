package httpserver

import (
	"errors"
	"net/http"
	"strconv"

	"goisekai/internal/bridge"
	"goisekai/internal/database"
	"goisekai/internal/hostnet"
	"goisekai/internal/pluginmanager"
	"goisekai/pkg/types"
)

// searchPageSize is the number of results per search page. 30 fills the
// 6-column results grid with 5 rows.
const searchPageSize = 30

// viewSearch renders the search form and, when q+pluginID are present, results.
func (s *Server) viewSearch(w http.ResponseWriter, r *http.Request) {
	plugins, err := s.service.ListPlugins()
	if err != nil {
		s.logger.Error("plugin list", "error", err)
	}
	q := r.URL.Query().Get("q")
	pluginID := r.URL.Query().Get("pluginID")
	if pluginID == "" {
		// Default to the first active plugin so the genre picker renders on a
		// fresh page load, not only once a plugin has been submitted.
		for _, p := range plugins {
			if p.IsActive {
				pluginID = p.ID
				break
			}
		}
	}
	genre := r.URL.Query().Get("genre")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	var genres []bridge.Genre
	if pluginID != "" {
		genres, _ = s.service.ListGenres(pluginID)
	}
	var results []types.Manga
	challenge := false
	if q != "" || genre != "" {
		if pluginID != "" {
			// Human verification wizard check: if the plugin needs human
			// verification and no cookies are saved yet, render the wizard
			// immediately without calling the plugin (avoids CDP timeout).
			// Once cookies exist, fall through to the normal search path below so
			// the saved cookies are used by the proxy.
			if pluginMeta := s.service.PluginMeta(pluginID); pluginMeta.NeedsHumanVerify {
				verifyState, hasVerify, verifyErr := s.service.GetPluginVerifyState(pluginID)
				if verifyErr == nil && (!hasVerify || verifyState.Cookies == "") {
					challenge = true
					results = nil
				} else {
					results, err = s.service.SearchManga(pluginID, types.SearchFilter{Query: q, Page: page, Genres: []string{genre}})
					if err != nil {
						if _, ok := errors.AsType[*hostnet.ChallengeError](err); ok {
							challenge = true
							results = nil
							s.logger.Warn("search blocked by challenge", "plugin", pluginID, "q", q)
						} else {
							s.logger.Error("search", "error", err, "plugin", pluginID, "q", q)
						}
					}
				}
			} else {
				results, err = s.service.SearchManga(pluginID, types.SearchFilter{Query: q, Page: page, Genres: []string{genre}})
				if err != nil {
					if _, ok := errors.AsType[*hostnet.ChallengeError](err); ok {
						challenge = true
						results = nil
						s.logger.Warn("search blocked by challenge", "plugin", pluginID, "q", q)
					} else {
						s.logger.Error("search", "error", err, "plugin", pluginID, "q", q)
					}
				}
			}
		} else {
			results, err = s.service.SearchManga(pluginID, types.SearchFilter{Query: q, Page: page, Genres: []string{genre}})
			if err != nil {
				if _, ok := errors.AsType[*hostnet.ChallengeError](err); ok {
					challenge = true
					results = nil
					s.logger.Warn("search blocked by challenge", "plugin", pluginID, "q", q)
				} else {
					s.logger.Error("search", "error", err, "plugin", pluginID, "q", q)
				}
			}
		}
	}
	pageSize := searchPageSize
	// Host-side pagination: plugins return ALL matching results; slice out the
	// requested page here so the template never renders more than one page.
	total := len(results)
	start := min((page-1)*pageSize, total)
	end := min(start+pageSize, total)
	pluginName := pluginID
	pluginIcon := ""
	var pluginMeta pluginmanager.LoadedPlugin
	var verifyRow database.PluginVerifyRow
	if pluginID != "" {
		names, icons := s.pluginDisplayMaps()
		if name := names[pluginID]; name != "" {
			pluginName = name
		}
		pluginIcon = icons[pluginID]
		// Human verification wizard data: PluginMeta self-loads the plugin so
		// the NeedsHumanVerify flag is visible before any search runs.
		pluginMeta = s.service.PluginMeta(pluginID)
		verifyRow, _, _ = s.service.GetPluginVerifyState(pluginID)
	}
	s.renderPage(w, r, "views/search", "search", map[string]any{
		"Plugins":          plugins,
		"Q":                q,
		"PluginID":         pluginID,
		"Genres":           genres,
		"Genre":            genre,
		"PluginName":       pluginName,
		"PluginIcon":       pluginIcon,
		"Results":          results[start:end],
		"Page":             page,
		"TotalPages":       max((total+pageSize-1)/pageSize, 1),
		"HasNext":          end < total,
		"ThumbRatio":       pluginMeta.ThumbRatio,
		"Challenge":        challenge,
		"NeedsHumanVerify": pluginMeta.NeedsHumanVerify,
		"VerifyURL":        pluginMeta.VerifyURL,
		"VerifyCookies":    verifyRow.Cookies,
		"VerifyUserAgent":  verifyRow.UserAgent,
	})
}
