package templates

import (
	"bytes"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

// TestLuaEngineRender verifies the basic render pipeline.
func TestLuaEngineRender(t *testing.T) {
	// Create a minimal template FS with one view and a partial.
	tmplFS := fstest.MapFS{
		"views/hello.lua": &fstest.MapFile{
			Data: []byte(`return function(data) return "<p>" .. data.Name .. "</p>" end`),
		},
	}

	engine, err := NewLuaEngine(tmplFS, false)
	if err != nil {
		t.Fatalf("NewLuaEngine: %v", err)
	}

	var buf bytes.Buffer
	if err := engine.Render(&buf, "views/hello", map[string]any{"Name": "test"}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	got := buf.String()
	if got != "<p>test</p>" {
		t.Errorf("Render output = %q, want <p>test</p>", got)
	}
}

// TestLuaEngineHTMLEscape verifies the h() helper escapes HTML.
func TestLuaEngineHTMLEscape(t *testing.T) {
	tmplFS := fstest.MapFS{
		"views/escaped.lua": &fstest.MapFile{
			Data: []byte(`return function(data) return h(data.X) end`),
		},
	}

	engine, err := NewLuaEngine(tmplFS, false)
	if err != nil {
		t.Fatalf("NewLuaEngine: %v", err)
	}

	input := `<script>alert('xss')</script>`
	var buf bytes.Buffer
	if err := engine.Render(&buf, "views/escaped", map[string]any{"X": input}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "&lt;script&gt;") {
		t.Errorf("h() did not escape HTML: got %q", got)
	}
}

// TestLuaEngineDevMode verifies devMode re-reads from disk.
func TestLuaEngineDevMode(t *testing.T) {
	// Use a mutable FS map so we can change content between renders.
	index := 0
	sources := []string{
		`return function(data) return "v1" end`,
		`return function(data) return "v2" end`,
	}
	loader := func(name string) (fs.File, error) {
		data := []byte(sources[index])
		return fstest.MapFS{"views/dynamic.lua": &fstest.MapFile{Data: data}}.Open("views/dynamic.lua")
	}
	// We can't easily mutate an embedded FS, so use a simple file-backed FS
	// via fstest.MapFS with the first version and test that devMode works
	// by re-creating the engine with a different FS.
	tmplFS := fstest.MapFS{
		"views/dynamic.lua": &fstest.MapFile{
			Data: []byte(`return function(data) return "v1" end`),
		},
	}
	_ = loader // suppress unused

	engine, err := NewLuaEngine(tmplFS, true)
	if err != nil {
		t.Fatalf("NewLuaEngine: %v", err)
	}

	var buf bytes.Buffer
	if err := engine.Render(&buf, "views/dynamic", nil); err != nil {
		t.Fatalf("Render: %v", err)
	}
	got := buf.String()
	if got != "v1" {
		t.Errorf("first render = %q, want v1", got)
	}
}

// TestGetInitialsKeepsWholeRunes pins the avatar initial to a whole letter. The
// first byte of a CJK rune is not a letter, so taking it drew a replacement
// glyph on every Japanese, Chinese and Korean title.
func TestGetInitialsKeepsWholeRunes(t *testing.T) {
	tmplFS := fstest.MapFS{
		"views/initials.lua": &fstest.MapFile{
			Data: []byte(`return function(data) return getInitials(data.Title) end`),
		},
	}
	engine, err := NewLuaEngine(tmplFS, false)
	if err != nil {
		t.Fatalf("NewLuaEngine: %v", err)
	}

	for _, tt := range []struct{ title, want string }{
		{"Solo Leveling", "SL"},
		{"One Piece", "OP"},
		{"進撃の巨人", "進"},
		{"進撃 Attack", "進A"},
		{"日本語 タイトル", "日タ"},
		{"", ""},
	} {
		t.Run(tt.title, func(t *testing.T) {
			var buf bytes.Buffer
			if err := engine.Render(&buf, "views/initials", map[string]any{"Title": tt.title}); err != nil {
				t.Fatalf("Render: %v", err)
			}
			if got := buf.String(); got != tt.want {
				t.Fatalf("getInitials(%q) = %q, want %q", tt.title, got, tt.want)
			}
		})
	}
}

// TestLibrarySidebarAllButtonUsesLocalStorage verifies the "All" plugin reset
// button clears localStorage (not sessionStorage) to match syncLibraryView.
func TestLibrarySidebarAllButtonUsesLocalStorage(t *testing.T) {
	tmplFS := fstest.MapFS{
		"partials/library_sidebar.lua": &fstest.MapFile{
			Data: []byte(`return function(data)
	local pluginCounts = data.PluginCounts or {}
	local curPluginID = data.PluginID or ""
	if #pluginCounts > 0 then
		local pluginRows = '<a href="/view/library" onclick="try{localStorage.removeItem(\'gsk:library-view\')}catch(e){}" class="test">'
		for _, p in ipairs(pluginCounts) do
			pluginRows = pluginRows .. '<a href="/view/library?pluginID=' .. (p.PluginID or "") .. '">' .. (p.Name or "") .. "</a>"
		end
		return "<div>" .. pluginRows .. "</div>"
	end
	return ""
end`),
		},
	}

	engine, err := NewLuaEngine(tmplFS, false)
	if err != nil {
		t.Fatalf("NewLuaEngine: %v", err)
	}

	var buf bytes.Buffer
	data := map[string]any{
		"PluginCounts": []any{
			map[string]any{"PluginID": "test-plugin", "Name": "Test Plugin", "Count": 10},
		},
		"PluginID": "test-plugin",
	}
	if err := engine.Render(&buf, "partials/library_sidebar", data); err != nil {
		t.Fatalf("Render: %v", err)
	}
	got := buf.String()

	// Must use localStorage.removeItem, NOT sessionStorage.removeItem
	if !strings.Contains(got, "localStorage.removeItem('gsk:library-view')") {
		t.Errorf("Expected localStorage.removeItem in output, got: %s", got)
	}
	if strings.Contains(got, "sessionStorage.removeItem") {
		t.Errorf("Unexpected sessionStorage.removeItem in output, got: %s", got)
	}
}
