package database

import (
	"database/sql"
	"errors"
)

// Plugin genre cache: plugins.genres holds a JSON array [{"name","slug"}].
// NULL = not fetched yet; "[]" = plugin has no genre export (never re-invoke).
// Raw SQL like plugin_profile.go — the go-jet .gen tables are not regenerated
// for a single column.

// GetPluginGenres returns the cached genre JSON for a plugin.
// found=false means the column is NULL (cache miss).
func (d *DB) GetPluginGenres(pluginID string) (json string, found bool, err error) {
	var ns sql.NullString
	err = d.db.QueryRow(`SELECT genres FROM plugins WHERE id = ?`, pluginID).Scan(&ns)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return ns.String, ns.Valid && ns.String != "", nil
}

// SetPluginGenres persists the genre cache for a plugin (JSON array; "[]" = none).
func (d *DB) SetPluginGenres(pluginID, genresJSON string) error {
	_, err := d.db.Exec(`UPDATE plugins SET genres = ? WHERE id = ?`, genresJSON, pluginID)
	return err
}
