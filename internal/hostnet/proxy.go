package hostnet

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"sync"
	"time"

	"goisekai/pkg/types"
)

// Proxy is a sandboxed HTTP client that enforces standard header injection and
// per-plugin cookie persistence. Plugins must not open sockets directly; all
// network access flows through here.
type Proxy struct {
	mu             sync.Mutex
	defaultHeaders map[string]string
	client         *http.Client
	jars           map[string]*cookiejar.Jar // keyed by plugin id
}

// defaultUA is a browser-like User-Agent so requests are less likely to be
// blocked by anti-bot measures.
const defaultUA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36"

// NewProxy initializes a Proxy with browser-like default headers, an
// http.Client with a 30s timeout, and an empty per-plugin jar map.
func NewProxy() *Proxy {
	return &Proxy{
		defaultHeaders: map[string]string{
			"User-Agent":      defaultUA,
			"Accept-Language": "en-US,en;q=0.9",
			// Empty Referer by default; only injected when a plugin sets one.
			"Referer": "",
		},
		client: &http.Client{Timeout: 30 * time.Second},
		jars:   make(map[string]*cookiejar.Jar),
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

// jar returns the cookie jar for pluginID, creating one on first use.
func (p *Proxy) jar(pluginID string) *cookiejar.Jar {
	p.mu.Lock()
	defer p.mu.Unlock()
	jar, ok := p.jars[pluginID]
	if !ok {
		j, err := cookiejar.New(nil)
		if err != nil {
			return nil
		}
		p.jars[pluginID] = j
		jar = j
	}
	return jar
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

	p.injectHeaders(httpReq.Header, req.Headers)

	// Shallow-copy the client so the per-plugin jar is isolated to this call
	// without racing on the shared client's Jar field.
	execClient := *p.client
	execClient.Jar = p.jar(pluginID)

	resp, err := execClient.Do(httpReq)
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

// injectHeaders applies default headers, then overlays the per-request headers
// (page-level headers win). An empty default Referer is skipped.
func (p *Proxy) injectHeaders(header http.Header, overrides map[string]string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for k, v := range p.defaultHeaders {
		if strings.EqualFold(k, "Referer") && v == "" {
			continue
		}
		header.Set(k, v)
	}
	for k, v := range overrides {
		header.Set(k, v)
	}
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
