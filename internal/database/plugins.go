package database

import (
	"goisekai/internal/database/.gen/model"
	. "goisekai/internal/database/.gen/table"

	. "github.com/go-jet/jet/v2/sqlite"
)

// RegisterPlugin inserts a plugin or, on a duplicate id, refreshes its metadata.
//
// is_active is only written on insert: discovery re-registers every plugin on
// each startup, and letting it overwrite the column would silently re-enable
// plugins the user had switched off. The stored flag stays the source of truth.
func (d *DB) RegisterPlugin(p Plugin) error {
	_, err := Plugins.INSERT(
		Plugins.ID,
		Plugins.Name,
		Plugins.Version,
		Plugins.WasmPath,
		Plugins.IsActive,
		Plugins.IconURL,
		Plugins.ThumbRatio,
	).VALUES(
		p.ID,
		p.Name,
		p.Version,
		p.WasmPath,
		boolToInt(p.IsActive),
		p.IconURL,
		Float(p.ThumbRatio),
	).ON_CONFLICT(Plugins.ID).DO_UPDATE(
		SET(
			Plugins.Version.SET(Plugins.EXCLUDED.Version),
			Plugins.WasmPath.SET(Plugins.EXCLUDED.WasmPath),
			Plugins.ThumbRatio.SET(Plugins.EXCLUDED.ThumbRatio),
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

// UpdatePluginIdentity refreshes the display name and logo for a plugin row.
func (d *DB) UpdatePluginIdentity(id, name, iconURL string) error {
	_, err := Plugins.UPDATE().
		SET(
			Plugins.Name.SET(String(name)),
			Plugins.IconURL.SET(String(iconURL)),
		).
		WHERE(Plugins.ID.EQ(String(id))).
		Exec(d.db)
	return err
}

// TogglePluginActive flips the is_active flag for a plugin.
func (d *DB) TogglePluginActive(id string) error {
	_, err := Plugins.UPDATE().
		SET(Plugins.IsActive.SET(Int(1).SUB(Plugins.IsActive))).
		WHERE(Plugins.ID.EQ(String(id))).
		Exec(d.db)
	return err
}

// SetPluginActive sets is_active explicitly. Refresh uses it to deactivate
// plugins whose files are gone, without toggling (which would re-enable an
// already-inactive row).
func (d *DB) SetPluginActive(id string, active bool) error {
	v := 0
	if active {
		v = 1
	}
	_, err := Plugins.UPDATE().
		SET(Plugins.IsActive.SET(Int(int64(v)))).
		WHERE(Plugins.ID.EQ(String(id))).
		Exec(d.db)
	return err
}

// DeletePlugin removes a plugin row entirely. Only for artifacts that were
// never real plugins (e.g. *.bak.* backup dirs that discovery once picked
// up). Real plugins are deactivated, not deleted, to preserve history.
func (d *DB) DeletePlugin(id string) error {
	_, err := Plugins.DELETE().
		WHERE(Plugins.ID.EQ(String(id))).
		Exec(d.db)
	return err
}
