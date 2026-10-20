---
id: wasm-cpp-plugin-toolchain
title: C++ WASM plugin toolchain (wasi-sdk) and binary-size comparison
type: finding
tags: [goisekai, plugin, wasm, cpp, wasi-sdk, rapidjson, tinygo, build, size]
---

# C++ WASM plugin toolchain (wasi-sdk) and binary-size comparison

Achievement: goIsekai WASM plugins can now be written in **C++17** and built with
the system **wasi-sdk** clang, alongside the Go (`//go:wasmexport`) path. Shipped:
`examples/plugins/wasm/komix_to_cpp/` and `examples/plugins/wasm/manhawato_cpp/`.
Commit `a628bd5`.

**Koreksi 2026-10-20:** dokumen ini dulu menyatakan `manhawato_cpp` "replaces the
Go wasm in `app_data/plugins/manhawato/`". Itu **tidak benar**. `md5sum` membuktikan
`app_data/plugins/manhawato/main.wasm` identik dengan versi **Go**
(`556a3890…` vs C++ `aca7086b…`), dan ukurannya 2 164 283 vs 306 538 bytes. Jadi
belum ada satu pun C++ twin yang benar-benar dipasang; ketujuhnya masih berupa
build di `examples/` tanpa bukti A/B live sebagai plugin terpasang.

## Toolchain

```
/opt/wasi-sdk/bin/clang++ -std=c++17 -O2 -fno-exceptions -fno-rtti \
  --target=wasm32-wasi -mexec-model=reactor -idirafter /usr/include \
  main.cpp -o main.wasm \
  -Wl,--export=malloc -Wl,--export=free -Wl,-z,stack-size=1048576
```

- `-fno-exceptions -fno-rtti` drops unwind tables + typeinfo (large).
- `-mexec-model=reactor` links `crt1-reactor.o`, which **already exports
  `_initialize`** and provides `malloc`/`free` — do **not** declare
  `export_name("malloc")`/`("free")`/`("_initialize")` yourself, or the linker
  dies with `[parse exception: duplicate export name]`. Just pass
  `-Wl,--export=malloc -Wl,--export=free`.
- `-idirafter /usr/include` finds the system JSON headers *after* wasi-sdk's own.
- **JSON = RapidJSON** (`/usr/include/rapidjson`, header-only). `nlohmann/json`
  **fails** under wasi-sdk libc++: its `std_fs.hpp` pulls `<filesystem>`, which
  the sysroot stubs out (`_LIBCPP_HAS_NO_FILESYSTEM_LIBRARY` fatal error).
- **wasm-ld needs a libxml2 shim** on distros shipping only `libxml2.so.16`
  (wasm-ld links `libxml2.so.2`). No root needed: symlink into a temp dir and
  prepend it to `LD_LIBRARY_PATH`. `komix_to_cpp/build.sh` does this
  automatically; `LIBRARY_PATH` does NOT work for this (that is compile-time).
  The `ldd` guard must test for `libxml2.so.2 => not found`, not just presence.

## Shared helpers (copy-paste between plugins)

`komix_to_cpp/` and `manhawato_cpp/` share three identical headers (verified by
`diff`):

- `wasm_abi.h` — `env.host_call` import, `alloc_mem`/`free_mem`/`pack`/
  `read_string`/`reply_json`/`to_json`, `host_call_fn`/`http_body`/`get_body`/
  `host_str`, `contract_version`.
- `util.h` — `trim`, `url_encode`/`url_decode`, `decode_string`, `humanize`,
  `genre_slug`, `normalize_status`, `parse_time`, `add_string`.
- `html_util.h` — `strip_html`, `unescape_html`, `first_sub`, `all_subs`,
  `img_url`, `attr`, `make_abs`.

Text helpers whose body is **delegated to the host** via `host_str()` (so the
logic cannot drift from the Lua/JS runtimes; names/signatures unchanged, no
call-site edits): `url_encode`→`text.url_encode`, `url_decode`→`text.url_decode`,
`parse_time`→`text.date_to_iso`, `strip_html`→`text.strip_html`,
`unescape_html`→`text.unescape`.

Kept **local on purpose** (host equivalent would change behavior):

- `normalize_status` — host `text.normalize_status` is an exact-key map
  (`dropped→Dropped`, `cancelled→Dropped`); the plugin's is substring-based
  (`contains "drop"` → `Cancelled`). Forcing host would change UI status labels.
- `humanize` — host `text.titlecase` only uppercases the first rune
  (`isekai-mesu` → `Isekai-mesu`), not per-word title case.

## C++ porting gotchas

- **Raw string delimiter**: patterns containing `)"` (e.g. `href="([^"]+)"`)
  terminate the default `R"( … )"` early and corrupt the next declaration. Use a
  custom delimiter: `R"re( … )re"`.
- **Go `(?s)` dot-all** has no `std::regex` inline flag → use `[\s\S]`.
  `std::regex::icase` covers Go's `(?i)`.

## Binary size (measured, `manhawato` plugin logic)

| toolchain | size | notes |
| --- | --- | --- |
| C++ / wasi-sdk clang 13 | **306–313 KB** | 708 functions; `std::regex` ≈ 287 KB of it |
| TinyGo 0.41.1 `-target wasi` | **1,917,497 B ≈ 1.9 MB** | probe build, see below |
| standard Go 1.27.2 (`-buildmode=c-shared`) | **3.9–5.2 MB** | reflection drags the type table |

~15× between C++ and standard Go. Why: standard Go carries the full runtime
(GC, goroutine scheduler, `panic`/`defer`, map, channel) plus **reflection type
descriptors** that DCE cannot strip, `encoding/json` (reflection-based) and the
whole RE2 engine. C++ carries only wasi-libc + bit of libc++ with
`-fno-exceptions -fno-rtti`; RapidJSON templates and `--gc-sections` emit only
what is used.

## TinyGo probe (why plugins do not use it)

Probe: copy `komix_to/main.go`, build with TinyGo 0.41.1 → **1.9 MB**, roughly
half of standard Go but ~6× C++.

- **TinyGo already exports `malloc`/`free`** (plus `calloc`, `realloc`,
  `fasyncify_*`) on its own. The Go plugin sources declare `//go:wasmexport
  malloc`/`free` → **duplicate export name**, build fails. The probe disabled
  those two attributes to measure.
- TinyGo `-target wasi` is **command style** (`_start`), not the reactor
  (`_initialize`) the host expects. A real TinyGo port needs `-buildmode=c-shared`
  plus ABI export fixes — not a one-line target swap. This is the root cause of
  the earlier fallback to standard-Go.
- `wasm-opt` (available at `~/.opt/tinygo/bin/wasm-opt`) could shrink either
  backend further; not applied here.

## Related

- `examples/plugins/wasm/komix_to_cpp/README.md` — build + ABI for the C++ side
- `okf-mcp/concept/wasm-plugin-build-scripts.md` — TinyGo/standard-Go build path
- `okf-mcp/concept/wasm-plugin-local-helpers-decision.md` — why hot paths stay local
- `docs/plugin-wasm-helpers.md` — `env.host_call` helper catalogue
