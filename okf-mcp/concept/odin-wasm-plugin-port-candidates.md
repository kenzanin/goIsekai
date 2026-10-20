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

Status: **8 plugin sudah diport ke Odin** dan diverifikasi live. Data dari scan
32 plugin Lua + build/test Odin + A/B live per endpoint.

## Yang sudah jadi (terverifikasi A/B identical dengan Lua)

| plugin | size `main.wasm` | helpers yang dipakai | verifikasi |
| --- | ---: | --- | --- |
| `atsumaru` | 414 KB | `text.url_encode`, `text.strip_html`, `text.normalize_status`, `text.date_to_iso`, `text.chapter_num`, `http_body` | search/detail/chapters/pages A/B identical |
| `comicasura` | 417 KB | `regex.find_all`, `regex.find`, `text.chapter_num`, `text.titlecase`, `text.strip_html`, `text.normalize_status`, `text.url_encode`, `http_body` | A/B identical |
| `comizy` | 416 KB | `text.url_encode`, `text.normalize_status`, `text.date_to_iso`, `http_body` | A/B identical, pages 22 img verified vs site |
| `mangade` | 305 KB | `text.url_encode`, `text.normalize_status`, `http_body` | A/B identical |
| `mangadex` | 321 KB | `text.url_encode`, `text.normalize_status`, `text.date_to_iso`, `http_body` | A/B identical, 36 manga / 1452 chapter |
| `mangafire` | 428 KB | `crypto.vrf_sign`, `text.url_encode`, `text.unescape`, `text.strip_html`, `text.normalize_status`, `text.date_to_iso`, `http_body` | detail identical, chapters 79/79 set+num identical, search superset |
| `manga_uno` | 416 KB | `text.url_encode`, `text.normalize_status`, `text.date_to_iso`, `http_body` | A/B identical |
| `weebcentral` | 417 KB | `html.*`, `regex.find/find_all/replace`, `text.url_encode`, `text.normalize_status`, `text.chapter_num` | search 32/32 + detail all fields + chapters 574/574 identical |
| `violetmanga` | 301 KB | `html.find_*`, `regex.find/find_all/replace`, `text.url_encode`, `text.normalize_status` | search 2307 B + detail all fields + pages identical; chapters 13/13 id+title identical, `chapter_num` 0→13..1 |
| `dynastyscan` | 419 KB | `html.*`, `regex.find/find_all`, `text.unescape/strip_html/normalize_status/date_to_iso/chapter_num`, `http_body` | search 1257 B + detail 656 B + pages 2023 B identical; chapters 32/32; `GetGenres` 120 genre live |
| `mangataro` | 410 KB | `html.*`, `text.url_encode/normalize_status/date_to_iso/chapter_num/sub`, `crypto.md5_hex`, `http_body` | search 3331 B + detail 415 B + pages 6291 B identical; chapters 88/88 id+order identical, `released_at` 0→nyata; `GetGenres` live |
| `lhtranslation` | 415 KB | `regex.find/find_all/find_all_pairs/replace`, `text.unescape/strip_html/normalize_status/date_to_iso/chapter_num`, `http.get_body`/`http.post_body` | search 521 B + detail 909 B identical; pages 33/33 url identical; chapters 4/4 id+order identical |
| `mangakatana` | 409 KB | `html.find_text/find_attr/find_list_text/find_list_attr`, `regex.find/find_all/sub`, `text.url_encode/normalize_status/unescape`, `http.get_body` | **search 2696 B + detail 749 B + pages 1197 B + chapters 1203 B semuanya IDENTICAL** |
| `mangaread` | 413 KB | `regex.find/find_all/find_all_pairs/sub`, `text.unescape/strip_html/normalize_status/date_to_iso/chapter_num/url_encode`, `http.get_body`/`http.post_body` | **search 2483 B + detail 767 B + pages 3868 B IDENTICAL**; chapters 22/22 id+order, `manga_id` `""`→slug |

### mangaread (port 2026-10-20) — Madara kedua, kartu search per-blok

264 LOC. Madara kedua setelah `lhtranslation`, jadi jebakannya sudah diketahui dan
diperlakukan sama: **chapter list hanya ada di balik
`POST /manga/<slug>/ajax/chapters/`** (diverifikasi live: 22 `wp-manga-chapter`
di respons AJAX, sedangkan halaman detail tidak memuatnya).

A/B dengan `plugin_cache` dibersihkan:

| endpoint | hasil |
| --- | --- |
| search | **IDENTICAL** (2483 B) |
| detail | **IDENTICAL** (767 B) |
| pages | **IDENTICAL** (3868 B) |
| chapters | 22/22 id + urutan sama; `manga_id` `""` → slug |

**Perbedaan penting dari `lhtranslation`: kartu search harus diparse per-blok.**
Markup-nya satu kartu per `div.row.c-tabs-item__content` (18 blok untuk query
uji), bukan `<a title=...>` seperti di lhtranslation. Dua kegagalan nyata kalau
tidak begitu:

1. Kemunculan **pertama** sebuah slug di halaman adalah `og:url` di `<head>`, jadi
   scan jendela ±3000 karakter di sekitar slug **melewatkan kartu** dan hasil
   pertama kehilangan cover-nya.
2. Jendela yang terlalu lebar membuat regex fallback **mencuri cover kartu
   tetangga**.

Jadi blok diambil sampai penutup `post-title`, dan cover selalu berada sebelum
judul di dalam blok yang sama.

Path genre juga beda: lhtranslation `/manga-genre/<slug>/`, mangaread
`/genres/<slug>/`, dan halaman ke-N adalah `/genres/<slug>/page/N/` (bukan
`?page=N`). `GetGenres` statis 37 entri; diverifikasi live (cache NULL →
**1369 B**).


### mangakatana (port 2026-10-20) — empat endpoint identik, dan cara membaca `page_urls`

265 LOC. **Hasil terbersih dari seluruh antrean: keempat endpoint byte-IDENTICAL**
dengan Lua, tanpa satu pun delta yang disengaja. A/B dengan `plugin_cache`
dibersihkan:

| endpoint | hasil |
| --- | --- |
| search | **IDENTICAL** (2696 B) |
| detail | **IDENTICAL** (749 B) |
| pages | **IDENTICAL** (1197 B) |
| chapters | **IDENTICAL** (1203 B) |

**Kontrak id (jangan diubah — host menempelkannya ke path URL):**
manga id = `<slug>` (`naruto.1205`), chapter id = `<slug>:<cXXX>`
(`naruto.1205:c700`). Segmen path tidak boleh mengandung slash, itu sebabnya
chapter id memakai titik dua dan `chapter_url` mengembalikannya jadi
`/manga/<slug>/<cXXX>`.

**Halaman baca tidak memuat URL gambar yang bisa dipakai.** Setiap `<img>`
punya `data-src="#"` dan JavaScript menyalin URL asli ke sana; daftar aslinya ada
di literal array JS (`var thzq=[...]`). Strategi port (sama seperti Lua): ambil
**array dengan URL `http` terbanyak** di halaman dan **abaikan nama variabelnya**,
sehingga rename variabel obfuscated tidak memutuskan plugin. Diverifikasi live:
37 URL gambar terangkat, `data-src` muncul 38×.

**Tiga hal lain yang diverifikasi live, bukan diasumsikan:**

1. Selector detail memang ada, tapi class-nya gabungan: `.value authors` dan
   `.value status completed` — bukan `class="authors"`. Cek `grep -c
   'class="authors"'` menghasilkan **0** dan sempat menyesatkan; yang benar
   `h.count('authors')` di dalam `div class="d-cell-small value authors"`.
2. Tabel chapter ada di halaman detail itu sendiri (6 baris untuk slug uji),
   selector `div.chapters table tbody tr td div.chapter a`, sudah newest-first.
3. `#related` bersarang di dalam `#single_book` dan mengulang bentuk kartu
   search, jadi semua lookup detail harus di-scope.

**Satu-satunya penyesuaian:** deskripsi dibaca sebagai `<p>` di dalam `div.summary`
dengan `<br>` → newline, karena `html.xpath_list_text(.../p/text())` yang dipakai
Lua **tidak diekspos ke wasm** (tidak ada `xpath*` di `abi/`). Kedua bentuk sepakat
pada markup live — paragraf itu tidak memuat `<br>` sama sekali. Nomor halaman
tetap 1-based seperti `ipairs` Lua, bukan 0.

`GetGenres` statis 27 entri; diverifikasi live (cache NULL → **987 B**).


### lhtranslation (port 2026-10-20) — Madara, chapter list hanya lewat AJAX POST

258 LOC. WordPress + tema Madara, **semua baca itu scrape** — tidak ada JSON API.
A/B dengan `plugin_cache` dibersihkan:

| endpoint | hasil |
| --- | --- |
| search | **IDENTICAL** (521 B) |
| detail | **IDENTICAL** (909 B) |
| pages | 33/33 url + urutan sama; `index` 0→0..32 |
| chapters | 4/4 id + urutan sama; `manga_id` + `released_at` |

**Yang wajib diketahui sebelum menyentuh plugin Madara mana pun:**

1. **Halaman detail TIDAK memuat `li.wp-manga-chapter` sama sekali.** Madara
   memuat daftar chapter lewat AJAX:
   `POST /manga/<slug>/ajax/chapters/` dengan header `X-Requested-With:
   xmlhttprequest`. Scrape halaman detail untuk chapter akan **diam-diam
   menghasilkan nol** — sudah dibuktikan live (count marker = 0), bukan dugaan.
2. **`img[data-src]` diawali tab dan newline.** URL hanya valid setelah di-trim.
3. Halaman detail punya dua blok judul mirip-`<h1>` (judul situs + judul manga);
   yang benar ada di dalam `div.post-title`, jadi blok itu harus di-scope dulu.

**Bug di sumber Lua yang ketahuan:** genre dikirim sebagai nama yang
di-URL-encode (`Gender Bender` → `/manga-genre/Gender%20Bender/`), dan situs
menjawab **404** — diverifikasi live, sementara `gender-bender` → 200. Port ini
menurunkan slug (`lower`, spasi→hyphen) sehingga filter itu berfungsi; 45 genre
lain tidak berubah (`Action` dan `action` sama-sama 200).

**Delta yang disengaja:** `pages[].index` diisi 0..N-1. Lua tidak mengirim field
itu sama sekali, dan host mengisinya 0 untuk semua baris (33 halaman → 33 nol).

**Catatan `released_at`:** chapter terbaru memakai tanggal relatif di situs
("20 hours ago"), jadi `text.date_to_iso` me-resolve-nya terhadap jam sekarang dan
nilainya bergeser tiap pemanggilan — dua kali berturut-turut memberi
`15:56:54Z` lalu `15:56:56Z`. Ini sifat situs, bukan regresi: chapter dengan
tanggal absolut (`chapter 1..3`) cocok persis dengan Lua.

`GetGenres` tetap statis (45 nama) dengan slug yang diturunkan; diverifikasi live
dengan me-NULL-kan `plugins.genres` → terisi **1672 B**.


### mangataro (port 2026-10-20) — bukti A/B + bug walltime di host

223 LOC. Header Lua bilang chapter list dan page list butuh
`host.browser.evaluate` (scroll-poll harvest), **tapi kodenya tidak pernah
memanggilnya**: chapter lewat AJAX bertoken, page lewat endpoint konten. Port ini
murni HTTP.

A/B dengan `plugin_cache` dibersihkan:

| endpoint | hasil |
| --- | --- |
| search | **IDENTICAL** (3331 B) |
| detail | **IDENTICAL** (415 B) |
| pages | **IDENTICAL** (6291 B) |
| chapters | 88/88 id + urutan sama; satu delta disengaja (`released_at`) |

Delta `chapters`: **`released_at` diisi** (`0001-01-01T00:00:00Z` →
`2026-10-13T02:43:38Z`). Sumbernya field `date` dari API yang berupa frasa relatif
("2 years ago"); `text.date_to_iso` me-resolve-nya jadi tanggal nyata. Lua
membuangnya.

**Token chapter.** Endpoint `GET /auth/manga-chapters?manga_id=<id>&_t=<token>&_ts=<unix>`
memakai token client-side: `md5(unix_ts .. "mng_ch_" .. YYYYMMDDHH)[:16]`, dengan
`YYYYMMDDHH` = jam UTC. Jam UTC diturunkan dari `text.date_to_iso` (yang
memformat UTC), bukan dari date lokal — sama seperti Lua. Formula ini diverifikasi
manual lewat shell dulu (`http=200`, 47 739 B) sebelum ditulis ke Odin.

#### Bug host: jam WASI beku di 2022-01-01

Percobaan pertama menghasilkan `chapters = []` dan log host:

```
url=.../auth/manga-chapters?manga_id=341206&_t=9d1040ef12ace9f8&_ts=1640995200 status=403
```

`1640995200` = **2022-01-01**. Penyebabnya bukan plugin: **wazero memberi jam
palsu kalau walltime tidak dikonfigurasi.** `internal/sys/sys.go:164` memakai
`platform.NewFakeWalltime()` bila `walltime == nil`, dan
`internal/platform/time.go:12` menetapkan `FakeEpochNanos = 1640995200000*ms`
alias 2022-01-01, naik 1 ms tiap pembacaan. Host tidak pernah memanggil
`WithSysWalltime()`, jadi **setiap** plugin wasm yang membaca jam dinding menerima
waktu beku.

Perbaikan di `internal/pluginmanager/wasm.go` — satu method di module config:

```go
mod, err := rt.InstantiateModule(m.ctx, compiled, wazero.NewModuleConfig().
    WithName(id).
    WithStartFunctions().
    WithSysWalltime())
```

Setelah itu chapters langsung `16175 B`, identik dengan baseline Lua (sebelumnya
`3 B`). **Pelajaran:** probe "build OK" untuk `time.now()` hanya membuktikan
kompilasi, **bukan** runtime — jam baru terbukti benar saat token diterima server.
Gejalanya menyesatkan: `403` polos tanpa petunjuk apa pun soal jam.

#### Statis vs scrape untuk genres

`GetGenres` tetap **statis** (15 tag) seperti Lua, bukan scrape `/tag/`. Alasannya
sama dengan sumbernya: halaman tag memuat kategori dan pengelompokan situs yang
tidak semuanya diterima URL archive. Diverifikasi live: cache `plugins.genres`
terisi **603 B** setelah di-NULL-kan.

**Catatan:** pengingat yang sama seperti `dynastyscan` — `plugins.genres = "[]"`
berarti "tidak ada export genre, jangan panggil lagi", jadi kolomnya harus
di-NULL-kan agar `GetGenres` yang baru dipanggil.


### violetmanga (port 2026-10-20) — bukti A/B

Kandidat terkecil yang tersisa (174 LOC). A/B dengan `plugin_cache` dibersihkan:

| endpoint | hasil |
| --- | --- |
| search | **IDENTICAL** (2386 B) |
| detail | **IDENTICAL** (1077 B) |
| pages | **IDENTICAL** (1125 B) |
| chapters | 13/13 id + title + url identical; `chapter_num` 0→13..1 |

`chapter_num` adalah tambahan sengaja (Lua tidak mengisinya sama sekali sehingga
semua chapter tersortir sebagai 0), pola yang sama dengan port sebelumnya.

**Dua jebakan yang ditemukan saat port ini:**

1. **`author` sempat hilang dan A/B yang menangkapnya.** Komentar Lua bilang situs
   hanya punya "uploader, bukan mangaka", jadi versi pertama sengaja
   mengosongkan `author`. A/B menunjukkan Lua mengirim `"author":"Serab"` —
   selector `.imptdt span.author i` memang menghasilkan nilai. **Jangan percaya
   komentar sumber; percaya payload A/B.** Sudah diperbaiki dan detail jadi
   identical.
2. **Id chapter mengandung `/`, jadi route sandbox `pages/{chapterID}` (satu
   segmen chi) hanya bisa diuji dengan URL-encode.** `curl .../pages/<id>`
   mentah membalas `404 page not found` — itu batasan route sandbox, bukan bug
   plugin. Pakai `python3 -c "urllib.parse.quote(id, safe='')"`. `id` sendiri
   **harus** tetap relative path (`/slug-chapter-N/`) karena route reader
   meng-URL-encode-nya; menyimpan URL absolut akan merusaknya.


### dynastyscan (port 2026-10-20) — bukti A/B

201 LOC, tapi hampir semuanya JSON — sumber Lua sudah memanfaatkannya:
`/series/<slug>.json` membawa detail **dan** chapter list (`taggings`), dan
`/chapters/<permalink>.json` membawa page list. Hanya search dan index genre yang
HTML. A/B dengan `plugin_cache` dibersihkan:

| endpoint | hasil |
| --- | --- |
| search | **IDENTICAL** (1257 B) |
| detail | **IDENTICAL** (656 B) |
| pages | **IDENTICAL** (2023 B) |
| chapters | 32/32 id + title + url sama; 3 delta disengaja (di bawah) |

Delta `chapters` yang disengaja:

1. **`manga_id` diisi** (`""` → `if_you_could_see_love`). Lua mengosongkannya.
2. **`released_at` diisi** dari `taggings[].released_on` (`0001-01-01T00:00:00Z`
   → tanggal nyata). Lua membuang field ini, jadi host menerima zero time.
3. **Urutan dua chapter `chapter_num == 0`** (`volume_1_extras` vs
   `volume_2_extras`) tertukar. Ini **bukan** regresi: keduanya bernilai 0, dan
   `table.sort` Lua tidak stabil sehingga urutannya arbitrer. Port memakai
   `slice.stable_sort_by` → urutan feed situs dipertahankan (`volume_1`
   released 2021-03-11 sebelum `volume_2` 2021-06-28). Konvensi yang sama dipakai
   port Odin sebelumnya (`stable_sort_by` untuk tie).

**Jebakan utama — `find_all_pairs` vs `find_all` (baru, penting):**

`abi.find_all_pairs` men-decode `[][]string`, tetapi host merender baris dengan
**satu** capture group sebagai **string telanjang**, bukan array
(`wasmRegexRow` di `internal/pluginmanager/wasm_host_dispatch.go:144`). Pola
`(?s)<dd>(.*?)</dd>` punya satu grup, jadi decode `[][]string` gagal **tanpa
error** dan search mengembalikan `[]` kosong. Perbaikannya satu kata:
`abi.find_all` (decode `[]string`). Aturan praktisnya:

| jumlah capture group | helper |
| --- | --- |
| 1 | `abi.find_all` → `[]string` |
| 2+ | `abi.find_all_pairs` → `[][]string` |
| 0 | `abi.find_all` (host mengembalikan whole match) |

**`GetGenres` (opsional) juga diporting** dan terverifikasi live: `/tags` HTML
menghasilkan **120 genre unik** (JSON `/tags.json` juga ada, tapi payload-nya
dikelompokkan per huruf `{"A":[...]}` dan iterasi map tidak berurutan, jadi HTML
yang sudah alfabetis lebih baik). Bukti: cache `plugins.genres` terisi ulang
**5009 byte** setelah di-NULL-kan, dan `/view/search?pluginID=dynastyscan`
merender **162 `<option>`** (120 genre + header/lainnya).

**Catatan cache genre:** kolom `plugins.genres` menyimpan `"[]"` sebagai
"plugin ini tidak punya export genre — jangan pernah dipanggil lagi". Nilai itu
**bertahan** dari sebelum port, jadi `GetGenres` yang baru tidak akan pernah
dipanggil sampai kolomnya di-NULL-kan:
`UPDATE plugins SET genres=NULL WHERE id='dynastyscan';` Ini sama kelasnya dengan
`plugin_cache` dan mudah tertukar dengan "plugin tidak punya genre".

**HAR mengonfirmasi asumsi port.** `dynasty-scans.har` (16.9 MB, 143 entry)
berisi **0 request XHR/fetch**, dan `/search?q=isekai` mengembalikan dokumen
penuh — jadi search memang server-rendered, bukan JS. HAR juga menunjukkan
`/tags/isekai` dan `/tags/isekai?view=groupings` dihit browser, cocok dengan
jalur archive yang dipakai port saat filter genre aktif.

`CGO_ENABLED=0 go test ./internal/pluginmanager` hijau (23.1 s).


### weebcentral (port 2026-10-19) — bukti A/B

Measured, cache cleared first:
`search` 32/32 identical (id+title+cover), `detail` identical on title/author/
status/cover/genres(6)/description, `chapters` 574/574 identical on id+title+
`released_at`. One deliberate improvement: `chapter_num` is **574/574 filled**
where Lua left it 0 for every row — chapter IDs are untouched, so reading
progress carries over (same plugin id `weebcentral`, same `plugin.json` name).

### mangafire (port 2026-10-19) — bukti A/B

First Odin port that uses `crypto.vrf_sign` (the runtime row, JSON-shaped), so it
also proves that path works from wasm. Measured with `plugin_cache` cleared:
`detail` **identical on every field** (title/author/status/cover/genres/
description), `chapters` **79/79** with identical id set and identical
`chapter_num` sequence (79/79 `released_at` filled), `pages` 142 URLs with the
required `Referer` header. `search` returns 479 where Lua returns 50 — see the
"dua bug di sumber Lua" section: the first 50 are identical, Lua simply stops
after page 1. Search takes ~5 s because it walks all 10 API pages under an 8 s
client budget; that is inherent to "return everything, host paginates".


## Kandidat yang GUGUR saat dicek live (jangan dicoba lagi tanpa cek dulu)

| plugin | hasil cek 2026-10-19 | keputusan |
| --- | --- | --- |
| `zeroscann` | `GET /manga?keyword=the` → **HTTP 500**, `Fatal error: Composer detected issues ... PHP 7.4.33`, site butuh PHP >= 8 | **batal**, scaffold dihapus. Plugin Lua-nya juga balik `[]` |
| `valirscans` | sandbox `/search` balik `null` (bukan `[]`) | tunda sampai sumbernya diperbaiki |

**Pelajaran: ranking helper tidak cukup — cek situsnya dulu.** Keduanya
peringkat atas di daftar lama justru karena helper-nya paling sederhana, dan
keduanya mati/rusak. Satu `curl` ke URL search menghemat satu port utuh.

## Dua jebakan yang ditemukan saat port weebcentral

**1. Pola regex Lua tidak portabel apa adanya — `.` di Lua cocok dengan
newline, di RE2 tidak.** Teks baris chapter weebcentral membungkus lintas baris
(`"Chapter 1\n ... Last Read\n <tanggal>"`), jadi
`\s*Last Read.*$` yang **bekerja di Lua** tidak match sama sekali di RE2 dan
seluruh baris mentah bocor ke judul. Perbaikannya `(?s)`:
`(?s)\s*Last Read.*$`. Ini kelas bug yang lolos A/B cepat kalau hanya beberapa
baris pertama dilihat — yang menangkapnya adalah membandingkan **seluruh 574
judul**, bukan sampel.

**2. `plugin_cache` bisa membuat verifikasi port tampak gagal.** Setelah swap
Lua→wasm, `/chapters` masih mengembalikan payload **Lua yang lama** (5 baris
cache `GetChapterList`) sehingga `chapter_num` tampak 0/574 dan sempat
disimpulkan bug di kode Odin. Padahal host helper-nya benar (probe
`text.chapter_num` → 1, 12.5, 3) dan Odin-nya benar. **Selalu**
`DELETE FROM plugin_cache WHERE plugin_id='<id>';` **sebelum** A/B, kalau tidak
kita membandingkan plugin lama dengan dirinya sendiri.

Kontrak ABI: semua export `contract_version/malloc/free/Search/GetMangaDetail/GetChapterList/GetPageList` + import `env.host_call`. Build satu perintah: `odin build . -target:wasi_wasm32 -no-entry-point -out:main.wasm`.

## Ranking kandidat migrasi selanjutnya

### Tier A — tanpa regex, JSON+HTTP saja (paling mudah)

LOC di bawah adalah **total `*.lua` di folder plugin**, bukan hanya `main.lua` —
hampir semua plugin punya `util.lua` dan angka `main.lua` saja menyesatkan
(`mangatown` 108 baris `main.lua` = 406 baris total).

Sudah selesai / ditolak (jangan dihitung lagi):

| plugin | LOC total | status |
| --- | ---: | --- |
| ~~`mangafire`~~ | 371 | **SELESAI** diport 2026-10-19 — 0 `gmatch`, lihat bukti A/B di atas |
| ~~`weebcentral`~~ | 390 | **SELESAI** diport 2026-10-19 |
| ~~`zeroscann`~~ | 239 | **DITOLAK** — situs 500 (butuh PHP 8) |
| ~~`valirscans`~~ | 225 | **DITOLAK** — sandbox balik `null` |
| ~~`kaliscan`~~ | 373 | **DITOLAK 2026-10-20** — `kaliscan.me`/`kaliscan.io` balik `000` |

**Faktor penentu bukan `gmatch`, dan bukan LOC.** Koreksi 2026-10-20: klaim lama
"semua `gmatch` itu `host.regex.gmatch`" **salah**. Ada 4 situs `gmatch` native
(`1manga:103`, `mangahub:97`, `violetmanga:164`, `weebcentral:116`) — dan
`weebcentral` yang **sudah diport** membuktikan itu bukan halangan: Lua-nya
`body:gmatch('id="tag-([^"]+)"')`, Odin-nya `find_all(body, 'id="tag-([^"]+)"')`,
satu baris, pola sama. Jadi `gmatch` native = swap loop, bukan blocker.

Yang benar-benar menentukan **bentuk keluaran `regex.find_all` per jumlah capture**
(`pluginutil.RegexFindAll` + `wasmRegexRow`): 0 capture → tiap baris array 1 elemen
(`whole := re.NumSubexp() == 0`), 1 capture → `[]string` datar, N capture →
`[][]string`. Karena `abi.find_all` (1 capture) dan `abi.find_all_pairs`
(N capture) sekarang sudah dipusatkan, biaya rewrite itu **nol baris helper** —
tinggal pilih fungsi yang cocok. `violetmanga:164` polanya 0-capture
(`"https?:\\?/\\?/[^\"']+"`), jadi ia tetap masuk `find_all`.

**Klaim Tier B juga salah.** `violetmanga` dan `mangataro` **tidak memanggil
`host.browser.*` sama sekali** — kata `browser` hanya muncul di komentar. Keduanya
`host.html.*` + `host.http.get_body` + `host.json.*` murni. Karena itu
`violetmanga` (174 LOC) — **kandidat terkecil yang tersisa** — sebelumnya tidak
pernah masuk tabel Tier A padahal lebih kecil dari semua isinya.

**Tier A yang sudah dikoreksi (0 atau 2 `gmatch`, semua masih hidup):**

| plugin | LOC total | gmatch | helper inti | catatan |
| --- | ---: | ---: | --- | --- |
| ~~`violetmanga`~~ | 174 | 1 native (0-capture) | `html.find_*`, `regex.find/find_all/replace`, `normalize_status` | **SELESAI 2026-10-20** — lihat bukti A/B di atas |
| ~~`dynastyscan`~~ | 201 | 2 host | `html.*`, `regex.find`, `text.trim/unescape` | **SELESAI 2026-10-20** — lihat bukti A/B di bawah |
| `mangataro` | 223 | 0 | `html.*`, `json.*`, `crypto.md5_hex`, jam | **kandidat terbaik berikutnya**. Butuh jam dinding; `os.time()` Lua → `time.time_to_unix(time.now())` di Odin (probe wasi: **build OK**). Token `md5(ts .. "mng_ch_" .. jamUTC)`. Timpa `text.date_to_iso` untuk jam UTC |
| `lhtranslation` | 258 | 2 host | `regex.find/find_index/match`, `text.strip_html` | 22 `regex.find` — paling padat regex |
| `mangakatana` | 265 | 2 host | `html.*`, `regex.find/match` | |
| `mangafreak` | 365 | 8 host | `regex.find`, `json.*` | Terbesar di tier ini; 8 rewrite |

Tersisa yang masih kandidat nyata: **`dynastyscan` → `mangataro` →
`lhtranslation` → `mangakatana`**. `mangahere` (5 `gmatch`) / `mangatown` (6) /
`fanfox` (6) tetap di belakang: lebih besar dan butuh rewrite lebih banyak.

### Tier B — pakai `browser.*` (masih belum ada port Odin yang membuktikannya)

Setelah koreksi, **tidak ada** kandidat Tier-B murni di daftar ini: `violetmanga`
dan `mangataro` ternyata bukan pengguna `browser.*`. `comix` (291) satu-satunya
yang benar-benar memanggil `host.browser.*` (5 situs). Konsekuensinya pembuktian
jalur `browser.*` di wasm **belum punya bukti live** — kalau `comix` dijadwalkan,
itu sekaligus test pertama jalur tersebut.

### Tier C — `regex.gmatch` host

`en-thunderscan` (4), `harimanga` (5), `madaradex` (5), `mangaread` (4),
`mangasushi` (4), `topmanhua` (5), `anisascans` (4), `mangabuddy` (3),
`mangahub` (3, plus `string.gmatch` native), `1manga` (2, native),
`mangahere` (5), `mangatown` (6), `fanfox` (6). Semua ini sekarang **bukan**
blocker: `host.regex.gmatch` dan `string.gmatch` sama-sama menjadi satu loop
`for row in abi.find_all(...)`.

## Peluang lebih besar daripada port Lua: 7 plugin TinyGo yang kembaran C++-nya sudah ada

**SELESAI 2026-10-20.** Ketujuh kembaran C++ sekarang **sudah terpasang** di
`app_data/plugins/` dan lolos A/B live terhadap baseline Go. Bagian ini disimpan
sebagai catatan keputusan, bukan lagi sebagai pekerjaan tertunda.

Survei 2026-10-20 menemukan pekerjaan yang **tidak** perlu menulis kode baru sama
sekali. Waktu itu tujuh plugin wasm masih TinyGo/standard-Go dan kembaran C++-nya
sudah dibangun di `examples/`, tetapi yang terpasang masih versi Go:

| plugin | Go (baseline) | C++ terpasang | rasio |
| --- | ---: | ---: | ---: |
| `komix_to` | 2 110 343 | 199 712 | 10.6× |
| `mangak` | 1 899 228 | 205 446 | 9.2× |
| `ksgroupscans` | 2 186 868 | 218 509 | 10.0× |
| `kunmanga_online` | 2 122 986 | 207 102 | 10.3× |
| `mangakakalot` | 2 181 593 | 213 731 | 10.2× |
| `manhawato` | 2 164 283 | 199 197 | 10.9× |
| `reimanga_net` | 1 834 751 | 106 850 | 17.2× |
| **total** | **14 500 052** | **1 350 547** | **10.7×** |

Angka C++ di tabel ini **lebih kecil** dari survei awal (mis. `komix_to` 313 116 →
199 712) karena `build.sh` C++ ternyata menyimpan binary basi saat compile gagal
— lihat "Jebakan utama" di `wasm-cpp-plugin-toolchain.md`. Setelah `rm -f main.wasm`
ditambahkan ke kedelapan `build.sh`, semua binary dibangun ulang dan mengecil
dengan flag yang sama.

`manhawato` adalah kasus paling tajam: `wasm-cpp-plugin-toolchain.md` dulu
menyatakan `manhawato_cpp` "replaces the Go wasm in `app_data/plugins/manhawato/`",
tetapi `md5sum` membuktikan yang terpasang masih **versi Go**. Klaim itu salah,
sudah dikoreksi, dan sekarang baru benar-benar terpasang.

Hasil A/B: search/detail/chapters **identik** untuk 6 dari 7 plugin. `pages`
berubah `null` → `[]` di semua plugin, yang aman karena host men-decode ke
`[]types.Page` (`internal/pluginmanager/api_detail.go:97`). `reimanga_net` justru
**lebih baik**: versi Go mengembalikan "plugin unreachable" untuk detail/chapters/
pages, versi C++ mengembalikan data nyata (126 chapter termasuk desimal `116.5`,
author, cover, genres, 4 page).

Untuk sisa plugin yang **belum** punya kembaran C++, Odin dan C++ ternyata setara
dari sisi ukuran (Odin ~416 KB vs C++ ~200–313 KB vs TinyGo ~2.1 MB) — jadi port
Odin tetap pilihan yang lebih baik untuk plugin baru (tanpa wasi-sdk, satu perintah
build, `abi/` bersama). Prioritas historisnya: pasang C++ twin dulu (murah), baru
port Odin untuk plugin yang memang belum punya twin. Itu sudah dijalankan.



## Catatan host

- `os.clock()` (budget waktu di search Lua) diganti Odin dengan
  `core:time.tick_now()`/`tick_since()` — sudah dipakai mangadex odin.
- `normalize_status`: semua port Odin pakai `abi.host_str("text.normalize_status", raw)`
  kecuali `manga_uno_odin` yang pakai substring sendiri (mirror ke C++ twin).
  Untuk port baru, pakai host helper biar konsisten.
- `comizy`/`mangade`: API pakai `Accept: application/json` — set di `kHeaders`.
- `mangadex`: cover CDN perlu `Referer: https://uploads.mangadex.org/` di `kHeaders`.
- `weebcentral`: gambar di `official.lowee.us` **wajib** `Referer: https://weebcentral.com/`
  (di `kHeaders`), sedangkan `temp.compsci88.com` tidak butuh.

## Gotcha Odin (ditemukan saat port weebcentral)

**`core:slice` tidak punya `reverse_in_place`.** Build gagal dengan
`'reverse_in_place' is not declared by 'slice'`. Balik manual:

```odin
n := len(out)
for i in 0 ..< n / 2 {
	out[i], out[n - 1 - i] = out[n - 1 - i], out[i]
}
```

**Probe `json.unmarshal` ke struct bisa berbohong.** Mengetes
`struct{ n: f64 }` lalu unmarshal `"1"` menghasilkan
`Unsupported_Type_Error` — bukan karena `f64` bermasalah, tapi karena field
`n` huruf kecil = **unexported**. Unmarshal `f64` mandiri dari `"1"` → `1`,
`"12.5"` → `12.5`, jadi konversinya normal. Saat memprobe perilaku JSON Odin,
pakai tipe skalar langsung atau field **berhuruf besar**, kalau tidak probe-nya
menghasilkan kesimpulan yang salah.

**Debugging "helper host tidak jalan" — urutannya penting.** Saat `chapter_num`
tampak 0/574, tersangka pertama adalah kode Odin. Yang membuktikan sebaliknya
adalah probe Go di sisi host (`dispatchHostCall` → `text.chapter_num` → 1, 12.5,
3), lalu `plugin_cache` ditemukan menyajikan payload lama. Probe dulu **kedua**
sisi (host + plugin) sebelum menyalahkan salah satunya.

## Gotcha Odin (ditemukan saat port mangafire)

**`strings.to_string(sb)` TIDAK menyalin buffer — dan `defer builder_destroy`
membuatnya jadi dangling.** Ini yang paling mahal: `vrf_url` mengembalikan
`strings.to_string(sb)` sambil `defer strings.builder_destroy(&sb)` aktif, jadi
pemanggil menerima slice ke memori yang sudah dibebaskan. Gejalanya **sunyi**:
URL terlihat benar saat di-print, tapi fetch di dalam wasm gagal, `ok=false`,
dan endpoint membalas `[]` tanpa satu pun error di log host. Perbaikannya:
builder **dimiliki pemanggil**, helper menulis ke `^strings.Builder`, dan
destroy terjadi setelah string dipakai.

**Field struct JSON wajib huruf besar.** Sama seperti jebakan probe di atas,
tapi di kode produksi: `items`/`meta`/`hid` huruf kecil membuat unmarshal
melewati field itu **tanpa error sama sekali**, sehingga `len(resp.items) == 0`
dan search balik `[]`. Semua field wire type harus exported; tag `json:"..."`
yang memikul ejaan aslinya. Gejalanya identik dengan dangling string di atas —
dua bug berbeda, satu symptom yang sama, jadi **instrumentasi dulu** (kembalikan
URL/`ok` lewat payload valid) sebelum menebak.

**`slice.sort_by` tidak stabil.** `table.sort` Lua juga tidak stabil, tetapi
hasilnya kebetulan konsisten. Untuk tie (`chapter_num` 37.1 vs 37.1) urutan bisa
berubah antar-run; pakai `slice.stable_sort_by` kalau urutan sumber harus
dipertahankan.

## Dua bug di sumber Lua yang ketahuan dari A/B mangafire

A/B terhadap Lua menemukan Lua-nya yang salah, bukan port-nya — jadi port
**tidak** boleh menyalin perilaku ini:

| gejala | sebab di Lua | efek | perilaku Odin |
| --- | --- | --- | --- |
| search cuma 50 hasil | baca `meta.last_page` / `meta.has_next`; API kirim `lastPage` / `hasNext` | selalu berhenti di halaman 1 | 479 hasil, 50 pertama identik |
| `released_at` = `0001-01-01T00:00:00Z` | baca `c.created_at`; API kirim `createdAt` | tanggal selalu kosong | 79/79 tanggal asli |

Pelajaran: **A/B bukan sekadar "samakan dengan Lua"** — kalau Lua punya bug yang
tertutup oleh nilai default, port yang benar akan tampak "berbeda". Periksa
payload API mentah dulu untuk memutuskan mana yang benar.

## Performance truth (bukan asumsi)

A/B mangadex detail (cache clear tiap request):
- wasm: ~0.18s steady (network-dominant, 687-byte JSON response)
- Lua: ~0.075s steady
Jadi **di JSON API endpoint, wasm tidak lebih cepat** — perbedaan hanya terasa di
endpoint HTML berat (search 300+KB + regex parsing), di situ Odin kompilasi
ungguli interpreter Lua.

## Normalisasi helper duplikat ke `abi/` (8 plugin, terverifikasi)

Sebelum port berikutnya, wrapper host-helper yang sebelum ini disalin ulang di
tiap `main.odin` diangkat ke `abi/wasm_abi.odin` sekali. Tujuannya: port
selanjutnya tidak lagi menulis ulang percabangan `[]string` vs `[][]string`.

**Yang diangkat (13 proc, byte-identical di mana pun terduplikasi):**
`url_encode` (8 salinan identik, 31 call site), `find_all`, `find_all_pairs`,
`find_one`, `find_text`, `find_attr`, `find_list_text`, `find_list_attr`, `sub`,
`date_to_iso`, `host_num`, `chapter_num`, `sanitize_title`.

**Yang SENGAJA dibiarkan lokal (jangan diangkat):**
- `decode_id` — 4 bentuk berbeda (simple string-unmarshal, 3-shape mangafire,
  empty-on-error manga_uno, trim comizy)
- `normalize_status` — 4 bentuk beda; `manga_uno_odin` punya tangga substring
  lokal yang meniru kembaran C++-nya, bukan wrapper host
- `parse_time` — 2 bentuk

**Mengapa lewat delegator satu baris, bukan `using import`.** Odin **menolak**
`using import` (`Syntax Error: 'using import' is not allowed, please use the
import name explicitly`), jadi nama tak-berkualifikasi tidak bisa diimpor ke
scope plugin. Yang jalan adalah **alias prosedur**, dan itu membuat tiap plugin
tetap memakai nama lokalnya tanpa satu pun call site berubah:

```odin
url_encode :: proc (s: string) -> string { return abi.url_encode(s) }
find_all   :: proc (subject, pattern: string) -> []string { return abi.find_all(subject, pattern) }
```

Call site tetap `find_all(html, pat)` — nol churn di 31 `url_encode` + 10
`find_one` + 7 `find_list_attr` dst. `find_all` vs `find_all_pairs` tetap dua
fungsi terpisah karena host membalas pola 1-capture sebagai array datar dan
pola multi-capture sebagai array-of-array; unmarshal yang salah bentuk adalah
**hard error** di Odin (`Unsupported_Type_Error`), bukan nol diam-diam — jadi
pilih berdasar jumlah capture pola.

**8 plugin, satu salinan `abi/` per direktori plugin (tetap).** Tidak dipakai
satu `wasm/abi/` bersama: itu memaksa tiap `build.sh` dan baris `import`
berubah, dan karena `using import` dilarang tidak ada yang didapat. 8 salinan
`abi/` kini **identik** (satu md5); penyebarannya lewat skrip, bukan tangan.

**Bukti tidak ada regresi perilaku.** A/B yang benar untuk refactor murni bukan
wasm-vs-Lua (port sudah punya perbaikan sengaja yang berbeda dari Lua —
`chapter_num` 0→N, `manga_id` `''`→id), melainkan **wasm pra-normalisasi vs
wasm pasca-normalisasi**. Cache `plugin_cache` dibersihkan, host restart,
`weebcentral`:

| endpoint | hasil |
| --- | --- |
| search | IDENTICAL (5124 B) |
| detail | IDENTICAL (782 B) |
| chapters | IDENTICAL (101974 B) |

`VERDICT: normalization is behaviour-preserving`. Ke-8 plugin juga lolos
`build.sh` (validasi export `contract_version malloc free Search GetMangaDetail
GetChapterList GetPageList` + import `env.host_call`); ukuran naik tipis
(~+150 B) karena satu definisi `json.unmarshal_string` yang diperiksa error.

**Satu delta sengaja:** `host_num` kanonik menambah `|| raw == "null"`.
Perilakunya identik — `null` tetap gagal unmarshal ke `f64` dan mengembalikan 0
— jadi ini penyeragaman guard, bukan perubahan perilaku.
