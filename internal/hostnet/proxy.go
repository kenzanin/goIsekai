package hostnet

import (
	"maps"
	"sync"

	tls_client "github.com/bogdanfinn/tls-client"
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
