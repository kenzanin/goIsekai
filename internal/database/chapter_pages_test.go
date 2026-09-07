package database

import (
	"path/filepath"
	"testing"
)

func TestChapterPagesRoundTrip(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	d, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()

	// Seed a manga + chapter so the FK is satisfied.
	if _, err := d.db.Exec(`INSERT INTO mangas (id, plugin_id, source_manga_id, title, in_library) VALUES ('m1','p1','s1','T',0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := d.db.Exec(`INSERT INTO chapters (id, manga_id, source_chapter_id, title, chapter_num) VALUES ('c1','m1','sc1','Ch1',1)`); err != nil {
		t.Fatal(err)
	}

	payload := []byte(`[{"index":0,"url":"https://img.example/1.png"},{"index":1,"url":"https://img.example/2.png"}]`)

	// Save
	if err := d.SaveChapterPages("c1", payload); err != nil {
		t.Fatalf("SaveChapterPages: %v", err)
	}

	// Read back
	got, err := d.GetChapterPages("c1")
	if err != nil {
		t.Fatalf("GetChapterPages: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("round-trip mismatch:\n  want: %s\n  got:  %s", payload, got)
	}

	// INSERT OR REPLACE: overwrite with new payload.
	payload2 := []byte(`[{"index":0,"url":"https://img.example/v2.png"}]`)
	if err := d.SaveChapterPages("c1", payload2); err != nil {
		t.Fatalf("SaveChapterPages overwrite: %v", err)
	}
	got2, err := d.GetChapterPages("c1")
	if err != nil {
		t.Fatalf("GetChapterPages after overwrite: %v", err)
	}
	if string(got2) != string(payload2) {
		t.Fatalf("overwrite mismatch:\n  want: %s\n  got:  %s", payload2, got2)
	}

	// Cache miss: nonexistent chapter returns (nil, nil).
	gotNil, errNil := d.GetChapterPages("garbage_nonexistent_id")
	if errNil != nil {
		t.Fatalf("expected nil error for cache miss, got: %v", errNil)
	}
	if gotNil != nil {
		t.Fatalf("expected nil bytes for cache miss, got: %s", gotNil)
	}
}
