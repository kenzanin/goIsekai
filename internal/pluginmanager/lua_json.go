package pluginmanager

import (
	"github.com/goccy/go-json"

	lua "github.com/mmcdole/lunar"
)

// jsonDecodeArgErr is the message both runtimes report when host.json.decode is
// handed something that is not a string. Sharing the constant keeps the Lua and
// JS error text identical.
const jsonDecodeArgErr = "host.json.decode: argument must be a string"

// jsonEncodeErr is the message both runtimes report when host.json.encode is
// handed a value with no JSON form. The per-runtime errors behind it (Lua's type
// names, Go's marshaller) differ, so they are collapsed onto one text.
const jsonEncodeErr = "host.json.encode: value cannot be represented as JSON"

// jsonDecodeString decodes a JSON document with the host codec. Both runtimes
// call it so a malformed document produces the same error text in Lua and JS.
func jsonDecodeString(s string) (any, error) {
	var decoded any
	if err := json.Unmarshal([]byte(s), &decoded); err != nil {
		return nil, err
	}
	return decoded, nil
}

// jsonEncodeValue marshals a runtime value with the host codec. Both runtimes
// call it for the same reason as jsonDecodeString.
func jsonEncodeValue(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// luaJSONDecode wraps host.json.decode for the Lua runtime: a JSON string
// becomes a Lua value (table, array, string, number, bool or nil). On failure it
// returns nil plus a message, the convention the other Lua natives use.
func luaJSONDecode(state *lua.State) lua.Value {
	v, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		s, ok := frame.String(0)
		if !ok {
			return frame.ReturnValues(lua.Nil(), lua.String(jsonDecodeArgErr))
		}
		decoded, err := jsonDecodeString(s)
		if err != nil {
			return frame.ReturnValues(lua.Nil(), lua.String(err.Error()))
		}
		luaval, err := goLunarValue(state, decoded)
		if err != nil {
			return frame.ReturnValues(lua.Nil(), lua.String(err.Error()))
		}
		return frame.ReturnValue(luaval)
	})
	return v.Value()
}

// luaJSONEncode wraps host.json.encode for the Lua runtime: a Lua value becomes
// a JSON string. Values with no JSON form (functions, userdata) are reported as
// nil plus a message rather than silently dropped.
func luaJSONEncode(state *lua.State) lua.Value {
	v, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		val, _ := frame.Argument(0)
		goVal, err := lunarToGo(val)
		if err != nil {
			return frame.ReturnValues(lua.Nil(), lua.String(jsonEncodeErr))
		}
		out, err := jsonEncodeValue(goVal)
		if err != nil {
			return frame.ReturnValues(lua.Nil(), lua.String(jsonEncodeErr))
		}
		return frame.ReturnValue(lua.String(out))
	})
	return v.Value()
}
