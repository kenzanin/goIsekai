package database

// UpsertManga inserts a manga or, on a duplicate (plugin_id, source_manga_id),
// updates the mutable fields and refreshes updated_at.
func (d *DB) UpsertManga(m Manga) error {
	_, err := d.db.Exec(`
		INSERT INTO mangas
			(id, plugin_id, source_manga_id, title, cover_url, description, status, in_library, created_at, updated_at)
	VALUES
			(?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(plugin_id, source_manga_id) DO UPDATE SET
			title = excluded.title,
			cover_url = excluded.cover_url,
			description = excluded.description,
			status = excluded.status,
			updated_at = CURRENT_TIMESTAMP`,
		m.ID, m.PluginID, m.SourceMangaID, m.Title, m.CoverURL, m.Description, m.Status, intFromBool(m.InLibrary),
	)
	return err
}

// ToggleLibrary flips the in_library flag for a manga.
func (d *DB) ToggleLibrary(mangaID string) error {
	_, err := d.db.Exec(
		`UPDATE mangas SET in_library = 1 - in_library, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		mangaID,
	)
	return err
}

// ListLibrary returns all in-library mangas, most recently updated first.
func (d *DB) ListLibrary() ([]Manga, error) {
	rows, err := d.db.Query(
		`SELECT id, plugin_id, source_manga_id, title, cover_url, description, status, in_library, created_at, updated_at
		FROM mangas WHERE in_library = 1 ORDER BY updated_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	return scanManga(rows)
}
