package pluginmanager

import (
	"context"
	"fmt"

	lua "github.com/mmcdole/lunar"

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
	types.GetAltTitlesFunc:   "getAltTitles",
	types.GetAltSummaryFunc:  "getAltSummary",
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
	defer func() { _ = state.RemoveContext() }()

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
