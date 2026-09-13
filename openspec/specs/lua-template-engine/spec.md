## Purpose

Provides Lua-based server-side template rendering, replacing CloudyKit/jet rendering HTML views, enabling plugin contributors modify frontend templates using same Lua language already know from plugin development.

## ADDED Requirements

### Requirement: Lua template rendering
The system SHALL execute Lua template files and return HTML strings, replacing jet's `tmpl.Execute()` calls. Lua template SHALL accept a function that receives data and returns HTML string. Engine SHALL create lightweight Lua VM per render call, precompile template bytecodes at startup for fast execution.

#### Scenario: Successful template render
- **WHEN** view handler calls `renderPage(w, "library", data)` with Lua templates enabled
- **THEN** Lua engine loads `library` template function, passes `data` Lua table, executes function, writes returned HTML string to `w`

#### Scenario: Template not found
- **WHEN** view handler requests template name does not exist in embedded filesystem
- **THEN** engine returns error without writing to `w`

#### Scenario: Lua template execution error
- **WHEN** Lua template throws runtime error (nil dereference, syntax error)
- **THEN** engine catches error, writes 500 response error message, logs full stack trace

### Requirement: Data marshaling Go → Lua
The system SHALL convert Go `map[string]any` data structures into Lua tables recursively, supporting nested maps, slices (as Lua sequences), strings, numbers, booleans, nil values. Marshaled data SHALL be available as global `data` table in every template execution.

#### Scenario: Nested data access
- **WHEN** handler passes `data["Ratios"]` as `map[string]string{"mangadex": "95%"}`
- **THEN** Lua template can access `data.Ratios.mangadex` and reads `"95%"`

#### Scenario: Slice data access
- **WHEN** handler passes `data["Mangas"]` as a slice of structs
- **THEN** Lua template can iterate over the slice as a numeric sequence

### Requirement: HTML auto-escape helper
The system SHALL expose global `h(str)` function in the Lua VM that HTML-escapes ampersands, angle brackets, quotes, apostrophes. All data interpolation in templates MUST use `h()` to prevent XSS — raw string concatenation is unescaped.

#### Scenario: Escape HTML entities
- **WHEN** template calls `h("<script>alert('xss')</script>")`
- **THEN** returns `&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;`

#### Scenario: Nil passthrough
- **WHEN** template calls `h(nil)`
- **THEN** returns empty string `""`

### Requirement: Go helper function registration
The system SHALL expose these functions as Lua globals available in templates: `formatDate`, `formatChapterNum`, `getInitials`, `formatBytes`, `pageWindow`, `pageURL`. Each function SHALL accept the same arguments as the jet counterpart and return the same formatted string.

#### Scenario: formatDate in Lua
- **WHEN** template calls `formatDate("2006-01-02T15:04:05Z", data.Manga.UpdatedAt)`
- **THEN** returns date formatted in user's local timezone

#### Scenario: formatChapterNum in Lua
- **WHEN** template calls `formatChapterNum("31.5b")`
- **THEN** returns `"Ch. 31.5B"`

### Requirement: Template composition
The system SHALL support `require("module.path")` inside Lua templates to load sibling template modules (layouts, partials) from the same embedded filesystem. The module SHALL return a Lua value (function table) that the caller can invoke.

#### Scenario: Layout composition
- **WHEN** view template calls `local base = require("layouts.base")` then `base(data, content)`
- **THEN** engine loads `layouts/base.lua`, executes it, returns composed HTML string

#### Scenario: Partial inclusion
- **WHEN** template calls `require("partials/some_part")`
- **THEN** engine loads and executes the partial, returns its output as a Lua value

#### Scenario: Dev mode fallback
- **WHEN** dev mode is enabled and a template file is missing from disk
- **THEN** engine falls back to the embedded version and logs a warning
