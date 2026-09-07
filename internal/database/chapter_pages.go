package database

import (
	"database/sql"
	"errors"
)

// SaveChapterPages persists the pre-marshaled JSON page list for a chapter.
// INSERT OR REPLACE ensures a second fetch overwrites the cached copy.
func (d *DB) SaveChapterPages(chapterID string, pages []byte) error {
	_, err := d.db.Exec(
		`INSERT OR REPLACE INTO chapter_pages (chapter_id, pages) VALUES (?, ?)`,
		chapterID, pages,
	)
	return err
}

// GetChapterPages returns the cached page-list JSON for a chapter.
// A cache miss returns (nil, nil) so callers can distinguish "not cached" from
// a real error.
func (d *DB) GetChapterPages(chapterID string) ([]byte, error) {
	var pages []byte
	err := d.db.QueryRow(
		`SELECT pages FROM chapter_pages WHERE chapter_id = ?`, chapterID,
	).Scan(&pages)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return pages, nil
}

// ResolveChapterID finds the database row ID for a chapter given the plugin
// ID and the source chapter ID. The chapters table stores row IDs as
// "pluginID|mangaID|sourceChapterID"; this query locates the right row by
// matching the prefix and the source_chapter_id column.
func (d *DB) ResolveChapterID(pluginID, sourceChapterID string) (string, error) {
	var id string
	err := d.db.QueryRow(
		`SELECT id FROM chapters WHERE source_chapter_id = ? AND id LIKE ? || '|%'`,
		sourceChapterID, pluginID,
	).Scan(&id)
	return id, err
}
