package httpserver

import (
	"io"
	"net/http"
	"os"
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
	s.Router.Post("/action/set-chapter-progress", s.handleSetChapterProgress)
	s.Router.Post("/action/mark-read/{pluginID}/{mangaID}/{chapterID}", s.handleMarkChapterRead)
	s.Router.Post("/action/mark-read-bulk", s.handleMarkChaptersReadBulk)
	s.Router.Post("/action/mark-read-range/{pluginID}/{mangaID}/{fromID}/{toID}", s.handleMarkChapterReadRange)
	s.Router.Post("/action/save-settings", s.handleSaveSettings)
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
	defer file.Close()

	tmp, err := os.CreateTemp("", "goisekai-plugin-*.wasm")
	if err != nil {
		s.logger.Error("install plugin: create temp", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // bridge copies the wasm into its own dir

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
	pluginID := r.PathValue("pluginID")
	if err := s.service.TogglePlugin(pluginID); err != nil {
		s.logger.Error("toggle plugin", "pluginID", pluginID, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.hxRedirect(w, "/view/plugins")
}

// handleToggleLibrary flips a manga's in-library flag.
func (s *Server) handleToggleLibrary(w http.ResponseWriter, r *http.Request) {
	pluginID := r.PathValue("pluginID")
	mangaID := r.PathValue("mangaID")
	if err := s.service.ToggleLibraryItem(pluginID, mangaID); err != nil {
		s.logger.Error("toggle library", "pluginID", pluginID, "mangaID", mangaID, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.hxRedirect(w, "/view/manga/"+pluginID+"/"+mangaID)
}

// handleSync re-fetches chapter lists for every library manga.
func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	if err := s.service.SyncLibrary(); err != nil {
		s.logger.Error("sync library", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.hxRedirect(w, "/view/library")
}

// handleSetChapterProgress records the last-read page for a chapter.
func (s *Server) handleSetChapterProgress(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.logger.Error("set chapter progress: parse form", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	pluginID := r.FormValue("pluginID")
	mangaID := r.FormValue("mangaID")
	chapterID := r.FormValue("chapterID")
	page, err := strconv.Atoi(r.FormValue("page"))
	if err != nil || page < 0 {
		s.logger.Error("set chapter progress: bad page", "page", r.FormValue("page"))
		http.Error(w, "invalid 'page' value", http.StatusBadRequest)
		return
	}
	if err := s.service.SetChapterProgress(pluginID, mangaID, chapterID, page); err != nil {
		s.logger.Error("set chapter progress", "pluginID", pluginID, "mangaID", mangaID, "chapterID", chapterID, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.hxRedirect(w, "/view/manga/"+pluginID+"/"+mangaID)
}

// handleMarkChapterRead marks a single chapter as read.
func (s *Server) handleMarkChapterRead(w http.ResponseWriter, r *http.Request) {
	pluginID := r.PathValue("pluginID")
	mangaID := r.PathValue("mangaID")
	chapterID := r.PathValue("chapterID")
	if err := s.service.MarkChapterRead(pluginID, mangaID, chapterID); err != nil {
		s.logger.Error("mark chapter read", "pluginID", pluginID, "mangaID", mangaID, "chapterID", chapterID, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.hxRedirect(w, "/view/manga/"+pluginID+"/"+mangaID)
}

// handleMarkChaptersReadBulk marks every chapter listed in the repeated
// chapterIDs form field as read.
func (s *Server) handleMarkChaptersReadBulk(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.logger.Error("mark chapters read: parse form", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	pluginID := r.FormValue("pluginID")
	mangaID := r.FormValue("mangaID")
	ids := r.Form["chapterIDs"]
	if len(ids) == 0 {
		http.Error(w, "no chapters selected", http.StatusBadRequest)
		return
	}
	for _, id := range ids {
		if err := s.service.MarkChapterRead(pluginID, mangaID, id); err != nil {
			s.logger.Error("mark chapter read", "pluginID", pluginID, "mangaID", mangaID, "chapterID", id, "error", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	s.hxRedirect(w, "/view/manga/"+pluginID+"/"+mangaID)
}

// handleMarkChapterReadRange marks every chapter from the first referenced id
// up to and including the second, in chapter_num order.
func (s *Server) handleMarkChapterReadRange(w http.ResponseWriter, r *http.Request) {
	pluginID := r.PathValue("pluginID")
	mangaID := r.PathValue("mangaID")
	fromID := r.PathValue("fromID")
	toID := r.PathValue("toID")
	if err := s.service.MarkChapterReadRange(pluginID, mangaID, fromID, toID); err != nil {
		s.logger.Error("mark chapter read range", "pluginID", pluginID, "mangaID", mangaID, "fromID", fromID, "toID", toID, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.hxRedirect(w, "/view/manga/"+pluginID+"/"+mangaID)
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
