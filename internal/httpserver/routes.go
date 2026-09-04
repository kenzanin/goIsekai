package httpserver

import (
	"io/fs"
	"net/http"

	"github.com/CloudyKit/jet/v6"
	"github.com/go-chi/chi/v5"
)

// routes registers every HTTP endpoint: static assets, HTML views, actions,
// the image proxy, and the API. Registration order does not matter (exact
// patterns win over prefixes).
func (s *Server) routes() {
	s.Router.Use(s.loggingMiddleware)
	s.registerStaticRoutes()
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

// registerViewRoutes maps HTML page routes. Views render full pages that
// extend /layouts/base.jet; hx-boost in the nav gives HTMX-style swaps.
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

// renderPage renders a view template with the `active` nav var set. Every
// view extends /layouts/base.jet, so this is always a full page.
func (s *Server) renderPage(w http.ResponseWriter, name, active string, data any) {
	vars := jet.VarMap{}
	vars.Set("active", active)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.engine.Render(w, name, vars, data); err != nil {
		s.logger.Error("render "+name, "error", err)
		// Header may already be written; best effort.
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
