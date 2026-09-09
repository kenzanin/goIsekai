package hostnet

import (
	"context"
	"fmt"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
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

	var browserCtx context.Context
	var browserCancel context.CancelFunc

	if cfg.Engine == "lightpanda" || cfg.Engine == "obscura" {
		// lightpanda/obscura run as an external daemon; connect to their CDP
		// endpoint.
		if !strings.HasPrefix(cfg.Path, "ws://") && !strings.HasPrefix(cfg.Path, "wss://") {
			return nil, "", fmt.Errorf("hostnet: %s cdp_path must be a ws:// URL, got %q", cfg.Engine, cfg.Path)
		}
		browserCtx, browserCancel = chromedp.NewRemoteAllocator(ctx, cfg.Path)
	} else {
		opts := append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.ExecPath(cfg.Path),
			chromedp.Headless,
			chromedp.NoFirstRun,
			chromedp.NoDefaultBrowserCheck,
			chromedp.DisableGPU,
		)
		var allocCtx context.Context
		allocCtx, browserCancel = chromedp.NewExecAllocator(ctx, opts...)
		browserCtx, _ = chromedp.NewContext(allocCtx)
	}
	defer browserCancel()

	tabCtx, tabCancel := chromedp.NewContext(browserCtx)
	defer tabCancel()

	// Navigate and let the interstitial's JS run; poll for the challenge marker
	// to disappear (or the timeout to fire).
	if err := chromedp.Run(tabCtx, chromedp.Navigate(url)); err != nil {
		return nil, "", fmt.Errorf("hostnet: navigate %s: %w", url, err)
	}
	if err := waitChallengeCleared(tabCtx, timeout); err != nil {
		return nil, "", err
	}

	// Harvest cookies scoped to the target host, plus the browser's UA.
	// GetCookies must run inside chromedp.Run (ActionFunc) — calling
	// network.GetCookies().Do(tabCtx) directly returns "invalid context"
	// against both lightpanda and chrome. lightpanda commits the cookie jar
	// asynchronously after a challenge reload, so poll briefly until the jar
	// is non-empty (or the budget is exhausted) before giving up.
	host := hostOf(url)
	var cookies []*network.Cookie
	for attempt := range 5 {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, "", fmt.Errorf("hostnet: challenge solve aborted: %w", ctx.Err())
			case <-time.After(300 * time.Millisecond):
			}
		}
		cookies = nil
		err := chromedp.Run(tabCtx, chromedp.ActionFunc(func(ctx context.Context) error {
			cs, err := network.GetCookies().Do(ctx)
			cookies = cs
			return err
		}))
		if err != nil {
			return nil, "", fmt.Errorf("hostnet: read cookies: %w", err)
		}
		if len(cookies) > 0 {
			break
		}
	}
	var ua string
	if err := chromedp.Run(tabCtx, chromedp.Evaluate(`navigator.userAgent`, &ua)); err != nil {
		// UA is best-effort; a missing UA degrades to the fast-path default.
		ua = ""
	}

	out := make([]*http.Cookie, 0, len(cookies))
	for _, c := range cookies {
		if !cookieMatchesHost(c.Domain, host) {
			continue // not for the target host
		}
		out = append(out, &http.Cookie{Name: c.Name, Value: c.Value})
	}
	return out, ua, nil
}
