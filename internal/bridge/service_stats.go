package bridge

import (
	"os"
	"path/filepath"
	"strings"

	"goisekai/internal/database"
	"goisekai/internal/logger"
)

// PluginDir returns the directory containing the plugin's main file, or "" if
// not found. Folder-based plugins (lua/js/yaegi) have WasmPath pointing at
// the plugin folder directly.
func (s *AppService) PluginDir(pluginID string) string {
	for _, p := range s.mgr.LoadedPlugins() {
		if p.ID != pluginID || p.WasmPath == "" {
			continue
		}
		if fi, err := os.Stat(p.WasmPath); err == nil && fi.IsDir() {
			return p.WasmPath
		}
		return filepath.Dir(p.WasmPath)
	}
	return ""
}

// SyncPluginMeta persists a loaded plugin's identity metadata (name, logo)
// to the database so deferred plugins show correct data on the next page view.
func (s *AppService) SyncPluginMeta(id string) {
	meta := s.PluginMeta(id)
	if meta.Name == "" && meta.Logo == "" {
		return
	}
	iconURL := meta.Logo
	if iconURL != "" && !strings.HasPrefix(iconURL, "http") && !strings.HasPrefix(iconURL, "data:") {
		iconURL = "/plugin-static/" + id + "/" + iconURL
	}
	if err := s.db.UpdatePluginIdentity(id, meta.Name, iconURL); err != nil {
		logger.Warn("sync plugin meta", "id", id, "error", err)
	}
}

// CacheStats returns cache metrics for the /api/stats endpoint.
func (s *AppService) CacheStats() (total, hits int64, err error) {
	return s.db.CacheStats()
}

// ListLibraryWithProgress returns per-manga chapter stats for the library grid.
func (s *AppService) ListLibraryWithProgress() ([]database.LibraryMangaStats, error) {
	return s.db.ListLibraryWithProgress()
}

// LibraryOverview returns aggregated library-wide stats for the stats row.
func (s *AppService) LibraryOverview() (database.LibraryOverview, error) {
	return s.db.LibraryOverview(database.StatusAlias(s.statusAlias))
}

// FindPotentialDuplicates returns groups of in-library manga that share a
// normalised title or alternative title.
func (s *AppService) FindPotentialDuplicates() ([]database.DuplicateGroup, error) {
	return s.db.FindPotentialDuplicates()
}

// CountLibraryByPlugin returns per-plugin in-library title counts.
func (s *AppService) CountLibraryByPlugin() ([]database.PluginCount, error) {
	return s.db.CountLibraryByPlugin()
}

// GetReadHistory returns the reading history enriched with plugin names.
func (s *AppService) GetReadHistory() ([]database.HistoryEntry, error) {
	entries, err := s.db.GetReadHistory()
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// LastReadChapter returns the most recently read chapter for a manga.
func (s *AppService) LastReadChapter(pluginID, mangaID string) (sourceChapterID string, pageNum int, ok bool) {
	mangaIntID, _ := s.db.ResolveMangaIntID(pluginID, mangaID)
	return s.db.LastReadChapter(mangaIntID)
}

// QueryMangaPluginIDs returns (manga_id, plugin_id) pairs for all in-library manga.
func (s *AppService) QueryMangaPluginIDs() ([]database.MangaPluginIDRow, error) {
	return s.db.QueryMangaPluginIDs()
}
