package bridge

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goisekai/internal/database"
	"goisekai/internal/hostnet"
	"goisekai/internal/pluginmanager"
)

// newLogoService builds an AppService with a temp cache dir and a real plugins
// dir so logo-cache paths and PluginDir have something to key off. It returns
// the cacheDir and pluginsDir strings so tests can locate files on disk.
func newLogoService(t *testing.T) (*AppService, string, string, *database.DB, *pluginmanager.Manager) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	pluginsDir := t.TempDir()
	mgr := pluginmanager.NewManager(nil, pluginsDir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("discover: %v", err)
	}
	cacheDir := t.TempDir()
	s := NewAppService(db, mgr, hostnet.NewProxy(), "", cacheDir, nil)
	t.Cleanup(s.Shutdown)
	return s, cacheDir, pluginsDir, db, mgr
}

// registerWasmFixture creates <pluginsDir>/<id>/main.wasm + plugin.json so the
// manager (and PluginDir) see a real wasm plugin, then re-discovers.
func registerWasmFixture(t *testing.T, pluginsDir string, mgr *pluginmanager.Manager, id, logoSource string) {
	t.Helper()
	dir := filepath.Join(pluginsDir, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.wasm"), []byte("wasm"), 0o644); err != nil {
		t.Fatalf("write main.wasm: %v", err)
	}
	jsonBody := `{"site_url":"https://example.com"}`
	if logoSource != "" {
		jsonBody = `{"site_url":"https://example.com","logo":"` + logoSource + `"}`
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(jsonBody), 0o644); err != nil {
		t.Fatalf("write plugin.json: %v", err)
	}
	if err := mgr.Discover(); err != nil {
		t.Fatalf("rediscover: %v", err)
	}
}

// isPNG reports whether b starts with the PNG magic.
func isPNG(b []byte) bool {
	return len(b) > 4 && b[0] == 0x89 && b[1] == 'P' && b[2] == 'N' && b[3] == 'G'
}

// A URL logo is fetched via the proxy and materialised into the per-plugin
// cache so the next page view renders without a live fetch. The proxy in the
// test has no live network, so the fetch is best-effort: assert on the helper
// contract — presence/absence agree, and when present the path shape is right.
func TestCachePluginLogoURL(t *testing.T) {
	s, cacheDir, pluginsDir, db, mgr := newLogoService(t)
	registerWasmFixture(t, pluginsDir, mgr, "urlplug", "https://cdn.example.com/logo.png")
	if err := db.RegisterPlugin(database.Plugin{ID: "urlplug", WasmPath: "x", IsActive: true}); err != nil {
		t.Fatalf("register plugin row: %v", err)
	}

	s.CachePluginLogo("urlplug")

	cached := s.PluginLogoCacheFile("urlplug", "logo.png")
	got := s.LogoCacheURL("urlplug")
	if (cached == "") != (got == "") {
		t.Errorf("PluginLogoCacheFile and LogoCacheURL disagree: path=%q url=%q", cached, got)
	}
	if got != "" {
		want := "/plugin-static/urlplug/logo.png"
		if got != want {
			t.Errorf("LogoCacheURL() = %q, want %q", got, want)
		}
		if !strings.HasPrefix(cached, filepath.Join(cacheDir, "images", "plugins")) {
			t.Errorf("cache path %q not under the image-cache plugins tree", cached)
		}
	}
}

// An inline (data: URI) logo is decoded and cached without any network.
func TestCachePluginLogoInline(t *testing.T) {
	s, _, pluginsDir, db, mgr := newLogoService(t)
	// Minimal 1x1 transparent PNG.
	pngB64 := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1SD+0AAAAFElEQVR4nGNgYGBgAAAABQAAh3M0ywAAAAAASUVORK5CYII="
	registerWasmFixture(t, pluginsDir, mgr, "inlineplug", "data:image/png;base64,"+pngB64)
	if err := db.RegisterPlugin(database.Plugin{ID: "inlineplug", WasmPath: "x", IsActive: true}); err != nil {
		t.Fatalf("register plugin row: %v", err)
	}

	s.CachePluginLogo("inlineplug")

	url := s.LogoCacheURL("inlineplug")
	if url == "" {
		t.Fatal("LogoCacheURL empty after caching an inline logo")
	}
	if !strings.Contains(url, "/plugin-static/inlineplug/") {
		t.Errorf("LogoCacheURL = %q, want a /plugin-static/inlineplug/ URL", url)
	}
	cached := s.PluginLogoCacheFile("inlineplug", filepath.Base(url[strings.LastIndex(url, "/")+1:]))
	if cached == "" {
		t.Fatalf("PluginLogoCacheFile empty for %q", url)
	}
	b, err := os.ReadFile(cached)
	if err != nil {
		t.Fatalf("read cached logo: %v", err)
	}
	if len(b) == 0 {
		t.Error("cached inline logo is empty")
	}
	if !isPNG(b) {
		t.Error("cached inline logo does not look like a PNG")
	}
}

// ClearAllPluginLogos wipes the per-plugin tree so a refresh re-resolves from
// the declared source instead of a stale copy.
func TestClearAllPluginLogos(t *testing.T) {
	s, cacheDir, pluginsDir, db, mgr := newLogoService(t)
	pngB64 := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1SD+0AAAAFElEQVR4nGNgYGBgAAAABQAAh3M0ywAAAAAASUVORK5CYII="
	registerWasmFixture(t, pluginsDir, mgr, "plugA", "data:image/png;base64,"+pngB64)
	if err := db.RegisterPlugin(database.Plugin{ID: "plugA", WasmPath: "x", IsActive: true}); err != nil {
		t.Fatalf("register plugin row: %v", err)
	}

	s.CachePluginLogo("plugA")
	if s.LogoCacheURL("plugA") == "" {
		t.Fatal("expected a cached logo before clear")
	}

	if err := s.ClearAllPluginLogos(); err != nil {
		t.Fatalf("ClearAllPluginLogos: %v", err)
	}
	if got := s.LogoCacheURL("plugA"); got != "" {
		t.Errorf("LogoCacheURL after clear = %q, want empty", got)
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "images", "plugins")); !os.IsNotExist(err) {
		t.Error("images/plugins tree still present after ClearAllPluginLogos")
	}
}

// A bare-filename logo is read straight from the plugin's own directory (the
// common case, including the wasm logo.png fallback) and cached.
func TestCachePluginLogoFile(t *testing.T) {
	s, cacheDir, pluginsDir, db, mgr := newLogoService(t)
	// A plugin that declares a bare filename logo; place the file in its dir.
	registerWasmFixture(t, pluginsDir, mgr, "fileplug", "") // no logo in plugin.json
	if err := db.RegisterPlugin(database.Plugin{ID: "fileplug", WasmPath: "x", IsActive: true}); err != nil {
		t.Fatalf("register plugin row: %v", err)
	}
	// A 1x1 PNG written beside main.wasm.
	pngBytes, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1SD+0AAAAFElEQVR4nGNgYGBgAAAABQAAh3M0ywAAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatalf("decode fixture png: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginsDir, "fileplug", "logo.png"), pngBytes, 0o644); err != nil {
		t.Fatalf("write logo.png: %v", err)
	}

	s.CachePluginLogo("fileplug")
	got := s.LogoCacheURL("fileplug")
	if got != "/plugin-static/fileplug/logo.png" {
		t.Errorf("LogoCacheURL = %q, want %q", got, "/plugin-static/fileplug/logo.png")
	}
	cached := s.PluginLogoCacheFile("fileplug", "logo.png")
	if !strings.HasPrefix(cached, filepath.Join(cacheDir, "images", "plugins")) {
		t.Errorf("cache path %q not under the plugins tree", cached)
	}
	b, err := os.ReadFile(cached)
	if err != nil {
		t.Fatalf("read cached logo: %v", err)
	}
	if !isPNG(b) {
		t.Error("cached file logo is not a PNG")
	}
}
