package bridge

import (
	"path/filepath"
	"testing"

	"goisekai/internal/database"
	"goisekai/internal/hostnet"
	"goisekai/internal/pluginmanager"
)

// TestRefreshPluginsDeactivatesMissing verifies rows whose files are gone get
// is_active=0 rather than being deleted (history references plugin_id).
func TestRefreshPluginsDeactivatesMissing(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	proxy := hostnet.NewProxy()
	mgr := pluginmanager.NewManager(proxy, t.TempDir())
	s := NewAppService(db, mgr, proxy, "", t.TempDir(), nil)
	t.Cleanup(s.Shutdown)

	// Stale row: files deleted, still active.
	if err := db.RegisterPlugin(database.Plugin{
		ID: "ghost-plugin", Name: "Ghost", Version: "1",
		WasmPath: filepath.Join(t.TempDir(), "no-such-dir"),
		IsActive: true,
	}); err != nil {
		t.Fatalf("register ghost: %v", err)
	}
	sum, err := s.RefreshPlugins()
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if sum.Deactivated != 1 {
		t.Fatalf("deactivated = %d, want 1", sum.Deactivated)
	}
	plugins, err := db.ListPlugins()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, p := range plugins {
		if p.ID == "ghost-plugin" && p.IsActive {
			t.Fatal("ghost-plugin should be inactive after refresh")
		}
	}
}

// TestRefreshPluginsPurgesBak verifies *.bak.* rows are deleted outright:
// they were never real plugins.
func TestRefreshPluginsPurgesBak(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	proxy := hostnet.NewProxy()
	mgr := pluginmanager.NewManager(proxy, t.TempDir())
	s := NewAppService(db, mgr, proxy, "", t.TempDir(), nil)
	t.Cleanup(s.Shutdown)

	if err := db.RegisterPlugin(database.Plugin{
		ID: "madaradex.wasm.bak.080039", Name: "bak", Version: "1",
		WasmPath: "/no/such/path",
		IsActive: false,
	}); err != nil {
		t.Fatalf("register bak: %v", err)
	}
	sum, err := s.RefreshPlugins()
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if sum.Purged != 1 {
		t.Fatalf("purged = %d, want 1", sum.Purged)
	}
	plugins, err := db.ListPlugins()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, p := range plugins {
		if p.ID == "madaradex.wasm.bak.080039" {
			t.Fatal("bak row should be deleted after refresh")
		}
	}
}
