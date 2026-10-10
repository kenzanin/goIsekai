// Package bridge is the final service layer binding plugin results and SQLite
// persistence to the frontend. It delegates search/detail/page lookups to the
// plugin manager, mirrors fetched manga into SQLite so progress can be tracked,
// and proxies image fetches through the sandboxed hostnet proxy.
package bridge

import (
	"context"
	"fmt"
	"sync"
	"time"

	"goisekai/internal/database"
	"goisekai/internal/enrich"
	"goisekai/internal/hostnet"
	"goisekai/internal/logger"
	"goisekai/internal/pluginmanager"
	"goisekai/internal/workers"
	"goisekai/pkg/types"
)

// AppService wires the plugin manager, hostnet proxy, and SQLite database into
// the single entry point the frontend calls.
type AppService struct {
	db          *database.DB
	mgr         *pluginmanager.Manager
	proxy       *hostnet.Proxy
	pool        *workers.Pool
	cfgPath     string
	cacheDir    string
	imageMu     sync.RWMutex
	imageCache  map[string][]byte
	imageFlight sync.Map // url -> *imageCall; concurrent readers share one network fetch
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
	shutdownFn  func()
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
		pool:        workers.New(loadWorkerPool(cfgPath)),
		enrich:      enrichReg,
		genres:      loadGenreIndex(cfgPath),
		statusAlias: loadStatusAlias(cfgPath),
		imgFormat:   loadImageFormat(cfgPath),
		coverMaxDim: loadCoverMaxDim(cfgPath),
		enhance:     loadEnhanceConfig(cfgPath),
	}
}

// Shutdown stops the worker pool.
func (s *AppService) Shutdown() {
	s.pool.Shutdown()
}

// TriggerShutdown signals the server to begin graceful shutdown.
// If shutdownFn is set, calls it; otherwise is a no-op.
func (s *AppService) TriggerShutdown() {
	if s.shutdownFn != nil {
		s.shutdownFn()
	}
}

// SetShutdownFn sets the function to call when shutdown is triggered.
func (s *AppService) SetShutdownFn(fn func()) {
	s.shutdownFn = fn
}

// GetPool returns the worker pool.
func (s *AppService) GetPool() *workers.Pool {
	return s.pool
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

// LogEnhanceStatus logs the effective image-enhance configuration at startup.
func (s *AppService) LogEnhanceStatus() {
	plugins := s.mgr.LoadedPlugins()
	ids := make([]string, len(plugins))
	for i, p := range plugins {
		ids[i] = p.ID
	}
	logger.Info(s.enhance.formatEnhanceStatus(ids))
}

// GetConfigPath returns the path to goisekai.ini.
func (s *AppService) GetConfigPath() string {
	return s.cfgPath
}

// GetChapterList fetches the chapter list for a manga from the plugin.
// Rides the interactive lane (task 4.1).
func (s *AppService) GetChapterList(pluginID, mangaID string) ([]types.Chapter, error) {
	var (
		chapters []types.Chapter
		err      error
	)
	runErr := s.runOnInteractive(context.Background(), pluginID, func(context.Context) error {
		chapters, err = s.mgr.GetChapterList(pluginID, mangaID)
		return err
	})
	if runErr != nil {
		return nil, fmt.Errorf("bridge: get chapter list: %w", runErr)
	}
	if err != nil {
		return nil, fmt.Errorf("bridge: get chapter list: %w", err)
	}
	return chapters, nil
}
