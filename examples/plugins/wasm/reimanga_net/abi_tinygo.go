//go:build wasip1 && tinygo

package main

import "unsafe"

// TinyGo's runtime exports malloc/free/calloc/realloc itself, so this build
// must not declare them — a second export makes wasm-opt abort with
// "duplicate export name". Allocations come straight from the Go heap, which
// the host reaches because wasm linear memory *is* that heap.

// pinned keeps the last reply reachable until the host has read it. One entry
// suffices: callWasm holds a per-plugin mutex, so only one reply is in flight.
var pinned [][]byte

func allocCopy(b []byte) uint32 {
	if len(b) == 0 {
		return 0
	}
	out := make([]byte, len(b))
	copy(out, b)
	pinned = append(pinned[:0], out)
	return uint32(uintptr(unsafe.Pointer(&out[0])))
}

// free keeps main.go's call sites identical to the !tinygo build: the host's
// wasmFree reaches TinyGo's runtime free, which is already a no-op.
func free(uint32) {}
