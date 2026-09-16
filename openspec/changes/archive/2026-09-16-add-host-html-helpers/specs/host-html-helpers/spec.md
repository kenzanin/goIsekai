## Purpose

Give Lua, JS and Yaegi plugins one host-backed way to read a scraped HTML page, so a plugin looks values up by CSS selector or XPath instead of hand-rolling tag walking, and the same lookup returns the same value in every runtime.

## ADDED Requirements

### Requirement: HTML document handle

The host SHALL expose a document handle built from a markup string: `host.html.parse(html)` in the Lua and JS runtimes, and `hostnet.Parse(html)` in the Yaegi runtime. The handle SHALL be an opaque value. A plugin SHALL reach the lookups by passing the handle back to the host's `host.html` group, not by calling methods on the handle, so the whole HTML surface stays a flat set of functions in one namespace, the way the other host groups are.

Parsing SHALL take the markup as a string argument supplied by the plugin. The host SHALL read no file, accept no path or URL, and open no network connection, so HTML parsing SHALL NOT widen the plugin file, IO or process surface.

A value that is not a handle from this host SHALL NOT be accepted where a handle is expected, so a handle cannot be substituted with a string, a table or a handle belonging to something else.

Malformed or truncated markup SHALL NOT be a parse failure: the handle SHALL be built from whatever tree the parser recovers. A non-string argument to the parse function SHALL fail and report the failure to the caller instead of returning a handle.

#### Scenario: Handle built from markup

- **WHEN** a Lua plugin calls `host.html.parse` with the markup string `<h1 class="t">Solo Leveling</h1>`
- **THEN** it receives a handle, and lookups that are given that handle see the document

#### Scenario: Lookups take the handle as an argument

- **WHEN** a plugin holds a handle and reads a value from it
- **THEN** it calls a function of the form `host.html.find_text(doc, selector)`, passing the handle as the first argument

#### Scenario: Non-string argument rejected

- **WHEN** a plugin calls `host.html.parse` with a number instead of a string
- **THEN** the call fails with an error about the argument and no handle is returned

#### Scenario: A value that is not a handle rejected

- **WHEN** a plugin passes a string, a table or some other value where a document handle is expected
- **THEN** the call fails with an error naming the argument, rather than being read as a document or failing obscurely

#### Scenario: Malformed markup still yields a handle

- **WHEN** a plugin calls `host.html.parse` with markup that is unclosed or otherwise invalid HTML
- **THEN** it receives a handle and a lookup against it returns the recovered content rather than failing

#### Scenario: Parsing cannot reach the filesystem

- **WHEN** a plugin calls `host.html.parse` with a filesystem path instead of markup
- **THEN** it receives a handle parsed from that literal text, and no host file is read

### Requirement: CSS selector lookups

The host SHALL expose four CSS-selector lookups in its `html` group, each taking the document handle as its first argument: `host.html.find_text(doc, selector)`, `host.html.find_attr(doc, selector, attr)`, `host.html.find_list_text(doc, selector)` and `host.html.find_list_attr(doc, selector, attr)`. The Yaegi runtime SHALL expose the equivalent `hostnet` functions `FindText`, `FindAttr`, `FindListText` and `FindListAttr`, in the same argument order.

- `find_text` SHALL return the text content of the first element matching the selector, with surrounding whitespace trimmed.
- `find_attr` SHALL return the value of the named attribute on the first element matching the selector.
- `find_list_text` SHALL return the trimmed text content of every element matching the selector, in document order.
- `find_list_attr` SHALL return the value of the named attribute for every element matching the selector, in document order.

#### Scenario: First match text read

- **WHEN** a plugin calls `find_text(doc, "span.status")` against markup containing `<span class="status">Ongoing</span>`
- **THEN** it receives `Ongoing` with surrounding whitespace removed

#### Scenario: First match attribute read

- **WHEN** a plugin calls `find_attr(doc, "a.next", "href")` against markup whose anchor carries `href="/read/1"`
- **THEN** it receives `/read/1`

#### Scenario: First match wins when several elements match

- **WHEN** a selector matches three elements with different text
- **THEN** `find_text` returns only the first one's text

#### Scenario: List of texts read

- **WHEN** a plugin calls `find_list_text` with a selector matching three elements carrying the genres `Action`, `Adventure` and `Drama`
- **THEN** it receives those three values in document order

#### Scenario: List of attributes read

- **WHEN** a plugin calls `find_list_attr(doc, "div.page img", "src")` against markup with four page images
- **THEN** it receives the four `src` values in document order

### Requirement: XPath lookups

The host SHALL expose four XPath lookups in its `html` group, each taking the document handle as its first argument: `host.html.xpath_text(doc, expr)`, `host.html.xpath_attr(doc, expr, attr)`, `host.html.xpath_list_text(doc, expr)` and `host.html.xpath_list_attr(doc, expr, attr)`. The Yaegi runtime SHALL expose the equivalent `hostnet` functions `XPathText`, `XPathAttr`, `XPathListText` and `XPathListAttr`, in the same argument order.

They SHALL return the same shapes as the CSS lookups from the nodes the XPath expression selects: trimmed text content for the text variants, the named attribute value for the attribute variants, the first match for the singular variants, and every match in document order for the list variants.

#### Scenario: XPath text read

- **WHEN** a plugin calls `xpath_text` with an expression selecting a chapter title element that has no class or id
- **THEN** it receives that element's trimmed text

#### Scenario: XPath attribute read

- **WHEN** a plugin calls `xpath_attr` with an expression selecting an image element and the attribute `data-src`
- **THEN** it receives that element's `data-src` value

#### Scenario: XPath reaching by position rather than selector

- **WHEN** a plugin calls `xpath_text` with an expression that selects a cell by its position among siblings
- **THEN** it receives that cell's text, so markup without stable classes is still readable

#### Scenario: XPath list of attributes read

- **WHEN** a plugin calls `xpath_list_attr` with an expression selecting many image nodes and the attribute `src`
- **THEN** it receives every selected node's `src` value in document order

### Requirement: Empty and missing results

A lookup that matches nothing SHALL return an empty result rather than failing: the empty string for the singular text and attribute lookups, `nil` for a Lua list, an empty array for a JS list, and an empty slice for a Yaegi list. An attribute that is absent on a matched element SHALL likewise return the empty result.

In the list lookups, an element that does not carry the named attribute SHALL be skipped rather than contributing an empty entry, so the returned list holds only real values and stays index-aligned with them.

#### Scenario: Selector matches nothing

- **WHEN** a plugin calls `find_text` with a selector that matches no element
- **THEN** it receives an empty string and no error

#### Scenario: Attribute absent on the matched element

- **WHEN** a plugin calls `find_attr` with an attribute the matched element does not carry
- **THEN** it receives an empty string and no error

#### Scenario: List lookup matches nothing

- **WHEN** a plugin calls `find_list_attr` with a selector that matches no element
- **THEN** it receives an empty list rather than a failure

#### Scenario: Elements without the attribute are skipped

- **WHEN** a plugin calls `find_list_attr` for `src` against three matched elements of which only two carry `src`
- **THEN** the returned list holds exactly those two values and no placeholder entry

### Requirement: Invalid selector or expression reporting

A selector or XPath expression the host cannot parse SHALL fail and report the failure to the caller rather than returning an empty or partial result, so a mistyped selector is distinguishable from a selector that simply matched nothing. The failure SHALL name the selector or expression that could not be parsed.

#### Scenario: Malformed CSS selector rejected

- **WHEN** a plugin calls `find_text` with a selector that is not valid CSS
- **THEN** the call fails with an error naming the selector, not with an empty string

#### Scenario: Malformed XPath expression rejected

- **WHEN** a plugin calls `xpath_text` with an expression that is not valid XPath
- **THEN** the call fails with an error naming the expression, not with an empty string

#### Scenario: Unsupported selector syntax is a failure, not a silent miss

- **WHEN** a plugin calls a CSS lookup with a selector the host's selector engine does not support
- **THEN** the call fails and reports the problem instead of returning as if nothing matched

### Requirement: Equivalent HTML helper surface across runtimes

The Lua, JS and Yaegi runtimes SHALL expose the same nine functions — the parse function and the eight lookups — with the same argument order and the same results: the same value for the same markup, selector or expression and attribute name, and the same error text for the same invalid argument, selector or expression. A call SHALL be movable between a Lua and a JS plugin without change. Each runtime SHALL report failures in its own established idiom — a second return value in Lua, a thrown error in JS, an `error` return in Yaegi — while the reported text stays identical.

The Lua and JS runtimes SHALL expose the nine functions as a flat `html` group on the existing `host` surface, alongside `text`, `codecs`, `crypto`, `json` and `http`. No function of the group SHALL require method-call syntax or a receiver, and the group SHALL NOT be reachable through a bare Lua global.

#### Scenario: Same markup, same text in every runtime

- **WHEN** a Lua plugin, a JS plugin and a Yaegi plugin each parse the same markup and read the same selector's text
- **THEN** all three receive the same string

#### Scenario: Same call text in Lua and JS

- **WHEN** the same lookup is written in a Lua plugin and in a JS plugin
- **THEN** the two calls read identically, with the handle as the first argument and no receiver syntax

#### Scenario: Same markup, same list in every runtime

- **WHEN** a Lua plugin and a JS plugin each parse the same markup and list the same selector's `src` attributes
- **THEN** both receive the same values in the same order

#### Scenario: Same invalid selector, same error text

- **WHEN** both runtimes run a CSS lookup with the same invalid selector
- **THEN** both report the same error text

#### Scenario: Existing host groups unaffected

- **WHEN** a plugin calls an already-shipped helper such as `host.text.strip_html` or `host.json.decode`
- **THEN** it behaves exactly as it did before the `html` group was added

#### Scenario: No HTML global registered in Lua

- **WHEN** a Lua plugin refers to a bare `html` or `htmlparse` global
- **THEN** the call errors with "attempt to index a nil value" because the helpers live only under `host.html`
