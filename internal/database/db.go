package database

import (
	"database/sql"
	"fmt"
	"github.com/goccy/go-json"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Manga struct {
	ID            int64
	PluginID      string
	SourceMangaID string
	Title         string
	CoverURL      string
	Description   string
	Status        string
	CoverDim      int64
	InLibrary     bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Chapter struct {
	ID              int64
	MangaID         int64
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
	IsSkipped       bool // user opted to skip this chapter
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
	defer func() { _ = rows.Close() }()
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

// migrateDbStorage is the special-cased runner for dbStorageMigration.
// It migrates chapters from string keys to integer surrogate keys, purges
// non-library chapters, deduplicates read_history, and prunes chapter_pages
// for fully-read chapters. The caller VACUUMs afterwards, since that cannot run
// inside the migration transaction.
func migrateDbStorage(tx *sql.Tx) error {
	// 1. Create new mangas table with integer id.
	if _, err := tx.Exec(`CREATE TABLE mangas_new (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
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
	)`); err != nil {
		return fmt.Errorf("create mangas_new: %w", err)
	}

	// Copy mangas data.
	if _, err := tx.Exec(`INSERT INTO mangas_new (plugin_id, source_manga_id, title, cover_url, description, status, in_library, created_at, updated_at, custom_title, custom_description, genres, cover_dim, author, new_since)
		SELECT plugin_id, source_manga_id, title, cover_url, description, status, in_library, created_at, updated_at, custom_title, custom_description, genres, cover_dim, author, new_since
		FROM mangas`); err != nil {
		return fmt.Errorf("insert mangas_new: %w", err)
	}

	// 2. Create new chapters table with integer manga_id FK.
	if _, err := tx.Exec(`CREATE TABLE chapters_new (
		id INTEGER PRIMARY KEY,
		manga_id INTEGER NOT NULL,
		source_chapter_id TEXT NOT NULL,
		title TEXT NOT NULL,
		chapter_num REAL NOT NULL,
		volume_num REAL,
		is_read INTEGER DEFAULT 0,
		is_skipped INTEGER DEFAULT 0,
		last_page_read INTEGER DEFAULT 0,
		download_status TEXT DEFAULT 'NOT_DOWNLOADED',
		fetched_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		total_pages INTEGER DEFAULT 0,
		UNIQUE(manga_id, source_chapter_id),
		FOREIGN KEY(manga_id) REFERENCES mangas_new(id) ON DELETE CASCADE
	)`); err != nil {
		return fmt.Errorf("create chapters_new: %w", err)
	}

	// 3. Create new chapter_pages table with integer chapter_id FK.
	if _, err := tx.Exec(`CREATE TABLE chapter_pages_new (
		chapter_id INTEGER PRIMARY KEY,
		pages TEXT NOT NULL,
		fetched_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(chapter_id) REFERENCES chapters_new(id) ON DELETE CASCADE
	)`); err != nil {
		return fmt.Errorf("create chapter_pages_new: %w", err)
	}

	// 4. Create new read_history table with integer chapter_id FK and UNIQUE constraint.
	if _, err := tx.Exec(`CREATE TABLE read_history_new (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		chapter_id INTEGER NOT NULL,
		page_num INTEGER NOT NULL,
		read_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(chapter_id),
		FOREIGN KEY(chapter_id) REFERENCES chapters_new(id) ON DELETE CASCADE
	)`); err != nil {
		return fmt.Errorf("create read_history_new: %w", err)
	}

	// 5. Create temp table mapping old string manga IDs to new integer IDs.
	if _, err := tx.Exec(`CREATE TEMP TABLE manga_id_map AS
		SELECT m.id AS old_id, mn.id AS new_id
		FROM mangas m
		INNER JOIN mangas_new mn ON m.plugin_id = mn.plugin_id AND m.source_manga_id = mn.source_manga_id`); err != nil {
		return fmt.Errorf("create manga_id_map: %w", err)
	}

	// 6. Copy chapters, keeping only those for in_library manga.
	if _, err := tx.Exec(`INSERT INTO chapters_new (manga_id, source_chapter_id, title, chapter_num, volume_num, is_read, last_page_read, total_pages, download_status, fetched_at)
		SELECT mnm.new_id, c.source_chapter_id, c.title, c.chapter_num, c.volume_num, c.is_read, c.last_page_read, c.total_pages, c.download_status, c.fetched_at
		FROM chapters c
		INNER JOIN mangas m ON c.manga_id = m.id
		INNER JOIN manga_id_map mnm ON c.manga_id = mnm.old_id
		WHERE m.in_library = 1`); err != nil {
		return fmt.Errorf("insert chapters_new: %w", err)
	}

	// 7. Copy read_history, keeping only newest row per chapter for in_library manga.
	if _, err := tx.Exec(`INSERT INTO read_history_new (chapter_id, page_num, read_at)
		SELECT cnew.id, rh.page_num, rh.read_at
		FROM read_history rh
		INNER JOIN chapters c ON rh.chapter_id = c.id
		INNER JOIN mangas m ON c.manga_id = m.id
		INNER JOIN manga_id_map mnm ON c.manga_id = mnm.old_id
		INNER JOIN chapters_new cnew ON cnew.manga_id = mnm.new_id AND c.source_chapter_id = cnew.source_chapter_id
		WHERE m.in_library = 1
		AND rh.id = (
			SELECT rh2.id FROM read_history rh2
			INNER JOIN chapters c2 ON rh2.chapter_id = c2.id
			INNER JOIN mangas m2 ON c2.manga_id = m2.id
			WHERE c2.source_chapter_id = c.source_chapter_id AND m2.in_library = 1
			ORDER BY rh2.read_at DESC, rh2.id DESC LIMIT 1
		)`); err != nil {
		return fmt.Errorf("insert read_history_new: %w", err)
	}

	// 8. Copy chapter_pages, keeping only non-read chapters.
	if _, err := tx.Exec(`INSERT INTO chapter_pages_new (chapter_id, pages, fetched_at)
		SELECT cnew.id, cp.pages, cp.fetched_at
		FROM chapter_pages cp
		INNER JOIN chapters c ON cp.chapter_id = c.id
		INNER JOIN mangas m ON c.manga_id = m.id
		INNER JOIN manga_id_map mnm ON c.manga_id = mnm.old_id
		INNER JOIN chapters_new cnew ON cnew.manga_id = mnm.new_id AND c.source_chapter_id = cnew.source_chapter_id
		WHERE m.in_library = 1 AND c.is_read = 0`); err != nil {
		return fmt.Errorf("insert chapter_pages_new: %w", err)
	}

	// 9. Drop old tables.
	if _, err := tx.Exec(`DROP TABLE read_history`); err != nil {
		return fmt.Errorf("drop read_history: %w", err)
	}
	if _, err := tx.Exec(`DROP TABLE chapter_pages`); err != nil {
		return fmt.Errorf("drop chapter_pages: %w", err)
	}
	if _, err := tx.Exec(`DROP TABLE chapters`); err != nil {
		return fmt.Errorf("drop chapters: %w", err)
	}
	if _, err := tx.Exec(`DROP TABLE mangas`); err != nil {
		return fmt.Errorf("drop mangas: %w", err)
	}

	// 10. Rename new tables.
	if _, err := tx.Exec(`ALTER TABLE mangas_new RENAME TO mangas`); err != nil {
		return fmt.Errorf("rename mangas_new: %w", err)
	}
	if _, err := tx.Exec(`ALTER TABLE chapters_new RENAME TO chapters`); err != nil {
		return fmt.Errorf("rename chapters_new: %w", err)
	}
	if _, err := tx.Exec(`ALTER TABLE chapter_pages_new RENAME TO chapter_pages`); err != nil {
		return fmt.Errorf("rename chapter_pages_new: %w", err)
	}
	if _, err := tx.Exec(`ALTER TABLE read_history_new RENAME TO read_history`); err != nil {
		return fmt.Errorf("rename read_history_new: %w", err)
	}

	// 11. Create index on chapters(manga_id).
	if _, err := tx.Exec(`CREATE INDEX idx_chapters_manga_id ON chapters(manga_id)`); err != nil {
		return fmt.Errorf("create idx_chapters_manga_id: %w", err)
	}

	// 12. Drop temp table.
	if _, err := tx.Exec(`DROP TABLE manga_id_map`); err != nil {
		return fmt.Errorf("drop manga_id_map: %w", err)
	}

	// 13. VACUUM runs in runMigrations after this transaction commits.

	return nil
}

// Close closes the underlying database handle.
func (d *DB) Close() error { return d.db.Close() }

// columnExists reports whether the table named in an ALTER TABLE migration
// already has that column, so the migration can be replayed safely.
func columnExists(tx *sql.Tx, ddl string) bool {
	fields := strings.Fields(ddl)
	if len(fields) < 6 || !strings.EqualFold(fields[0], "ALTER") {
		return false
	}
	table, column := fields[2], fields[5]
	var cnt int
	_ = tx.QueryRow(
		fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name=?", table), column,
	).Scan(&cnt)
	return cnt > 0
}

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
		if i == skipColumnMigration || i == authorMigration {
			// Idempotent: test DBs and databases rebuilt by the storage
			// migration may already carry these columns.
			if !columnExists(tx, migrations[i]) {
				if _, err := tx.Exec(migrations[i]); err != nil {
					return fmt.Errorf("applying migration %d: %w", i, err)
				}
			}
			continue
		}
		if i == dbStorageMigration {
			if err := migrateDbStorage(tx); err != nil {
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
	if err := tx.Commit(); err != nil {
		return err
	}
	// The storage migration drops the old tables but leaves their pages in the
	// file; VACUUM reclaims them. It cannot run inside a transaction, so it
	// happens after the commit and only when the migration actually ran.
	if v <= dbStorageMigration && dbStorageMigration < len(migrations) {
		if _, err := d.db.Exec("VACUUM"); err != nil {
			return fmt.Errorf("vacuum after storage migration: %w", err)
		}
	}
	return nil
}
