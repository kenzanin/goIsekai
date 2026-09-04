# Playground: mangafire (WASM plugin)

Cara menguji plugin `mangafire` lewat sandbox API playground. Server harus jalan (default `http://127.0.0.1:8080`).

## List plugin

```bash
curl -s http://127.0.0.1:8080/api/sandbox/plugins/
```

## Search (upstream cap limit=50 per query)

```bash
curl -s 'http://127.0.0.1:8080/api/sandbox/plugins/mangafire/search?q=one+piece'
```

Contoh manga id: `mompv`

## Detail manga

```bash
curl -s http://127.0.0.1:8080/api/sandbox/plugins/mangafire/detail/mompv
```

## Chapter list (newest-first)

```bash
curl -s http://127.0.0.1:8080/api/sandbox/plugins/mangafire/chapters/mompv
```

Contoh chapter id: `8818808`

## Page list

```bash
curl -s http://127.0.0.1:8080/api/sandbox/plugins/mangafire/pages/8818808
```

## Hot reload setelah rebuild WASM

```bash
make build-mangafire   # rebuild + install ke app_data/plugins/
curl -s -X POST http://127.0.0.1:8080/api/sandbox/plugins/mangafire/reload
curl -s 'http://127.0.0.1:8080/api/sandbox/plugins/mangafire/search?q=one+piece' | head -c 200
```

## CDP helper (kalau kena challenge/captcha)

```bash
curl -s http://127.0.0.1:8080/api/sandbox/cdp/status
curl -s http://127.0.0.1:8080/api/sandbox/cdp/test
curl -s 'http://127.0.0.1:8080/api/sandbox/cdp/cookies?domain=mangafire.to'
```
