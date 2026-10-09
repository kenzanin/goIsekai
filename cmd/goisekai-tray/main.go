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
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"fyne.io/systray"
	"goisekai/internal/config"
)

type app struct {
	cmd    *exec.Cmd
	logF   *os.File
	iconB  []byte
	exited chan error
	dead   bool

	serverBin string // server binary to spawn ([tray] server_bin)
	workDir   string // child cwd; the server resolves goisekai.ini relative to it
	url       string // Open Browser target ([tray] url)
	logPath   string // child stdout/stderr ([tray] log_file)
}

func main() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "locate executable: %v\n", err)
		os.Exit(1)
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
		fmt.Fprintf(os.Stderr, "load %s: %v (using defaults)\n", cfgPath, err)
		cfg = config.Default()
	}

	a := &app{serverBin: cfg.TrayServerBin}
	if a.serverBin == "" {
		a.serverBin = filepath.Join(exeDir, "goisekai")
	}
	a.workDir = filepath.Dir(a.serverBin)
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
		fmt.Fprintf(os.Stderr, "read icon: %v\n", err)
		os.Exit(1)
	}
	if _, err := os.Stat(a.serverBin); err != nil {
		fmt.Fprintf(os.Stderr, "server binary missing: %s (run `just build`)\n", a.serverBin)
		os.Exit(1)
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

// resolveURL builds the browser target from the server's bind address. A
// bind-all address cannot be opened as-is, so it loops back to 127.0.0.1.
func resolveURL(host string, port int) string {
	if port <= 0 {
		port = 8080
	}
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, strconv.Itoa(port)) + "/"
}

// resolveDir makes a possibly relative config path absolute against base (the
// child's working directory), the same way the server resolves it.
func resolveDir(path, base string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(base, path)
}

func (a *app) onReady() {
	systray.SetIcon(a.iconB)
	systray.SetTitle("goIsekai")
	systray.SetTooltip("goIsekai — manga server")

	mOpen := systray.AddMenuItem("Open Browser", "Open the reader UI")
	mRestart := systray.AddMenuItem("Restart Server", "Stop and start the server")
	mQuit := systray.AddMenuItem("Quit", "Stop server and exit tray")

	if err := a.start(); err != nil {
		systray.SetTooltip("goIsekai — FAILED to start: " + err.Error())
	}

	for {
		select {
		case <-mOpen.ClickedCh:
			openBrowser(a.url)
		case <-mRestart.ClickedCh:
			a.stop()
			time.Sleep(500 * time.Millisecond)
			if err := a.start(); err != nil {
				systray.SetTooltip("goIsekai — FAILED: " + err.Error())
			}
		case err, ok := <-a.exited:
			if ok && err != nil && !a.dead {
				systray.SetTooltip("goIsekai — server exited: " + err.Error())
			}
		case <-mQuit.ClickedCh:
			a.dead = true
			a.stop()
			systray.Quit()
			return
		}
	}
}

func (a *app) start() error {
	if err := os.MkdirAll(filepath.Dir(a.logPath), 0o755); err != nil {
		return err
	}
	logF, err := os.OpenFile(a.logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	// No -logLevel flag: the server reads log_level from goisekai.ini itself
	// (flag > file > default). Everything config-worthy lives in the ini.
	cmd := exec.Command(a.serverBin)
	cmd.Dir = a.workDir
	cmd.Stdout = logF
	cmd.Stderr = logF
	if err := cmd.Start(); err != nil {
		_ = logF.Close() // error path: the Start error is what matters
		return err
	}
	a.cmd, a.logF = cmd, logF
	a.exited = make(chan error, 1)
	go func() { a.exited <- cmd.Wait() }()
	systray.SetTooltip("goIsekai — running at " + a.url)
	return nil
}

func (a *app) stop() {
	if a.cmd == nil || a.cmd.Process == nil {
		return
	}
	_ = a.cmd.Process.Signal(syscall.SIGTERM)
	done := make(chan struct{})
	go func() { _, _ = a.cmd.Process.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		_ = a.cmd.Process.Kill()
		<-done
	}
	select {
	case <-a.exited:
	default:
	}
	_ = a.logF.Close()
	a.cmd, a.logF = nil, nil
}

func (a *app) onExit() { a.stop() }

func openBrowser(u string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", u)
	default:
		return
	}
	_ = cmd.Start()
}
