package hostnet

import (
	"fmt"
	"io"
	"strings"

	http "github.com/bogdanfinn/fhttp"

	"goisekai/pkg/types"
)

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
