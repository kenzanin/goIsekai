package database

import (
	. "github.com/go-jet/jet/v2/sqlite"
	"goisekai/internal/database/.gen/model"
	. "goisekai/internal/database/.gen/table"
)

// UpsertManga inserts a manga or, on a duplicate (plugin_id, source_manga_id),
// refreshes the mutable columns and updated_at.
func (d *DB) UpsertManga(m Manga) error {
	_, err := Mangas.INSERT(
		Mangas.ID,
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
		m.ID,
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
			Mangas.Title.SET(Mangas.EXCLUDED.Title),
			Mangas.CoverURL.SET(Mangas.EXCLUDED.CoverURL),
			Mangas.Description.SET(Mangas.EXCLUDED.Description),
			Mangas.Status.SET(Mangas.EXCLUDED.Status),
			Mangas.UpdatedAt.SET(RawTimestamp("CURRENT_TIMESTAMP")),
		),
	).Exec(d.db)
	return err
}

// ToggleLibrary flips the in_library flag (0 <-> 1) for a manga.
func (d *DB) ToggleLibrary(mangaID string) error {
	_, err := Mangas.UPDATE().
		SET(Mangas.InLibrary.SET(Int(1).SUB(Mangas.InLibrary))).
		WHERE(Mangas.ID.EQ(String(mangaID))).
		Exec(d.db)
	return err
}

// ListLibrary returns all in-library manga ordered by last update.
func (d *DB) ListLibrary() ([]Manga, error) {
	var models []model.Mangas
	err := Mangas.SELECT(Mangas.AllColumns).
		WHERE(Mangas.InLibrary.EQ(Int(1))).
		ORDER_BY(Mangas.UpdatedAt.DESC()).
		Query(d.db, &models)
	if err != nil {
		return nil, err
	}
	result := make([]Manga, len(models))
	for i, m := range models {
		result[i] = mangaFromModel(m)
	}
	return result, nil
}
