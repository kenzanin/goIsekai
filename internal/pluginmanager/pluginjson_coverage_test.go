package pluginmanager

import (
	"os"
	"path/filepath"
	"testing"
)

// Metadata lives in plugin.json for every runtime, so a plugin shipped without
// one shows no display name, no logo and no site_url — and the loader has
// nothing left to fall back on. This guards the migration invariant across the
// whole examples tree: every plugin folder carries a plugin.json with a name.
func TestEveryShippedPluginDeclaresMetadata(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..", "examples")
	var folders []string
	for _, pattern := range []string{
		filepath.Join(root, "plugins", "*", "*"),
		filepath.Join(root, "info", "*"),
	} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatalf("glob %s: %v", pattern, err)
		}
		folders = append(folders, matches...)
	}
	if len(folders) == 0 {
		t.Fatal("no plugin folders found — the glob is wrong, not the tree")
	}

	for _, dir := range folders {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}
		name := filepath.Base(dir)

		// A folder that does not define a source ABI is not a plugin (a
		// shared header dir, say); only folders with an entry point count.
		if !hasEntryPoint(dir) {
			continue
		}

		meta := readPluginJSONMeta(dir)
		if meta.Name == "" {
			t.Errorf("%s: plugin.json declares no name", name)
		}
	}
}

// hasEntryPoint reports whether a folder holds a plugin entry point.
func hasEntryPoint(dir string) bool {
	for _, f := range []string{"main.lua", "main.js", "main.wasm", "main.go"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			return true
		}
	}
	return false
}
