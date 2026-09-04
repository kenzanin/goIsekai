package hostnet

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/bogdanfinn/fhttp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	"goisekai/internal/logger"
)

// solveDefaultTimeout bounds a single challenge solve when the caller did not
// configure a timeout.
const solveDefaultTimeout = 30 * time.Second

// engineCoolDown is the circuit-breaker window: an engine that failed within
// the last engineCoolDown is skipped by the fallback chain so a dead daemon
// isn't re-tried on every challenge.
const engineCoolDown = 30 * time.Second

// engineHealth tracks the last failure time per CDP engine. A successful solve
// clears the entry (reset on success).
var engineHealth = struct {
	sync.Mutex
	lastFail map[string]time.Time
}{lastFail: make(map[string]time.Time)}

func engineFailed(engine string) {
	engineHealth.Lock()
	defer engineHealth.Unlock()
	engineHealth.lastFail[engine] = time.Now()
}

func engineSucceeded(engine string) {
	engineHealth.Lock()
	defer engineHealth.Unlock()
	delete(engineHealth.lastFail, engine)
}

// engineTripped reports whether engine failed within the last engineCoolDown.
func engineTripped(engine string) bool {
	engineHealth.Lock()
	defer engineHealth.Unlock()
	t, ok := engineHealth.lastFail[engine]
	return ok && time.Since(t) < engineCoolDown
}

// cdpFallbackChain returns the ordered engine names to attempt for a solve:
// the configured engine first, then the "obscura" and "lightpanda" daemon
// engines as fallbacks. Duplicates are removed (the configured engine is not
// repeated when it is already one of the fallbacks) and disabled engines
// ("", "off") are dropped.
func cdpFallbackChain(configured string) []string {
	candidates := []string{configured, "obscura", "lightpanda"}
	chain := make([]string, 0, len(candidates))
	seen := make(map[string]bool, len(candidates))
	for _, e := range candidates {
		if e == "" || e == "off" || seen[e] {
			continue
		}
		seen[e] = true
		chain = append(chain, e)
	}
	return chain
}

// solveChallenge launches a CDP browser engine, navigates it to url, waits for
// the anti-bot interstitial to clear, and harvests the resulting cookies plus
// the browser's User-Agent. It is the concrete solver installed by default;
// tests swap it out via Proxy.solveChallenge.
//
// The engine and binary/endpoint come from cfg:
//   - "chrome"     → cfg.Path is the chrome binary path, launched as a subprocess.
//   - "lightpanda" → cfg.Path is a CDP websocket URL (ws://host:port).
//   - "obscura"    → cfg.Path is a CDP websocket URL (ws://host:port).
//
// Engines are attempted in fallback-chain order (see cdpFallbackChain): the
// configured engine first, then the "obscura" and "lightpanda" alternatives.
// At most one fallback engine is tried after the primary fails, and engines
// that failed within engineCoolDown are skipped (circuit breaker). A fallback
// attempt is logged at WARN.
func solveChallenge(cfg CDPConfig, url string) ([]*http.Cookie, string, error) {
	var lastErr error
	tried := 0
	for _, engine := range cdpFallbackChain(cfg.Engine) {
		if tried >= 2 {
			break // primary + max 1 retry
		}
		if engineTripped(engine) {
			logger.Warn("cdp: skipping engine (recent failure)", "engine", engine, "url", url)
			continue
		}
		tried++
		attempt := cfg
		attempt.Engine = engine
		cookies, ua, err := solveWithEngine(attempt, url)
		if err == nil {
			engineSucceeded(engine)
			return cookies, ua, nil
		}
		engineFailed(engine)
		lastErr = err
		logger.Warn("cdp: engine failed, trying next", "engine", engine, "error", err, "url", url)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("hostnet: no CDP engine available (all engines skipped)")
	}
	return nil, "", lastErr
}

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
