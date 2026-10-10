package database

import (
	"database/sql"
	"fmt"
	"strings"
)

// libraryFTSIndex inserts in-library manga into library_fts: the active title,
// the alternative titles joined into one field, and the keys search joins back
// on. An empty mangaRowID indexes the whole library.
const libraryFTSIndex = `INSERT INTO library_fts (title, alt, plugin_id, manga_row_id)
	SELECT m.title, COALESCE((SELECT group_concat(title, ' ') FROM alt_titles WHERE manga_row_id = m.id), ''), m.plugin_id, m.id
	FROM mangas m WHERE m.in_library = 1`

// ftsExecer is satisfied by *sql.DB and *sql.Tx, so the same statement serves
// the single-row re-index, the full rebuild and the migration.
type ftsExecer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

// indexLibraryFTS indexes every in-library manga, or just mangaRowID.
func indexLibraryFTS(x ftsExecer, mangaRowID string) error {
	if mangaRowID == "" {
		_, err := x.Exec(libraryFTSIndex)
		return err
	}
	_, err := x.Exec(libraryFTSIndex+` AND m.id = ?`, mangaRowID)
	return err
}

// SyncFTS (re)indexes a single manga row in library_fts: the row is removed
// first, then re-inserted with its alt titles when it is still in the library.
func (d *DB) SyncFTS(mangaRowID string) error {
	return SyncFTSTx(d.db, mangaRowID)
}

// SyncFTSTx is SyncFTS against an open transaction, used by source migration.
// Keys are compared as text: FTS5 stores manga_row_id without column affinity,
// so a bound integer id and its string form are different values.
func SyncFTSTx(tx ftsExecer, mangaRowID string) error {
	if _, err := tx.Exec(`DELETE FROM library_fts WHERE CAST(manga_row_id AS TEXT) = ?`, mangaRowID); err != nil {
		return err
	}
	return indexLibraryFTS(tx, mangaRowID)
}

// RebuildLibraryFTS wipes and fully re-indexes library_fts from the mangas and
// alt_titles tables. Used for maintenance and tests.
func (d *DB) RebuildLibraryFTS() error {
	if _, err := d.db.Exec(`DELETE FROM library_fts`); err != nil {
		return err
	}
	return indexLibraryFTS(d.db, "")
}

// LibraryFTSDrift reports whether library_fts disagrees with the in-library
// mangas: a missing row, a dangling or duplicate row, or stale title/alt
// content. Key comparison is text-based so integer and legacy text row keys
// both match.
func (d *DB) LibraryFTSDrift() (bool, error) {
	var drift int
	err := d.db.QueryRow(`SELECT EXISTS (
		SELECT 1 FROM mangas m
		WHERE m.in_library = 1
		  AND CAST(m.id AS TEXT) NOT IN (SELECT CAST(manga_row_id AS TEXT) FROM library_fts)
	) OR EXISTS (
		SELECT 1 FROM library_fts f
		WHERE CAST(f.manga_row_id AS TEXT) NOT IN (SELECT CAST(id AS TEXT) FROM mangas WHERE in_library = 1)
	) OR (SELECT COUNT(*) FROM mangas WHERE in_library = 1) <> (SELECT COUNT(*) FROM library_fts)
	OR EXISTS (
		SELECT 1 FROM mangas m
		WHERE m.in_library = 1
		  AND EXISTS (SELECT 1 FROM library_fts f
			WHERE CAST(f.manga_row_id AS TEXT) = CAST(m.id AS TEXT)
			  AND (f.title IS NOT m.title
			       OR f.alt IS NOT COALESCE((SELECT group_concat(title, ' ') FROM alt_titles WHERE manga_row_id = m.id), '')))
	)`).Scan(&drift)
	return drift == 1, err
}

// EnsureLibraryFTS rebuilds library_fts when LibraryFTSDrift reports
// disagreement with the library, and reports whether a rebuild ran.
func (d *DB) EnsureLibraryFTS() (bool, error) {
	drift, err := d.LibraryFTSDrift()
	if err != nil || !drift {
		return false, err
	}
	return true, d.RebuildLibraryFTS()
}

// CandidateRow is a library search hit resolved back to its manga.
type CandidateRow struct {
	MangaRowID    string
	PluginID      string
	SourceMangaID string
	Title         string
	Description   string
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
	rows, err := d.db.Query(`SELECT f.manga_row_id, f.plugin_id, m.source_manga_id, m.title, m.description
		FROM library_fts f JOIN mangas m ON m.id = f.manga_row_id
		WHERE library_fts MATCH ? ORDER BY rank`, match)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []CandidateRow
	for rows.Next() {
		var c CandidateRow
		if err := rows.Scan(&c.MangaRowID, &c.PluginID, &c.SourceMangaID, &c.Title, &c.Description); err != nil {
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
