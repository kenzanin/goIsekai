package main

import (
	"os"
	"path/filepath"
	"strconv"
	"time"

	"goisekai/internal/config"
	"goisekai/internal/database"
	"goisekai/internal/hostnet"
	"goisekai/internal/logger"
)

// setupDirs ensures the data, plugins, info and cache directories exist.
func setupDirs(cfg *config.Config) (dataDir, pluginsDir, infoDir, cacheDir string) {
	// Data directory holds the SQLite file and the plugins/ wasm directory.
	dataDir = cfg.DataDir
	pluginsDir = filepath.Join(dataDir, "plugins")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		logger.Fatal("mkdir plugins dir", "error", err)
	}
	infoDir = cfg.InfoDir
	if infoDir == "" {
		infoDir = filepath.Join(dataDir, "info")
	}
	if err := os.MkdirAll(infoDir, 0o755); err != nil {
		logger.Fatal("mkdir info dir", "error", err)
	}

	cacheDir = cfg.CacheDir
	if cacheDir == "" {
		cacheDir = filepath.Join(dataDir, "cache")
	}
	if err := os.MkdirAll(filepath.Join(cacheDir, "images"), 0o755); err != nil {
		logger.Fatal("mkdir cache dir", "error", err)
	}
	return
}

// writePIDFile records the process ID so the stop subcommand can find it and
// returns a cleanup function that removes the file.
func writePIDFile(dataDir string) func() {
	pidPath := filepath.Join(dataDir, "goisekai.pid")
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o644); err != nil {
		logger.Warn("write PID file", "error", err)
		return func() {}
	}
	return func() {
		logger.Info("removing PID file", "path", pidPath)
		if err := os.Remove(pidPath); err != nil {
			logger.Warn("remove PID file", "error", err)
		}
	}
}

// setupProxy builds the HTTP proxy with default headers, CDP configuration
// and the persisted TLS-profile pins.
func setupProxy(cfg *config.Config, db *database.DB) *hostnet.Proxy {
	proxy := hostnet.NewProxy()
	proxy.SetDefaultHeader("User-Agent", cfg.UserAgent)
	proxy.SetDefaultHeader("Accept-Language", cfg.AcceptLanguage)
	proxy.SetDefaultHeader("Referer", cfg.Referer)
	proxy.ConfigureCDP(hostnet.CDPConfig{
		Engine:  cfg.CDPEngine,
		Path:    cfg.CDPPath,
		Timeout: time.Duration(cfg.CDPSolveTimeout) * time.Second,
	})

	// Restore persisted TLS-profile pins and wire persistence so a plugin's
	// winning profile survives restarts without a re-probe.
	if pins, perr := db.GetPluginProfiles(); perr == nil {
		proxy.SetPinnedProfiles(pins)
	} else {
		logger.Warn("load plugin profile pins", "error", perr)
	}

	proxy.SetPersistPin(func(pluginID, profile string) {
		if err := db.SetPluginProfile(pluginID, profile); err != nil {
			logger.Warn("persist plugin profile", "plugin", pluginID, "error", err)
		}
	})
	return proxy
}
