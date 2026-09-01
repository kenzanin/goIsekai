package hostnet

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	http "github.com/bogdanfinn/fhttp"
)

// solveDefaultTimeout bounds a single challenge solve when the caller did not
// configure a timeout.
const solveDefaultTimeout = 30 * time.Second

// solveChallenge launches a CDP browser engine, navigates it to url, waits for
// the anti-bot interstitial to clear, and harvests the resulting cookies plus
// the browser's User-Agent. It is the concrete solver installed by default;
// tests swap it out via Proxy.solveChallenge.
//
// The engine and binary/endpoint come from cfg:
//   - "chrome"     → cfg.Path is the chrome binary path, launched as a subprocess.
//   - "lightpanda" → cfg.Path is a CDP websocket URL (ws://host:port).
func solveChallenge(cfg CDPConfig, url string) ([]*http.Cookie, string, error) {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = solveDefaultTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var browserCtx context.Context
	var browserCancel context.CancelFunc

	if cfg.Engine == "lightpanda" {
		// lightpanda runs as an external daemon; connect to its CDP endpoint.
		if !strings.HasPrefix(cfg.Path, "ws://") && !strings.HasPrefix(cfg.Path, "wss://") {
			return nil, "", fmt.Errorf("hostnet: lightpanda cdp_path must be a ws:// URL, got %q", cfg.Path)
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
	for attempt := 0; attempt < 5; attempt++ {
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

// cookieMatchesHost reports whether a cookie domain (which never carries a
// port and may carry a leading dot) is scoped to the target host (which may
// carry a port).
func cookieMatchesHost(domain, host string) bool {
	if host == "" || domain == "" {
		return true
	}
	// Strip any port from the host: cookie domains are host-only.
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	domain = strings.TrimPrefix(domain, ".")
	return host == domain || strings.HasSuffix(host, "."+domain)
}

// waitChallengeCleared polls the tab until the challenge interstitial clears or
// ctx times out. It returns nil once the marker is gone so the caller can
// harvest cookies.
func waitChallengeCleared(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		var body string
		// Fetching document.body.innerText is cheaper than a full screenshot
		// and is enough to detect the Cloudflare "Just a moment" interstitial.
		if err := chromedp.Run(ctx, chromedp.Evaluate(`document.body ? document.body.innerText : ""`, &body)); err == nil {
			lower := strings.ToLower(body)
			if !strings.Contains(lower, "just a moment") && !strings.Contains(lower, "challenge-platform") {
				return nil
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("hostnet: challenge solve timed out after %s", timeout)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("hostnet: challenge solve aborted: %w", ctx.Err())
		case <-time.After(750 * time.Millisecond):
		}
	}
}

// hostOf returns the host (and port, if any) of a URL, or "" on parse failure.
func hostOf(rawurl string) string {
	// Reuse the same tolerant parsing as normalizeDomain, but keep the port so
	// cookie scoping matches the jar's host keying.
	if i := strings.Index(rawurl, "://"); i >= 0 {
		rawurl = rawurl[i+3:]
	}
	if i := strings.IndexAny(rawurl, "/?#"); i >= 0 {
		rawurl = rawurl[:i]
	}
	return rawurl
}
