package database

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// fixtureStorageSchema is the schema as it stood just before dbStorageMigration,
// when mangas and chapters still used string keys. The test builds a database on
// it so Open() runs the storage migration over realistic data.
const fixtureStorageSchema = `
CREATE TABLE mangas (
    id TEXT PRIMARY KEY,
    plugin_id TEXT NOT NULL,
    source_manga_id TEXT NOT NULL,
    title TEXT NOT NULL,
    cover_url TEXT,
    description TEXT,
    status TEXT,
    in_library INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    custom_title INTEGER NOT NULL DEFAULT 0,
    custom_description INTEGER NOT NULL DEFAULT 0,
    genres TEXT,
    cover_dim INTEGER DEFAULT 0,
    author TEXT NOT NULL DEFAULT '',
    new_since TIMESTAMP,
    UNIQUE(plugin_id, source_manga_id)
);
CREATE TABLE chapters (
    id TEXT PRIMARY KEY,
    manga_id TEXT NOT NULL,
    source_chapter_id TEXT NOT NULL,
    title TEXT NOT NULL,
    chapter_num REAL NOT NULL,
    volume_num REAL,
    is_read INTEGER DEFAULT 0,
    is_skipped INTEGER DEFAULT 0,
    last_page_read INTEGER DEFAULT 0,
    download_status TEXT NOT NULL DEFAULT 'NOT_DOWNLOADED',
    total_pages INTEGER DEFAULT 0,
    fetched_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE chapter_pages (
    chapter_id TEXT PRIMARY KEY,
    pages TEXT NOT NULL,
    fetched_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE read_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chapter_id TEXT NOT NULL,
    page_num INTEGER NOT NULL,
    read_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
`

// TestStorageMigrationRebuildsFixtureDB migrates a database built on the old
// string-key schema and asserts which rows survive, that the metadata and
// progress columns come through intact, and that the VACUUM ending the migration
// reclaims the space freed by the dropped tables.
func TestStorageMigrationRebuildsFixtureDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.db")
	raw, err := sql.Open("sqlite", path+"?_foreign_keys=1")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	if _, err := raw.Exec(fixtureStorageSchema); err != nil {
		t.Fatalf("fixture schema: %v", err)
	}
	if _, err := raw.Exec(fmt.Sprintf("PRAGMA user_version = %d", dbStorageMigration)); err != nil {
		t.Fatalf("fixture version: %v", err)
	}

	seed := func(query string, args ...any) {
		t.Helper()
		if _, err := raw.Exec(query, args...); err != nil {
			t.Fatalf("seed %q: %v", query, err)
		}
	}

	// A library manga: keeps its metadata, its unread chapter (with pages), and
	// loses the cached pages of the chapter already marked read.
	seed(`INSERT INTO mangas (id, plugin_id, source_manga_id, title, in_library, new_since, author)
		VALUES ('p1|lib', 'p1', 'lib', 'Library title', 1, '2026-01-02 03:04:05', 'An Author')`)
	seed(`INSERT INTO chapters (id, manga_id, source_chapter_id, title, chapter_num, last_page_read, total_pages)
		VALUES ('p1|lib|keep', 'p1|lib', 'keep', 'Keep', 1, 3, 10)`)
	seed(`INSERT INTO chapters (id, manga_id, source_chapter_id, title, chapter_num, is_read)
		VALUES ('p1|lib|done', 'p1|lib', 'done', 'Done', 2, 1)`)
	seed(`INSERT INTO chapter_pages (chapter_id, pages) VALUES ('p1|lib|keep', '["k"]')`)
	seed(`INSERT INTO chapter_pages (chapter_id, pages) VALUES ('p1|lib|done', '["d"]')`)
	seed(`INSERT INTO read_history (chapter_id, page_num, read_at) VALUES ('p1|lib|keep', 1, '2026-01-01 00:00:00')`)
	seed(`INSERT INTO read_history (chapter_id, page_num, read_at) VALUES ('p1|lib|keep', 2, '2026-01-03 00:00:00')`)

	// A non-library manga: its chapters and pages are dropped, the manga row stays
	// as detail-view cache. The bulk is what the VACUUM should reclaim.
	seed(`INSERT INTO mangas (id, plugin_id, source_manga_id, title)
		VALUES ('p1|cache', 'p1', 'cache', 'Cache title')`)
	padding := strings.Repeat("x", 20_000)
	for i := range 300 {
		sourceID := fmt.Sprintf("c%d", i)
		seed(`INSERT INTO chapters (id, manga_id, source_chapter_id, title, chapter_num)
			VALUES (?, 'p1|cache', ?, ?, ?)`, fmt.Sprintf("p1|cache|%s", sourceID), sourceID, sourceID, i+1)
		seed(`INSERT INTO chapter_pages (chapter_id, pages) VALUES (?, ?)`, fmt.Sprintf("p1|cache|%s", sourceID), padding)
	}

	var bytesBefore int
	if err := raw.QueryRow(`SELECT page_count * page_size FROM pragma_page_count(), pragma_page_size()`).Scan(&bytesBefore); err != nil {
		t.Fatalf("size before: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("close fixture: %v", err)
	}

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	count := func(query string) int {
		t.Helper()
		var n int
		if err := db.db.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("count %q: %v", query, err)
		}
		return n
	}

	var version int
	if err := db.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("user_version: %v", err)
	}
	if version != len(migrations) {
		t.Errorf("user_version = %d, want %d", version, len(migrations))
	}
	if got := count(`SELECT COUNT(*) FROM mangas`); got != 2 {
		t.Errorf("mangas = %d, want 2 (library row + detail-view cache)", got)
	}
	if got := count(`SELECT COUNT(*) FROM chapters`); got != 2 {
		t.Errorf("chapters = %d, want 2 (library manga only)", got)
	}
	if got := count(`SELECT COUNT(*) FROM chapter_pages`); got != 1 {
		t.Errorf("chapter_pages = %d, want 1 (unread library chapter)", got)
	}
	if got := count(`SELECT COUNT(*) FROM read_history`); got != 1 {
		t.Errorf("read_history = %d, want 1 (newest row per chapter)", got)
	}

	var title, author, newSince string
	if err := db.db.QueryRow(
		`SELECT title, author, new_since FROM mangas WHERE plugin_id = 'p1' AND source_manga_id = 'lib'`,
	).Scan(&title, &author, &newSince); err != nil {
		t.Fatalf("library row: %v", err)
	}
	if title != "Library title" || author != "An Author" {
		t.Errorf("metadata lost: title=%q author=%q", title, author)
	}
	if !strings.HasPrefix(newSince, "2026-01-02") {
		t.Errorf("new_since lost: %q", newSince)
	}
	if got := count(`SELECT COUNT(*) FROM chapters WHERE source_chapter_id = 'keep' AND last_page_read = 3`); got != 1 {
		t.Error("chapter progress did not survive the migration")
	}

	var bytesAfter int
	if err := db.db.QueryRow(`SELECT page_count * page_size FROM pragma_page_count(), pragma_page_size()`).Scan(&bytesAfter); err != nil {
		t.Fatalf("size after: %v", err)
	}
	if bytesAfter*2 > bytesBefore {
		t.Errorf("VACUUM did not reclaim space: %d -> %d bytes", bytesBefore, bytesAfter)
	}
}
