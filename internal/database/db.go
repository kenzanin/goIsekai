package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Manga struct {
	ID            string
	PluginID      string
	SourceMangaID string
	Title         string
	CoverURL      string
	Description   string
	Status        string
	InLibrary     bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Chapter struct {
	ID              string
	MangaID         string
	SourceChapterID string
	Title           string
	ChapterNum      float64
	VolumeNum       float64
	IsRead          bool
	LastPageRead    int
	TotalPages      int
	DownloadStatus  string
	FetchedAt       time.Time
}

// ChapterProgress is the per-chapter read progress surfaced to the UI.
type ChapterProgress struct {
	SourceChapterID string
	LastPageRead    int
	TotalPages      int
	IsRead          bool // manually marked read (mark-read actions)
	Done            bool // IsRead OR fully read (LastPageRead >= TotalPages > 0)
	CachedPages     int  // page files present in the disk cache (populated by the bridge layer)
}

type Plugin struct {
	ID         string
	Name       string
	Version    string
	WasmPath   string
	IsActive   bool
	IconURL    string
	ThumbRatio float64
}

const (
	DownloadNotDownloaded = "NOT_DOWNLOADED"
	DownloadDownloading   = "DOWNLOADING"
	DownloadDownloaded    = "DOWNLOADED"
)

type DB struct{ db *sql.DB }

// Open opens the SQLite database at path, enables foreign keys so cascade
// deletes work, and applies pending migrations.
func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path+"?_foreign_keys=1&_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	// modernc.org/sqlite is pure Go; ping ensures the file is usable.
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	d := &DB{db: db}
	if err := d.runMigrations(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return d, nil
}

// altTitleJSON mirrors the {source, titles} JSON shape stored in the legacy
// mangas.alt_titles column.
type altTitleJSON struct {
	Source string   `json:"source"`
	Titles []string `json:"titles"`
}

// migrateAltTitles is the special-cased runner for altTitlesMigration.
// It creates the alt_titles table, copies JSON data out of mangas.alt_titles,
// drops the column, creates library_fts, and backfills the FTS index.
func migrateAltTitles(tx *sql.Tx) error {
	// 1. Create alt_titles table.
	if _, err := tx.Exec(`CREATE TABLE alt_titles (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		manga_row_id TEXT NOT NULL REFERENCES mangas(id) ON DELETE CASCADE,
		title TEXT NOT NULL,
		source TEXT NOT NULL,
		UNIQUE(manga_row_id, title)
	)`); err != nil {
		return fmt.Errorf("create alt_titles: %w", err)
	}

	// 2. Copy JSON rows into the new table.
	rows, err := tx.Query(`SELECT id, alt_titles FROM mangas WHERE alt_titles IS NOT NULL AND alt_titles != ''`)
	if err != nil {
		return fmt.Errorf("query alt_titles JSON: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var mangaID, payload string
		if err := rows.Scan(&mangaID, &payload); err != nil {
			return fmt.Errorf("scan alt_titles: %w", err)
		}
		var at altTitleJSON
		if err := json.Unmarshal([]byte(payload), &at); err != nil {
			continue // skip malformed JSON
		}
		for _, t := range at.Titles {
			if _, err := tx.Exec(`INSERT OR IGNORE INTO alt_titles (manga_row_id, title, source) VALUES (?, ?, ?)`, mangaID, t, at.Source); err != nil {
				return fmt.Errorf("insert alt_title: %w", err)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// 3. Drop the legacy alt_titles column from mangas.
	if _, err := tx.Exec(`ALTER TABLE mangas DROP COLUMN alt_titles`); err != nil {
		return fmt.Errorf("drop alt_titles column: %w", err)
	}

	// 4. Create the FTS5 virtual table for library search.
	if _, err := tx.Exec(`CREATE VIRTUAL TABLE library_fts USING fts5(title, alt, plugin_id UNINDEXED, manga_row_id UNINDEXED)`); err != nil {
		return fmt.Errorf("create library_fts: %w", err)
	}

	// 5. Backfill library_fts from existing in-library manga rows.
	if _, err := tx.Exec(`INSERT INTO library_fts (title, alt, plugin_id, manga_row_id) SELECT title, '', plugin_id, id FROM mangas WHERE in_library = 1`); err != nil {
		return fmt.Errorf("backfill library_fts: %w", err)
	}

	return nil
}

// Close closes the underlying database handle.
func (d *DB) Close() error { return d.db.Close() }

// runMigrations applies any not-yet-applied migrations inside a transaction,
// gating on PRAGMA user_version and bumping it after each applied statement.
func (d *DB) runMigrations() error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var v int
	if err := tx.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		return err
	}
	for i := v; i < len(migrations); i++ {
		if i == altTitlesMigration {
			if err := migrateAltTitles(tx); err != nil {
				return fmt.Errorf("applying migration %d: %w", i, err)
			}
			continue
		}
		if _, err := tx.Exec(migrations[i]); err != nil {
			return fmt.Errorf("applying migration %d: %w", i, err)
		}
	}
	if len(migrations) > v {
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", len(migrations))); err != nil {
			return err
		}
	}
	return tx.Commit()
}
