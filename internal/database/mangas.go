package database

import (
	"time"

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

// IsInLibrary reports whether a manga is currently saved in the library.
// A missing row simply means "not in library".
func (d *DB) IsInLibrary(mangaID string) (bool, error) {
	var rows []struct {
		InLibrary int
	}
	err := Mangas.SELECT(Mangas.InLibrary).
		WHERE(Mangas.ID.EQ(String(mangaID))).
		Query(d.db, &rows)
	if len(rows) == 0 {
		return false, nil
	}
	return rows[0].InLibrary == 1, err
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

// LibraryMangaStats holds per-manga aggregation for the library grid.
//
// qrm matches result columns to named-struct fields via two-part alias tags
// (anonymous structs match by bare lowercase column alias instead); a named
// struct without matching alias tags silently yields zero rows.
type LibraryMangaStats struct {
	MangaID       string     `alias:"mangas.manga_id"`
	TotalChapters int        `alias:"stats.total_chapters"`
	ReadChapters  int        `alias:"stats.read_chapters"`
	NewSince      *time.Time `alias:"mangas.new_since"`
	HasNew        bool       // derived: NewSince != nil
}

// ListLibraryWithProgress returns in-library manga with chapter count stats.
// HasNew comes from the new_since stamp (set when a sync finds new chapters,
// cleared when the manga is opened) — not from unread count, so the badge
// disappears on open as specified.
func (d *DB) ListLibraryWithProgress() ([]LibraryMangaStats, error) {
	readCond := Chapters.IsRead.EQ(Int(1)).OR(
		Chapters.TotalPages.GT(Int(0)).AND(Chapters.LastPageRead.GT_EQ(Chapters.TotalPages)))
	var out []LibraryMangaStats
	err := SELECT(
		Mangas.ID.AS("mangas.manga_id"),
		COUNT(Chapters.ID).AS("stats.total_chapters"),
		COALESCE(SUM(CASE().WHEN(readCond).THEN(Int(1)).ELSE(Int(0))), Int(0)).AS("stats.read_chapters"),
		Mangas.NewSince.AS("mangas.new_since"),
	).FROM(Mangas.LEFT_JOIN(Chapters, Chapters.MangaID.EQ(Mangas.ID))).
		WHERE(Mangas.InLibrary.EQ(Int(1))).
		GROUP_BY(Mangas.ID).
		ORDER_BY(Mangas.UpdatedAt.DESC()).
		Query(d.db, &out)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].HasNew = out[i].NewSince != nil
	}
	return out, nil
}

// MarkMangaNew stamps new_since so the library card shows the [New] badge
// until the manga is opened.
func (d *DB) MarkMangaNew(mangaRowID string) error {
	_, err := Mangas.UPDATE().
		SET(Mangas.NewSince.SET(RawTimestamp("CURRENT_TIMESTAMP"))).
		WHERE(Mangas.ID.EQ(String(mangaRowID))).
		Exec(d.db)
	return err
}

// ClearMangaNew resets the [New] badge after the manga is opened.
func (d *DB) ClearMangaNew(pluginID, sourceMangaID string) error {
	_, err := Mangas.UPDATE().
		SET(Mangas.NewSince.SET(TimestampExp(NULL))).
		WHERE(Mangas.PluginID.EQ(String(pluginID)).AND(Mangas.SourceMangaID.EQ(String(sourceMangaID)))).
		Exec(d.db)
	return err
}

// MangaPluginIDRow pairs a manga row-ID with its plugin-ID.
type MangaPluginIDRow struct {
	MangaID  string `alias:"mangas.manga_id"`
	PluginID string `alias:"mangas.plugin_id"`
}

// QueryMangaPluginIDs returns (manga_id, plugin_id) for all in-library manga.
func (d *DB) QueryMangaPluginIDs() ([]MangaPluginIDRow, error) {
	var out []MangaPluginIDRow
	err := SELECT(Mangas.ID.AS("mangas.manga_id"), Mangas.PluginID.AS("mangas.plugin_id")).
		FROM(Mangas).
		WHERE(Mangas.InLibrary.EQ(Int(1))).
		Query(d.db, &out)
	return out, err
}
