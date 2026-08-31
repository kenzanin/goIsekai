package database

import (
	"testing"
)

// TestPluginVerifyRoundtrip covers the upsert-then-get and overwrite paths for
// a plugin's verification data (1.3).
func TestPluginVerifyRoundtrip(t *testing.T) {
	db := openTestDB(t)

	first := PluginVerifyRow{PluginID: "p1", VerifyURL: "https://mangadex.org/verify", Cookies: "a=1; b=2", UserAgent: "Mozilla/5.0"}
	if err := db.UpsertPluginVerify(first); err != nil {
		t.Fatalf("upsert 1: %v", err)
	}

	got, ok, err := db.GetPluginVerify("p1")
	if err != nil {
		t.Fatalf("get 1: %v", err)
	}
	if !ok {
		t.Fatal("expected row after first upsert")
	}
	if got.PluginID != "p1" || got.VerifyURL != first.VerifyURL || got.Cookies != first.Cookies || got.UserAgent != first.UserAgent {
		t.Fatalf("roundtrip mismatch: %+v", got)
	}
	if got.UpdatedAt == 0 {
		t.Fatalf("updated_at not stamped: %+v", got)
	}
	firstStamp := got.UpdatedAt

	// Overwrite with different values: the row is updated, not duplicated.
	second := PluginVerifyRow{PluginID: "p1", VerifyURL: "https://mangadex.org/verify2", Cookies: "cf_clearance=xyz", UserAgent: "curl/8"}
	if err := db.UpsertPluginVerify(second); err != nil {
		t.Fatalf("upsert 2: %v", err)
	}
	got, ok, err = db.GetPluginVerify("p1")
	if err != nil {
		t.Fatalf("get 2: %v", err)
	}
	if !ok {
		t.Fatal("expected row after overwrite")
	}
	if got.VerifyURL != second.VerifyURL || got.Cookies != second.Cookies || got.UserAgent != second.UserAgent {
		t.Fatalf("overwrite did not replace values: %+v", got)
	}
	if got.UpdatedAt < firstStamp {
		t.Fatalf("updated_at did not advance: first=%d after=%d", firstStamp, got.UpdatedAt)
	}

	var count int
	if err := db.db.QueryRow(`SELECT COUNT(*) FROM plugin_verify`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row after overwrite, got %d", count)
	}
}

// TestPluginVerifyMissing reports not-found for an unknown plugin.
func TestPluginVerifyMissing(t *testing.T) {
	db := openTestDB(t)

	got, ok, err := db.GetPluginVerify("nope")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if ok {
		t.Fatalf("expected not-found, got %+v", got)
	}
}

// TestThumbRatioPersistence covers the plugins thumb_ratio column: rows written
// without the column (default 0) read back as 0, and RegisterPlugin persists
// and overwrites a declared ratio.
func TestThumbRatioPersistence(t *testing.T) {
	db := openTestDB(t)

	// A row created by older code paths (no thumb_ratio in the INSERT) must
	// surface as 0 via the column default.
	if _, err := db.db.Exec(`INSERT INTO plugins (id, name, version, wasm_path, is_active) VALUES ('p0', 'p0', '1', '/x.wasm', 1)`); err != nil {
		t.Fatalf("raw insert: %v", err)
	}
	plugins, err := db.ListPlugins()
	if err != nil {
		t.Fatalf("ListPlugins: %v", err)
	}
	if got := plugins[0].ThumbRatio; got != 0 {
		t.Fatalf("expected default thumb_ratio 0, got %v", got)
	}

	if err := db.RegisterPlugin(Plugin{ID: "p1", Name: "p1", Version: "1", WasmPath: "/p1.wasm", IsActive: true, ThumbRatio: 0.703}); err != nil {
		t.Fatalf("register with ratio: %v", err)
	}
	plugins, err = db.ListPlugins()
	if err != nil {
		t.Fatalf("ListPlugins: %v", err)
	}
	byID := map[string]Plugin{}
	for _, p := range plugins {
		byID[p.ID] = p
	}
	if got := byID["p1"].ThumbRatio; got != 0.703 {
		t.Fatalf("expected thumb_ratio 0.703, got %v", got)
	}

	// Upsert without a ratio resets the column to 0.
	if err := db.RegisterPlugin(Plugin{ID: "p1", Name: "p1", Version: "1", WasmPath: "/p1.wasm", IsActive: true}); err != nil {
		t.Fatalf("register without ratio: %v", err)
	}
	plugins, err = db.ListPlugins()
	if err != nil {
		t.Fatalf("ListPlugins: %v", err)
	}
	byID = map[string]Plugin{}
	for _, p := range plugins {
		byID[p.ID] = p
	}
	if got := byID["p1"].ThumbRatio; got != 0 {
		t.Fatalf("expected thumb_ratio reset to 0, got %v", got)
	}
}
