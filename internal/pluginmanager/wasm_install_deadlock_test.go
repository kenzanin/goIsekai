package pluginmanager

import (
	"path/filepath"
	"testing"
	"time"

	"goisekai/internal/hostnet"
)

// Install must not deadlock on a wasm plugin.
//
// Install holds m.mu for the whole copy-and-load, and loadWasm reaches
// ensureWasmRuntime, which used to take m.mu again. sync.RWMutex is not
// reentrant, so every wasm plugin install hung the host forever — the UI
// action that installs a .wasm never returned.
//
// The wasm runtime is now guarded by its own lock, so this completes. The
// timeout is the assertion: a regression deadlocks here instead of hanging
// the whole suite.
func TestInstallWasmPluginDoesNotDeadlock(t *testing.T) {
	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	done := make(chan error, 1)
	go func() {
		_, err := mgr.Install(filepath.Join("..", "..", "examples", "plugins", "wasm", "reimanga_net_cpp"))
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Install of a wasm plugin: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Install of a wasm plugin deadlocked: ensureWasmRuntime took a lock the caller already held")
	}
}

// The same path, reached the other way: LoadPlugin hot-loads a wasm plugin
// from a folder and also holds the manager lock while it does.
func TestLoadPluginHotWasmDoesNotDeadlock(t *testing.T) {
	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	done := make(chan error, 1)
	go func() {
		_, err := mgr.LoadPlugin(filepath.Join("..", "..", "examples", "plugins", "wasm", "reimanga_net_cpp"))
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("LoadPlugin of a wasm plugin: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("LoadPlugin of a wasm plugin deadlocked: ensureWasmRuntime took a lock the caller already held")
	}
}
