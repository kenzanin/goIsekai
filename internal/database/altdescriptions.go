package database

import "fmt"

// AltDescriptionRow is one stored alternative description for a manga.
type AltDescriptionRow struct {
	Description string
	Source      string
}

// AddAltDescriptions inserts each description via INSERT OR IGNORE (the UNIQUE
// (manga_row_id, description) constraint dedups) and returns how many rows were
// actually inserted.
func (d *DB) AddAltDescriptions(mangaRowID string, descriptions []string, source string) (int, error) {
	inserted := 0
	for _, desc := range descriptions {
		res, err := d.db.Exec(`INSERT OR IGNORE INTO alt_descriptions (manga_row_id, description, source) VALUES (?, ?, ?)`, mangaRowID, desc, source)
		if err != nil {
			return inserted, err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return inserted, err
		}
		inserted += int(n)
	}
	return inserted, nil
}

// RemoveAltDescription deletes a single alternative description from a manga.
func (d *DB) RemoveAltDescription(mangaRowID, description string) error {
	_, err := d.db.Exec(`DELETE FROM alt_descriptions WHERE manga_row_id = ? AND description = ?`, mangaRowID, description)
	return err
}

// SwapMainDescription promotes newDesc to be the manga's main description.
// The old description is demoted into alt_descriptions (source 'plugin'),
// any alt_descriptions row equal to newDesc is dropped, and the manga's
// custom_description flag is set so UpsertManga will not overwrite it. All
// changes are atomic.
func (d *DB) SwapMainDescription(pluginID, sourceMangaID, newDesc string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var rowID string
	var oldDesc string
	if err := tx.QueryRow(`SELECT id, description FROM mangas WHERE plugin_id = ? AND source_manga_id = ?`, pluginID, sourceMangaID).Scan(&rowID, &oldDesc); err != nil {
		return fmt.Errorf("resolve manga: %w", err)
	}
	if oldDesc == newDesc {
		return tx.Commit()
	}

	// Remove the chosen description from alt_descriptions (it becomes the main).
	if _, err := tx.Exec(`DELETE FROM alt_descriptions WHERE manga_row_id = ? AND description = ?`, rowID, newDesc); err != nil {
		return err
	}
	// Demote the old main description into alt_descriptions (if non-empty and
	// not already present).
	if oldDesc != "" {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO alt_descriptions (manga_row_id, description, source) VALUES (?, ?, ?)`, rowID, oldDesc, pluginID); err != nil {
			return err
		}
	}
	// Set the new main description and lock it from plugin overwrites.
	if _, err := tx.Exec(`UPDATE mangas SET description = ?, custom_description = 1 WHERE id = ?`, newDesc, rowID); err != nil {
		return err
	}
	return tx.Commit()
}

// ListAltDescriptions returns a manga's alternative descriptions ordered by description.
func (d *DB) ListAltDescriptions(mangaRowID string) ([]AltDescriptionRow, error) {
	rows, err := d.db.Query(`SELECT description, source FROM alt_descriptions WHERE manga_row_id = ? ORDER BY description`, mangaRowID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []AltDescriptionRow
	for rows.Next() {
		var r AltDescriptionRow
		if err := rows.Scan(&r.Description, &r.Source); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
