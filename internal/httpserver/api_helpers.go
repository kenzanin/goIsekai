package httpserver

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"

	"github.com/goccy/go-json"

	"goisekai/internal/bridge"
)

// writeJSON encodes v as JSON with the given HTTP status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeErr writes a JSON error envelope {"error":"msg"} with the given status.
func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// serviceQueueFull maps lane backpressure from bridge calls to 503 (design
// D6: overload visible, request deadline honored). Returns true when it wrote
// the response; the caller returns immediately.
func (s *Server) serviceQueueFull(w http.ResponseWriter, err error, op string) bool {
	if !bridge.IsQueueFull(err) {
		return false
	}
	s.logger.Warn("interactive lane saturated", "op", op, "error", err)
	writeErr(w, http.StatusServiceUnavailable, op+" busy, retry shortly")
	return true
}

// requireAPIKey returns a middleware that enforces X-API-Key authentication
// when an API key is configured. When the key is empty the middleware is a
// no-op passthrough.
func (s *Server) requireAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.apiKey == "" {
			next.ServeHTTP(w, r)
			return
		}
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-API-Key")), []byte(s.apiKey)) != 1 {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// warnIfOpenAPI logs a warning when the server listens on a non-loopback
// address without an API key configured.
func warnIfOpenAPI(logger *slog.Logger, host, apiKey string) {
	if apiKey != "" {
		return
	}
	h := strings.ToLower(host)
	if h == "127.0.0.1" || h == "localhost" || h == "::1" || h == "" {
		return
	}
	logger.Warn("API is exposed without authentication — set -apiKey or api_key in goisekai.ini",
		"host", host)
}

// ---------------------------------------------------------------------------
// API endpoint handlers
// ---------------------------------------------------------------------------
