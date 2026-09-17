package pluginmanager

import (
	lua "github.com/mmcdole/lunar"
)

// Argument coercion: the wrappers below use CoerceString, so a plugin
// passing a JSON number reaches a string helper intact instead of as "".
//
// luaStr1 wraps a string->string helper as a Lua native.
func luaStr1(state *lua.State, fn func(string) string) lua.Value {
	v, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		s, _ := frame.CoerceString(0)
		return frame.ReturnValue(lua.String(fn(s)))
	})
	return v.Value()
}

// luaStr1Err wraps a string->(string,error) helper, returning nil+message on error.
func luaStr1Err(state *lua.State, fn func(string) (string, error)) lua.Value {
	v, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		s, _ := frame.CoerceString(0)
		out, err := fn(s)
		if err != nil {
			return frame.ReturnValues(lua.Nil(), lua.String(err.Error()))
		}
		return frame.ReturnValue(lua.String(out))
	})
	return v.Value()
}

// luaStr2 wraps a (string,string)->string helper as a Lua native.
func luaStr2(state *lua.State, fn func(string, string) string) lua.Value {
	v, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		a, _ := frame.CoerceString(0)
		b, _ := frame.CoerceString(1)
		return frame.ReturnValue(lua.String(fn(a, b)))
	})
	return v.Value()
}

// luaFloat1 wraps a string->float64 helper as a Lua native.
func luaFloat1(state *lua.State, fn func(string) float64) lua.Value {
	v, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		s, _ := frame.CoerceString(0)
		return frame.ReturnValue(lua.Number(fn(s)))
	})
	return v.Value()
}

// luaStr2Err wraps a (string,string)->(string,error) helper.
func luaStr2Err(state *lua.State, fn func(string, string) (string, error)) lua.Value {
	v, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		a, _ := frame.CoerceString(0)
		b, _ := frame.CoerceString(1)
		out, err := fn(a, b)
		if err != nil {
			return frame.ReturnValues(lua.Nil(), lua.String(err.Error()))
		}
		return frame.ReturnValue(lua.String(out))
	})
	return v.Value()
}
