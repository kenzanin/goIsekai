package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

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
