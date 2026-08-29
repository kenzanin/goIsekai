package database

import (
	"database/sql"
	"time"
)

// scanTime converts a SQLite value (text from CURRENT_TIMESTAMP, []byte, or
// time.Time) into a time.Time, tolerating the driver's variable encoding.
func scanTime(v any) time.Time {
	switch t := v.(type) {
	case time.Time:
		return t
	case []byte:
		return parseTime(t)
	case string:
		return parseTime([]byte(t))
	case nil:
		return time.Time{}
	default:
		return time.Time{}
	}
}

// parseTime tries the timestamps SQLite's CURRENT_TIMESTAMP emits.
func parseTime(b []byte) time.Time {
	formats := []string{
		"2006-01-02 15:04:05",
		time.RFC3339,
		"2006-01-02T15:04:05",
	}
	for _, f := range formats {
		if tm, err := time.Parse(f, string(b)); err == nil {
			return tm
		}
	}
	return time.Time{}
}

func boolFromInt(v int) bool { return v != 0 }

func intFromBool(b bool) int {
	if b {
		return 1
	}
	return 0
}

func scanManga(rows *sql.Rows) ([]Manga, error) {
	defer func() {
		_ = rows.Close()
	}()
	var out []Manga
	for rows.Next() {
		var (
			m         Manga
			inLibrary int
			createdAt any
			updatedAt any
		)
		if err := rows.Scan(
			&m.ID, &m.PluginID, &m.SourceMangaID, &m.Title, &m.CoverURL,
			&m.Description, &m.Status, &inLibrary, &createdAt, &updatedAt,
		); err != nil {
			return nil, err
		}
		m.InLibrary = boolFromInt(inLibrary)
		m.CreatedAt = scanTime(createdAt)
		m.UpdatedAt = scanTime(updatedAt)
		out = append(out, m)
	}
	return out, rows.Err()
}

func scanPlugin(rows *sql.Rows) ([]Plugin, error) {
	defer func() {
		_ = rows.Close()
	}()
	var out []Plugin
	for rows.Next() {
		var (
			p        Plugin
			isActive int
		)
		if err := rows.Scan(
			&p.ID, &p.Name, &p.Version, &p.WasmPath, &isActive, &p.IconURL,
		); err != nil {
			return nil, err
		}
		p.IsActive = boolFromInt(isActive)
		out = append(out, p)
	}
	return out, rows.Err()
}
