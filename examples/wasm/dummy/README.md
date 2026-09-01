# dummy-plugin

A reference manga-source plugin for goIsekai. It implements the full host/plugin
ABI and serves a small hardcoded catalog, so it works offline and demonstrates
every contract the runtime expects. Use it as the starting point for a real
source plugin (e.g. a mangadex.org parser).

## ABI contract

A plugin is a `wasip1` WASM module built with `-buildmode=c-shared`. It must
export:

| Export | Signature | Purpose |
| --- | --- | --- |
| `contract_version` | `() -> i32` | Must return `types.ContractVersion` (currently `1`) or the host rejects the plugin. |
| `malloc` | `(size: i32) -> i32` | Host-allocated input buffers. |
| `free` | `(ptr: i32) -> ()` | Host-freed buffers. |
| `Search` | `(ptr, len: i32) -> i64` | `SearchFilter` JSON in → `[]Manga` JSON out. |
| `GetMangaDetail` | `(ptr, len: i32) -> i64` | JSON-encoded manga id in → `Manga` JSON out. |
| `GetChapterList` | `(ptr, len: i32) -> i64` | JSON-encoded manga id in → `[]Chapter` JSON out. |
| `GetPageList` | `(ptr, len: i32) -> i64` | JSON-encoded chapter id in → `[]Page` JSON out. |

And imports from the host (`env` module):

| Import | Signature | Purpose |
| --- | --- | --- |
| `host_http_request` | `(ptr, len: i32) -> i64` | All network access. `HTTPRequest` JSON in → `HTTPResponse` JSON out. |

The `(ptr, len)` i64 convention is: **low 32 bits = pointer, high 32 bits =
length**. Input ids are JSON-encoded strings (`json.Marshal(mangaID)`), so
decode them with `json.Unmarshal`.

## Build

```sh
cd examples/dummy-plugin
make build          # produces dummy.wasm
```

or from the repo root:

```sh
GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o dummy.wasm ./examples/dummy-plugin
```

## Install

Two ways:

1. **Via the app**: open goIsekai → Plugins → Install, pick `dummy.wasm`. The
   app copies it into `<data_dir>/plugins/` and loads it immediately.
2. **Manually**: `make install` (copies into `../../app_data/plugins/`), then
   restart goIsekai. Override the data dir with `make install DATA_DIR=<dir>`.

## What it returns

The dummy catalog is three hardcoded manga (`dummy-solo`, `dummy-romance`,
`dummy-horror`), each with three chapters and eight placeholder image pages
(picsum.photos). `Search` matches the query against the title,
case-insensitively; an empty query returns the whole catalog.

## Writing a real source plugin

The hardcoded `catalog` is the only part to replace. A real parser would, for
example:

```go
func Search(ptr, length uint32) uint64 {
	var f types.SearchFilter
	_ = json.Unmarshal([]byte(readString(ptr, length)), &f)
	resp, err := fetch("https://api.mangadex.org/manga?title=" + f.Query)
	if err != nil {
		return returnJSON([]types.Manga{})
	}
	// ... parse resp.Body into []types.Manga ...
	return returnJSON(results)
}
```

See `fetch` in `main.go` for the complete host-request round-trip. The host
injects default headers, keeps a per-plugin cookie jar, and blocks direct
sockets — plugins do all networking through `host_http_request`.
