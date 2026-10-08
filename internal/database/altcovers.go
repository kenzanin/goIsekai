package database

// AltCoverRow is one alternative cover candidate for a manga, gathered from
// an enrichment source (info script) so the user can pick it as the cover.
type AltCoverRow struct {
	URL    string
	Source string
}

// AddAltCovers inserts cover candidates via INSERT OR IGNORE (the UNIQUE
// (manga_row_id, url) constraint dedups) and returns how many rows were
// actually inserted.
func (d *DB) AddAltCovers(mangaRowID string, covers []AltCoverRow) (int, error) {
	inserted := 0
	for _, c := range covers {
		if c.URL == "" {
			continue
		}
		res, err := d.db.Exec(`INSERT OR IGNORE INTO alt_covers (manga_row_id, url, source) VALUES (?, ?, ?)`, mangaRowID, c.URL, c.Source)
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

// ListAltCovers returns a manga's alternative cover candidates, newest first.
func (d *DB) ListAltCovers(mangaRowID string) ([]AltCoverRow, error) {
	rows, err := d.db.Query(`SELECT url, source FROM alt_covers WHERE manga_row_id = ? ORDER BY id`, mangaRowID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []AltCoverRow
	for rows.Next() {
		var r AltCoverRow
		if err := rows.Scan(&r.URL, &r.Source); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// RemoveAltCovers deletes all cover candidates for a manga (used by
// ResetEnrichment and maintenance).
func (d *DB) RemoveAltCovers(mangaRowID string) error {
	_, err := d.db.Exec(`DELETE FROM alt_covers WHERE manga_row_id = ?`, mangaRowID)
	return err
}
