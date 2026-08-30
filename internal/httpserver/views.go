package httpserver

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"

	"goisekai/internal/config"
	"goisekai/pkg/types"
)

// fsSub returns the embedded frontend subtree (frontend/…) for /static.
func fsSub() (fs.FS, error) {
	return fs.Sub(defaultAssets, "frontend")
}

// defaultAssets holds the assets FS injected by New so view helpers can
// reach it without threading it through every handler.
var defaultAssets embed.FS

// viewLibrary renders the library grid (also the home page).
func (s *Server) viewLibrary(w http.ResponseWriter, r *http.Request) {
	mangas, err := s.service.ListLibrary()
	if err != nil {
		s.logger.Error("library list", "error", err)
	}
	s.renderPage(w, "views/library.jet", "library", map[string]any{"Mangas": mangas})
}

// viewSearch renders the search form and, when q+pluginID are present, results.
func (s *Server) viewSearch(w http.ResponseWriter, r *http.Request) {
	plugins, err := s.service.ListPlugins()
	if err != nil {
		s.logger.Error("plugin list", "error", err)
	}
	q := r.URL.Query().Get("q")
	pluginID := r.URL.Query().Get("pluginID")
	var results []types.Manga
	if q != "" && pluginID != "" {
		results, err = s.service.SearchManga(pluginID, types.SearchFilter{Query: q})
		if err != nil {
			s.logger.Error("search", "error", err, "plugin", pluginID, "q", q)
		}
	}
	s.renderPage(w, "views/search.jet", "search", map[string]any{
		"Plugins":  plugins,
		"Q":        q,
		"PluginID": pluginID,
		"Results":  results,
	})
}

// viewMangaDetail renders a manga's info plus its chapter list.
func (s *Server) viewMangaDetail(w http.ResponseWriter, r *http.Request) {
	pluginID := chi.URLParam(r, "pluginID")
	mangaID := chi.URLParam(r, "mangaID")
	manga, chapters, err := s.service.GetMangaDetails(pluginID, mangaID)
	if err != nil {
		s.logger.Error("manga detail", "error", err, "plugin", pluginID, "manga", mangaID)
		http.Error(w, "failed to load manga: "+err.Error(), http.StatusBadGateway)
		return
	}
	s.renderPage(w, "views/detail.jet", "search", map[string]any{
		"PluginID": pluginID,
		"MangaID": mangaID,
		"Manga":    manga,
		"Chapters": chapters,
	})
}

// viewPlugins renders the plugin manager page.
func (s *Server) viewPlugins(w http.ResponseWriter, _ *http.Request) {
	plugins, err := s.service.ListPlugins()
	if err != nil {
		s.logger.Error("plugin list", "error", err)
	}
	s.renderPage(w, "views/plugins.jet", "plugins", map[string]any{"Plugins": plugins})
}

// viewSettings renders the current goisekai.ini values.
func (s *Server) viewSettings(w http.ResponseWriter, _ *http.Request) {
	path := s.service.GetConfigPath()
	cfg, err := config.Load(path)
	if err != nil {
		s.logger.Error("config load", "error", err, "path", path)
	}
	s.renderPage(w, "views/settings.jet", "settings", map[string]any{
		"Config": cfg,
		"Path":   path,
	})
}

// viewLogs renders the in-memory log buffer with a 2s HTMX poll.
func (s *Server) viewLogs(w http.ResponseWriter, _ *http.Request) {
	s.renderPage(w, "views/logs.jet", "logs", map[string]any{"Logs": s.service.GetLogs()})
}

// jsonMarshalLogs writes lines as a JSON array.
func jsonMarshalLogs(w http.ResponseWriter, lines []string) error {
	enc := json.NewEncoder(w)
	return enc.Encode(lines)
}
