package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"goisekai/internal/config"
)

// runStop reads the PID file from the data directory and sends SIGTERM.
func runStop() {
	cfgPath := os.Getenv("GOISEKAI_CONFIG")
	if cfgPath == "" {
		cfgPath = "goisekai.ini"
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	pidPath := filepath.Join(cfg.DataDir, "goisekai.pid")
	data, err := os.ReadFile(pidPath)
	if err != nil {
		log.Fatalf("read PID file: %v", err)
	}

	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		log.Fatalf("invalid PID %q: %v", pidStr, err)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		log.Fatalf("find process %d: %v", pid, err)
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		log.Fatalf("send SIGTERM to %d: %v", pid, err)
	}

	fmt.Printf("sent SIGTERM to process %d\n", pid)
}
