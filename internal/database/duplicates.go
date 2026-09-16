package database

import (
	"fmt"
	. "github.com/go-jet/jet/v2/sqlite"
	. "goisekai/internal/database/.gen/table"
)

// DuplicateGroup is a set of ≥2 in-library manga that share a normalised
// title or alternative title (or, when from different plugins, a normalised
// description or alternative description).  The UI consumes these to flag
// likely dupes.
// getMangaIntIDFromRowID looks up the integer manga ID from source-based rowID.
// Returns 0 if not found.
func (d *DB) getMangaIntIDFromRowID(rowID string) (int64, error) {
	var pluginID, sourceMangaID string
	for i, c := range rowID {
		if c == '|' {
			pluginID = rowID[:i]
			sourceMangaID = rowID[i+1:]
			break
		}
	}
	if pluginID == "" {
		return 0, fmt.Errorf("invalid rowID format: %s", rowID)
	}
	var id int64
	err := Mangas.SELECT(Mangas.ID).
		WHERE(Mangas.PluginID.EQ(String(pluginID)).AND(Mangas.SourceMangaID.EQ(String(sourceMangaID)))).
		Query(d.db, &id)
	return id, err
}

type DuplicateGroup struct {
	Key     string  // normalised key that was matched
	Title   string  // most readable original title (main title preferred)
	Members []Manga // the manga in this group (≥2)
}

// minDescKeyLen is the minimum length (in runes) of a normalised description
// before it takes part in duplicate matching.  Short blurbs ("read online
// free", SEO placeholder text) are too generic to be evidence on their own.
const minDescKeyLen = 100
