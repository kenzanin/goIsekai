package database

import (
	"time"

	"goisekai/internal/database/.gen/model"

	. "goisekai/internal/database/.gen/table"

	. "github.com/go-jet/jet/v2/sqlite"
)

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

// ListLibraryStale returns in-library manga whose updated_at predates cutoff.
// The scheduler uses it to decide which titles need a refresh from their plugin.
func (d *DB) ListLibraryStale(cutoff time.Time) ([]Manga, error) {
	var models []model.Mangas
	err := Mangas.SELECT(Mangas.AllColumns).
		WHERE(Mangas.InLibrary.EQ(Int(1)).
			AND(Mangas.UpdatedAt.LT(RawTimestamp("'"+cutoff.UTC().Format(time.DateTime)+"'")))).
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

// MarkMangaNew stamps new_since so the library card shows the [New] badge
// until the manga is opened.
func (d *DB) MarkMangaNew(mangaID int64) error {
	_, err := Mangas.UPDATE().
		SET(Mangas.NewSince.SET(RawTimestamp("CURRENT_TIMESTAMP"))).
		WHERE(Mangas.ID.EQ(Int(mangaID))).
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

// ClearMangaNewString resets the [New] badge using source identifiers.
func (d *DB) ClearMangaNewString(pluginID, sourceMangaID string) error {
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

// LibraryCategories returns the categories of every in-library manga, keyed by
// the manga row-ID. One query for the whole library: ListEnrichment answers the
// same question for a single manga, which would mean a query per card.
//
// Categories double as the library's tag set - they are what enrichment writes
// (genres, sources the user tagged) and what a "show me only Isekai" filter has
// to match against.
func (d *DB) LibraryCategories() (map[string][]string, error) {
	rows, err := d.db.Query(`
		SELECT c.manga_row_id, c.category
		FROM manga_categories c
		JOIN mangas m ON m.id = c.manga_row_id
		WHERE m.in_library = 1
		ORDER BY c.manga_row_id, c.category`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make(map[string][]string)
	for rows.Next() {
		var rowID, category string
		if err := rows.Scan(&rowID, &category); err != nil {
			return nil, err
		}
		out[rowID] = append(out[rowID], category)
	}
	return out, rows.Err()
}

// LibraryCategoryCounts returns how many in-library manga carry each category,
// most common first, so the filter can offer tags in a useful order.
func (d *DB) LibraryCategoryCounts() (map[string]int, error) {
	rows, err := d.db.Query(`
		SELECT c.category, COUNT(*) AS n
		FROM manga_categories c
		JOIN mangas m ON m.id = c.manga_row_id
		WHERE m.in_library = 1
		GROUP BY c.category`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make(map[string]int)
	for rows.Next() {
		var category string
		var n int
		if err := rows.Scan(&category, &n); err != nil {
			return nil, err
		}
		out[category] = n
	}
	return out, rows.Err()
}
