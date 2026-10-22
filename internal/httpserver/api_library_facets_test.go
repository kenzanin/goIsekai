package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

// TestAPILibraryFacetsShape pins the payload the desktop client reads. Every
// key must be present even when the library is empty, because QML indexes into
// them without a null check.
func TestAPILibraryFacetsShape(t *testing.T) {
	s, db := testServerFullDB(t, "", false)
	mangaID := seedManga(t, db, "p1", "m1", "Title One")
	if err := db.AddCategory(strconv.FormatInt(mangaID, 10), "Action"); err != nil {
		t.Fatalf("add category: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/library/facets", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, k := range []string{"overview", "plugins", "categories", "tags", "duplicate_count"} {
		if _, ok := body[k]; !ok {
			t.Fatalf("missing %q in %v", k, body)
		}
	}
	tags, ok := body["tags"].(map[string]any)
	if !ok {
		t.Fatalf("tags is %T, want object", body["tags"])
	}
	got, ok := tags["p1:m1"].([]any)
	if !ok || len(got) == 0 {
		t.Fatalf("tags not keyed plugin_id:source_manga_id: %#v", tags)
	}
	if got[0] != "Action" {
		t.Fatalf("tag = %v, want Action", got[0])
	}
}

// TestAPILibraryItemsCarrySortFields covers the three fields the client needs
// to offer the web sidebar's sorts without a second request per manga.
func TestAPILibraryItemsCarrySortFields(t *testing.T) {
	s, db := testServerFullDB(t, "", false)
	seedManga(t, db, "p1", "m1", "Title One")

	req := httptest.NewRequest("GET", "/api/library", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var items []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	for _, k := range []string{"updated_at", "last_read_at", "dup_group"} {
		if _, ok := items[0][k]; !ok {
			t.Fatalf("missing %q in %v", k, items[0])
		}
	}
	// -1 means "not a duplicate", which the client tests against; a zero
	// default would claim every manga is in group 0.
	if got := items[0]["dup_group"]; got != float64(-1) {
		t.Fatalf("dup_group = %v, want -1", got)
	}
}

// TestAPILibraryFacetsEmptyLibrary makes sure the shapes still marshal when
// there is nothing to report — nil slices would become JSON null and break
// QML's .length.
func TestAPILibraryFacetsEmptyLibrary(t *testing.T) {
	s, _ := testServerFullDB(t, "", false)

	req := httptest.NewRequest("GET", "/api/library/facets", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Plugins    []any `json:"plugins"`
		Categories []any `json:"categories"`
		Tags       any   `json:"tags"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Plugins == nil || body.Categories == nil {
		t.Fatalf("plugins/categories marshalled as null: %s", rec.Body.String())
	}
	if body.Tags == nil {
		t.Fatalf("tags marshalled as null: %s", rec.Body.String())
	}
}
