package hostnet

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"

	"goisekai/pkg/types"
)

// isChallengeResponse reports whether a response looks like an anti-bot
// challenge: 403/503 with an HTML body carrying a known marker.
func isChallengeResponse(resp types.HTTPResponse) bool {
	if resp.Status != 403 && resp.Status != 503 {
		return false
	}
	if !strings.Contains(resp.Headers["Content-Type"], "text/html") {
		return false
	}
	return IsChallengeResponse(resp.Status, []byte(resp.Body))
}

// solveAndSeed runs the browser engine against targetURL and seeds the
// harvested cookies + browser UA into the plugin's verify-cookie store.
//
// The wait is bounded by ctx: the engine ladder (obscura → lightpanda → …)
// can run for minutes and is not ctx-aware itself, so without this wrapper a
// dead-cookie session would block the whole plugin invoke past its deadline.
// On ctx expiry the caller unblocks immediately (the orphaned solve keeps
// running; its result is dropped) and the challenge surfaces as a
// ChallengeError so the wizard can re-prompt for fresh cookies.
func (p *Proxy) solveAndSeed(ctx context.Context, pluginID, targetURL string) error {
	// manual_cookies: the plugin declares its cookies are pasted by the user
	// (wizard), never solved — fail fast so the challenge re-opens the wizard
	// instead of burning the invoke budget on an engine that never clears it.
	if p.manualCookiesHint(pluginID) {
		return fmt.Errorf("hostnet: manual_cookies plugin %s: challenge left to the verify wizard", pluginID)
	}
	solver := p.solveChallenge
	if solver == nil {
		return errors.New("hostnet: no challenge solver installed")
	}
	cfg := p.CDPConfig()
	// Clamp the solve budget to what the caller can actually wait for. The
	// configured cdp_solve_timeout (often 120s) routinely exceeds the plugin
	// invoke budget (15-30s), so a solve would burn its full engine timeout on
	// every attempt and still be abandoned — and with a 2-attempt engine
	// budget that is 4 minutes of work for a caller that left after 15
	// seconds. Bounding each attempt by the remaining ctx keeps the attempt
	// inside the caller's budget instead of past it.
	if dl, ok := ctx.Deadline(); ok {
		if remaining := time.Until(dl); remaining > 0 && (cfg.Timeout <= 0 || remaining < cfg.Timeout) {
			cfg.Timeout = remaining
		}
	}
	type solveResult struct {
		cookies []*http.Cookie
		ua      string
		err     error
	}
	ch := make(chan solveResult, 1)
	p.SubmitSolve(func() {
		cookies, ua, err := solver(cfg, targetURL)
		ch <- solveResult{cookies: cookies, ua: ua, err: err}
	})
	var r solveResult
	select {
	case r = <-ch:
	case <-ctx.Done():
		return ctx.Err()
	}
	if r.err != nil {
		return r.err
	}
	return p.SetVerifyCookies(pluginID, hostOf(targetURL), cookieHeader(r.cookies), r.ua)
}

// cookieHeader serializes cookies into a "a=1; b=2" header string for the
// verify-cookie parser.
func cookieHeader(cookies []*http.Cookie) string {
	parts := make([]string, 0, len(cookies))
	for _, c := range cookies {
		if c == nil || c.Name == "" {
			continue
		}
		parts = append(parts, c.Name+"="+c.Value)
	}
	return strings.Join(parts, "; ")
}
