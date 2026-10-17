# manhawato_cpp — WASM Plugin (C++17 via wasi-sdk)

Source plugin for **manhwato.com**, a 1:1 C++17 port of the Go plugin in
`../manhawato/main.go`. Built with the system **wasi-sdk** clang to a
`wasm32-wasi` reactor module (~0.3 MB). This build replaces the Go wasm at
`app_data/plugins/manhawato/main.wasm`.

Site shape (Madara-like, server-rendered HTML):
`/manhwa-genre/<slug>` (genre listing), `/search?q=` (search),
`/manhwa/<slug>` (detail), `/manhwa/<slug>/<chapter-slug>` (chapter),
pages are `<img>` tags under `/images/manga/`.

- `Manga.ID` = manhwa slug; `Chapter.ID` = `<mangaSlug>:<chapterSlug>`.

## Build

```bash
cd examples/plugins/wasm/manhawato_cpp
./build.sh        # produces main.wasm and verifies the required exports
```

Same toolchain and flags as `../komix_to_cpp/` — see that folder's README for the
full command, the `libxml2.so.2` shim, and the ABI table.

## Shared helpers

`wasm_abi.h`, `util.h`, `html_util.h` are copied verbatim from
`../komix_to_cpp/` (keep them byte-identical). They carry the host ABI glue,
text/JSON helpers and the regex scraping helpers; plugin-specific logic lives in
`main.cpp`.
