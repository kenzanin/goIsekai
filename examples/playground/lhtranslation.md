# Playground: lhtranslation (Lua plugin)

Cara menguji plugin `lhtranslation` lewat sandbox API playground. Server harus jalan (default `http://127.0.0.1:8080`).

Plugin ini proof-of-concept runtime **Lunar** (Lua 5.4) untuk site WordPress Madara (https://lhtranslation.net).

## Load / reload

Plugin di-scan otomatis dari `app_data/plugins/lhtranslation/`. Untuk load manual dari path lain atau reload setelah edit:

```bash
curl -s -X POST http://127.0.0.1:8080/api/sandbox/plugins/load \
  -H 'Content-Type: application/json' \
  -d '{"path":"/path/to/lhtranslation"}'
curl -s -X POST http://127.0.0.1:8080/api/sandbox/plugins/lhtranslation/reload
```

## Search (page size 24, paged via `paged=`)

```bash
curl -s 'http://127.0.0.1:8080/api/sandbox/plugins/lhtranslation/search?q=isekai'
curl -s 'http://127.0.0.1:8080/api/sandbox/plugins/lhtranslation/search?q=massage&page=2'
```

Contoh manga id: `isekai-harem-massage-kami-no-shiatsu-de-nayameru-monster-musume-tachi-o-momihogusu`

## Detail manga

```bash
curl -s http://127.0.0.1:8080/api/sandbox/plugins/lhtranslation/detail/isekai-harem-massage-kami-no-shiatsu-de-nayameru-monster-musume-tachi-o-momihogusu
```

Mengembalikan title, author, status (dinormalisasi: Ongoing/Completed/...), genres, cover_url, description.

## Chapter list (newest-first via POST /manga/SLUG/ajax/chapters/)

```bash
curl -s http://127.0.0.1:8080/api/sandbox/plugins/lhtranslation/chapters/isekai-harem-massage-kami-no-shiatsu-de-nayameru-monster-musume-tachi-o-momihogusu
```

Contoh chapter id: `isekai-harem-massage-kami-no-shiatsu-de-nayameru-monster-musume-tachi-o-momihogusu:chapter-3`

## Page list

```bash
# %3A = ':' yang di-encode
curl -s 'http://127.0.0.1:8080/api/sandbox/plugins/lhtranslation/pages/isekai-harem-massage-kami-no-shiatsu-de-nayameru-monster-musume-tachi-o-momihogusu%3Achapter-3'
```

## Image via proxy

```bash
curl -o /tmp/cover.jpg 'http://127.0.0.1:8080/image?pluginID=lhtranslation&url=https%3A%2F%2Flhtranslation.net%2Fwp-content%2Fuploads%2F2026%2F03%2Fisekai-harem-193x278.jpg'
```

## Catatan Madara

- Chapter list **bukan** admin-ajax: `POST {base}/manga/SLUG/ajax/chapters/` (di temukan di `madara-core/assets/js/script.js`)
- Cover & page images pakai atribut `data-src` (lazyload), bukan `src` (placeholder `dflazy.jpg`)
- Search: `/?s=QUERY&post_type=wp-manga`, halaman lanjut `&paged=N`
- Tanggal chapter format `July 7, 2026` → dikonversi ke RFC3339 di plugin
