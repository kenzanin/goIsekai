package pluginmanager

import (
	"path/filepath"
	"strings"
	"testing"

	"goisekai/internal/hostnet"
)

// wantHostPayload is every host native's output for the shared fixture input,
// joined by "|". Both the Lua and JS fixtures must produce exactly this.
const wantHostPayload = "a%20b|a b|&|hi|bold x|Abc|aGk=|hi|YT9i|6869|hi|" +
	"ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad|" +
	"900150983cd24fb0d6963f7d28e17f72|" +
	"f7bc83f430538424b13298e6aa6fb143ef4d59a14946175997479dbc2d1a3cd8|" +
	"0206"

func TestLuaHostNatives(t *testing.T) {
	dir := t.TempDir()
	if err := copyDir("testdata/luahost", filepath.Join(dir, "luahost")); err != nil {
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
	if got := strings.TrimSpace(d.Description); got != wantHostPayload {
		t.Fatalf("lua host payload mismatch:\n got %q\nwant %q", got, wantHostPayload)
	}
}

func TestJSHostNatives(t *testing.T) {
	dir := t.TempDir()
	if err := copyDir("testdata/jshost", filepath.Join(dir, "jshost")); err != nil {
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
	if got := strings.TrimSpace(d.Description); got != wantHostPayload {
		t.Fatalf("js host payload mismatch:\n got %q\nwant %q", got, wantHostPayload)
	}
}
