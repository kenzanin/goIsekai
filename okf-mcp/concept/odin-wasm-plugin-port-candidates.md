---
id: odin-wasm-plugin-port-candidates
title: Odin WASM plugin port candidates (Lua -> Odin)
description: >
  Tier-ranked list of Lua plugins that are already ported to Odin in
  examples/plugins/wasm/*_odin/, plus the verification state and next
  candidate for migration. Based on the helper-parity gate and live A/B.
tags:
  - goisekai
  - plugins
  - wasm
  - odin
  - migration
generated:
  by: process:reasonix
  at: "2026-10-19T00:00:00Z"
---

# Odin WASM plugin port candidates (Lua -> Odin)

Status: **4 dari 6 kandidat utama sudah diport ke Odin** dan diverifikasi live. Ini catatan kandidat yang sudah ada plus calon berikutnya. Semua data dari scan 32 plugin Lua + build/test Odin.

## Yang sudah jadi (terverifikasi A/B identical dengan Lua)

| plugin | size `main.wasm` | helpers yang dipakai | verifikasi |
| --- | ---: | --- | --- |
| `atsumaru` | 414 KB | `text.url_encode`, `text.strip_html`, `text.normalize_status`, `text.date_to_iso`, `text.chapter_num`, `http_body` | search/detail/chapters/pages A/B identical |
| `comicasura` | 417 KB | `regex.find_all`, `regex.find`, `text.chapter_num`, `text.titlecase`, `text.strip_html`, `text.normalize_status`, `text.url_encode`, `http_body` | A/B identical |
| `comizy` | 416 KB | `text.url_encode`, `text.normalize_status`, `text.date_to_iso`, `http_body` | A/B identical, pages 22 img verified vs site |
| `mangade` | 305 KB | `text.url_encode`, `text.normalize_status`, `http_body` | A/B identical |
| `mangadex` | 321 KB | `text.url_encode`, `text.normalize_status`, `text.date_to_iso`, `http_body` | A/B identical, 36 manga / 1452 chapter |
| `manga_uno` | 416 KB | `text.url_encode`, `text.normalize_status`, `text.date_to_iso`, `http_body` | A/B identical |

Kontrak ABI: semua export `contract_version/malloc/free/Search/GetMangaDetail/GetChapterList/GetPageList` + import `env.host_call`. Build satu perintah: `odin build . -target:wasi_wasm32 -no-entry-point -out:main.wasm`.

## Ranking kandidat migrasi selanjutnya

### Tier A — tanpa regex, JSON+HTTP saja (paling mudah)

| plugin | lines | helpers | catatan |
| --- | ---: | --- | --- |
| `mangatown` | 108 | `text.url_encode`, `http.get_body`, `json.*`, `os.clock` | terkecil setelah mangade; pure JSON API |
| `mangahere` | 159 | sama | — |
| `fanfox` | 165 | sama | — |
| `mangafreak` | 161 | `+ regex.find` | sudah supported |
| `kaliscan` | 209 | `+ regex.find` | sudah supported |
| `zeroscann` | 239 | `text.url_encode` + `html.parse` | scraping, tapi pakai host.html.* |
| `weebcentral` | 283 | host.html.*, `os.clock` | — |
| `valirscans` | 225 | `regex.find_all/find_index/quote` | semua sudah supported |
| `mangafire` | 308 | `crypto.vrf_sign`, `os.clock` | sudah ada di wasm |

### Tier B — pakai `browser.*` (baru di-port, perlu verifikasi live)

`violetmanga` (174), `mangataro` (223), `comix` (291). Semua helper
`browser.fetch/evaluate/evaluate_with_init` sudah ter-wire di
`wasm_host_dispatch.go`, tapi belum ada port Odin yang pakai — jadi perlu proof
di situs sebenarnya.

### Tier C — pakai `regex.gmatch` (perlu rewrite ke `regex.find_all`)

`dynastyscan`, `en-thunderscan`, `harimanga`, `lhtranslation`, `madaradex`,
`mangaread`, `mangasushi`, `topmanhua`, `anisascans`, `mangabuddy`,
`mangakatana`, `mangahub` (423), `1manga` (511). `regex.gmatch` adalah Lua
iterator tanpa setara JSON — gap yang didokumentasikan di `wasmHelperGaps`.

## Catatan host

- `os.clock()` (budget waktu di search Lua) diganti Odin dengan
  `core:time.tick_now()`/`tick_since()` — sudah dipakai mangadex odin.
- `normalize_status`: semua port Odin pakai `abi.host_str("text.normalize_status", raw)`
  kecuali `manga_uno_odin` yang pakai substring sendiri (mirror ke C++ twin).
  Untuk port baru, pakai host helper biar konsisten.
- `comizy`/`mangade`: API pakai `Accept: application/json` — set di `kHeaders`.
- `mangadex`: cover CDN perlu `Referer: https://uploads.mangadex.org/` di `kHeaders`.

## Performance truth (bukan asumsi)

A/B mangadex detail (cache clear tiap request):
- wasm: ~0.18s steady (network-dominant, 687-byte JSON response)
- Lua: ~0.075s steady
Jadi **di JSON API endpoint, wasm tidak lebih cepat** — perbedaan hanya terasa di
endpoint HTML berat (search 300+KB + regex parsing), di situ Odin kompilasi
ungguli interpreter Lua.
