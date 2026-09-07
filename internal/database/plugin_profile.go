package database

// SetPluginProfile persists a plugin's pinned TLS profile ("" clears the pin).
func (d *DB) SetPluginProfile(pluginID, profile string) error {
	_, err := d.db.Exec(`UPDATE plugins SET http_profile = ? WHERE id = ?`, profile, pluginID)
	return err
}

// GetPluginProfiles returns the persisted per-plugin TLS profile pins, keyed by
// plugin id. Only rows with a non-empty pin are included.
func (d *DB) GetPluginProfiles() (map[string]string, error) {
	rows, err := d.db.Query(`SELECT id, http_profile FROM plugins WHERE http_profile != ''`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := map[string]string{}
	for rows.Next() {
		var id, profile string
		if err := rows.Scan(&id, &profile); err != nil {
			return nil, err
		}
		out[id] = profile
	}
	return out, rows.Err()
}
