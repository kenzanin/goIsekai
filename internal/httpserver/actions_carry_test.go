package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// carryQuery must transport the caller's page state (e.g. ?ChPage=2) across a
// redirect back to the same page, or in-place actions silently reset pagination.
func TestCarryQueryPreservesRefererQueryOnSamePath(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/action/toggle-skip/x/y", nil)
	r.Header.Set("Referer", "http://127.0.0.1:8080/view/manga/x/y?ChPage=2")

	got := carryQuery(r, "/view/manga/x/y")
	if want := "/view/manga/x/y?ChPage=2"; got != want {
		t.Errorf("carryQuery = %q, want %q", got, want)
	}
}

func TestCarryQueryIgnoresDifferentPath(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/action/migrate/x/y", nil)
	r.Header.Set("Referer", "http://127.0.0.1:8080/view/manga/x/y?ChPage=2")

	if got := carryQuery(r, "/view/manga/other/z"); got != "/view/manga/other/z" {
		t.Errorf("carryQuery across paths = %q, want no query", got)
	}
}

// The redirect's own params win; the referer may only add keys that are absent.
func TestCarryQueryDoesNotOverrideRedirectParams(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/action/toggle-skip/x/y", nil)
	r.Header.Set("Referer", "http://127.0.0.1:8080/view/manga/x/y?toast=Old&ChPage=2")

	got := carryQuery(r, "/view/manga/x/y?toast=New")
	// url.Values.Encode sorts keys: ChPage < toast.
	if want := "/view/manga/x/y?ChPage=2&toast=New"; got != want {
		t.Errorf("carryQuery = %q, want %q", got, want)
	}
}

func TestCarryQueryNoRefererNoQuery(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/action/toggle-skip/x/y", nil)

	if got := carryQuery(r, "/view/manga/x/y"); got != "/view/manga/x/y" {
		t.Errorf("carryQuery without referer = %q, want bare path", got)
	}
}
