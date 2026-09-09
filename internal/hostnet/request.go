package hostnet

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	http "github.com/bogdanfinn/fhttp"

	"goisekai/pkg/types"
)

// Request builds, executes, and returns the response for a plugin HTTP request.
// Per-page headers overlay the default headers, and cookies are persisted per
// plugin so they survive across calls.
//
// Two distinct failure modes are handled differently:
//   - Anti-bot CHALLENGES (CF JS challenge / Turnstile): solved via the browser
//     engine (CDP) when configured, surfacing ChallengeError when not.
//   - WAF BLOCKS (utls fingerprint detection): retried through the TLS-profile
//     ladder, pinning the first profile that clears the block. The CDP engine is
//     the last resort when the ladder is exhausted.
func (p *Proxy) Request(pluginID string, req types.HTTPRequest) (types.HTTPResponse, error) {
	// needs_js: preemptively solve + seed cookies via the browser engine so the
	// client-side site is already cleared when the fast path runs.
	if p.needsJSHint(pluginID) && p.CDPConfig().enabled() {
		_ = p.solveAndSeed(pluginID, req.URL)
	}

	// Stdlib h2 path: pinned plugins skip tls-client entirely (WAF blocks the
	// utls fingerprint but allows Go's stock TLS + h2).
	if p.stdlibPrefers(pluginID) {
		return p.doRequestStd(pluginID, req)
	}

	resp, err := p.doRequest(pluginID, req)
	if err != nil {
		return types.HTTPResponse{}, err
	}

	// ── Challenge response (CF JS / Turnstile) ──
	// These require JavaScript execution, not a different TLS fingerprint.
	if isChallengeResponse(resp) {
		if !p.CDPConfig().enabled() {
			return types.HTTPResponse{}, &ChallengeError{VerifyURL: req.URL}
		}
		if err := p.solveAndSeed(pluginID, req.URL); err != nil {
			return types.HTTPResponse{}, &ChallengeError{VerifyURL: req.URL}
		}
		retried, rerr := p.doRequest(pluginID, req)
		// A solve that seeds cookies but does not clear the challenge (chained
		// challenge, stale clearance) must NOT surface as success: the plugin
		// relies on ChallengeError to show "source requires verification".
		if rerr != nil || isChallengeResponse(retried) {
			return types.HTTPResponse{}, &ChallengeError{VerifyURL: req.URL}
		}
		return retried, nil
	}

	// ── WAF block (utls fingerprint detection) ──
	// The server rejects the browser fingerprint itself; try a different TLS
	// profile via the ladder (plugin hints then default rotation). An HTML body
	// is a JS interstitial wearing the WAF marker, so it needs the browser
	// engine — skip the ladder and treat it as a challenge.
	if isWafBlock(resp) {
		htmlBlock := strings.Contains(resp.Headers["Content-Type"], "text/html")
		if !htmlBlock {
			if retried, ok := p.tryLadder(pluginID, req); ok {
				return retried, nil
			}
		}
		// Ladder exhausted (or HTML block). Last resort: solve via the engine,
		// seed cookies, and retry once.
		if p.CDPConfig().enabled() && p.solveAndSeed(pluginID, req.URL) == nil {
			if retried, rerr := p.doRequest(pluginID, req); rerr == nil && !isWafBlock(retried) && !isChallengeResponse(retried) {
				return retried, nil
			}
		}
		return types.HTTPResponse{}, &ChallengeError{VerifyURL: req.URL}
	}

	return resp, nil
}

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
func (p *Proxy) solveAndSeed(pluginID, targetURL string) error {
	solver := p.solveChallenge
	if solver == nil {
		return errors.New("hostnet: no challenge solver installed")
	}
	cfg := p.CDPConfig()
	cookies, ua, err := solver(cfg, targetURL)
	if err != nil {
		return err
	}
	return p.SetVerifyCookies(pluginID, hostOf(targetURL), cookieHeader(cookies), ua)
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

// HandleRequest decodes a request JSON string, executes it, and returns the
// response marshaled to a JSON string. Malformed JSON or network failures are
// returned as errors (never panic).
func (p *Proxy) HandleRequest(pluginID string, requestJSON string) (string, error) {
	var req types.HTTPRequest
	if err := json.Unmarshal([]byte(requestJSON), &req); err != nil {
		return "", fmt.Errorf("hostnet: malformed request JSON: %w", err)
	}

	resp, err := p.Request(pluginID, req)
	if err != nil {
		return "", fmt.Errorf("hostnet: request failed: %w", err)
	}

	out, err := json.Marshal(resp)
	if err != nil {
		return "", fmt.Errorf("hostnet: marshal response: %w", err)
	}
	return string(out), nil
}
