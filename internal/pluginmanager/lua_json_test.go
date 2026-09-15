package pluginmanager

import (
	"strings"
	"testing"

	"goisekai/internal/hostnet"
)

// luaEval runs one chunk against a Lua state with the production globals
// registered, returning the raw runtime error.
func luaEval(t *testing.T, chunk string) error {
	t.Helper()
	state, err := createLuaState("jsoncheck")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = state.Close() }()

	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	if err := mgr.setupGlobals(state, "jsoncheck"); err != nil {
		t.Fatal(err)
	}
	fn, err := state.Load("chunk.lua", strings.NewReader(chunk))
	if err != nil {
		t.Fatal(err)
	}
	_, err = state.Call(fn.Value())
	return err
}

// TestLuaJSONGlobalRemoved covers the breaking change: the bare json global is
// gone, so the old calls fail instead of quietly working.
func TestLuaJSONGlobalRemoved(t *testing.T) {
	err := luaEval(t, `return json.encode({})`)
	if err == nil {
		t.Fatal("json.encode still resolves; the global was not removed")
	}
	if !strings.Contains(err.Error(), "nil value") {
		t.Fatalf("want an index-a-nil-value error, got: %v", err)
	}
}

// TestLuaJSONReachableThroughHost covers the replacement path: JSON is still
// available, through host.json.
func TestLuaJSONReachableThroughHost(t *testing.T) {
	if err := luaEval(t, `assert(host.json.decode('{"a":1}').a == 1)`); err != nil {
		t.Fatalf("host.json.decode unreachable: %v", err)
	}
}
