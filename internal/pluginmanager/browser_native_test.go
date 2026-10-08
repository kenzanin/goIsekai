package pluginmanager

import (
	"testing"

	"github.com/dop251/goja"

	"goisekai/internal/hostnet"
)

// TestLuaBrowserFetchDisabled verifies host.browser.fetch returns nil when
// CDP is off, rather than erroring. The browser group must exist in the
// registration even when no engine is configured.
func TestLuaBrowserFetchDisabled(t *testing.T) {
	chunk := `
		local html = host.browser.fetch("https://example.com/")
		assert(html == nil, "fetch with CDP off should return nil")
	`
	if err := luaEval(t, chunk); err != nil {
		t.Fatalf("host.browser.fetch (Lua): %v", err)
	}
}

// TestLuaBrowserEvaluateDisabled verifies host.browser.evaluate returns nil
// when CDP is off.
func TestLuaBrowserEvaluateDisabled(t *testing.T) {
	chunk := `
		local out = host.browser.evaluate("https://example.com/", "() => document.title")
		assert(out == nil, "evaluate with CDP off should return nil")
	`
	if err := luaEval(t, chunk); err != nil {
		t.Fatalf("host.browser.evaluate (Lua): %v", err)
	}
}

// TestJSBrowserFetchDisabled verifies host.browser.fetch returns null when
// CDP is off.
func TestJSBrowserFetchDisabled(t *testing.T) {
	vm := goja.New()
	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	if err := registerJSHostNatives(vm, mgr, "test"); err != nil {
		t.Fatalf("registerJSHostNatives: %v", err)
	}
	v, err := vm.RunString(`host.browser.fetch("https://example.com/")`)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !v.SameAs(goja.Null()) {
		t.Fatalf("expected null, got %v", v)
	}
}

// TestJSBrowserEvaluateDisabled verifies host.browser.evaluate returns null
// when CDP is off.
func TestJSBrowserEvaluateDisabled(t *testing.T) {
	vm := goja.New()
	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	if err := registerJSHostNatives(vm, mgr, "test"); err != nil {
		t.Fatalf("registerJSHostNatives: %v", err)
	}
	v, err := vm.RunString(`host.browser.evaluate("https://example.com/", "() => document.title")`)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !v.SameAs(goja.Null()) {
		t.Fatalf("expected null, got %v", v)
	}
}
