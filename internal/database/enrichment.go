package database

import "fmt"

// CategoryRow is one stored category for a manga.
type CategoryRow struct {
	Category string
	Source   string
}

// AddCategory adds a single category to a manga's actual categories.
// Returns nil on success; duplicate is a no-op.
func (d *DB) AddCategory(mangaRowID, category string) error {
	_, err := d.db.Exec(`INSERT OR IGNORE INTO manga_categories (manga_row_id, category, source) VALUES (?, ?, 'user')`, mangaRowID, category)
	return err
}

// AddCategories inserts each category via INSERT OR IGNORE (the UNIQUE
// (manga_row_id, category) constraint dedups) and returns how many rows were
// actually inserted.
func (d *DB) AddCategories(mangaRowID string, categories []string, source string) (int, error) {
	inserted := 0
	for _, c := range categories {
		res, err := d.db.Exec(`INSERT OR IGNORE INTO manga_categories (manga_row_id, category, source) VALUES (?, ?, ?)`, mangaRowID, c, source)
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

// ListCategories returns a manga's categories ordered by category.
func (d *DB) ListCategories(mangaRowID string) ([]CategoryRow, error) {
	rows, err := d.db.Query(`SELECT category, source FROM manga_categories WHERE manga_row_id = ? ORDER BY category`, mangaRowID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []CategoryRow
	for rows.Next() {
		var r CategoryRow
		if err := rows.Scan(&r.Category, &r.Source); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// RelatedRow is one stored related/recommended manga.
type RelatedRow struct {
	Title  string
	URL    string
	Source string
}

// AddRelated inserts each related manga via INSERT OR IGNORE (the UNIQUE
// (manga_row_id, title) constraint dedups) and returns how many rows were
// actually inserted.
func (d *DB) AddRelated(mangaRowID string, related []RelatedRow, source string) (int, error) {
	inserted := 0
	for _, r := range related {
		url := r.URL
		res, err := d.db.Exec(`INSERT OR IGNORE INTO manga_related (manga_row_id, title, url, source) VALUES (?, ?, ?, ?)`, mangaRowID, r.Title, url, source)
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

// ListRelated returns a manga's related/recommended manga ordered by title.
func (d *DB) ListRelated(mangaRowID string) ([]RelatedRow, error) {
	rows, err := d.db.Query(`SELECT title, url, source FROM manga_related WHERE manga_row_id = ? ORDER BY title`, mangaRowID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []RelatedRow
	for rows.Next() {
		var r RelatedRow
		if err := rows.Scan(&r.Title, &r.URL, &r.Source); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// RemoveCategory deletes a single category from a manga.
func (d *DB) RemoveCategory(mangaRowID, category string) error {
	_, err := d.db.Exec(`DELETE FROM manga_categories WHERE manga_row_id = ? AND category = ?`, mangaRowID, category)
	return err
}

// RemoveRelated deletes a single related manga from a manga.
func (d *DB) RemoveRelated(mangaRowID, title string) error {
	_, err := d.db.Exec(`DELETE FROM manga_related WHERE manga_row_id = ? AND title = ?`, mangaRowID, title)
	return err
}

// EnrichmentRow is a generic enrichment record stored in the DB.
type EnrichmentRow struct {
	Value  string
	URL    string
	Source string
}

// ListEnrichment returns enrichment items for a manga by kind.
func (d *DB) ListEnrichment(mangaRowID string, kind string) ([]EnrichmentRow, error) {
	switch kind {
	case "categories":
		rows, err := d.db.Query(`SELECT category, source FROM manga_categories WHERE manga_row_id = ? ORDER BY category`, mangaRowID)
		if err != nil {
			return nil, err
		}
		defer func() { _ = rows.Close() }()
		var out []EnrichmentRow
		for rows.Next() {
			var r CategoryRow
			if err := rows.Scan(&r.Category, &r.Source); err != nil {
				return nil, err
			}
			out = append(out, EnrichmentRow{Value: r.Category, Source: r.Source})
		}
		return out, rows.Err()
	case "related":
		rows, err := d.db.Query(`SELECT title, url, source FROM manga_related WHERE manga_row_id = ? ORDER BY title`, mangaRowID)
		if err != nil {
			return nil, err
		}
		defer func() { _ = rows.Close() }()
		var out []EnrichmentRow
		for rows.Next() {
			var r RelatedRow
			if err := rows.Scan(&r.Title, &r.URL, &r.Source); err != nil {
				return nil, err
			}
			out = append(out, EnrichmentRow{Value: r.Title, URL: r.URL, Source: r.Source})
		}
		return out, rows.Err()
	default:
		return nil, fmt.Errorf("enrichment: unknown kind %q", kind)
	}
}

// ResolveMangaRowID resolves a plugin_id + source_manga_id pair to its manga row ID.
func (d *DB) ResolveMangaRowID(pluginID, sourceMangaID string) (string, error) {
	var id string
	err := d.db.QueryRow(`SELECT id FROM mangas WHERE plugin_id = ? AND source_manga_id = ?`, pluginID, sourceMangaID).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("resolve manga row id: %w", err)
	}
	return id, nil
}

// ResetEnrichment deletes all enrichment data for a manga: alt titles, alt summaries,
// categories, related, and clears genre/title/synopsis overrides.
func (d *DB) ResetEnrichment(mangaRowID string) error {
	if _, err := d.db.Exec(`DELETE FROM alt_titles WHERE manga_row_id = ?`, mangaRowID); err != nil {
		return err
	}
	if _, err := d.db.Exec(`DELETE FROM alt_descriptions WHERE manga_row_id = ?`, mangaRowID); err != nil {
		return err
	}
	if _, err := d.db.Exec(`DELETE FROM manga_categories WHERE manga_row_id = ?`, mangaRowID); err != nil {
		return err
	}
	if _, err := d.db.Exec(`DELETE FROM manga_related WHERE manga_row_id = ?`, mangaRowID); err != nil {
		return err
	}
	if _, err := d.db.Exec(`DELETE FROM read_history WHERE chapter_id IN (SELECT id FROM chapters WHERE manga_id = ?)`, mangaRowID); err != nil {
		return err
	}
	// Restore title/synopsis to plugin originals.
	if _, err := d.db.Exec(`UPDATE mangas SET custom_title = 0, custom_description = 0, genres = NULL WHERE id = ?`, mangaRowID); err != nil {
		return err
	}
	return nil
}
