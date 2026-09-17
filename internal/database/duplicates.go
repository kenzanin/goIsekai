package database

import "strconv"

// DuplicateGroup is a set of ≥2 in-library manga that share a normalised
// title or alternative title (or, when from different plugins, a normalised
// description or alternative description).  The UI consumes these to flag
// likely dupes.
type DuplicateGroup struct {
	Key     string  // normalised key that was matched
	Title   string  // most readable original title (main title preferred)
	Members []Manga // the manga in this group (≥2)
}

// minDescKeyLen is the minimum length (in runes) of a normalised description
// before it takes part in duplicate matching.  Short blurbs ("read online
// free", SEO placeholder text) are too generic to be evidence on their own.
const minDescKeyLen = 100

// mangaIDFromRowID reads the manga primary key that alt_titles and
// alt_descriptions keep in their manga_row_id column. Those tables key on
// mangas.id, so a row id that is not a number (the older "plugin|sourceID"
// form) cannot be attributed to a manga and resolves to 0.
func mangaIDFromRowID(rowID string) int64 {
	id, err := strconv.ParseInt(rowID, 10, 64)
	if err != nil {
		return 0
	}
	return id
}
