package pluginmanager

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goisekai/internal/hostnet"
	"goisekai/pkg/types"
)

func TestLuaHTTPGet(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.Header().Set("X-Custom", "hello")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	dir := t.TempDir()
	if err := copyDir("testdata/luahost", filepath.Join(dir, "luahost")); err != nil {
		t.Fatal(err)
	}

	pluginCode := `PLUGIN = { contract_version = 1, name = "Lua HTTP test" }

function get_manga_detail(a)
	local ok, resp = pcall(function()
		return host.http.get("` + srv.URL + `/test")
	end)
	if not ok then return string.format('{"id":"H1","title":"payload","description":"err:%s"}', tostring(resp)) end
	if not resp then return string.format('{"id":"H1","title":"payload","description":"nil resp"}') end
	-- Escape quotes in body for valid JSON
	local safeBody = string.gsub(resp.body, '"', '\\"')
	return string.format('{"id":"H1","title":"payload","description":"%d|%s|%s"}', resp.status, resp.headers["X-Custom"], safeBody)
end
function search_manga(a) return "[]" end
function get_chapter_list(a) return "[]" end
function get_page_list(a) return "[]" end`

	if err := os.WriteFile(filepath.Join(dir, "luahost", "main.lua"), []byte(pluginCode), 0o644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	// Get the raw plugin call output for debugging
	p, _ := mgr.get("luahost")
	if p != nil {
		in, _ := json.Marshal("m1")
		out, cerr := mgr.call(p, types.GetMangaDetailFunc, string(in))
		t.Logf("raw plugin output: %q", out)
		t.Logf("call error: %v", cerr)
	}

	d, err := mgr.GetMangaDetail("luahost", "m1")
	if err != nil {
		t.Fatalf("GetMangaDetail: %v", err)
	}
	t.Logf("got description: %q", d.Description)
	expected := `200|hello|{"ok":true}`
	if got := strings.TrimSpace(d.Description); got != expected {
		t.Fatalf("lua http.get mismatch:\n got %q\nwant %q", got, expected)
	}
}

func TestLuaHTTPPost(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	dir := t.TempDir()
	if err := copyDir("testdata/luahost", filepath.Join(dir, "luahost")); err != nil {
		t.Fatal(err)
	}

	pluginCode := `PLUGIN = { contract_version = 1, name = "Lua HTTP post test" }

function get_manga_detail(a)
	local resp = host.http.post("` + srv.URL + `/echo", "hello-world", {Content_Type = "text/plain"})
	return string.format('{"id":"H1","title":"payload","description":"%d|%s"}', resp.status, resp.body)
end
function search_manga(a) return "[]" end
function get_chapter_list(a) return "[]" end
function get_page_list(a) return "[]" end`

	if err := os.WriteFile(filepath.Join(dir, "luahost", "main.lua"), []byte(pluginCode), 0o644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	d, err := mgr.GetMangaDetail("luahost", "m1")
	if err != nil {
		t.Fatalf("GetMangaDetail: %v", err)
	}
	expected := `200|hello-world`
	if got := strings.TrimSpace(d.Description); got != expected {
		t.Fatalf("lua http.post mismatch:\n got %q\nwant %q", got, expected)
	}
}

func TestJSHTTPGet(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.Header().Set("X-Custom", "hello")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	dir := t.TempDir()
	if err := copyDir("testdata/jshost", filepath.Join(dir, "jshost")); err != nil {
		t.Fatal(err)
	}

	pluginCode := `var PLUGIN = { contract_version: 1, name: "JS HTTP test" };

function getMangaDetail(a) {
	var resp = host.http.get("` + srv.URL + `/test");
	return JSON.stringify({id: "H1", title: "payload", description: JSON.stringify({s: resp.status, h: resp.headers["X-Custom"], b: resp.body})});
}
function searchManga(a) { return "[]"; }
function getChapterList(a) { return "[]"; }
function getPageList(a) { return "[]"; }`

	if err := os.WriteFile(filepath.Join(dir, "jshost", "main.js"), []byte(pluginCode), 0o644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	d, err := mgr.GetMangaDetail("jshost", "m1")
	if err != nil {
		t.Fatalf("GetMangaDetail: %v", err)
	}
	expected := `{"s":200,"h":"hello","b":"{\"ok\":true}"}`
	if got := strings.TrimSpace(d.Description); got != expected {
		t.Fatalf("js http.get mismatch:\n got %q\nwant %q", got, expected)
	}
}
