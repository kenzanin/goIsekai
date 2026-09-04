# Playground: dummy (test plugin, semua runtime)

Plugin `dummy` ada dalam 3 runtime (wasm/lua/js di `examples/plugins/*/dummy/`) untuk uji infrastruktur plugin tanpa akses internet — ABI call yang langsung return data statis.

```bash
# search — plugin dummy umumnya mengabaikan query
curl -s 'http://127.0.0.1:8080/api/sandbox/plugins/dummy/search?q=test'
```

Kalau dummy tidak ter-install di `app_data/plugins/`, load dari path eksternal:

```bash
# WASM dummy
curl -s -X POST http://127.0.0.1:8080/api/sandbox/plugins/load \
  -H 'Content-Type: application/json' \
  -d '{"path":"examples/plugins/wasm/dummy/dummy.wasm"}'

# Lua dummy (folder)
curl -s -X POST http://127.0.0.1:8080/api/sandbox/plugins/load \
  -H 'Content-Type: application/json' \
  -d '{"path":"examples/plugins/lua/dummy"}'

# JS dummy (folder)
curl -s -X POST http://127.0.0.1:8080/api/sandbox/plugins/load \
  -H 'Content-Type: application/json' \
  -d '{"path":"examples/plugins/js/dummy"}'

# unload / reload
curl -s -X POST http://127.0.0.1:8080/api/sandbox/plugins/dummy/unload
curl -s -X POST http://127.0.0.1:8080/api/sandbox/plugins/dummy/reload
```

Detail/chapters/pages mengikuti endpoint yang sama seperti plugin lain (`detail/{mangaID}`, `chapters/{mangaID}`, `pages/{chapterID}`) dengan ID yang dikembalikan search dummy.
