//go:build wasip1

// Package main is a minimal test plugin for the pluginmanager package. It
// implements the Extism PDK ABI so the host can exercise the runtime round-trip.
package main

import (
	"encoding/json"

	"github.com/extism/go-pdk"

	"goisekai/pkg/types"
)

//go:wasmexport contract_version
func contractVersion() int32 {
	pdk.OutputString("1")
	return 0
}

//go:wasmexport Search
func Search() int32 {
	var f types.SearchFilter
	_ = json.Unmarshal(pdk.Input(), &f)
	b, _ := json.Marshal([]types.Manga{
		{ID: "m1", Title: "Test Manga", CoverURL: "https://example.com/cover.jpg"},
	})
	pdk.Output(b)
	return 0
}

//go:wasmexport GetMangaDetail
func GetMangaDetail() int32 {
	_ = pdk.Input()
	b, _ := json.Marshal(types.Manga{ID: "m1", Title: "Test Manga", CoverURL: "https://example.com/cover.jpg"})
	pdk.Output(b)
	return 0
}

//go:wasmexport GetChapterList
func GetChapterList() int32 {
	_ = pdk.Input()
	b, _ := json.Marshal([]types.Chapter{
		{ID: "c1", MangaID: "m1", Title: "Ch 1", ChapterNum: 1},
	})
	pdk.Output(b)
	return 0
}

//go:wasmexport GetPageList
func GetPageList() int32 {
	_ = pdk.Input()
	b, _ := json.Marshal([]types.Page{
		{Index: 0, URL: "https://example.com/1.jpg"},
		{Index: 1, URL: "https://example.com/2.jpg"},
	})
	pdk.Output(b)
	return 0
}

func main() {}
