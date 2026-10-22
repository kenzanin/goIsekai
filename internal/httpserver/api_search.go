package httpserver

import (
	"errors"
	"net/http"
	"slices"
	"strconv"

	"goisekai/internal/hostnet"
	"goisekai/pkg/types"
)

// apiSearchResponse is the JSON shape for GET /search.
type apiSearchResponse struct {
	Results []types.Manga `json:"results"`
	HasNext bool          `json:"has_next"`
	Page    int           `json:"page"`
}

// apiSearch mirrors viewSearch: host-side pagination with searchPageSize.
func (s *Server) apiSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	pluginID := r.URL.Query().Get("pluginID")
	// genre is repeatable: ?genre=a&genre=b. The first value only decides
	// whether the request counts as filtered; all of them reach the plugin.
	genreValues := r.URL.Query()["genre"]
	genre := ""
	if len(genreValues) > 0 {
		genre = genreValues[0]
	}
	if (q == "" && genre == "") || pluginID == "" {
		writeErr(w, http.StatusBadRequest, "missing required query parameters: q or genre, pluginID")
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	results, err := s.service.SearchManga(pluginID, types.SearchFilter{
		Query:  q,
		Page:   page,
		Genres: slices.DeleteFunc(genreValues, func(g string) bool { return g == "" }),
	})
	if err != nil {
		if _, ok := errors.AsType[*hostnet.ChallengeError](err); ok {
			writeErr(w, http.StatusForbidden, "source requires verification")
			return
		}
		if s.serviceQueueFull(w, err, "search") {
			return
		}
		s.logger.Error("api search", "error", err, "plugin", pluginID, "q", q)
		writeErr(w, http.StatusBadGateway, "search failed: "+err.Error())
		return
	}
	s.service.SyncPluginMeta(pluginID)
	pageSize := searchPageSize
	total := len(results)
	// Same contract split as viewSearch: a plugin that declares paginates:true
	// already returned just this page, so the host must not slice it again.
	if s.service.PluginMeta(pluginID).Paginates {
		// Same rule as viewSearch: the plugin reports no total, so any results
		// mean a next page may exist.
		writeJSON(w, http.StatusOK, apiSearchResponse{
			Results: results,
			HasNext: total > 0,
			Page:    page,
		})
		return
	}
	start := min((page-1)*pageSize, total)
	end := min(start+pageSize, total)
	writeJSON(w, http.StatusOK, apiSearchResponse{
		Results: results[start:end],
		HasNext: end < total,
		Page:    page,
	})
}
