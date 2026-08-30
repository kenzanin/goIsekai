package httpserver

import (
	"embed"
	"fmt"
	"log/slog"
	"net/http"
	"os/exec"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"goisekai/internal/bridge"
	"goisekai/internal/templates"
)

// Server wraps a Chi router and HTTP server.
type Server struct {
	Router  *chi.Mux
	addr    string
	logger  *slog.Logger
	assets  embed.FS
	service *bridge.AppService
	engine  *templates.Engine
}

// New creates a new Server with Chi middleware and all routes registered.
func New(host string, port int, assets embed.FS, svc *bridge.AppService, logger *slog.Logger, engine *templates.Engine) *Server {
	defaultAssets = assets

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	s := &Server{
		Router:  r,
		addr:    fmt.Sprintf("%s:%d", host, port),
		logger:  logger,
		assets:  assets,
		service: svc,
		engine:  engine,
	}
	s.routes()
	return s
}

// ListenAndServe starts the HTTP server (blocking).
func (s *Server) ListenAndServe() error {
	s.logger.Info("starting HTTP server", "addr", "http://"+s.addr)
	return http.ListenAndServe(s.addr, s.Router)
}

// OpenBrowser launches the default browser pointing at the server address.
func (s *Server) OpenBrowser() {
	url := "http://" + s.addr
	go func() {
		// ponytail: xdg-open covers Linux; extend map for windows/darwin when
		// cross-platform builds actually ship.
		_ = exec.Command("xdg-open", url).Start()
	}()
}
