# Playground: mangadex (JS plugin)

Cara menguji plugin `mangadex` lewat sandbox API playground. Server harus jalan (default `http://127.0.0.1:8080`).

Semua endpoint di bawah `/api/sandbox/plugins/`.

## List plugin

```bash
curl -s http://127.0.0.1:8080/api/sandbox/plugins/
```

## Search

```bash
curl -s 'http://127.0.0.1:8080/api/sandbox/plugins/mangadex/search?q=isekai'
# dengan halaman (host-side slicing, default search_page_size=24):
curl -s 'http://127.0.0.1:8080/api/sandbox/plugins/mangadex/search?q=isekai&page=2'
```

Contoh hasil (dipotong):

```json
[{"id":"f9c33607-9180-4ba6-b85c-e4b5faee7192","title":"...","cover_url":"https://mangadex.org/covers/..."}]
```

## Detail manga

```bash
curl -s http://127.0.0.1:8080/api/sandbox/plugins/mangadex/detail/f9c33607-9180-4ba6-b85c-e4b5faee7192
```

## Chapter list (newest-first)

```bash
curl -s http://127.0.0.1:8080/api/sandbox/plugins/mangadex/chapters/f9c33607-9180-4ba6-b85c-e4b5faee7192
```

Contoh chapter id: `9f20eb08-abe9-4fb6-acde-5f85049fe24d`

## Page list

```bash
curl -s http://127.0.0.1:8080/api/sandbox/plugins/mangadex/pages/9f20eb08-abe9-4fb6-acde-5f85049fe24d
```

## Hot reload setelah edit main.js

Plugin ini pure JS (goja) — edit `app_data/plugins/mangadex/main.js`, lalu:

```bash
curl -s -X POST http://127.0.0.1:8080/api/sandbox/plugins/mangadex/reload
# lalu search lagi untuk verifikasi
curl -s 'http://127.0.0.1:8080/api/sandbox/plugins/mangadex/search?q=isekai' | head -c 200
```
