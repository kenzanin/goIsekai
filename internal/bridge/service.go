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
	"goisekai/internal/hostnet"
	"goisekai/internal/logger"
	"goisekai/internal/pluginmanager"
	"goisekai/pkg/types"
)

// AppService wires the plugin manager, hostnet proxy, and SQLite database into
// the single entry point the frontend calls.
type AppService struct {
	db         *database.DB
	mgr        *pluginmanager.Manager
	proxy      *hostnet.Proxy
	cfgPath    string
	cacheDir   string
	imageMu    sync.RWMutex
	imageCache map[string][]byte
	imgSem     chan struct{} // caps concurrent image fetches per host process
	imgPaceMu  sync.Mutex
	imgPace    map[string]time.Time // host -> earliest allowed next request (MD@Home pacing)
}

// NewAppService returns an AppService backed by the supplied database, plugin
// manager, and hostnet proxy.
func NewAppService(db *database.DB, mgr *pluginmanager.Manager, proxy *hostnet.Proxy, cfgPath, cacheDir string) *AppService {
	return &AppService{
		db:         db,
		mgr:        mgr,
		proxy:      proxy,
		cfgPath:    cfgPath,
		cacheDir:   cacheDir,
		imageCache: make(map[string][]byte),
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

// CDPStatus returns the current CDP engine configuration.
func (s *AppService) CDPStatus() hostnet.CDPConfig {
	return s.proxy.CDPConfig()
}

// CDPCookies returns cookies from all per-plugin jars matching the domain.
func (s *AppService) CDPCookies(domain string) []hostnet.CDPCookie {
	return s.proxy.CDPCookies(domain)
}

// GetChapterList fetches the chapter list for a manga from the plugin.
func (s *AppService) GetChapterList(pluginID, mangaID string) ([]types.Chapter, error) {
	chapters, err := s.mgr.GetChapterList(pluginID, mangaID)
	if err != nil {
		return nil, fmt.Errorf("bridge: get chapter list: %w", err)
	}
	return chapters, nil
}

// TestCDP launches the configured CDP engine against the given URL, waits for
// the challenge to clear, and returns the harvested cookies and User-Agent.
func (s *AppService) TestCDP(targetURL string) ([]hostnet.CDPCookie, string, error) {
	cfg := s.proxy.CDPConfig()
	if cfg.Engine == "" || cfg.Engine == "off" {
		return nil, "", fmt.Errorf("CDP engine is not configured")
	}
	cookies, ua, err := s.proxy.TestCDP(cfg, targetURL)
	if err != nil {
		return nil, "", err
	}
	var out []hostnet.CDPCookie
	for _, c := range cookies {
		out = append(out, hostnet.CDPCookie{
			Name: c.Name, Value: c.Value, Domain: c.Domain,
			Path: c.Path, Secure: c.Secure, HTTPOnly: c.HttpOnly,
		})
	}
	return out, ua, nil
}

// ListLibraryWithProgress returns per-manga chapter stats for the library grid.
func (s *AppService) ListLibraryWithProgress() ([]database.LibraryMangaStats, error) {
	return s.db.ListLibraryWithProgress()
}

// LibraryOverview returns aggregated library-wide stats for the stats row.
func (s *AppService) LibraryOverview() (database.LibraryOverview, error) {
	return s.db.LibraryOverview()
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
func (s *AppService) LastReadChapter(mangaRowID string) (sourceChapterID string, pageNum int, ok bool) {
	return s.db.LastReadChapter(mangaRowID)
}

// QueryMangaPluginIDs returns (manga_id, plugin_id) pairs for all in-library manga.
func (s *AppService) QueryMangaPluginIDs() ([]database.MangaPluginIDRow, error) {
	return s.db.QueryMangaPluginIDs()
}

// PluginDir returns the directory containing the plugin's main file, or "" if
// not found. Folder-based plugins (lua/js/scriggo) have WasmPath pointing at
// the folder itself; wasm plugins have it pointing at the .wasm file.
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
