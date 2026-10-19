//go:build wasip1 && !tinygo

package main

import "unsafe"

// The host allocates plugin input and output buffers through the module's
// exported malloc/free (internal/pluginmanager/wasm_abi.go). TinyGo's runtime
// already exports both, and a second declaration aborts wasm-opt with
// "duplicate export name" — so the pair lives behind a build tag, with the
// TinyGo counterpart in abi_tinygo.go.

// buffers keeps every live allocation reachable: the host holds the pointer,
// but only this map stops the Go slice being collected while the host writes
// into it.
var buffers = map[uint32][]byte{}

//go:wasmexport malloc
func malloc(n uint32) uint32 {
	if n == 0 {
		n = 1 // the host reads a zero pointer as allocation failure
	}
	b := make([]byte, n)
	ptr := uint32(uintptr(unsafe.Pointer(&b[0])))
	buffers[ptr] = b
	return ptr
}

//go:wasmexport free
func free(ptr uint32) { delete(buffers, ptr) }

func allocCopy(b []byte) uint32 {
	if len(b) == 0 {
		return 0
	}
	ptr := malloc(uint32(len(b)))
	copy(buffers[ptr], b)
	return ptr
}
