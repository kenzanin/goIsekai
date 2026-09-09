package database

import (
	"encoding/json"
	"fmt"
)

// GetAltTitles returns the stored alt titles as the JSON payload
// {"source": first row's source, "titles": [...]}, or "" when absent.
func (d *DB) GetAltTitles(pluginID, sourceMangaID string) (string, error) {
	rowID, err := d.mangaRowID(pluginID, sourceMangaID)
	if err != nil {
		return "", err
	}
	rows, err := d.db.Query(`SELECT title, source FROM alt_titles WHERE manga_row_id = ?`, rowID)
	if err != nil {
		return "", err
	}
	defer func() { _ = rows.Close() }()
	type alt struct {
		Title  string `json:"title"`
		Source string `json:"source"`
	}
	var alts []alt
	for rows.Next() {
		var a alt
		if err := rows.Scan(&a.Title, &a.Source); err != nil {
			return "", err
		}
		alts = append(alts, a)
	}
	if len(alts) == 0 {
		return "", rows.Err()
	}
	payload := struct {
		Source string   `json:"source"`
		Titles []string `json:"titles"`
	}{Source: alts[0].Source, Titles: []string{}}
	for _, a := range alts {
		payload.Titles = append(payload.Titles, a.Title)
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// mangaRowID resolves the mangas.id PK from (plugin_id, source_manga_id).
func (d *DB) mangaRowID(pluginID, sourceMangaID string) (string, error) {
	var id string
	err := d.db.QueryRow(`SELECT id FROM mangas WHERE plugin_id = ? AND source_manga_id = ?`, pluginID, sourceMangaID).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("manga row: %w", err)
	}
	return id, nil
}
