package pluginmanager

import (
	lua "github.com/mmcdole/lunar"
)

// luaStr1 wraps a string->string helper as a Lua native.
func luaStr1(state *lua.State, fn func(string) string) lua.Value {
	v, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		s, _ := frame.String(0)
		return frame.ReturnValue(lua.String(fn(s)))
	})
	return v.Value()
}

// luaStr1Err wraps a string->(string,error) helper, returning nil+message on error.
func luaStr1Err(state *lua.State, fn func(string) (string, error)) lua.Value {
	v, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		s, _ := frame.String(0)
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
		a, _ := frame.String(0)
		b, _ := frame.String(1)
		return frame.ReturnValue(lua.String(fn(a, b)))
	})
	return v.Value()
}

// luaStr2Err wraps a (string,string)->(string,error) helper.
func luaStr2Err(state *lua.State, fn func(string, string) (string, error)) lua.Value {
	v, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		a, _ := frame.String(0)
		b, _ := frame.String(1)
		out, err := fn(a, b)
		if err != nil {
			return frame.ReturnValues(lua.Nil(), lua.String(err.Error()))
		}
		return frame.ReturnValue(lua.String(out))
	})
	return v.Value()
}
