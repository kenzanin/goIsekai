package httpserver

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

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
}

// hxRedirect answers a successful action with a 303 See Other redirect —
// plain HTML form posts (browser follows it) and HTMX (follows redirects
// natively) both land on the target page. 303 forces GET after POST.
func (s *Server) hxRedirect(w http.ResponseWriter, location string) {
	w.Header().Set("Location", location)
	w.WriteHeader(http.StatusSeeOther)
}

// handleInstallPlugin saves the uploaded .wasm to a temp file and installs it
// through the bridge.
func (s *Server) handleInstallPlugin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		s.logger.Error("install plugin: parse form", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		s.logger.Error("install plugin: missing file field", "error", err)
		http.Error(w, "missing 'file' field", http.StatusBadRequest)
		return
	}
	defer func() { _ = file.Close() }()

	tmp, err := os.CreateTemp("", "goisekai-plugin-*.wasm")
	if err != nil {
		s.logger.Error("install plugin: create temp", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }() // bridge copies the wasm into its own dir

	if _, err := io.Copy(tmp, file); err != nil {
		_ = tmp.Close()
		s.logger.Error("install plugin: copy upload", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := tmp.Close(); err != nil {
		s.logger.Error("install plugin: close temp", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := s.service.InstallPlugin(tmpPath); err != nil {
		s.logger.Error("install plugin", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.hxRedirect(w, "/view/plugins")
}

// handleTogglePlugin flips a plugin's active flag.
func (s *Server) handleTogglePlugin(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	if err := s.service.TogglePlugin(pluginID); err != nil {
		s.logger.Error("toggle plugin", "pluginID", pluginID, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.hxRedirect(w, "/view/plugins")
}

// handleSaveVerify stores pasted verification cookies/UA for a plugin.
func (s *Server) handleSaveVerify(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	if err := r.ParseForm(); err != nil {
		s.logger.Error("save verify: parse form", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.service.SavePluginVerify(pluginID, r.FormValue("cookies"), r.FormValue("user_agent")); err != nil {
		s.logger.Error("save verify", "pluginID", pluginID, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.hxRedirect(w, "/view/plugins")
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

// handleClearLogs empties the in-memory log buffer.
func (s *Server) handleClearLogs(w http.ResponseWriter, _ *http.Request) {
	s.service.ClearLogs()
	w.Header().Set("Location", "/view/logs")
	w.WriteHeader(303)
}

// handleExportCBZ builds a .cbz archive for one chapter and serves it as a
// file download. The title for the filename is taken from the form.
func (s *Server) handleExportCBZ(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	chapterID := param(r, "chapterID")
	if err := r.ParseForm(); err != nil {
		s.logger.Error("export cbz: parse form", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	title := r.FormValue("title")
	if title == "" {
		title = chapterID
	}
	path, err := s.service.ExportCBZ(pluginID, mangaID, chapterID, title)
	if err != nil {
		s.logger.Error("export cbz", "pluginID", pluginID, "mangaID", mangaID, "chapterID", chapterID, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filepath.Base(path)+"\"")
	w.Header().Set("Content-Type", "application/vnd.comicbook+zip")
	http.ServeFile(w, r, path)
}

// handleClearMangaCache removes every cached image for one manga.
func (s *Server) handleClearMangaCache(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	if err := s.service.ClearMangaCache(pluginID, mangaID); err != nil {
		s.logger.Error("clear manga cache", "pluginID", pluginID, "mangaID", mangaID, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.hxRedirect(w, "/view/manga/"+pluginID+"/"+mangaID)
}

// handleClearAllCache removes the entire image cache directory.
func (s *Server) handleClearAllCache(w http.ResponseWriter, _ *http.Request) {
	if err := s.service.ClearAllCache(); err != nil {
		s.logger.Error("clear all cache", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.hxRedirect(w, "/view/settings")
}

// handleFetchAltTitles resolves alternative titles via the provider plugin
// and redirects back to the manga detail page. The server is taken from the
// form (browser flow).
func (s *Server) handleFetchAltTitles(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	if err := r.ParseForm(); err != nil {
		s.logger.Error("fetch alt titles: parse form", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	server := r.FormValue("server")
	if _, err := s.service.FetchAltTitles(pluginID, mangaID, server); err != nil {
		s.logger.Error("fetch alt titles", "pluginID", pluginID, "mangaID", mangaID, "server", server, "error", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	s.hxRedirect(w, "/view/manga/"+pluginID+"/"+mangaID)
}

// handleSetTitle promotes the submitted title to be the manga's main title.
func (s *Server) handleSetTitle(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	if err := r.ParseForm(); err != nil {
		s.logger.Error("set title: parse form", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	title := r.FormValue("title")
	if err := s.service.SetMainTitle(pluginID, mangaID, title); err != nil {
		s.logger.Error("set title", "pluginID", pluginID, "mangaID", mangaID, "title", title, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.hxRedirect(w, "/view/manga/"+pluginID+"/"+mangaID)
}

// handleRemoveAltTitle removes the submitted alternative title.
func (s *Server) handleRemoveAltTitle(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	mangaID := param(r, "mangaID")
	if err := r.ParseForm(); err != nil {
		s.logger.Error("remove alt title: parse form", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	title := r.FormValue("title")
	if err := s.service.RemoveAltTitle(pluginID, mangaID, title); err != nil {
		s.logger.Error("remove alt title", "pluginID", pluginID, "mangaID", mangaID, "title", title, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.hxRedirect(w, "/view/manga/"+pluginID+"/"+mangaID)
}
