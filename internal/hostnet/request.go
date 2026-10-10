package hostnet

import (
	"context"
	"fmt"
	"github.com/goccy/go-json"
	"strings"

	"goisekai/internal/logger"
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
	return p.RequestContext(context.Background(), pluginID, req)
}

// RequestContext is Request with a caller context. Cancelling ctx tears down the
// in-flight upstream call — including the TLS-profile ladder retries — instead
// of letting it run to the client timeout. Plugin ABI calls have no request
// context and use Request; the reader's per-page image fetches use this so a
// chapter switch releases the work.
func (p *Proxy) RequestContext(ctx context.Context, pluginID string, req types.HTTPRequest) (types.HTTPResponse, error) {
	// needs_js: preemptively solve + seed cookies via the browser engine so the
	// client-side site is already cleared when the fast path runs. Skipped when
	// the plugin already carries cookies for the target host (pasted verify
	// cookies or an earlier solve): a solve here is a multi-minute CDP cascade
	// that would burn every plugin-invoke budget even though the jar is clear.
	if p.needsJSHint(pluginID) && p.CDPConfig().enabled() && !p.hasVerifyCookie(pluginID, hostOf(req.URL)) {
		_ = p.solveAndSeed(ctx, pluginID, req.URL)
	}

	// Stdlib h2 path: pinned plugins skip tls-client entirely (WAF blocks the
	// utls fingerprint but allows Go's stock TLS + h2).
	if p.stdlibPrefers(pluginID) {
		return p.doRequestStd(ctx, pluginID, req)
	}

	resp, err := p.doRequest(ctx, pluginID, req)
	if err != nil {
		return types.HTTPResponse{}, err
	}

	// ── Challenge response (CF JS / Turnstile) ──
	// These require JavaScript execution, not a different TLS fingerprint.
	if isChallengeResponse(resp) {
		if !p.CDPConfig().enabled() {
			return types.HTTPResponse{}, &ChallengeError{VerifyURL: req.URL}
		}
		if err := p.solveAndSeed(ctx, pluginID, req.URL); err != nil {
			return types.HTTPResponse{}, &ChallengeError{VerifyURL: req.URL}
		}
		retried, rerr := p.doRequest(ctx, pluginID, req)
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
			if retried, ok := p.tryLadder(ctx, pluginID, req); ok {
				return retried, nil
			}
		}
		// Ladder exhausted (or HTML block). Last resort: solve via the engine,
		// seed cookies, and retry once.
		if p.CDPConfig().enabled() && p.solveAndSeed(ctx, pluginID, req.URL) == nil {
			if retried, rerr := p.doRequest(ctx, pluginID, req); rerr == nil && !isWafBlock(retried) && !isChallengeResponse(retried) {
				return retried, nil
			}
		}
		return types.HTTPResponse{}, &ChallengeError{VerifyURL: req.URL}
	}

	return resp, nil
}

// HandleRequest decodes a request JSON string, executes it, and returns the
// response marshaled to a JSON string. Malformed JSON or network failures are
// returned as errors (never panic). It is HandleRequestContext with a
// background context — callers that run inside a plugin invoke should pass
// their invoke context so a CDP solve cascade cannot outlive the invoke
// deadline.
func (p *Proxy) HandleRequest(pluginID string, requestJSON string) (string, error) {
	return p.HandleRequestContext(context.Background(), pluginID, requestJSON)
}

// HandleRequestContext is HandleRequest with a caller context: it bounds both
// the upstream call and any challenge-solve cascade triggered along the way.
func (p *Proxy) HandleRequestContext(ctx context.Context, pluginID string, requestJSON string) (string, error) {
	var req types.HTTPRequest
	if err := json.Unmarshal([]byte(requestJSON), &req); err != nil {
		return "", fmt.Errorf("hostnet: malformed request JSON: %w", err)
	}

	resp, err := p.RequestContext(ctx, pluginID, req)
	if err != nil {
		logger.Warn("plugin request failed",
			"plugin", pluginID, "method", req.Method, "url", req.URL, "error", err)
		return "", fmt.Errorf("hostnet: request failed: %w", err)
	}
	if resp.Status < 200 || resp.Status >= 300 {
		logger.Warn("plugin request non-2xx",
			"plugin", pluginID, "method", req.Method, "url", req.URL, "status", resp.Status)
	}

	out, err := json.Marshal(resp)
	if err != nil {
		return "", fmt.Errorf("hostnet: marshal response: %w", err)
	}
	return string(out), nil
}
