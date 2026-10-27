package httpserver

import (
	"errors"
	"net/http"
	"slices"
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
	// genre is repeatable: ?genre=a&genre=b. The first value drives the
	// single-select UI; every value goes to the plugin, whose SearchFilter
	// already accepts a list.
	genreValues := r.URL.Query()["genre"]
	genre := ""
	if len(genreValues) > 0 {
		genre = genreValues[0]
	}
	filterGenres := slices.DeleteFunc(genreValues, func(g string) bool { return g == "" })
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	// The plugin's metadata decides who paginates, so read it before searching.
	var pluginMeta pluginmanager.LoadedPlugin
	if pluginID != "" {
		pluginMeta = s.service.PluginMeta(pluginID)
	}
	filter := types.SearchFilter{Query: q, Page: page, Genres: filterGenres}
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
			if pluginMeta.NeedsHumanVerify {
				verifyState, hasVerify, verifyErr := s.service.GetPluginVerifyState(pluginID)
				if verifyErr == nil && (!hasVerify || verifyState.Cookies == "") {
					challenge = true
					results = nil
				} else {
					results, err = s.service.SearchManga(pluginID, filter)
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
				results, err = s.service.SearchManga(pluginID, filter)
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
			results, err = s.service.SearchManga(pluginID, filter)
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
	// Who paginates depends on the plugin. The default contract is that a
	// plugin returns every match and the host slices out one page. A plugin
	// that declares paginates:true has already asked the remote source for just
	// this page, so slicing again would skip rows that were never returned:
	// page 2 of a 12-result search came back empty because the host dropped the
	// first 30 rows of an already-paged 12-row answer.
	total := len(results)
	start, end := 0, total
	hasNext := false
	if pluginMeta.Paginates {
		// The plugin pages upstream and does not report a total. A result set
		// can fill the page (→ a next page may exist) or be short (→ we are on
		// the last page). Using len(result)>0 always would surface one empty
		// "next" page after the last non-empty one (e.g. 1 result → page 2
		// link that returns []), so gate on whether the page looks full.
		// ponytail: add a has_next field to the plugin ABI when an extra click
		// starts mattering.
		hasNext = total >= pageSize
	} else {
		start = min((page-1)*pageSize, total)
		end = min(start+pageSize, total)
		hasNext = end < total
	}
	// Version the cover URLs so a browser cache entry keyed on the bare /image
	// URL cannot keep serving a stale image. /image answers with a week-long
	// max-age and its URL carries no version, so a cover cached under the wrong
	// bytes for that URL survives a fix until the entry expires. The detail page
	// already appends cb; the results grid did not.
	coverVersions := make(map[string]string, end-start)
	for _, r := range results[start:end] {
		if r.CoverURL == "" {
			continue
		}
		if v := s.service.CoverCacheVersion(pluginID, r.CoverURL); v != "" {
			coverVersions[r.CoverURL] = v
		}
	}
	pluginName := pluginID
	pluginIcon := ""
	var verifyRow database.PluginVerifyRow
	if pluginID != "" {
		names, icons := s.pluginDisplayMaps()
		if name := names[pluginID]; name != "" {
			pluginName = name
		}
		pluginIcon = icons[pluginID]
		verifyRow, _, _ = s.service.GetPluginVerifyState(pluginID)
	}
	// TotalPages drives the numbered pager. A host-paged search knows its total;
	// a self-paging plugin does not report one, so offer just the next page when
	// there is one rather than inventing a count.
	totalPages := max((total+pageSize-1)/pageSize, 1)
	if pluginMeta.Paginates {
		totalPages = page
		if hasNext {
			totalPages = page + 1
		}
	}
	s.renderPage(w, r, "views/search", "search", map[string]any{
		"Plugins":          plugins,
		"Q":                q,
		"PluginID":         pluginID,
		"Genres":           genres,
		"Genre":            genre,
		"GenresSelected":   filterGenres,
		"PluginName":       pluginName,
		"PluginIcon":       pluginIcon,
		"Results":          results[start:end],
		"CoverVersions":    coverVersions,
		"Page":             page,
		"TotalPages":       totalPages,
		"HasNext":          hasNext,
		"ThumbRatio":       pluginMeta.ThumbRatio,
		"Challenge":        challenge,
		"NeedsHumanVerify": pluginMeta.NeedsHumanVerify,
		"VerifyURL":        pluginMeta.VerifyURL,
		"VerifyCookies":    verifyRow.Cookies,
		"VerifyUserAgent":  verifyRow.UserAgent,
	})
}
