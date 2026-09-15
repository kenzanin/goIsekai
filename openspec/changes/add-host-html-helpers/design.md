# Design: Host HTML Helpers

## Context

See proposal.md — Why.

Current state that constrains the approach:

- The shared `host` surface is built in two places: `registerHostNatives` (`internal/pluginmanager/lua_natives.go`) and `registerJSHostNatives` (`internal/pluginmanager/js_natives.go`). Both build named groups (`text`, `codecs`, `crypto`, `json`, `http`) as plain tables/objects of natives, so a new group is additive rather than a structural change.
- The per-runtime wrappers in `lua_helpers.go` and `js_helpers.go` are all string-in/string-out (`luaStr1Err`, `jsStr2Err`, ...). They cannot carry a document receiver, so the HTML natives need their own constructors — exactly the situation `host.json` hit when it needed a value-shaped signature.
- Shared logic that both runtimes call lives in a package the runtimes import (`internal/pluginutil` for text/codecs/crypto, `goccy/go-json` for `host.json`). Sharing one implementation is what makes the existing "identical semantics across runtimes" requirement testable.
- The Yaegi runtime has no `host` table at all. It gets a synthetic `hostnet` package (`yaegiHostPkg` in `internal/pluginmanager/yaegi.go`), installed through `interp.Exports{"hostnet/hostnet": ...}`, and the import check at load time rejects anything that is not the Go stdlib or `hostnet`. That bridge is deliberately limited to stdlib-compatible types because Yaegi cannot parse `bodganfinn/fhttp` source.
- Neither library is present. `golang.org/x/net v0.58.0` is, which is what `htmlquery` needs for `x/net/html`.
- `openspec/specs/plugin-runtime/spec.md` carries the live "Safe Lua stdlib subset" requirement that names the sanctioned surface, and `docs/host-native-helpers-plan.md` carries the per-runtime inventory that plugin authors read.

## Goals / Non-Goals

**Goals:**

- One implementation of the parse-and-lookup semantics, with three thin adapters rather than three copies.
- A handle-based API: a page is parsed once and queried many times.
- Identical values and identical error text across Lua, JS and Yaegi.
- Purely additive: no existing helper, plugin or ABI function changes behaviour.

**Non-Goals:**

- Wrapping the parsers as transitive string helpers (`host.html.find_text(html, selector)`). The handle supersedes it; shipping both would be two surfaces to keep in sync.
- Exposing the parsers to the Go `.so` runtime or the WASM/Extism runtime. `.so` plugins link the host ABI and cannot import `goisekai/internal/...`; Extism plugins speak JSON over the ABI. Neither has a helper surface to extend, and neither is in this change's scope.
- Handing plugins a raw selection or node handle (`doc.find(sel).attr(...)` chains). Only the eight lookups are exposed, so the plugin-visible surface is small enough to keep stable.
- Adding a `host` table to Yaegi. The bridge exists precisely so interpreted plugins reach host code without one.
- Concurrency work: invocations are already serialized per plugin instance by the existing per-plugin mutex, and the handle is confined to one instance.

## Decisions

**A document handle, chosen over a per-call document argument.** A manga page is routinely 100 KB–1 MB and a scraper runs a handful of lookups against one page (`find_list_attr` for the images, `find_text` for the title, `find_attr` for the next-chapter link). Passing the markup to every lookup re-parses the page on each call — several DOM trees built to answer one page's worth of questions. The handle parses once. The alternative is not without merit: it would be a pure function like every other host helper, with no lifetime to reason about. It was rejected because the cost lands in exactly the loop a scraper is.

**The handler lives in its own package, behind an exported document type.** A new `internal/htmldoc` package holds `Document` plus `Parse`, and the eight lookups are methods on `Document`. Three runtimes then need three adapters and zero duplicated logic: Lua and JS bind the methods as closures already carrying the document, and Yaegi exports the type itself. The alternative — implementing the lookups inside `lua_html.go` and `js_html.go` — would put the trim rules, the skip rule and the error text in two or three places, and the "equivalent surface across runtimes" requirement would then be maintained by hand rather than by construction.

**Lua and JS use a dot-style method call, not Lua's colon.** Each lookup is registered as a closure bound to the document, so `doc.find_text(sel)` works with the same text in Lua and JS. Lua's `doc:find_text(sel)` would pass the receiver as the first argument, and the natives would have to accept and discard a leading table — a second convention the tests would have to cover. One convention across the two script runtimes is what makes the cross-runtime identity assertion meaningful.

**Yaegi reaches the same code as `hostnet.Parse` / `hostnet.Document`.** There is no `host` table in Yaegi, so the Lua/JS address does not exist there; the synthetic bridge is the sanctioned surface. The names differ because the surfaces differ; the semantics and error text do not. Exporting a struct type through `interp.Exports` is the one piece of this design not already proven in the repo, so it carries an explicit fallback in Risks.

**A lookup that matches nothing succeeds; a selector that cannot be parsed fails.** Returning `""` for both would make an empty result ambiguous between "the page has no such element" and "you typed the selector wrong", which are the two things a scraper author most needs to tell apart when a site changes its layout. Malformed markup is a third case and is never an error: `x/net/html` always recovers some tree, so there is nothing to report.

**List lookups skip elements that lack the named attribute.** `find_list_attr` is the call a scraper uses to pull every page image, then indexes the result to pair it with a page number or a chapter link. Emitting an empty-string placeholder for an element without the attribute would silently shift every later pair. Skipping keeps the list index-aligned with the values that exist.

**The handle has no explicit release.** The tree dies with the plugin's last reference to the handle and is reclaimed by the Go garbage collector, which is global and runs under real memory pressure. A `doc.close()`-style API was rejected for three reasons. It would need a use-after-free answer for all eight lookups, turning every one of them into a two-state call (`""` on a closed handle silently hides the bug; an error adds a state machine to eight methods to save one tree). It is not a defence against the failure it appears to defend against: the plugin that retains every handle is exactly the plugin that would not call `close()`. And the case it does help — a plugin that walks many pages while keeping only the current handle reachable — is already handled by the collector, because each iteration makes the previous tree unreachable. → Upgrade path: if a real plugin ever needs several trees live at once and its peak memory becomes a problem, `doc.close()` (or a scoped `host.html.with(html, fn)`) is the addition, and it can land without changing any of the eight lookups, since a closed handle is only ever a failure case that today does not exist.

**Both libraries ship, not one.** goquery covers CSS selection, which is the common case; htmlquery covers XPath, which is the escape hatch for markup with no stable class or id, for reaching a value from a sibling or ancestor, and for selecting by text content. htmlquery already builds on goquery's DOM, so the second library adds an XPath expression engine and a second entry point, not a second tree.

## Risks / Trade-offs

- **Two new direct dependencies plus transitives.** goquery is the de-facto CSS-selector layer for `x/net/html` and is the same DOM htmlquery builds on, so the pair is the standard choice rather than an exotic one. → Mitigation: run `go mod tidy`, keep `make check` clean, and confirm `golang.org/x/net` does not need to move.
- **The handle keeps a DOM tree on the Go heap, where no VM limit sees it.** `MaxHeapBytes: 64 << 20` in `lua_state.go` caps the *Lua* heap only, and the goja VM is created with no limit at all (`goja.New()`). The tree behind a handle is a Go object held by a Go closure, so a Lua table entry holding the handle costs a few dozen bytes while the tree it points at can be megabytes: a plugin can look comfortably inside its heap cap while the host's Go heap grows. → Mitigation: this is deferred reclamation, not a leak. The tree becomes unreachable as soon as the plugin drops the handle and the global Go collector reclaims it under real memory pressure, so a plugin that walks pages while keeping only the current handle reachable holds one tree at a time. The remaining case is a plugin that deliberately retains every handle it has ever parsed, which is that plugin author's own memory bill and is not curable host-side. Stated ceiling: peak host memory for one invocation is proportional to the number of trees the plugin keeps reachable, and the per-invocation timeout is the only host-side bound.
- **The handle keeps a DOM tree alive, where per-call parsing would not.** A plugin that parses inside a loop holds one tree per iteration until the collector runs. → Mitigation: each iteration makes the previous tree unreachable, so the collector reclaims it; a plugin that needs several trees live at once can be given the `close()` upgrade path named under Decisions if peak memory ever becomes real.
- **XPath is a second query language and a pathological expression can be expensive.** → Mitigation: it runs inside the existing per-invocation timeout and per-instance memory cap, so a bad expression is a slow or failed call, not an escape or a host crash.
- **The markup, selectors and expressions are all plugin-supplied, so an invalid one must never panic the host.** → Mitigation: every unparseable selector or expression is converted to an error return, the cases are asserted in `internal/pluginmanager/host_helpers_test.go`, and the natives already run behind the invocation timeout and panic isolation that every other host helper uses.
- **The cross-runtime equivalence requirement rots if a lookup is ever implemented per runtime.** → Mitigation: one shared implementation, plus a test that runs each lookup in both script runtimes against the same markup and asserts identical values and identical error text — the same shape as the existing `host.json` identity test.
- **Exporting a handle type through the Yaegi bridge is unproven here.** Yaegi maps an export symbol name to a value or type, and the symbol name has to be the name the plugin writes; yaegi's own stdlib table does exactly this (`"File": reflect.ValueOf((*fs.File)(nil))`), and this design keeps the Go type name equal to the exported symbol name to match. → Mitigation: `examples/plugins/yaegi/yaegidemo/main.go` is extended to parse and query, so the export is exercised by an existing fixture. If Yaegi still refuses the type, the fallback for Yaegi only is the stateless form — a plain `hostnet` function taking the markup as an argument — which needs no type export and still satisfies the result-and-error-text half of the equivalence requirement.

## Migration Plan

1. Add the two dependencies and tidy the module; confirm the existing `golang.org/x/net` version is unchanged.
2. Add `internal/htmldoc` with `Parse` and the eight methods; test it directly against fixed markup, which is the only place the parsing semantics need asserting.
3. Add `internal/pluginmanager/lua_html.go` and register an `html` group in `lua_natives.go`.
4. Add `internal/pluginmanager/js_html.go` and register the `html` group in `js_natives.go`.
5. Export the parse function and the document type on the Yaegi bridge in `yaegi.go`, and extend the Yaegi demo plugin so the export is exercised.
6. Add the cross-runtime assertions to `internal/pluginmanager/host_helpers_test.go`.
7. Update the per-runtime inventory in `docs/host-native-helpers-plan.md`.
8. Gate on `make check`, `make build`, and the full `CGO_ENABLED=0 go test ./internal/... ./pkg/... ./cmd/...`, then read a real page through a Lua plugin in a running instance to confirm a scraper can actually use it.

Rollback: remove the `html` group registrations and the Yaegi bridge exports. Nothing else depends on them and no plugin was changed, so the surface is additive in both directions and the dependency can simply be dropped from the module if the change is abandoned.

## Open Questions

- Whether the Go `.so` and WASM/Extism runtimes should eventually get the same lookups for cross-runtime parity. Deferrable: it needs no spec change and no task in this change.
