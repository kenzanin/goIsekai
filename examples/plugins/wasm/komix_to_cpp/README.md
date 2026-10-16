# komix_to_cpp — WASM Plugin (C++ via Zig CC)

Minimal WASM plugin for **komix.to** (manga reader). C++ freestanding, compiled
with `zig cc` targeting `wasm32-freestanding`. No stdlib, no C++ runtime.

## Why Freestanding C++

Zig 0.18-dev does not yet ship `wasm32-wasip1` for `zig cc` in C/C++ mode.
The freestanding target works today and produces a bare-bones `.wasm` that the
goIsekai wazero host can load directly.

Trade-offs:

- No `std::string`, no `<map>`, no `<vector>` — all memory is a bump
  allocator over a 512 KB static heap.
- No JSON library (nlohmann/json etc.) — string manipulation only.
- `malloc`/`free` are exported to the host so it can allocate guest buffers.

## Build

Requires: `zig` (tested with 0.18.0-dev)

```bash
cd examples/plugins/wasm/komix_to_cpp
just build      # produces main.wasm (~8 KB)
just check      # verify exports/imports
just clean      # remove build artifacts
```

The justfile:

```
zig cc -x c++ -std=c++20 -target wasm32-freestanding -O2 \
    main.cpp -o main.wasm -nostdlib -Wl,--no-entry
```

## ABI

Exports (C-linkage, `__attribute__((export_name(...)))`):

| Symbol               | Type | Purpose |
|----------------------|------|---------|
| `contract_version`   | i32  | ABI version (returns 1) |
| `Init`               | i64  | Called at load; returns plugin metadata JSON |
| `Search`             | i64  | JSON in → JSON array of manga |
| `GetMangaDetail`     | i64  | manga id → manga detail JSON |
| `GetChapterList`     | i64  | manga id → chapter list JSON |
| `GetPageList`        | i64  | chapter id → page list JSON |
| `GetGenres`          | i64  | → genre list JSON |
| `malloc` / `free`    | i32  | guest memory allocation for host buffers |

Import:

| Module    | Function     | Purpose |
|-----------|--------------|---------|
| `env`     | `host_call`  | generic host helper (HTTP, JSON, regex, etc.) |

Function call convention: `(in_ptr u32, in_len u32) → i64`
where `i64 = (result_len << 32) | result_ptr`.

## Using Nlohmann/JSON

**Not possible** in freestanding mode. `nlohmann::json` requires the C++
standard library (`<string>`, `<vector>`, `<map>`, `<memory>`), which is
unavailable without a stdlib. To use it you would need a wasm target with
full stdlib support (e.g. `emcc` / emscripten, or a zig build that links
`libcxx`). See `../komix_to/` (Go TinyGo) for the stdlib-having path.
