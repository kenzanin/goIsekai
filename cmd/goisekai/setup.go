package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"goisekai/internal/config"
	"goisekai/internal/database"
	"goisekai/internal/hostnet"
	"goisekai/internal/logger"
)

// checkSingleInstance fatals if another live goisekai process holds the same
// host:port. A stale PID file (process already dead) or a different port
// proceed normally.
func checkSingleInstance(dataDir string, host string, port int) {
	pidPath := filepath.Join(dataDir, "goisekai.pid")
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return // no PID file — first instance
	}
	pid, parseErr := strconv.Atoi(strings.TrimSpace(string(data)))
	if parseErr != nil || pid <= 0 || pid == os.Getpid() {
		return // unparseable or our own — proceed (overwrite the stale file)
	}
	// Is the other process alive?
	if proc, err := os.FindProcess(pid); err == nil {
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			return // process not alive — stale PID file, proceed
		}
		// Alive. Compare the port it was launched with (-port / config),
		// via /proc/<pid>/cmdline + its config. Different port → allow.
		if cmdlinePort, ok := cmdlinePortOf(pid); ok && cmdlinePort == port {
			otherHost := host
			if h, ok := cmdlineFlag(pid, "-host"); ok {
				otherHost = h
			}
			logger.Fatal(fmt.Sprintf("goIsekai is already running (pid %d) on %s:%d — use a different -port, or stop it first with: goisekai stop",
				pid, otherHost, port))
		}
	}
}

// cmdlinePortOf returns the port a running goisekai process serves on: its
// -port flag if set, otherwise the port of the config file it loads
// (GOISEKAI_CONFIG env var, else ./goisekai.ini). ok=false when either is
// undetermined — the caller then falls through to a port collision on bind.
func cmdlinePortOf(pid int) (port int, ok bool) {
	if p, ok := cmdlineFlag(pid, "-port"); ok {
		if n, err := strconv.Atoi(p); err == nil {
			return n, true
		}
		return 0, false
	}
	// No -port flag: parse the config the process would load. The data dir
	// default is <wd>/data, so read its goisekai.ini.
	for _, path := range []string{"goisekai.ini", filepath.Join("..", "goisekai.ini")} {
		cfgPath := absFromProc(pid, path)
		if cfgPath == "" {
			continue
		}
		if p, ok := configPortFromIni(cfgPath); ok {
			return p, true
		}
	}
	return 0, false
}

// cmdlineFlag returns the value of flag f from /proc/<pid>/cmdline. Flags may
// appear as "-flag value" or "-flag=value".
func cmdlineFlag(pid int, f string) (string, bool) {
	raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return "", false
	}
	args := strings.Fields(string(raw))
	for i, a := range args {
		if a == f && i+1 < len(args) {
			v := args[i+1]
			if f == "-port" {
				if _, err := strconv.Atoi(v); err == nil {
					return v, true
				}
				return "", false
			}
			return v, true
		}
		if v, found := strings.CutPrefix(a, f+"="); found {
			if f == "-port" {
				if _, err := strconv.Atoi(v); err == nil {
					return v, true
				}
				return "", false
			}
			return v, true
		}
	}
	return "", false
}

// absFromProc resolves a relative ini path against the process's own working
// directory (/proc/<pid>/cwd), empty when it cannot.
func absFromProc(pid int, rel string) string {
	cwd, err := os.Readlink(fmt.Sprintf("/proc/%d/cwd", pid))
	if err != nil {
		return ""
	}
	return filepath.Join(cwd, rel)
}

// configPortFromIni reads [server] port= from an ini file. The config package
// is intentionally not imported here: the running process may be built from a
// different checkout, and parsing one key with os is cheaper than the
// whole-config load.
func configPortFromIni(path string) (int, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if ok && strings.TrimSpace(k) == "port" {
			if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				return n, true
			}
		}
	}
	return 0, false
}

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
	proxy.SetSecCHUA(cfg.SecCHUA)
	proxy.ConfigureCDP(hostnet.CDPConfig{
		Engine:  cfg.CDPEngine,
		Path:    cfg.CDPPath,
		Timeout: time.Duration(cfg.CDPSolveTimeout) * time.Second,
		// Fallback daemons need their own ws:// endpoint; without this a
		// fallback engine inherits the chrome binary path and cannot connect.
		Paths: map[string]string{
			"obscura":    cfg.CDPObscuraPath,
			"lightpanda": cfg.CDPLightpandaPath,
		},
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
