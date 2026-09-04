# Playground: mangzio (JS plugin)

Cara menguji plugin `mangzio` lewat sandbox API playground. Server harus jalan (default `http://127.0.0.1:8080`).

Chapter ID mengandung `:` — percent-encode (`%3A`) di URL curl.

## List plugin

```bash
curl -s http://127.0.0.1:8080/api/sandbox/plugins/
```

## Search (return-all; host slices per search_page_size)

```bash
curl -s 'http://127.0.0.1:8080/api/sandbox/plugins/mangzio/search?q=isekai'
```

Contoh manga id: `the-greatest-estate-developer`

## Detail manga

```bash
curl -s http://127.0.0.1:8080/api/sandbox/plugins/mangzio/detail/the-greatest-estate-developer
```

## Chapter list (newest-first)

```bash
curl -s http://127.0.0.1:8080/api/sandbox/plugins/mangzio/chapters/the-greatest-estate-developer
```

Contoh chapter id: `the-greatest-estate-developer:chapter-223.9`

## Page list

```bash
curl -s 'http://127.0.0.1:8080/api/sandbox/plugins/mangzio/pages/the-greatest-estate-developer%3Achapter-223.9'
```

## Hot reload setelah edit main.js

```bash
curl -s -X POST http://127.0.0.1:8080/api/sandbox/plugins/mangzio/reload
curl -s 'http://127.0.0.1:8080/api/sandbox/plugins/mangzio/search?q=estate' | head -c 200
```
