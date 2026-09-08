package httpserver

import (
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// routes registers every HTTP endpoint: static assets, HTML views, actions,
// the image proxy, and the API. Registration order does not matter (exact
// patterns win over prefixes).
func (s *Server) routes() {
	s.Router.Use(s.loggingMiddleware)
	s.registerStaticRoutes()
	s.registerPluginStaticRoutes()
	s.registerViewRoutes()
	s.registerActionRoutes()
	s.registerImageRoutes()
	// /api group with optional API-key authentication.
	s.Router.Route("/api", func(r chi.Router) {
		r.Use(s.requireAPIKey)
		s.registerAPIRoutes(r)
		s.registerReaderRoutes(r)
		s.registerWSRoutes(r)
		s.registerSandboxRoutes(r)
	})
}

// registerStaticRoutes serves the embedded frontend dir under /static/.
func (s *Server) registerStaticRoutes() {
	staticFS, err := fs.Sub(s.assets, "frontend")
	if err != nil {
		s.logger.Error("static assets unavailable", "error", err)
		return
	}
	fileServer := brHandler(http.FS(staticFS))
	s.Router.Handle("/static/*", http.StripPrefix("/static/", fileServer))
}

// registerViewRoutes maps HTML page routes. Views render full Lua templates.
func (s *Server) registerViewRoutes() {
	s.Router.Get("/", s.viewLibrary)
	s.Router.Get("/view/library", s.viewLibrary)
	s.Router.Get("/view/search", s.viewSearch)
	s.Router.Get("/view/manga/{pluginID}/{mangaID}", s.viewMangaDetail)
	s.Router.Get("/view/plugins", s.viewPlugins)
	s.Router.Get("/view/settings", s.viewSettings)
	s.Router.Get("/view/logs", s.viewLogs)
	s.Router.Get("/view/history", s.viewHistory)
}

// renderPage renders a Lua template with the `active` nav var set.
// When the client sends X-Partial: true, only the <main> content is rendered
// (no layout wrapper) — used by the SPA router.
func (s *Server) renderPage(w http.ResponseWriter, r *http.Request, name, active string, data any) {
	var m map[string]any
	if data == nil {
		m = map[string]any{}
	} else if converted, ok := data.(map[string]any); ok {
		m = converted
	} else {
		m = map[string]any{}
	}
	m["active"] = active
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	var err error
	if r != nil && r.Header.Get("X-Partial") == "true" {
		err = s.engine.RenderPartial(w, name, m)
	} else {
		err = s.engine.Render(w, name, m)
	}
	if err != nil {
		s.logger.Error("render "+name, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
