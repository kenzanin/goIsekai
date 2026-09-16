package database

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/goccy/go-json"
)

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
