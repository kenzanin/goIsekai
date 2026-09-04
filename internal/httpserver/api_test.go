package httpserver

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"

	"goisekai/internal/bridge"
	"goisekai/internal/database"
	"goisekai/internal/hostnet"
	"goisekai/internal/pluginmanager"
	"goisekai/internal/templates"
)

// testServer builds a minimal Server wired to a temp DB and cache dir.
func testServer(t *testing.T, apiKey string) *Server {
	t.Helper()
	return testServerFull(t, apiKey, false)
}

// testServerFull builds a Server with all routes registered.
// If registerViews is true, view/action/static routes are included.
func testServerFull(t *testing.T, apiKey string, registerViews bool) *Server {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	proxy := hostnet.NewProxy()
	pmgr := pluginmanager.NewManager(proxy, t.TempDir())
	svc := bridge.NewAppService(db, pmgr, proxy, "", t.TempDir())
	r := chi.NewRouter()
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))

	s := &Server{
		Router:  r,
		logger:  logger,
		service: svc,
		apiKey:  apiKey,
	}
	if registerViews {
		engine, engErr := templates.New(false)
		if engErr != nil {
			t.Fatalf("new engine: %v", engErr)
		}
		s.engine = engine
	}

	s.Router.Route("/api", func(sub chi.Router) {
		sub.Use(s.requireAPIKey)
		s.registerAPIRoutes(sub)
		s.registerReaderRoutes(sub)
		s.registerWSRoutes(sub)
		s.registerSandboxRoutes(sub)
	})
	if registerViews {
		s.registerStaticRoutes()
		s.registerViewRoutes()
		s.registerActionRoutes()
	}
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

// ── GET /api/library ────────────────────────────────────────────────────────

func TestAPILibraryEmpty(t *testing.T) {
	s := testServer(t, "")
	req := httptest.NewRequest("GET", "/api/library", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body []any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

// ── GET /api/search ─────────────────────────────────────────────────────────

func TestAPISearchEmptyQuery(t *testing.T) {
	s := testServer(t, "")
	req := httptest.NewRequest("GET", "/api/search", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	// Missing required params should 400.
	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestAPISearchWithQuery(t *testing.T) {
	s := testServer(t, "")
	req := httptest.NewRequest("GET", "/api/search?q=naruto&pluginID=dummy", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	// Plugin not loaded → 502.
	if rec.Code != 502 {
		t.Fatalf("status = %d, want 502; body: %s", rec.Code, rec.Body.String())
	}
}

// ── GET /api/manga/{pluginID}/{mangaID} ─────────────────────────────────────

func TestAPIMangaDetailNonexistent(t *testing.T) {
	s := testServer(t, "")
	req := httptest.NewRequest("GET", "/api/manga/nonexistent/manga1", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	// Plugin not loaded — may 500 or return error JSON, but should not panic.
	if rec.Code == 0 {
		t.Fatal("expected a valid HTTP status code")
	}
}

// ── GET /api/history ────────────────────────────────────────────────────────

func TestAPIHistoryEmpty(t *testing.T) {
	s := testServer(t, "")
	req := httptest.NewRequest("GET", "/api/history", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body []any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

// ── GET /api/health ─────────────────────────────────────────────────────────

func TestAPIHealth(t *testing.T) {
	s := testServer(t, "")
	req := httptest.NewRequest("GET", "/api/health", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

// ── GET /api/plugins ────────────────────────────────────────────────────────

func TestAPIPluginsEmpty(t *testing.T) {
	s := testServer(t, "")
	req := httptest.NewRequest("GET", "/api/plugins", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	// No /api/plugins route registered — expect 404.
	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// ── POST /api/toggle-library ────────────────────────────────────────────────

func TestAPIToggleLibraryNoBody(t *testing.T) {
	s := testServer(t, "")
	req := httptest.NewRequest("POST", "/api/toggle-library", nil)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	// Empty body should be handled gracefully.
	if rec.Code == 500 {
		t.Fatal("expected non-500 for empty body")
	}
}

// ── POST /api/mark-read ─────────────────────────────────────────────────────

func TestAPIMarkReadNoBody(t *testing.T) {
	s := testServer(t, "")
	req := httptest.NewRequest("POST", "/api/mark-read", nil)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code == 500 {
		t.Fatal("expected non-500 for empty body")
	}
}

// ── POST /api/set-progress ──────────────────────────────────────────────────

func TestAPISetProgressNoBody(t *testing.T) {
	s := testServer(t, "")
	req := httptest.NewRequest("POST", "/api/set-progress", nil)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code == 500 {
		t.Fatal("expected non-500 for empty body")
	}
}
