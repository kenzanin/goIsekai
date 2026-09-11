package httpserver

import (
	"archive/zip"
	"io"
	"net/http"
	"os"
	"path/filepath"
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

// handleInstallPlugin accepts an uploaded .zip file (plugin folder),
// extracts it to a temp directory, and installs it through the bridge.
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

	// Create a temp directory for extraction.
	tmpDir, err := os.MkdirTemp("", "goisekai-plugin-*.zip")
	if err != nil {
		s.logger.Error("install plugin: create temp dir", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Read the file into a buffer.
	buf, err := io.ReadAll(file)
	if err != nil {
		s.logger.Error("install plugin: read upload", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Open as zip.
	bufReader := &bytesBuffer{data: buf}
	zipReader, err := zip.NewReader(bufReader, int64(len(buf)))
	if err != nil {
		_ = os.Remove(tmpDir)
		s.logger.Error("install plugin: not a valid zip", "error", err)
		http.Error(w, "invalid zip file", http.StatusBadRequest)
		return
	}

	// Extract all files.
	for _, zf := range zipReader.File {
		dstPath := filepath.Join(tmpDir, zf.Name)
		if zf.Name == "." || zf.Name == "/" {
			continue
		}
		if zf.FileInfo().IsDir() {
			if err := os.MkdirAll(dstPath, 0o755); err != nil {
				s.logger.Error("install plugin: mkdir", "error", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			s.logger.Error("install plugin: open zip file", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
			_ = rc.Close()
			s.logger.Error("install plugin: mkdir parent", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		dst, err := os.Create(dstPath)
		if err != nil {
			_ = rc.Close()
			s.logger.Error("install plugin: create file", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := io.Copy(dst, rc); err != nil {
			_ = dst.Close()
			_ = rc.Close()
			s.logger.Error("install plugin: write file", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = dst.Close()
		_ = rc.Close()
	}

	// Pass the extracted directory to the bridge.
	if err := s.service.InstallPlugin(tmpDir); err != nil {
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

// bytesBuffer wraps a byte slice to satisfy io.ReaderAt.
type bytesBuffer struct {
	data []byte
}

func (b *bytesBuffer) ReadAt(p []byte, off int64) (n int, err error) {
	if off >= int64(len(b.data)) {
		return 0, io.EOF
	}
	n = copy(p, b.data[off:])
	return n, nil
}
