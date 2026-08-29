package database

import (
	. "github.com/go-jet/jet/v2/sqlite"
	"goisekai/internal/database/.gen/model"
	. "goisekai/internal/database/.gen/table"
)

// RegisterPlugin inserts a plugin or, on a duplicate id, refreshes its metadata.
func (d *DB) RegisterPlugin(p Plugin) error {
	_, err := Plugins.INSERT(
		Plugins.ID,
		Plugins.Name,
		Plugins.Version,
		Plugins.WasmPath,
		Plugins.IsActive,
		Plugins.IconURL,
	).VALUES(
		p.ID,
		p.Name,
		p.Version,
		p.WasmPath,
		boolToInt(p.IsActive),
		p.IconURL,
	).ON_CONFLICT(Plugins.ID).DO_UPDATE(
		SET(
			Plugins.Name.SET(Plugins.EXCLUDED.Name),
			Plugins.Version.SET(Plugins.EXCLUDED.Version),
			Plugins.WasmPath.SET(Plugins.EXCLUDED.WasmPath),
			Plugins.IsActive.SET(Plugins.EXCLUDED.IsActive),
			Plugins.IconURL.SET(Plugins.EXCLUDED.IconURL),
		),
	).Exec(d.db)
	return err
}

// ListPlugins returns all plugins ordered by name.
func (d *DB) ListPlugins() ([]Plugin, error) {
	var models []model.Plugins
	err := Plugins.SELECT(Plugins.AllColumns).
		ORDER_BY(Plugins.Name).
		Query(d.db, &models)
	if err != nil {
		return nil, err
	}
	result := make([]Plugin, len(models))
	for i, m := range models {
		result[i] = pluginFromModel(m)
	}
	return result, nil
}
