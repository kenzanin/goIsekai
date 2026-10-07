package pluginmanager

import (
	"os"
	"path/filepath"
	"testing"

	"goisekai/internal/hostnet"
)

// A wasm plugin folder ships the compiled binary next to the main.go it was built
// from. Both discovery passes match it, so whichever runs first wins. It has to
// be wasm: under Yaegi the plugin runs interpreted, with different performance
// and semantics, and the examples/plugins/wasm set was silently loaded that way.
func TestDiscoverPrefersWasmOverYaegiForTheSameFolder(t *testing.T) {
	dir := t.TempDir()
	folder := filepath.Join(dir, "wa")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Both entry points present, exactly like examples/plugins/wasm/*.
	for _, name := range []string{"main.wasm", "main.go"} {
		if err := os.WriteFile(filepath.Join(folder, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	p, ok := mgr.plugins["wa"]
	if !ok {
		t.Fatal("plugin wa was not registered at all")
	}
	if p.kind != "wasm" {
		t.Errorf("plugin wa registered as kind %q, want \"wasm\"", p.kind)
	}
}

// The ordering fix must not swallow a real Yaegi plugin: a folder with only
// main.go still has to be found.
func TestDiscoverStillFindsAPlainYaegiFolder(t *testing.T) {
	dir := t.TempDir()
	folder := filepath.Join(dir, "ya")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(folder, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	p, ok := mgr.plugins["ya"]
	if !ok {
		t.Fatal("plugin ya was not registered")
	}
	if p.kind != "yaegi" {
		t.Errorf("plugin ya registered as kind %q, want \"yaegi\"", p.kind)
	}
}
