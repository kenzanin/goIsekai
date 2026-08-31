package hostnet

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	http "github.com/bogdanfinn/fhttp"

	"goisekai/pkg/types"
)

// Request builds, executes, and returns the response for a plugin HTTP request.
// Per-page headers overlay the default headers, and cookies are persisted per
// plugin so they survive across calls.
func (p *Proxy) Request(pluginID string, req types.HTTPRequest) (types.HTTPResponse, error) {
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
	defer func() {
		_ = resp.Body.Close()
	}()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.HTTPResponse{}, fmt.Errorf("hostnet: read response body: %w", err)
	}

	// Anti-bot challenge interstitials (403/503 with an HTML body carrying a
	// known marker) surface as a typed error so callers can prompt for human
	// verification. The text/html guard keeps image/binary responses fail-open.
	if resp.StatusCode == 403 || resp.StatusCode == 503 {
		if strings.Contains(resp.Header.Get("Content-Type"), "text/html") && IsChallengeResponse(resp.StatusCode, raw) {
			return types.HTTPResponse{}, &ChallengeError{}
		}
	}

	return types.HTTPResponse{
		Status:  resp.StatusCode,
		Headers: flattenHeaders(resp.Header),
		Body:    string(raw),
	}, nil
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
