## Tasks

### 1. Root Makefile targets
- [ ] 1.1 Add `build-plugins` target that iterates `examples/wasm/*/` and runs `make build` in each
- [ ] 1.2 Add `install-plugins` target that copies `.wasm` files to `app_data/plugins/`
- [ ] 1.3 Add `all` target that depends on `build-plugins` then `build`
- [ ] 1.4 Ensure error handling: exit on first failure with plugin name

### 2. Standardize plugin Makefiles
- [ ] 2.1 Verify each `examples/wasm/*/Makefile` has a `build` target
- [ ] 2.2 Add `build` target to any plugin that's missing one

### 3. Verify
- [ ] 3.1 `make build-plugins` builds all WASM plugins successfully
- [ ] 3.2 `make install-plugins` copies .wasm files to app_data/plugins/
- [ ] 3.3 `make all` builds plugins then host binary
- [ ] 3.4 Intentionally break one plugin, verify `make build-plugins` exits non-zero
