package database

import (
	"time"

	"goisekai/internal/database/.gen/model"
)

// boolToInt converts a bool to the 0/1 integer stored in SQLite INTEGER columns.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// derefStr returns the string pointed to by p, or "" if p is nil.
func derefStr(p *string) string {
	if p != nil {
		return *p
	}
	return ""
}

// derefBool reports whether p is non-nil and non-zero.
func derefBool(p *int64) bool {
	return p != nil && *p != 0
}

// derefFloat returns the value pointed to by p, or 0 if p is nil.
func derefFloat(p *float64) float64 {
	if p != nil {
		return *p
	}
	return 0
}

// derefTime returns the time pointed to by p, or the zero time if p is nil.
func derefTime(p *time.Time) time.Time {
	if p != nil {
		return *p
	}
	return time.Time{}
}

// mangaFromModel maps a jet-managed Manga struct onto the public Manga.
func mangaFromModel(m model.Mangas) Manga {
	return Manga{
		ID:            derefStr(m.ID),
		PluginID:      m.PluginID,
		SourceMangaID: m.SourceMangaID,
		Title:         m.Title,
		CoverURL:      derefStr(m.CoverURL),
		Description:   derefStr(m.Description),
		Status:        derefStr(m.Status),
		InLibrary:     derefBool(m.InLibrary),
		CreatedAt:     derefTime(m.CreatedAt),
		UpdatedAt:     derefTime(m.UpdatedAt),
	}
}

// pluginFromModel maps a jet-managed Plugin struct onto the public Plugin.
func pluginFromModel(m model.Plugins) Plugin {
	return Plugin{
		ID:         derefStr(m.ID),
		Name:       m.Name,
		Version:    m.Version,
		WasmPath:   m.WasmPath,
		IsActive:   derefBool(m.IsActive),
		IconURL:    derefStr(m.IconURL),
		ThumbRatio: derefFloat(m.ThumbRatio),
	}
}
