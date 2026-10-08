package hostnet

import (
	"context"
	"fmt"
	"time"

	http "github.com/bogdanfinn/fhttp"
	"github.com/go-rod/rod/lib/proto"
)

// solveWithEngine runs a single challenge solve against one engine. cfg.Engine
// names the engine and cfg.Path locates it (a binary path for "chrome", a
// ws:// URL for "lightpanda" and "obscura").
func solveWithEngine(cfg CDPConfig, url string) ([]*http.Cookie, string, error) {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = solveDefaultTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_ = ctx // rod drives its own timeouts; ctx bounds the whole solve.
	browser, err := rodConnect(cfg)
	if err != nil {
		return nil, "", err
	}
	defer browser.MustClose()

	page, err := browser.Page(proto.TargetCreateTarget{URL: url})
	if err != nil {
		return nil, "", fmt.Errorf("hostnet: rod new page %s: %w", url, err)
	}
	// Let the document settle before evaluating; the challenge poll below
	// re-tries, so a transient not-ready here is not fatal.
	_ = page.WaitStable(time.Second)
	if err := waitChallengeClearedRod(page, timeout); err != nil {
		return nil, "", err
	}

	// Harvest cookies scoped to the target host, plus the browser's UA.
	// The cookie jar may commit asynchronously after a challenge reload, so
	// poll briefly until it is non-empty (or the budget is exhausted).
	host := hostOf(url)
	var cookies []*http.Cookie
	for attempt := range 5 {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, "", fmt.Errorf("hostnet: challenge solve aborted: %w", ctx.Err())
			case <-time.After(300 * time.Millisecond):
			}
		}
		raw, err := browser.GetCookies()
		if err != nil {
			return nil, "", fmt.Errorf("hostnet: read cookies: %w", err)
		}
		cookies = nil
		for _, c := range raw {
			if !cookieMatchesHost(c.Domain, host) {
				continue // not for the target host
			}
			cookies = append(cookies, &http.Cookie{Name: c.Name, Value: c.Value})
		}
		if len(cookies) > 0 {
			break
		}
	}
	var ua string
	if v, err := page.Eval(`navigator.userAgent`); err == nil {
		ua = v.Value.String()
	}
	// A missing UA degrades to the fast-path default; it is best-effort.

	return cookies, ua, nil
}
