package hostnet

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"sort"
	"strings"
	"sync"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"

	"goisekai/pkg/types"
)

// Proxy is a sandboxed HTTP client that enforces standard header injection and
// per-plugin cookie persistence. Plugins must not open sockets directly; all
// network access flows through here.
type Proxy struct {
	mu             sync.Mutex
	defaultHeaders map[string]string
	clients        map[string]tls_client.HttpClient // keyed by plugin id
}

// defaultUA is a browser-like User-Agent so requests are less likely to be
// blocked by anti-bot measures.
const defaultUA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36"

// defaultHeaderOrder fixes the order in which default headers are applied so
// the derived HeaderOrderKey is deterministic regardless of map iteration.
var defaultHeaderOrder = []string{"User-Agent", "Accept-Language", "Referer"}

// NewProxy initializes a Proxy with browser-like default headers and an empty
// per-plugin client map. Each client is created lazily on first use.
func NewProxy() *Proxy {
	return &Proxy{
		defaultHeaders: map[string]string{
			"User-Agent":      defaultUA,
			"Accept-Language": "en-US,en;q=0.9",
			// Empty Referer by default; only injected when a plugin sets one.
			"Referer": "",
		},
		clients: make(map[string]tls_client.HttpClient),
	}
}

// DefaultHeaders returns a copy of the default header set applied to every
// request.
func (p *Proxy) DefaultHeaders() map[string]string {
	out := make(map[string]string, len(p.defaultHeaders))
	maps.Copy(out, p.defaultHeaders)
	return out
}

// SetDefaultHeader overrides a default header applied to every request.
func (p *Proxy) SetDefaultHeader(key, value string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.defaultHeaders[key] = value
}

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

	return types.HTTPResponse{
		Status:  resp.StatusCode,
		Headers: flattenHeaders(resp.Header),
		Body:    string(raw),
	}, nil
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
