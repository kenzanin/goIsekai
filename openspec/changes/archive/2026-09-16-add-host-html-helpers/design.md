# Design: Host HTML Helpers

## Context

See proposal.md — Why.

Current state that constrains the approach:

- The shared `host` surface is built in two places: `registerHostNatives` (`internal/pluginmanager/lua_natives.go`) and `registerJSHostNatives` (`internal/pluginmanager/js_natives.go`). Both build named groups (`text`, `codecs`, `crypto`, `json`, `http`) as plain tables/objects of natives, so a new group is additive rather than a structural change.
- The per-runtime wrappers in `lua_helpers.go` and `js_helpers.go` are all string-in/string-out (`luaStr1Err`, `jsStr2Err`, ...). They cannot take or return a document handle, so the HTML natives need their own constructors — exactly the situation `host.json` hit when it needed a value-shaped signature.
- Lunar v0.1.1 already ships the machinery for an opaque Go-backed handle: `state.NewUserDataType[T]` (`userdata_type.go`) mints a State-local, metatable-bound userdata carrying a Go payload, and `descriptor.FromArgument(frame, i)` reads it back as `(T, bool)`. Registration lives outside the Lua registry, so `debug.getregistry` cannot substitute the metatable, and the typed read is what turns a wrong argument into a clean error instead of a bad cast. No host-side id registry or lifetime bookkeeping is needed for it.
- Shared logic that both runtimes call lives in a package the runtimes import (`internal/pluginutil` for text/codecs/crypto, `goccy/go-json` for `host.json`). Sharing one implementation is what makes the existing "identical semantics across runtimes" requirement testable.
- The Yaegi runtime has no `host` table at all. It gets a synthetic `hostnet` package (`yaegiHostPkg` in `internal/pluginmanager/yaegi.go`), installed through `interp.Exports{"hostnet/hostnet": ...}`, and the import check at load time rejects anything that is not the Go stdlib or `hostnet`. That bridge is deliberately limited to stdlib-compatible types because Yaegi cannot parse `bodganfinn/fhttp` source.
- Neither library is present. `golang.org/x/net v0.58.0` is, which is what `htmlquery` needs for `x/net/html`.
- `openspec/specs/plugin-runtime/spec.md` carries the live "Safe Lua stdlib subset" requirement that names the sanctioned surface, and `docs/host-native-helpers-plan.md` carries the per-runtime inventory that plugin authors read.

## Goals / Non-Goals

**Goals:**

- One implementation of the parse-and-lookup semantics, with three thin adapters rather than three copies.
- One flat namespace and one calling convention: nine functions under `host.html`, the handle passed as an ordinary first argument, so a plugin author who has read `host.json` already knows how to call `host.html`.
- A handle-based API: a page is parsed once and queried many times.
- Identical values and identical error text across Lua, JS and Yaegi.
- Purely additive: no existing helper, plugin or ABI function changes behaviour.

**Non-Goals:**

- Wrapping the parsers as transitive string helpers (`host.html.find_text(html, selector)`, passing the raw markup to every lookup). The handle supersedes it: same flat namespace, but the page is parsed once instead of once per lookup.
- Handing the handle methods (`doc.find_text(selector)`). See Decisions.
- Exposing the parsers to the Go `.so` runtime or the WASM/Extism runtime. `.so` plugins link the host ABI and cannot import `goisekai/internal/...`; Extism plugins speak JSON over the ABI. Neither has a helper surface to extend, and neither is in this change's scope.
- Exposing selection or node handles (`host.html.find(doc, sel)` returning something chainable). Only the nine functions are exposed, so the plugin-visible surface is small enough to keep stable.
- Adding a `host` table to Yaegi. The bridge exists precisely so interpreted plugins reach host code without one.
- Concurrency work: invocations are already serialized per plugin instance by the existing per-plugin mutex, and a handle is confined to one instance.

## Decisions

**A document handle, chosen over passing the markup to every lookup.** A manga page is routinely 100 KB–1 MB and a scraper runs a handful of lookups against one page (`find_list_attr` for the images, `find_text` for the title, `find_attr` for the next-chapter link). Passing the raw markup to every lookup re-parses the page on each call — several DOM trees built to answer one page's worth of questions. The handle parses once. The rejected alternative has real merit (a pure function like every other host helper, with no value to carry around), and the handle keeps that property anyway as long as it stays a plain argument.

**Nine functions in the `html` group, with the handle as an ordinary argument — not methods on the handle.** This was the deciding call, and the method form lost. A method form (`doc.find_text(sel)`) would make `host.html` the only host group whose contents are one function plus an object, so a plugin author who has read `host.text` or `host.json` would guess wrong about `host.html`. It would also split the documented surface across two places, and — worst given this repo's audience — it would put Lua's `:`/`.` ambiguity on every call: a plugin author writing `doc:find_text(sel)` out of habit passes the handle as the selector and gets a baffling error. With the handle passed as an argument, Lua and JS calls read identically (`host.html.find_text(doc, "h1")`), the group is one table of nine functions, and there is exactly one convention in the whole plugin-facing surface. The cost is the same either way: neither form preserves the one-argument arity a plugin author might expect, because the document has to be named somewhere.

**The handle is an opaque typed userdata in Lua, not a table of bound closures.** `state.NewUserDataType[*htmldoc.Document]` gives a State-local, metatable-bound userdata, and `FromArgument` reads it back as `(*htmldoc.Document, bool)`, so an argument that is a string, a table, or a handle from some other source fails as a clean error instead of a bad cast. A table of bound closures would work with no new API at all, but the plugin could then mutate its own handle (`doc.find_text = nil`) and the host could not tell one table from another. The userdata also keeps the method set out of the plugin's sight: it carries a metatable the host owns, not the Go type's exported methods. A registry of integer ids was rejected as strictly worse — it would need allocation, reuse and cleanup bookkeeping that the typed userdata gets from the runtime.

**The handle lives in its own package, behind an exported document type.** A new `internal/htmldoc` package holds `Document` plus `Parse`, and the eight lookups are methods on `Document` so the shared code has a natural home. The three runtimes are then thin adapters over one implementation: Lua and JS register functions that read the handle argument and call a method, and Yaegi exports the same methods as functions. The alternative — implementing the lookups inside `lua_html.go` and `js_html.go` — would put the trim rules, the skip rule and the error text in two or three places, and the "equivalent surface across runtimes" requirement would then be maintained by hand rather than by construction.

**Yaegi reaches the same code as plain `hostnet` functions.** There is no `host` table in Yaegi, so the Lua/JS address does not exist there; the synthetic bridge is the sanctioned surface. The bridge therefore exports `Parse` plus the eight lookups as ordinary functions taking the handle as a *Go-typed* first argument — `hostnet.FindText(doc, "h1")` sits alongside the existing `hostnet.Get(url)` rather than introducing a method style the bridge does not otherwise use. The names differ from the Lua/JS ones because the surfaces differ; the argument order, semantics and error text do not. Exporting a struct type through `interp.Exports` is the one piece of this design not already proven in the repo, so it carries an explicit fallback in Risks.

**A lookup that matches nothing succeeds; a selector that cannot be parsed fails.** Returning `""` for both would make an empty result ambiguous between "the page has no such element" and "you typed the selector wrong", which are the two things a scraper author most needs to tell apart when a site changes its layout. Malformed markup is a third case and is never an error: `x/net/html` always recovers some tree, so there is nothing to report.

**List lookups skip elements that lack the named attribute.** `find_list_attr` is the call a scraper uses to pull every page image, then indexes the result to pair it with a page number or a chapter link. Emitting an empty-string placeholder for an element without the attribute would silently shift every later pair. Skipping keeps the list index-aligned with the values that exist.

**The handle has no explicit release.** The tree dies with the plugin's last reference to the handle and is reclaimed by the Go garbage collector, which is global and runs under real memory pressure. A `host.html.close(doc)`-style function was rejected for three reasons. It would need a use-after-free answer on all eight lookups, turning each into a two-state call (`""` on a closed handle silently hides the bug; an error adds a state machine to eight functions to save one tree). It is not a defence against the failure it appears to defend against: the plugin that retains every handle is exactly the plugin that would not call `close`. And the case it does help — a plugin that walks many pages while keeping only the current handle reachable — is already handled by the collector, because each iteration makes the previous tree unreachable. → Upgrade path: if a real plugin ever needs several trees live at once and its peak memory becomes a problem, `host.html.close(doc)` (or a scoped `host.html.with(html, fn)`) is the addition. It can land without changing any of the eight lookups, because a closed handle is only ever a failure case that does not exist today, and the typed userdata would just report it as one more rejected argument.

**Both libraries ship, not one.** goquery covers CSS selection, which is the common case; htmlquery covers XPath, which is the escape hatch for markup with no stable class or id, for reaching a value from a sibling or ancestor, and for selecting by text content. htmlquery already builds on goquery's DOM, so the second library adds an XPath expression engine and a second entry point, not a second tree.

## Risks / Trade-offs

- **Two new direct dependencies plus transitives.** goquery is the de-facto CSS-selector layer for `x/net/html` and is the same DOM htmlquery builds on, so the pair is the standard choice rather than an exotic one. → Mitigation: run `go mod tidy`, keep `make check` clean, and confirm `golang.org/x/net` does not need to move.
- **The handle keeps a DOM tree on the Go heap, where no VM limit sees it.** `MaxHeapBytes: 64 << 20` in `lua_state.go` caps the *Lua* heap only, and the goja VM is created with no limit at all (`goja.New()`). The tree behind a handle is a Go object reachable only from the userdata, so a value holding the handle costs a few dozen bytes while the tree it points at can be megabytes: a plugin can look comfortably inside its heap cap while the host's Go heap grows. → Mitigation: this is deferred reclamation, not a leak. The tree becomes unreachable as soon as the plugin drops the handle and the global Go collector reclaims it under real memory pressure, so a plugin that walks pages while keeping only the current handle reachable holds one tree at a time. The remaining case is a plugin that deliberately retains every handle it has ever parsed, which is that plugin author's own memory bill and is not curable host-side. Stated ceiling: peak host memory for one invocation is proportional to the number of trees the plugin keeps reachable, and the per-invocation timeout is the only host-side bound.
- **The handle keeps a DOM tree alive, where per-call parsing would not.** A plugin that parses inside a loop holds one tree per iteration until the collector runs. → Mitigation: each iteration makes the previous tree unreachable, so the collector reclaims it; a plugin that needs several trees live at once can be given the `host.html.close(doc)` upgrade path named under Decisions if peak memory ever becomes real.
- **goja may hand the JS plugin the Go method set.** `vm.ToValue` on a Go pointer exposes its exported methods as properties, so passing the handle through naively would give JS plugins `doc.FindText(sel)` alongside `host.html.find_text(doc, sel)` — exactly the second calling convention this design rejected, arriving by accident. → Mitigation: the JS handle is a JS-side object, not the Go pointer itself; the pointer is carried in a field the plugin cannot traverse, and the eight functions are the only way to read it. The JS fixture asserts the method set is not reachable.
- **XPath is a second query language and a pathological expression can be expensive.** → Mitigation: it runs inside the existing per-invocation timeout and per-instance memory cap, so a bad expression is a slow or failed call, not an escape or a host crash.
- **The markup, selectors and expressions are all plugin-supplied, so an invalid one must never panic the host.** → Mitigation: every unparseable selector or expression is converted to an error return, the cases are asserted in `internal/pluginmanager/host_helpers_test.go`, and the natives already run behind the invocation timeout and panic isolation that every other host helper uses.
- **The cross-runtime equivalence requirement rots if a lookup is ever implemented per runtime.** → Mitigation: one shared implementation, plus a test that runs each lookup in both script runtimes against the same markup and asserts identical values and identical error text — the same shape as the existing `host.json` identity test.
- **Exporting a handle type through the Yaegi bridge is unproven here.** Yaegi maps an export symbol name to a value or type, and the symbol name has to be the name the plugin writes; yaegi's own stdlib table does exactly this (`"File": reflect.ValueOf((*fs.File)(nil))`), and this design keeps the Go type name equal to the exported symbol name to match. → Mitigation: `examples/plugins/yaegi/yaegidemo/main.go` is extended to parse and query, so the export is exercised by an existing fixture. If Yaegi still refuses the type, the fallback is Yaegi-only and keeps the same lookups: without an exported handle type there is nothing to pass, so the `hostnet` functions take the markup string per call (`hostnet.FindText(html, selector)`). That costs a re-parse per lookup for Yaegi alone and still satisfies the result-and-error-text half of the equivalence requirement.

## Migration Plan

1. Add the two dependencies and tidy the module; confirm the existing `golang.org/x/net` version is unchanged.
2. Add `internal/htmldoc` with `Parse` and the eight lookups; test it directly against fixed markup, which is the only place the parsing semantics need asserting.
3. Add `internal/pluginmanager/lua_html.go`: register the userdata type, and wrap the lookups as natives that read the handle from argument 0 and register an `html` group in `lua_natives.go`.
4. Add `internal/pluginmanager/js_html.go`, registering the `html` group in `js_natives.go` with the handle carried in a JS-side object rather than the raw Go pointer.
5. Export the parse function and the document type on the Yaegi bridge in `yaegi.go`, and extend the Yaegi demo plugin so the export is exercised.
6. Add the cross-runtime assertions to `internal/pluginmanager/host_helpers_test.go`.
7. Update the per-runtime inventory in `docs/host-native-helpers-plan.md`.
8. Gate on `make check`, `make build`, and the full `CGO_ENABLED=0 go test ./internal/... ./pkg/... ./cmd/...`, then read a real page through a Lua plugin in a running instance to confirm a scraper can actually use it.

Rollback: remove the `html` group registrations and the Yaegi bridge exports. Nothing else depends on them and no plugin was changed, so the surface is additive in both directions and the dependency can simply be dropped from the module if the change is abandoned.

## Open Questions

- Whether the Go `.so` and WASM/Extism runtimes should eventually get the same lookups for cross-runtime parity. Deferrable: it needs no spec change and no task in this change.
