package pluginmanager

import (
	"unsafe"
)

// writeCString copies s plus a NUL terminator into a buffer the caller owns,
// returning the byte count or -1 when it does not fit.
func writeCString(out *byte, outCap int32, s string) int32 {
	if out == nil || outCap <= 0 || len(s) >= int(outCap) {
		return -1
	}
	buf := unsafe.Slice(out, int(outCap))
	n := copy(buf, s)
	buf[n] = 0
	return int32(n)
}

// cString reads a NUL-terminated C string at ptr. purego keeps its own
// CString/GoString helpers internal, so the host walks the bytes itself; JSON
// payloads never contain a NUL, which is what makes the terminator safe.
//
// ptr is unsafe.Pointer rather than uintptr because purego hands void* back as
// a real pointer; keeping it typed is what lets go vet follow the value instead
// of flagging a "possible misuse of unsafe.Pointer".
func cString(ptr unsafe.Pointer) string {
	if ptr == nil {
		return ""
	}
	return cStringAt(ptr)
}

// cStringFrom is cString for the callback path, where the pointer arrives as a
// *byte. Going through unsafe.Pointer(*byte) keeps vet able to follow it, so no
// nolint is needed.
func cStringFrom(p *byte) string {
	if p == nil {
		return ""
	}
	return cStringAt(unsafe.Pointer(p))
}

func cStringAt(base unsafe.Pointer) string {
	var b []byte
	for i := 0; ; i++ {
		c := *(*byte)(unsafe.Add(base, i))
		if c == 0 {
			break
		}
		b = append(b, c)
	}
	return string(b)
}
