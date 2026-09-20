package httpserver

import (
	"errors"
	"net/http"
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
	genre := r.URL.Query().Get("genre")
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
		Genres: []string{genre},
	})
	if err != nil {
		if _, ok := errors.AsType[*hostnet.ChallengeError](err); ok {
			writeErr(w, http.StatusForbidden, "source requires verification")
			return
		}
		s.logger.Error("api search", "error", err, "plugin", pluginID, "q", q)
		writeErr(w, http.StatusBadGateway, "search failed: "+err.Error())
		return
	}
	s.service.SyncPluginMeta(pluginID)
	pageSize := searchPageSize
	total := len(results)
	start := min((page-1)*pageSize, total)
	end := min(start+pageSize, total)
	writeJSON(w, http.StatusOK, apiSearchResponse{
		Results: results[start:end],
		HasNext: end < total,
		Page:    page,
	})
}
