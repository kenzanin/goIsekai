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
	clientBin string // desktop client to launch ([tray] client_wails — the Wails webview wrapper)
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
	// [tray] client_wails: explicit path to the Wails webview wrapper. Empty
	// derives bin/goisekai-wails next to the server binary (where
	// `just build-wails` puts it), falling back to a bare goisekai-wails beside
	// the server for deployments that flatten the layout.
	a.clientBin = cfg.TrayClientWails
	if a.clientBin == "" {
		underBin := filepath.Join(a.workDir, "bin", "goisekai-wails")
		beside := filepath.Join(a.workDir, "goisekai-wails")
		if _, err := os.Stat(underBin); err == nil {
			a.clientBin = underBin
		} else {
			a.clientBin = beside
		}
		logger.Info("client_wails not set, deriving", "path", a.clientBin)
	} else {
		a.clientBin = resolveDir(a.clientBin, a.workDir)
		logger.Info("client_wails from config", "raw", cfg.TrayClientWails, "path", a.clientBin)
	}
	// Resolve once here so the menu item's state and the launch agree: a path
	// that exists only after a later rebuild would otherwise leave the entry
	// disabled for the rest of the session.
	if fi, err := os.Stat(a.clientBin); err != nil {
		logger.Warn("desktop client missing, tray entry disabled",
			"path", a.clientBin, "error", err, "hint", "run `just build-wails`")
	} else if fi.Mode()&0o111 == 0 {
		logger.Warn("desktop client is not executable",
			"path", a.clientBin, "mode", fi.Mode().String())
	} else {
		logger.Info("desktop client ready", "path", a.clientBin, "size", fi.Size())
	}
	a.url = cfg.TrayURL
	if a.url == "" {
		a.url = resolveURL(cfg.Host, cfg.Port)
	}
	a.logPath = cfg.TrayLogFile
	if a.logPath == "" {
		a.logPath = filepath.Join(resolveDir(cfg.DataDir, a.workDir), "tray-server.log")
	}
	// Route this process's own logs into the same file the server child writes
	// to. A tray started by a desktop launcher has no terminal, so stderr would
	// be lost and a failed client launch would leave no trace. os.Stderr is the
	// stream logger.Init captures, so reassigning it is enough.
	if err := os.MkdirAll(filepath.Dir(a.logPath), 0o755); err == nil {
		if f, err := os.OpenFile(a.logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
			os.Stderr = f
			_ = f.Close()
			os.Stderr = os.NewFile(2, "/dev/stderr") // restore stderr
		}
	}
	if err := logger.Init(cfg.LogLevel); err != nil {
		logger.Warn("invalid log_level, falling back to info", "value", cfg.LogLevel, "error", err)
		_ = logger.Init("info")
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
