package database

import (
	"github.com/goccy/go-json"
	"slices"
	"time"

	"goisekai/internal/database/.gen/model"
	. "goisekai/internal/database/.gen/table"

	. "github.com/go-jet/jet/v2/sqlite"
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
			Mangas.Title.SET(RawString("CASE WHEN mangas.custom_title = 1 THEN mangas.title ELSE excluded.title END")),
			Mangas.CoverURL.SET(Mangas.EXCLUDED.CoverURL),
			Mangas.Description.SET(RawString("CASE WHEN mangas.custom_description = 1 THEN mangas.description ELSE excluded.description END")),
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
	err := Mangas.SELECT(Mangas.InLibrary.AS("in_library")).
		WHERE(Mangas.ID.EQ(String(mangaID))).
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

// PluginCount is one row of the per-plugin library title counts.
type PluginCount struct {
	PluginID string `alias:"mangas.plugin_id"`
	Count    int    `alias:"stats.count"`
}

// CountLibraryByPlugin returns how many in-library titles each plugin
// contributes, ordered by count descending.
func (d *DB) CountLibraryByPlugin() ([]PluginCount, error) {
	var rows []PluginCount
	stmt := SELECT(
		Mangas.PluginID.AS("mangas.plugin_id"),
		COUNT(Mangas.ID).AS("stats.count"),
	).FROM(Mangas).
		WHERE(Mangas.InLibrary.EQ(Int(1))).
		GROUP_BY(Mangas.PluginID)
	if err := stmt.Query(d.db, &rows); err != nil {
		return nil, err
	}
	slices.SortFunc(rows, func(a, b PluginCount) int { return b.Count - a.Count })
	return rows, nil
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

// SetMangaGenres stores a user-defined genre override as a JSON text array.
// Pass nil to clear the override (return to plugin-supplied genres).
func (d *DB) SetMangaGenres(mangaRowID string, genres []string) error {
	var payload *string
	if genres != nil {
		raw, err := json.Marshal(genres)
		if err != nil {
			return err
		}
		s := string(raw)
		payload = &s
	}
	_, err := d.db.Exec(`UPDATE mangas SET genres = ? WHERE id = ?`, payload, mangaRowID)
	return err
}

// SetMangaCoverDim sets the cover dim overlay flag (0 = off, 1 = on).
func (d *DB) SetMangaCoverDim(mangaRowID string, dim int64) error {
	_, err := d.db.Exec(`UPDATE mangas SET cover_dim = ? WHERE id = ?`, dim, mangaRowID)
	return err
}

// SetMangaAuthor stores the author captured by an enrichment provider.
// An empty author clears the value.
func (d *DB) SetMangaAuthor(mangaRowID, author string) error {
	_, err := d.db.Exec(`UPDATE mangas SET author = ? WHERE id = ?`, author, mangaRowID)
	return err
}

// GetMangaAuthor returns the stored enrichment author for a manga.
// Returns ("", false) when nothing was fetched yet.
func (d *DB) GetMangaAuthor(mangaRowID string) (string, bool, error) {
	var author string
	err := d.db.QueryRow(`SELECT author FROM mangas WHERE id = ?`, mangaRowID).Scan(&author)
	if err != nil {
		return "", false, err
	}
	if author == "" {
		return "", false, nil
	}
	return author, true, nil
}

// GetMangaCoverDim returns the current cover_dim flag for a manga.
// Returns (0, false) when no override exists.
func (d *DB) GetMangaCoverDim(mangaRowID string) (int64, bool, error) {
	var dim *int64
	err := d.db.QueryRow(`SELECT cover_dim FROM mangas WHERE id = ?`, mangaRowID).Scan(&dim)
	if err != nil {
		return 0, false, err
	}
	if dim == nil {
		return 0, false, nil
	}
	return *dim, true, nil
}

// GetMangaGenres returns the stored genre override for a manga.
// Returns the genre list and true on success; returns (nil, false) when no override exists.
func (d *DB) GetMangaGenres(mangaRowID string) ([]string, bool, error) {
	var genresJSON *string
	err := d.db.QueryRow(`SELECT genres FROM mangas WHERE id = ?`, mangaRowID).Scan(&genresJSON)
	if err != nil {
		return nil, false, err
	}
	if genresJSON == nil || *genresJSON == "" {
		return nil, false, nil
	}
	var genres []string
	if err := json.Unmarshal([]byte(*genresJSON), &genres); err != nil {
		return nil, false, err
	}
	return genres, true, nil
}
