package database

import (
	"path/filepath"
	"testing"
)

// TestPluginGenresRoundTrip covers the three cache states: NULL (never
// fetched), a JSON list, and the "[]" no-export marker.
func TestPluginGenresRoundTrip(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "genres.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err := db.RegisterPlugin(Plugin{ID: "p1", Name: "p1", IsActive: true}); err != nil {
		t.Fatalf("register: %v", err)
	}

	if raw, found, err := db.GetPluginGenres("p1"); err != nil || found || raw != "" {
		t.Fatalf("fresh row: raw=%q found=%v err=%v, want miss", raw, found, err)
	}

	if err := db.SetPluginGenres("p1", `[{"name":"Action","slug":"action"}]`); err != nil {
		t.Fatalf("set: %v", err)
	}
	raw, found, err := db.GetPluginGenres("p1")
	if err != nil || !found || raw != `[{"name":"Action","slug":"action"}]` {
		t.Fatalf("after set: raw=%q found=%v err=%v", raw, found, err)
	}

	if err := db.SetPluginGenres("p1", "[]"); err != nil {
		t.Fatalf("set empty: %v", err)
	}
	raw, found, err = db.GetPluginGenres("p1")
	if err != nil || !found || raw != "[]" {
		t.Fatalf("empty marker: raw=%q found=%v err=%v, want \"[]\"", raw, found, err)
	}

	if _, found, err := db.GetPluginGenres("missing"); err != nil || found {
		t.Fatalf("missing plugin: found=%v err=%v, want miss", found, err)
	}
}
