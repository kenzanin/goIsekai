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

// MangaTitleIfCustom returns the stored main title and whether it is a
// user-set (custom) title that must override the plugin-sourced one.
func (d *DB) MangaTitleIfCustom(pluginID, sourceMangaID string) (string, bool, error) {
	var title string
	var custom bool
	err := d.db.QueryRow(`SELECT title, custom_title FROM mangas WHERE plugin_id = ? AND source_manga_id = ?`, pluginID, sourceMangaID).Scan(&title, &custom)
	if err != nil {
		return "", false, fmt.Errorf("manga custom title: %w", err)
	}
	return title, custom, nil
}
func (d *DB) MangaTitle(pluginID, sourceMangaID string) (string, error) {
	var title string
	err := d.db.QueryRow(`SELECT title FROM mangas WHERE plugin_id = ? AND source_manga_id = ?`, pluginID, sourceMangaID).Scan(&title)
	if err != nil {
		return "", fmt.Errorf("manga title: %w", err)
	}
	return title, nil
}
