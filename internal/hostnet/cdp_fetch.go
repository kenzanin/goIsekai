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
	// The plugin's declared invoke budget overrides the global cdp_solve_timeout
	// so a plugin waiting on its own JS loop is not cut off mid-wait.
	timeout := p.pluginCDPDeadline(pluginID)
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

	// Create the target at about:blank and navigate explicitly. SPAs like
	// comix.to fire a client-side router chain (>10 navigations) on load, which
	// trips Chromium's CDP hard limit "{-32000 Too many client-initiated
	// navigations, the chain reached its limit of 10 documents}" when the URL is
	// passed to TargetCreateTarget (it waits for the chain to settle). Opening
	// at about:blank and navigating ourselves lets us tolerate that error: the
	// page still renders, so we give JS a bounded window and read the DOM
	// regardless of the mid-flight nav-chain abort.
	page, err := browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		logger.Debug("cdp fetch new page fail", "plugin", pluginID, "elapsed", time.Since(start).String(), "error", err)
		return "", fmt.Errorf("hostnet: rod new page %s: %w", url, err)
	}
	logger.Debug("cdp fetch page created", "plugin", pluginID, "elapsed", time.Since(start).String())
	page = page.Context(ctx)
	if navErr := page.Navigate(url); navErr != nil {
		// Tolerate the 10-document nav-chain limit: the document is still
		// loading and renders usable DOM; proceed to wait+read below.
		logger.Debug("cdp fetch navigate tolerated", "plugin", pluginID, "elapsed", time.Since(start).String(), "navigate_err", navErr)
	} else {
		logger.Debug("cdp fetch navigated", "plugin", pluginID, "elapsed", time.Since(start).String())
	}
	// SPA pages (e.g. mangataro) never fire loadEventFired and never idle, so
	// WaitStable/WaitLoad block until the full ctx budget. page.Navigate
	// returns on commit; give lazy JS a bounded window (ctx-bounded) to render
	// the chapter list before eval.
	time.Sleep(2 * time.Second)
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
	timeout := p.pluginCDPDeadline(pluginID)
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
	if navErr := page.Navigate(url); navErr != nil {
		// Tolerate Chromium's nav-chain limit: SPAs like comix.to fire >10
		// client-side navigations on load; the document still renders, so
		// proceed to the challenge wait + eval below. Mirrors BrowserFetch.
		logger.Debug("cdp evaluate navigate tolerated", "plugin", pluginID, "elapsed", time.Since(start).String(), "navigate_err", navErr)
	}
	logger.Debug("cdp evaluate navigated", "plugin", pluginID, "elapsed", time.Since(start).String(), "nav_elapsed", time.Since(navStart).String())
	page = page.Context(ctx)
	// SPA pages (e.g. mangataro) never fire loadEventFired and never idle, so
	// WaitStable/WaitLoad block until the full ctx budget. page.Navigate
	// returns on commit; give lazy JS a bounded window (ctx-bounded) to render
	// the chapter list before eval.
	time.Sleep(2 * time.Second)
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
