## Context
Each WASM plugin has its own Makefile under `examples/wasm/<name>/`. The root Makefile only builds the host binary. Plugin builds require manual cd + make + cp.

## Goals
1. Single command to build all plugins
2. Single command for full build (plugins + host)
3. Install target to copy to app_data
4. CI-friendly error handling

## Decisions

### D1: Root Makefile targets
Add three targets to the root Makefile:
- `build-plugins`: iterates over `examples/wasm/*/Makefile` directories, runs `make build` in each
- `install-plugins`: copies `examples/wasm/*/dist/*.wasm` to `app_data/plugins/`
- `all`: depends on `build-plugins` then `build`

### D2: Plugin discovery
Use shell glob `for dir in examples/wasm/*/; do ... done` to discover plugins. Each plugin directory must have a Makefile with a `build` target.

### D3: Error handling
Each plugin build runs in a subshell. On failure, the loop exits immediately with the failing plugin's name. Uses `set -e` pattern.

### D4: JS and Lua plugins
JS plugins (goja) and Lua plugins don't need compilation — they're source files. Only WASM plugins need building. The `build-plugins` target handles WASM only.

## Risks
- Plugin Makefile inconsistency — mitigated by requiring a standard `build` target
- Build order matters if plugins share dependencies — not the case currently
