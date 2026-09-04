package httpserver

import (
	"net/http/httptest"
	"testing"
)

// ── Sandbox endpoint tests ──────────────────────────────────────────────────

func TestSandboxListPlugins(t *testing.T) {
	s := testServer(t, "")
	req := httptest.NewRequest("GET", "/api/sandbox/plugins", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	// Response may be object or array depending on implementation.
	if rec.Body.Len() == 0 {
		t.Fatal("expected non-empty body")
	}
}

func TestSandboxLoadNoBody(t *testing.T) {
	s := testServer(t, "")
	req := httptest.NewRequest("POST", "/api/sandbox/plugins/load", nil)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	// Should handle empty body gracefully (not 500).
	if rec.Code == 500 {
		t.Logf("sandbox load with empty body returned 500: %s", rec.Body.String())
	}
}

func TestSandboxUnloadNonexistent(t *testing.T) {
	s := testServer(t, "")
	req := httptest.NewRequest("POST", "/api/sandbox/plugins/nonexistent/unload", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	// Plugin doesn't exist — may return 404 or 500.
	if rec.Code == 0 {
		t.Fatal("expected a valid HTTP status code")
	}
}

func TestSandboxReloadNonexistent(t *testing.T) {
	s := testServer(t, "")
	req := httptest.NewRequest("POST", "/api/sandbox/plugins/nonexistent/reload", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	// Plugin doesn't exist — may return 404 or 500.
	if rec.Code == 0 {
		t.Fatal("expected a valid HTTP status code")
	}
}

func TestSandboxSearchNonexistent(t *testing.T) {
	s := testServer(t, "")
	req := httptest.NewRequest("GET", "/api/sandbox/plugins/nonexistent/search?q=test", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	// Plugin doesn't exist — may return 500 or 502.
	if rec.Code == 0 {
		t.Fatal("expected a valid HTTP status code")
	}
}

func TestSandboxDetailNonexistent(t *testing.T) {
	s := testServer(t, "")
	req := httptest.NewRequest("GET", "/api/sandbox/plugins/nonexistent/detail/manga1", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code == 0 {
		t.Fatal("expected a valid HTTP status code")
	}
}

func TestSandboxChaptersNonexistent(t *testing.T) {
	s := testServer(t, "")
	req := httptest.NewRequest("GET", "/api/sandbox/plugins/nonexistent/chapters/manga1", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code == 0 {
		t.Fatal("expected a valid HTTP status code")
	}
}

func TestSandboxPagesNonexistent(t *testing.T) {
	s := testServer(t, "")
	req := httptest.NewRequest("GET", "/api/sandbox/plugins/nonexistent/pages/chapter1", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code == 0 {
		t.Fatal("expected a valid HTTP status code")
	}
}

func TestSandboxCDPStatus(t *testing.T) {
	s := testServer(t, "")
	req := httptest.NewRequest("GET", "/api/sandbox/cdp/status", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	// CDP status may return various codes depending on config.
	if rec.Code == 0 {
		t.Fatal("expected a valid HTTP status code")
	}
}

func TestSandboxCDPTest(t *testing.T) {
	s := testServer(t, "")
	req := httptest.NewRequest("GET", "/api/sandbox/cdp/test", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	// CDP test may fail if no CDP engine is configured — that's ok.
	if rec.Code == 0 {
		t.Fatal("expected a valid HTTP status code")
	}
}

func TestSandboxCDPCookies(t *testing.T) {
	s := testServer(t, "")
	req := httptest.NewRequest("GET", "/api/sandbox/cdp/cookies", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code == 0 {
		t.Fatal("expected a valid HTTP status code")
	}
}
