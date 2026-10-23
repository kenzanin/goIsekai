// Command goisekai-tray wraps the goisekai server with a system tray icon
// (XFCE/gtk StatusNotifier via dbus). It spawns the server binary as a child,
// keeps logs in a file under the data dir, and offers tray actions:
// Open Browser / Restart / Quit.
//
// Settings come from goisekai.ini ([tray] section — all optional, see
// config.Tray*), so a deployment moves paths and URLs around by editing the
// ini instead of rebuilding this binary. Empty keys derive defaults: server
// binary next to the tray, URL from the server's host/port, log under
// data_dir, icon from packaging/desktop next to the tray.
package main

import (
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"goisekai/internal/config"
	"goisekai/internal/logger"

	"fyne.io/systray"
)

type app struct {
	cmd    *exec.Cmd
	logF   *os.File
	iconB  []byte
	exited chan error
	dead   bool

	serverBin string // server binary to spawn ([tray] server_bin)
	qmlBin    string // desktop client; derived from serverBin's directory
	workDir   string // child cwd; the server resolves goisekai.ini relative to it
	url       string // Open Browser target ([tray] url)
	logPath   string // child stdout/stderr ([tray] log_file)
}

func main() {
	exe, err := os.Executable()
	if err != nil {
		logger.Fatal("locate executable", "error", err)
	}
	exeDir := filepath.Dir(exe)

	// Read the same goisekai.ini the server will see: $GOISEKAI_CONFIG when
	// set, else the file next to this binary (the child's cwd is the server
	// binary dir, so the server resolves plain "goisekai.ini" the same way).
	cfgPath := os.Getenv("GOISEKAI_CONFIG")
	if cfgPath == "" {
		cfgPath = filepath.Join(exeDir, "goisekai.ini")
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		logger.Warn("load config, using defaults", "path", cfgPath, "error", err)
		cfg = config.Default()
	}

	a := &app{serverBin: cfg.TrayServerBin}
	if a.serverBin == "" {
		a.serverBin = filepath.Join(exeDir, "goisekai")
	}
	a.workDir = filepath.Dir(a.serverBin)
	// No separate config key: the desktop client ships next to the server
	// binary, so moving server_bin moves this too.
	a.qmlBin = filepath.Join(a.workDir, "goisekai-qt6")
	a.url = cfg.TrayURL
	if a.url == "" {
		a.url = resolveURL(cfg.Host, cfg.Port)
	}
	a.logPath = cfg.TrayLogFile
	if a.logPath == "" {
		a.logPath = filepath.Join(resolveDir(cfg.DataDir, a.workDir), "tray-server.log")
	}
	iconPath := cfg.TrayIcon
	if iconPath == "" {
		iconPath = filepath.Join(exeDir, "packaging", "desktop", "goisekai-icon.png")
	}

	icon, err := os.ReadFile(iconPath)
	if err != nil {
		logger.Fatal("read icon", "path", iconPath, "error", err)
	}
	if _, err := os.Stat(a.serverBin); err != nil {
		logger.Fatal("server binary missing (run `just build`)", "path", a.serverBin)
	}
	a.iconB = icon
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		systray.Quit()
	}()
	systray.Run(a.onReady, a.onExit)
}
