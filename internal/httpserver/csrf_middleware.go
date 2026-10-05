package httpserver

import (
	"fmt"
	"net/http"
)

// requireCSRFToken refuses a state-changing request that does not carry this
// process's CSRF token. It is mounted on the /action group only, so view,
// static, image and API routes are untouched and no GET path can be affected.
//
// A caller presents the token on the X-CSRF-Token header (script-driven posts),
// the csrf_token form field (plain form posts), or the csrf_token query field
// (the few callers that navigate rather than post in place).
func (s *Server) requireCSRFToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isUnsafeMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}
		// An unminted token means the server failed to generate one at startup;
		// refusing is the only safe answer, and saying so beats a confusing 403.
		if s.csrfToken == "" {
			s.logger.Error("csrf: no token was minted; refusing a state-changing request",
				"method", r.Method, "path", r.URL.Path)
			http.Error(w, csrfRejection(r, "the server could not mint a CSRF token"), http.StatusForbidden)
			return
		}
		if csrfTokenMatches(csrfCandidate(r), s.csrfToken) {
			next.ServeHTTP(w, r)
			return
		}
		s.logger.Warn("csrf: refused a state-changing request with a missing or invalid token",
			"method", r.Method, "path", r.URL.Path,
			"origin", r.Header.Get("Origin"), "referer", r.Header.Get("Referer"))
		http.Error(w, csrfRejection(r, "the CSRF token was missing or did not match"), http.StatusForbidden)
	})
}

// csrfRejection names the endpoint and the reason. A bare status line leaves the
// caller — a human or the action layer — with nothing to act on, and the most
// likely cause of a rejection here is a page that predates a server restart.
func csrfRejection(r *http.Request, reason string) string {
	return fmt.Sprintf("goIsekai refused %s %s: %s. Reload the page and try again; "+
		"a page open across a server restart carries a token from the previous run.",
		r.Method, r.URL.Path, reason)
}
