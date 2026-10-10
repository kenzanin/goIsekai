## Purpose

Provides Lua-based server-side template rendering, replacing CloudyKit/jet rendering HTML views, enabling plugin contributors modify frontend templates using same Lua language already know from plugin development.

## Requirements

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

### Requirement: Template cache is the default render path

On a normal run, the system SHALL serve every render from the bytecode and source caches built at startup, and SHALL NOT read any template file from disk while serving requests. Re-reading templates from disk SHALL happen only when hot reload is explicitly enabled, and SHALL be limited to templates whose content has changed since the last check.

#### Scenario: Steady-state render does not touch disk
- **WHEN** a view route renders repeatedly with hot reload disabled
- **THEN** every render is served from the in-process caches and no template file is read from disk

#### Scenario: Modules pulled in by require are served from the cache too
- **WHEN** a view requires partial modules and renders repeatedly with hot reload disabled
- **THEN** those partials are served from the source cache and their files are not read from disk on each render

#### Scenario: Hot reload disabled is the default
- **WHEN** the server starts without an explicit hot reload setting
- **THEN** the engine runs in cache mode, and an edit to a template file is not visible until the process restarts

#### Scenario: Changed template is picked up under hot reload
- **WHEN** hot reload is enabled and a template file's contents differ from the previously loaded contents
- **THEN** that template is recompiled before its next render, and templates whose contents are unchanged keep their existing cache entries

#### Scenario: Unchanged templates are not recompiled under hot reload
- **WHEN** hot reload is enabled and only one template file was edited
- **THEN** only the edited template is recompiled; the remaining templates keep their existing cache entries

### Requirement: Compiling templates is bounded per render

A module that a template pulls in with require SHALL be compiled from cached source rather than recompiled from disk, so the per-render cost of a page view stays bounded by the view and layout themselves. Eliminating the remaining per-render compilation is not part of this capability.

#### Scenario: Repeated renders do not recompile partials from disk
- **WHEN** a view that requires several partial modules is rendered many times with hot reload disabled
- **THEN** no template file is read from disk by any of those renders

### Requirement: Template cache is safe under concurrent renders

The system SHALL allow concurrent requests to render templates without corrupting the cache. A cache refresh triggered by hot reload SHALL NOT be observed partially by an in-flight render.

#### Scenario: Concurrent renders during a refresh
- **WHEN** several requests render views at the same time one template file changes under hot reload
- **THEN** every request completes with either the previous or the refreshed template, and none observes a partially written cache entry

#### Scenario: A template that fails to recompile
- **WHEN** hot reload is enabled and an edited template no longer compiles
- **THEN** the previously cached bytecode for that template continues to serve requests and the failure is logged
