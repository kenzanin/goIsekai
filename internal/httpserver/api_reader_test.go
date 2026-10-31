package httpserver

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"goisekai/internal/bridge"
	"goisekai/internal/database"
	"goisekai/internal/hostnet"
	"goisekai/internal/pluginmanager"
)

// readerFixture is the minimal wiring for the reader API handlers: a server
// whose service talks to a temp DB. No manga rows — the 404/400 paths are the
// contract under test here.
func readerFixture(t *testing.T) *Server {
	t.Helper()

	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	proxy := hostnet.NewProxy()
	pmgr := pluginmanager.NewManager(proxy, t.TempDir())
	svc := bridge.NewAppService(db, pmgr, proxy, "", t.TempDir(), nil)

	return &Server{
		logger:  slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
		service: svc,
	}
}

// chiRouteCtx attaches a chi route context carrying the given params.
func chiRouteCtx(req *http.Request, rctx *chi.Context) context.Context {
	return context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
}

func TestApiSetProgressValidation(t *testing.T) {
	s := readerFixture(t)

	post := func(pluginID, mangaID, chapterID, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/api/progress/"+pluginID+"/"+mangaID+"/"+chapterID, strings.NewReader(body))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("pluginID", pluginID)
		rctx.URLParams.Add("mangaID", mangaID)
		rctx.URLParams.Add("chapterID", chapterID)
		req = req.WithContext(chiRouteCtx(req, rctx))
		rec := httptest.NewRecorder()
		s.apiSetProgress(rec, req)
		return rec
	}

	t.Run("invalid json body is 400", func(t *testing.T) {
		if rec := post("t", "m", "c", "{not json"); rec.Code != 400 {
			t.Fatalf("got %d, want 400", rec.Code)
		}
	})
	t.Run("negative page is 400", func(t *testing.T) {
		if rec := post("t", "m", "c", `{"page":-1}`); rec.Code != 400 {
			t.Fatalf("got %d, want 400", rec.Code)
		}
	})
	t.Run("unknown manga still queues 200 (row resolution is async)", func(t *testing.T) {
		// EnqueueSetChapterProgress hands the row lookup to the worker lane,
		// so an unknown manga cannot fail the request synchronously — the
		// handler's only synchronous rejections are body/page validation.
		rec := post("t", "missing", "c", `{"page":1}`)
		if rec.Code != 200 {
			t.Fatalf("got %d, want 200 (queued)", rec.Code)
		}
	})
}

func TestReaderDataMissingParamsIs400(t *testing.T) {
	s := readerFixture(t)

	req := httptest.NewRequest("GET", "/api/reader-data/t/m/c", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("pluginID", "") // simulates a mis-routed empty param
	rctx.URLParams.Add("mangaID", "m")
	rctx.URLParams.Add("chapterID", "c")
	req = req.WithContext(chiRouteCtx(req, rctx))
	rec := httptest.NewRecorder()
	s.readerData(rec, req)

	if rec.Code != 400 {
		t.Fatalf("empty pluginID: got %d, want 400", rec.Code)
	}
}
