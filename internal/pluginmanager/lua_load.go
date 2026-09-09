package pluginmanager

import (
	"bytes"
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
	_ = osTable.RawSetString("time", osTime.Value())

	osDate, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		return frame.ReturnString(time.Now().Format(time.RFC3339))
	})
	_ = osTable.RawSetString("date", osDate.Value())

	osClock, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		return frame.ReturnNumber(float64(time.Now().UnixNano()) / 1e9)
	})
	_ = osTable.RawSetString("clock", osClock.Value())

	_ = state.RawSetGlobal("os", osTable.Value())

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
		ret := lua.Nil()
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
				_ = state.Close()
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
	_ = state.RawSetGlobal("require", reqFn.Value())

	// Harden base: strip file-reading entry points.
	_ = state.RawSetGlobal("dofile", lua.Nil())
	_ = state.RawSetGlobal("loadfile", lua.Nil())

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
	_ = jsonTbl.RawSetString("encode", jsonEncode.Value())

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
	_ = jsonTbl.RawSetString("decode", jsonDecode.Value())

	_ = state.RawSetGlobal("json", jsonTbl.Value())

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
		_ = logTbl.RawSetString(lvlName, fn.Value())
	}
	_ = state.RawSetGlobal("log", logTbl.Value())

	// Register http_request(req_table) global — mirrors hostHTTPRequest proxy.
	httpFn, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		val, _ := frame.Argument(0)
		goVal, err := lunarToGo(val)
		if err != nil {
			return frame.ReturnValue(errorTable(state, "http_request encode error: "+err.Error()))
		}
		// Empty Lua tables encode as [] (ABI array convention), but the
		// proxy expects headers to be an object — normalize before sending.
		if req, ok := goVal.(map[string]any); ok {
			switch req["headers"].(type) {
			case []any, nil:
				req["headers"] = map[string]any{}
			}
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
	_ = state.RawSetGlobal("http_request", httpFn.Value())

	// Load main.lua via state.Load (reader-backed, no ScriptLoader needed).
	mainPath := filepath.Join(dir, "main.lua")
	mainData, err := readFile(mainPath)
	if err != nil {
		_ = state.Close()
		return nil, fmt.Errorf("lua read %s: %w", mainPath, err)
	}
	loaded, err := state.Load("main.lua", bytes.NewReader(mainData))
	if err != nil {
		_ = state.Close()
		return nil, fmt.Errorf("lua load %s: %w", mainPath, err)
	}
	if _, err := state.Call(loaded.Value()); err != nil {
		_ = state.Close()
		return nil, fmt.Errorf("lua exec %s: %w", mainPath, err)
	}

	// Read the PLUGIN metadata table.
	pluginVal, err := state.RawGlobal("PLUGIN")
	if err != nil || pluginVal.IsNil() || pluginVal.Kind() != lua.TableKind {
		_ = state.Close()
		return nil, fmt.Errorf("lua plugin %s: PLUGIN global is not a table", id)
	}
	pluginTbl, _ := pluginVal.AsTable()

	metaJSON, err := lunarTableToJSON(state, pluginTbl)
	if err != nil {
		_ = state.Close()
		return nil, fmt.Errorf("lua plugin %s: encode PLUGIN: %w", id, err)
	}
	var meta types.PluginMeta
	if err := json.Unmarshal(metaJSON, &meta); err != nil {
		_ = state.Close()
		return nil, fmt.Errorf("lua plugin %s: decode PLUGIN metadata: %w", id, err)
	}

	// Resolve contract_version from the PLUGIN table.
	verVal := pluginTbl.RawGetString("contract_version")
	if verVal.IsNil() {
		_ = state.Close()
		return nil, fmt.Errorf("lua plugin %s: PLUGIN.contract_version missing", id)
	}
	verNum, ok := verVal.AsNumber()
	if !ok {
		_ = state.Close()
		return nil, fmt.Errorf("lua plugin %s: PLUGIN.contract_version not a number", id)
	}
	contractVer := int32(verNum)
	if err := types.CheckVersion(contractVer); err != nil {
		_ = state.Close()
		return nil, fmt.Errorf("lua plugin %s: %w", id, err)
	}

	// Verify all ABI globals are functions (snake_case Lua names).
	// GetAltTitles is OPTIONAL (enricher capability) — mirror js.go.
	for abi, name := range luaFnNames {
		if abi == types.GetAltTitlesFunc || abi == types.GetAltSummaryFunc {
			continue
		}
		fn, err := state.RawGlobal(name)
		if err != nil || fn.Kind() != lua.FunctionKind {
			_ = state.Close()
			return nil, fmt.Errorf("lua plugin %s: global %q is not a function", id, name)
		}
	}

	return &loadedPlugin{
		id: id,
		// Return the plugin FOLDER, not the entry file: ensureLoaded passes this
		// path back to loadLua on lazy reload, and loadLua appends "main.lua"
		// itself — a file path here would produce "main.lua/main.lua".
		wasmPath:        dir,
		kind:            "lua",
		loaded:          true,
		lunar:           state,
		contractVersion: contractVer,
		meta:            meta,
	}, nil
}
