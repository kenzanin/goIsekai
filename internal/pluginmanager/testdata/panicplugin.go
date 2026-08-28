//go:build wasip1

// Package main is a test plugin that panics inside Search. It exists to prove
// that a misbehaving plugin returns a Go error to the host instead of crashing
// the reader (acceptance criterion 7.2).
package main

import "unsafe"

const abiVersion int32 = 1

var allocations = map[uint32][]byte{}

//go:wasmexport malloc
func malloc(size uint32) uint32 {
	b := make([]byte, size)
	ptr := uint32(uintptr(unsafe.Pointer(unsafe.SliceData(b))))
	allocations[ptr] = b
	return ptr
}

//go:wasmexport free
func free(ptr uint32) { delete(allocations, ptr) }

//go:wasmexport contract_version
func contractVersion() int32 { return abiVersion }

//go:wasmexport Search
func Search(ptr, length uint32) uint64 {
	panic("boom from plugin")
}

//go:wasmexport GetMangaDetail
func GetMangaDetail(ptr, length uint32) uint64 { return 0 }

//go:wasmexport GetChapterList
func GetChapterList(ptr, length uint32) uint64 { return 0 }

//go:wasmexport GetPageList
func GetPageList(ptr, length uint32) uint64 { return 0 }

func main() {}
