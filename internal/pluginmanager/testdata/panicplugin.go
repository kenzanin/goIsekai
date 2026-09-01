//go:build wasip1

// Package main is a test plugin that panics inside Search. It exists to prove
// that a misbehaving plugin returns a Go error to the host instead of crashing
// the reader (acceptance criterion 7.2).
package main

import "github.com/extism/go-pdk"

//go:wasmexport contract_version
func contractVersion() int32 {
	pdk.OutputString("1")
	return 0
}

//go:wasmexport Search
func Search() int32 {
	panic("boom from plugin")
}

//go:wasmexport GetMangaDetail
func GetMangaDetail() int32 { return 0 }

//go:wasmexport GetChapterList
func GetChapterList() int32 { return 0 }

//go:wasmexport GetPageList
func GetPageList() int32 { return 0 }

func main() {}
