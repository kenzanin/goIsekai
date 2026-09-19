// Package bridge is the final service layer binding plugin results and SQLite
// persistence to the frontend. It delegates search/detail/page lookups to the
// plugin manager, mirrors fetched manga into SQLite so progress can be tracked,
// and proxies image fetches through the sandboxed hostnet proxy.
package bridge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"goisekai/internal/database"
	"goisekai/internal/enrich"
	"goisekai/internal/hostnet"
	"goisekai/internal/logger"
	"goisekai/internal/pluginmanager"
	"goisekai/pkg/types"
)

// AppService wires the plugin manager, hostnet proxy, and SQLite database into
// the single entry point the frontend calls.
type AppService struct {
	db          *database.DB
	mgr         *pluginmanager.Manager
	proxy       *hostnet.Proxy
	cfgPath     string
	cacheDir    string
	imageMu     sync.RWMutex
	imageCache  map[string][]byte
	hostLanesMu sync.Mutex
	hostLanes   map[string]*hostLanes // host -> priority lanes
	imgPaceMu   sync.Mutex
	imgPace     map[string]time.Time // host -> earliest allowed next request (MD@Home pacing)
	enrich      *enrich.Registry
	genres      *genreIndex
	statusAlias map[string][]string
	imgFormat   ImageFormat
	coverMaxDim int
	enhance     enhanceConfig
}

// NewAppService returns an AppService backed by the supplied database, plugin
// manager, hostnet proxy, and enrichment registry.
func NewAppService(db *database.DB, mgr *pluginmanager.Manager, proxy *hostnet.Proxy, cfgPath, cacheDir string, enrichReg *enrich.Registry) *AppService {
	return &AppService{
		db:          db,
		mgr:         mgr,
		proxy:       proxy,
		cfgPath:     cfgPath,
		cacheDir:    cacheDir,
		imageCache:  make(map[string][]byte),
		enrich:      enrichReg,
		genres:      loadGenreIndex(cfgPath),
		statusAlias: loadStatusAlias(cfgPath),
		imgFormat:   loadImageFormat(cfgPath),
		coverMaxDim: loadCoverMaxDim(cfgPath),
		enhance:     loadEnhanceConfig(cfgPath),
	}
}

// Log receives a console message from the frontend and writes it to the Go logger.
func (s *AppService) Log(level string, msg string) {
	switch level {
	case "error":
		logger.Error("[ui] " + msg)
	case "warn":
		logger.Warn("[ui] " + msg)
	default:
		logger.Debug("[ui] " + msg)
	}
}

// GetConfigPath returns the path to goisekai.ini.
func (s *AppService) GetConfigPath() string {
	return s.cfgPath
}

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

// GetChapterList fetches the chapter list for a manga from the plugin.
func (s *AppService) GetChapterList(pluginID, mangaID string) ([]types.Chapter, error) {
	chapters, err := s.mgr.GetChapterList(pluginID, mangaID)
	if err != nil {
		return nil, fmt.Errorf("bridge: get chapter list: %w", err)
	}
	return chapters, nil
}
