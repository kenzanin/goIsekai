## 1. Dependencies

- [x] 1.1 Add `github.com/PuerkitoBio/goquery` and `github.com/antchfx/htmlquery` as direct dependencies with `go get`, then `go mod tidy`; verify `go build ./internal/... ./cmd/... ./pkg/...` succeeds and `git diff go.mod` shows no change to the existing `golang.org/x/net` version
- [x] 1.2 Run `make check`; verify it reports no lint or format issues, so the two new dependencies and their transitives introduce no production-Go problems

## 2. Shared document implementation

- [x] 2.1 Add `internal/htmldoc` with an exported `Document` type and `Parse(markup string) (*Document, error)`, backed by goquery for the tree and htmlquery for XPath; verify the package builds and a direct unit test parses a fixed markup string into a document
- [x] 2.2 Implement `FindText`, `FindAttr`, `FindListText` and `FindListAttr` on `Document`; verify unit tests assert trimmed first-match text, first-match attribute value, all-matches text in document order, and all-matches attribute values in document order
- [x] 2.3 Implement `XPathText`, `XPathAttr`, `XPathListText` and `XPathListAttr` on `Document`; verify unit tests assert the same four shapes for expressions selecting by position and by text content rather than by class
- [x] 2.4 Add the empty-result behaviour: an unmatched lookup returns the empty string or empty slice, an absent attribute returns the empty string, and a list lookup skips elements that do not carry the attribute; verify with unit tests covering each case, including three matched elements of which only two carry the attribute
- [x] 2.5 Add the failure behaviour: a non-string parse argument, an unparseable CSS selector, and an unparseable XPath expression each return an error naming the offending value, while malformed markup still yields a document; verify with unit tests asserting an error for each and a non-nil document for the malformed-markup case
- [x] 2.6 Confirm the package reads no file and opens no connection, so it cannot widen the plugin surface; verify by inspection of the package's imports and by a test that passes a filesystem path as markup and receives a document parsed from that literal text

## 3. Lua runtime

- [x] 3.1 Add `internal/pluginmanager/lua_html.go` registering the document handle as a typed Lunar userdata (`state.NewUserDataType`) and wrapping the nine functions as natives that read the handle from argument 0, with a failed lookup returning `nil` plus a message the way the other Lua natives do; verify the package builds
- [x] 3.2 Register the `html` group in `registerHostNatives` (`internal/pluginmanager/lua_natives.go`), beside `text`, `codecs`, `crypto`, `json` and `http`; verify a Lua fixture reaches `host.html.parse` and reads a value with `host.html.find_text(doc, selector)`
- [x] 3.3 Verify the handle is opaque and type-checked: a plugin mutating the handle changes nothing, and passing a string, a table or a handle from elsewhere where a handle is expected fails with an error naming the argument rather than being read as a document — `TestLuaHTMLWrongArgumentType` (a string and a table are both rejected by name)
- [x] 3.4 Verify the already-shipped helpers still behave as before by calling `host.text.strip_html` and `host.json.decode` in the same fixture, and verify a bare `html` global is still absent so the group is the only entry point — `TestLuaHTMLGroupLeavesExistingHelpersAlone`, `TestLuaHTMLGroupNotBareGlobal`

## 4. JS runtime

- [x] 4.1 Add `internal/pluginmanager/js_html.go` wrapping the nine functions as goja natives that read the handle from the first call argument and throw on failure in the runtime's established idiom, carrying the Go document in a JS-side object the plugin cannot traverse rather than passing the Go pointer itself; verify the package builds
- [x] 4.2 Register the `html` group in `registerJSHostNatives` (`internal/pluginmanager/js_natives.go`); verify a JS fixture reaches `host.html.parse` and reads a value out of the same fixed markup the Lua fixture uses
- [x] 4.3 Verify the Go method set is not reachable from JS: a JS fixture inspecting the handle finds no `FindText`-style property, so `host.html.find_text(doc, selector)` is the only way to read it
- [x] 4.4 Verify a value that is not a handle is rejected in JS too, by passing a plain object where a handle is expected and observing an error rather than a silent empty result — `TestJSHTMLWrongArgumentType` (a string and a plain object are both rejected)

## 5. Yaegi runtime

- [x] 5.1 Export `htmldoc.Parse` and the `Document` type on the synthetic `hostnet` package in `internal/pluginmanager/yaegi.go` as ordinary functions taking the handle as a Go-typed first argument, keeping the exported symbol names equal to the names a plugin writes; verify a Yaegi plugin source can declare and use the handle without a load-time error
- [x] 5.2 Extend `examples/plugins/yaegi/yaegidemo/main.go` to parse markup and run a lookup; verify the plugin still loads under the sandbox's import check and the lookup returns the expected value — `TestYaegiHTMLDemoExampleLoads` loads the shipped example (compiling its body, so an unexported symbol would be a load error) and reads a title plus two image URLs back out of it
- [x] 5.3 If Yaegi rejects the exported type, fall back to the Yaegi-only form recorded in design.md, where the bridge functions take the markup string per call, and verify the same lookup value and error text are still produced — not triggered: Yaegi v0.16.1 accepted the exported `*htmldoc.Document`, so the fallback stayed unused

## 6. Cross-runtime equivalence

- [x] 6.1 Add assertions to `internal/pluginmanager/host_helpers_test.go` that run the same markup and selector through the Lua and JS runtimes and require identical values for a text lookup, an attribute lookup, and both list lookups; verify the test fails when one runtime's result is altered — `TestHostHTMLSameValuesEverywhere`
- [x] 6.2 Add an assertion that the two script runtimes report identical error text for the same invalid selector and the same invalid XPath expression; verify the test fails when one runtime's message is altered — `TestHostHTMLErrorTextIdentical` (full-string equality, 8 cases)
- [x] 6.3 Add an assertion that a lookup written for Lua and the same lookup written for JS are the same function name and the same argument order, with the handle first and no receiver; verify the test fails if a runtime registers the lookup under a different name or order — `TestHostHTMLSameCallTextInLuaAndJS`
- [x] 6.4 Add an assertion that the Yaegi bridge returns the same lookup value as the script runtimes for the same markup and selector; verify the test fails when the Yaegi result is altered — `TestHostHTMLYaegiMatchesScriptRuntimes`, plus `TestHostHTMLYaegiFixtureSharesMarkup` to keep the fixture's copy of the markup honest

## 7. Documentation

- [x] 7.1 Add `host.html.*` and the Yaegi `hostnet` HTML functions to the per-runtime inventory in `docs/host-native-helpers-plan.md`; verify the list names all nine functions for each runtime that has them, with their argument order
- [x] 7.2 Add a plugin-facing example that fetches a page and reads a list of image URLs through the handle; verify the example runs in a plugin or is exercised by a test, so it is not a snippet nobody has executed

## 8. Verification

- [x] 8.1 Run `CGO_ENABLED=0 go test ./internal/... ./pkg/... ./cmd/...` and `make build`; verify both pass
- [x] 8.2 Run a Lua plugin against a real manga page in a running instance and read a title plus a list of image URLs; verify the scraper gets real values rather than empty results, and that an intentionally wrong selector reports the parse error instead — `GOISEKAI_LIVE=1 go test ./internal/pluginmanager/ -run TestLuaHTMLScrapeLivePage` reads a real page through the host proxy: title `Weeb Central`, 183 image URLs, 138 links, and the wrong expression reports `invalid XPath expression`
- [x] 8.3 Confirm the sandbox is not widened: verify `io.open` and `os.execute` remain unavailable in Lua, and that no new import is allowed in a Yaegi plugin — `TestLuaPluginNoUnsafeGlobals` (io is nil, `os.execute` is nil) and `TestYaegiSandbox` (a `goisekai/...` import is still rejected)
