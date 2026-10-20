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

// InfoPluginDir returns the directory containing an enrichment script, or ""
// when no script with that id is registered. Needed because info scripts are
// not manga sources: LoadedPlugins excludes them, so PluginDir cannot see them,
// and a logo beside main.wasm/main.lua would otherwise be unreachable.
func (s *AppService) InfoPluginDir(pluginID string) string {
	for _, p := range s.mgr.InfoPlugins() {
		if p.ID != pluginID || p.WasmPath == "" {
			continue
		}
		// wasmPath is the entry FILE for wasm scripts and the FOLDER for
		// lua/js/yaegi, so normalise to a directory either way.
		if fi, err := os.Stat(p.WasmPath); err == nil && fi.IsDir() {
			return p.WasmPath
		}
		return filepath.Dir(p.WasmPath)
	}
	return ""
}

// InfoPluginLogoURL returns the URL for an enrichment script's logo, or ""
// when it has none.
//
// Info scripts live under info_dir, not the plugin directory, so they need
// their own static path — and an id can exist as BOTH a scraper and an info
// script (mangadex is), so the scraper's cached logo must not shadow the
// script's own file. The declared plugin.json logo wins; otherwise the usual
// filenames beside the entry are probed.
func (s *AppService) InfoPluginLogoURL(pluginID string) string {
	dir := s.InfoPluginDir(pluginID)
	if dir == "" {
		return ""
	}

	name := ""
	for _, p := range s.mgr.InfoPlugins() {
		if p.ID != pluginID {
			continue
		}
		src := p.Logo
		if src != "" && !strings.HasPrefix(src, "http://") &&
			!strings.HasPrefix(src, "https://") && !strings.HasPrefix(src, "data:") {
			base := filepath.Base(src)
			if fi, err := os.Stat(filepath.Join(dir, base)); err == nil && !fi.IsDir() {
				name = base
			}
		}
		break
	}
	if name == "" {
		for _, cand := range []string{"logo.png", "logo.svg", "logo.jpg", "logo.ico"} {
			if fi, err := os.Stat(filepath.Join(dir, cand)); err == nil && !fi.IsDir() {
				name = cand
				break
			}
		}
	}
	if name == "" {
		return ""
	}
	return "/info-static/" + pluginID + "/" + name
}

// SyncPluginMeta persists a loaded plugin's identity metadata (name, logo)
// to the database so deferred plugins show correct data on the next page view.
// Every logo shape (file, URL, inline) is first materialised into the
// per-plugin cache (<cacheDir>/images/plugins/<id>/logo.<ext>); the DB row
// stores the stable cache URL so all runtimes render identically without a
// live fetch or a loaded VM at next startup.
func (s *AppService) SyncPluginMeta(id string) {
	meta := s.PluginMeta(id)
	if meta.Name == "" && meta.Logo == "" {
		return
	}
	// Best-effort: cache the declared source, then point the row at it.
	s.CachePluginLogo(id)
	iconURL := s.LogoCacheURL(id)
	if iconURL == "" {
		iconURL = meta.Logo
		if iconURL != "" && !strings.HasPrefix(iconURL, "http") && !strings.HasPrefix(iconURL, "data:") {
			iconURL = "/plugin-static/" + id + "/" + iconURL
		}
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
