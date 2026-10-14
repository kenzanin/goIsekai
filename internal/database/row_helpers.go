package database

import "database/sql"

// scanRows is a generic helper to scan rows into a slice of any type.
// It handles the common rows.Next() loop pattern with proper error handling.
func scanRows[T any](rows *sql.Rows, scanItem func(*T) error) ([]T, error) {
	defer func() { _ = rows.Close() }()
	var out []T
	for rows.Next() {
		var item T
		if err := scanItem(&item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
