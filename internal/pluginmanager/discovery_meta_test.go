package pluginmanager

import (
	"os"
	"path/filepath"
	"testing"

	"goisekai/internal/hostnet"
)

// Every runtime must get its display name and logo from plugin.json at
// discovery, without instantiating its VM. Before this, only wasm seeded its
// metadata at discovery, so the plugin list showed a blank name/logo for any
// lua/js/yaegi plugin that had not been invoked yet.
func TestDiscoverSeedsMetaFromPluginJSONForEveryKind(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		kind  string
		entry string
	}{
		{"lua", "main.lua"},
		{"js", "main.js"},
		{"yaegi", "main.go"},
		{"wasm", "main.wasm"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			id := "seeded"
			folder := filepath.Join(dir, id)
			if err := os.MkdirAll(folder, 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			// A stub entry point: discovery must not need to execute it.
			if err := os.WriteFile(filepath.Join(folder, tc.entry), []byte("x"), 0o644); err != nil {
				t.Fatalf("write entry: %v", err)
			}
			if err := os.WriteFile(filepath.Join(folder, "plugin.json"),
				[]byte(`{"name":"Seeded","site_url":"https://seeded.test","logo":"logo.png","description":"a blurb"}`), 0o644); err != nil {
				t.Fatalf("write plugin.json: %v", err)
			}

			mgr := NewManager(hostnet.NewProxy(), dir)
			if err := mgr.Discover(); err != nil {
				t.Fatalf("Discover: %v", err)
			}
			defer func() { _ = mgr.Close() }()

			p, ok := mgr.plugins[id]
			if !ok {
				t.Fatalf("plugin %s was not registered", id)
			}
			if p.kind != tc.kind {
				t.Fatalf("kind = %q, want %q", p.kind, tc.kind)
			}
			if p.loaded {
				t.Fatal("discovery must not instantiate the runtime")
			}
			if p.meta.Name != "Seeded" {
				t.Errorf("Name = %q, want %q", p.meta.Name, "Seeded")
			}
			if p.meta.SiteURL != "https://seeded.test" {
				t.Errorf("SiteURL = %q, want %q", p.meta.SiteURL, "https://seeded.test")
			}
			if p.meta.Logo != "logo.png" {
				t.Errorf("Logo = %q, want %q", p.meta.Logo, "logo.png")
			}
			if p.meta.Description != "a blurb" {
				t.Errorf("Description = %q, want %q", p.meta.Description, "a blurb")
			}
		})
	}
}

// A plugin declaring a different ABI version is refused at discovery, before a
// runtime is created — that is the point of moving the version into
// plugin.json. A plugin declaring nothing still loads (the version defaults).
func TestDiscoverRejectsMismatchedContractVersion(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeLuaFolder := func(name, pluginJSON string) {
		folder := filepath.Join(dir, name)
		if err := os.MkdirAll(folder, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(folder, "main.lua"), []byte("PLUGIN = {}"), 0o644); err != nil {
			t.Fatalf("write main.lua: %v", err)
		}
		if pluginJSON != "" {
			if err := os.WriteFile(filepath.Join(folder, "plugin.json"), []byte(pluginJSON), 0o644); err != nil {
				t.Fatalf("write plugin.json: %v", err)
			}
		}
	}
	writeLuaFolder("future", `{"contract_version":99}`)
	writeLuaFolder("silent", "")

	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	if _, ok := mgr.plugins["future"]; ok {
		t.Error("a plugin declaring contract_version 99 must not be registered")
	}
	if _, ok := mgr.plugins["silent"]; !ok {
		t.Error("a plugin declaring no contract_version must still be registered")
	}
}
