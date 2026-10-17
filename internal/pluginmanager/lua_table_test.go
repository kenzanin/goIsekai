package pluginmanager

import (
	"testing"

	"goisekai/internal/hostnet"
	"goisekai/pkg/types"
)

// The Lua ABI accepts a table as well as a JSON string: the host marshals a
// returned table to JSON itself. This is the lunarTableToJSON path, and it is
// what lets a plugin build results with Lua literals instead of hand-rolling
// JSON text.
func TestLuaTableReturnIsMarshalledToJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writePluginFolder(t, dir, "tableret", "main.lua", `
function search(a)
  return {
    { id = "T1", title = "table title", status = "Ongoing" },
    { id = "T2", title = "second" },
  }
end
function get_manga_detail(a)
  local id = host.json.decode(a)
  return { id = id, title = "table detail" }
end
function get_chapter_list(a)
  local id = host.json.decode(a)
  return { { id = id .. ":c1", chapter_num = 1.5 } }
end
function get_page_list(a)
  local id = host.json.decode(a)
  return { { index = 1, url = "https://example.test/1.jpg" } }
end
`)
	mgr := NewManager(hostnet.NewProxy(), dir)
	if err := mgr.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	defer func() { _ = mgr.Close() }()

	mangas, err := mgr.Search("tableret", types.SearchFilter{Query: "x"})
	if err != nil {
		t.Fatalf("Search (table return): %v", err)
	}
	if len(mangas) != 2 || mangas[0].ID != "T1" || mangas[0].Title != "table title" || mangas[0].Status != "Ongoing" {
		t.Fatalf("unexpected mangas from table return: %+v", mangas)
	}

	detail, err := mgr.GetMangaDetail("tableret", "m9")
	if err != nil {
		t.Fatalf("GetMangaDetail (table return): %v", err)
	}
	if detail.ID != "m9" || detail.Title != "table detail" {
		t.Fatalf("unexpected detail from table return: %+v", detail)
	}

	chapters, err := mgr.GetChapterList("tableret", "m9")
	if err != nil {
		t.Fatalf("GetChapterList (table return): %v", err)
	}
	if len(chapters) != 1 || chapters[0].ChapterNum != 1.5 {
		t.Fatalf("unexpected chapters from table return: %+v", chapters)
	}

	pages, err := mgr.GetPageList("tableret", "m9:c1")
	if err != nil {
		t.Fatalf("GetPageList (table return): %v", err)
	}
	if len(pages) != 1 || pages[0].URL != "https://example.test/1.jpg" {
		t.Fatalf("unexpected pages from table return: %+v", pages)
	}
}
