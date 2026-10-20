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

**Koreksi 2026-10-20 (pagi):** dokumen ini dulu menyatakan `manhawato_cpp` "replaces
the Go wasm in `app_data/plugins/manhawato/`". Itu **tidak benar**. `md5sum`
membuktikan `app_data/plugins/manhawato/main.wasm` identik dengan versi **Go**
(`556a3890…` vs C++ `aca7086b…`), dan ukurannya 2 164 283 vs 306 538 bytes. Saat
itu belum ada satu pun C++ twin yang benar-benar dipasang.

**Status 2026-10-20 (sore): ketujuhnya SUDAH terpasang dan terverifikasi A/B.**
Lihat bagian "Live swap" di bawah — klaim di atas sudah usang, tapi disimpan
sebagai catatan bahwa "sudah dibuild di `examples/`" ≠ "sudah dipasang".

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

## Live swap 2026-10-20 — ketujuh C++ twin dipasang

Ketujuh C++ twin sekarang benar-benar menggantikan Go/TinyGo di
`app_data/plugins/`. Aturan yang dipegang: **plugin id dan `name` di `plugin.json`
tidak berubah**, jadi library/progress user tidak hilang (`Komix`, `MangaK`,
`KSGroupScans`, `KunManga`, `MangaKakalot`, `Manhwato`, `ReiManga`). Yang berubah
hanya `main.wasm` + field `built_with`.

| plugin | Go (bytes) | C++ (bytes) | ratio |
| --- | ---: | ---: | ---: |
| komix_to | 2 110 343 | 199 712 | 10.6× |
| mangak | 1 899 228 | 205 446 | 9.2× |
| ksgroupscans | 2 186 868 | 218 509 | 10.0× |
| kunmanga_online | 2 122 986 | 207 102 | 10.3× |
| mangakakalot | 2 181 593 | 213 731 | 10.2× |
| manhawato | 2 164 283 | 199 197 | 10.9× |
| reimanga_net | 1 834 751 | 106 850 | 17.2× |
| **total** | **14 500 052** | **1 350 547** | **10.7×** |

Sumber C++ di `examples/` **tidak dihapus** (arsip permanen); hanya
`app_data/plugins/` yang di-swap.

### A/B: Go baseline vs C++ terpasang

`plugin_cache` **wajib** dihapus per plugin sebelum capture
(`DELETE FROM plugin_cache WHERE plugin_id='X'`), kalau tidak payload lama yang
disajikan dan A/B jadi palsu. Endpoint: `search?q=love`, `detail/{id}`,
`chapters/{id}`, `pages/{chapterID}` (`{chapterID}` satu segmen → URL-encode).

Hasil akhir: **search/detail/chapters identik** untuk 6 dari 7 plugin;
`reimanga_net` justru **lebih baik** (lihat bawah). Semua perbedaan yang tersisa
sudah diverifikasi aman atau diperbaiki.

**Perbedaan yang AMAN (bukan regresi):**

- **`pages`: Go `null` → C++ `[]`** di ketujuh plugin. Host men-decode ke
  `var result []types.Page` lalu `json.Unmarshal`, jadi `null` dan `[]` sama-sama
  panjang 0 (`internal/pluginmanager/api_detail.go:97`). C++ kebetulan lebih benar
  bentuknya (array valid).
- **`released_at` kehilangan milidetik** (`…23.526Z` → `…23Z`, `komix_to`).
  Field-nya `time.Time` (`pkg/types/types.go:34`), keduanya RFC3339 valid.
- **Urutan `search` bisa beda** walau set hasil sama. Diuji: plugin yang sama,
  dua panggilan berurutan → urutan stabil, jadi ini urutan dari situs, bukan
  nondeterminisme plugin.

**Perbedaan yang diperbaiki (C++ lebih benar):**

- **`reimanga_net`**: versi Go mengembalikan
  `{"error":"bridge: plugin reimanga_net is unreachable…"}` untuk detail,
  chapters **dan** pages — praktis mati. C++ mengembalikan data nyata: 126 chapter
  (termasuk desimal `116.5`), author `Fargo`, cover, deskripsi, genres, dan 4 page.
  Ini peningkatan besar, bukan regresi.
- **`kunmanga_online`**: `cover_url` Go
  `https://www.kunmanga.onlinethumb/love-4-a-walk.webp` (slash hilang, URL rusak);
  C++ `https://www.kunmanga.online/thumb/love-4-a-walk.webp`. Sumber C++ sudah
  mencatat ini sebagai `ponytail:` comment.

**Bug C++ yang ditemukan dan diperbaiki saat swap:**

- **`mangakakalot_cpp`**: `for (…; count < 48; …)` — cap hardcoded yang
  membuang 2 hasil terakhir (Go mengembalikan 50, C++ 48). Cap dihapus; Go juga
  tidak punya batas, host yang memaginasi. Setelah perbaikan: 50/50 identik.

### Jebakan utama: `build.sh` C++ menelan kegagalan compile

`manhawato` kehilangan `status` setelah swap. Penyebabnya **bukan** kode, tapi
**binary basi**: `main.cpp` diubah 10-19 11:31, `main.wasm` masih 10-17 11:24.
Akarnya ada di `build.sh`:

```sh
"$WASI_SDK/bin/clang++" … main.cpp -o main.wasm \
    2>&1 | grep -v "no version information available" || true
test -f main.wasm
```

Exit status baris itu adalah milik `grep`, dan `|| true` menelannya habis. Saat
compile gagal, `main.wasm` **lama** tetap ada, `test -f` lolos, dan skrip
mencetak `exports OK` beserta ukuran binary lama. Persis ini yang terjadi: satu
build gagal, dilaporkan sukses, dan binary 2 hari yang lalu ikut ter-commit.

Perbaikan (diterapkan ke **kedelapan** `*_cpp/build.sh`): `rm -f main.wasm`
sebelum baris compile, sehingga kegagalan membuat `test -f` gagal keras. Setelah
`rm -f`, semua binary yang dibangun ulang jauh lebih kecil (mis. `komix_to`
313 116 → 199 712 B); flag compile tidak berubah dan validasi export tetap lolos,
serta A/B menunjukkan perilaku identik — jadi ukuran lama berasal dari artefak
build yang lebih tua, bukan dari flag yang hilang. **Pelajaran: jangan pernah
mempercayai `exports OK` dari skrip yang memfilter output compiler tanpa
memeriksa status compile.**

### Catatan operasional

- `komix.to` kadang butuh ~16.7 s untuk satu halaman, melebihi `invokeTimeout`
  host (15 s, `internal/pluginmanager/manager.go:22`). Modul wasm lalu ditutup dan
  panggilan **berikutnya** gagal dengan `malloc failed for input` — pesan yang
  menyesatkan. Setelah restart server, plugin normal kembali. Jadi `malloc failed`
  pada plugin C++/wasm berarti "modul sudah mati", bukan "alokasi gagal".

## Related

- `examples/plugins/wasm/komix_to_cpp/README.md` — build + ABI for the C++ side
- `okf-mcp/concept/wasm-plugin-build-scripts.md` — TinyGo/standard-Go build path
- `okf-mcp/concept/wasm-plugin-local-helpers-decision.md` — why hot paths stay local
- `docs/plugin-wasm-helpers.md` — `env.host_call` helper catalogue
