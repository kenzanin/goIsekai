package database

import (
	"goisekai/internal/database/.gen/model"

	. "goisekai/internal/database/.gen/table"

	. "github.com/go-jet/jet/v2/sqlite"
)

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
			Mangas.CoverURL.SET(Mangas.EXCLUDED.CoverURL),
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
	return id, err
}

// ToggleLibrary flips the in_library flag (0 <-> 1) for a manga.
func (d *DB) ToggleLibrary(mangaID int64) error {
	_, err := Mangas.UPDATE().
		SET(Mangas.InLibrary.SET(Int(1).SUB(Mangas.InLibrary))).
		WHERE(Mangas.ID.EQ(Int(mangaID))).
		Exec(d.db)
	return err
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
