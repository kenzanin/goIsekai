package hostnet

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	http "github.com/bogdanfinn/fhttp"
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
