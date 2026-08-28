package database

// RegisterPlugin inserts a plugin or, on a duplicate id, refreshes its fields.
func (d *DB) RegisterPlugin(p Plugin) error {
	_, err := d.db.Exec(`
		INSERT INTO plugins
			(id, name, version, wasm_path, is_active, icon_url)
	VALUES
			(?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			version = excluded.version,
			wasm_path = excluded.wasm_path,
			is_active = excluded.is_active,
			icon_url = excluded.icon_url`,
		p.ID, p.Name, p.Version, p.WasmPath, intFromBool(p.IsActive), p.IconURL,
	)
	return err
}

// ListPlugins returns all registered plugins ordered by name.
func (d *DB) ListPlugins() ([]Plugin, error) {
	rows, err := d.db.Query(
		`SELECT id, name, version, wasm_path, is_active, icon_url FROM plugins ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	return scanPlugin(rows)
}
