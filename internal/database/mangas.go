package database

import (
	"strconv"

	"goisekai/internal/database/.gen/model"

	. "goisekai/internal/database/.gen/table"

	. "github.com/go-jet/jet/v2/sqlite"
)

// SetMangaCover force-writes a manga's cover_url, bypassing the custom_cover
// guard that keeps plugin re-syncs from overwriting a user pick. Used by the
// explicit set-cover action, which IS the user pick.
func (d *DB) SetMangaCover(pluginID, sourceMangaID, coverURL string) error {
	_, err := Mangas.UPDATE().
		SET(Mangas.CoverURL.SET(String(coverURL))).
		WHERE(Mangas.PluginID.EQ(String(pluginID)).AND(Mangas.SourceMangaID.EQ(String(sourceMangaID)))).
		Exec(d.db)
	if err != nil {
		return err
	}
	var id int64
	if err := d.db.QueryRow(
		`SELECT id FROM mangas WHERE plugin_id = ? AND source_manga_id = ?`,
		pluginID, sourceMangaID,
	).Scan(&id); err != nil {
		return err
	}
	return d.SyncFTS(strconv.FormatInt(id, 10))
}

// UpsertManga inserts a manga or, on a duplicate (plugin_id, source_manga_id),
// refreshes the mutable columns and updated_at. Returns the manga ID.
func (d *DB) UpsertManga(m Manga) (int64, error) {
	_, err := Mangas.INSERT(
		Mangas.PluginID,
		Mangas.SourceMangaID,
		Mangas.Title,
		Mangas.CoverURL,
		Mangas.Description,
		Mangas.Status,
		Mangas.InLibrary,
		Mangas.CreatedAt,
		Mangas.UpdatedAt,
	).VALUES(
		m.PluginID,
		m.SourceMangaID,
		m.Title,
		m.CoverURL,
		m.Description,
		m.Status,
		boolToInt(m.InLibrary),
		RawTimestamp("CURRENT_TIMESTAMP"),
		RawTimestamp("CURRENT_TIMESTAMP"),
	).ON_CONFLICT(Mangas.PluginID, Mangas.SourceMangaID).DO_UPDATE(
		SET(
			Mangas.Title.SET(RawString("CASE WHEN mangas.custom_title = 1 THEN mangas.title ELSE excluded.title END")),
			Mangas.CoverURL.SET(RawString("CASE WHEN mangas.custom_cover = 1 THEN mangas.cover_url ELSE excluded.cover_url END")),
			Mangas.Description.SET(RawString("CASE WHEN mangas.custom_description = 1 THEN mangas.description ELSE excluded.description END")),
			Mangas.Status.SET(Mangas.EXCLUDED.Status),
			Mangas.UpdatedAt.SET(RawTimestamp("CURRENT_TIMESTAMP")),
		),
	).Exec(d.db)
	if err != nil {
		return 0, err
	}
	// last_insert_rowid is stale when the ON CONFLICT DO UPDATE path fires,
	// so resolve the id from the unique key instead.
	var id int64
	err = d.db.QueryRow(
		`SELECT id FROM mangas WHERE plugin_id = ? AND source_manga_id = ?`,
		m.PluginID, m.SourceMangaID,
	).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, d.SyncFTS(strconv.FormatInt(id, 10))
}

// ToggleLibrary flips the in_library flag (0 <-> 1) for a manga and keeps the
// search index in step with the new membership.
func (d *DB) ToggleLibrary(mangaID int64) error {
	_, err := Mangas.UPDATE().
		SET(Mangas.InLibrary.SET(Int(1).SUB(Mangas.InLibrary))).
		WHERE(Mangas.ID.EQ(Int(mangaID))).
		Exec(d.db)
	if err != nil {
		return err
	}
	return d.SyncFTS(strconv.FormatInt(mangaID, 10))
}

// IsInLibrary reports whether a manga is currently saved in the library.
// A missing row simply means "not in library".
func (d *DB) IsInLibrary(mangaID int64) (bool, error) {
	var rows []struct {
		InLibrary int
	}
	err := Mangas.SELECT(Mangas.InLibrary.AS("in_library")).
		WHERE(Mangas.ID.EQ(Int(mangaID))).
		Query(d.db, &rows)
	if len(rows) == 0 {
		return false, nil
	}
	return rows[0].InLibrary == 1, err
}

// RepointManga rewrites one manga's source identity by row id. It sets
// plugin_id, source_manga_id plus the refreshed title, cover, description and
// status, keeping custom_title and custom_description authoritative the same
// way UpsertManga does (a user-edited title/description is not overwritten).
func (d *DB) RepointManga(mangaID int64, pluginID, sourceMangaID, title, coverURL, description, status string) error {
	_, err := d.db.Exec(
		`UPDATE mangas SET
			plugin_id = ?,
			source_manga_id = ?,
			title = CASE WHEN custom_title = 1 THEN title ELSE ? END,
			cover_url = CASE WHEN custom_cover = 1 THEN cover_url ELSE ? END,
			description = CASE WHEN custom_description = 1 THEN description ELSE ? END,
			status = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		pluginID, sourceMangaID, title, coverURL, description, status, mangaID,
	)
	if err != nil {
		return err
	}
	return d.SyncFTS(strconv.FormatInt(mangaID, 10))
}

// GetMangaCached fetches a cached manga from the database by plugin ID and source manga ID.
// Returns (Manga, true) on cache hit, or (zero value, false) when absent.
func (d *DB) GetMangaCached(pluginID, sourceMangaID string) (Manga, error) {
	var out []model.Mangas
	err := Mangas.SELECT(Mangas.AllColumns).
		WHERE(Mangas.PluginID.EQ(String(pluginID)).AND(Mangas.SourceMangaID.EQ(String(sourceMangaID)))).
		Query(d.db, &out)
	if err != nil || len(out) == 0 {
		return Manga{}, err
	}
	return mangaFromModel(out[0]), nil
}
