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
	"net"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
)

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
