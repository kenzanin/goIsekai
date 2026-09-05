// Command jetgen regenerates the jet v2 typed table/model code for the
// database layer from the same DDL used by internal/database/runMigrations.
//
// jet's own CLI pins mattn/go-sqlite3 (CGO); this program uses the pure-Go
// modernc.org/sqlite driver so `go run ./cmd/jetgen` works under
// CGO_ENABLED=0.
package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "modernc.org/sqlite"

	sqlitegen "github.com/go-jet/jet/v2/generator/sqlite"
)

// ddl mirrors internal/database/schema.go's migrations slice exactly so the
// generated types stay in sync with the applied schema.
var ddl = []string{
	`CREATE TABLE IF NOT EXISTS mangas (
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
    UNIQUE(plugin_id, source_manga_id)
);`,
	`CREATE TABLE IF NOT EXISTS chapters (
    id TEXT PRIMARY KEY,
    manga_id TEXT NOT NULL,
    source_chapter_id TEXT NOT NULL,
    title TEXT NOT NULL,
    chapter_num REAL NOT NULL,
    volume_num REAL,
    is_read INTEGER DEFAULT 0,
    last_page_read INTEGER DEFAULT 0,
    download_status TEXT DEFAULT 'NOT_DOWNLOADED',
    fetched_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(manga_id) REFERENCES mangas(id) ON DELETE CASCADE
);`,
	`CREATE TABLE IF NOT EXISTS read_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chapter_id TEXT NOT NULL,
    page_num INTEGER NOT NULL,
    read_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(chapter_id) REFERENCES chapters(id) ON DELETE CASCADE
);`,
	`CREATE TABLE IF NOT EXISTS plugins (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    version TEXT NOT NULL,
    wasm_path TEXT NOT NULL,
    is_active INTEGER DEFAULT 1,
    icon_url TEXT
);`,
	`ALTER TABLE chapters ADD COLUMN total_pages INTEGER DEFAULT 0;`,
	`CREATE TABLE IF NOT EXISTS plugin_verify (
    plugin_id TEXT PRIMARY KEY,
    verify_url TEXT NOT NULL DEFAULT '',
    cookies TEXT NOT NULL DEFAULT '',
    user_agent TEXT NOT NULL DEFAULT '',
    updated_at INTEGER NOT NULL DEFAULT 0
);`,
	`ALTER TABLE plugins ADD COLUMN thumb_ratio REAL DEFAULT 0;`,
	`ALTER TABLE mangas ADD COLUMN new_since TIMESTAMP;`,
	// alt_titles column was added then dropped; the replacement table is below.
	`CREATE TABLE IF NOT EXISTS alt_titles (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		manga_row_id TEXT NOT NULL REFERENCES mangas(id) ON DELETE CASCADE,
		title TEXT NOT NULL,
		source TEXT NOT NULL,
		UNIQUE(manga_row_id, title)
	);`,
	`CREATE VIRTUAL TABLE IF NOT EXISTS library_fts USING fts5(title, alt, plugin_id UNINDEXED, manga_row_id UNINDEXED);`,
	`ALTER TABLE mangas ADD COLUMN custom_title INTEGER NOT NULL DEFAULT 0;`,
}

func main() {
	tmp, err := os.CreateTemp("", "jetgen-*.db")
	if err != nil {
		fatal(err)
	}
	path := tmp.Name()
	// Ensure the temp file is removed no matter how we exit.
	defer func() { _ = os.Remove(path) }()
	_ = tmp.Close()

	db, err := sql.Open("sqlite", "file:"+path+"?_foreign_keys=1")
	if err != nil {
		fatal(err)
	}
	defer func() { _ = db.Close() }()

	for _, stmt := range ddl {
		if _, err := db.Exec(stmt); err != nil {
			fatal(fmt.Errorf("applying DDL: %w", err))
		}
	}

	if err := sqlitegen.GenerateDB(db, "internal/database/.gen"); err != nil {
		fatal(err)
	}

	fmt.Println("generated internal/database/.gen")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "jetgen:", err)
	os.Exit(1)
}