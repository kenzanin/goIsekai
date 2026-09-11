package pluginmanager

import (
	"encoding/json"

	lua "github.com/mmcdole/lunar"

	"goisekai/internal/logger"
)

// setupGlobals registers json, log, and http_request globals on the state.
func (m *Manager) setupGlobals(state *lua.State, id string) error {
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

	registerHostNatives(state)

	return nil
}
