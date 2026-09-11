package pluginmanager

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"goisekai/internal/hostnet"
)

// loadYaegiFixture sets up a Manager and loads a Yaegi plugin from testdata.
func loadYaegiFixture(t *testing.T, name string) *Manager {
	t.Helper()
	m := NewManager(hostnet.NewProxy(), "testdata")
	p, err := m.loadYaegi(name, "testdata/"+name)
	if err != nil {
		t.Fatalf("loadYaegi(%s): %v", name, err)
	}
	m.plugins[name] = p
	return m
}

func TestYaegiBasic(t *testing.T) {
	m := loadYaegiFixture(t, "yaegitest")

	out, err := callYaegi(m, m.plugins["yaegitest"], "Search", `"test"`)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if out == "" {
		t.Fatal("Search returned empty result")
	}
	t.Logf("Search result: %s", out)

	out, err = callYaegi(m, m.plugins["yaegitest"], "GetMangaDetail", `"test"`)
	if err != nil {
		t.Fatalf("GetMangaDetail failed: %v", err)
	}
	if out == "" {
		t.Fatal("GetMangaDetail returned empty result")
	}

	out, err = callYaegi(m, m.plugins["yaegitest"], "GetChapterList", `"test"`)
	if err != nil {
		t.Fatalf("GetChapterList failed: %v", err)
	}
	if out == "" {
		t.Fatal("GetChapterList returned empty result")
	}

	out, err = callYaegi(m, m.plugins["yaegitest"], "GetPageList", `"test"`)
	if err != nil {
		t.Fatalf("GetPageList failed: %v", err)
	}
	if out == "" {
		t.Fatal("GetPageList returned empty result")
	}
}

func TestYaegiHTTP(t *testing.T) {
	// Start a local HTTP server for the plugin to call via hostnet.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"ok":true,"path":"%s"}`, r.URL.Path)
	}))
	defer srv.Close()

	mgr := loadYaegiFixture(t, "yaegihttp")
	p := mgr.plugins["yaegihttp"]

	// Extract port from srv.URL (e.g., "http://127.0.0.1:54321")
	port := srv.URL[len("http://127.0.0.1:"):]

	out, err := callYaegi(mgr, p, "Search", port)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	t.Logf("HTTP result: %s", out)
	if out == "" {
		t.Fatal("HTTP returned empty result")
	}
}

func TestYaegiTimeout(t *testing.T) {
	m := loadYaegiFixture(t, "yaegitimeout")

	p := m.plugins["yaegitimeout"]

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := p.yaegi.callWithTimeout(ctx, "Search", "test")
	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}
	t.Logf("Timeout error: %v", err)
}

func TestYaegiPanic(t *testing.T) {
	m := loadYaegiFixture(t, "yaegipanic")

	_, err := callYaegi(m, m.plugins["yaegipanic"], "Search", `"test"`)
	if err == nil {
		t.Fatal("Expected panic error, got nil")
	}
	t.Logf("Panic error: %v", err)
}

func TestYaegiBadPlugin(t *testing.T) {
	m := NewManager(hostnet.NewProxy(), "testdata")

	_, err := m.loadYaegi("yaegibad", "testdata/yaegibad")
	if err == nil {
		t.Fatal("Expected error for bad plugin, got nil")
	}
	t.Logf("Bad plugin error: %v", err)
}

func TestYaegiSandbox(t *testing.T) {
	m := NewManager(hostnet.NewProxy(), "testdata")

	_, err := m.loadYaegi("yaegisandbox", "testdata/yaegisandbox")
	if err == nil {
		t.Fatal("Expected sandbox error, got nil")
	}
	t.Logf("Sandbox error: %v", err)
}
