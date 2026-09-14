package httpserver

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// Asset URLs carry no version param, so a stale cached copy would survive a
// rebuild and keep serving fixed code as broken. Every response must force
// revalidation, whether it comes from the brotli sibling or the plain file.
func TestBrHandlerForcesRevalidation(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "app.js"), []byte("plain"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.js.br"), []byte("brotli"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := brHandler(http.FS(os.DirFS(dir)))

	for _, tc := range []struct {
		name string
		enc  string
		want string
	}{
		{"brotli", "br", "brotli"},
		{"plain", "", "plain"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/app.js", nil)
			if tc.enc != "" {
				req.Header.Set("Accept-Encoding", tc.enc)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
				t.Errorf("Cache-Control = %q, want no-cache", got)
			}
			if got := rec.Body.String(); got != tc.want {
				t.Errorf("body = %q, want %q", got, tc.want)
			}
		})
	}
}
