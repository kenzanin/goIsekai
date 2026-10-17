---
type: Concept
title: Native .so plugin ABI (purego loader)
description: How goIsekai loads C/C++ .so plugins without cgo — the ownership contract, the export set, and why stdlib `plugin` could never work.
tags:
  - goisekai
  - plugins
  - abi
  - purego
  - cgo
generated:
  by: process:reasonix
  at: "2026-10-18T00:00:00Z"
---

# Native .so plugin ABI

## Why purego

`goplug.go` originally used the stdlib `plugin` package, which requires **cgo**
and a toolchain identical to the host's. The host builds `CGO_ENABLED=0`, so no
`.so` could ever load — `loadGo`/`callGo` sat at 0% coverage and the `go` kind was
a registry of files that never instantiated. `github.com/ebitengine/purego`
reaches `dlopen`/`dlsym` through the platform loader directly
(`dlfcn_nocgo_linux.go` on Linux), so the host stays cgo-free. It was already in
`go.sum` as an indirect dependency (`v0.10.1`).

**Decision:** purego replaces stdlib `plugin` for the `go` kind — one `.so`
mechanism, not a second `native`/`cpp` kind. Nothing was lost, because the stdlib
path never worked in this build configuration.

## The ABI

Each entry point takes a NUL-terminated JSON string and returns a NUL-terminated
JSON string:

```c
char *Search(const char *json);
char *GetMangaDetail(const char *json);
char *GetChapterList(const char *json);
char *GetPageList(const char *json);
char *GetGenres(const char *json);   // optional
void  plugin_free(char *p);          // required
```

- **Ownership: the plugin allocates, the host copies, then calls `plugin_free` on
  exactly that pointer.** purego does not free a returned `char*`, so ownership
  has to be explicit or C/C++/Rust authors would each guess differently. This
  mirrors the alloc/free pairing wasm already uses.
- **`plugin_free` is required.** `loadGo` rejects a library without it, because
  every call would otherwise leak.
- **Input needs no manual buffer:** purego converts a Go `string` argument to a
  NUL-terminated C string for the call and keeps it alive across it.
- **Errors travel as the shared `{"error":"code"}` envelope**, the same shape the
  other runtimes use, so `PluginError()` renders it. A lone `error` key is
  treated as failure; any other object is a payload.

Return shapes must match the host decoders, not a wrapper: `Search` and
`GetChapterList` decode a **bare JSON array**, `GetMangaDetail` a **bare object**,
and `types.Chapter.ChapterNum` is a **number**, not a string. Getting this wrong
is the easy mistake — the dummy fixture originally returned
`{"results":[...]}` (the HTTP API shape, not the plugin ABI) and
`"chapter_num":"1"`.

## Registration and metadata

`.so` plugins follow the same two shapes as every other runtime: a folder plugin
(`<dir>/<id>/main.so` with `plugin.json` beside it) or a bare `<dir>/<id>.so`.
They register **lazily** — `discoverGo` reads `plugin.json` and never opens the
library, so display name/logo/site_url are known before any invoke. That is the
same contract the metadata migration established for all five runtimes.

## Lifecycle

`nativeLib` wraps the dlopen handle: `openNativeLib`, `bind` (resolve-then-register,
so an absent optional symbol like `GetGenres` is an ordinary error instead of the
panic `RegisterLibFunc` raises on a miss), and `close`. `reload.go` closes the
handle on unload. A non-unix build gets `goplug_native_other.go`, where opening
fails with a clear message rather than breaking the build.

**`callGo` applies no deadline.** The call is a direct C call the host cannot
interrupt, unlike the script runtimes whose VMs support interruption.

## Fixture

`examples/plugins/native/dummy/` — a C++ `.so` whose entry points answer from
canned fixtures, built by `build.sh` (`g++ -shared -fPIC`). `*.so` is gitignored,
so `goplug_native_test.go` **skips** when `libdummy.so` is absent instead of
failing. The fixture echoes the query back so the test proves the input string
crossed the Go→C boundary intact, and returns the error envelope from one entry
point to prove the host converts it into an error.

## Gotcha worth remembering

`cString` walks the bytes manually because purego keeps its `CString`/`GoString`
helpers **internal**. The `uintptr`→`unsafe.Pointer` conversion triggers
`govet`'s `unsafeptr` check — unavoidable, since dlopen hands symbols back as
`uintptr` — so it carries a `//nolint:govet` with the reason inline.
