# Playground: kaliscan (Lua plugin)

Cara menguji plugin `kaliscan` lewat sandbox API playground. Server harus jalan (default `http://127.0.0.1:8080`).

Catatan ID: chapter ID kaliscan mengandung `:` (contoh `slug:chapter-12`). Chrome meng-encode `:` sebagai `%3A` di URL path — selalu percent-encode ID di curl (`%3A`), host otomatis decode-nya.

## List plugin

```bash
curl -s http://127.0.0.1:8080/api/sandbox/plugins/
```

## Search (akumulasi semua halaman upstream, page size 48)

```bash
curl -s 'http://127.0.0.1:8080/api/sandbox/plugins/kaliscan/search?q=isekai'
curl -s 'http://127.0.0.1:8080/api/sandbox/plugins/kaliscan/search?q=cleric&page=2'
```

Contoh manga id: `14974-a-hot-wet-job-for-three-adult-toy-tester-`

## Detail manga

```bash
curl -s http://127.0.0.1:8080/api/sandbox/plugins/kaliscan/detail/14974-a-hot-wet-job-for-three-adult-toy-tester-
```

## Chapter list (newest-first)

```bash
curl -s http://127.0.0.1:8080/api/sandbox/plugins/kaliscan/chapters/14974-a-hot-wet-job-for-three-adult-toy-tester-
```

Contoh chapter id: `14974-a-hot-wet-job-for-three-adult-toy-tester-:chapter-12`

## Page list

```bash
# %3A = ':' yang di-encode
curl -s 'http://127.0.0.1:8080/api/sandbox/plugins/kaliscan/pages/14974-a-hot-wet-job-for-three-adult-toy-tester-%3Achapter-12'
```

## Hot reload setelah edit main.lua / util.lua

Plugin ini Lua (Lunar VM). Edit file di `app_data/plugins/kaliscan/`, lalu:

```bash
curl -s -X POST http://127.0.0.1:8080/api/sandbox/plugins/kaliscan/reload
curl -s 'http://127.0.0.1:8080/api/sandbox/plugins/kaliscan/search?q=cleric' | head -c 200
```

## Load plugin dari path eksternal

```bash
curl -s -X POST http://127.0.0.1:8080/api/sandbox/plugins/load \
  -H 'Content-Type: application/json' \
  -d '{"path":"examples/plugins/lua/kaliscan"}'

# unload (revert ke deferred; auto lazy-load lagi saat dipakai)
curl -s -X POST http://127.0.0.1:8080/api/sandbox/plugins/kaliscan/unload
```
