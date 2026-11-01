package pluginmanager

import (
	"testing"

	"modernc.org/quickjs"

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
// CDP is off. A JS null crosses into Go as a nil any.
func TestJSBrowserFetchDisabled(t *testing.T) {
	vm, err := quickjs.NewVM()
	if err != nil {
		t.Fatalf("NewVM: %v", err)
	}
	defer func() { _ = vm.Close() }()

	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	if err := registerJSHostNatives(vm, mgr, "test"); err != nil {
		t.Fatalf("registerJSHostNatives: %v", err)
	}
	v, err := vm.Eval(`host.browser.fetch("https://example.com/")`, quickjs.EvalGlobal)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if v != nil {
		t.Fatalf("expected null, got %v", v)
	}
}

// TestJSBrowserEvaluateDisabled verifies host.browser.evaluate returns null
// when CDP is off.
func TestJSBrowserEvaluateDisabled(t *testing.T) {
	vm, err := quickjs.NewVM()
	if err != nil {
		t.Fatalf("NewVM: %v", err)
	}
	defer func() { _ = vm.Close() }()

	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	if err := registerJSHostNatives(vm, mgr, "test"); err != nil {
		t.Fatalf("registerJSHostNatives: %v", err)
	}
	v, err := vm.Eval(`host.browser.evaluate("https://example.com/", "() => document.title")`, quickjs.EvalGlobal)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if v != nil {
		t.Fatalf("expected null, got %v", v)
	}
}
