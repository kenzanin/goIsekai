package httpserver

import (
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

// brHandler wraps an http.FileSystem and serves a pre-compressed `.br` sibling
// (e.g. reader.js.br) when the client accepts brotli and the file exists. This
// avoids compressing at request time: assets are compressed once at build time
// by `make br`, then embedded and committed. The explicit Content-Encoding makes
// chi's Compress middleware skip re-compression.
func brHandler(fsys http.FileSystem) http.Handler {
	fileServer := http.FileServer(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !acceptsBrotli(r) {
			fileServer.ServeHTTP(w, r)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			fileServer.ServeHTTP(w, r)
			return
		}
		f, err := fsys.Open(name + ".br")
		if err != nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		defer f.Close()
		st, err := f.Stat()
		if err != nil || st.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}
		if ct := mime.TypeByExtension(filepath.Ext(name)); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		w.Header().Set("Content-Encoding", "br")
		w.Header().Add("Vary", "Accept-Encoding")
		http.ServeContent(w, r, name+".br", st.ModTime(), f)
	})
}

// acceptsBrotli reports whether the Accept-Encoding header lists br. `*` and
// explicit br (with or without a q-value) both count; `br;q=0` is treated as
// accepted — the extra complexity of honoring q=0 is not worth it here.
func acceptsBrotli(r *http.Request) bool {
	for _, enc := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		if strings.TrimSpace(strings.SplitN(enc, ";", 2)[0]) == "br" {
			return true
		}
	}
	return false
}
