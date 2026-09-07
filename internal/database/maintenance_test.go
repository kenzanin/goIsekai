package database

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBackupToCreatesFileAndPrunes(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	d, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()

	dir := filepath.Join(t.TempDir(), "backups")
	for i := range 7 {
		if _, err := d.BackupTo(dir, 5); err != nil {
			t.Fatalf("backup %d: %v", i, err)
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 5 {
		t.Fatalf("want 5 backups, got %d", len(entries))
	}
}

func TestPruneOrphans(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	d, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()

	// Insert manga + chapter + history, then delete manga to create orphans
	if _, err := d.db.Exec(`INSERT INTO mangas (id, plugin_id, source_manga_id, title, in_library) VALUES ('m1','p1','s1','T',0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := d.db.Exec(`INSERT INTO chapters (id, manga_id, source_chapter_id, title, chapter_num) VALUES ('c1','m1','sc1','Ch1',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := d.db.Exec(`INSERT INTO read_history (chapter_id, page_num) VALUES ('c1', 1)`); err != nil {
		t.Fatal(err)
	}
	// Delete manga -> chapters + history orphaned
	if _, err := d.db.Exec(`DELETE FROM mangas WHERE id='m1'`); err != nil {
		t.Fatal(err)
	}

	summary, err := d.PruneOrphans()
	if err != nil {
		t.Fatal(err)
	}

	var h, c int
	_ = d.db.QueryRow(`SELECT COUNT(*) FROM read_history`).Scan(&h)
	_ = d.db.QueryRow(`SELECT COUNT(*) FROM chapters`).Scan(&c)
	if h != 0 || c != 0 {
		t.Fatalf("orphans remain: history=%d chapters=%d summary=%q", h, c, summary)
	}
}
