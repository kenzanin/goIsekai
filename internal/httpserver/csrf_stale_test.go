package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Only a well-formed token from another process is recoverable by reloading. A
// missing or malformed token is a genuine refusal, and telling that client to
// reload would just hide it.
func TestReloadHintOnlyForAWellFormedForeignToken(t *testing.T) {
	wrongToken := strings.Repeat("ab", csrfTokenLen)
	cases := []struct {
		name       string
		token      string
		wantStale  bool
		wantReason string
	}{
		{"foreign well-formed token", wrongToken, true, "earlier run of the server"},
		{"no token", "", false, "missing or did not match"},
		{"malformed token", "not-a-token", false, "missing or did not match"},
		{"right length, not hex", strings.Repeat("zz", csrfTokenLen), false, "missing or did not match"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := testServerFull(t, "", true)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/action/mark-all-read",
				strings.NewReader("pluginID=p&mangaID=m"))
			if tc.token != "" {
				req.Header.Set(csrfHeader, tc.token)
			}
			s.Router.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", rec.Code)
			}
			got := rec.Header().Get(csrfStaleHeader)
			if tc.wantStale && got == "" {
				t.Errorf("%s not set; the client cannot tell this refusal is recoverable", csrfStaleHeader)
			}
			if !tc.wantStale && got != "" {
				t.Errorf("%s = %q; a genuine refusal must not trigger a reload", csrfStaleHeader, got)
			}
			if !strings.Contains(rec.Body.String(), tc.wantReason) {
				t.Errorf("body does not mention %q; got %q", tc.wantReason, rec.Body.String())
			}
		})
	}
}

// The server's own token is what a fresh page carries; presenting it must never
// be mistaken for a stale one.
func TestCurrentTokenIsNotFlaggedStale(t *testing.T) {
	s := testServerFull(t, "", true)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/action/mark-all-read",
		strings.NewReader("pluginID=p&mangaID=m"))
	req.Header.Set(csrfHeader, s.csrfToken)
	s.Router.ServeHTTP(rec, req)
	if got := rec.Header().Get(csrfStaleHeader); got != "" {
		t.Errorf("%s = %q on a request carrying the current token", csrfStaleHeader, got)
	}
}

func TestLooksLikeToken(t *testing.T) {
	valid := strings.Repeat("ab", csrfTokenLen)
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"minted shape", valid, true},
		{"empty", "", false},
		{"too short", valid[:len(valid)-2], false},
		{"too long", valid + "ab", false},
		{"not hex", strings.Repeat("zz", csrfTokenLen), false},
		{"uppercase hex", strings.Repeat("AB", csrfTokenLen), false},
		{"a real uuid shape", "d550c8b4-4a3f-4f2c-9c1e-8a5f3f6c1d2e", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := looksLikeToken(tc.in); got != tc.want {
				t.Errorf("looksLikeToken(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
