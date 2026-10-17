package httpserver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReaderPrefsSurviveAChapterSwitch pins the per-manga reader prefs
// persistence contract in reader.js.
//
// Switching chapter is a full page load, so a debounced save that has not fired
// yet is thrown away with the document: the next chapter then read the previous
// server blob and the setting looked like it had reset per chapter. Two
// properties carry the guarantee:
//
//   - a pending debounce is flushed on pagehide/visibilitychange, with keepalive
//     so the request outlives the document;
//   - the zoom restore prefers the local mirror, which is written synchronously
//     on every change, over the server snapshot embedded at page load — reading
//     the server value first snapped the zoom back on every page turn, since the
//     blob is only as fresh as the last settled POST.
//
// This is a shape pin, not a behavioural test: reader.js is a self-executing
// IIFE that wires DOM events at load time and exposes nothing, so its functions
// cannot be called from a harness.
func TestReaderPrefsSurviveAChapterSwitch(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(frontendLibDir, "reader.js"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)

	flushAt := strings.Index(src, "function flushPrefs()")
	if flushAt < 0 {
		t.Fatal("reader.js has no flushPrefs(); a pending debounce is lost on chapter switch")
	}
	if !strings.Contains(src, "addEventListener('pagehide', flushPrefs)") {
		t.Error("reader.js does not flush prefs on pagehide; switching chapter before the " +
			"600ms debounce drops the write and the next chapter opens with the old settings")
	}
	flush := cutFunction(src[flushAt:])
	if !strings.Contains(flush, "postPrefs(true)") {
		t.Error("flushPrefs does not POST with keepalive; the request can be cancelled " +
			"as the document unloads, which is exactly when it matters")
	}

	zoomAt := strings.Index(src, "function restoreZoom()")
	if zoomAt < 0 {
		t.Fatal("reader.js has no restoreZoom()")
	}
	zoom := cutFunction(src[zoomAt:])
	local := strings.Index(zoom, "gi_zoom_${viewMode}.${scope}")
	srv := strings.Index(zoom, "srvZoom ||")
	if local < 0 || srv < 0 {
		t.Fatalf("restoreZoom layout changed: local=%d srv=%d", local, srv)
	}
	if local > srv {
		t.Error("restoreZoom reads the server snapshot before the local mirror; every page " +
			"turn would snap the zoom back to the chapter's opening value")
	}
}

// cutFunction returns the body of the function the slice starts at, so an
// assertion cannot accidentally match a later function that happens to use the
// same identifier.
func cutFunction(fn string) string {
	if end := strings.Index(fn, "\n  }"); end > 0 {
		return fn[:end]
	}
	return fn
}
