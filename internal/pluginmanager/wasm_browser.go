package pluginmanager

import (
	"time"

	"goisekai/internal/logger"
)

// The CDP-backed browser helpers, exposed to wasm through the same
// env.host_call dispatcher as everything else. They mirror the Lua and JS
// natives exactly, including the failure shape: an unavailable engine or a
// failed navigation yields JSON null, not an error envelope, because a plugin
// treats "the browser could not get this" as "no data" and falls back. The
// proxy calls are deliberately the non-ctx variants, matching Lua/JS — the CDP
// layer applies its own page budget.

// wasmBrowserFetch is host.browser.fetch(url) → rendered HTML, or null.
func (m *Manager) wasmBrowserFetch(id, url string) (any, error) {
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

// wasmBrowserEvaluate is host.browser.evaluate(url, js) → result, or null.
func (m *Manager) wasmBrowserEvaluate(id, url, js string) (any, error) {
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

// wasmBrowserEvaluateWithInit is host.browser.evaluate_with_init(url, initJS,
// js) → result, or null. initJS runs before any page script so fetch hooks can
// capture the site's own API calls.
func (m *Manager) wasmBrowserEvaluateWithInit(id, url, initJS, js string) (any, error) {
	start := time.Now()
	logger.Debug("browser.evaluate_with_init start", "plugin", id, "url", url, "init_len", len(initJS), "js_len", len(js))
	out, err := m.proxy.BrowserEvaluateWithInit(id, url, initJS, js)
	if err != nil {
		logger.Debug("browser.evaluate_with_init fail", "plugin", id, "url", url, "elapsed", time.Since(start).String(), "error", err)
		return nil, nil
	}
	logger.Debug("browser.evaluate_with_init done", "plugin", id, "url", url, "elapsed", time.Since(start).String(), "out_len", len(out))
	return out, nil
}
