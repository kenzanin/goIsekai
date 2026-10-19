package pluginmanager

import (
	"context"
	"sort"
	"strings"
	"testing"

	lua "github.com/mmcdole/lunar"

	"goisekai/internal/hostnet"
)

// This file is the drift alarm for the host helper surface. The three runtimes
// (lua, js, wasm) are supposed to expose the same helpers, but nothing enforced
// that: a helper added to the Lua table silently stayed missing from the wasm
// dispatch, and only a plugin port would discover it. The tests below read the
// REAL Lua host table and compare it against the wasm dispatcher, so adding a
// helper to one side only makes the suite red on the other.

// wasmHelperGaps lists the Lua host helpers the wasm dispatch deliberately does
// not implement, each with the reason. The list is meant to shrink: once a gap
// is wired up, TestWasmHelperGapListStaysHonest fails until its entry is
// removed, so this cannot rot into a stale lie about what wasm supports.
var wasmHelperGaps = map[string]string{
	"regex.gmatch": "Lua iterator protocol has no JSON equivalent; wasm uses regex.find_all",
}

// unknownHostFn is the sentinel dispatchHostCall reports for a name it has no
// case for. Only this error means "missing"; any other error (bad input, a
// cancelled proxy request) still proves the name is wired.
const unknownHostFn = "unknown host function"

// luaHostHelpers walks the production host table and returns every
// "<namespace>.<name>" it exposes, plus the "log.*" globals registered beside
// it. Reading the real table rather than a literal list is the whole point: a
// helper added to one runtime only shows up here.
func luaHostHelpers(t *testing.T) []string {
	t.Helper()
	state, err := createLuaState("parity")
	if err != nil {
		t.Fatalf("create state: %v", err)
	}
	defer func() { _ = state.Close() }()
	mgr := NewManager(hostnet.NewProxy(), t.TempDir())
	if err := mgr.setupGlobals(state, "parity"); err != nil {
		t.Fatalf("setup globals: %v", err)
	}

	var out []string
	hostVal, err := state.RawGlobal("host")
	if err != nil {
		t.Fatalf("host global: %v", err)
	}
	hostTbl, ok := hostVal.AsTable()
	if !ok {
		t.Fatal("host is not a table")
	}
	for key, val, more, err := hostTbl.Next(lua.Nil()); more; key, val, more, err = hostTbl.Next(key) {
		if err != nil {
			t.Fatalf("walk host: %v", err)
		}
		ns, ok := key.AsString()
		if !ok {
			continue
		}
		sub, ok := val.AsTable()
		if !ok {
			continue
		}
		for k, _, more, err := sub.Next(lua.Nil()); more; k, _, more, err = sub.Next(k) {
			if err != nil {
				t.Fatalf("walk host.%s: %v", ns, err)
			}
			name, ok := k.AsString()
			if !ok {
				continue
			}
			out = append(out, ns+"."+name)
		}
	}

	// log.* are globals rather than host.*, but they are part of the same
	// helper surface a plugin programs against.
	logVal, err := state.RawGlobal("log")
	if err != nil {
		t.Fatalf("log global: %v", err)
	}
	if logTbl, ok := logVal.AsTable(); ok {
		for k, _, more, err := logTbl.Next(lua.Nil()); more; k, _, more, err = logTbl.Next(k) {
			if err != nil {
				t.Fatalf("walk log: %v", err)
			}
			name, ok := k.AsString()
			if !ok {
				continue
			}
			out = append(out, "log."+name)
		}
	}

	sort.Strings(out)
	return out
}

// dispatchReportsUnknown calls one wasm helper and reports whether the
// dispatcher rejected the name. Panics count as "reachable": a helper that
// blows up on the dummy input below is wired, just not exercised properly here.
func dispatchReportsUnknown(m *Manager, fn string) bool {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // stops http/browser helpers from touching the network

	unknown := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				unknown = false
			}
		}()
		_, err := m.dispatchHostCall(ctx, "parity", fn, nil)
		unknown = err != nil && strings.Contains(err.Error(), unknownHostFn)
	}()
	return unknown
}

// TestWasmHostHelperParity fails when the Lua tree exposes a helper the wasm
// dispatch cannot reach. Either implement it in dispatchHostCall, or record it
// in wasmHelperGaps with a reason.
func TestWasmHostHelperParity(t *testing.T) {
	names := luaHostHelpers(t)
	if len(names) < 30 {
		t.Fatalf("walked only %d helpers; the host table walk is broken", len(names))
	}
	m := NewManager(hostnet.NewProxy(), t.TempDir())

	var missing []string
	for _, fn := range names {
		if _, gap := wasmHelperGaps[fn]; gap {
			continue
		}
		if dispatchReportsUnknown(m, fn) {
			missing = append(missing, fn)
		}
	}
	if len(missing) > 0 {
		t.Errorf("Lua exposes helpers the wasm dispatch rejects: %v\n"+
			"implement them in dispatchHostCall, or list them in wasmHelperGaps with a reason", missing)
	}
}

// TestWasmHelperGapListStaysHonest keeps wasmHelperGaps from going stale: every
// entry must exist on the Lua side and must genuinely still be missing from the
// wasm dispatch. Wiring a gap up therefore forces its entry out.
func TestWasmHelperGapListStaysHonest(t *testing.T) {
	names := map[string]bool{}
	for _, n := range luaHostHelpers(t) {
		names[n] = true
	}
	m := NewManager(hostnet.NewProxy(), t.TempDir())

	for fn, reason := range wasmHelperGaps {
		if strings.TrimSpace(reason) == "" {
			t.Errorf("%s: a gap entry needs a reason", fn)
		}
		if !names[fn] {
			t.Errorf("%s is in wasmHelperGaps but Lua does not expose it (stale or misspelled)", fn)
		}
		if !dispatchReportsUnknown(m, fn) {
			t.Errorf("%s is in wasmHelperGaps but the wasm dispatch reaches it; remove the entry", fn)
		}
	}
}

// TestWasmDispatchesEveryHelperItAdvertises is the mirror image: every name in
// the wasm dispatch tables must resolve, so a typo in a map key (which would
// make the helper silently unusable) is caught.
func TestWasmDispatchesEveryHelperItAdvertises(t *testing.T) {
	m := NewManager(hostnet.NewProxy(), t.TempDir())
	for fn := range wasmHostStr1 {
		if dispatchReportsUnknown(m, fn) {
			t.Errorf("%s is in wasmHostStr1 but the dispatcher rejects it", fn)
		}
	}
	for fn := range wasmHostStr1Err {
		if dispatchReportsUnknown(m, fn) {
			t.Errorf("%s is in wasmHostStr1Err but the dispatcher rejects it", fn)
		}
	}
	for fn := range wasmHostStr2 {
		if dispatchReportsUnknown(m, fn) {
			t.Errorf("%s is in wasmHostStr2 but the dispatcher rejects it", fn)
		}
	}
	for fn := range wasmHostStr2Err {
		if dispatchReportsUnknown(m, fn) {
			t.Errorf("%s is in wasmHostStr2Err but the dispatcher rejects it", fn)
		}
	}
	for fn := range wasmHostStr3Err {
		if dispatchReportsUnknown(m, fn) {
			t.Errorf("%s is in wasmHostStr3Err but the dispatcher rejects it", fn)
		}
	}
	for fn := range wasmHostStr4Err {
		if dispatchReportsUnknown(m, fn) {
			t.Errorf("%s is in wasmHostStr4Err but the dispatcher rejects it", fn)
		}
	}
}

// TestLuaHostHelperWalkIsNonTrivial guards the walk itself: if lunar's table
// API ever stops yielding the nested namespaces, every parity check above would
// pass vacuously against an empty list.
func TestLuaHostHelperWalkIsNonTrivial(t *testing.T) {
	names := luaHostHelpers(t)
	for _, want := range []string{"text.url_encode", "html.find_text", "regex.find", "http.get"} {
		found := false
		for _, n := range names {
			if n == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("host table walk missed %s; got %v", want, names)
		}
	}
}
