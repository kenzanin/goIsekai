## Purpose

Provides a Lua-based server-side template engine that replaces CloudyKit/jet for rendering HTML views, enabling plugin contributors to modify frontend templates using the same Lua language they already know from plugin development.

## ADDED Requirements

### Requirement: Lua template rendering
The system SHALL execute Lua template files that return HTML strings, replacing jet's `tmpl.Execute()` calls. Each Lua template SHALL be a function that receives a data table and returns an HTML string. The engine SHALL create a lightweight Lua VM per render call and precompile template bytecodes at startup for fast execution.

#### Scenario: Successful template render
- **WHEN** a view handler calls `renderPage(w, "library", data)` with Lua templates enabled
- **THEN** the Lua engine loads the `library` template function, passes `data` as a Lua table, executes the function, and writes the returned HTML string to `w`

#### Scenario: Template not found
- **WHEN** a view handler requests a template name that does not exist in the embedded filesystem
- **THEN** the engine returns an error without writing to `w`

#### Scenario: Lua template execution error
- **WHEN** a Lua template throws a runtime error (nil dereference, syntax error)
- **THEN** the engine catches the error, writes a 500 response with the error message, and logs the full stack trace

### Requirement: Data marshaling from Go to Lua
The system SHALL convert Go `map[string]any` data structures into Lua tables recursively, supporting nested maps, slices (as Lua sequences), strings, numbers, booleans, and nil values. The marshaled data SHALL be available as a global `data` table in every template execution.

#### Scenario: Nested data access
- **WHEN** a handler passes `data["Ratios"]` as `map[string]string{"mangadex": "95%"}`
- **THEN** the Lua template can access `data.Ratios.mangadex` and read `"95%"`

#### Scenario: Slice data access
- **WHEN** a handler passes `data["Mangas"]` as a slice of structs
- **THEN** the Lua template can iterate with `for _, m in ipairs(data.Mangas) do` and access fields like `m.Title`, `m.MangaID`

#### Scenario: Nil value handling
- **WHEN** a Go data value is `nil`
- **THEN** the Lua template receives it as `nil` and `data.field == nil` evaluates to `true`

### Requirement: HTML auto-escape helper
The system SHALL expose a global `h(str)` function in every template VM that HTML-escapes ampersands, angle brackets, quotes, and apostrophes. All data interpolation in templates MUST use `h()` to prevent XSS — raw string concatenation is unescaped.

#### Scenario: Escape HTML entities
- **WHEN** a template calls `h("<script>alert('xss')</script>")`
- **THEN** the function returns `&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;`

#### Scenario: Nil passthrough
- **WHEN** a template calls `h(nil)`
- **THEN** the function returns an empty string `""`

### Requirement: Go helper function registration
The system SHALL expose the following Go functions as Lua globals available in every template: `formatDate`, `formatChapterNum`, `getInitials`, `formatBytes`, `pageWindow`, `pageURL`. Each function SHALL accept the same parameters as its jet counterpart and return the same formatted string.

#### Scenario: formatDate in Lua
- **WHEN** a template calls `formatDate("2006-01-02T15:04:05Z", data.Manga.UpdatedAt)`
- **THEN** the function returns the date formatted in the user's local timezone

#### Scenario: formatChapterNum in Lua
- **WHEN** a template calls `formatChapterNum("31.5b")`
- **THEN** the function returns `"Ch. 31.5B"`

### Requirement: Template composition via require
The system SHALL support `require("module.path")` inside Lua templates to load sibling template modules (layouts, partials) from the same embedded filesystem. Each required module SHALL return a Lua value (function or table) that the caller can invoke.

#### Scenario: Layout composition
- **WHEN** a view template calls `local base = require("layouts.base")` and then `base(data, content)`
- **THEN** the engine loads `layouts/base.lua`, executes it, and returns the composed HTML string

#### Scenario: Partial inclusion
- **WHEN** a layout template calls `require("partials.nav")` to get a nav renderer
- **THEN** the engine loads `partials/nav.lua` and the returned function produces the nav HTML

### Requirement: Development mode hot reload
When dev mode is enabled (`--dev` flag), the system SHALL read template files from disk on each render instead of using embedded bytecodes, enabling live template editing without server restart. The system SHALL fall back to embedded templates if a disk file is missing.

#### Scenario: Dev mode reads from disk
- **WHEN** dev mode is enabled and a template file is modified on disk
- **THEN** the next render call picks up the changes without restart

#### Scenario: Dev mode fallback to embedded
- **WHEN** dev mode is enabled and a template file is missing from disk
- **THEN** the engine falls back to the embedded version and logs a warning
