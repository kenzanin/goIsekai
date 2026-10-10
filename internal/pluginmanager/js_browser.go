package pluginmanager

import (
	"github.com/dop251/goja"
)

// jsBrowserFetch wraps host.browser.fetch(url) → rendered HTML, or null when
// the browser is unavailable or the fetch failed.
func jsBrowserFetch(vm *goja.Runtime, m *Manager, id string) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		url := call.Arguments[0].String()
		html, err := m.proxy.BrowserFetch(id, url)
		if err != nil {
			return goja.Null()
		}
		return vm.ToValue(html)
	}
}

// jsBrowserEvaluate wraps host.browser.evaluate(url, js) → result string, or
// null when the browser is unavailable or the evaluation failed.
func jsBrowserEvaluate(vm *goja.Runtime, m *Manager, id string) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		url := call.Arguments[0].String()
		js := ""
		if len(call.Arguments) > 1 {
			js = call.Arguments[1].String()
		}
		out, err := m.proxy.BrowserEvaluate(id, url, js)
		if err != nil {
			return goja.Null()
		}
		return vm.ToValue(out)
	}
}

// jsBrowserEvaluateWithInit wraps host.browser.evaluate_with_init(url, initJS,
// js) → result string. initJS runs before any page script.
func jsBrowserEvaluateWithInit(vm *goja.Runtime, m *Manager, id string) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		url := call.Arguments[0].String()
		initJS := ""
		js := ""
		if len(call.Arguments) > 1 {
			initJS = call.Arguments[1].String()
		}
		if len(call.Arguments) > 2 {
			js = call.Arguments[2].String()
		}
		out, err := m.proxy.BrowserEvaluateWithInit(id, url, initJS, js)
		if err != nil {
			return goja.Null()
		}
		return vm.ToValue(out)
	}
}
