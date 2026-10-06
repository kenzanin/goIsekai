package httpserver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// reader.js and alpine-components.js are self-wiring IIFEs that expose nothing,
// so their behaviour cannot be unit tested from Go. These are shape pins: they
// fail if the shape that makes export-cbz download its file is removed.
//
// The bug they guard: on job "done" the handler toasted msg.path, a filesystem
// path like app_data/cache/exports/... that no browser can open. The export ran
// and succeeded, and the button looked like it had done nothing.
func TestExportCBZClientActuallyFetchesTheArchive(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "cmd", "goisekai", "frontend", "lib", "alpine-components.js"))
	if err != nil {
		t.Fatalf("read alpine-components.js: %v", err)
	}
	js := string(src)

	// The link comes from the enqueue response, which knows the filename before
	// the job has written anything.
	if !strings.Contains(js, "data?.url") && !strings.Contains(js, "data.url") {
		t.Error("the enqueue response's url is never read; the client has nothing to fetch")
	}
	// Something must actually be clicked, not just displayed.
	if !strings.Contains(js, "createElement('a')") {
		t.Error("no anchor is created, so the archive is never requested")
	}
	if !strings.Contains(js, ".click()") {
		t.Error("the anchor is never clicked, so no download is started")
	}

	// Pin the branch that used to be the whole behaviour.
	done := js[strings.Index(js, "msg.status === 'done'"):]
	if len(done) > 4000 {
		done = done[:4000]
	}
	if !strings.Contains(done, "resultURL") {
		t.Error("the done branch ignores resultURL; it must download from the returned link")
	}
}
