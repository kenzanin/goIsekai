package pluginmanager

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadPluginJSONMeta(t *testing.T) {
	t.Parallel()
	t.Run("valid", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "plugin.json"),
			[]byte(`{"site_url":"https://example.com","name":"Example"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		meta := readPluginJSONMeta(dir)
		if meta.SiteURL != "https://example.com" {
			t.Fatalf("SiteURL = %q", meta.SiteURL)
		}
		if meta.Name != "Example" {
			t.Fatalf("Name = %q", meta.Name)
		}
	})
	t.Run("missing file", func(t *testing.T) {
		if meta := readPluginJSONMeta(t.TempDir()); meta.SiteURL != "" {
			t.Fatalf("want zero meta, got %+v", meta)
		}
	})
	t.Run("malformed", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(`{oops`), 0o644); err != nil {
			t.Fatal(err)
		}
		if meta := readPluginJSONMeta(dir); meta.SiteURL != "" {
			t.Fatalf("want zero meta, got %+v", meta)
		}
	})
	t.Run("empty dir", func(t *testing.T) {
		if meta := readPluginJSONMeta(""); meta.SiteURL != "" {
			t.Fatalf("want zero meta, got %+v", meta)
		}
	})
}
