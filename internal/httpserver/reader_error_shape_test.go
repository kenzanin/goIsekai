package httpserver

import (
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
	if !strings.Contains(js, "Server error (${r.status})") {
		t.Error("non-text bodies must still fall back to the bare status")
	}

	// The host's plumbing is peeled so the reader sees the reason, not the stack.
	for _, want := range []string{
		"failed to load pages:",
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
