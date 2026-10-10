package hostnet

import (
	"context"
	nethttp "net/http"
	"time"

	"goisekai/internal/logger"
)

// Preconnect opens a HEAD request to the given host to warm the connection pool.
// It is fire-and-forget: failures are logged at debug level and silently ignored.
func (p *Proxy) Preconnect(host string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := nethttp.NewRequestWithContext(ctx, nethttp.MethodHead, "https://"+host, nil)
	if err != nil {
		logger.Debug("preconnect build request", "host", host, "error", err)
		return
	}
	req.Header.Set("User-Agent", defaultUA)

	resp, err := p.stdlibTransport.RoundTrip(req)
	if err != nil {
		logger.Debug("preconnect failed", "host", host, "error", err)
		return
	}
	_ = resp.Body.Close()
	if resp.StatusCode >= 400 {
		logger.Debug("preconnect non-2xx", "host", host, "status", resp.StatusCode)
		logger.Debug("transport request completed", "host", host, "status", resp.StatusCode)
		return
	}
	logger.Info("preconnected", "host", host, "status", resp.StatusCode)
}

// GetTransportStats returns current transport stats (idle connections count).
// Returns 0 if stats not available (stdlib Transport has no public stats API).
func (p *Proxy) GetTransportStats() (idleConns int) {
	// stdlib Transport doesn't expose idle connection count publicly
	// We log at debug level on each preconnect instead
	return 0
}
