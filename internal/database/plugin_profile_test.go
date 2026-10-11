package database

import (
	"testing"
)

// Plugin profile pins persist per-plugin TLS choices: set shows up in the
// map, clearing with "" drops the row from the map, and unpinned plugins
// never appear.
func TestPluginProfilePinAndClear(t *testing.T) {
	db := openTestDB(t)
	if err := db.RegisterPlugin(Plugin{ID: "p1", Name: "P1", IsActive: true}); err != nil {
		t.Fatalf("register p1: %v", err)
	}
	if err := db.RegisterPlugin(Plugin{ID: "p2", Name: "P2", IsActive: true}); err != nil {
		t.Fatalf("register p2: %v", err)
	}

	if err := db.SetPluginProfile("p1", "chrome120"); err != nil {
		t.Fatalf("set pin: %v", err)
	}
	profiles, err := db.GetPluginProfiles()
	if err != nil {
		t.Fatalf("get profiles: %v", err)
	}
	if profiles["p1"] != "chrome120" {
		t.Fatalf("p1 pin = %q, want chrome120", profiles["p1"])
	}
	if _, ok := profiles["p2"]; ok {
		t.Fatalf("unpinned p2 must not appear in map: %v", profiles)
	}

	// Clearing with "" must drop the pin from the map.
	if err := db.SetPluginProfile("p1", ""); err != nil {
		t.Fatalf("clear pin: %v", err)
	}
	profiles, err = db.GetPluginProfiles()
	if err != nil {
		t.Fatalf("get profiles after clear: %v", err)
	}
	if _, ok := profiles["p1"]; ok {
		t.Fatalf("cleared p1 must not appear in map: %v", profiles)
	}
}
