package pluginmanager

import (
	"testing"

	"goisekai/internal/hostnet"
	"goisekai/pkg/types"
)

// Live test against www.mangahere.cc; skips unless -run mangahere_live.
func TestMangaHereLiveDetail(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	mgr := NewManager(hostnet.NewProxy(), "../../app_data/plugins")
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	detail, err := mgr.GetMangaDetail("mangahere", "kumo_desu_ga_nani_ka")
	if err != nil {
		t.Fatalf("GetMangaDetail: %v", err)
	}
	t.Logf("detail: %+v", detail)
	if detail.Title == "" {
		t.Fatal("empty title")
	}
}

func TestMangaHereLiveChapters(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	mgr := NewManager(hostnet.NewProxy(), "../../app_data/plugins")
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	chapters, err := mgr.GetChapterList("mangahere", "kumo_desu_ga_nani_ka")
	if err != nil {
		t.Fatalf("GetChapterList: %v", err)
	}
	t.Logf("chapters: %d, first: %+v", len(chapters), chapters[0])
	if len(chapters) == 0 {
		t.Fatal("no chapters")
	}
}

func TestMangaHereLivePages(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	mgr := NewManager(hostnet.NewProxy(), "../../app_data/plugins")
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	if _, err := mgr.Search("mangahere", types.SearchFilter{Query: "kumo"}); err != nil {
		t.Logf("search warmup: %v", err)
	}
	pages, err := mgr.GetPageList("mangahere", "kumo_desu_ga_nani_ka:c007.5")
	if err != nil {
		t.Fatalf("GetPageList: %v", err)
	}
	t.Logf("pages: %d", len(pages))
	if len(pages) == 0 {
		t.Fatal("no pages")
	}
}
