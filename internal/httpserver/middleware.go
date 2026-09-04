package httpserver

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"log/slog"
)

// statusWriter wraps http.ResponseWriter to capture the status code and
// response size for logging.
type statusWriter struct {
	http.ResponseWriter
	status int
	size   int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.size += n
	return n, err
}

// Hijack forwards to the wrapped writer so WebSocket upgrades (which need
// connection hijacking) survive this wrapper.
func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("wrapped writer does not support Hijack")
	}
	return h.Hijack()
}

// Flush forwards so streaming responses keep working through the wrapper.
func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// requestID generates a random 8-char hex ID.
func requestID() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// logSkipPaths lists path prefixes that should not be logged.
var logSkipPaths = []string{
	"/health",
	"/ready",
	"/static/",
}

// logSkipExts lists file extensions that should not be logged.
var logSkipExts = []string{
	".js", ".css", ".webp", ".br", ".png", ".jpg", ".svg", ".woff2", ".ico",
}

// shouldSkipLog returns true if the request path matches a skip rule.
func shouldSkipLog(path string) bool {
	for _, p := range logSkipPaths {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	for _, ext := range logSkipExts {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

// loggingMiddleware logs each HTTP request with structured fields after the
// handler returns. Skips health checks and static asset requests.
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if shouldSkipLog(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// Preserve or generate request ID.
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			id = requestID()
		}
		w.Header().Set("X-Request-Id", id)

		sw := &statusWriter{ResponseWriter: w}
		start := time.Now()
		next.ServeHTTP(sw, r)
		latency := time.Since(start)

		attrs := []slog.Attr{
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", sw.status),
			slog.Int("size_bytes", sw.size),
			slog.Int64("latency_ms", latency.Milliseconds()),
			slog.String("request_id", id),
		}

		switch {
		case sw.status >= 500:
			s.logger.LogAttrs(r.Context(), slog.LevelError, "http", attrs...)
		case sw.status >= 400:
			s.logger.LogAttrs(r.Context(), slog.LevelWarn, "http", attrs...)
		default:
			s.logger.LogAttrs(r.Context(), slog.LevelInfo, "http", attrs...)
		}
	})
}
