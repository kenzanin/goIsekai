package hostnet

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// tryLadder walks the candidate ladder (plugin hints then the default ladder),
// retrying the request with each untried profile. It returns the first
// response that is not a challenge and not a WAF block, and pins the winning
// profile only when that response is a clean 2xx/3xx (a 429/5xx from a rung is
// a real origin answer, not a fingerprint win — it must not become permanent).
// ok is false when every candidate fails.
func (p *Proxy) tryLadder(pluginID string, req types.HTTPRequest) (types.HTTPResponse, bool) {
	tried := p.pin(pluginID)
	if tried == "" {
		tried = defaultProfileName
	}
	seen := map[string]bool{tried: true}

	// Without a browser engine fallback, a block-all origin burns the entire
	// profile ladder on every request (8 probes). When no pin exists yet,
	// restrict to stdlib — it provides the key h2 bypass without the burst
	// cost. Pinned plugins still get full rotation (the pin may have gone
	// stale).
	candidates := p.ladderFor(pluginID)

	for _, name := range candidates {
		if seen[name] {
			continue
		}
		seen[name] = true

		if name == stdlibProfileName {
			retried, rerr := p.doRequestStd(pluginID, req)
			if rerr == nil && !isChallengeResponse(retried) && !isWafBlock(retried) {
				if retried.Status < 400 {
					p.markStdlib(pluginID)
				}
				return retried, true
			}
			continue
		}

		retried, rerr := p.doRequestProfile(pluginID, name, req)
		if rerr == nil && !isChallengeResponse(retried) && !isWafBlock(retried) {
			if retried.Status < 400 {
				p.setPin(pluginID, name)
			}
			return retried, true
		}
	}
	return types.HTTPResponse{}, false
}

// ladderFor returns the ordered candidate profile names for pluginID: the
// plugin's declared hints followed by the default ladder, deduplicated.
func (p *Proxy) ladderFor(pluginID string) []string {
	p.mu.Lock()
	hints := append([]string{}, p.hints[pluginID]...)
	p.mu.Unlock()

	out := make([]string, 0, len(hints)+8)
	seen := map[string]bool{}
	for _, n := range append(hints, defaultLadder()...) {
		if seen[n] || !isKnownProfile(n) {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	return out
}

// doRequest executes a single fast-path request with no challenge handling,
// using the plugin's pinned profile (or the default when unpinned).
func (p *Proxy) doRequest(pluginID string, req types.HTTPRequest) (types.HTTPResponse, error) {
	prof := p.pin(pluginID)
	if prof == "" || prof == stdlibProfileName {
		prof = defaultProfileName
	}
	return p.doRequestProfile(pluginID, prof, req)
}

// doRequestProfile executes a single fast-path request using an explicit TLS
// profile, with no challenge handling.
func (p *Proxy) doRequestProfile(pluginID, profileName string, req types.HTTPRequest) (types.HTTPResponse, error) {
	method := req.Method
	if method == "" {
		method = http.MethodGet
	}

	body := strings.NewReader(req.Body)
	httpReq, err := http.NewRequest(method, req.URL, body)
	if err != nil {
		return types.HTTPResponse{}, fmt.Errorf("hostnet: build request: %w", err)
	}

	httpReq.Header = p.buildHeaders(req.Headers)

	// A per-plugin UA override (from SetVerifyCookies) wins over both the
	// default and any per-request User-Agent so the clearance cookie always
	// travels with the browser identity it was issued to.
	if ua := p.uaOverride(pluginID); ua != "" {
		httpReq.Header.Set("User-Agent", ua)
	}

	client, err := p.clientFor(pluginID, profileName)
	if err != nil {
		return types.HTTPResponse{}, fmt.Errorf("hostnet: init client: %w", err)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return types.HTTPResponse{}, fmt.Errorf("hostnet: execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.HTTPResponse{}, fmt.Errorf("hostnet: read response body: %w", err)
	}

	return types.HTTPResponse{
		Status:  resp.StatusCode,
		Headers: flattenHeaders(resp.Header),
		Body:    string(raw),
	}, nil
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
