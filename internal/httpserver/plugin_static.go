package httpserver

import (
	"net/http"
	"path/filepath"
	"strings"
)

// registerPluginStaticRoutes maps /plugin-static/{pluginID}/{file} to files
// inside the plugin's own directory (logos etc.).
func (s *Server) registerPluginStaticRoutes() {
	s.Router.Get("/plugin-static/{pluginID}/{file}", s.servePluginStatic)
}

// servePluginStatic serves static files (logos, etc.) from a plugin's directory.
// Path traversal is blocked: the resolved path must stay within the plugin dir.
func (s *Server) servePluginStatic(w http.ResponseWriter, r *http.Request) {
	pluginID := param(r, "pluginID")
	file := param(r, "file")

	// Find the plugin's directory from the manager.
	dir := s.service.PluginDir(pluginID)
	if dir == "" {
		http.NotFound(w, r)
		return
	}

	// Sanitize: no path separators, no dots beyond the filename.
	file = filepath.Base(file)
	if file == "." || file == ".." || strings.Contains(file, "/") || strings.Contains(file, "\\") {
		http.NotFound(w, r)
		return
	}

	target := filepath.Join(dir, file)
	// Ensure resolved path stays within plugin dir.
	if !strings.HasPrefix(target, dir) {
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, target)
}
