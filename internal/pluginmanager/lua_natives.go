package pluginmanager

import (
	lua "github.com/mmcdole/lunar"
)

// registerHostNatives installs the shared `host` table
// (text/codecs/crypto/json/http) on the Lua state.
//
// The list of helpers lives in hostSurface (host_surface.go) — this function
// only supplies the Lua calling convention. Adding a helper means adding a row
// there, not editing every runtime.
func registerHostNatives(state *lua.State, m *Manager, id string) {
	htmlTbl, err := htmlGroup(state)
	if err != nil {
		panic(err)
	}
	tables := map[string]*lua.Table{
		"text":    luaNewTable(state),
		"codecs":  luaNewTable(state),
		"crypto":  luaNewTable(state),
		"json":    luaNewTable(state),
		"html":    htmlTbl,
		"regex":   regexGroup(state),
		"http":    luaNewTable(state),
		"browser": luaNewTable(state),
	}

	// Pure helpers: same Go function as the wasm dispatcher and the JS object.
	for _, h := range hostSurface {
		if h.kind == kindRuntime {
			continue
		}
		ns, fn := splitHostName(h.name)
		t, ok := tables[ns]
		if !ok {
			continue
		}
		_ = t.RawSetString(fn, luaPure(state, h))
	}

	// Helpers whose call shape is Lua-specific.
	_ = tables["text"].RawSetString("normalize_status", luaNormalizeStatus(state))
	_ = tables["crypto"].RawSetString("vrf_sign", luaVrfSign(state))
	_ = tables["json"].RawSetString("decode", luaJSONDecode(state))
	_ = tables["json"].RawSetString("encode", luaJSONEncode(state))
	_ = tables["http"].RawSetString("get", luaHttpGet(state, m, id))
	_ = tables["http"].RawSetString("post", luaHttpPost(state, m, id))
	_ = tables["http"].RawSetString("get_body", luaHTTPGetBody(state, m, id))
	_ = tables["http"].RawSetString("post_body", luaHTTPPostBody(state, m, id))

	// host.browser — fetch pages through a real browser (CDP) so
	// client-side JavaScript runs. For sites whose API is signed by their
	// own front-end code, this is the only way to get a valid response.
	_ = tables["browser"].RawSetString("fetch", luaBrowserFetch(state, m, id))
	_ = tables["browser"].RawSetString("evaluate", luaBrowserEvaluate(state, m, id))
	_ = tables["browser"].RawSetString("evaluate_with_init", luaBrowserEvaluateWithInit(state, m, id))

	host, _ := state.NewTable()
	for ns, t := range tables {
		_ = host.RawSetString(ns, t.Value())
	}
	_ = state.RawSetGlobal("host", host.Value())
}

// luaNewTable makes one namespace table, discarding lunar's always-nil error.
func luaNewTable(state *lua.State) *lua.Table {
	t, _ := state.NewTable()
	return t
}

// luaPure wraps one table entry as a Lua native, picking the wrapper that
// matches the helper's kind.
func luaPure(state *lua.State, h hostHelper) lua.Value {
	switch h.kind {
	case kindStr1:
		return luaStr1(state, h.fn.(func(string) string))
	case kindStr1Err:
		return luaStr1Err(state, h.fn.(func(string) (string, error)))
	case kindStr2:
		return luaStr2(state, h.fn.(func(string, string) string))
	case kindStr2Err:
		return luaStr2Err(state, h.fn.(func(string, string) (string, error)))
	case kindStr3Err:
		return luaStr3Err(state, h.fn.(func(string, string, string) (string, error)))
	case kindStr4Err:
		return luaStr4Err(state, h.fn.(func(string, string, string, string) (string, error)))
	case kindFloat1:
		return luaFloat1(state, h.fn.(func(string) float64))
	}
	panic("luaPure: unexpected helper kind for " + h.name)
}
