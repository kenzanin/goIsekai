package hostnet

import (
	"context"
	"fmt"
	"io"
	"maps"
	"strings"

	http "github.com/bogdanfinn/fhttp"

	"goisekai/pkg/types"
)

// tryLadder walks the candidate ladder (plugin hints then the default ladder),
// retrying the request with each untried profile. On each profile it first
// walks the plugin's remaining User-Agent entries (task 1.5) before falling
// through to the next TLS profile; a UA that clears the block on the current
// profile is pinned and returned. It returns the first response that is not
// blocked (a challenge, a WAF block, or an extra block the caller reports —
// the image path passes isHTMLImageBlock so a 200 HTML interstitial keeps the
// walk going instead of being pinned as a win), and pins the winning profile
// only when that response is a clean 2xx/3xx (a 429/5xx from a rung is a real
// origin answer, not a fingerprint win — it must not become permanent).
// ok is false when every candidate fails.
func (p *Proxy) tryLadder(ctx context.Context, pluginID string, req types.HTTPRequest, extraBlock func(types.HTTPResponse) bool) (types.HTTPResponse, bool) {
	tried := p.pin(pluginID)
	if tried == "" {
		tried = defaultProfileName
	}
	seen := map[string]bool{tried: true}
	seenUA := map[string]bool{}

	blocked := func(resp types.HTTPResponse) bool {
		if isChallengeResponse(resp) || isWafBlock(resp) {
			return true
		}
		return extraBlock != nil && extraBlock(resp)
	}

	// A verify-seeded or pinned UA already tried by the caller before reaching
	// tryLadder counts as attempted, so the walk starts from the untried rest.
	if p.uaPin(pluginID) != "" {
		seenUA[p.uaPin(pluginID)] = true
	}

	// Without a browser engine fallback, a block-all origin burns the entire
	// profile ladder on every request (8 probes). When no pin exists yet,
	// restrict to stdlib — it provides the key h2 bypass without the burst
	// cost. Pinned plugins still get full rotation (the pin may have gone
	// stale).
	candidates := p.ladderFor(pluginID)

	// runProfile executes the request on profile `name` with an explicit UA
	// override (empty = no override, the resolved default is used).
	runProfile := func(name, uaOverride string) (types.HTTPResponse, bool, error) {
		orig := req.Headers
		probe := types.HTTPRequest{
			Method:  req.Method,
			URL:     req.URL,
			Body:    req.Body,
			Headers: map[string]string{},
		}
		maps.Copy(probe.Headers, orig)
		if uaOverride != "" {
			probe.Headers["User-Agent"] = uaOverride
		}
		var resp types.HTTPResponse
		var rerr error
		if name == stdlibProfileName {
			resp, rerr = p.doRequestStd(ctx, pluginID, probe)
		} else {
			resp, rerr = p.doRequestProfile(ctx, pluginID, name, probe)
		}
		ok := rerr == nil && !blocked(resp)
		return resp, ok, rerr
	}

	for _, name := range candidates {
		// Cancelled caller: stop probing profiles rather than burning the rest
		// of the ladder on a request nobody is waiting for.
		if ctx.Err() != nil {
			break
		}
		if seen[name] {
			continue
		}
		seen[name] = true

		// Walk the plugin's untried User-Agent entries on this profile before
		// falling through to the next TLS profile (decision 3). A plugin that
		// declares no ladder probes exactly once per rung with the resolved
		// (host-default) UA — the pre-ladder shape — so probes-per-rung only
		// grows for plugins that opt in via user_agents.
		var walk []string
		if p.uaList(pluginID) != nil {
			walk = p.uaCandidates(pluginID)
		} else {
			walk = []string{""} // no override: resolveUserAgent picks the default
		}
		for _, ua := range walk {
			if ctx.Err() != nil {
				break
			}
			if p.uaList(pluginID) != nil {
				if seenUA[ua] {
					continue
				}
				seenUA[ua] = true
			}
			retried, ok, rerr := runProfile(name, ua)
			if rerr != nil || !ok {
				continue
			}
			if retried.Status < 400 {
				// A clean answer: pin the profile, and the winning UA entry
				// only when the plugin declared a ladder.
				if name == stdlibProfileName {
					p.markStdlib(pluginID)
				} else {
					p.setPin(pluginID, name)
				}
				if p.uaList(pluginID) != nil {
					p.setUaPin(pluginID, ua)
				}
				return retried, true
			}
			// A 429/5xx is a real origin answer, not a fingerprint win: hand
			// it to the caller unpinned (matches the profile-rung rule).
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
func (p *Proxy) doRequest(ctx context.Context, pluginID string, req types.HTTPRequest) (types.HTTPResponse, error) {
	prof := p.pin(pluginID)
	if prof == "" || prof == stdlibProfileName {
		prof = defaultProfileName
	}
	return p.doRequestProfile(ctx, pluginID, prof, req)
}

// doRequestProfile executes a single fast-path request using an explicit TLS
// profile, with no challenge handling.
func (p *Proxy) doRequestProfile(ctx context.Context, pluginID, profileName string, req types.HTTPRequest) (types.HTTPResponse, error) {
	method := req.Method
	if method == "" {
		method = http.MethodGet
	}

	body := strings.NewReader(req.Body)
	httpReq, err := http.NewRequestWithContext(ctx, method, req.URL, body)
	if err != nil {
		return types.HTTPResponse{}, fmt.Errorf("hostnet: build request: %w", err)
	}

	httpReq.Header = p.buildHeaders(req.Headers)

	// A plugin-supplied Host header is a vhost override (mangade.io's valid
	// certificate fronts the api.mangade.io vhost this way). Promote it to
	// req.Host: the HTTP/2 :authority reads req.Host, and leaving it only in
	// the Header map would silently downgrade the override on h2 connections.
	if h := httpReq.Header.Get("Host"); h != "" {
		httpReq.Host = h
		httpReq.Header.Del("Host")
	}

	// A per-request UA the plugin set on this call, and the plugin's declared
	// ladder (first/pinned entry), resolve through one helper so the tls and
	// stdlib paths cannot drift. A verify-seeded UA still wins over both;
	// Sec-CH-UA is kept in lockstep by the reapply below.
	perReqUA := ""
	for k, v := range req.Headers {
		if strings.EqualFold(k, "User-Agent") {
			perReqUA = v
		}
	}
	if ua := p.resolveUserAgent(pluginID, perReqUA); ua != "" {
		httpReq.Header.Set("User-Agent", ua)
		p.reapplySecCHUA(httpReq.Header)
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

	// Flaky ISP edges reset the TCP connection mid-body-read ("read:
	// connection reset by peer"). The response headers already committed, so
	// a partial body is unrecoverable — but a fresh connection usually
	// succeeds, so retry the whole request a couple of times on that specific
	// transient read error before giving up.
	var raw []byte
	for readAttempt := range 3 {
		raw, err = io.ReadAll(resp.Body)
		if err == nil {
			break
		}
		if !isTransientReadErr(err) || ctx.Err() != nil {
			return types.HTTPResponse{}, fmt.Errorf("hostnet: read response body: %w", err)
		}
		_ = resp.Body.Close()
		resp, err = client.Do(httpReq)
		if err != nil {
			return types.HTTPResponse{}, fmt.Errorf("hostnet: retry execute request: %w", err)
		}
		_ = readAttempt
	}

	return types.HTTPResponse{
		Status:  resp.StatusCode,
		Headers: flattenHeaders(resp.Header),
		Body:    string(raw),
	}, nil
}
