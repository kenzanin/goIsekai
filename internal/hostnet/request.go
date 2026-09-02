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
// Anti-bot challenge handling: when a plugin declares needs_js, the browser
// engine is run preemptively to clear the site before the fast path. When a
// challenge response is detected and an engine is enabled, the host solves the
// challenge, seeds the harvested cookies into the plugin's jar, and retries the
// original request once before surfacing a challenge error.
func (p *Proxy) Request(pluginID string, req types.HTTPRequest) (types.HTTPResponse, error) {
	// needs_js: preemptively solve + seed cookies via the browser engine so the
	// client-side site is already cleared when the fast path runs.
	if p.needsJSHint(pluginID) && p.CDPConfig().enabled() {
		_ = p.solveAndSeed(pluginID, req.URL)
	}

	resp, err := p.doRequest(pluginID, req)
	if err != nil {
		return types.HTTPResponse{}, err
	}

	if !isChallengeResponse(resp) {
		return resp, nil
	}

	// Challenge detected. Solve via the engine, seed cookies, and retry once.
	if p.CDPConfig().enabled() && p.solveAndSeed(pluginID, req.URL) == nil {
		if retried, rerr := p.doRequest(pluginID, req); rerr == nil && !isChallengeResponse(retried) {
			return retried, nil
		}
	}

	return types.HTTPResponse{}, &ChallengeError{}
}

// doRequest executes a single fast-path request with no challenge handling.
func (p *Proxy) doRequest(pluginID string, req types.HTTPRequest) (types.HTTPResponse, error) {
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

	client, err := p.client(pluginID)
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
