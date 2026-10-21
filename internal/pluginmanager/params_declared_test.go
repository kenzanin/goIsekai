package pluginmanager

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// A plugin reads a param by name, and an undeclared name reads as "". That is
// silent: the plugin keeps running and builds URLs out of an empty string.
//
// So every param a shipped plugin reads must be declared in its plugin.json.
// This guards the migration that moved the site facts out of the Lua sources
// into the metadata.
func TestEveryParamAReadPluginDeclares(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..", "examples", "plugins")
	read := regexp.MustCompile(`host\.plugin\.param\(\s*["']([a-zA-Z0-9_]+)["']`)

	for _, kind := range []string{"lua", "js", "wasm", "yaegi", "native"} {
		pattern := filepath.Join(root, kind, "*")
		dirs, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatalf("glob %s: %v", pattern, err)
		}
		for _, dir := range dirs {
			name := filepath.Base(dir)

			declared := map[string]bool{}
			meta, err := os.ReadFile(filepath.Join(dir, "plugin.json"))
			if err == nil {
				var m struct {
					SiteURL string            `json:"site_url"`
					Params  map[string]string `json:"params"`
				}
				if err := json.Unmarshal(meta, &m); err != nil {
					t.Errorf("%s/%s: plugin.json is not valid JSON: %v", kind, name, err)
					continue
				}
				for k := range m.Params {
					declared[k] = true
				}
				// The host exposes the site origin as a param of its own.
				if m.SiteURL != "" {
					declared["site_url"] = true
				}
			}

			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				ext := filepath.Ext(e.Name())
				if ext != ".lua" && ext != ".js" && ext != ".go" {
					continue
				}
				data, err := os.ReadFile(filepath.Join(dir, e.Name()))
				if err != nil {
					continue
				}
				for _, m := range read.FindAllStringSubmatch(string(data), -1) {
					if !declared[m[1]] {
						t.Errorf("%s/%s/%s reads param %q, but plugin.json does not declare it",
							kind, name, e.Name(), m[1])
					}
				}
			}
		}
	}
}
