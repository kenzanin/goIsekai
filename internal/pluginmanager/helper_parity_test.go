package pluginmanager

import (
	"context"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/dop251/goja"
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

// TestLuaCoversTheWholeHostSurface is the direction nothing checked before:
// every row in hostSurface must actually be reachable from the Lua runtime. The
// other parity tests start from the Lua table, so an over-declared row (a helper
// listed here that no runtime wires) would sit in the table looking available
// while being unreachable from every plugin.
func TestLuaCoversTheWholeHostSurface(t *testing.T) {
	luaNames := map[string]bool{}
	for _, n := range luaHostHelpers(t) {
		luaNames[n] = true
	}

	var missing []string
	for _, fn := range hostSurfaceNames() {
		if !luaNames[fn] {
			missing = append(missing, fn)
		}
	}
	if len(missing) > 0 {
		t.Errorf("hostSurface declares helpers the Lua runtime does not wire: %v\n"+
			"wire them in registerHostNatives, or drop the row", missing)
	}
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
// the shared host surface must resolve, so a typo in a table key (which would
// make the helper silently unusable) is caught.
func TestWasmDispatchesEveryHelperItAdvertises(t *testing.T) {
	m := NewManager(hostnet.NewProxy(), t.TempDir())
	for _, fn := range hostSurfaceNames() {
		if _, gap := wasmHelperGaps[fn]; gap {
			continue
		}
		if dispatchReportsUnknown(m, fn) {
			t.Errorf("%s is in hostSurface but the dispatcher rejects it", fn)
		}
	}
}

// TestHostSurfaceDeclaresEveryRuntime helper keeps the table honest about the
// entries it leaves to each runtime: a kindRuntime row needs a reason, and a
// kindPure row needs a function, so a half-filled row cannot compile silently.
func TestHostSurfaceRowsAreComplete(t *testing.T) {
	for _, h := range hostSurface {
		ns, fn := splitHostName(h.name)
		if ns == "" || fn == "" {
			t.Errorf("%q is not a <namespace>.<helper> name", h.name)
		}
		if h.kind == kindRuntime {
			if h.fn != nil {
				t.Errorf("%s is kindRuntime but carries a function", h.name)
			}
			if strings.TrimSpace(h.note) == "" {
				t.Errorf("%s is kindRuntime without a note saying why", h.name)
			}
			continue
		}
		if h.fn == nil {
			t.Errorf("%s is kindPure but has no function", h.name)
		}
	}

	seen := map[string]bool{}
	for _, h := range hostSurface {
		if seen[h.name] {
			t.Errorf("%s appears twice in hostSurface", h.name)
		}
		seen[h.name] = true
	}
}

// jsHelperRenames records helpers whose JS spelling differs from Lua's for a
// language reason rather than an oversight. Lua builds a `log` table and calls
// log.warn(msg); JS spells the same levels console.warn(msg), because `console`
// is the global a JS author already reaches for.
var jsHelperRenames = map[string]string{
	"log.debug": "debug",
	"log.info":  "info",
	"log.warn":  "warn",
	"log.error": "error",
}

// TestYaegiSurfaceMatchesHostSurface keeps the two tables from drifting. Every
// yaegi entry that names a hostSurface row must publish the SAME Go function —
// compared by pointer identity, not by output — so a helper can be reimplemented
// on one side without the other silently keeping the old behaviour.
func TestYaegiSurfaceMatchesHostSurface(t *testing.T) {
	var bad []string
	seen := map[string]bool{}

	for _, h := range yaegiHelpers {
		if seen[h.goName] {
			t.Errorf("hostnet.%s appears twice in yaegiHelpers", h.goName)
		}
		seen[h.goName] = true

		if h.helper == "" {
			t.Errorf("hostnet.%s names no hostSurface row", h.goName)
			continue
		}

		row, ok := hostSurfaceIndex[h.helper]
		if !ok {
			t.Errorf("hostnet.%s names %q, which hostSurface does not declare", h.goName, h.helper)
			continue
		}

		// The function yaegi actually publishes.
		published := h.fn
		if published == nil {
			published = row.fn
		}
		if published == nil {
			bad = append(bad, h.goName+" (no function on either side)")
			continue
		}
		// A kindRuntime row carries no function (Lua and JS cannot share its
		// shape), so yaegi supplies its own and there is nothing to compare.
		// Only a kindPure row must publish the identical function.
		if row.fn != nil && reflect.ValueOf(published).Pointer() != reflect.ValueOf(row.fn).Pointer() {
			bad = append(bad, h.goName+" != "+h.helper)
		}
	}

	if len(bad) > 0 {
		t.Errorf("yaegi publishes a different Go function than hostSurface: %v\n"+
			"point both at the same function instead of reimplementing it", bad)
	}
}

// yaegiNotApplicable are hostSurface rows with no yaegi counterpart, each with
// the reason. Listing them beats skipping silently: a new hostSurface row that
// is neither wired nor explained makes TestYaegiSurfaceCoversTheScriptAliases
// fail, so the gap has to be a decision instead of an oversight.
var yaegiNotApplicable = map[string]string{
	"json.decode":  "an interpreted Go plugin calls stdlib encoding/json directly",
	"json.encode":  "an interpreted Go plugin calls stdlib encoding/json directly",
	"regex.gmatch": "Lua iterator protocol; Go ranges over RegexFindAll's slice",
	"plugin.param": "a yaegi plugin reads its params through hostnet.Param, a per-plugin bridge function like Get and Post",
}

// TestYaegiSurfaceCoversTheScriptAliases is the coverage direction: every
// hostSurface row should either be reachable from a yaegi plugin or be listed in
// yaegiNotApplicable with a reason. yaegi used to expose only 11 hand-written
// functions, so nothing kept a new helper from being script-runtime-only.
func TestYaegiSurfaceCoversTheScriptAliases(t *testing.T) {
	published := map[string]bool{}
	for _, h := range yaegiHelpers {
		published[h.helper] = true
	}

	var missing []string
	for _, row := range hostSurface {
		// http/browser/log/html are per-runtime for reasons that apply to yaegi
		// too (they need the manager, plugin id or an opaque handle).
		switch splitNamespace(row.name) {
		case "http", "browser", "log", "html":
			continue
		}
		if published[row.name] {
			continue
		}
		if _, ok := yaegiNotApplicable[row.name]; ok {
			continue
		}
		missing = append(missing, row.name)
	}
	if len(missing) > 0 {
		t.Errorf("hostSurface rows with no yaegi counterpart: %v\n"+
			"add them to yaegiHelpers, or record why in yaegiNotApplicable", missing)
	}

	// The exclusion list must not go stale either.
	for name, reason := range yaegiNotApplicable {
		if strings.TrimSpace(reason) == "" {
			t.Errorf("%s: an exclusion needs a reason", name)
		}
		if _, ok := hostSurfaceIndex[name]; !ok {
			t.Errorf("%s is excluded from yaegi but hostSurface does not declare it", name)
		}
		if published[name] {
			t.Errorf("%s is excluded from yaegi but published anyway; remove the exclusion", name)
		}
	}
}

// TestYaegiExportsResolve guards the map actually handed to the interpreter: a
// row whose helper key no longer exists would be skipped silently.
func TestYaegiExportsResolve(t *testing.T) {
	exports := yaegiExports()
	if len(exports) != len(yaegiHelpers) {
		t.Errorf("yaegiExports published %d of %d entries; some row did not resolve",
			len(exports), len(yaegiHelpers))
	}
	for _, h := range yaegiHelpers {
		if _, ok := exports[h.goName]; !ok {
			t.Errorf("hostnet.%s did not resolve to a function", h.goName)
		}
	}
}

// splitNamespace returns the namespace half of "<namespace>.<helper>".
func splitNamespace(name string) string {
	ns, _ := splitHostName(name)
	return ns
}

// TestJSHostHelperParity closes the gap the table was introduced for: JS used
// to restate the helper list by hand and fell behind Lua (text.trim,
// text.unescape, text.lua_escape were missing). Now every name Lua exposes must
// also be reachable from JS, either at the same path or through a documented
// rename.
func TestJSHostHelperParity(t *testing.T) {
	names := luaHostHelpers(t)

	vm := goja.New()
	if err := vm.Set("console", newJSConsole("parity")); err != nil {
		t.Fatalf("set console: %v", err)
	}
	if err := registerJSHostNatives(vm, NewManager(hostnet.NewProxy(), t.TempDir()), "parity"); err != nil {
		t.Fatalf("registerJSHostNatives: %v", err)
	}
	hostVal := vm.Get("host")
	if hostVal == nil || goja.IsUndefined(hostVal) {
		t.Fatal("JS runtime did not install a host object")
	}
	host := hostVal.ToObject(vm)

	var missing []string
	for _, fn := range names {
		if _, gap := wasmHelperGaps[fn]; gap {
			continue
		}
		if renamed, ok := jsHelperRenames[fn]; ok {
			// console.debug/info/warn/error — see newJSConsole.
			con := vm.Get("console")
			if con == nil || goja.IsUndefined(con) {
				missing = append(missing, fn+" (no console global)")
				continue
			}
			if v := con.ToObject(vm).Get(renamed); v == nil || goja.IsUndefined(v) {
				missing = append(missing, fn+" (no console."+renamed+")")
			}
			continue
		}
		ns, field := splitHostName(fn)
		group := host.Get(ns)
		if group == nil || goja.IsUndefined(group) {
			missing = append(missing, fn+" (no host."+ns+")")
			continue
		}
		if v := group.ToObject(vm).Get(field); v == nil || goja.IsUndefined(v) {
			missing = append(missing, fn)
		}
	}
	if len(missing) > 0 {
		t.Errorf("Lua exposes helpers the JS runtime lacks: %v\n"+
			"add them to hostSurface, or record the gap in wasmHelperGaps", missing)
	}
}

// TestLuaHostHelperWalkIsNonTrivial guards the walk itself: if lunar's table
// API ever stops yielding the nested namespaces, every parity check above would
// pass vacuously against an empty list.
func TestLuaHostHelperWalkIsNonTrivial(t *testing.T) {
	names := luaHostHelpers(t)
	for _, want := range []string{"text.url_encode", "html.find_text", "regex.find", "http.get"} {
		found := slices.Contains(names, want)
		if !found {
			t.Errorf("host table walk missed %s; got %v", want, names)
		}
	}
}
