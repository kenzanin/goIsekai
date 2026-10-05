package httpserver

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
)

// csrfTokenLabel is the fixed message the token is an HMAC over. It exists so
// the derived token is bound to this protocol version rather than being an
// arbitrary hex string.
const csrfTokenLabel = "goisekai-csrf-v1"

// csrfHeader and csrfField are the channels a caller may present the token on,
// in the order the middleware checks them.
const (
	csrfHeader = "X-CSRF-Token"
	csrfField  = "csrf_token"
)

// csrfTokenLen is the byte length of the HMAC-SHA256 the token is derived from;
// mintCSRFToken hex-encodes it, so the wire form is twice this.
const csrfTokenLen = 32

// csrfStaleHeader marks a rejection the client can recover from by reloading.
// It is set only when the caller presented a well-formed token that simply does
// not belong to this process, which in practice means the page was rendered
// before a restart. There is no htmx in this app, so the header is read by our
// own fetch wrapper rather than by a library.
const csrfStaleHeader = "X-GoIsekai-Reload"

// mintCSRFToken derives a token from a freshly generated secret. It is called
// once per process, at startup: the secret never leaves the server, so a token
// captured from one run is useless against the next. Deriving the value once
// (rather than per request) is what keeps it stable across page loads, which is
// what lets a form rendered earlier still validate.
func mintCSRFToken() (string, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(csrfTokenLabel))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

// csrfCandidate returns the token the caller presented, or "" if none. Header
// first, then the form field, then the query field: a plain form post carries
// the hidden field, and the query field covers the minority of callers that
// navigate rather than posting in place.
func csrfCandidate(r *http.Request) string {
	if v := r.Header.Get(csrfHeader); v != "" {
		return v
	}
	if v := r.PostFormValue(csrfField); v != "" {
		return v
	}
	return r.URL.Query().Get(csrfField)
}

// csrfTokenMatches compares in constant time so a caller cannot learn the token
// by timing repeated attempts.
func csrfTokenMatches(got, want string) bool {
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// looksLikeToken reports whether a candidate has the shape of a minted token.
// It exists to tell "the page is stale" apart from "this request never had a
// token": the first is recoverable with a reload, the second is a real refusal
// and reloading would only hide it.
func looksLikeToken(candidate string) bool {
	if len(candidate) != csrfTokenLen*2 {
		return false
	}
	for _, r := range candidate {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

// unsafeMethods are the methods requireCSRFToken guards. A GET is treated as
// safe by default, so a future mutating GET has to be added here deliberately
// rather than slipping through.
var unsafeMethods = map[string]bool{
	http.MethodPost:   true,
	http.MethodPut:    true,
	http.MethodPatch:  true,
	http.MethodDelete: true,
}

// isUnsafeMethod reports whether a request method needs a CSRF token.
func isUnsafeMethod(method string) bool {
	return unsafeMethods[method]
}
