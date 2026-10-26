//go:build wasip1

package main

import "unsafe"

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

func free(uint32) {}
