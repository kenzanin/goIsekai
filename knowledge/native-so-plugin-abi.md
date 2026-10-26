---
type: Concept
title: Native .so plugin ABI (purego loader)
description: "How goIsekai loads C/C++ .so plugins without cgo — the ownership contract, the export set, and why stdlib plugin could never work."
tags: [goisekai, plugins, abi, purego, cgo, Concept]
generated: { by: agent/mcp, at: "2026-10-26T15:42:32Z" }
status: stable
---

# Native .so plugin ABI (purego loader)

## Why purego

goplug.go originally used the stdlib plugin package, which requires cgo and a toolchain identical to the host. The host builds CGO_ENABLED=0, so no .so could ever load. github.com/ebitengine/purego reaches dlopen/dlsym through the platform loader directly (dlfcn_nocgo_linux.go on Linux), so the host stays cgo-free. It was already in go.sum as an indirect dependency (v0.10.1).

Decision: purego replaces stdlib plugin for the go kind — one .so mechanism, not a second native/cpp kind.

## The ABI

Each entry point takes a NUL-terminated JSON string and returns a NUL-terminated JSON string:

```c
char *Search(const char *json);
char *GetMangaDetail(const char *json);
char *GetChapterList(const char *json);
char *GetPageList(const char *json);
char *GetGenres(const char *json);   // optional
void  plugin_free(char *p);          // required
```

- Ownership: the plugin allocates, the host copies, then calls plugin_free on exactly that pointer. purego does not free a returned char*, so ownership must be explicit.
- plugin_free is required. loadGo rejects a library without it.
- Input needs no manual buffer: purego converts a Go string argument to a NUL-terminated C string for the call.
- Errors travel as the shared {"error":"code"} envelope.

Return shapes must match the host decoders: Search and GetChapterList decode a bare JSON array, GetMangaDetail a bare object, types.Chapter.ChapterNum is a number not a string.

## Registration and metadata

.so plugins follow the same two shapes as every other runtime: a folder plugin or a bare .so. They register lazily. .so is gitignored.

## Lifecycle

nativeLib wraps the dlopen handle: openNativeLib, bind (resolve-then-register), close. reload.go closes the handle on unload. Reload is UnloadPlugin then ensureLoaded, load-bearing because dlopen of a path already loaded returns the cached handle and keeps running old code. Covered by TestReloadNativePluginStaysUsable, TestReloadWasmPluginStaysUsable, TestFailedNativeReloadKeepsThePluginRegistered.

callGo bounds the host helpers, not the call. The C call itself cannot be interrupted (no VM). callGo publishes its ctx on p.curCtx; bridge reads it. Without it those run on context.Background.

## Host helpers: the plugin_init bridge

A wasm guest reaches host helpers through env.host_call import. A .so has no import table, so direction is reversed: host calls an optional export with a function pointer:

```c
typedef int32_t (*goisekai_host_call_fn)(const char *req_json, char *out, int32_t cap);
void plugin_init(goisekai_host_call_fn fn);
```

Plugin sends the same envelope wasm uses ({fn, args}), reads JSON back from its own buffer. One bridge reaches every helper via dispatchHostCall. No allocation crosses the two allocators. plugin_init is optional. wireHostCall is platform-split. native_abi.h is the plugin-side half.

## Fixture

examples/plugins/native/dummy/ — C++ .so answering from canned fixtures, built by build.sh. *.so is gitignored, so goplug_native_test.go skips when libdummy.so absent. TestNativePluginReachesHostHelpers: fixture GetMangaDetail asks host to run text.titlecase on "host bridge" and returns "Host bridge".

## Gotcha

cString walks bytes manually (purego keeps CString/GoString internal). uintptr to unsafe.Pointer conversion triggers govet unsafeptr check, carries //nolint:govet.
