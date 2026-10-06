package httpserver

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

// The export enqueue response must carry a download link. It used to carry only
// a job id, so the client waited on the socket, learned a filesystem path, and
// had nothing to fetch - the button looked inert.
func TestExportCBZEnqueueHandsBackADownloadURL(t *testing.T) {
	s := testServerFull(t, "", true)
	req := csrfPost(s, "/action/export-cbz/dummy/manga1/chapter1", strings.NewReader("title=Ch.%201"))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code >= 400 {
		t.Fatalf("enqueue failed: %d %s", rec.Code, rec.Body.String())
	}
	var ref jobRef
	if err := json.Unmarshal(rec.Body.Bytes(), &ref); err != nil {
		t.Fatalf("decode jobRef: %v (body %s)", err, rec.Body.String())
	}
	if ref.JobID == "" {
		t.Error("no job_id in the response")
	}
	if !strings.HasPrefix(ref.URL, "/exports/dummy/manga1/") {
		t.Errorf("url = %q, want an /exports/dummy/manga1/ link", ref.URL)
	}
	if !strings.HasSuffix(ref.URL, ".cbz") {
		t.Errorf("url = %q, want a .cbz link", ref.URL)
	}
}

// A download for something that is not there is a 404, and a traversal attempt
// is the same 404 - never a 500 that would distinguish "refused" from "missing".
func TestExportDownloadRoute(t *testing.T) {
	s := testServerFull(t, "", true)
	for _, name := range []string{
		"nothing-here.cbz",
		"..%2f..%2fgoisekai.ini",
		"..",
	} {
		req := httptest.NewRequest("GET", "/exports/dummy/manga1/"+name, nil)
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)
		if rec.Code != 404 {
			t.Errorf("GET %s: status %d, want 404", name, rec.Code)
		}
		if strings.Contains(rec.Body.String(), "goisekai") {
			t.Errorf("GET %s leaked file content", name)
		}
	}
}
