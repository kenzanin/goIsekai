# Add Host HTML Helpers

## Why

Scraping HTML is the single biggest source of duplicated, fragile code across this repo's plugins. Every source hand-rolls its own tag walking with Lua `string.find`/`string.match` or JS regexes, and those break the moment a site reorders attributes or nests a tag. The host already solved the same problem for text, codecs and crypto by exposing one Go-backed helper surface (`host.text`, `host.codecs`, `host.crypto`, `host.json`), and HTML parsing is the largest remaining hole in it.

CSS selectors cover the ordinary case in one line; XPath is the escape hatch for markup that has no stable class or id, where the value lives in a sibling or ancestor, or where the match is defined by text content.

## What Changes

- Add two HTML parsing libraries as direct dependencies: `github.com/PuerkitoBio/goquery` (CSS selectors, plus `cascadia` transitively) and `github.com/antchfx/htmlquery` (XPath, on the already-present `golang.org/x/net/html`).
- Add a `host.html` helper group with 9 functions in the Lua and JS runtimes, and the matching functions on the synthetic `hostnet` package in the Yaegi runtime. `parse` builds an opaque document handle from a markup string; the eight lookups take that handle as their first argument, so the whole group is flat functions in one namespace with no receiver syntax and identical call text in Lua and JS:
  - `parse(html)`.
  - CSS selectors (`goquery`): `find_text(doc, selector)`, `find_attr(doc, selector, attr)`, `find_list_text(doc, selector)`, `find_list_attr(doc, selector, attr)`.
  - XPath (`htmlquery`): `xpath_text(doc, expr)`, `xpath_attr(doc, expr, attr)`, `xpath_list_text(doc, expr)`, `xpath_list_attr(doc, expr, attr)`.
- Document the new group alongside the existing host helper inventory.

No existing helper changes behaviour, no plugin ABI function changes, and nothing is removed.

## Capabilities

### New Capabilities

- `host-html-helpers`: Host-exposed HTML scraping natives — CSS selector lookups via goquery and XPath lookups via htmlquery — available to the Lua and JS plugin runtimes with identical semantics and error reporting, plus the equivalent functions on the Yaegi `hostnet` bridge.

### Modified Capabilities

- `plugin-runtime`: the "Safe Lua stdlib subset" requirement enumerates the sanctioned helper surface by name. The new `host.html` group extends it, so the requirement's description of where HTML parsing is available changes.
- `yaegi-runtime`: the sandboxing and networking requirements state that the synthetic `hostnet` package exposes exactly `Get` and `Post`. New functions are added to that package, so the enumeration changes.

## Impact

- `go.mod` / `go.sum` — two new direct dependencies and their transitive tree (`cascadia`, `golang.org/x/net/html`).
- `internal/pluginmanager/` — a shared HTML helper implementation plus one registration site per runtime: `lua_natives.go` (`host.html` group), `js_natives.go` (`host.html` group), `yaegi.go` (new methods on `yaegiHostPkg` exported as `hostnet.*`).
- `internal/pluginmanager/host_helpers_test.go` — the existing cross-runtime host helper tests gain HTML cases.
- `docs/host-native-helpers-plan.md` — the per-runtime host surface inventory gains `host.html.*`.
- No database, HTTP API, or template change.
- No plugin is forced to change: the helpers are additive, so an existing plugin keeps working untouched.
