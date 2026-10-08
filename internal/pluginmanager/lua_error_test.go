package pluginmanager

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goisekai/internal/hostnet"
)

// TestLuaPluginErrorReasonSurvives locks the ABI contract that a plugin reports
// failure as (nil, reason). The reason is the only thing that distinguishes a
// chapter the site cannot serve from a chapter that genuinely has no pages, so
// collapsing it to "returned nil" throws away the diagnosis the plugin wrote.
// Regression: callLua used to read only vals[0] and discarded vals[1].
func TestLuaPluginErrorReasonSurvives(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "luaerror")
	if err := copyDir("testdata/luaerror", dst); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	_, err := mgr.GetPageList("luaerror", "any-chapter")
	if err == nil {
		t.Fatal("GetPageList returned no error, want the plugin's reason")
	}
	// The host's wording, with the plugin's detail appended.
	if !strings.Contains(err.Error(), "this chapter has no images on the site") {
		t.Fatalf("error did not use the host wording: %v", err)
	}
	if !strings.Contains(err.Error(), "chapter 1") {
		t.Fatalf("error lost the plugin's detail: %v", err)
	}
	// The collapsed form is the regression this guards against.
	if strings.Contains(err.Error(), "returned nil") {
		t.Fatalf("error is still the collapsed form: %v", err)
	}
	// The raw code must not leak through as the user-facing text.
	if strings.Contains(err.Error(), "no_pages:") {
		t.Fatalf("the raw code leaked to the reader instead of the wording: %v", err)
	}
}

// TestLuaPluginCodeBecomesHostWording pins the wiring, not just the renderer:
// callLua has to run the plugin's code through PluginError on the way out. The
// two tests above would still pass if that call were removed, because they only
// prove PluginError renders correctly when called directly.
func TestLuaPluginCodeBecomesHostWording(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "luacode")
	if err := copyDir("testdata/luacode", dst); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	_, err := mgr.GetPageList("luacode", "any-chapter")
	if err == nil {
		t.Fatal("GetPageList returned no error, want the host wording")
	}
	if !strings.Contains(err.Error(), "this chapter has no images on the site (chapter 7)") {
		t.Fatalf("callLua did not map the code to the host wording: %v", err)
	}
	if strings.Contains(err.Error(), "no_pages") {
		t.Fatalf("the raw code reached the reader: %v", err)
	}
}

// TestLuaPluginNilWithoutReasonStillReports guards the other branch: a plugin
// that returns bare nil must keep producing a diagnosable error rather than an
// empty one, so this must NOT be mistaken for an empty page list.
func TestLuaPluginNilWithoutReasonStillReports(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "luabare")
	if err := copyDir("testdata/luaerror", dst); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	// Overwrite the page function with one that returns bare nil.
	if err := writeFileForTest(filepath.Join(dst, "main.lua"), strings.Replace(
		mustRead(t, filepath.Join("testdata/luaerror", "main.lua")),
		`return nil, "no_pages: chapter 1"`, "return nil", 1,
	)); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	_, err := mgr.GetPageList("luabare", "any-chapter")
	if err == nil {
		t.Fatal("GetPageList returned no error for a bare nil return")
	}
	if !strings.Contains(err.Error(), "returned nil") {
		t.Fatalf("bare nil lost its message: %v", err)
	}
}

// TestLuaPluginEmptyListIsNotAnError pins the distinction the reason exists for:
// a plugin that successfully returns an empty list is a valid answer, not a
// failure. If empty became an error, every genuinely empty chapter would 502.
func TestLuaPluginEmptyListIsNotAnError(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "luaempty")
	if err := copyDir("testdata/luatest", dst); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	if err := writeFileForTest(filepath.Join(dst, "main.lua"), strings.Replace(
		mustRead(t, filepath.Join("testdata/luatest", "main.lua")),
		`return host.json.encode({{index = 0, url = "https://example.com/img/1.png"}})`,
		`return host.json.encode({})`, 1,
	)); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	pages, err := mgr.GetPageList("luaempty", "any-chapter")
	if err != nil {
		t.Fatalf("an empty page list must not be an error: %v", err)
	}
	if len(pages) != 0 {
		t.Fatalf("want 0 pages, got %d", len(pages))
	}
}

// mustRead reads a fixture file for string substitution.
func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
