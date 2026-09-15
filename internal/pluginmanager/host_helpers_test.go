package pluginmanager

import (
	"testing"

	"github.com/dop251/goja"

	"goisekai/internal/hostnet"
)

// TestLuaLuaEscape covers the host native plugins call instead of shipping
// their own copy. The slug case is the one that bit: '-' is the lazy
// quantifier, so an unescaped slug silently matches the wrong text.
func TestLuaLuaEscape(t *testing.T) {
	chunk := `
		assert(host.text.lua_escape("a-b") == "a%-b", host.text.lua_escape("a-b"))
		assert(host.text.lua_escape("a.b") == "a%.b")
		assert(host.text.lua_escape("50%") == "50%%")
		assert(host.text.lua_escape("a(b)[c]") == "a%(b%)%[c%]")
		assert(host.text.lua_escape("a+b*c?") == "a%+b%*c%?")
		assert(host.text.lua_escape("plain") == "plain")
		-- the whole point: the escaped slug is a literal pattern
		assert(string.find("x-a-b-y", host.text.lua_escape("a-b")) == 3)
	`
	if err := luaEval(t, chunk); err != nil {
		t.Fatalf("host.text.lua_escape: %v", err)
	}
}

// TestLuaNormalizeStatusForms covers both call shapes plugins use: the
// one-argument default-vocabulary form, and the two-argument form with a
// plugin-supplied map. An empty map must mean "use the defaults", not
// "pass everything through untouched".
func TestLuaNormalizeStatusForms(t *testing.T) {
	chunk := `
		assert(host.text.normalize_status("ongoing") == "Ongoing")
		assert(host.text.normalize_status("RELEASING") == "Ongoing")
		assert(host.text.normalize_status("cancelled") == "Dropped")
		assert(host.text.normalize_status("On-Going") == "Ongoing")
		assert(host.text.normalize_status("") == "")
		-- unknown values pass through, not blanked
		assert(host.text.normalize_status("Weird Status") == "Weird Status")
		-- empty map == defaults
		assert(host.text.normalize_status({}, "completed") == "Completed")
		-- explicit map wins
		assert(host.text.normalize_status({serialised = "Ongoing"}, "serialised") == "Ongoing")
	`
	if err := luaEval(t, chunk); err != nil {
		t.Fatalf("host.text.normalize_status: %v", err)
	}
}

// TestJSNormalizeStatusForms mirrors the Lua coverage for the JS runtime,
// which exposes the same callable with a .default property attached.
func TestJSNormalizeStatusForms(t *testing.T) {
	vm := goja.New()
	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	if err := registerJSHostNatives(vm, mgr, "norms"); err != nil {
		t.Fatalf("registerJSHostNatives: %v", err)
	}

	chunk := `
		function assert(c, msg) { if (!c) throw new Error(msg || "assertion failed"); }
		assert(host.text.normalize_status("ongoing") === "Ongoing");
		assert(host.text.normalize_status("RELEASING") === "Ongoing");
		assert(host.text.normalize_status("cancelled") === "Dropped");
		assert(host.text.normalize_status("Weird Status") === "Weird Status");
		assert(host.text.normalize_status({}, "completed") === "Completed");
		assert(host.text.normalize_status({serialised: "Ongoing"}, "serialised") === "Ongoing");
	`
	if _, err := vm.RunString(chunk); err != nil {
		t.Fatalf("host.text helpers in goja: %v", err)
	}
}
