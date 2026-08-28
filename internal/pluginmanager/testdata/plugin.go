//go:build wasip1

// Package main is a minimal test plugin for the pluginmanager package. It
// implements the full ABI contract (contract_version, malloc/free, and the four
// JSON-over-memory functions) so the host can exercise the runtime round-trip.
package main

import (
	"encoding/json"
	"unsafe"

	"goisekai/pkg/types"
)

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

// readString reinterprets a (ptr, len) pair in WASM linear memory as a Go
// string. Safe because every pointer handed to the host originated from malloc,
// which pins the backing slice in allocations.
func readString(ptr, length uint32) string {
	return unsafe.String((*byte)(unsafe.Pointer(uintptr(ptr))), int(length))
}

// returnJSON marshals v and returns the packed (ptr, len) i64 the ABI expects.
func returnJSON(v any) uint64 {
	b, err := json.Marshal(v)
	if err != nil {
		return uint64(0)<<32 | uint64(0)
	}
	ptr := malloc(uint32(len(b)))
	copy(allocations[ptr], b)
	return uint64(uint32(len(b)))<<32 | uint64(ptr)
}

//go:wasmexport Search
func Search(ptr, length uint32) uint64 {
	var f types.SearchFilter
	_ = json.Unmarshal([]byte(readString(ptr, length)), &f)
	return returnJSON([]types.Manga{
		{ID: "m1", Title: "Test Manga", CoverURL: "https://example.com/cover.jpg"},
	})
}

//go:wasmexport GetMangaDetail
func GetMangaDetail(ptr, length uint32) uint64 {
	_ = readString(ptr, length)
	return returnJSON(types.Manga{ID: "m1", Title: "Test Manga", CoverURL: "https://example.com/cover.jpg"})
}

//go:wasmexport GetChapterList
func GetChapterList(ptr, length uint32) uint64 {
	_ = readString(ptr, length)
	return returnJSON([]types.Chapter{
		{ID: "c1", MangaID: "m1", Title: "Ch 1", ChapterNum: 1},
	})
}

//go:wasmexport GetPageList
func GetPageList(ptr, length uint32) uint64 {
	_ = readString(ptr, length)
	return returnJSON([]types.Page{
		{Index: 0, URL: "https://example.com/1.jpg"},
		{Index: 1, URL: "https://example.com/2.jpg"},
	})
}

func main() {}
