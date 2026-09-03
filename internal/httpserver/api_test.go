package httpserver

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"

	"goisekai/internal/bridge"
	"goisekai/internal/database"
	"goisekai/internal/hostnet"
)

// testServer builds a minimal Server wired to a temp DB and cache dir.
func testServer(t *testing.T, apiKey string) *Server {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	svc := bridge.NewAppService(db, nil, hostnet.NewProxy(), "", t.TempDir())
	r := chi.NewRouter()
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	s := &Server{
		Router:  r,
		logger:  logger,
		service: svc,
		apiKey:  apiKey,
	}
	s.Router.Route("/api", func(sub chi.Router) {
		sub.Use(s.requireAPIKey)
		s.registerAPIRoutes(sub)
	})
	return s
}

// ── writeJSON / writeErr envelope shape ─────────────────────────────────────

func TestWriteJSONBarePayload(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, map[string]string{"status": "ok"})
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("body = %v, want {status: ok}", body)
	}
	// Must NOT be wrapped in an "error" envelope.
	if _, ok := body["error"]; ok {
		t.Fatal("bare payload should not contain 'error' key")
	}
}

func TestWriteErrEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	writeErr(rec, http.StatusBadRequest, "bad thing")
	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["error"] != "bad thing" {
		t.Fatalf("error = %q, want %q", body["error"], "bad thing")
	}
}

// ── requireAPIKey middleware ─────────────────────────────────────────────────

func TestRequireAPIKeyEmptyPassthrough(t *testing.T) {
	s := testServer(t, "")
	req := httptest.NewRequest("GET", "/api/health", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("empty key should pass through; got %d", rec.Code)
	}
}

func TestRequireAPIKeyAccepts(t *testing.T) {
	s := testServer(t, "secret")
	req := httptest.NewRequest("GET", "/api/health", nil)
	req.Header.Set("X-API-Key", "secret")
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("matching key should pass; got %d", rec.Code)
	}
}

func TestRequireAPIKeyRejectsMissing(t *testing.T) {
	s := testServer(t, "secret")
	req := httptest.NewRequest("GET", "/api/health", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("missing key should 401; got %d", rec.Code)
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"] != "unauthorized" {
		t.Fatalf("error body = %v", body)
	}
}

func TestRequireAPIKeyRejectsWrong(t *testing.T) {
	s := testServer(t, "secret")
	req := httptest.NewRequest("GET", "/api/health", nil)
	req.Header.Set("X-API-Key", "wrong")
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("wrong key should 401; got %d", rec.Code)
	}
}

// ── GET /api/image endpoint ─────────────────────────────────────────────────

func TestAPIImageValidPNG(t *testing.T) {
	// Build a tiny valid PNG.
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	imgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBuf.Bytes())
	}))
	t.Cleanup(imgSrv.Close)

	s := testServer(t, "")
	url := imgSrv.URL + "/test.png"
	req := httptest.NewRequest("GET", "/api/image/p1/m1/c1?url="+url, nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	ct := rec.Header().Get("Content-Type")
	if ct == "" {
		t.Fatal("missing Content-Type header")
	}
	// The response body should be the image bytes (at least as long as the PNG).
	if rec.Body.Len() < 8 {
		t.Fatalf("body too short: %d bytes", rec.Body.Len())
	}
}

func TestAPIImageMissingURL(t *testing.T) {
	s := testServer(t, "")
	req := httptest.NewRequest("GET", "/api/image/p1/m1/c1", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"] == "" {
		t.Fatal("expected non-empty error message")
	}
}

func TestAPIImage502OnGarbage(t *testing.T) {
	garbageSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("this is not an image at all"))
	}))
	t.Cleanup(garbageSrv.Close)

	s := testServer(t, "")
	url := garbageSrv.URL + "/bad.png"
	req := httptest.NewRequest("GET", "/api/image/p1?url="+url, nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != 502 {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
}

func TestAPIImage401WhenKeyConfigured(t *testing.T) {
	s := testServer(t, "secret")
	req := httptest.NewRequest("GET", "/api/image/p1/m1/c1?url=http://example.com/img.png", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestAPIImageRefererPassthrough(t *testing.T) {
	// Serve a tiny PNG but verify the Referer header arrives.
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	var gotReferer string
	imgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReferer = r.Header.Get("Referer")
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBuf.Bytes())
	}))
	t.Cleanup(imgSrv.Close)

	s := testServer(t, "")
	url := imgSrv.URL + "/ref.png"
	req := httptest.NewRequest("GET", "/api/image/p1/m1/c1?url="+url, nil)
	req.Header.Set("Referer", "http://manga-site.example/ch/1")
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if gotReferer != "http://manga-site.example/ch/1" {
		t.Fatalf("Referer not forwarded: got %q", gotReferer)
	}
}

func TestAPIImageEmptyMangaIDChapterID(t *testing.T) {
	// Thumbnail case: mangaID and chapterID may be empty in path.
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	imgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBuf.Bytes())
	}))
	t.Cleanup(imgSrv.Close)

	s := testServer(t, "")
	url := imgSrv.URL + "/thumb.png"
	req := httptest.NewRequest("GET", "/api/image/p1?url="+url, nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

// Silence unused import warnings for helper packages.
var _ = os.Stat
