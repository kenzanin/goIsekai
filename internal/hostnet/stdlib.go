package hostnet

import (
	"fmt"
	"io"
	nethttp "net/http"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"

	"goisekai/pkg/types"
)

// Stdlib h2 fallback: some WAFs (e.g. mangafire's) block the utls browser
// fingerprint itself while letting Go's stock TLS + HTTP/2 through. When the
// tls-client path is blocked, the request is retried once over a stdlib
// client; a plugin whose fallback succeeds sticks to the stdlib path.

var stdTransport = &nethttp.Transport{ForceAttemptHTTP2: true}

// stdlibPrefers reports whether pluginID is pinned to the stdlib path.
func (p *Proxy) stdlibPrefers(pluginID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stdlibPref[pluginID]
}

// markStdlib pins pluginID to the stdlib path.
func (p *Proxy) markStdlib(pluginID string) {
	p.mu.Lock()
	p.stdlibPref[pluginID] = true
	p.mu.Unlock()
}

// doRequestStd executes a single request over the stdlib client (HTTP/2 via
// ALPN). No challenge handling; headers mirror the tls-client build.
func (p *Proxy) doRequestStd(pluginID string, req types.HTTPRequest) (types.HTTPResponse, error) {
	method := req.Method
	if method == "" {
		method = nethttp.MethodGet
	}
	sreq, err := nethttp.NewRequest(method, req.URL, strings.NewReader(req.Body))
	if err != nil {
		return types.HTTPResponse{}, fmt.Errorf("hostnet: build stdlib request: %w", err)
	}
	for k, v := range p.headerMap(req.Headers) {
		sreq.Header.Set(k, v)
	}
	if ua := p.uaOverride(pluginID); ua != "" {
		sreq.Header.Set("User-Agent", ua)
	}

	client := &nethttp.Client{Transport: stdTransport, Timeout: 30 * time.Second}
	resp, err := client.Do(sreq)
	if err != nil {
		return types.HTTPResponse{}, fmt.Errorf("hostnet: stdlib execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.HTTPResponse{}, fmt.Errorf("hostnet: stdlib read response body: %w", err)
	}
	out := make(map[string]string, len(resp.Header))
	for k, vals := range resp.Header {
		out[k] = strings.Join(vals, ", ")
	}
	return types.HTTPResponse{Status: resp.StatusCode, Headers: out, Body: string(raw)}, nil
}

// headerMap flattens the tls-client header build (defaults + overrides) into a
// plain map for the stdlib request, skipping the fhttp-only ordering key.
func (p *Proxy) headerMap(overrides map[string]string) map[string]string {
	h := p.buildHeaders(overrides)
	out := make(map[string]string, len(h))
	for k, vals := range h {
		if k == http.HeaderOrderKey || len(vals) == 0 {
			continue
		}
		out[k] = vals[0]
	}
	return out
}

// isWafBlock reports a JSON anti-bot block (e.g. mangafire's
// {"error":"captcha_required","challenge":"/@waf/challenge"}) that the
// HTML-body challenge detector misses.
func isWafBlock(resp types.HTTPResponse) bool {
	if resp.Status != 403 && resp.Status != 503 {
		return false
	}
	b := strings.ToLower(resp.Body)
	if len(b) > 4096 {
		b = b[:4096]
	}
	return strings.Contains(b, "captcha_required") || strings.Contains(b, "@waf/challenge")
}
