package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The reader collapsed every failed reader-data fetch to "Server error (502)",
// so the plugin's own message - the only part that says what actually went wrong,
// and the part that distinguishes "the site rotated its key" from "the CDN is
// down" - never reached the person reading. These pin the shape that fixes it.
func TestReaderSurfacesThePluginsOwnReason(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "cmd", "goisekai", "frontend", "lib", "reader.js"))
	if err != nil {
		t.Fatalf("read reader.js: %v", err)
	}
	js := string(src)

	// The failure body must be read; throwing on status alone discards it.
	if !strings.Contains(js, "readerDataFailure") {
		t.Error("no readerDataFailure path; a non-ok response still loses its body")
	}
	if !strings.Contains(js, "r.text()") && !strings.Contains(js, ".text()") {
		t.Error("the error body is never read, so nothing useful can be shown")
	}
	// The one path that must stay: a body that is not text (an HTML error page
	// from a proxy, say) must not be dumped into the panel.
	if !strings.Contains(js, "Server error (${status})") {
		t.Error("a body with no error field must still fall back to the bare status")
	}

	// The envelope is the contract; scraping text is not. /api/reader-data
	// answers writeErr's {"error": ...} like every other /api route.
	if !strings.Contains(js, "JSON.parse(raw)") {
		t.Error("the error envelope is not parsed")
	}
	if strings.Contains(js, "failed to load pages:") {
		t.Error("still stripping a transport prefix the server no longer sends")
	}

	// The host's plumbing is peeled so the reader sees the reason, not the stack.
	for _, want := range []string{
		"get page list:",
		`\[string "`,
	} {
		if !strings.Contains(js, want) {
			t.Errorf("the plumbing prefix %q is not stripped", want)
		}
	}
}

// An empty page list is the other failure shape (the plugin returns [] rather
// than raising), and the message must say it is the plugin's list, not the
// server's, or the user goes looking in the wrong place.
func TestReaderExplainsAnEmptyPageList(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "cmd", "goisekai", "frontend", "lib", "reader.js"))
	if err != nil {
		t.Fatalf("read reader.js: %v", err)
	}
	js := string(src)
	if !strings.Contains(js, "no pages") || !strings.Contains(js, "plugin") {
		t.Error("the empty-page-list message does not say the plugin returned nothing")
	}
}

// The server side of the same contract. /api/reader-data used http.Error, so it
// answered a failure with plain text while every other /api route answers with
// {"error": ...}; that is what forced reader.js into scraping the body.
func TestReaderDataAnswersWithTheErrorEnvelope(t *testing.T) {
	srv := testServer(t, "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/reader-data/1manga/m/ch", nil)
	srv.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusBadGateway {
		t.Fatalf("expected a failure status for an unknown chapter, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content-type = %q, want application/json: the envelope is the contract", ct)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not the JSON envelope (%v): %q", err, rec.Body.String())
	}
	if body["error"] == "" {
		t.Error("the envelope carries no error message")
	}
}
