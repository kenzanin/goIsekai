## Context

The frontend currently uses CloudyKit/jet v6 for server-side HTML rendering (13 template files, 1,022 lines) with a single render choke point at `internal/templates/engine.go`. Custom vanilla JS (`app.js`, 219 lines) handles toasts, confirm modals, and data-confirm delegation. HTMX is imported but unused — zero `hx-*` attributes exist in templates; the codebase evolved past the original HTMX fragment architecture. The `reader.js` (671 lines) is a standalone canvas SPA. Plugin contributors already write Lua but cannot modify frontend templates without learning jet's unfamiliar DSL. The moon runtime (lunar v0.1.1) is already a project dependency for Lua plugins.

## Goals / Non-Goals

**Goals:**
- Replace jet with Lua templates so plugin contributors can modify UI using familiar Lua
- Replace custom vanilla JS with Alpine.js for declarative interactivity
- Remove dead HTMX import and error handlers
- Add SPA client-side navigation (fetch + DOM morph) for instant page transitions
- Maintain identical server-rendered HTML output and API contracts

**Non-Goals:**
- Rewrite `reader.js` — canvas manga reader stays as-is (too complex, no Alpine value)
- Change any API endpoints or data contracts
- Add a build step (no npm, no bundler — Alpine.js via CDN)
- Change the plugin system, hostnet, or database schema
- Add SSR hydration or React/Vue-style reactivity

## Decisions

### D1: Lua template engine via lunar runtime

**Decision:** Use the existing lunar (mmcdole/lunar v0.1.1) library — same runtime as plugins — for template execution, with a lighter configuration (no http_request, no json globals, no 15s timeout).

**Alternatives considered:**
- `gopher-lua` directly: Would add a second Lua dependency. Lunar wraps gopher-lua already; using it keeps one Lua path.
- `html/template` (Go stdlib): Replaces one DSL with another; contributors still need Go knowledge. Rejected per user preference.
- `templ` (type-safe Go templates): Requires build step, Go knowledge, and a `go generate` workflow. Rejected per user preference.
- Scriggo: Already rejected as a template engine (sandbox lacks strings/regexp/html packages).

**Rationale:** Lunar is already in go.mod. Template VMs are lighter than plugin VMs (no host functions, no timeout). Lua string concatenation for HTML is verbose but dead simple — every plugin author already knows Lua.

### D2: Template composition via Lua require()

**Decision:** Layouts and partials are Lua modules loaded via `require("layouts.base")`. Each returns a function that accepts data + content and returns HTML.

**Pattern:**
```lua
-- layouts/base.lua returns function(data, content) → html_string
-- views/library.lua calls base(data, content_html) to wrap in layout
-- partials/nav.lua returns function(data) → nav_html
```

**Rationale:** Lunar already has custom `require()` for sibling `.lua` modules (built for plugins). Reuses the exact same mechanism. No need for jet's `extends`/`block`/`include` — Lua function composition is more explicit.

### D3: Per-request VM creation (no pooling)

**Decision:** Create a fresh Lua VM per render call, compile templates to bytecode at startup and cache bytecodes.

**Alternatives considered:**
- VM pool with mutex: Adds complexity; lunar VMs are not goroutine-safe and resetting state between renders is error-prone.
- Single shared VM with serialization: Blocks all renders during one render; unacceptable at 24 req/s.

**Rationale:** Lunar VM creation is ~0.5ms (measured on plugin VMs with host functions; template VMs without host functions are faster). At 8 pages × ~2 req/s, this is negligible. Fresh VM = zero shared state risk.

### D4: Alpine.js via CDN (no build step)

**Decision:** Load Alpine.js 3.x from CDN in base layout. No npm, no bundler, no `Alpine.start()` customization beyond `Alpine.data()` registration.

**Alternatives considered:**
- Local embedded copy: Same file, just served from `/static/lib/alpine.min.js`. Marginal benefit — CDN has cache advantages for dev, and the binary embeds it anyway via `go:embed`.
- Alpine + HTMX together: HTMX is dead weight (zero usage). Remove entirely.

**Rationale:** CDN is simplest. The project already uses Tailwind via static CSS (no build step). Alpine follows the same zero-build philosophy.

### D5: SPA router via Alpine.js component

**Decision:** A thin `router()` Alpine.js component in base layout intercepts internal link clicks, fetches the target URL with `X-Partial: true`, extracts `<main>` content, and morphs the DOM.

**Server-side support:** The `renderPage` function checks for `X-Partial` header — if present, renders only the `<main>` block (no layout wrapper). This avoids fetching the full page including Alpine.js CDN on every navigation.

**Alternatives considered:**
- Full client-side routing with Alpine stores: Over-engineered for 7 pages. Fetch + morph is simpler.
- Turbo Drive / View Transitions API: View Transitions is newer and less supported. Turbo adds another dependency.

**Rationale:** 7 pages with simple data. Fetch + morph + `pushState` covers the case. The `X-Partial` header keeps payload small (~5-30KB vs ~50KB full page).

### D6: Toast and confirm as Alpine stores (replaces app.js)

**Decision:** `Alpine.store('toast')` and `Alpine.store('confirm')` replace the entire `app.js` IIFE. The `data-confirm` attribute delegation is replaced by Alpine's `x-on:click` with `await $store.confirm.show()`.

**Rationale:** Alpine stores are globally accessible from any `x-data` component. The Promise-based confirm pattern maps directly. Toast auto-dismiss uses Alpine's `x-init` + `setTimeout`. No custom JS needed.

### D7: Action endpoints return JSON (not HTML fragments)

**Decision:** POST action endpoints (`/action/toggle-*`, `/action/clear-*`, etc.) return `{"status": "ok", ...}` JSON instead of HTML fragments. Alpine.js components handle the response: show toast, morph affected DOM section via a targeted re-fetch.

**Alternatives considered:**
- Keep HTML fragment responses: Would require maintaining jet→Lua rendering for action responses too. More work for no benefit since Alpine can re-fetch the affected section.
- Return just status code: Loses data the client needs (e.g., new `active` state, `in_library` state).

**Rationale:** JSON is simpler to produce and consume. The affected DOM section can be re-fetched via SPA router for the updated content. This decouples action handlers from template rendering.

## Risks / Trade-offs

**[XSS in Lua templates]** → Lua string concatenation does not auto-escape HTML. `h()` global is mandatory for all data interpolation. Mitigated by making `h()` the first function documented in every template file and in the developer guide. Could add a linter check for raw string concat in templates.

**[Performance regression]** → Lua VM creation per request adds ~0.5ms vs jet's precompiled templates. At typical load this is invisible. If profiling shows it matters, bytecodes can be cached and VM creation optimized.

**[Template verbosity]** → Lua string concat is more verbose than jet's `{{.Field}}` syntax. A `library.jet` line like `{{.Title}}` becomes `h(data.Title)`. Offset: Lua is more explicit, contributors already know it, and the verbosity catches XSS by default.

**[SPA navigation stale state]** → Client-side navigation means server-side state (cookies, session) stays fresh, but Alpine.js component state on the old page is discarded. This is correct behavior (same as full page reload). Components re-initialize on morph.

**[Dev mode disk reads]** → Reading from disk on every render in dev mode adds I/O overhead. Only active in dev mode; production uses precompiled bytecodes from embedded FS.

**[`X-Partial` header coupling]** → SPA router and server must agree on the partial rendering convention. If a client sends `X-Partial: true` without the router (e.g., curl), it gets a layout-less HTML fragment. This is acceptable — the header is a hint, not a security boundary.

## Migration Plan

1. **Phase 1 (Engine + scaffold):** Build LuaEngine, convert base/blank layouts + partials to Lua, add Alpine.js CDN to base, remove HTMX import, build SPA router skeleton
2. **Phase 2 (Alpine components):** Write toast store, confirm store, data-confirm replacement, per-page x-data components
3. **Phase 3 (View migration):** Convert 8 views from jet to Lua, smallest first → largest last. Each view is independently testable.
4. **Phase 4 (Cleanup):** Remove jet dependency, remove app.js, remove inline scripts, make check clean

**Rollback:** Keep jet engine alongside during migration (feature flag or build tag). If Lua templates have issues, flip back to jet by changing the engine constructor.

## Open Questions

None — all design decisions are resolved. The approach is straightforward migration with clear phase boundaries.
