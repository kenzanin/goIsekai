package database

import "database/sql"

// execInsertIgnoreGeneric is a helper for the common INSERT OR IGNORE pattern
// used by AddCategories, AddAltTitles, AddAltDescriptions, etc.
func execInsertIgnoreGeneric(db *sql.DB, mangaRowID string, items []string, source string, insertTemplate string) (int, error) {
	inserted := 0
	for _, item := range items {
		res, err := db.Exec(insertTemplate, mangaRowID, item, source)
		if err != nil {
			return inserted, err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return inserted, err
		}
		inserted += int(n)
	}
	return inserted, nil
}

// execInsertIgnoreRowGeneric is a helper for INSERT OR IGNORE with complex row structs.
func execInsertIgnoreRowGeneric[T any](db *sql.DB, items []T, insertFunc func(*T) error) (int, error) {
	inserted := 0
	for i := range items {
		if err := insertFunc(&items[i]); err != nil {
			return inserted, err
		}
		// Note: we need to track rows affected, but the helper pattern
		// is different for structs vs simple strings
		inserted++
	}
	return inserted, nil
}
