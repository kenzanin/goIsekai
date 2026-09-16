package database

import (
	"database/sql"
	"errors"
)

// SaveChapterPages persists the pre-marshaled JSON page list for a chapter.
// INSERT OR REPLACE ensures a second fetch overwrites the cached copy.
func (d *DB) SaveChapterPages(chapterID int64, pages []byte) error {
	_, err := d.db.Exec(
		`INSERT OR REPLACE INTO chapter_pages (chapter_id, pages) VALUES (?, ?)`,
		chapterID, pages,
	)
	return err
}

// GetChapterPages returns the cached page-list JSON for a chapter.
// A cache miss returns (nil, nil) so callers can distinguish "not cached" from
// a real error.
func (d *DB) GetChapterPages(chapterID int64) ([]byte, error) {
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

// ResolveMangaIntID looks up the integer manga ID from source-based identifiers.
// A missing row returns 0, so callers can treat the manga as absent without
// distinguishing "unknown" from a real query failure.
func (d *DB) ResolveMangaIntID(pluginID, sourceMangaID string) (int64, error) {
	var id int64
	err := d.db.QueryRow(
		`SELECT id FROM mangas WHERE plugin_id = ? AND source_manga_id = ?`,
		pluginID, sourceMangaID,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return id, err
}

// ResolveChapterIntID looks up the integer chapter ID from source-based
// identifiers. An empty mangaID resolves by plugin and source chapter id alone,
// which is what the reader knows when it opens a chapter. A missing row returns
// 0, matching ResolveMangaIntID.
func (d *DB) ResolveChapterIntID(pluginID, mangaID, sourceChapterID string) (int64, error) {
	var id int64
	var err error
	if mangaID == "" {
		err = d.db.QueryRow(
			`SELECT c.id FROM chapters c INNER JOIN mangas m ON c.manga_id = m.id
			WHERE m.plugin_id = ? AND c.source_chapter_id = ?`,
			pluginID, sourceChapterID,
		).Scan(&id)
	} else {
		var mangaIntID int64
		if mangaIntID, err = d.ResolveMangaIntID(pluginID, mangaID); err != nil {
			return 0, err
		}
		err = d.db.QueryRow(
			`SELECT id FROM chapters WHERE manga_id = ? AND source_chapter_id = ?`,
			mangaIntID, sourceChapterID,
		).Scan(&id)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return id, err
}
