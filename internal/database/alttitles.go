package database

import (
	"fmt"
	"strings"
)

// AltTitleRow is one stored alternative title for a manga.
type AltTitleRow struct {
	Title  string
	Source string
}

// AddAltTitles inserts each title via INSERT OR IGNORE (the UNIQUE
// (manga_row_id, title) constraint dedups) and returns how many rows were
// actually inserted.
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
	return inserted, nil
}

// RemoveAltTitle deletes a single alternative title from a manga.
func (d *DB) RemoveAltTitle(mangaRowID, title string) error {
	_, err := d.db.Exec(`DELETE FROM alt_titles WHERE manga_row_id = ? AND title = ?`, mangaRowID, title)
	return err
}

// ListAltTitles returns a manga's alternative titles ordered by title.
func (d *DB) ListAltTitles(mangaRowID string) ([]AltTitleRow, error) {
	rows, err := d.db.Query(`SELECT title, source FROM alt_titles WHERE manga_row_id = ? ORDER BY title`, mangaRowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
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
	if _, err := tx.Exec(`UPDATE library_fts SET title = ?, alt = COALESCE((SELECT group_concat(title, ' ') FROM alt_titles WHERE manga_row_id = ?), '') WHERE manga_row_id = ?`, newTitle, rowID, rowID); err != nil {
		return err
	}
	return tx.Commit()
}

// SyncFTS (re)indexes a single manga row in library_fts: the row is removed
// first, then re-inserted with its alt titles when it is still in the library.
func (d *DB) SyncFTS(mangaRowID string) error {
	if _, err := d.db.Exec(`DELETE FROM library_fts WHERE manga_row_id = ?`, mangaRowID); err != nil {
		return err
	}
	_, err := d.db.Exec(`INSERT INTO library_fts (title, alt, plugin_id, manga_row_id)
		SELECT m.title, COALESCE((SELECT group_concat(title, ' ') FROM alt_titles WHERE manga_row_id = m.id), ''), m.plugin_id, m.id
		FROM mangas m WHERE m.id = ? AND m.in_library = 1`, mangaRowID)
	return err
}

// RebuildLibraryFTS wipes and fully re-indexes library_fts from the mangas and
// alt_titles tables. Used for maintenance and tests.
func (d *DB) RebuildLibraryFTS() error {
	if _, err := d.db.Exec(`DELETE FROM library_fts`); err != nil {
		return err
	}
	_, err := d.db.Exec(`INSERT INTO library_fts (title, alt, plugin_id, manga_row_id)
		SELECT m.title, COALESCE((SELECT group_concat(title, ' ') FROM alt_titles WHERE manga_row_id = m.id), ''), m.plugin_id, m.id
		FROM mangas m WHERE m.in_library = 1`)
	return err
}

// CandidateRow is a library search hit resolved back to its manga.
type CandidateRow struct {
	MangaRowID    string
	PluginID      string
	SourceMangaID string
	Title         string
}

// SearchLibraryFTS runs a prefix-tokenized FTS5 query over title+alt and
// resolves hits to their manga rows. Query characters that could break the
// FTS5 MATCH syntax (stray quotes, unbalanced parentheses) are rejected.
func (d *DB) SearchLibraryFTS(q string) ([]CandidateRow, error) {
	match, err := buildFTSPrefixQuery(q)
	if err != nil {
		return nil, err
	}
	if match == "" {
		return nil, nil
	}
	rows, err := d.db.Query(`SELECT f.manga_row_id, f.plugin_id, m.source_manga_id, m.title
		FROM library_fts f JOIN mangas m ON m.id = f.manga_row_id
		WHERE library_fts MATCH ? ORDER BY rank`, match)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CandidateRow
	for rows.Next() {
		var c CandidateRow
		if err := rows.Scan(&c.MangaRowID, &c.PluginID, &c.SourceMangaID, &c.Title); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// buildFTSPrefixQuery turns a plain query into an FTS5 MATCH expression:
// each whitespace-separated token becomes a quoted prefix term ("tok"*).
func buildFTSPrefixQuery(q string) (string, error) {
	if strings.TrimSpace(q) == "" {
		return "", nil
	}
	if strings.Count(q, `"`)%2 == 1 {
		return "", fmt.Errorf("unbalanced quotes in search query")
	}
	if strings.Count(q, "(") != strings.Count(q, ")") {
		return "", fmt.Errorf("unbalanced parentheses in search query")
	}
	fields := strings.Fields(q)
	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		if strings.Contains(f, `"`) {
			return "", fmt.Errorf("unsupported character %q in search query", `"`)
		}
		parts = append(parts, `"`+f+`"*`)
	}
	return strings.Join(parts, " "), nil
}
