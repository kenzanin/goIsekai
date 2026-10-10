package database

import (
	"fmt"
)

// AltTitleRow is one stored alternative title for a manga.
type AltTitleRow struct {
	Title  string
	Source string
}

// AddAltTitles inserts each title via INSERT OR IGNORE (the UNIQUE
// (manga_row_id, title) constraint dedups) and returns how many rows were
// actually inserted. The row is re-indexed so new titles become searchable.
func (d *DB) AddAltTitles(mangaRowID string, titles []string, source string) (int, error) {
	inserted := 0
	for _, t := range titles {
		res, err := d.db.Exec(`INSERT OR IGNORE INTO alt_titles (manga_row_id, title, source) VALUES (?, ?, ?)`, mangaRowID, t, source)
		if err != nil {
			return inserted, err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return inserted, err
		}
		inserted += int(n)
	}
	if inserted == 0 {
		return 0, nil
	}
	return inserted, d.SyncFTS(mangaRowID)
}

// RemoveAltTitle deletes a single alternative title from a manga and
// re-indexes the row so the dropped title stops matching.
func (d *DB) RemoveAltTitle(mangaRowID, title string) error {
	if _, err := d.db.Exec(`DELETE FROM alt_titles WHERE manga_row_id = ? AND title = ?`, mangaRowID, title); err != nil {
		return err
	}
	return d.SyncFTS(mangaRowID)
}

// ListAltTitles returns a manga's alternative titles ordered by title.
func (d *DB) ListAltTitles(mangaRowID string) ([]AltTitleRow, error) {
	rows, err := d.db.Query(`SELECT title, source FROM alt_titles WHERE manga_row_id = ? ORDER BY title`, mangaRowID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []AltTitleRow
	for rows.Next() {
		var r AltTitleRow
		if err := rows.Scan(&r.Title, &r.Source); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// SwapMainTitle promotes newTitle to be the manga's main title: the old main
// title is demoted into alt_titles (source 'user'), any alt_titles row equal
// to newTitle is dropped, and the library_fts row is rebuilt. All changes are
// atomic.
func (d *DB) SwapMainTitle(pluginID, sourceMangaID, newTitle string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var rowID, oldTitle string
	if err := tx.QueryRow(`SELECT id, title FROM mangas WHERE plugin_id = ? AND source_manga_id = ?`, pluginID, sourceMangaID).Scan(&rowID, &oldTitle); err != nil {
		return fmt.Errorf("resolve manga: %w", err)
	}
	if oldTitle == newTitle {
		return tx.Commit()
	}

	if _, err := tx.Exec(`DELETE FROM alt_titles WHERE manga_row_id = ? AND title = ?`, rowID, newTitle); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT OR IGNORE INTO alt_titles (manga_row_id, title, source) VALUES (?, ?, ?)`, rowID, oldTitle, pluginID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE mangas SET title = ?, custom_title = 1 WHERE id = ?`, newTitle, rowID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE library_fts SET title = ?, alt = COALESCE((SELECT group_concat(title, ' ') FROM alt_titles WHERE manga_row_id = ?), '') WHERE CAST(manga_row_id AS TEXT) = ?`, newTitle, rowID, rowID); err != nil {
		return err
	}
	return tx.Commit()
}
