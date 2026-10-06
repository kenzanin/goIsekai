package bridge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The download endpoint takes a name from the browser, so it is the one place in
// the export path where a traversal would turn "download my export" into "read
// any file the process can open".
func TestOpenExportRefusesToLeaveTheExportDirectory(t *testing.T) {
	s := newTestServiceWithCache(t)

	// A real export is reachable, and its bytes are the ones written.
	dir := filepath.Join(s.exportDir(), "plug", "m1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	want := []byte("PK\x03\x04 pretend zip")
	if err := os.WriteFile(filepath.Join(dir, "Ch. 1.cbz"), want, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	f, info, err := s.OpenExport("plug", "m1", "Ch. 1.cbz")
	if err != nil {
		t.Fatalf("a finished export should open: %v", err)
	}
	if info.Size() != int64(len(want)) {
		t.Errorf("size = %d, want %d", info.Size(), len(want))
	}
	_ = f.Close()

	// Anything that tries to climb out is refused. The secret stands in for
	// whatever the process can read and the browser must not.
	secret := filepath.Join(s.cacheDir, "secret.txt")
	if err := os.WriteFile(secret, []byte("do not serve me"), 0o644); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	for _, name := range []string{
		"../secret.txt",
		"../../secret.txt",
		"..%2fsecret.txt",
		"a/../../secret.txt",
		"..",
	} {
		f, _, err := s.OpenExport("plug", "m1", name)
		if err == nil {
			_ = f.Close()
			t.Errorf("%q was served; it must be refused", name)
			continue
		}
		if f != nil {
			t.Errorf("%q returned an open file alongside an error", name)
		}
	}

	// The plugin and manga segments are paths too, not just the name.
	if _, _, err := s.OpenExport("..", "..", "goisekai.ini"); err == nil {
		t.Error("a traversal through the manga segment was served")
	}
}

// ExportURL is handed to the browser before the job runs, so it has to be the
// same file OpenExport will later serve.
func TestExportURLMatchesWhatOpenExportServes(t *testing.T) {
	s := newTestServiceWithCache(t)
	const title = "Ch. 7: the & one/with slash"
	got := s.ExportURL("plug", "m1", title)

	if !strings.HasPrefix(got, "/exports/plug/m1/") {
		t.Fatalf("url %q should start with /exports/plug/m1/", got)
	}
	// The separator inside the title is neutralised, not encoded into a path.
	if strings.Count(got, "/") != 4 {
		t.Errorf("url %q has extra path segments; the title's slash escaped", got)
	}
	unescaped := strings.ReplaceAll(strings.TrimPrefix(got, "/exports/plug/m1/"), "%2F", "/")
	if !strings.HasSuffix(unescaped, ".cbz") {
		t.Errorf("url %q should end in .cbz", got)
	}
}
