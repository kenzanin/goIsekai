package pluginmanager

import (
	lua "github.com/mmcdole/lunar"
)

// luaBrowserFetch wraps host.browser.fetch(url) → rendered HTML, or nil when
// the browser is unavailable or the fetch failed.
func luaBrowserFetch(state *lua.State, m *Manager, id string) lua.Value {
	fn, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		url, _ := frame.CoerceString(0)
		html, err := m.proxy.BrowserFetch(id, url)
		if err != nil {
			return frame.ReturnValue(lua.Nil())
		}
		return frame.ReturnValue(lua.String(html))
	})
	return fn.Value()
}

// luaBrowserEvaluate wraps host.browser.evaluate(url, js) → result string, or
// nil when the browser is unavailable or the evaluation failed.
func luaBrowserEvaluate(state *lua.State, m *Manager, id string) lua.Value {
	fn, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		url, _ := frame.CoerceString(0)
		js, _ := frame.CoerceString(1)
		out, err := m.proxy.BrowserEvaluate(id, url, js)
		if err != nil {
			return frame.ReturnValue(lua.Nil())
		}
		return frame.ReturnValue(lua.String(out))
	})
	return fn.Value()
}

// luaBrowserEvaluateWithInit wraps host.browser.evaluate_with_init(url,
// initJS, js) → result string. initJS runs before any page script, so fetch
// hooks capture the site's own API calls.
func luaBrowserEvaluateWithInit(state *lua.State, m *Manager, id string) lua.Value {
	fn, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		url, _ := frame.CoerceString(0)
		initJS, _ := frame.CoerceString(1)
		js, _ := frame.CoerceString(2)
		out, err := m.proxy.BrowserEvaluateWithInit(id, url, initJS, js)
		if err != nil {
			return frame.ReturnValue(lua.Nil())
		}
		return frame.ReturnValue(lua.String(out))
	})
	return fn.Value()
}
