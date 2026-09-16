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
	if _, err := d.db.Exec(`INSERT INTO mangas (plugin_id, source_manga_id, title, in_library) VALUES ('p1','s1','T',0)`); err != nil {
		t.Fatal(err)
	}
	// Get the auto-assigned manga ID.
	var mangaID int64
	if err := d.db.QueryRow(`SELECT id FROM mangas WHERE plugin_id = 'p1' AND source_manga_id = 's1'`).Scan(&mangaID); err != nil {
		t.Fatal(err)
	}
	if _, err := d.db.Exec(`INSERT INTO chapters (manga_id, source_chapter_id, title, chapter_num) VALUES (?, 'sc1','Ch1',1)`, mangaID); err != nil {
		t.Fatal(err)
	}
	// Get the auto-assigned chapter ID.
	var chapterID int64
	if err := d.db.QueryRow(`SELECT id FROM chapters WHERE source_chapter_id = 'sc1'`).Scan(&chapterID); err != nil {
		t.Fatal(err)
	}

	payload := []byte(`[{"index":0,"url":"https://img.example/1.png"},{"index":1,"url":"https://img.example/2.png"}]`)

	// Save
	if err := d.SaveChapterPages(chapterID, payload); err != nil {
		t.Fatalf("SaveChapterPages: %v", err)
	}

	// Read back
	got, err := d.GetChapterPages(chapterID)
	if err != nil {
		t.Fatalf("GetChapterPages: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("round-trip mismatch:\n  want: %s\n  got:  %s", payload, got)
	}

	// INSERT OR REPLACE: overwrite with new payload.
	payload2 := []byte(`[{"index":0,"url":"https://img.example/v2.png"}]`)
	if err := d.SaveChapterPages(chapterID, payload2); err != nil {
		t.Fatalf("SaveChapterPages overwrite: %v", err)
	}
	got2, err := d.GetChapterPages(chapterID)
	if err != nil {
		t.Fatalf("GetChapterPages after overwrite: %v", err)
	}
	if string(got2) != string(payload2) {
		t.Fatalf("overwrite mismatch:\n  want: %s\n  got:  %s", payload2, got2)
	}

	// Cache miss: nonexistent chapter returns (nil, nil).
	gotNil, errNil := d.GetChapterPages(0)
	if errNil != nil {
		t.Fatalf("expected nil error for cache miss, got: %v", errNil)
	}
	if gotNil != nil {
		t.Fatalf("expected nil bytes for cache miss, got: %s", gotNil)
	}
}
