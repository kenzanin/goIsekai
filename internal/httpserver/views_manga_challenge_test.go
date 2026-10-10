package httpserver

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"goisekai/internal/bridge"
	"goisekai/internal/database"
	"goisekai/internal/hostnet"
	"goisekai/internal/pluginmanager"
	"goisekai/internal/templates"
)

// challengeTestServer is testServerFullDB with a real plugins dir, so a fixture
// plugin declaring needs_human_verify is discoverable. The stock harness points
// the manager at an empty dir, which can never reach the human-verify gate.
func challengeTestServer(t *testing.T, pluginsDir string) (*Server, *database.DB) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	proxy := hostnet.NewProxy()
	pmgr := pluginmanager.NewManager(proxy, pluginsDir)
	if err := pmgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	svc := bridge.NewAppService(db, pmgr, proxy, "", t.TempDir(), nil)
	engine, err := templates.New(os.DirFS("../templates"), false)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	s := &Server{
		Router:    chi.NewRouter(),
		logger:    slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
		service:   svc,
		engine:    engine,
		csrfToken: mintTestCSRF(t),
	}
	s.registerStaticRoutes()
	s.registerViewRoutes()
	return s, db
}

// writeChallengePlugin drops a Lua plugin fixture into a temp plugins dir. The
// plugin declares needs_human_verify and fetches from fetchURL, which the test
// points at a server answering with a Cloudflare-style challenge.
func writeChallengePlugin(t *testing.T, fetchURL string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "blocked"), 0o755); err != nil {
		t.Fatal(err)
	}
	src := `PLUGIN = { contract_version = 1, name = "Blocked", needs_human_verify = true, verify_url = "https://example.com/verify" }

function search_manga(a) return "[]" end

function get_manga_detail(a)
  local resp = host.http.get("` + fetchURL + `/manga")
  return host.json.encode({ id = a, title = "live title" })
end

function get_chapter_list(a) return "[]" end
function get_page_list(a) return "[]" end
`
	if err := os.WriteFile(filepath.Join(dir, "blocked", "main.lua"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// challengeBody is the anti-bot interstitial the proxy recognises as a challenge
// (403 + a known marker in the first 8KiB).
const challengeBody = `<!DOCTYPE html><html><head><title>Just a moment...</title>
<script src="/cdn-cgi/challenge-platform/h/b/orchestrate/chl_page/v1"></script></head>
<body><h1>Checking your browser</h1></body></html>`

// TestMangaDetailBlockedServesPersistedCopy pins the offline-first fallback on
// the detail view: a plugin blocked by human verification must not blank out a
// manga the library already holds. Both block paths converge on it — no cookies
// saved yet (the gate skips the fetch), and cookies that went stale mid-session
// (the fetch returns ChallengeError). The wizard must still render over the
// served data, so a user pasting cookies sees the page, not an empty shell.
func TestMangaDetailBlockedServesPersistedCopy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(challengeBody))
	}))
	defer srv.Close()

	s, db := challengeTestServer(t, writeChallengePlugin(t, srv.URL))
	rowID := seedManga(t, db, "blocked", "m1", "Persisted Title")
	seedChapters(t, db, rowID,
		database.Chapter{SourceChapterID: "m1:c1", ChapterNum: 1},
		database.Chapter{SourceChapterID: "m1:c2", ChapterNum: 2},
	)

	cases := []struct {
		name string
		// seedVerify populates the plugin's saved verification state.
		seedVerify bool
	}{
		// Gate path: NeedsHumanVerify is set and no cookies exist, so the fetch
		// is skipped entirely and challenge is raised from metadata alone.
		{name: "no cookies saved", seedVerify: false},
		// Stale-cookie path: cookies exist so the fetch runs, and the upstream
		// answers with a challenge that surfaces as ChallengeError.
		{name: "cookies went stale", seedVerify: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.seedVerify {
				if err := db.UpsertPluginVerify(database.PluginVerifyRow{
					PluginID:  "blocked",
					VerifyURL: "https://example.com/verify",
					Cookies:   "cf_clearance=stale",
					UserAgent: "test-agent",
				}); err != nil {
					t.Fatalf("upsert verify: %v", err)
				}
			}

			req := httptest.NewRequest("GET", "/view/manga/blocked/m1", nil)
			rec := httptest.NewRecorder()
			s.Router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
			}
			body := rec.Body.String()

			// The persisted copy must be served, not blanked out.
			if !strings.Contains(body, "Persisted Title") {
				t.Errorf("persisted title missing — detail view blanked out while blocked")
			}
			// Each persisted chapter must link into the reader. The +1 Continue
			// button also points at a chapter, so assert on the IDs themselves
			// rather than a raw link count.
			for _, id := range []string{"m1:c1", "m1:c2"} {
				if !strings.Contains(body, "view/read/blocked/m1/"+id) {
					t.Errorf("chapter link %s missing", id)
				}
			}
			if !strings.Contains(body, "Showing cached data") {
				t.Errorf("cached-data notice missing")
			}
			// The wizard must stay visible over the served data.
			if !strings.Contains(body, "This site needs human verification") {
				t.Errorf("verification banner missing")
			}
			if !strings.Contains(body, "verify-modal") {
				t.Errorf("verify modal missing")
			}
		})
	}
}

// TestMangaDetailBlockedNoPersistedCopy keeps the degraded path honest: when a
// blocked plugin has nothing stored, the wizard still renders rather than the
// handler returning nil and the user getting a blank page with no explanation.
func TestMangaDetailBlockedNoPersistedCopy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(challengeBody))
	}))
	defer srv.Close()

	s, _ := challengeTestServer(t, writeChallengePlugin(t, srv.URL))

	req := httptest.NewRequest("GET", "/view/manga/blocked/unknown", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "This site needs human verification") {
		t.Errorf("verification banner missing")
	}
	if !strings.Contains(body, "verify-modal") {
		t.Errorf("verify modal missing")
	}
}
