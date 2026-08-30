// Package bridge is the final service layer binding plugin results and SQLite
// persistence to the frontend. It delegates search/detail/page lookups to the
// plugin manager, mirrors fetched manga into SQLite so progress can be tracked,
// and proxies image fetches through the sandboxed hostnet proxy.
package bridge

import (
	"encoding/json"
	"fmt"
	"sync"

	"goisekai/internal/config"
	"goisekai/internal/database"
	"goisekai/internal/hostnet"
	"goisekai/internal/logger"
	"goisekai/internal/pluginmanager"
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

// ReloadConfig re-reads goisekai.ini and returns the new config as JSON.
func (s *AppService) ReloadConfig() (string, error) {
	if s.cfgPath == "" {
		return "", fmt.Errorf("bridge: no config path set")
	}
	cfg, err := config.Load(s.cfgPath)
	if err != nil {
		return "", fmt.Errorf("bridge: reload config: %w", err)
	}
	b, _ := json.Marshal(cfg)
	return string(b), nil
}

// GetConfigPath returns the path to goisekai.ini.
func (s *AppService) GetConfigPath() string {
	return s.cfgPath
}
