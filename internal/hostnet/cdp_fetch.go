package hostnet

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod/lib/proto"

	"goisekai/internal/logger"
)

// BrowserFetch navigates to url in a rod browser, waits for JS to run and any
// challenge to clear, and returns the rendered HTML. This lets plugins read
// pages whose content is rendered client-side by the site's own JavaScript.
func (p *Proxy) BrowserFetch(pluginID, url string) (string, error) {
	if !p.cdp.enabled() {
		return "", fmt.Errorf("hostnet: browser fetch requires a CDP engine (cdp_engine is off)")
	}
	timeout := p.cdp.Timeout
	if timeout <= 0 {
		timeout = solveDefaultTimeout
	}
	start := time.Now()
	logger.Debug("cdp fetch start", "plugin", pluginID, "url", url, "timeout", timeout.String())
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_ = ctx // rod drives its own timeouts; ctx bounds the whole fetch.
	connStart := time.Now()
	browser, err := rodConnect(p.cdp)
	if err != nil {
		logger.Debug("cdp fetch connect fail", "plugin", pluginID, "elapsed", time.Since(start).String(), "error", err)
		return "", err
	}
	defer browser.MustClose()
	logger.Debug("cdp fetch connected", "plugin", pluginID, "elapsed", time.Since(connStart).String())

	page, err := browser.Page(proto.TargetCreateTarget{URL: url})
	if err != nil {
		logger.Debug("cdp fetch new page fail", "plugin", pluginID, "elapsed", time.Since(start).String(), "error", err)
		return "", fmt.Errorf("hostnet: rod new page %s: %w", url, err)
	}
	logger.Debug("cdp fetch page created", "plugin", pluginID, "elapsed", time.Since(start).String())
	_ = page.WaitStable(time.Second)
	logger.Debug("cdp fetch stable", "plugin", pluginID, "elapsed", time.Since(start).String())
	challStart := time.Now()
	if err := waitChallengeClearedRod(page, timeout); err != nil {
		logger.Debug("cdp fetch challenge fail", "plugin", pluginID, "elapsed", time.Since(start).String(), "chall_elapsed", time.Since(challStart).String(), "error", err)
		return "", err
	}
	logger.Debug("cdp fetch challenge cleared", "plugin", pluginID, "elapsed", time.Since(start).String(), "chall_elapsed", time.Since(challStart).String())

	// Give client-side rendering a moment to settle before reading.
	time.Sleep(1500 * time.Millisecond)

	html, err := page.HTML()
	if err != nil {
		logger.Debug("cdp fetch read html fail", "plugin", pluginID, "elapsed", time.Since(start).String(), "error", err)
		return "", fmt.Errorf("hostnet: read rendered html: %w", err)
	}
	logger.Debug("cdp fetch done", "plugin", pluginID, "elapsed", time.Since(start).String(), "bytes", len(html))
	return html, nil
}

// BrowserEvaluate navigates to url in a rod browser, executes js in the page
// context after JS has run and any challenge has cleared, and returns the
// result as a string. The script should return a JSON string; complex values
// are stringified by the page before crossing the CDP boundary.
func (p *Proxy) BrowserEvaluate(pluginID, url, js string) (string, error) {
	return p.browserEvaluateWithInit(pluginID, url, "", js)
}

// BrowserEvaluateWithInit is BrowserEvaluate with an init script that runs
// via EvalOnNewDocument before any page script. Use it to install fetch/XHR
// hooks so the plugin can capture API responses the site's own JS fetches.
func (p *Proxy) BrowserEvaluateWithInit(pluginID, url, initJS, js string) (string, error) {
	return p.browserEvaluateWithInit(pluginID, url, initJS, js)
}

func (p *Proxy) browserEvaluateWithInit(pluginID, url, initJS, js string) (string, error) {
	if !p.cdp.enabled() {
		return "", fmt.Errorf("hostnet: browser evaluate requires a CDP engine (cdp_engine is off)")
	}
	timeout := p.cdp.Timeout
	if timeout <= 0 {
		timeout = solveDefaultTimeout
	}
	start := time.Now()
	logger.Debug("cdp evaluate start", "plugin", pluginID, "url", url, "timeout", timeout.String(), "init_len", len(initJS), "js_len", len(js))
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_ = ctx
	connStart := time.Now()
	browser, err := rodConnect(p.cdp)
	if err != nil {
		logger.Debug("cdp evaluate connect fail", "plugin", pluginID, "elapsed", time.Since(start).String(), "error", err)
		return "", err
	}
	defer browser.MustClose()
	logger.Debug("cdp evaluate connected", "plugin", pluginID, "elapsed", time.Since(connStart).String())

	page, err := browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		logger.Debug("cdp evaluate new page fail", "plugin", pluginID, "elapsed", time.Since(start).String(), "error", err)
		return "", fmt.Errorf("hostnet: rod new page: %w", err)
	}
	logger.Debug("cdp evaluate page created", "plugin", pluginID, "elapsed", time.Since(start).String())
	// Install the init script before any page script runs, so fetch/XHR
	// hooks capture the site's own API calls from the first request.
	if initJS != "" {
		if _, err := page.EvalOnNewDocument(initJS); err != nil {
			logger.Debug("cdp evaluate init fail", "plugin", pluginID, "elapsed", time.Since(start).String(), "error", err)
			return "", fmt.Errorf("hostnet: init script: %w", err)
		}
	}
	navStart := time.Now()
	if err := page.Navigate(url); err != nil {
		logger.Debug("cdp evaluate navigate fail", "plugin", pluginID, "elapsed", time.Since(start).String(), "error", err)
		return "", fmt.Errorf("hostnet: browser navigate %s: %w", url, err)
	}
	logger.Debug("cdp evaluate navigated", "plugin", pluginID, "elapsed", time.Since(start).String(), "nav_elapsed", time.Since(navStart).String())
	_ = page.WaitStable(time.Second)
	logger.Debug("cdp evaluate stable", "plugin", pluginID, "elapsed", time.Since(start).String())
	challStart := time.Now()
	if err := waitChallengeClearedRod(page, timeout); err != nil {
		logger.Debug("cdp evaluate challenge fail", "plugin", pluginID, "elapsed", time.Since(start).String(), "chall_elapsed", time.Since(challStart).String(), "error", err)
		return "", err
	}
	logger.Debug("cdp evaluate challenge cleared", "plugin", pluginID, "elapsed", time.Since(start).String(), "chall_elapsed", time.Since(challStart).String())

	// Rod Eval expects a function; wrap bare expressions.
	wrapped := js
	trimmed := strings.TrimSpace(js)
	if !strings.HasPrefix(trimmed, "(") && !strings.HasPrefix(trimmed, "function") && !strings.HasPrefix(trimmed, "async") {
		wrapped = "() => " + js
	}
	v, err := page.Eval(wrapped)
	if err != nil {
		logger.Debug("cdp evaluate eval fail", "plugin", pluginID, "elapsed", time.Since(start).String(), "error", err)
		return "", fmt.Errorf("hostnet: evaluate: %w", err)
	}
	out := v.Value.String()
	logger.Debug("cdp evaluate done", "plugin", pluginID, "elapsed", time.Since(start).String(), "out_len", len(out))
	return out, nil
}
