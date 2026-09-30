// Command goisekai-tray wraps the goisekai server with a system tray icon
// (XFCE/gtk StatusNotifier via dbus). It spawns the server binary as a child,
// keeps logs in a file under the data dir, and offers tray actions:
// Open Browser / Restart / Quit.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"fyne.io/systray"
)

const (
	repoRoot    = "/home/kenzanin/tmp/goIsekai"
	serverBin   = repoRoot + "/goisekai"
	url         = "http://127.0.0.1:8080/"
	iconPNGPath = repoRoot + "/packaging/desktop/goisekai-icon.png"
	logPath     = repoRoot + "/app_data/tray-server.log"
)

type app struct {
	cmd    *exec.Cmd
	logF   *os.File
	iconB  []byte
	exited chan error
	dead   bool
}

func main() {
	icon, err := os.ReadFile(iconPNGPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read icon: %v\n", err)
		os.Exit(1)
	}
	if _, err := os.Stat(serverBin); err != nil {
		fmt.Fprintf(os.Stderr, "server binary missing: %s (run `just build`)\n", serverBin)
		os.Exit(1)
	}
	a := &app{iconB: icon}
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		systray.Quit()
	}()
	systray.Run(a.onReady, a.onExit)
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
			openBrowser(url)
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
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return err
	}
	logF, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	cmd := exec.Command(serverBin, "-logLevel", "info")
	cmd.Dir = repoRoot
	cmd.Stdout = logF
	cmd.Stderr = logF
	if err := cmd.Start(); err != nil {
		_ = logF.Close() // error path: the Start error is what matters
		return err
	}
	a.cmd, a.logF = cmd, logF
	a.exited = make(chan error, 1)
	go func() { a.exited <- cmd.Wait() }()
	systray.SetTooltip("goIsekai — running at " + url)
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
