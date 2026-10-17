# komix_to_cpp — WASM Plugin (C++17 via wasi-sdk)

Source plugin for **komix.to**, written in C++17 and compiled with the system
**wasi-sdk** clang to a `wasm32-wasi` reactor module. ~**0.3 MB** — about 10×
smaller than the Go wasm build of the same site (`../komix_to/`, ~4 MB).

## Build

Requires: `/opt/wasi-sdk` (clang 13, wasm32-wasi sysroot) and RapidJSON headers
(`/usr/include/rapidjson`, header-only).

```bash
cd examples/plugins/wasm/komix_to_cpp
./build.sh        # produces main.wasm and verifies the required exports
```

What `build.sh` runs:

```sh
/opt/wasi-sdk/bin/clang++ -std=c++17 -O2 -fno-exceptions -fno-rtti \
  --target=wasm32-wasi -mexec-model=reactor -idirafter /usr/include \
  main.cpp -o main.wasm \
  -Wl,--export=malloc -Wl,--export=free -Wl,-z,stack-size=1048576
```

- `-mexec-model=reactor` links `crt1-reactor.o`, which exports `_initialize` and
  provides `malloc`/`free`. Do **not** declare `export_name("malloc")`,
  `("free")` or `("_initialize")` in the source — the linker then fails with
  `duplicate export name`.
- On distros that ship only `libxml2.so.16`, `wasm-ld` needs a `libxml2.so.2`
  shim; `build.sh` creates one in a temp dir and puts it on `LD_LIBRARY_PATH`
  (`LIBRARY_PATH` does not work for this). No root required.

## JSON

**RapidJSON** (`#include <rapidjson/...>`), header-only. `nlohmann/json` does
**not** compile under wasi-sdk's libc++ here — its `<filesystem>` dependency is
stubbed out in the sysroot (`_LIBCPP_HAS_NO_FILESYSTEM_LIBRARY`).

## Shared helpers

This folder ships three headers that are copied verbatim into
`../manhawato_cpp/` (keep them identical — they are the C++ plugin's reusable
base, the analogue of the Go helpers):

| header | contents |
| --- | --- |
| `wasm_abi.h` | `env.host_call` import; `alloc_mem`/`free_mem`/`pack`/`read_string`/`reply_json`/`to_json`; `host_call_fn`/`http_body`/`get_body`/`host_str`; `contract_version` |
| `util.h` | `trim`, `url_encode`/`url_decode`, `decode_string`, `humanize`, `genre_slug`, `normalize_status`, `parse_time`, `add_string` |
| `html_util.h` | `strip_html`, `unescape_html`, `first_sub`, `all_subs`, `img_url`, `attr`, `make_abs` |

Text helpers delegate to the host via `host_str()` where an equivalent exists, so
the logic cannot drift from the Lua/JS runtimes:
`url_encode`→`text.url_encode`, `url_decode`→`text.url_decode`,
`parse_time`→`text.date_to_iso`, `strip_html`→`text.strip_html`,
`unescape_html`→`text.unescape`. `normalize_status` and `humanize` stay local
on purpose — the host equivalents use a different vocabulary / title-casing.

## ABI

Exports (C linkage, `__attribute__((export_name(...)))`):

| Symbol | Type | Purpose |
| --- | --- | --- |
| `contract_version` | i32 | ABI version (returns 1) |
| `Search` | i64 | JSON filter in → JSON array of manga |
| `GetMangaDetail` | i64 | manga id → manga detail JSON |
| `GetChapterList` | i64 | manga id → chapter list JSON |
| `GetPageList` | i64 | chapter id → page list JSON |
| `GetGenres` | i64 | → genre list JSON |

`malloc`, `free`, `_initialize` and `memory` come from the reactor CRT/libc++.

Import:

| Module | Function | Purpose |
| --- | --- | --- |
| `env` | `host_call` | generic host helper (HTTP, JSON, regex, …) |

Call convention: `(in_ptr u32, in_len u32) → i64` where
`i64 = (result_len << 32) | result_ptr`.

## Porting notes

- Regex patterns containing `)"` (e.g. `href="([^"]+)"`) terminate the default
  `R"( … )"` raw string early — use a custom delimiter: `R"re( … )re"`.
- Go `(?s)` dot-all → `[\s\S]` (`std::regex` has no inline flags);
  `std::regex::icase` covers Go's `(?i)`.

See `../../../../okf-mcp/concept/wasm-cpp-plugin-toolchain.md` for the full
toolchain write-up and the size comparison against Go/TinyGo.
