package pluginmanager

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// knownPluginErrorCodes mirrors the codes a plugin is allowed to return. Kept as
// a literal rather than derived from the map so the test still fails if a code
// is added without the test being updated.
var knownPluginErrorCodes = []string{
	"no_pages", "upstream_unavailable", "upstream_auth_failed",
	"decrypt_key_mismatch", "decrypt_failed", "envelope_unrecognised",
}

// TestPluginsReportCodedErrors stops plugins from inventing their own wording.
// Every (nil, "...") reason must be a known code, optionally with ": detail" —
// otherwise the reader sees whatever sentence that plugin happened to write,
// which is what produced eleven phrasings of the same few failures.
func TestPluginsReportCodedErrors(t *testing.T) {
	root := filepath.Join("..", "..", "examples", "plugins")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Skipf("no plugin tree: %v", err)
	}
	known := map[string]bool{}
	for _, c := range knownPluginErrorCodes {
		known[c] = true
	}

	found := 0
	for _, kind := range entries {
		kindDir := filepath.Join(root, kind.Name())
		plugins, err := os.ReadDir(kindDir)
		if err != nil {
			continue
		}
		for _, p := range plugins {
			path := filepath.Join(kindDir, p.Name(), "main.lua")
			src, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			for i, line := range strings.Split(string(src), "\n") {
				const marker = `return nil, "`
				_, after, ok := strings.Cut(line, marker)
				if !ok {
					continue
				}
				rest := after
				before0, _, ok0 := strings.Cut(rest, `"`)
				if !ok0 {
					continue
				}
				reason := before0
				code, _, _ := strings.Cut(reason, ":")
				if !known[code] {
					t.Errorf("%s:%d returns an uncoded plugin error %q - plugins must return a code "+
						"from the documented set so the host owns the wording", path, i+1, reason)
				}
				found++
			}
		}
	}
	if found == 0 {
		t.Fatal("found no (nil, reason) returns to check; the scan is broken")
	}
	t.Logf("checked %d coded error returns", found)
}

// TestEveryCodeIsUsedAtLeastSomewhere keeps the registry honest: a code nothing
// can raise is dead weight, and the distinctness test would keep passing.
func TestEveryCodeIsUsedAtLeastSomewhere(t *testing.T) {
	root := filepath.Join("..", "..", "examples", "plugins")
	used := map[string]bool{}
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".lua") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		for _, code := range knownPluginErrorCodes {
			// Match the bare code and the "code: detail" form alike.
			if strings.Contains(string(src), `"`+code) || strings.Contains(string(src), `"`+code+`"`) {
				used[code] = true
			}
		}
		return nil
	})
	for _, c := range knownPluginErrorCodes {
		if !used[c] {
			t.Errorf("code %q is defined but no plugin returns it", c)
		}
	}
}
