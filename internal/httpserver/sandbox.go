package httpserver

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"goisekai/internal/bridge"
	"goisekai/pkg/types"
)

// Sandbox wraps an AppService and exposes plugin-dev endpoints.
type Sandbox struct {
	svc *bridge.AppService
}

// NewSandbox returns a Sandbox bound to the given service.
func NewSandbox(svc *bridge.AppService) *Sandbox {
	return &Sandbox{svc: svc}
}

// registerSandboxRoutes adds /api/sandbox/* endpoints for plugin development.
func (s *Server) registerSandboxRoutes() {
	sb := NewSandbox(s.service)

	s.Router.Route("/api/sandbox/plugins", func(r chi.Router) {
		// List loaded plugins
		r.Get("/", sb.handleList)

		// Hot-load a plugin from an external path
		r.Post("/load", sb.handleLoad)

		// Per-plugin endpoints
		r.Route("/{pluginID}", func(r chi.Router) {
			r.Post("/unload", sb.handleUnload)
			r.Post("/reload", sb.handleReload)

			// Plugin API sandbox — call functions and see raw output
			r.Get("/search", sb.handleSearch)
			r.Get("/detail/{mangaID}", sb.handleDetail)
			r.Get("/chapters/{mangaID}", sb.handleChapters)
			r.Get("/pages/{chapterID}", sb.handlePages)
		})
	})
}

// --- plugin lifecycle ---

func (s *Sandbox) handleList(w http.ResponseWriter, r *http.Request) {
	plugins := s.svc.PluginMetas()
	writeJSON(w, plugins)
}

type loadRequest struct {
	Path string `json:"path"`
}

func (s *Sandbox) handleLoad(w http.ResponseWriter, r *http.Request) {
	var req loadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
		writeErr(w, http.StatusBadRequest, "missing path")
		return
	}
	id, err := s.svc.LoadPluginHot(req.Path)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]string{"id": id})
}

func (s *Sandbox) handleUnload(w http.ResponseWriter, r *http.Request) {
	id := param(r, "pluginID")
	if err := s.svc.UnloadPlugin(id); err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, map[string]string{"status": "unloaded"})
}

func (s *Sandbox) handleReload(w http.ResponseWriter, r *http.Request) {
	id := param(r, "pluginID")
	newID, err := s.svc.ReloadPlugin(id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]string{"id": newID})
}

// --- plugin API sandbox ---

func (s *Sandbox) handleSearch(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	q := r.URL.Query().Get("q")
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		_ = json.Unmarshal([]byte(p), &page)
	}

	results, err := s.svc.SearchManga(pluginID, types.SearchFilter{Query: q, Page: page})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, results)
}

func (s *Sandbox) handleDetail(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")

	manga, _, err := s.svc.GetMangaDetails(pluginID, mangaID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, manga)
}

func (s *Sandbox) handleChapters(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")

	_, chapters, err := s.svc.GetMangaDetails(pluginID, mangaID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, chapters)
}

func (s *Sandbox) handlePages(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	chapterID := param(r, "chapterID")

	pages, err := s.svc.GetPageList(pluginID, chapterID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, pages)
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
