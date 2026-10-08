package httpserver

import (
	"net/http"
	"net/url"
	"os"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	"goisekai/internal/config"
)

// registerActionRoutes mounts the form-action endpoints. Most handlers mutate
// state then redirect to the owning view with hxRedirect, so a plain browser
// form post reloads that view and the client-side action layer can swap the
// response into the page (no fragment templates to keep in sync).
//
// Every action lives under /action, which carries requireCSRFToken. Mounting
// them one by one on the root router is what previously left them ungated; a
// new action added here cannot escape the guard.
func (s *Server) registerActionRoutes() {
	s.Router.Route("/action", func(r chi.Router) {
		r.Use(s.requireCSRFToken)
		s.registerActionPostRoutes(r)
	})
}

// registerActionPostRoutes declares the mutating action endpoints. r is the
// CSRF-guarded /action sub-router.
func (s *Server) registerActionPostRoutes(r chi.Router) {
	r.Post("/install-plugin", s.handleInstallPlugin)
	r.Post("/toggle-plugin/{pluginID}", s.handleTogglePlugin)
	r.Post("/refresh-plugins", s.handleRefreshPlugins)
	r.Post("/toggle-library/{pluginID}/{mangaID}", s.handleToggleLibrary)
	r.Post("/sync", s.handleSync)
	r.Post("/sync-manga/{pluginID}/{mangaID}", s.handleSyncManga)
	r.Post("/set-title/{pluginID}/{mangaID}", s.handleSetTitle)
	r.Post("/remove-alt-title/{pluginID}/{mangaID}", s.handleRemoveAltTitle)
	r.Post("/remove-alt-summary/{pluginID}/{mangaID}", s.handleRemoveAltSummary)
	r.Post("/set-summary/{pluginID}/{mangaID}", s.handleSetSummary)
	r.Post("/fetch-enrichment/{pluginID}/{mangaID}", s.handleFetchEnrichment)
	r.Post("/remove-genre/{pluginID}/{mangaID}", s.handleRemoveGenre)
	r.Post("/remove-related/{pluginID}/{mangaID}", s.handleRemoveRelated)
	r.Post("/remove-category/{pluginID}/{mangaID}", s.handleRemoveCategory)
	r.Post("/add-category/{pluginID}/{mangaID}", s.handleAddCategory)
	r.Post("/add-genre/{pluginID}/{mangaID}", s.handleAddGenre)
	r.Post("/reset-enrichment/{pluginID}/{mangaID}", s.handleResetEnrichment)
	r.Post("/set-chapter-progress", s.handleSetChapterProgress)
	r.Post("/mark-read/{pluginID}/{mangaID}/{chapterID}", s.handleMarkChapterRead)
	r.Post("/reset-progress/{pluginID}/{mangaID}/{chapterID}", s.handleResetChapterProgress)
	r.Post("/toggle-skip/{pluginID}/{mangaID}/{chapterID}", s.handleToggleChapterSkip)
	r.Post("/toggle-cover-dim/{pluginID}/{mangaID}", s.handleToggleCoverDim)
	r.Post("/refetch-cover/{pluginID}/{mangaID}", s.handleRefetchCover)
	r.Post("/chapter-actions", s.handleChapterActions)
	r.Post("/save-settings", s.handleSaveSettings)
	r.Post("/save-verify/{pluginID}", s.handleSaveVerify)
	r.Post("/export-cbz/{pluginID}/{mangaID}/{chapterID}", s.handleExportCBZ)
	r.Post("/clear-logs", s.handleClearLogs)
	r.Post("/clear-cache-all", s.handleClearAllCache)
	r.Post("/test-profile/{pluginID}", s.handleTestProfile)
	r.Post("/migrate-source/{pluginID}/{mangaID}", s.handleMigrateSource)
	r.Post("/reset-profile/{pluginID}", s.handleResetProfile)
	r.Post("/restart", s.handleRestart)
}

// hxRedirect answers a successful action with a 303 See Other redirect —
// the browser (and fetch) follows it natively, landing on the target page.
// 303 forces GET after POST. The name is a leftover from the old HTMX layer.
//
// The originating page's query string (e.g. ?ChPage=2) is carried over when
// the redirect returns to the same path, so an in-place action does not reset
// pagination or drop filters.
func (s *Server) hxRedirect(w http.ResponseWriter, r *http.Request, location string) {
	w.Header().Set("Location", carryQuery(r, location))
	w.WriteHeader(http.StatusSeeOther)
}

// carryQuery merges the referer's query string into location when both point
// at the same path. Location's own params win; a referer toast= is dropped
// (re-showing a stale toast on every later action would be a bug).
func carryQuery(r *http.Request, location string) string {
	ref := r.Referer()
	if ref == "" {
		return location
	}
	u, err := url.Parse(ref)
	if err != nil {
		return location
	}
	loc, err := url.Parse(location)
	if err != nil || loc.Path != u.Path || u.RawQuery == "" {
		return location
	}
	q := loc.Query()
	for k, vs := range u.Query() {
		if k == "toast" {
			continue
		}
		if _, dup := q[k]; dup {
			continue
		}
		for _, v := range vs {
			q.Add(k, v)
		}
	}
	loc.RawQuery = q.Encode()
	return loc.String()
}

// handleSaveSettings applies only the settings keys present in the form and
// writes the config back to disk.
func (s *Server) handleSaveSettings(w http.ResponseWriter, r *http.Request) {
	cfgPath := s.service.GetConfigPath()
	if cfgPath == "" {
		s.logger.Error("save settings: no config path set")
		http.Error(w, "no config path set", http.StatusInternalServerError)
		return
	}
	if err := r.ParseForm(); err != nil {
		s.logger.Error("save settings: parse form", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		s.logger.Error("save settings: load config", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, ok := r.Form["host"]; ok {
		cfg.Host = r.FormValue("host")
	}
	if _, ok := r.Form["port"]; ok {
		if n, err := strconv.Atoi(r.FormValue("port")); err == nil && n > 0 {
			cfg.Port = n
		}
	}
	if _, ok := r.Form["title"]; ok {
		cfg.Title = r.FormValue("title")
	}
	if _, ok := r.Form["log_level"]; ok {
		cfg.LogLevel = r.FormValue("log_level")
	}
	if _, ok := r.Form["user_agent"]; ok {
		cfg.UserAgent = r.FormValue("user_agent")
	}
	if _, ok := r.Form["accept_language"]; ok {
		cfg.AcceptLanguage = r.FormValue("accept_language")
	}
	if _, ok := r.Form["referer"]; ok {
		cfg.Referer = r.FormValue("referer")
	}
	if _, ok := r.Form["update_stale_days"]; ok {
		if n, err := strconv.Atoi(r.FormValue("update_stale_days")); err == nil && n >= 1 {
			cfg.UpdateStaleDays = n
		}
	}
	if err := cfg.Save(cfgPath); err != nil {
		s.logger.Error("save settings: write config", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.hxRedirect(w, r, "/view/settings")
}

// handleRestart re-executes the current binary with its original arguments,
// giving a true in-place restart that works under nohup, a shell loop, or a
// supervisor. The response is flushed first so the client sees the 303.
func (s *Server) handleRestart(w http.ResponseWriter, r *http.Request) {
	s.hxRedirect(w, r, "/view/settings")
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	s.logger.Info("restart requested", "remote", r.RemoteAddr)
	go func() {
		time.Sleep(500 * time.Millisecond)
		exe, err := os.Executable()
		if err != nil {
			s.logger.Error("restart: resolve executable", "error", err)
			return
		}
		s.logger.Info("re-executing", "path", exe, "args", os.Args)
		if err := syscall.Exec(exe, os.Args, os.Environ()); err != nil {
			s.logger.Error("restart: exec", "error", err)
		}
	}()
}
