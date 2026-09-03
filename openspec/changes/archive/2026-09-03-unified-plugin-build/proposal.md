## Why
`make build` at root does not rebuild WASM plugins. User must manually `cd examples/wasm/X && make build && cp dist/X.wasm ../../app_data/plugins/`. Easy to forget, leads to stale plugin binaries and confusing bugs.

## What Changes
- Root `make build-plugins` target that builds all WASM plugins
- `make all` = build-plugins + build host
- Optional: `make install-plugins` copies to app_data/plugins/
- CI-friendly: exit non-zero on any plugin build failure

## Capabilities
### New Capabilities
- `unified-plugin-build`: Single-command build for all WASM plugins with install to app_data

### Modified Capabilities
(none)

## Impact
- `Makefile` — new targets: build-plugins, install-plugins, all
- `examples/wasm/*/Makefile` — ensure each has a consistent build target
- No Go code changes
