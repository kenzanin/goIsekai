package pluginmanager

import (
	"encoding/json"
	"goisekai/internal/hostnet"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TrackKnownHosts scans <pluginsDir>/*/plugin.json for site_url values and
// persists the extracted hosts; invalid JSON and missing site_url are
// skipped, and GetKnownHosts reads the file back (nil when absent).
func TestTrackKnownHostsExtractsAndSkips(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	pluginsDir := t.TempDir()

	write := func(dir, name, content string) {
		t.Helper()
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(pluginsDir, "good"), "plugin.json", `{"site_url":"https://one.example.com/path"}`)
	write(filepath.Join(pluginsDir, "second"), "plugin.json", `{"site_url":"https://two.example.org/"}`)
	write(filepath.Join(pluginsDir, "broken"), "plugin.json", `{"site_url":`)
	write(filepath.Join(pluginsDir, "nosite"), "plugin.json", `{"name":"x"}`)

	if err := os.MkdirAll(filepath.Join(home, "app_data"), 0755); err != nil {
		t.Fatal(err)
	}
	m := NewManager(hostnet.NewProxy(), pluginsDir)
	m.TrackKnownHosts()

	data, err := os.ReadFile(filepath.Join(home, "app_data", "known_hosts.json"))
	if err != nil {
		t.Fatalf("known_hosts.json not written: %v", err)
	}
	var hosts []string
	if err := json.Unmarshal(data, &hosts); err != nil {
		t.Fatalf("known_hosts.json is not JSON: %v", err)
	}
	if len(hosts) != 2 || !slices.Contains(hosts, "one.example.com") || !slices.Contains(hosts, "two.example.org") {
		t.Fatalf("hosts = %v, want [one.example.com two.example.org]", hosts)
	}

	// Round-trip through the reader.
	if got := m.GetKnownHosts(); len(got) != 2 {
		t.Fatalf("GetKnownHosts = %v, want 2 entries", got)
	}

	// Empty plugins dir (no plugin.json): writes an empty list, no crash.
	emptyHome := t.TempDir()
	t.Setenv("HOME", emptyHome)
	if err := os.MkdirAll(filepath.Join(emptyHome, "app_data"), 0755); err != nil {
		t.Fatal(err)
	}
	m2 := NewManager(hostnet.NewProxy(), t.TempDir())
	m2.TrackKnownHosts()
	if got := m2.GetKnownHosts(); len(got) != 0 {
		t.Fatalf("GetKnownHosts empty = %v, want []", got)
	}

	// Missing file entirely: nil.
	missingHome := t.TempDir()
	t.Setenv("HOME", missingHome)
	m3 := NewManager(hostnet.NewProxy(), t.TempDir())
	if got := m3.GetKnownHosts(); got != nil {
		t.Fatalf("GetKnownHosts missing = %v, want nil", got)
	}
}
