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

// TestBrotliSiblingsMatchSource pins the pre-compressed asset contract.
//
// brHandler serves `<name>.br` whenever the client accepts brotli and only
// falls back to the plain file when the .br is missing or unreadable. The
// siblings are tracked in git and produced by `just br`, so editing a source
// asset without regenerating them ships code the browser never sees — silently,
// with no server-side error. That already happened twice: a stale
// alpine-components.js.br kept serving September code after a fix landed, and
// it read as "the fix did not work".
//
// Decompress each .br and compare it against its source. A mismatch means the
// next build ships old JavaScript.
func TestBrotliSiblingsMatchSource(t *testing.T) {
	entries, err := os.ReadDir(frontendLibDir)
	if err != nil {
		t.Fatal(err)
	}

	var checked int
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".br") {
			continue
		}
		source := filepath.Join(frontendLibDir, strings.TrimSuffix(name, ".br"))
		src, err := os.ReadFile(source)
		if err != nil {
			t.Errorf("%s has no source file: %v", name, err)
			continue
		}

		compressed, err := os.ReadFile(filepath.Join(frontendLibDir, name))
		if err != nil {
			t.Errorf("read %s: %v", name, err)
			continue
		}
		decoded, err := io.ReadAll(brotli.NewReader(bytes.NewReader(compressed)))
		if err != nil {
			t.Errorf("%s is not valid brotli: %v", name, err)
			continue
		}
		if !bytes.Equal(decoded, src) {
			t.Errorf("%s is stale: it decompresses to %d bytes but %s is %d bytes — "+
				"run `just br`", name, len(decoded), filepath.Base(source), len(src))
			continue
		}
		checked++
	}

	if checked == 0 {
		t.Fatal("no .br assets found; this test is not actually checking anything")
	}
}

// TestReaderReadAheadUsesAbortableFetch pins the read-ahead cancellation
// contract in reader.js.
//
// The guarantee is that read-ahead can be abandoned: the reader must not leave
// requests running for pages nobody will read, because each one holds an
// image-lane worker until its upstream read times out. Three properties carry
// that guarantee, and all three have to hold together:
//
//   - read-ahead issues fetch() with an AbortController signal. `new Image()`
//     has no cancellation handle and assigning src = "" does not stop an
//     in-flight response, so a bare Image.src read-ahead cannot be abandoned.
//   - the chapter-switch path aborts both directions, so leaving a chapter
//     tears down read-ahead for the chapter being left.
//   - an in-flight read-ahead is not re-requested on every page turn.
//
// This is a shape pin, not a behavioural test: reader.js is a self-executing
// IIFE that wires DOM events at load time and exposes nothing, so its functions
// cannot be called from a harness. Upgrading this to real coverage means
// lifting the read-ahead bookkeeping out of the IIFE into a DOM-free module —
// worth doing when a second frontend behaviour needs coverage, not before.
func TestReaderReadAheadUsesAbortableFetch(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(frontendLibDir, "reader.js"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)

	// The helpers must be defined before the section that uses them.
	warmAt := strings.Index(src, "function warmImage(")
	commitAt := strings.Index(src, "function commitChapter(")
	if warmAt < 0 {
		t.Fatal("reader.js has no warmImage(); read-ahead lost its abortable fetch path")
	}
	if commitAt < 0 {
		t.Fatal("reader.js has no commitChapter(); cannot locate the chapter-switch path")
	}

	warm := src[warmAt:]

	// 1. Read-ahead must go through fetch with a signal, not Image.src.
	if !strings.Contains(warm, "signal:") {
		t.Error("warmImage does not pass an AbortController signal to fetch; " +
			"read-ahead can no longer be abandoned")
	}
	if !strings.Contains(warm, "catch(") {
		t.Error("warmImage has no catch; an aborted read-ahead would surface as an unhandled rejection")
	}

	// 2. Every prefetch call site must route through warmImage. The prefetch
	//    helpers are the only place read-ahead is issued; a new Image() there
	//    reintroduces the uncancellable path.
	start := strings.Index(src, "function prefetch()")
	end := strings.Index(src, "// ---- Vertical strip mode")
	if start < 0 || end < 0 || end <= start {
		t.Fatal("reader.js layout changed: cannot locate prefetch section")
	}
	prefetch := src[start:end]
	if strings.Contains(prefetch, "new Image()") {
		t.Error("a prefetch helper assigns new Image().src directly; that request " +
			"cannot be cancelled when the reader leaves the chapter")
	}
	if !strings.Contains(prefetch, "warmImage(") {
		t.Error("no prefetch helper routes through warmImage(); read-ahead is not abortable")
	}

	// 3. Chapter switch must abort both directions, and an in-flight read-ahead
	//    must not be re-requested.
	if !strings.Contains(src[commitAt:], "abortAllWarm()") {
		t.Error("commitChapter does not abort read-ahead; switching chapters leaves " +
			"the old chapter's requests running")
	}
	if !strings.Contains(warm, "warming[key]") {
		t.Error("warmImage has no in-flight guard; prefetch() runs on every page turn " +
			"and would re-request a page until its first fetch lands")
	}
}