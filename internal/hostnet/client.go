package hostnet

import (
	"fmt"
	"sort"
	"strings"

	nethttp "net/http"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
)

// clientFor returns the tls-client for pluginID+profile, creating one on first
// use with that browser TLS profile and the plugin's isolated cookie jar.
// Clients are cached per (plugin, profile) so rotation does not discard cookies.
func (p *Proxy) clientFor(pluginID, profileName string) (tls_client.HttpClient, error) {
	prof, ok := profileByName(profileName)
	if !ok {
		return nil, fmt.Errorf("hostnet: unknown tls profile %q", profileName)
	}
	key := pluginID + "\x00" + profileName

	p.mu.Lock()
	defer p.mu.Unlock()
	if c, ok := p.clients[key]; ok {
		return c, nil
	}
	c, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(),
		tls_client.WithTimeoutSeconds(30),
		tls_client.WithClientProfile(prof),
		tls_client.WithCookieJar(tls_client.NewCookieJar()),
	)
	if err != nil {
		return nil, err
	}
	if seed, ok := p.pendingVerify[pluginID]; ok {
		c.SetCookies(seed.url(), seed.cookies)
	}
	p.clients[key] = c
	return c, nil
}

// uaOverride returns the per-plugin User-Agent override, or "" if none is set.
func (p *Proxy) uaOverride(pluginID string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.uaOverrides[pluginID]
}

// secCHUAFor derives the Sec-CH-UA client hint for a Chromium-family
// User-Agent, returning "" for non-Chromium browsers (Firefox, Safari) which
// must not send the header. Kept tolerant: a UA without a recognizable Chrome
// version yields "".
func secCHUAFor(ua string) string {
	_, after, ok := strings.Cut(ua, "Chrome/")
	if !ok {
		return ""
	}
	ver := after
	if j := strings.IndexAny(ver, " ."); j >= 0 {
		ver = ver[:j]
	}
	if ver == "" {
		return ""
	}
	return `"Chromium";v="` + ver + `", "Google Chrome";v="` + ver + `", "Not-A.Brand";v="99"`
}

// buildHeaders assembles the complete header set for a request. tls-client
// applies its client defaults only when req.Header is empty, so a populated
// header fully replaces them; we must therefore supply the full set here.
// The returned Header carries a lowercase HeaderOrderKey for stable ordering.
func (p *Proxy) buildHeaders(overrides map[string]string) http.Header {
	p.mu.Lock()
	defer p.mu.Unlock()

	header := make(http.Header)
	order := make([]string, 0, len(p.defaultHeaders)+len(overrides))

	for _, k := range defaultHeaderOrder {
		v := p.defaultHeaders[k]
		if strings.EqualFold(k, "Referer") && v == "" {
			continue
		}
		header.Set(k, v)
		order = append(order, strings.ToLower(k))
	}

	// Per-request overrides, in deterministic (sorted) order.
	keys := make([]string, 0, len(overrides))
	for k := range overrides {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		header.Set(k, overrides[k])
		order = append(order, strings.ToLower(k))
	}
	// Keep the client hint in lockstep with the User-Agent actually being
	// sent (per-request or per-plugin overrides included) unless the config
	// supplies one explicitly; drop it for non-Chromium UAs, whose browsers
	// never send the hint.
	if hint := p.secCHUAHint(header.Get("User-Agent")); hint != "" {
		if header.Get("Sec-CH-UA") == "" {
			order = append(order, "sec-ch-ua")
		}
		header.Set("Sec-CH-UA", hint)
	} else if p.secCHUAExplicit == "" {
		header.Del("Sec-CH-UA")
	}
	header[http.HeaderOrderKey] = order
	return header
}

// flattenHeaders collapses multi-valued response headers into a single
// comma-joined string map.
func flattenHeaders(h http.Header) map[string]string {
	out := make(map[string]string, len(h))
	for k, vals := range h {
		out[k] = strings.Join(vals, ", ")
	}
	return out
}

// reapplySecCHUA refreshes the Sec-CH-UA hint after a late User-Agent change
// on a tls-client request header. Explicit config pins always win.
func (p *Proxy) reapplySecCHUA(h http.Header) {
	p.mu.Lock()
	defer p.mu.Unlock()
	ua := h.Get("User-Agent")
	switch {
	case p.secCHUAExplicit != "":
		h.Set("Sec-CH-UA", p.secCHUAExplicit)
	case ua != "" && secCHUAFor(ua) != "":
		h.Set("Sec-CH-UA", secCHUAFor(ua))
	default:
		h.Del("Sec-CH-UA")
	}
}

// reapplySecCHUAStd is reapplySecCHUA for stdlib request headers.
func (p *Proxy) reapplySecCHUAStd(h nethttp.Header) {
	p.mu.Lock()
	defer p.mu.Unlock()
	ua := h.Get("User-Agent")
	switch {
	case p.secCHUAExplicit != "":
		h.Set("Sec-CH-UA", p.secCHUAExplicit)
	case ua != "" && secCHUAFor(ua) != "":
		h.Set("Sec-CH-UA", secCHUAFor(ua))
	default:
		h.Del("Sec-CH-UA")
	}
}
