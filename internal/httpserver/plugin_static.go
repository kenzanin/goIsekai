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

	// Sanitize: no path separators, no dots beyond the filename.
	file = filepath.Base(file)
	if file == "." || file == ".." || strings.Contains(file, "/") || strings.Contains(file, "\\") {
		http.NotFound(w, r)
		return
	}

	// Serve a logo that's been materialised into the per-plugin cache
	// (<cacheDir>/images/plugins/<id>/) before falling back to the live
	// plugin directory. This lets a URL or inline (data: URI) logo render
	// instantly at next startup without a live fetch or a loaded VM.
	if cached := s.service.PluginLogoCacheFile(pluginID, file); cached != "" {
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, cached)
		return
	}

	// Find the plugin's directory from the manager.
	dir := s.service.PluginDir(pluginID)
	if dir == "" {
		http.NotFound(w, r)
		return
	}

	target := filepath.Join(dir, file)
	// Ensure resolved path stays within plugin dir.
	if !strings.HasPrefix(target, dir) {
		http.NotFound(w, r)
		return
	}

	// Logos are swapped in place under the same filename — without this the
	// browser heuristically caches the old bytes (only Last-Modified is set)
	// and stale logos show up for days. no-cache forces a cheap revalidate.
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, target)
}
