package httpserver

import (
	"io"
	"net/http"
	"os"
)

// handleTestProfile runs a single GET against the plugin's site URL (or an
// explicit url form field) using the requested TLS profile and reports the
// resulting HTTP status as JSON. It does not change the pinned profile.
func (s *Server) handleTestProfile(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	if err := r.ParseForm(); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	profile := r.FormValue("profile")
	if profile == "" {
		writeErr(w, http.StatusBadRequest, "missing 'profile' field")
		return
	}
	status, err := s.service.TestProfile(pluginID, profile, r.FormValue("url"))
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      status >= 200 && status < 400,
		"profile": profile,
		"status":  status,
	})
}

// handleResetProfile clears the plugin's pinned TLS profile.
func (s *Server) handleResetProfile(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	s.service.ResetProfile(pluginID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
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
