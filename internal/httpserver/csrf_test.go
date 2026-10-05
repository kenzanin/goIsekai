package httpserver

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"goisekai/internal/database"
)

const sha256Size = 32

// Task 2.1: minting produces a full-length, unguessable token.
func TestMintCSRFTokenIsFullLength(t *testing.T) {
	tok, err := mintCSRFToken()
	if err != nil {
		t.Fatalf("mintCSRFToken: %v", err)
	}
	if tok == "" {
		t.Fatal("token is empty")
	}
	if len(tok) != 2*sha256Size {
		t.Errorf("token is %d hex chars, want %d (SHA-256)", len(tok), 2*sha256Size)
	}
	if strings.ContainsAny(tok, "ghijklmnopqrstuvwxyz") {
		t.Error("token is not lowercase hex")
	}
}

// Task 2.1: a token captured from one run must not work against the next, so
// every mint differs.
func TestMintCSRFTokenDiffersPerCall(t *testing.T) {
	seen := map[string]bool{}
	for range 20 {
		tok, err := mintCSRFToken()
		if err != nil {
			t.Fatalf("mintCSRFToken: %v", err)
		}
		if seen[tok] {
			t.Fatal("two mints produced the same token; the secret is not randomised")
		}
		seen[tok] = true
	}
}

// Task 2.1: the token a Server hands out must not change between requests —
// otherwise every form already sitting in a browser would stop validating.
func TestServerCSRFTokenIsStable(t *testing.T) {
	s := testServer(t, "")
	if s.csrfToken == "" {
		t.Fatal("server has no csrf token")
	}
	first := s.csrfToken
	for range 5 {
		if s.csrfToken != first {
			t.Fatal("server token changed between reads")
		}
	}
}

// Task 3.1: all three presentation channels are accepted, in priority order.
func TestCSRFCandidateChannels(t *testing.T) {
	for _, tc := range []struct {
		name  string
		build func() *http.Request
		want  string
	}{
		{"header", func() *http.Request {
			r := httptest.NewRequest(http.MethodPost, "/action/x", nil)
			r.Header.Set(csrfHeader, "from-header")
			return r
		}, "from-header"},
		{"form field", func() *http.Request {
			r := httptest.NewRequest(http.MethodPost, "/action/x", strings.NewReader(csrfField+"=from-form"))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			return r
		}, "from-form"},
		{"query field", func() *http.Request {
			return httptest.NewRequest(http.MethodPost, "/action/x?"+csrfField+"=from-query", nil)
		}, "from-query"},
		{"header wins over field", func() *http.Request {
			r := httptest.NewRequest(http.MethodPost, "/action/x?"+csrfField+"=from-query", nil)
			r.Header.Set(csrfHeader, "from-header")
			return r
		}, "from-header"},
		{"none", func() *http.Request {
			return httptest.NewRequest(http.MethodPost, "/action/x", nil)
		}, ""},
	} {
		if got := csrfCandidate(tc.build()); got != tc.want {
			t.Errorf("%s: candidate = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestCSRFMatchesRejectsWrongLength(t *testing.T) {
	if !csrfTokenMatches("abc", "abc") {
		t.Error("identical tokens did not match")
	}
	if csrfTokenMatches("abc", "abcd") {
		t.Error("a prefix matched")
	}
	if csrfTokenMatches("", "abc") {
		t.Error("empty token matched")
	}
}

func TestUnsafeMethodSet(t *testing.T) {
	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		if !isUnsafeMethod(m) {
			t.Errorf("%s should be guarded", m)
		}
	}
	for _, m := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		if isUnsafeMethod(m) {
			t.Errorf("%s should not be guarded", m)
		}
	}
}

// mintTestCSRF gives a hand-built test Server the token New would have minted,
// so tests exercise the same guarded code path as production.
func mintTestCSRF(t *testing.T) string {
	t.Helper()
	tok, err := mintCSRFToken()
	if err != nil {
		t.Fatalf("mintCSRFToken: %v", err)
	}
	return tok
}

// Task 2.2 + 2.3: the rendered page carries the token in a meta tag, so the
// frontend can read it without parsing the document.
func TestRenderedPageCarriesCSRFToken(t *testing.T) {
	s := testServerFull(t, "", true)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/view/library", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	want := `<meta name="csrf-token" content="` + s.csrfToken + `">`
	if !strings.Contains(body, want) {
		t.Errorf("rendered page does not carry %s\n\ngot head:\n%s", want, firstLines(body, 20))
	}
	// Task 2.4: the library page's sync form must carry the same value as a
	// hidden field, or a plain browser post would be refused.
	hidden := `<input type="hidden" name="csrf_token" value="` + s.csrfToken + `">`
	if !strings.Contains(body, hidden) {
		t.Errorf("rendered page has no %s\n\ngot body:\n%s", hidden, firstLines(body, 400))
	}
}

// A partial must not re-emit the tag: the SPA drops partial HTML into a live
// <main> that already has it.
func TestPartialRenderDoesNotDuplicateCSRFMeta(t *testing.T) {
	s := testServerFull(t, "", true)
	req := httptest.NewRequest(http.MethodGet, "/view/library", nil)
	req.Header.Set("X-Partial", "true")
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if strings.Contains(rec.Body.String(), `name="csrf-token"`) {
		t.Error("partial render repeated the csrf-token meta tag")
	}
}

func firstLines(s string, n int) string {
	lines := strings.SplitN(s, "\n", n+1)
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}

// guardedServer builds a server with the real route table so the guard is
// exercised through routing, not by calling the middleware directly.
func guardedServer(t *testing.T) *Server {
	t.Helper()
	s := testServerFull(t, "", true)
	return s
}

// Task 3.1 + 3.2: a mutation without a token is refused with a client error and
// a body that names the endpoint and the reason.
func TestActionRefusesMissingToken(t *testing.T) {
	s := guardedServer(t)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/action/sync", nil))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "/action/sync") {
		t.Errorf("body does not name the endpoint: %q", body)
	}
	if !strings.Contains(body, "CSRF") {
		t.Errorf("body does not say the token was rejected: %q", body)
	}
	if strings.TrimSpace(body) == http.StatusText(http.StatusForbidden) {
		t.Error("body is a bare status line")
	}
}

// Task 3.1: a wrong token is refused too — the check is not just presence.
func TestActionRefusesWrongToken(t *testing.T) {
	s := guardedServer(t)
	req := httptest.NewRequest(http.MethodPost, "/action/sync", nil)
	req.Header.Set(csrfHeader, strings.Repeat("a", 64))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 with a wrong token", rec.Code)
	}
}

// Task 3.1: the correct token on any channel lets the request through to the
// handler, which then answers with its own redirect.
func TestActionAcceptsTokenOnEveryChannel(t *testing.T) {
	for _, tc := range []struct {
		name  string
		build func(s *Server) *http.Request
	}{
		{"header", func(s *Server) *http.Request {
			r := httptest.NewRequest(http.MethodPost, "/action/sync", nil)
			r.Header.Set(csrfHeader, s.csrfToken)
			return r
		}},
		{"form field", func(s *Server) *http.Request {
			r := httptest.NewRequest(http.MethodPost, "/action/sync",
				strings.NewReader(csrfField+"="+s.csrfToken))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			return r
		}},
		{"query field", func(s *Server) *http.Request {
			return httptest.NewRequest(http.MethodPost, "/action/sync?"+csrfField+"="+s.csrfToken, nil)
		}},
	} {
		s := guardedServer(t)
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, tc.build(s))
		if rec.Code == http.StatusForbidden {
			t.Errorf("%s: refused a request carrying the correct token", tc.name)
		}
	}
}

// Task 3.1 + 3.4: the guard must cover every action route and nothing else.
//
// The middleware runs before routing within the /action sub-router, so a POST to
// any path under /action is refused without a token — including one that does
// not exist. That is the intended shape: the guard owns the prefix. What must
// not happen is a refusal leaking outside /action.
func TestAllActionRoutesAreGuarded(t *testing.T) {
	s := guardedServer(t)

	// Every registered /action route must be reachable through the guarded
	// sub-router, or the guard is not on the path that serves it.
	actionRoutes := 0
	if err := chi.Walk(s.Router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if strings.HasPrefix(route, "/action/") {
			actionRoutes++
		}
		return nil
	}); err != nil {
		t.Fatalf("chi.Walk: %v", err)
	}
	if actionRoutes == 0 {
		t.Fatal("no /action routes registered; the probe is broken")
	}
	if actionRoutes < 30 {
		t.Errorf("only %d action routes found, want the full set (32)", actionRoutes)
	}

	// Nothing outside /action may be refused for a missing token.
	for _, route := range []string{"/view/library", "/view/settings", "/api/plugins", "/api/logs"} {
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, route, nil))
		if rec.Code == http.StatusForbidden {
			t.Errorf("POST %s returned 403; the guard leaks outside /action", route)
		}
	}

	// And a real action route must still refuse one.
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/action/sync", nil))
	if rec.Code != http.StatusForbidden {
		t.Errorf("POST /action/sync without a token = %d, want 403", rec.Code)
	}
}

// Task 3.4: view, static and API routes stay reachable without a token.
func TestNonActionRoutesUnaffected(t *testing.T) {
	s := guardedServer(t)
	for _, route := range []string{"/view/library", "/view/settings", "/api/plugins"} {
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, route, nil))
		if rec.Code == http.StatusForbidden {
			t.Errorf("GET %s returned 403; the guard leaked outside /action", route)
		}
	}
}

// Task 3.5: a correctly-tokenized request must reach the handler and mutate,
// exactly as before the guard existed. Asserted against the database rather
// than a status code, so "the guard let it through" is not mistaken for "the
// handler did its job".
func TestTokenizedActionsStillMutate(t *testing.T) {
	t.Run("mark-read", func(t *testing.T) {
		s, db := testServerFullDB(t, "", true)
		mangaID := seedManga(t, db, "p1", "m1", "Title")
		seedChapters(t, db, mangaID, database.Chapter{
			SourceChapterID: "c1",
			ChapterNum:      1,
			TotalPages:      10,
		})

		before := progressFor(t, db, mangaID, "c1")
		if before.IsRead {
			t.Fatal("chapter starts already read; the test would prove nothing")
		}

		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, csrfPost(s, "/action/mark-read/p1/m1/c1", nil))
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("mark-read status = %d, want 303\n%s", rec.Code, rec.Body)
		}

		after := progressFor(t, db, mangaID, "c1")
		if !after.IsRead {
			t.Error("chapter is still unread; the tokenized request did not mutate")
		}
	})

	t.Run("save-settings reaches the handler", func(t *testing.T) {
		s := testServerFull(t, "", true)
		body := strings.NewReader(url.Values{
			"title":        {"Renamed Reader"},
			"width":        {"1280"},
			"height":       {"800"},
			"log_level":    {"info"},
			"image_format": {"avif"},
		}.Encode())
		req := csrfPost(s, "/action/save-settings", body)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)
		// The test service has no config path on disk, so the handler reports
		// that with a 500. What matters here is that it is not a 403: the guard
		// passed the request through and the handler ran.
		if rec.Code == http.StatusForbidden {
			t.Fatalf("save-settings was refused by the CSRF guard")
		}
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("save-settings status = %d, want the handler's own error", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "no config path set") {
			t.Errorf("body = %q, want the handler's own message", rec.Body.String())
		}
	})
}

// progressFor reads one chapter's read state from the database.
func progressFor(t *testing.T, db *database.DB, mangaRowID int64, sourceChapterID string) database.ChapterProgress {
	t.Helper()
	all, err := db.GetChapterProgressForManga(mangaRowID)
	if err != nil {
		t.Fatalf("GetChapterProgressForManga: %v", err)
	}
	for _, p := range all {
		if p.SourceChapterID == sourceChapterID {
			return p
		}
	}
	t.Fatalf("chapter %q not found for manga row %d", sourceChapterID, mangaRowID)
	return database.ChapterProgress{}
}

// Task 4.1 + 4.2: every state-changing fetch must send the token. A bundle that
// reads the meta tag but forgets one call site would fail only in the browser,
// so pin the header on each site here.
func TestFrontendSendsCSRFHeaderOnEveryActionFetch(t *testing.T) {
	checks := []struct {
		file string
		want int
		what string
	}{
		{"alpine-components.js", 3, "action form intercepts and job submit"},
		{"reader.js", 1, "chapter progress write"},
	}
	for _, c := range checks {
		data, err := os.ReadFile(filepath.Join(frontendLibDir, c.file))
		if err != nil {
			t.Fatalf("read %s: %v", c.file, err)
		}
		src := string(data)
		if !strings.Contains(src, `meta[name="csrf-token"]`) {
			t.Errorf("%s never reads the csrf-token meta tag", c.file)
		}
		if got := strings.Count(src, "'X-CSRF-Token': csrfToken"); got < c.want {
			t.Errorf("%s has %d X-CSRF-Token header sites, want at least %d (%s)",
				c.file, got, c.want, c.what)
		}
	}

	// reader.js builds its progress body without a form, so the header is its
	// only channel — it must not be relying on a hidden field.
	reader, err := os.ReadFile(filepath.Join(frontendLibDir, "reader.js"))
	if err != nil {
		t.Fatal(err)
	}
	postSite := "'X-CSRF-Token': csrfToken"
	if !strings.Contains(string(reader), postSite) {
		t.Error("reader.js has no CSRF header on its progress write")
	}
}

// Every layout must publish the token, not just the default one. The reader uses
// layouts/blank and reader.js posts chapter progress with no form to carry a
// hidden field, so a missing meta tag there silently 403s every progress write
// while the default pages keep working.
func TestEveryLayoutPublishesCSRFToken(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join("..", "templates", "layouts"))
	if err != nil {
		t.Fatalf("read layouts dir: %v", err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".lua") {
			names = append(names, e.Name())
		}
	}
	if len(names) == 0 {
		t.Fatal("no layouts found")
	}
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join("..", "templates", "layouts", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if !strings.Contains(string(data), `name="csrf-token"`) {
			t.Errorf("layouts/%s does not emit the csrf-token meta tag; any view using "+
				"it cannot authenticate its script-driven posts", name)
		}
	}
}

// A view that opts out of the default layout still has to ship the token.
func TestReaderPagePublishesCSRFToken(t *testing.T) {
	s := testServerFull(t, "", true)
	req := httptest.NewRequest(http.MethodGet, "/view/read/p/m/c", nil)
	rec := httptest.NewRecorder()
	s.renderPage(rec, req, "views/reader", "", map[string]any{
		"_layout": "blank",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	want := `<meta name="csrf-token" content="` + s.csrfToken + `">`
	if !strings.Contains(body, want) {
		t.Errorf("reader page does not carry %s\n\ngot head:\n%s", want, firstLines(body, 25))
	}
}

// A helper call that lands inside a Lua long-bracket string is not a syntax
// error — it compiles fine and silently ships the call's own source text into
// the HTML. That is exactly what happened in views/settings.lua, where the forms
// live inside one [[...]] block: the hidden field never rendered, so the settings
// POST carried no token and was refused. luacheck cannot see it (the text is a
// string), and a test that only renders /view/library misses it.
//
// So assert on the rendered output of every view: the literal must never appear.
func TestNoTemplateLeaksTheCSRFHelperSource(t *testing.T) {
	views := []struct {
		route string
		view  string
	}{
		{"/view/library", "views/library"},
		{"/view/settings", "views/settings"},
		{"/view/logs", "views/logs"},
		{"/view/plugins", "views/plugins"},
		{"/view/about", "views/about"},
		{"/view/updates", "views/updates"},
		{"/view/history", "views/history"},
	}
	s := testServerFull(t, "", true)
	for _, v := range views {
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, v.route, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s: status = %d, want 200", v.route, rec.Code)
			continue
		}
		if body := rec.Body.String(); strings.Contains(body, "csrfInput") {
			t.Errorf("GET %s renders the csrfInput call as literal text; a form in %s "+
				"has no hidden token and every submit from it will be refused",
				v.route, v.view)
		}
	}
}

// The settings page posts with a native form navigation (Sec-Fetch-Mode:
// navigate), so the hidden field is the only thing carrying the token there.
func TestSettingsFormsCarryHiddenToken(t *testing.T) {
	s := testServerFull(t, "", true)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/view/settings", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	hidden := `<input type="hidden" name="csrf_token" value="` + s.csrfToken + `">`

	for _, action := range []string{"/action/save-settings", "/action/clear-cache-all"} {
		i := strings.Index(body, `action="`+action+`"`)
		if i < 0 {
			t.Errorf("%s form not found on the settings page", action)
			continue
		}
		rest := body[i:]
		if end := strings.Index(rest, "</form>"); end >= 0 {
			rest = rest[:end]
		}
		if !strings.Contains(rest, hidden) {
			t.Errorf("the %s form has no hidden csrf_token input", action)
		}
	}
}
