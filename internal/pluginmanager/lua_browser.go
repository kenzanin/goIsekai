package pluginmanager

import (
	"time"

	lua "github.com/mmcdole/lunar"

	"goisekai/internal/logger"
)

// luaBrowserFetch wraps host.browser.fetch(url) → rendered HTML, or nil when
// the browser is unavailable or the fetch failed.
func luaBrowserFetch(state *lua.State, m *Manager, id string) lua.Value {
	fn, _ := state.NewNativeFunction(func(frame lua.Frame) lua.Outcome {
		url, _ := frame.CoerceString(0)
		start := time.Now()
		logger.Debug("browser.fetch start", "plugin", id, "url", url)
		html, err := m.proxy.BrowserFetch(id, url)
		if err != nil {
			logger.Debug("browser.fetch fail", "plugin", id, "url", url, "elapsed", time.Since(start).String(), "error", err)
			return frame.ReturnValue(lua.Nil())
		}
		logger.Debug("browser.fetch done", "plugin", id, "url", url, "elapsed", time.Since(start).String(), "bytes", len(html))
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
		start := time.Now()
		logger.Debug("browser.evaluate start", "plugin", id, "url", url, "js_len", len(js))
		out, err := m.proxy.BrowserEvaluate(id, url, js)
		if err != nil {
			logger.Debug("browser.evaluate fail", "plugin", id, "url", url, "elapsed", time.Since(start).String(), "error", err)
			return frame.ReturnValue(lua.Nil())
		}
		logger.Debug("browser.evaluate done", "plugin", id, "url", url, "elapsed", time.Since(start).String(), "out_len", len(out))
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
		start := time.Now()
		logger.Debug("browser.evaluate_with_init start", "plugin", id, "url", url, "init_len", len(initJS), "js_len", len(js))
		out, err := m.proxy.BrowserEvaluateWithInit(id, url, initJS, js)
		if err != nil {
			logger.Debug("browser.evaluate_with_init fail", "plugin", id, "url", url, "elapsed", time.Since(start).String(), "error", err)
			return frame.ReturnValue(lua.Nil())
		}
		logger.Debug("browser.evaluate_with_init done", "plugin", id, "url", url, "elapsed", time.Since(start).String(), "out_len", len(out))
		return frame.ReturnValue(lua.String(out))
	})
	return fn.Value()
}
