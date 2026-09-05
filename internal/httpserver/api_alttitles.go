package httpserver

import (
	"encoding/json"
	"net/http"
	"strings"

	"goisekai/internal/database"
)

// apiAltTitleServers lists every alt-title server declared by discovered plugins.
func (s *Server) apiAltTitleServers(w http.ResponseWriter, _ *http.Request) {
	servers := s.service.AltTitleServers()
	out := make([]map[string]string, 0, len(servers))
	for _, e := range servers {
		out = append(out, map[string]string{
			"provider": e.ProviderPluginID,
			"server":   e.ServerID,
			"name":     e.Name,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// apiFetchAltTitles fetches titles from the chosen provider server and merges
// them into the stored list for a manga.
func (s *Server) apiFetchAltTitles(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	var body struct {
		Server string `json:"server"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Server == "" {
		writeErr(w, http.StatusBadRequest, "server is required")
		return
	}
	known := false
	for _, e := range s.service.AltTitleServers() {
		if e.ServerID == body.Server {
			known = true
			break
		}
	}
	if !known {
		writeErr(w, http.StatusBadRequest, "unknown server")
		return
	}
	rows, err := s.service.FetchAltTitles(pluginID, mangaID, body.Server)
	if err != nil {
		s.logger.Error("api fetch alt titles", "error", err, "plugin", pluginID, "manga", mangaID, "server", body.Server)
		writeErr(w, http.StatusBadGateway, "fetch failed")
		return
	}
	writeJSON(w, http.StatusOK, altTitleRowsPayload(rows))
}

// apiRemoveAltTitle deletes one stored alternative title.
func (s *Server) apiRemoveAltTitle(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	var body struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == "" {
		writeErr(w, http.StatusBadRequest, "title is required")
		return
	}
	if cur, _ := s.service.MangaTitle(pluginID, mangaID); cur == body.Title {
		writeErr(w, http.StatusBadRequest, "cannot remove the current main title")
		return
	}
	if err := s.service.RemoveAltTitle(pluginID, mangaID, body.Title); err != nil {
		s.logger.Error("api remove alt title", "error", err, "plugin", pluginID, "manga", mangaID)
		writeErr(w, http.StatusInternalServerError, "remove failed")
		return
	}
	rows, _ := s.service.ListAltTitles(pluginID, mangaID)
	writeJSON(w, http.StatusOK, altTitleRowsPayload(rows))
}

// apiSetTitle swaps the manga's main title with a stored alternative.
func (s *Server) apiSetTitle(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	var body struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == "" {
		writeErr(w, http.StatusBadRequest, "title is required")
		return
	}
	if err := s.service.SetMainTitle(pluginID, mangaID, body.Title); err != nil {
		writeErr(w, http.StatusBadRequest, "title not in stored alternative titles")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"title": body.Title})
}

// apiLibrarySearch ranks library manga by fuzzy score over title + alt titles.
func (s *Server) apiLibrarySearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeErr(w, http.StatusBadRequest, "q is required")
		return
	}
	hits, err := s.service.SearchLibrary(q)
	if err != nil {
		s.logger.Error("api library search", "error", err, "q", q)
		writeErr(w, http.StatusInternalServerError, "search failed")
		return
	}
	out := make([]map[string]any, 0, len(hits))
	for _, h := range hits {
		out = append(out, map[string]any{
			"plugin_id":       h.PluginID,
			"source_manga_id": h.SourceMangaID,
			"title":           h.Title,
			"score":           h.Score,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// altTitleRowsPayload converts DB rows to the wire shape.
func altTitleRowsPayload(rows []database.AltTitleRow) []map[string]string {
	out := make([]map[string]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]string{"title": r.Title, "source": r.Source})
	}
	return out
}
