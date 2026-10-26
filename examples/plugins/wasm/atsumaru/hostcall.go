//go:build wasip1

package main

import (
	"encoding/json"
	"unsafe"
)

//go:wasmimport env host_call
func hostCallRaw(ptr, length uint32) uint64

func readString(ptr, n uint32) string {
	if n == 0 || ptr == 0 {
		return ""
	}
	return string(unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), n))
}

func pack(ptr, n uint32) uint64 { return uint64(n)<<32 | uint64(ptr) }

func reply(v any) uint64 {
	b, err := json.Marshal(v)
	if err != nil || len(b) == 0 {
		b = []byte("null")
	}
	return pack(allocCopy(b), uint32(len(b)))
}

func hostCall(fn string, args ...string) string {
	req, err := json.Marshal(struct {
		Fn   string   `json:"fn"`
		Args []string `json:"args"`
	}{fn, args})
	if err != nil {
		return ""
	}
	ptr := allocCopy(req)
	out := hostCallRaw(ptr, uint32(len(req)))
	free(ptr)
	resp, n := uint32(out), uint32(out>>32)
	if resp == 0 || n == 0 {
		return ""
	}
	s := readString(resp, n)
	free(resp)
	return s
}

func param(name string) string {
	var s string
	if json.Unmarshal([]byte(hostCall("plugin.param", name)), &s) != nil {
		return ""
	}
	return s
}

func hostString(fn string, args ...string) string {
	raw := hostCall(fn, args...)
	if raw == "" || raw == "null" || raw[0] == '{' {
		return ""
	}
	var s string
	if json.Unmarshal([]byte(raw), &s) != nil {
		return ""
	}
	return s
}

type httpResp struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

func httpGet(url, headersJSON string) (httpResp, bool) {
	var r httpResp
	if json.Unmarshal([]byte(hostCall("http.get", url, headersJSON)), &r) != nil {
		return httpResp{}, false
	}
	return r, true
}

func httpPost(url, body, headersJSON string) (httpResp, bool) {
	var r httpResp
	if json.Unmarshal([]byte(hostCall("http.post", url, body, headersJSON)), &r) != nil {
		return httpResp{}, false
	}
	return r, true
}

// ponytail: one-line urlEncode delegates to host text.url_encode — same
// result, no local percent-encoding code. Ceiling: one extra IPC round-trip
// per call; upgrade to a local encoder only if profiling shows it matters.
func urlEncode(s string) string { return hostString("text.url_encode", s) }

func logInfo(msg string)  { hostCall("log.info", msg) }
func logWarn(msg string)  { hostCall("log.warn", msg) }
func logError(msg string) { hostCall("log.error", msg) }
