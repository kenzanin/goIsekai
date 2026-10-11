package pluginmanager

import (
	"time"

	"github.com/dop251/goja"

	"goisekai/internal/logger"
)

// jsBrowserFetch wraps host.browser.fetch(url) → rendered HTML, or null when
// the browser is unavailable or the fetch failed.
func jsBrowserFetch(vm *goja.Runtime, m *Manager, id string) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		url := call.Arguments[0].String()
		start := time.Now()
		logger.Debug("browser.fetch start", "plugin", id, "url", url)
		html, err := m.proxy.BrowserFetch(id, url)
		if err != nil {
			logger.Debug("browser.fetch fail", "plugin", id, "url", url, "elapsed", time.Since(start).String(), "error", err)
			return goja.Null()
		}
		logger.Debug("browser.fetch done", "plugin", id, "url", url, "elapsed", time.Since(start).String(), "bytes", len(html))
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
		start := time.Now()
		logger.Debug("browser.evaluate start", "plugin", id, "url", url, "js_len", len(js))
		out, err := m.proxy.BrowserEvaluate(id, url, js)
		if err != nil {
			logger.Debug("browser.evaluate fail", "plugin", id, "url", url, "elapsed", time.Since(start).String(), "error", err)
			return goja.Null()
		}
		logger.Debug("browser.evaluate done", "plugin", id, "url", url, "elapsed", time.Since(start).String(), "out_len", len(out))
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
		start := time.Now()
		logger.Debug("browser.evaluate_with_init start", "plugin", id, "url", url, "init_len", len(initJS), "js_len", len(js))
		out, err := m.proxy.BrowserEvaluateWithInit(id, url, initJS, js)
		if err != nil {
			logger.Debug("browser.evaluate_with_init fail", "plugin", id, "url", url, "init_len", len(initJS), "js_len", len(js), "elapsed", time.Since(start).String(), "error", err)
			return goja.Null()
		}
		logger.Debug("browser.evaluate_with_init done", "plugin", id, "url", url, "elapsed", time.Since(start).String(), "out_len", len(out))
		return vm.ToValue(out)
	}
}
