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

**Reload is `UnloadPlugin` then `ensureLoaded`, and that ordering is load-bearing.**
`ReloadPlugin` used to re-instantiate by hand and pass `p.wasmPath` to the
folder-based `LoadPlugin`. For `.so` and wasm plugins `wasmPath` is the entry
**file**, not a folder, and `LoadPlugin` probes only `main.lua`/`main.js`/
`main.go`/`main.wasm` — so reload could never re-open either kind. It also
`delete`d the registry entry *before* loading, so any failed reload made a
running plugin vanish from the UI until restart. Both are gone; on failure the
plugin stays registered but unloaded, and the next call lazy-loads it again, so a
bad edit is recoverable by fixing the file.

The `dlclose` is correctness, not hygiene: `dlopen` of a path that is already
loaded returns the **cached handle** and keeps running the old code, so without
releasing the old handle first a reloaded native plugin would execute the version
it started with. Verified by measurement — `dlclose` + `dlopen` on a replaced
file does pick up the new build. Covered by `TestReloadNativePluginStaysUsable`,
`TestReloadWasmPluginStaysUsable`, and `TestFailedNativeReloadKeepsThePluginRegistered`.

Known gap this leaves open: `Install`/`LoadPluginHot` still do not recognise
`main.so`, so installing or hot-loading a native plugin fails (only discovery
registers it). `discoverGo` is unaffected.


**`callGo` bounds the host helpers, not the call.** The C call itself cannot be
interrupted — there is no VM to trap, unlike the script runtimes. So the invoke
deadline applies to the *helpers the plugin calls back into* (an HTTP fetch, a
CDP solve): `callGo` publishes its ctx on `p.curCtx` and the bridge reads it.
Without that those would run on `context.Background()` and outlive the invoke.

## Host helpers: the `plugin_init` bridge

A wasm guest reaches host helpers through the `env.host_call` **import**. A `.so`
has no import table, so the direction is reversed: the host calls an optional
export with a function pointer at load time.

```c
typedef int32_t (*goisekai_host_call_fn)(const char *req_json, char *out, int32_t cap);
void plugin_init(goisekai_host_call_fn fn);
```

The plugin then sends the *same envelope wasm uses* —
`{"fn":"text.url_encode","args":[...]}` — and reads JSON back out of **its own**
buffer. Two consequences worth keeping:

- **One bridge reaches every helper**, because `dispatchHostCall` is already the
  single place they all resolve. Nothing needs adding per helper as the host
  grows `http.*`, `text.*`, `html.*`, `json.*`, `regex.*`, `codecs.*`, `crypto.*`.
- **No allocation crosses the two allocators.** The host writes into a buffer the
  plugin owns (`out`/`outCap`), so nothing malloc'd by Go is freed by the plugin
  or vice versa. A `-1` reply means "did not fit" and the plugin retries bigger.

`plugin_init` is **optional**: a plugin without it still loads and simply has no
helpers, which is what every native plugin had before this. `wireHostCall` is
platform-split like the rest of the loader (`goplug_native.go` /
`goplug_native_other.go`) so purego stays quarantined.

The callback runs on the same goroutine that made the C call, while `callGo`
holds `p.mu` — so it reads `p.curCtx` **directly**; taking `p.mu` there would
deadlock.

`native_abi.h` is the plugin-side half, the `.so` counterpart of `wasm_abi.h`:
same helper names, same envelope, so plugin code ports between runtimes. Its
`host_call_raw` doubles the buffer on a `-1` reply (64 KiB → 8 MiB), which is how
a full HTML page gets through.

`Think twice before removing the bridge`: with it, native plugins can delegate
text/HTML/JSON work to the host instead of re-implementing it with libraries —
which is the whole reason the roliascan port could shrink to libcurl + libxml2.

## Fixture

`examples/plugins/native/dummy/` — a C++ `.so` whose entry points answer from
canned fixtures, built by `build.sh` (`g++ -shared -fPIC`). `*.so` is gitignored,
so `goplug_native_test.go` **skips** when `libdummy.so` is absent instead of
failing. The fixture echoes the query back so the test proves the input string
crossed the Go→C boundary intact, and returns the error envelope from one entry
point to prove the host converts it into an error.

`TestNativePluginReachesHostHelpers` is the bridge's proof: the fixture's
`GetMangaDetail` asks the host to run `text.titlecase` on "host bridge" and
returns the answer in `description`, so the test asserts `"Host bridge"` — an
empty value means `plugin_init` was never wired.

## Gotcha worth remembering

`cString` walks the bytes manually because purego keeps its `CString`/`GoString`
helpers **internal**. The `uintptr`→`unsafe.Pointer` conversion triggers
`govet`'s `unsafeptr` check — unavoidable, since dlopen hands symbols back as
`uintptr` — so it carries a `//nolint:govet` with the reason inline.
