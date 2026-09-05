package httpserver

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os/exec"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"goisekai/internal/bridge"
	"goisekai/internal/templates"
)

// Server wraps a Chi router and HTTP server.
type Server struct {
	Router  *chi.Mux
	http    *http.Server
	logger  *slog.Logger
	assets  embed.FS
	service *bridge.AppService
	engine  *templates.Engine
	apiKey  string
}

// New creates a new Server with Chi middleware and all routes registered.
func New(host string, port int, apiKey string, assets embed.FS, svc *bridge.AppService, logger *slog.Logger, engine *templates.Engine) *Server {

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	// Compress must not wrap WebSocket upgrades (it buffers and kills the 101
	// handshake) — skip requests asking for Connection: Upgrade.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.EqualFold(r.Header.Get("Connection"), "Upgrade") {
				next.ServeHTTP(w, r)
				return
			}
			middleware.Compress(5)(next).ServeHTTP(w, r)
		})
	})

	addr := fmt.Sprintf("%s:%d", host, port)
	s := &Server{
		Router:  r,
		http:    &http.Server{Addr: addr, Handler: r},
		logger:  logger,
		assets:  assets,
		service: svc,
		engine:  engine,
		apiKey:  apiKey,
	}
	warnIfOpenAPI(logger, host, apiKey)
	s.routes()
	return s
}

// param returns the named route parameter, percent-decoded. Browsers may
// percent-encode reserved characters in URL paths (Chrome encodes ":" as %3A
// during fetch normalization); chi matches on r.URL.RawPath and returns the raw
// segment to both chi.URLParam and r.PathValue. Decode here so a chapter ID like
// "slug%3Achapter-97" reaches the plugin as "slug:chapter-97".
func param(r *http.Request, name string) string {
	if v := chi.URLParam(r, name); v != "" {
		if dec, err := url.PathUnescape(v); err == nil {
			return dec
		}
		return v
	}
	if v := r.PathValue(name); v != "" {
		if dec, err := url.PathUnescape(v); err == nil {
			return dec
		}
		return v
	}
	return ""
}

// ListenAndServe starts the HTTP server (blocking).
func (s *Server) ListenAndServe() error {
	s.logger.Info("starting HTTP server", "addr", "http://"+s.http.Addr)
	return s.http.ListenAndServe()
}

// Shutdown gracefully shuts down the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down HTTP server")
	return s.http.Shutdown(ctx)
}

// OpenBrowser launches the default browser pointing at the server address.
func (s *Server) OpenBrowser() {
	url := "http://" + s.http.Addr
	go func() {
		// ponytail: xdg-open covers Linux; extend map for windows/darwin when
		// cross-platform builds actually ship.
		_ = exec.Command("xdg-open", url).Start()
	}()
}
