package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"goisekai/internal/version"
)

// The About page is where a user checks which build they are running, so the
// version must actually reach the rendered HTML.
func TestAboutPageShowsVersion(t *testing.T) {
	s := testServerFull(t, "", true)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/view/about", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Version") {
		t.Error("About page does not mention a version")
	}
	if !strings.Contains(body, version.String()) {
		t.Errorf("About page does not show %q", version.String())
	}
	// The badge is built with h(), so the value must not be able to inject markup.
	if !strings.Contains(body, "<span style=\"color:#818cf8;font-family:ui-monospace,monospace;\">") {
		t.Error("version badge markup is missing; the template change was reverted?")
	}
}
