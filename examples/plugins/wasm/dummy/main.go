//go:build wasip1

// Command dummy-plugin is a reference manga-source plugin for goIsekai. It
// implements the Extism PDK ABI and serves a small hardcoded catalog so it
// works offline. Use it as the starting point for a real source plugin: replace
// the hardcoded catalog with host_http_request calls.
//
// Build (from this directory):
//
//	GOOS=wasip1 GOARCH=wasm go build -o dummy.wasm .
//
// Then install dummy.wasm through the app's Plugins screen (or drop it into
// <data_dir>/plugins/) and restart.
package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/extism/go-pdk"

	"goisekai/pkg/types"
)

//go:wasmexport contract_version
func contractVersion() int32 {
	pdk.OutputString("1")
	return 0
}

var releasedAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// catalog is the dummy source's hardcoded library.
var catalog = []types.Manga{
	{
		ID:          "dummy-solo",
		Title:       "Solo Leveling Clone",
		CoverURL:    "https://picsum.photos/seed/dummy-solo/400/560",
		Author:      "Dummy Author",
		Description: "A dummy isekai action series used as a plugin reference.",
		Status:      "ongoing",
		Genres:      []string{"action", "fantasy", "isekai"},
	},
	{
		ID:          "dummy-romance",
		Title:       "My Dummy Girlfriend",
		CoverURL:    "https://picsum.photos/seed/dummy-romance/400/560",
		Author:      "Dummy Author",
		Description: "A dummy slice-of-life romance series.",
		Status:      "completed",
		Genres:      []string{"romance", "slice of life"},
	},
	{
		ID:          "dummy-horror",
		Title:       "The Dummy Below",
		CoverURL:    "https://picsum.photos/seed/dummy-horror/400/560",
		Author:      "Dummy Author",
		Description: "A dummy horror/mystery series.",
		Status:      "ongoing",
		Genres:      []string{"horror", "mystery"},
	},
}

func chaptersFor(mangaID string) []types.Chapter {
	return []types.Chapter{
		{ID: mangaID + ":chapter-1", MangaID: mangaID, Title: "Chapter 1", ChapterNum: 1, ReleasedAt: releasedAt, URL: "https://example.com/" + mangaID + "/1"},
		{ID: mangaID + ":chapter-2", MangaID: mangaID, Title: "Chapter 2", ChapterNum: 2, ReleasedAt: releasedAt, URL: "https://example.com/" + mangaID + "/2"},
		{ID: mangaID + ":chapter-3", MangaID: mangaID, Title: "Chapter 3", ChapterNum: 3, ReleasedAt: releasedAt, URL: "https://example.com/" + mangaID + "/3"},
	}
}

func pagesFor(chapterID string, n int) []types.Page {
	pages := make([]types.Page, n)
	for i := range pages {
		pages[i] = types.Page{
			Index: i,
			URL:   fmt.Sprintf("https://picsum.photos/seed/%s-%d/800/1200", chapterID, i),
		}
	}
	return pages
}

func findManga(id string) *types.Manga {
	for i := range catalog {
		if catalog[i].ID == id {
			return &catalog[i]
		}
	}
	return nil
}

//go:wasmexport Search
func Search() int32 {
	var f types.SearchFilter
	_ = json.Unmarshal(pdk.Input(), &f)
	q := strings.ToLower(strings.TrimSpace(f.Query))

	var results []types.Manga
	for _, m := range catalog {
		if q == "" || strings.Contains(strings.ToLower(m.Title), q) {
			results = append(results, m)
		}
	}
	b, _ := json.Marshal(results)
	pdk.Output(b)
	return 0
}

//go:wasmexport GetMangaDetail
func GetMangaDetail() int32 {
	var id string
	_ = json.Unmarshal(pdk.Input(), &id)
	if m := findManga(id); m != nil {
		b, _ := json.Marshal(*m)
		pdk.Output(b)
	} else {
		b, _ := json.Marshal(types.Manga{})
		pdk.Output(b)
	}
	return 0
}

//go:wasmexport GetChapterList
func GetChapterList() int32 {
	var id string
	_ = json.Unmarshal(pdk.Input(), &id)
	if findManga(id) == nil {
		b, _ := json.Marshal([]types.Chapter{})
		pdk.Output(b)
	} else {
		b, _ := json.Marshal(chaptersFor(id))
		pdk.Output(b)
	}
	return 0
}

//go:wasmexport GetPageList
func GetPageList() int32 {
	var id string
	_ = json.Unmarshal(pdk.Input(), &id)
	b, _ := json.Marshal(pagesFor(id, 8))
	pdk.Output(b)
	return 0
}

func main() {}
