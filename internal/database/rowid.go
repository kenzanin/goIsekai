package database

import "fmt"

// MangaRowID resolves the internal row id for a manga given its plugin and
// source identifiers, or returns an error when the manga is not found.
func (d *DB) MangaRowID(pluginID, sourceMangaID string) (string, error) {
	var id string
	err := d.db.QueryRow(`SELECT id FROM mangas WHERE plugin_id = ? AND source_manga_id = ?`, pluginID, sourceMangaID).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("manga row: %w", err)
	}
	return id, nil
}

// MangaTitle returns the current main title for a manga, or an error when
// the manga is not found.
func (d *DB) MangaTitle(pluginID, sourceMangaID string) (string, error) {
	var title string
	err := d.db.QueryRow(`SELECT title FROM mangas WHERE plugin_id = ? AND source_manga_id = ?`, pluginID, sourceMangaID).Scan(&title)
	if err != nil {
		return "", fmt.Errorf("manga title: %w", err)
	}
	return title, nil
}
