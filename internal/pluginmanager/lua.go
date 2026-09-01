package pluginmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
	luajson "layeh.com/gopher-json"

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

// loadLua creates a sandboxed Lua VM, loads <dir>/main.lua, reads the PLUGIN
// metadata table, and returns a ready-to-call loadedPlugin.
func (m *Manager) loadLua(id, dir string) (*loadedPlugin, error) {
	L := lua.NewState(lua.Options{SkipOpenLibs: true})

	// Open a curated subset of the standard library — no io, no debug.
	for _, fn := range []lua.LGFunction{
		lua.OpenBase,
		lua.OpenString,
		lua.OpenTable,
		lua.OpenMath,
	} {
		L.Push(L.NewFunction(fn))
		if err := L.PCall(0, lua.MultRet, nil); err != nil {
			L.Close()
			return nil, fmt.Errorf("lua open stdlib: %w", err)
		}
	}

	// Register a hand-built "os" library with only time/date/clock.
	osTable := L.NewTable()
	L.SetField(osTable, "time", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(time.Now().Unix()))
		return 1
	}))
	L.SetField(osTable, "date", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(time.Now().Format(time.RFC3339)))
		return 1
	}))
	L.SetField(osTable, "clock", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(float64(time.Now().UnixNano()) / 1e9))
		return 1
	}))
	L.SetGlobal("os", osTable)

	// Custom require(name): loads sibling .lua modules from the plugin folder
	// only — no package lib, no path search, no cpath, no ".." escapes.
	// (Opening lua.OpenPackage would reintroduce loadfile/dofile-based loaders
	// and a global path search we would have to fight; a 20-line require we
	// own is the smaller, safer surface.)
	moduleCache := map[string]lua.LValue{}
	L.SetGlobal("require", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		if name == "" || name != filepath.Base(name) || strings.Contains(name, "..") {
			L.RaiseError("require: %q is not a plain module name (plugin folder only)", name)
		}
		if v, ok := moduleCache[name]; ok {
			L.Push(v)
			return 1
		}
		if err := L.DoFile(filepath.Join(dir, name+".lua")); err != nil {
			L.RaiseError("require %s: %v", name, err)
		}
		ret := L.Get(-1) // module chunk's return value
		L.Pop(1)
		moduleCache[name] = ret
		L.Push(ret)
		return 1
	}))
	// Harden base: strip file-reading entry points when present.
	L.SetGlobal("dofile", lua.LNil)
	L.SetGlobal("loadfile", lua.LNil)

	// Register json.encode / json.decode as a global table (gopher-json's
	// Preload needs package.preload, which we never open).
	jsonTbl := L.NewTable()
	L.SetField(jsonTbl, "encode", L.NewFunction(func(L *lua.LState) int {
		b, err := luajson.Encode(L.Get(1))
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LString(b))
		return 1
	}))
	L.SetField(jsonTbl, "decode", L.NewFunction(func(L *lua.LState) int {
		v, err := luajson.Decode(L, []byte(L.CheckString(1)))
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(v)
		return 1
	}))
	L.SetGlobal("json", jsonTbl)

	// Register http_request(req_table) global — mirrors hostHTTPRequest proxy.
	L.SetGlobal("http_request", L.NewFunction(func(L *lua.LState) int {
		reqTbl := L.CheckTable(1)
		reqJSON, err := luajson.Encode(reqTbl)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString("http_request encode error: " + err.Error()))
			return 2
		}
		// gopher-json encodes an empty Lua table as a JSON array, so
		// `headers = {}` becomes "headers":[] which fails to unmarshal into
		// map[string]string. Drop empty-array headers before the proxy sees it.
		norm, nerr := normalizeLuaRequest(reqJSON)
		if nerr != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString("http_request normalize: " + nerr.Error()))
			return 2
		}
		respJSON, err := m.proxy.HandleRequest(id, string(norm))
		if err != nil {
			errTbl := L.NewTable()
			errTbl.RawSetString("status", lua.LNumber(0))
			errTbl.RawSetString("error", lua.LString(err.Error()))
			L.Push(errTbl)
			return 1
		}
		respVal, err := luajson.Decode(L, []byte(respJSON))
		if err != nil {
			errTbl := L.NewTable()
			errTbl.RawSetString("status", lua.LNumber(0))
			errTbl.RawSetString("error", lua.LString("decode response: "+err.Error()))
			L.Push(errTbl)
			return 1
		}
		L.Push(respVal)
		return 1
	}))

	// Load main.lua.
	mainPath := filepath.Join(dir, "main.lua")
	if err := L.DoFile(mainPath); err != nil {
		L.Close()
		return nil, fmt.Errorf("lua dofile %s: %w", mainPath, err)
	}

	// Read the PLUGIN metadata table.
	pluginTbl := L.GetGlobal("PLUGIN")
	if pluginTbl == lua.LNil || pluginTbl.Type() != lua.LTTable {
		L.Close()
		return nil, fmt.Errorf("lua plugin %s: PLUGIN global is not a table", id)
	}
	metaJSON, err := luajson.Encode(pluginTbl)
	if err != nil {
		L.Close()
		return nil, fmt.Errorf("lua plugin %s: encode PLUGIN: %w", id, err)
	}
	var meta types.PluginMeta
	if err := json.Unmarshal([]byte(metaJSON), &meta); err != nil {
		L.Close()
		return nil, fmt.Errorf("lua plugin %s: decode PLUGIN metadata: %w", id, err)
	}

	// Resolve contract_version from the PLUGIN table.
	verVal := pluginTbl.(*lua.LTable).RawGetString("contract_version")
	if verVal == lua.LNil {
		L.Close()
		return nil, fmt.Errorf("lua plugin %s: PLUGIN.contract_version missing", id)
	}
	verNum, ok := verVal.(lua.LNumber)
	if !ok {
		L.Close()
		return nil, fmt.Errorf("lua plugin %s: PLUGIN.contract_version not a number", id)
	}
	contractVer := int32(verNum)
	if err := types.CheckVersion(contractVer); err != nil {
		L.Close()
		return nil, fmt.Errorf("lua plugin %s: %w", id, err)
	}

	// Verify all ABI globals are functions (snake_case Lua names).
	for _, name := range luaFnNames {
		fn := L.GetGlobal(name)
		if fn == lua.LNil || fn.Type() != lua.LTFunction {
			L.Close()
			return nil, fmt.Errorf("lua plugin %s: global %q is not a function", id, name)
		}
	}

	logger.Debug("lua plugin loaded", "id", id, "version", contractVer)
	return &loadedPlugin{
		id:              id,
		wasmPath:        mainPath,
		kind:            "lua",
		lua:             L,
		contractVersion: contractVer,
		meta:            meta,
	}, nil
}

// normalizeLuaRequest fixes gopher-json's empty-table encoding for map fields:
// "headers":[] → "headers":{} (dropped) so proxy unmarshal succeeds.
func normalizeLuaRequest(reqJSON []byte) ([]byte, error) {
	var m map[string]any
	if err := json.Unmarshal(reqJSON, &m); err != nil {
		return nil, err
	}
	if h, ok := m["headers"]; ok {
		if _, isEmpty := h.([]any); isEmpty {
			delete(m, "headers")
		}
	}
	return json.Marshal(m)
}

// callLua invokes a Lua ABI function with a JSON string argument under timeout,
// and returns the result as a JSON string.
func callLua(p *loadedPlugin, fnName, inputJSON string) (string, error) {
	luaName, ok := luaFnNames[fnName]
	if !ok {
		return "", fmt.Errorf("lua plugin %s: no lua mapping for %s", p.id, fnName)
	}
	L := p.lua
	fn := L.GetGlobal(luaName)
	if fn == lua.LNil || fn.Type() != lua.LTFunction {
		return "", fmt.Errorf("lua plugin %s: %s is not a function", p.id, fnName)
	}

	ctx, cancel := context.WithTimeout(context.Background(), invokeTimeout)
	defer cancel()
	// gopher-lua LState is not goroutine-safe; the reader fires neighbors +
	// detail + pages concurrently at one VM, which corrupts the stack
	// ("attempt to index a non-table object(function)" et al). Serialize.
	p.mu.Lock()
	defer p.mu.Unlock()
	L.SetContext(ctx)

	if err := L.CallByParam(lua.P{
		Fn:      fn,
		NRet:    1,
		Protect: true,
	}, lua.LString(inputJSON)); err != nil {
		return "", fmt.Errorf("lua plugin %s %s: %w", p.id, fnName, err)
	}
	result := L.Get(-1)
	L.Pop(1)

	switch v := result.(type) {
	case *lua.LNilType:
		return "", fmt.Errorf("lua plugin %s %s: returned nil", p.id, fnName)
	case lua.LBool:
		return "", fmt.Errorf("lua plugin %s %s: returned bool, want string or table", p.id, fnName)
	case *lua.LTable:
		outJSON, err := luajson.Encode(v)
		if err != nil {
			return "", fmt.Errorf("lua plugin %s %s: encode result table: %w", p.id, fnName, err)
		}
		return string(outJSON), nil
	default:
		return v.String(), nil
	}
}
