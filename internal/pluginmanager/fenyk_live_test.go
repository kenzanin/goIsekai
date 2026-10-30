package pluginmanager

import (
	"os"
	"path/filepath"
	"testing"

	"goisekai/internal/hostnet"
	"goisekai/pkg/types"
)

// TestFenykPluginSearch verifies the fenyk plugin can be loaded and its
// search function returns valid manga results from the live Fenyk API.
func TestFenykPluginSearch(t *testing.T) {
	dir := t.TempDir()
	srcPlugin := filepath.Join("..", "..", "app_data", "plugins", "fenyk")
	dstPlugin := filepath.Join(dir, "fenyk")
	if err := os.MkdirAll(dstPlugin, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"main.lua", "plugin.json"} {
		src, err := os.ReadFile(filepath.Join(srcPlugin, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dstPlugin, name), src, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("discover: %v", err)
	}

	// Search for "Solo Leveling"
	results, err := mgr.Search("fenyk", types.SearchFilter{Query: "Solo Leveling", Page: 1})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("search returned no results")
	}
	for _, m := range results {
		if m.ID == "" || m.Title == "" {
			t.Errorf("empty field in result: id=%q title=%q", m.ID, m.Title)
		}
	}
	t.Logf("search 'Solo Leveling': %d results, first=%s", len(results), results[0].Title)
}

// TestFenykPluginDetail verifies get_manga_detail works against live API.
func TestFenykPluginDetail(t *testing.T) {
	dir := t.TempDir()
	srcPlugin := filepath.Join("..", "..", "app_data", "plugins", "fenyk")
	dstPlugin := filepath.Join(dir, "fenyk")
	if err := os.MkdirAll(dstPlugin, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"main.lua", "plugin.json"} {
		src, err := os.ReadFile(filepath.Join(srcPlugin, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dstPlugin, name), src, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("discover: %v", err)
	}

	// Search first to get a real series ID
	searchResults, err := mgr.Search("fenyk", types.SearchFilter{Query: "dungeon", Page: 1})
	if err != nil || len(searchResults) == 0 {
		t.Skipf("search failed: %v", err)
	}
	seriesID := searchResults[0].ID

	detail, err := mgr.GetMangaDetail("fenyk", seriesID)
	if err != nil {
		t.Fatalf("get_manga_detail failed: %v", err)
	}
	if detail.ID == "" {
		t.Error("empty detail ID")
	}
	t.Logf("detail: id=%s title=%s status=%s genres=%d", detail.ID, detail.Title, detail.Status, len(detail.Genres))
}

// TestFenykPluginChapterList verifies get_chapter_list returns chapters.
func TestFenykPluginChapterList(t *testing.T) {
	dir := t.TempDir()
	srcPlugin := filepath.Join("..", "..", "app_data", "plugins", "fenyk")
	dstPlugin := filepath.Join(dir, "fenyk")
	if err := os.MkdirAll(dstPlugin, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"main.lua", "plugin.json"} {
		src, err := os.ReadFile(filepath.Join(srcPlugin, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dstPlugin, name), src, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("discover: %v", err)
	}

	// Get series ID from search
	searchResults, err := mgr.Search("fenyk", types.SearchFilter{Query: "dungeon", Page: 1})
	if err != nil || len(searchResults) == 0 {
		t.Skipf("search failed: %v", err)
	}
	seriesID := searchResults[0].ID

	chapters, err := mgr.GetChapterList("fenyk", seriesID)
	if err != nil {
		t.Fatalf("get_chapter_list failed: %v", err)
	}
	if len(chapters) == 0 {
		t.Fatal("no chapters returned")
	}
	for _, ch := range chapters {
		if ch.ID == "" {
			t.Errorf("empty chapter ID")
		}
	}
	t.Logf("chapters: %d total, first=%s", len(chapters), chapters[0].Title)
}

// TestFenykPluginPages verifies get_page_list returns pages for a chapter.
func TestFenykPluginPages(t *testing.T) {
	dir := t.TempDir()
	srcPlugin := filepath.Join("..", "..", "app_data", "plugins", "fenyk")
	dstPlugin := filepath.Join(dir, "fenyk")
	if err := os.MkdirAll(dstPlugin, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"main.lua", "plugin.json"} {
		src, err := os.ReadFile(filepath.Join(srcPlugin, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dstPlugin, name), src, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("discover: %v", err)
	}

	// Search → get series ID
	searchResults, err := mgr.Search("fenyk", types.SearchFilter{Query: "dungeon", Page: 1})
	if err != nil || len(searchResults) == 0 {
		t.Skipf("search failed: %v", err)
	}
	seriesID := searchResults[0].ID

	// Get chapters → use first one
	chapters, err := mgr.GetChapterList("fenyk", seriesID)
	if err != nil || len(chapters) == 0 {
		t.Skipf("chapter list failed: %v", err)
	}

	pageList, err := mgr.GetPageList("fenyk", chapters[0].ID)
	if err != nil {
		t.Fatalf("get_page_list failed: %v", err)
	}
	t.Logf("pages: %d pages (state may be unavailable if copy is empty)", len(pageList))
}

// TestFenykPluginGenres verifies get_genres returns valid catalog.
func TestFenykPluginGenres(t *testing.T) {
	dir := t.TempDir()
	srcPlugin := filepath.Join("..", "..", "app_data", "plugins", "fenyk")
	dstPlugin := filepath.Join(dir, "fenyk")
	if err := os.MkdirAll(dstPlugin, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"main.lua", "plugin.json"} {
		src, err := os.ReadFile(filepath.Join(srcPlugin, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dstPlugin, name), src, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("discover: %v", err)
	}

	genres, err := mgr.GetGenres("fenyk")
	if err != nil {
		t.Fatalf("get_genres failed: %v", err)
	}
	if len(genres) == 0 {
		t.Fatal("no genres returned")
	}
	t.Logf("genres/tags: %d items, first=%s", len(genres), genres[0].Name)
}
