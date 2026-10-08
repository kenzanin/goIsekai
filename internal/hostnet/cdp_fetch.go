package hostnet

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod/lib/proto"
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
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_ = ctx // rod drives its own timeouts; ctx bounds the whole fetch.
	browser, err := rodConnect(p.cdp)
	if err != nil {
		return "", err
	}
	defer browser.MustClose()

	page, err := browser.Page(proto.TargetCreateTarget{URL: url})
	if err != nil {
		return "", fmt.Errorf("hostnet: rod new page %s: %w", url, err)
	}
	_ = page.WaitStable(time.Second)
	if err := waitChallengeClearedRod(page, timeout); err != nil {
		return "", err
	}

	// Give client-side rendering a moment to settle before reading.
	time.Sleep(1500 * time.Millisecond)

	html, err := page.HTML()
	if err != nil {
		return "", fmt.Errorf("hostnet: read rendered html: %w", err)
	}
	return html, nil
}

// BrowserEvaluate navigates to url in a rod browser, executes js in the page
// context after JS has run and any challenge has cleared, and returns the
// result as a string. The script should return a JSON string; complex values
// are stringified by the page before crossing the CDP boundary.
func (p *Proxy) BrowserEvaluate(pluginID, url, js string) (string, error) {
	if !p.cdp.enabled() {
		return "", fmt.Errorf("hostnet: browser evaluate requires a CDP engine (cdp_engine is off)")
	}
	timeout := p.cdp.Timeout
	if timeout <= 0 {
		timeout = solveDefaultTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_ = ctx
	browser, err := rodConnect(p.cdp)
	if err != nil {
		return "", err
	}
	defer browser.MustClose()

	page, err := browser.Page(proto.TargetCreateTarget{URL: url})
	if err != nil {
		return "", fmt.Errorf("hostnet: rod new page %s: %w", url, err)
	}
	_ = page.WaitStable(time.Second)
	if err := waitChallengeClearedRod(page, timeout); err != nil {
		return "", err
	}

	// Rod Eval expects a function; wrap bare expressions.
	wrapped := js
	trimmed := strings.TrimSpace(js)
	if !strings.HasPrefix(trimmed, "(") && !strings.HasPrefix(trimmed, "function") && !strings.HasPrefix(trimmed, "async") {
		wrapped = "() => " + js
	}
	v, err := page.Eval(wrapped)
	if err != nil {
		return "", fmt.Errorf("hostnet: evaluate: %w", err)
	}
	return v.Value.String(), nil
}
