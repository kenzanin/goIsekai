package httpserver

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
)

// readerSource reads the shipped reader bundle. reader.js is served from disk
// (not embedded), so this is the same bytes the browser gets.
func readerSource(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "cmd", "goisekai", "frontend", "lib", "reader.js"))
	if err != nil {
		t.Fatalf("read reader.js: %v", err)
	}
	return string(data)
}

// The previous chapter used to be warmed only while the reader sat on page 1.
// Pressing Back from anywhere else therefore found no prevPages and paid a cold
// reader-data fetch plus a cold image for every page of the chapter retreated
// into — which is where Back felt slow.
func TestPrevChapterWarmIsNotGatedOnPageOne(t *testing.T) {
	src := readerSource(t)
	if strings.Contains(src, "current === 0 && prevChID") {
		t.Error("reader.js still gates the prev-chapter warm on current === 0; " +
			"Back from the middle of a chapter will find no prevPages")
	}
	if !strings.Contains(src, "prevWarmTried") {
		t.Error("reader.js has no prevWarmTried flag; the eager warm would re-request " +
			"a failing neighbour on every page draw")
	}
}

// The flag has to be cleared wherever prevPages is dropped, or a second visit to
// the same chapter would skip the warm entirely.
func TestPrevWarmFlagResetsWithTheCachedPages(t *testing.T) {
	src := readerSource(t)
	clears := strings.Count(src, "prevWarmTried = false")
	clearsOfPages := strings.Count(src, "prevPages = null")
	if clears < clearsOfPages {
		t.Errorf("prevWarmTried = false appears %d time(s) but prevPages = null %d time(s); "+
			"a cleared page list must re-arm the warm", clears, clearsOfPages)
	}
	if clears < 2 {
		t.Errorf("prevWarmTried = false appears %d time(s), want the declaration plus both "+
			"reset sites (adoptNeighbor and commitChapter)", clears)
	}
}

// The brotli sibling is what the server actually sends. A stale one silently
// ships the old prefetch logic even though reader.js on disk is correct.
func TestReaderBrotliMatchesSource(t *testing.T) {
	path := filepath.Join("..", "..", "cmd", "goisekai", "frontend", "lib", "reader.js.br")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("no brotli sibling: %v", err)
	}
	decompressed, err := brotliDecode(data)
	if err != nil {
		t.Fatalf("reader.js.br is not valid brotli: %v", err)
	}
	if !strings.Contains(decompressed, "prevWarmTried") {
		t.Error("reader.js.br does not contain prevWarmTried; the compressed sibling is stale " +
			"and the browser would run the old prefetch logic. Regenerate it with brotli.")
	}
}

// brotliDecode is a thin wrapper so the test reads clearly.
func brotliDecode(b []byte) (string, error) {
	r := brotli.NewReader(bytes.NewReader(b))
	out, err := io.ReadAll(r)
	return string(out), err
}
