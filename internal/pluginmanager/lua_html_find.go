package pluginmanager

import (
	lua "github.com/mmcdole/lunar"
	"goisekai/internal/htmldoc"
)

// htmlParse wraps host.html.parse for the Lua runtime: a markup string becomes
// a document handle. On failure it returns nil plus a message, the Lua native
// convention.
func htmlParse(state *lua.State, dt *lua.UserDataType[*htmldoc.Document]) lua.Value {
	v, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		s, ok := frame.String(0)
		if !ok {
			return frame.ReturnValues(lua.Nil(), lua.String(htmlParseArgErr))
		}
		doc, err := htmldoc.Parse(s)
		if err != nil {
			return frame.ReturnValues(lua.Nil(), lua.String(err.Error()))
		}
		handle, err := dt.New(doc)
		if err != nil {
			return frame.ReturnValues(lua.Nil(), lua.String("create handle: "+err.Error()))
		}
		return frame.ReturnValue(handle.Value())
	})
	return v.Value()
}

// htmlFindText wraps host.html.find_text for the Lua runtime.
func htmlFindText(state *lua.State, dt *lua.UserDataType[*htmldoc.Document]) lua.Value {
	v, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		handle, ok := dt.FromArgument(frame, 0)
		if !ok {
			return frame.ReturnValues(lua.Nil(), lua.String("host.html.find_text: argument 0 must be a document handle"))
		}
		selector, ok := frame.String(1)
		if !ok {
			return frame.ReturnValues(lua.Nil(), lua.String("host.html.find_text: argument 1 must be a string"))
		}
		text, err := handle.FindText(selector)
		if err != nil {
			return frame.ReturnValues(lua.Nil(), lua.String(err.Error()))
		}
		return frame.ReturnValue(lua.String(text))
	})
	return v.Value()
}

// htmlFindAttr wraps host.html.find_attr for the Lua runtime.
func htmlFindAttr(state *lua.State, dt *lua.UserDataType[*htmldoc.Document]) lua.Value {
	v, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		handle, ok := dt.FromArgument(frame, 0)
		if !ok {
			return frame.ReturnValues(lua.Nil(), lua.String("host.html.find_attr: argument 0 must be a document handle"))
		}
		selector, ok := frame.String(1)
		if !ok {
			return frame.ReturnValues(lua.Nil(), lua.String("host.html.find_attr: argument 1 must be a string"))
		}
		attr, ok := frame.String(2)
		if !ok {
			return frame.ReturnValues(lua.Nil(), lua.String("host.html.find_attr: argument 2 must be a string"))
		}
		val, err := handle.FindAttr(selector, attr)
		if err != nil {
			return frame.ReturnValues(lua.Nil(), lua.String(err.Error()))
		}
		return frame.ReturnValue(lua.String(val))
	})
	return v.Value()
}

// htmlFindListText wraps host.html.find_list_text for the Lua runtime.
func htmlFindListText(state *lua.State, dt *lua.UserDataType[*htmldoc.Document]) lua.Value {
	v, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		handle, ok := dt.FromArgument(frame, 0)
		if !ok {
			return frame.ReturnValues(lua.Nil(), lua.String("host.html.find_list_text: argument 0 must be a document handle"))
		}
		selector, ok := frame.String(1)
		if !ok {
			return frame.ReturnValues(lua.Nil(), lua.String("host.html.find_list_text: argument 1 must be a string"))
		}
		texts, err := handle.FindListText(selector)
		if err != nil {
			return frame.ReturnValues(lua.Nil(), lua.String(err.Error()))
		}
		tbl, _ := state.NewTable()
		for i, t := range texts {
			_ = tbl.RawSetInt(i+1, lua.String(t))
		}
		return frame.ReturnValue(tbl.Value())
	})
	return v.Value()
}

// htmlFindListAttr wraps host.html.find_list_attr for the Lua runtime.
func htmlFindListAttr(state *lua.State, dt *lua.UserDataType[*htmldoc.Document]) lua.Value {
	v, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		handle, ok := dt.FromArgument(frame, 0)
		if !ok {
			return frame.ReturnValues(lua.Nil(), lua.String("host.html.find_list_attr: argument 0 must be a document handle"))
		}
		selector, ok := frame.String(1)
		if !ok {
			return frame.ReturnValues(lua.Nil(), lua.String("host.html.find_list_attr: argument 1 must be a string"))
		}
		attr, ok := frame.String(2)
		if !ok {
			return frame.ReturnValues(lua.Nil(), lua.String("host.html.find_list_attr: argument 2 must be a string"))
		}
		vals, err := handle.FindListAttr(selector, attr)
		if err != nil {
			return frame.ReturnValues(lua.Nil(), lua.String(err.Error()))
		}
		tbl, _ := state.NewTable()
		for i, v := range vals {
			_ = tbl.RawSetInt(i+1, lua.String(v))
		}
		return frame.ReturnValue(tbl.Value())
	})
	return v.Value()
}
