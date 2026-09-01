# Proposal: lua-plugin-runtime

## What & Why

Add Lua as a second plugin runtime alongside WASM, so plugins can be written as plain `.lua` script folders — no toolchain needed. Today, authoring a plugin requires the TinyGo toolchain to compile a `.wasm`; a Lua plugin needs only a text editor, which dramatically lowers the barrier to growing the plugin ecosystem (the main advantage Suwayomi has over goIsekai today).

Lua is also a natural fit for the actual work plugins do — fetching JSON/HTML and parsing it with `string.match`/`gsub` patterns.

The plugin layout follows the user's requested shape: source collections live in `plugins/lua/<site>/` in the repo, each folder is one plugin (folder name = plugin ID) with `main.lua` as the required entry point and any number of sibling `.lua` modules it `require`s. Installing copies the folder recursively into `app_data/plugins/<site>/`.

## Capabilities

- `plugin-runtime` (existing): gains Lua as a second plugin kind — discovery of `*/main.lua` folders alongside `*.wasm`, per-plugin-folder `require` sandboxing, host `http_request` exposed as a Lua global, ABI parity (contract version, search/detail/chapters/pages, Init metadata incl. verify fields + thumb_ratio), install via recursive folder copy.

##Impact / Alignment

- Two plugin tiers: Lua = easy-to-author tier for casual plugin writers; WASM = hardened tier (memory cap, syscall-free sandbox) unchanged.
- No changes to bridge/UI/hostnet contracts — Lua plugins are indistinguishable from WASM plugins everywhere above the Manager.
- New dependency: `github.com/yuin/gopher-lua` (MIT, pure Go, CGO-free — keeps the single-binary build intact).
- Risk accepted: Lua sandbox is weaker than WASM (no memory cap, library-level restriction instead of capability isolation). Mitigations: register only safe stdlib subsets (`string`, `table`, `math`, `os.time/date` only), context timeout per invocation, serialized per-plugin invocation.

## Non-goals

- Hot-reload of edited Lua files without restart.
- Any bridge/UI/schema changes beyond what Discover/Install already do for WASM.
- Converting the existing mangadex WASM plugin to Lua.
