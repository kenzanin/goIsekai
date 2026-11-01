package pluginmanager

import (
	"time"

	"goisekai/internal/logger"
	"modernc.org/quickjs"
)

// jsBrowserFetch wraps host.browser.fetch(url) → rendered HTML, or null when
// the browser is unavailable or the fetch failed.
func jsBrowserFetch(vm *quickjs.VM, m *Manager, id string) quickjs.HostFunc {
	return func(args []any) (any, error) {
		url, _ := args[0].(string)
		start := time.Now()
		logger.Debug("browser.fetch start", "plugin", id, "url", url)
		html, err := m.proxy.BrowserFetch(id, url)
		if err != nil {
			logger.Debug("browser.fetch fail", "plugin", id, "url", url, "elapsed", time.Since(start).String(), "error", err)
			return nil, nil
		}
		logger.Debug("browser.fetch done", "plugin", id, "url", url, "elapsed", time.Since(start).String(), "bytes", len(html))
		return html, nil
	}
}

// jsBrowserEvaluate wraps host.browser.evaluate(url, js) → result string, or
// null when the browser is unavailable or the evaluation failed.
func jsBrowserEvaluate(vm *quickjs.VM, m *Manager, id string) quickjs.HostFunc {
	return func(args []any) (any, error) {
		url, _ := args[0].(string)
		js := ""
		if len(args) > 1 {
			js, _ = args[1].(string)
		}
		start := time.Now()
		logger.Debug("browser.evaluate start", "plugin", id, "url", url, "js_len", len(js))
		out, err := m.proxy.BrowserEvaluate(id, url, js)
		if err != nil {
			logger.Debug("browser.evaluate fail", "plugin", id, "url", url, "elapsed", time.Since(start).String(), "error", err)
			return nil, nil
		}
		logger.Debug("browser.evaluate done", "plugin", id, "url", url, "elapsed", time.Since(start).String(), "out_len", len(out))
		return out, nil
	}
}

// jsBrowserEvaluateWithInit wraps host.browser.evaluate_with_init(url, initJS,
// js) → result string. initJS runs before any page script.
func jsBrowserEvaluateWithInit(vm *quickjs.VM, m *Manager, id string) quickjs.HostFunc {
	return func(args []any) (any, error) {
		url, _ := args[0].(string)
		initJS := ""
		js := ""
		if len(args) > 1 {
			initJS, _ = args[1].(string)
		}
		if len(args) > 2 {
			js, _ = args[2].(string)
		}
		start := time.Now()
		logger.Debug("browser.evaluate_with_init start", "plugin", id, "url", url, "init_len", len(initJS), "js_len", len(js))
		out, err := m.proxy.BrowserEvaluateWithInit(id, url, initJS, js)
		if err != nil {
			logger.Debug("browser.evaluate_with_init fail", "plugin", id, "url", url, "init_len", len(initJS), "js_len", len(js), "elapsed", time.Since(start).String(), "error", err)
			return nil, nil
		}
		logger.Debug("browser.evaluate_with_init done", "plugin", id, "url", url, "elapsed", time.Since(start).String(), "out_len", len(out))
		return out, nil
	}
}
