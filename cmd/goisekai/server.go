package main

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"goisekai/internal/config"
	"goisekai/internal/logger"
)

// runStop reads the PID file from the data directory and sends SIGTERM. Its
// outcome goes through the same logger the server uses, so `goisekai stop`
// leaves a record in the log path instead of a bare stdout line.
func runStop() {
	cfgPath := os.Getenv("GOISEKAI_CONFIG")
	if cfgPath == "" {
		cfgPath = "goisekai.ini"
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	// The logger needs a level; the config supplies it, same as the server.
	if err := logger.Init(cfg.LogLevel); err != nil {
		log.Fatalf("init logger: %v", err)
	}

	pidPath := filepath.Join(cfg.DataDir, "goisekai.pid")
	data, err := os.ReadFile(pidPath)
	if err != nil {
		logger.Fatal("read PID file", "path", pidPath, "error", err)
	}

	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		logger.Fatal("invalid PID", "value", pidStr, "error", err)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		logger.Fatal("find process", "pid", pid, "error", err)
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		logger.Fatal("send SIGTERM", "pid", pid, "error", err)
	}

	logger.Info("sent SIGTERM", "pid", pid)
}

// applyFlagOverrides copies non-empty CLI flag values over the loaded config.
// Called after config.Load and before anything reads the config, so an unset
// flag never clobbers a value the ini file provided.
func applyFlagOverrides(
	cfg *config.Config,
	host, cdpEngine, cdpPath, apiKey *string,
	port *int,
) {
	if *host != "" {
		cfg.Host = *host
	}
	if *port != 0 {
		cfg.Port = *port
	}
	if *cdpEngine != "" {
		cfg.CDPEngine = *cdpEngine
	}
	if *cdpPath != "" {
		cfg.CDPPath = *cdpPath
	}
	if *apiKey != "" {
		cfg.APIKey = *apiKey
	}
}
