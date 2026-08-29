//go:build wasip1

// Command dummy-plugin is a reference manga-source plugin for goIsekai. It
// implements the full host/plugin ABI (contract_version, malloc/free, and the
// four JSON-over-memory functions) and serves a small hardcoded catalog so it
// works offline. Use it as the starting point for a real source plugin (e.g. a
// mangadex.org parser): replace the hardcoded catalog with host_http_request
// calls (see fetch below).
//
// Build (from the repo root):
//
//	GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o dummy.wasm ./examples/dummy-plugin
//
// Then install dummy.wasm through the app's Plugins screen (or drop it into
// <data_dir>/plugins/) and restart.
package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unsafe"

	"goisekai/pkg/types"
)

// abiVersion must match types.ContractVersion or the host rejects the plugin.
const abiVersion int32 = 1

// allocations pins malloc'd byte slices so the Go GC does not reclaim memory
// that the host still holds offsets into.
var allocations = map[uint32][]byte{}

//go:wasmexport malloc
func malloc(size uint32) uint32 {
	b := make([]byte, size)
	ptr := uint32(uintptr(unsafe.Pointer(unsafe.SliceData(b))))
	allocations[ptr] = b
	return ptr
}

//go:wasmexport free
func free(ptr uint32) {
	delete(allocations, ptr)
}

//go:wasmexport contract_version
func contractVersion() int32 { return abiVersion }

// readString reinterprets a (ptr, len) pair in WASM linear memory as a Go string.
func readString(ptr, length uint32) string {
	return unsafe.String((*byte)(unsafe.Pointer(uintptr(ptr))), int(length))
}

// readID decodes a host-supplied id. The host JSON-encodes ids
// (json.Marshal(mangaID)), so this unmarshals the input string.
func readID(ptr, length uint32) string {
	var id string
	_ = json.Unmarshal([]byte(readString(ptr, length)), &id)
	return id
}

// returnJSON marshals v and returns the packed (ptr, len) i64 the ABI expects:
// low 32 bits = pointer, high 32 bits = length.
func returnJSON(v any) uint64 {
	b, err := json.Marshal(v)
	if err != nil {
		return 0
	}
	ptr := malloc(uint32(len(b)))
	copy(allocations[ptr], b)
	return uint64(uint32(len(b)))<<32 | uint64(ptr)
}

// hostHTTPRequest is imported from the host ("env" module). It performs all
// network access on the plugin's behalf: the host injects the default headers
// (User-Agent, Accept-Language, Referer), manages a per-plugin cookie jar, and
// enforces the no-direct-socket rule. Plugins must never make their own
// network calls.
//
//go:wasmimport env host_http_request
func hostHTTPRequest(ptr, length uint32) uint64

// unpack splits the host's packed (ptr, len) i64 into its parts.
func unpack(packed uint64) (ptr, length uint32) {
	return uint32(packed), uint32(packed >> 32)
}

// fetch performs an HTTP GET through the host proxy and decodes the response.
// It is the reference pattern for a real source plugin: build an HTTPRequest,
// marshal it, call host_http_request, then unpack and unmarshal the result.
// The dummy catalog below is hardcoded, so fetch is not called at runtime —
// swap a catalog lookup for a fetch call to pull live data from a site.
func fetch(url string) (*types.HTTPResponse, error) {
	b, err := json.Marshal(types.HTTPRequest{Method: "GET", URL: url})
	if err != nil {
		return nil, err
	}
	inPtr := malloc(uint32(len(b)))
	copy(allocations[inPtr], b)
	defer free(inPtr)

	outPtr, outLen := unpack(hostHTTPRequest(inPtr, uint32(len(b))))
	if outPtr == 0 || outLen == 0 {
		return nil, fmt.Errorf("host_http_request returned empty result")
	}
	defer free(outPtr)

	var resp types.HTTPResponse
	if err := json.Unmarshal([]byte(readString(outPtr, outLen)), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// releasedAt is a fixed timestamp for the dummy chapters (time.Date avoids the
// wasip1 wall-clock, which is not guaranteed in the sandbox).
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

// chaptersFor returns three dummy chapters for a manga.
func chaptersFor(mangaID string) []types.Chapter {
	return []types.Chapter{
		{ID: mangaID + "-ch1", MangaID: mangaID, Title: "Chapter 1", ChapterNum: 1, ReleasedAt: releasedAt, URL: "https://example.com/" + mangaID + "/1"},
		{ID: mangaID + "-ch2", MangaID: mangaID, Title: "Chapter 2", ChapterNum: 2, ReleasedAt: releasedAt, URL: "https://example.com/" + mangaID + "/2"},
		{ID: mangaID + "-ch3", MangaID: mangaID, Title: "Chapter 3", ChapterNum: 3, ReleasedAt: releasedAt, URL: "https://example.com/" + mangaID + "/3"},
	}
}

// pagesFor returns n placeholder image pages for a chapter.
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

// findManga returns the catalog entry with the given id, or nil.
func findManga(id string) *types.Manga {
	for i := range catalog {
		if catalog[i].ID == id {
			return &catalog[i]
		}
	}
	return nil
}

//go:wasmexport Search
func Search(ptr, length uint32) uint64 {
	var f types.SearchFilter
	_ = json.Unmarshal([]byte(readString(ptr, length)), &f)
	q := strings.ToLower(strings.TrimSpace(f.Query))

	var results []types.Manga
	for _, m := range catalog {
		if q == "" || strings.Contains(strings.ToLower(m.Title), q) {
			results = append(results, m)
		}
	}
	return returnJSON(results)
}

//go:wasmexport GetMangaDetail
func GetMangaDetail(ptr, length uint32) uint64 {
	if m := findManga(readID(ptr, length)); m != nil {
		return returnJSON(*m)
	}
	return returnJSON(types.Manga{})
}

//go:wasmexport GetChapterList
func GetChapterList(ptr, length uint32) uint64 {
	id := readID(ptr, length)
	if findManga(id) == nil {
		return returnJSON([]types.Chapter{})
	}
	return returnJSON(chaptersFor(id))
}

//go:wasmexport GetPageList
func GetPageList(ptr, length uint32) uint64 {
	return returnJSON(pagesFor(readID(ptr, length), 8))
}

func main() {}
