package pluginmanager

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	lua "github.com/mmcdole/lunar"

	"goisekai/internal/logger"
	"goisekai/pkg/types"
)

// luaFnNames maps host ABI function names to the snake_case globals a Lua
// plugin defines (the Lua-facing ABI; design.md). One map drives both the
// load-time verification and the call-time dispatch.
var luaFnNames = map[string]string{
	types.SearchFunc:         "search_manga",
	types.GetMangaDetailFunc: "get_manga_detail",
	types.GetChapterListFunc: "get_chapter_list",
	types.GetPageListFunc:    "get_page_list",
}

// loadLua creates a sandboxed Lunar 5.4 VM, loads <dir>/main.lua, reads the
// PLUGIN metadata table, and returns a ready-to-call loadedPlugin.
func (m *Manager) loadLua(id, dir string) (*loadedPlugin, error) {
	// NO ScriptLoader: prevents the package library from registering require.
	// We register our own sandboxed require below.
	state, err := lua.New(lua.Options{
		Libraries: lua.LibrarySet{
			lua.BaseLibrary,
			lua.StringLibrary,
			lua.TableLibrary,
			lua.MathLibrary,
		},
		MaxHeapBytes: 64 << 20, // 64 MiB heap cap
	})
	if err != nil {
		return nil, fmt.Errorf("lua new state: %w", err)
	}

	// Register a hand-built "os" library with only time/date/clock.
	osTable, _ := state.NewTable()

	osTime, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		return frame.ReturnNumber(float64(time.Now().Unix()))
	})
	osTable.RawSetString("time", osTime.Value())

	osDate, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		return frame.ReturnString(time.Now().Format(time.RFC3339))
	})
	osTable.RawSetString("date", osDate.Value())

	osClock, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		return frame.ReturnNumber(float64(time.Now().UnixNano()) / 1e9)
	})
	osTable.RawSetString("clock", osClock.Value())

	state.RawSetGlobal("os", osTable.Value())

	// Custom require(name): loads sibling .lua modules from the plugin folder
	// only — no package lib, no path search, no cpath, no ".." escapes.
	// ponytail: Lunar forbids re-entering state.Call from a native callback,
	// so modules are pre-executed in Go before main.lua (alphabetical order);
	// a module requiring a later-alphabetical sibling fails. Add a topo-sort
	// here if a plugin ever needs that.
	moduleCache := map[string]lua.Value{}

	preload := func(name string) error {
		if _, exists := moduleCache[name]; exists {
			return nil
		}
		data, err := readFile(filepath.Join(dir, name+".lua"))
		if err != nil {
			return err
		}
		loaded, err := state.Load(name+".lua", bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("load: %w", err)
		}
		results, err := state.Call(loaded.Value())
		if err != nil {
			return err
		}
		var ret lua.Value = lua.Nil()
		if len(results) > 0 {
			ret = results[0]
		}
		moduleCache[name] = ret
		return nil
	}

	// Pre-execute every sibling module except main.lua before main runs, so
	// require() inside main (and inside earlier modules) is a cache hit.
	if entries, err := os.ReadDir(dir); err == nil {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			n := e.Name()
			if !e.IsDir() && strings.HasSuffix(n, ".lua") && n != "main.lua" {
				names = append(names, strings.TrimSuffix(n, ".lua"))
			}
		}
		sort.Strings(names)
		for _, n := range names {
			if err := preload(n); err != nil {
				state.Close()
				return nil, fmt.Errorf("lua plugin %s: preload %s: %w", id, n, err)
			}
		}
	}

	reqFn, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		name, ok := frame.String(0)
		if !ok {
			frame.ThrowError(fmt.Errorf("require: argument must be a string"))
			return frame.ReturnNil()
		}
		if name == "" || name != filepath.Base(name) || strings.Contains(name, "..") {
			frame.ThrowError(fmt.Errorf("require: %q is not a plain module name (plugin folder only)", name))
			return frame.ReturnNil()
		}
		if v, exists := moduleCache[name]; exists {
			return frame.ReturnValue(v)
		}
		// Not preloaded: either missing or required lazily from a callback,
		// which Lunar forbids (no re-entry). Try an idle-time load anyway:
		// reachable only if main.lua is somehow re-executing top-level code.
		if err := preload(name); err != nil {
			frame.ThrowError(fmt.Errorf("require %s: %v", name, err))
			return frame.ReturnNil()
		}
		return frame.ReturnValue(moduleCache[name])
	})
	state.RawSetGlobal("require", reqFn.Value())

	// Harden base: strip file-reading entry points.
	state.RawSetGlobal("dofile", lua.Nil())
	state.RawSetGlobal("loadfile", lua.Nil())

	// Register json.encode / json.decode as a global table.
	jsonTbl, _ := state.NewTable()

	jsonEncode, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		val, _ := frame.Argument(0)
		goVal, err := lunarToGo(val)
		if err != nil {
			return frame.ReturnValues(lua.Nil(), lua.String(err.Error()))
		}
		b, err := json.Marshal(goVal)
		if err != nil {
			return frame.ReturnValues(lua.Nil(), lua.String(err.Error()))
		}
		return frame.ReturnValue(lua.String(string(b)))
	})
	jsonTbl.RawSetString("encode", jsonEncode.Value())

	jsonDecode, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		s, ok := frame.String(0)
		if !ok {
			return frame.ReturnValues(lua.Nil(), lua.String("json.decode: argument must be a string"))
		}
		var goVal any
		if err := json.Unmarshal([]byte(s), &goVal); err != nil {
			return frame.ReturnValues(lua.Nil(), lua.String(err.Error()))
		}
		luaval, err := goLunarValue(state, goVal)
		if err != nil {
			return frame.ReturnValues(lua.Nil(), lua.String(err.Error()))
		}
		return frame.ReturnValue(luaval)
	})
	jsonTbl.RawSetString("decode", jsonDecode.Value())

	state.RawSetGlobal("json", jsonTbl.Value())

	// Register log.debug/info/warn/error(msg, ...) globals.
	logTbl, _ := state.NewTable()
	for lvlName, logFn := range map[string]func(string, ...any){
		"debug": logger.Debug,
		"info":  logger.Info,
		"warn":  logger.Warn,
		"error": logger.Error,
	} {
		fn, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
			msg, _ := frame.String(0)
			logFn(msg, "plugin", id)
			return frame.Return()
		})
		logTbl.RawSetString(lvlName, fn.Value())
	}
	state.RawSetGlobal("log", logTbl.Value())

	// Register http_request(req_table) global — mirrors hostHTTPRequest proxy.
	httpFn, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		val, _ := frame.Argument(0)
		goVal, err := lunarToGo(val)
		if err != nil {
			return frame.ReturnValue(errorTable(state, "http_request encode error: "+err.Error()))
		}
		reqJSON, err := json.Marshal(goVal)
		if err != nil {
			return frame.ReturnValue(errorTable(state, "http_request marshal: "+err.Error()))
		}
		respJSON, err := m.proxy.HandleRequest(id, string(reqJSON))
		if err != nil {
			return frame.ReturnValue(errorTable(state, err.Error()))
		}
		var respVal any
		if err := json.Unmarshal([]byte(respJSON), &respVal); err != nil {
			return frame.ReturnValue(errorTable(state, "decode response: "+err.Error()))
		}
		luaval, err := goLunarValue(state, respVal)
		if err != nil {
			return frame.ReturnValue(errorTable(state, "convert response: "+err.Error()))
		}
		return frame.ReturnValue(luaval)
	})
	state.RawSetGlobal("http_request", httpFn.Value())

	// Load main.lua via state.Load (reader-backed, no ScriptLoader needed).
	mainPath := filepath.Join(dir, "main.lua")
	mainData, err := readFile(mainPath)
	if err != nil {
		state.Close()
		return nil, fmt.Errorf("lua read %s: %w", mainPath, err)
	}
	loaded, err := state.Load("main.lua", bytes.NewReader(mainData))
	if err != nil {
		state.Close()
		return nil, fmt.Errorf("lua load %s: %w", mainPath, err)
	}
	if _, err := state.Call(loaded.Value()); err != nil {
		state.Close()
		return nil, fmt.Errorf("lua exec %s: %w", mainPath, err)
	}

	// Read the PLUGIN metadata table.
	pluginVal, err := state.RawGlobal("PLUGIN")
	if err != nil || pluginVal.IsNil() || pluginVal.Kind() != lua.TableKind {
		state.Close()
		return nil, fmt.Errorf("lua plugin %s: PLUGIN global is not a table", id)
	}
	pluginTbl, _ := pluginVal.AsTable()

	metaJSON, err := lunarTableToJSON(state, pluginTbl)
	if err != nil {
		state.Close()
		return nil, fmt.Errorf("lua plugin %s: encode PLUGIN: %w", id, err)
	}
	var meta types.PluginMeta
	if err := json.Unmarshal(metaJSON, &meta); err != nil {
		state.Close()
		return nil, fmt.Errorf("lua plugin %s: decode PLUGIN metadata: %w", id, err)
	}

	// Resolve contract_version from the PLUGIN table.
	verVal := pluginTbl.RawGetString("contract_version")
	if verVal.IsNil() {
		state.Close()
		return nil, fmt.Errorf("lua plugin %s: PLUGIN.contract_version missing", id)
	}
	verNum, ok := verVal.AsNumber()
	if !ok {
		state.Close()
		return nil, fmt.Errorf("lua plugin %s: PLUGIN.contract_version not a number", id)
	}
	contractVer := int32(verNum)
	if err := types.CheckVersion(contractVer); err != nil {
		state.Close()
		return nil, fmt.Errorf("lua plugin %s: %w", id, err)
	}

	// Verify all ABI globals are functions (snake_case Lua names).
	for _, name := range luaFnNames {
		fn, err := state.RawGlobal(name)
		if err != nil || fn.Kind() != lua.FunctionKind {
			state.Close()
			return nil, fmt.Errorf("lua plugin %s: global %q is not a function", id, name)
		}
	}

	return &loadedPlugin{
		id:              id,
		wasmPath:        mainPath,
		kind:            "lua",
		loaded:          true,
		lunar:           state,
		contractVersion: contractVer,
		meta:            meta,
	}, nil
}

// callLua invokes a Lua ABI function with a JSON string argument under timeout,
// and returns the result as a JSON string.
func callLua(p *loadedPlugin, fnName, inputJSON string) (string, error) {
	luaName, ok := luaFnNames[fnName]
	if !ok {
		return "", fmt.Errorf("lua plugin %s: no lua mapping for %s", p.id, fnName)
	}

	// Lunar State is not goroutine-safe; serialize per-plugin access.
	p.mu.Lock()
	defer p.mu.Unlock()

	state := p.lunar

	fnVal, err := state.RawGlobal(luaName)
	if err != nil || fnVal.Kind() != lua.FunctionKind {
		return "", fmt.Errorf("lua plugin %s: %s is not a function", p.id, fnName)
	}

	// Set a wall-clock deadline on the State's context.
	ctx, cancel := context.WithTimeout(context.Background(), invokeTimeout)
	defer cancel()
	if err := state.SetContext(ctx); err != nil {
		return "", fmt.Errorf("lua plugin %s %s: set context: %w", p.id, fnName, err)
	}
	defer state.RemoveContext()

	vals, err := state.Call(fnVal, lua.String(inputJSON))
	if err != nil {
		return "", fmt.Errorf("lua plugin %s %s: %w", p.id, fnName, err)
	}
	if len(vals) == 0 {
		return "", fmt.Errorf("lua plugin %s %s: returned no values", p.id, fnName)
	}
	res := vals[0]
	switch {
	case res.IsNil():
		return "", fmt.Errorf("lua plugin %s %s: returned nil", p.id, fnName)
	case res.Kind() == lua.BoolKind:
		return "", fmt.Errorf("lua plugin %s %s: returned bool, want string or table", p.id, fnName)
	case res.Kind() == lua.TableKind:
		tbl, _ := res.AsTable()
		jsonBytes, err := lunarTableToJSON(state, tbl)
		if err != nil {
			return "", fmt.Errorf("lua plugin %s %s: encode result table: %w", p.id, fnName, err)
		}
		return string(jsonBytes), nil
	default:
		s, _ := res.AsString()
		return s, nil
	}
}

// ---------------------------------------------------------------------------
// Lunar ↔ Go value conversion (replaces gopher-json entirely)
// ---------------------------------------------------------------------------

// lunarToGo converts a Lunar Value to a Go interface{} suitable for json.Marshal.
func lunarToGo(val lua.Value) (any, error) {
	switch val.Kind() {
	case lua.NilKind:
		return nil, nil
	case lua.BoolKind:
		b, _ := val.AsBool()
		return b, nil
	case lua.NumberKind:
		n, _ := val.AsNumber()
		return n, nil
	case lua.StringKind:
		s, _ := val.AsString()
		return s, nil
	case lua.TableKind:
		tbl, _ := val.AsTable()
		// Functions in tables are silently dropped (not JSON-serializable).
		goVal, err := tableToGoMap(tbl)
		if err != nil {
			return nil, err
		}
		return goVal, nil
	default:
		return nil, fmt.Errorf("unsupported Lua type: %s", val.Kind())
	}
}

// tableToGoMap iterates a Lunar table using tbl.Next and returns a map or slice.
func tableToGoMap(tbl *lua.Table) (any, error) {
	if tbl == nil {
		return nil, nil
	}
	// First pass: collect all key-value pairs.
	type kv struct {
		key   lua.Value
		value lua.Value
		isInt bool
		intK  int
	}
	var entries []kv
	var cur lua.Value = lua.Nil()
	for {
		k, v, ok, err := tbl.Next(cur)
		if err != nil {
			return nil, fmt.Errorf("table iteration: %w", err)
		}
		if !ok {
			break
		}
		entry := kv{key: k, value: v}
		if k.Kind() == lua.NumberKind {
			n, _ := k.AsNumber()
			if n == float64(int(n)) && n >= 1 {
				entry.isInt = true
				entry.intK = int(n)
			}
		}
		entries = append(entries, entry)
		cur = k
	}

	if len(entries) == 0 {
		// Empty Lua table is ambiguous; ABI results are arrays, so emit [].
		return []any{}, nil
	}

	// If all keys are sequential integers 1..n, return a slice.
	allInt := true
	maxInt := 0
	for _, e := range entries {
		if !e.isInt {
			allInt = false
			break
		}
		if e.intK > maxInt {
			maxInt = e.intK
		}
	}
	if allInt && maxInt == len(entries) {
		arr := make([]any, len(entries))
		for _, e := range entries {
			goVal, err := lunarToGo(e.value)
			if err != nil {
				return nil, err
			}
			arr[e.intK-1] = goVal
		}
		return arr, nil
	}

	// Map-like: string keys.
	m := make(map[string]any, len(entries))
	for _, e := range entries {
		if e.key.Kind() == lua.StringKind {
			k, _ := e.key.AsString()
			goVal, err := lunarToGo(e.value)
			if err != nil {
				return nil, err
			}
			m[k] = goVal
		}
	}
	return m, nil
}

// lunarTableToJSON marshals a Lunar table directly to JSON by walking it via tbl.Next.
func lunarTableToJSON(state *lua.State, tbl *lua.Table) ([]byte, error) {
	goVal, err := tableToGoMap(tbl)
	if err != nil {
		return nil, err
	}
	return json.Marshal(goVal)
}

// goLunarValue converts a Go interface{} (from json.Unmarshal) to a Lunar Value.
func goLunarValue(state *lua.State, v any) (lua.Value, error) {
	switch val := v.(type) {
	case nil:
		return lua.Nil(), nil
	case bool:
		return lua.Bool(val), nil
	case float64:
		return lua.Number(val), nil
	case string:
		return state.String(val), nil
	case []any:
		tbl, err := state.NewTable()
		if err != nil {
			return lua.Nil(), err
		}
		for i, item := range val {
			lv, err := goLunarValue(state, item)
			if err != nil {
				return lua.Nil(), err
			}
			tbl.RawSetInt(i+1, lv) // Lua 1-indexed
		}
		return tbl.Value(), nil
	case map[string]any:
		tbl, err := state.NewTable()
		if err != nil {
			return lua.Nil(), err
		}
		for k, item := range val {
			lv, err := goLunarValue(state, item)
			if err != nil {
				return lua.Nil(), err
			}
			tbl.RawSetString(k, lv)
		}
		return tbl.Value(), nil
	default:
		return state.String(fmt.Sprintf("%v", v)), nil
	}
}

// errorTable creates a Lua table {status=0, error=msg} for proxy error returns.
func errorTable(state *lua.State, msg string) lua.Value {
	tbl, _ := state.NewTable()
	tbl.RawSetString("status", lua.Number(0))
	tbl.RawSetString("error", state.String(msg))
	return tbl.Value()
}

// readFile reads a file into memory. Used for loading .lua files without
// ScriptLoader (state.Load takes an io.Reader).
func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
