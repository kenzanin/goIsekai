package httpserver

import (
	"bytes"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// --- Enrichment read ---

func TestAPIEnrichmentNonExistentManga(t *testing.T) {
	s := testServer(t, "")
	req := httptest.NewRequest("GET", "/api/manga/nonexistent/manga1/enrichment", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != 404 {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// --- Fetch enrichment ---

func TestAPIFetchEnrichMissingSource(t *testing.T) {
	s := testServer(t, "")
	req := httptest.NewRequest("GET", "/api/manga/p1/m1/enrich", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// --- Set title ---

func TestAPISetTitleMissingBody(t *testing.T) {
	s := testServer(t, "")
	req := httptest.NewRequest("PUT", "/api/manga/p1/m1/title", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestAPISetTitleEmptyTitle(t *testing.T) {
	s := testServer(t, "")
	body := bytes.NewBufferString(`{"title":""}`)
	req := httptest.NewRequest("PUT", "/api/manga/p1/m1/title", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestAPISetTitleUnknownTitle(t *testing.T) {
	s := testServer(t, "")
	body := bytes.NewBufferString(`{"title":"Does Not Exist"}`)
	req := httptest.NewRequest("PUT", "/api/manga/p1/m1/title", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// --- Remove alt title ---

func TestAPIRemoveAltTitleMissingBody(t *testing.T) {
	s := testServer(t, "")
	req := httptest.NewRequest("DELETE", "/api/manga/p1/m1/alt-titles", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// --- Remove category ---

func TestAPIRemoveCategoryMissingBody(t *testing.T) {
	s := testServer(t, "")
	req := httptest.NewRequest("DELETE", "/api/manga/p1/m1/categories", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// --- Remove related ---

func TestAPIRemoveRelatedMissingBody(t *testing.T) {
	s := testServer(t, "")
	req := httptest.NewRequest("DELETE", "/api/manga/p1/m1/related", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// --- Action form ---
// Guards the detail-page form-action path: the SPA posts a URL-encoded body
// and the handler must populate the field via ParseForm/FormValue.
func TestActionSetTitleParsesFormField(t *testing.T) {
	s, db := testServerFullDB(t, "", true)
	seedManga(t, db, "p1|m1", "p1", "m1", "Main Title")
	if _, err := db.AddAltTitles("p1|m1", []string{"Some Alt Title"}, "src"); err != nil {
		t.Fatalf("add alt titles: %v", err)
	}

	body := strings.NewReader(url.Values{"title": {"Some Alt Title"}}.Encode())
	req := httptest.NewRequest("POST", "/action/set-title/p1/m1", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Some Alt Title") {
		t.Error("response should contain the set title")
	}
}
