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

| plugin | LOC total | helpers | catatan |
| --- | ---: | --- | --- |
| `mangafreak` | 365 | `text.url_encode`, `http.get_body`, `json.*`, `os.clock`, `+ regex.find` | **kandidat terbaik berikutnya**: paling kecil, `gmatch` 8× perlu rewrite |
| `kaliscan` | 373 | `+ regex.find` | `gmatch` 4× |
| `mangahere` | 393 | sama | `gmatch` 5× |
| `mangatown` | 406 | `text.url_encode`, `http.get_body`, `html.*`, `os.clock` | `gmatch` 7×, plus chapterfun + Dean-Edwards unpacker |
| `fanfox` | 431 | sama | `gmatch` 6× |
| ~~`mangafire`~~ | 371 | `crypto.vrf_sign`, `os.clock` | **SELESAI** diport 2026-10-19 — 0 `gmatch`, lihat bukti A/B di atas |
| ~~`weebcentral`~~ | 390 | host.html.*, `os.clock` | **SELESAI** diport 2026-10-19 |
| ~~`zeroscann`~~ | 239 | `text.url_encode` + `html.parse` | **DITOLAK** — situs 500 (butuh PHP 8) |
| ~~`valirscans`~~ | 225 | `regex.find_all/find_index/quote` | **DITOLAK** — sandbox balik `null` |

**Faktor penentu sebenarnya: `gmatch`, bukan LOC.** `mangafire` menang sebagai
port berikutnya karena satu-satunya kandidat dengan **0 `gmatch`** (gap wasm yang
terdokumentasi di `wasmHelperGaps`), meski bukan yang terkecil. Semua kandidat
tersisa butuh rewrite `gmatch` → `regex.find_all` (4-8 situs per plugin), jadi
ukuran file tidak lagi menjadi patokan utama.

Tersisa yang benar-benar kandidat: `mangafreak`, `kaliscan`, `mangahere`,
`mangatown`, `fanfox` — semuanya butuh rewrite `gmatch`.

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
