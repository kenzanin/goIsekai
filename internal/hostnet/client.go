package hostnet

import (
	"sort"
	"strings"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

// client returns the tls-client for pluginID, creating one on first use with a
// browser TLS profile and an isolated cookie jar.
func (p *Proxy) client(pluginID string) (tls_client.HttpClient, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	c, ok := p.clients[pluginID]
	if ok {
		return c, nil
	}
	c, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(),
		tls_client.WithTimeoutSeconds(30),
		tls_client.WithClientProfile(profiles.Chrome_146),
		tls_client.WithCookieJar(tls_client.NewCookieJar()),
	)
	if err != nil {
		return nil, err
	}
	p.clients[pluginID] = c
	return c, nil
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
