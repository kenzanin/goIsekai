package database

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
