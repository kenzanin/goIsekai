package templates

import (
	"io"
	"testing"
	"testing/fstest"
)

func benchEngine(b *testing.B) *LuaEngine {
	b.Helper()
	tmplFS := fstest.MapFS{
		"views/bench.lua": &fstest.MapFile{Data: []byte(`return function(data) return "<div>" .. tostring(data.n) .. "</div>" end`)},
	}
	engine, err := NewLuaEngine(tmplFS, false)
	if err != nil {
		b.Fatalf("NewLuaEngine: %v", err)
	}
	return engine
}

// BenchmarkLuaRender measures the full render path as served per HTML request.
func BenchmarkLuaRender(b *testing.B) {
	engine := benchEngine(b)
	data := map[string]any{"n": 42, "active": "x"}
	b.ReportAllocs()
	for b.Loop() {
		if err := engine.Render(io.Discard, "views/bench", data); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLuaNewVM isolates the per-render VM setup (libraries + helpers)
// from template execution, to show whether VM creation dominates a render.
func BenchmarkLuaNewVM(b *testing.B) {
	engine := benchEngine(b)
	b.ReportAllocs()
	for b.Loop() {
		s, err := engine.newVM()
		if err != nil {
			b.Fatal(err)
		}
		_ = s.Close()
	}
}
