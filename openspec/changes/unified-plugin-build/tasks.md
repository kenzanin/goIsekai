## Tasks

### 1. Root Makefile targets
- [x] 1.1 Add `build-plugins` target that iterates `examples/wasm/*/` and runs `make build` in each
- [x] 1.2 Add `install-plugins` target that copies `.wasm` files to `app_data/plugins/`
- [x] 1.3 Add `all` target that depends on `build-plugins` then `build`
- [x] 1.4 Ensure error handling: exit on first failure with plugin name

### 2. Standardize plugin Makefiles
- [x] 2.1 Verify each `examples/wasm/*/Makefile` has a `build` target
- [x] 2.2 Add `build` target to any plugin that's missing one

### 3. Verify
- [x] 3.1 `make build-plugins` builds all WASM plugins successfully
- [x] 3.2 `make install-plugins` copies .wasm files to app_data/plugins/
- [x] 3.3 `make all` builds plugins then host binary
- [x] 3.4 Intentionally break one plugin, verify `make build-plugins` exits non-zero
