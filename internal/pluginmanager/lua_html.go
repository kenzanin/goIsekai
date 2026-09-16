package pluginmanager

import (
	"fmt"
	lua "github.com/mmcdole/lunar"
	"goisekai/internal/htmldoc"
)

// htmlParseArgErr is the message both runtimes report when host.html.parse is
// handed something that is not a string. Keeping it shared ensures Lua and JS
// error text matches.
const htmlParseArgErr = "host.html.parse: argument must be a string"

// htmlXPathText wraps host.html.xpath_text for the Lua runtime.
func htmlXPathText(state *lua.State, dt *lua.UserDataType[*htmldoc.Document]) lua.Value {
	v, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		handle, ok := dt.FromArgument(frame, 0)
		if !ok {
			return frame.ReturnValues(lua.Nil(), lua.String("host.html.xpath_text: argument 0 must be a document handle"))
		}
		expr, ok := frame.String(1)
		if !ok {
			return frame.ReturnValues(lua.Nil(), lua.String("host.html.xpath_text: argument 1 must be a string"))
		}
		text, err := handle.XPathText(expr)
		if err != nil {
			return frame.ReturnValues(lua.Nil(), lua.String(err.Error()))
		}
		return frame.ReturnValue(lua.String(text))
	})
	return v.Value()
}

// htmlXPathAttr wraps host.html.xpath_attr for the Lua runtime.
func htmlXPathAttr(state *lua.State, dt *lua.UserDataType[*htmldoc.Document]) lua.Value {
	v, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		handle, ok := dt.FromArgument(frame, 0)
		if !ok {
			return frame.ReturnValues(lua.Nil(), lua.String("host.html.xpath_attr: argument 0 must be a document handle"))
		}
		expr, ok := frame.String(1)
		if !ok {
			return frame.ReturnValues(lua.Nil(), lua.String("host.html.xpath_attr: argument 1 must be a string"))
		}
		attr, ok := frame.String(2)
		if !ok {
			return frame.ReturnValues(lua.Nil(), lua.String("host.html.xpath_attr: argument 2 must be a string"))
		}
		val, err := handle.XPathAttr(expr, attr)
		if err != nil {
			return frame.ReturnValues(lua.Nil(), lua.String(err.Error()))
		}
		return frame.ReturnValue(lua.String(val))
	})
	return v.Value()
}

// htmlXPathListText wraps host.html.xpath_list_text for the Lua runtime.
func htmlXPathListText(state *lua.State, dt *lua.UserDataType[*htmldoc.Document]) lua.Value {
	v, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		handle, ok := dt.FromArgument(frame, 0)
		if !ok {
			return frame.ReturnValues(lua.Nil(), lua.String("host.html.xpath_list_text: argument 0 must be a document handle"))
		}
		expr, ok := frame.String(1)
		if !ok {
			return frame.ReturnValues(lua.Nil(), lua.String("host.html.xpath_list_text: argument 1 must be a string"))
		}
		texts, err := handle.XPathListText(expr)
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

// htmlXPathListAttr wraps host.html.xpath_list_attr for the Lua runtime.
func htmlXPathListAttr(state *lua.State, dt *lua.UserDataType[*htmldoc.Document]) lua.Value {
	v, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		handle, ok := dt.FromArgument(frame, 0)
		if !ok {
			return frame.ReturnValues(lua.Nil(), lua.String("host.html.xpath_list_attr: argument 0 must be a document handle"))
		}
		expr, ok := frame.String(1)
		if !ok {
			return frame.ReturnValues(lua.Nil(), lua.String("host.html.xpath_list_attr: argument 1 must be a string"))
		}
		attr, ok := frame.String(2)
		if !ok {
			return frame.ReturnValues(lua.Nil(), lua.String("host.html.xpath_list_attr: argument 2 must be a string"))
		}
		vals, err := handle.XPathListAttr(expr, attr)
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

// htmlGroup builds the host.html table with all nine functions. The document
// handle descriptor is created per state and closed over by the natives, so two
// plugin states in one process never share one.
func htmlGroup(state *lua.State) (*lua.Table, error) {
	dt, err := lua.NewUserDataType[*htmldoc.Document](state, "htmldoc.Document")
	if err != nil {
		return nil, fmt.Errorf("create htmldoc handle type: %w", err)
	}
	group, _ := state.NewTable()
	_ = group.RawSetString("parse", htmlParse(state, dt))
	_ = group.RawSetString("find_text", htmlFindText(state, dt))
	_ = group.RawSetString("find_attr", htmlFindAttr(state, dt))
	_ = group.RawSetString("find_list_text", htmlFindListText(state, dt))
	_ = group.RawSetString("find_list_attr", htmlFindListAttr(state, dt))
	_ = group.RawSetString("xpath_text", htmlXPathText(state, dt))
	_ = group.RawSetString("xpath_attr", htmlXPathAttr(state, dt))
	_ = group.RawSetString("xpath_list_text", htmlXPathListText(state, dt))
	_ = group.RawSetString("xpath_list_attr", htmlXPathListAttr(state, dt))
	return group, nil
}
