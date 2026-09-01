package pluginmanager

import (
	"path/filepath"
	"testing"

	"goisekai/internal/hostnet"
	"goisekai/pkg/types"
)

// jsPluginsDir copies the JS fixture (main.js) into a temp plugins dir.
func jsPluginsDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dst := filepath.Join(dir, "jstest")
	if err := copyDir("testdata/jstest", dst); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	return dir
}

func TestJSPluginDiscoveryAndSearch(t *testing.T) {
	dir := jsPluginsDir(t)
	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	mangas, err := mgr.Search("jstest", types.SearchFilter{Query: "test"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(mangas) != 1 || mangas[0].ID != "J1" || mangas[0].Title != "JS test" {
		t.Fatalf("unexpected Search result: %+v", mangas)
	}

	detail, err := mgr.GetMangaDetail("jstest", "m1")
	if err != nil {
		t.Fatalf("GetMangaDetail: %v", err)
	}
	if detail.Title != "Detail m1" {
		t.Fatalf("unexpected detail: %+v", detail)
	}

	chapters, err := mgr.GetChapterList("jstest", "m1")
	if err != nil {
		t.Fatalf("GetChapterList: %v", err)
	}
	if len(chapters) != 1 || chapters[0].ChapterNum != 1.0 || chapters[0].MangaID != "m1" {
		t.Fatalf("unexpected chapters: %+v", chapters)
	}

	pages, err := mgr.GetPageList("jstest", "m1:c1")
	if err != nil {
		t.Fatalf("GetPageList: %v", err)
	}
	if len(pages) != 1 || pages[0].URL != "https://example.com/img/1.png" {
		t.Fatalf("unexpected pages: %+v", pages)
	}

	// Meta from the PLUGIN object must survive into PluginMeta.
	p := mgr.plugins["jstest"]
	if p.meta.VerifyURL != "https://example.com" || !p.meta.NeedsHumanVerify || p.meta.ThumbRatio != 0.7 {
		t.Fatalf("unexpected meta: %+v", p.meta)
	}
	if p.kind != "js" {
		t.Fatalf("kind = %q, want js", p.kind)
	}
}

func TestJSPluginEmptyQueryReturnsEmpty(t *testing.T) {
	dir := jsPluginsDir(t)
	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	mangas, err := mgr.Search("jstest", types.SearchFilter{Query: ""})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(mangas) != 0 {
		t.Fatalf("want 0 results for empty query, got %d", len(mangas))
	}
}
