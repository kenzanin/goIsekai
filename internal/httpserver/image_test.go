package httpserver

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"goisekai/internal/bridge"
	"goisekai/internal/database"
	"goisekai/internal/hostnet"
	"goisekai/internal/pluginmanager"
)

// testImageServer creates a minimal server for testing image endpoint.
func testImageServer(t *testing.T) *Server {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	proxy := hostnet.NewProxy()
	pmgr := pluginmanager.NewManager(proxy, t.TempDir())
	svc := bridge.NewAppService(db, pmgr, proxy, "", t.TempDir(), nil)

	r := chi.NewRouter()
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	s := &Server{
		Router:  r,
		logger:  logger,
		service: svc,
	}
	s.registerImageRoutes()
	return s
}

func TestImageRouteRegistered(t *testing.T) {
	s := testImageServer(t)

	req := httptest.NewRequest("GET", "/image", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code == http.StatusNotFound {
		t.Error("image route not registered")
	}
}

func TestImageMissingParamsReturnsBadRequest(t *testing.T) {
	s := testImageServer(t)

	tests := []struct {
		name string
		url  string
	}{
		{"missing pluginID", "/image?url=http://example.com"},
		{"missing url", "/image?pluginID=test"},
		{"missing both", "/image"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.url, nil)
			rec := httptest.NewRecorder()
			s.Router.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected 400 Bad Request, got %d", rec.Code)
			}
		})
	}
}

func TestImageRouteExistsWithPrioParam(t *testing.T) {
	s := testImageServer(t)

	// Verify the endpoint accepts the prio parameter without crashing
	// We use a dummy URL - the endpoint will fail to fetch but should handle prio
	req := httptest.NewRequest("GET", "/image?pluginID=test&url=http://dummy.local/img.png&prio=high&referer=http://test.com", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	// Should not be 404 (route must exist)
	// May be 502 (can't fetch from dummy) or 400 (validation error)
	if rec.Code == http.StatusNotFound {
		t.Error("image route not found with prio parameter")
	}
}
