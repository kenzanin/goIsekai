## Purpose

Give Lua, JS and Yaegi plugins one host-backed way to read a scraped HTML page, so a plugin looks values up by CSS selector or XPath instead of hand-rolling tag walking, and the same lookup returns the same value in every runtime.

## ADDED Requirements

### Requirement: HTML document handle

The host SHALL expose a document handle built from a markup string: `host.html.parse(html)` in the Lua and JS runtimes, and `hostnet.Parse(html)` in the Yaegi runtime. The handle SHALL expose the eight lookup functions defined below and SHALL be the only way a plugin reaches them.

Parsing SHALL take the markup as a string argument supplied by the plugin. The handle SHALL NOT read a file, accept a path or URL, or open a network connection, so HTML parsing SHALL NOT widen the plugin file, IO or process surface.

Malformed or truncated markup SHALL NOT be a parse failure: the handle SHALL be built from whatever tree the parser recovers. A non-string argument SHALL fail and report the failure to the caller instead of returning a handle.

#### Scenario: Handle built from markup

- **WHEN** a Lua plugin calls `host.html.parse` with the markup string `<h1 class="t">Solo Leveling</h1>`
- **THEN** it receives a handle, and lookups on that handle see the document

#### Scenario: Non-string argument rejected

- **WHEN** a plugin calls `host.html.parse` with a number instead of a string
- **THEN** the call fails with an error about the argument and no handle is returned

#### Scenario: Malformed markup still yields a handle

- **WHEN** a plugin calls `host.html.parse` with markup that is unclosed or otherwise invalid HTML
- **THEN** it receives a handle and a lookup against it returns the recovered content rather than failing

#### Scenario: Handle cannot reach the filesystem

- **WHEN** a plugin calls `host.html.parse` with a filesystem path instead of markup
- **THEN** it receives a handle parsed from that literal text, and no host file is read

### Requirement: CSS selector lookups

The document handle SHALL expose four CSS-selector lookups, available as `doc.find_text(selector)`, `doc.find_attr(selector, attr)`, `doc.find_list_text(selector)` and `doc.find_list_attr(selector, attr)` in the Lua and JS runtimes, and as `FindText`, `FindAttr`, `FindListText` and `FindListAttr` methods on the Yaegi handle.

- `find_text` SHALL return the text content of the first element matching the selector, with surrounding whitespace trimmed.
- `find_attr` SHALL return the value of the named attribute on the first element matching the selector.
- `find_list_text` SHALL return the trimmed text content of every element matching the selector, in document order.
- `find_list_attr` SHALL return the value of the named attribute for every element matching the selector, in document order.

#### Scenario: First match text read

- **WHEN** a plugin calls `find_text` with a selector matching `<span class="status">Ongoing</span>`
- **THEN** it receives `Ongoing` with surrounding whitespace removed

#### Scenario: First match attribute read

- **WHEN** a plugin calls `find_attr` with a selector matching an anchor whose `href` is `/read/1`
- **THEN** it receives `/read/1`

#### Scenario: First match wins when several elements match

- **WHEN** a selector matches three elements with different text
- **THEN** `find_text` returns only the first one's text

#### Scenario: List of texts read

- **WHEN** a plugin calls `find_list_text` with a selector matching three elements carrying the genres `Action`, `Adventure` and `Drama`
- **THEN** it receives those three values in document order

#### Scenario: List of attributes read

- **WHEN** a plugin calls `find_list_attr` with a selector matching four page images and the attribute `src`
- **THEN** it receives the four `src` values in document order

### Requirement: XPath lookups

The document handle SHALL expose four XPath lookups, available as `doc.xpath_text(expr)`, `doc.xpath_attr(expr, attr)`, `doc.xpath_list_text(expr)` and `doc.xpath_list_attr(expr, attr)` in the Lua and JS runtimes, and as `XPathText`, `XPathAttr`, `XPathListText` and `XPathListAttr` methods on the Yaegi handle.

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

The Lua, JS and Yaegi runtimes SHALL expose the same eight lookups with the same results: the same value for the same markup, selector or expression and attribute name, and the same error text for the same invalid selector or expression. Each runtime SHALL report failures in its own established idiom — a second return value in Lua, a thrown error in JS, an `error` return in Yaegi — while the reported text stays identical. The Lua and JS runtimes SHALL expose the lookups on the existing `host` surface as a `html` group, alongside `text`, `codecs`, `crypto`, `json` and `http`.

#### Scenario: Same markup, same text in every runtime

- **WHEN** a Lua plugin, a JS plugin and a Yaegi plugin each parse the same markup and read the same selector's text
- **THEN** all three receive the same string

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
