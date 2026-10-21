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
	"path/filepath"
	"syscall"
	"time"

	"fyne.io/systray"
)

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
