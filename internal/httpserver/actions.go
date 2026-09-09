package httpserver

import (
	"net/http"
	"os"
	"strconv"
	"syscall"
	"time"

	"goisekai/internal/config"
)

// registerActionRoutes mounts the HTMX form-action endpoints. Every handler
// mutates state then answers with an HX-Redirect so htmx performs a fresh
// full-page navigation to the owning view (no fragment templates to keep
// in sync).
func (s *Server) registerActionRoutes() {
	s.Router.Post("/action/install-plugin", s.handleInstallPlugin)
	s.Router.Post("/action/toggle-plugin/{pluginID}", s.handleTogglePlugin)
	s.Router.Post("/action/toggle-library/{pluginID}/{mangaID}", s.handleToggleLibrary)
	s.Router.Post("/action/sync", s.handleSync)
	s.Router.Post("/action/fetch-alt-titles/{pluginID}/{mangaID}", s.handleFetchAltTitles)
	s.Router.Post("/action/set-title/{pluginID}/{mangaID}", s.handleSetTitle)
	s.Router.Post("/action/remove-alt-title/{pluginID}/{mangaID}", s.handleRemoveAltTitle)
	s.Router.Post("/action/fetch-alt-summaries/{pluginID}/{mangaID}", s.handleFetchAltSummaries)
	s.Router.Post("/action/remove-alt-summary/{pluginID}/{mangaID}", s.handleRemoveAltSummary)
	s.Router.Post("/action/set-summary/{pluginID}/{mangaID}", s.handleSetSummary)
	s.Router.Post("/action/set-chapter-progress", s.handleSetChapterProgress)
	s.Router.Post("/action/mark-read/{pluginID}/{mangaID}/{chapterID}", s.handleMarkChapterRead)
	s.Router.Post("/action/mark-read-bulk", s.handleMarkChaptersReadBulk)
	s.Router.Post("/action/mark-read-range/{pluginID}/{mangaID}/{fromID}/{toID}", s.handleMarkChapterReadRange)
	s.Router.Post("/action/reset-progress/{pluginID}/{mangaID}/{chapterID}", s.handleResetChapterProgress)
	s.Router.Post("/action/reset-progress-all/{pluginID}/{mangaID}", s.handleResetMangaProgress)
	s.Router.Post("/action/clear-logs", s.handleClearLogs)
	s.Router.Post("/action/save-settings", s.handleSaveSettings)
	s.Router.Post("/action/save-verify/{pluginID}", s.handleSaveVerify)
	s.Router.Post("/action/export-cbz/{pluginID}/{mangaID}/{chapterID}", s.handleExportCBZ)
	s.Router.Post("/action/clear-cache/{pluginID}/{mangaID}", s.handleClearMangaCache)
	s.Router.Post("/action/clear-cache-all", s.handleClearAllCache)
	s.Router.Post("/action/test-profile/{pluginID}", s.handleTestProfile)
	s.Router.Post("/action/reset-profile/{pluginID}", s.handleResetProfile)
	// Restart the application process via syscall.Exec (true re-exec).
	s.Router.Post("/action/restart", s.handleRestart)
}

// hxRedirect answers a successful action with a 303 See Other redirect —
// plain HTML form posts (browser follows it) and HTMX (follows redirects
// natively) both land on the target page. 303 forces GET after POST.
func (s *Server) hxRedirect(w http.ResponseWriter, location string) {
	w.Header().Set("Location", location)
	w.WriteHeader(http.StatusSeeOther)
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
	if err := cfg.Save(cfgPath); err != nil {
		s.logger.Error("save settings: write config", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.hxRedirect(w, "/view/settings")
}

// handleRestart re-executes the current binary with its original arguments,
// giving a true in-place restart that works under nohup, a shell loop, or a
// supervisor. The response is flushed first so the client sees the 303.
func (s *Server) handleRestart(w http.ResponseWriter, r *http.Request) {
	s.hxRedirect(w, "/view/settings")
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
