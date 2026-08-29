## Purpose

Exposes the reader's core operations to the Wails frontend and serves manga images through a local proxy protocol so the UI can read content without CORS or hotlink restrictions.

## ADDED Requirements

### Requirement: Search service binding
The host SHALL expose a `SearchManga` service binding that accepts a plugin id and a `SearchFilter` and returns matching manga.

#### Scenario: Search via binding
- **WHEN** the frontend calls `SearchManga` with a plugin id and filter
- **THEN** the host returns `[]Manga` results or an error

### Requirement: Manga detail binding
The host SHALL expose a `GetMangaDetails` service binding returning a manga and its chapter list.

#### Scenario: Fetch manga details
- **WHEN** the frontend calls `GetMangaDetails` with a plugin id and manga id
- **THEN** the host returns the `Manga`, its `[]Chapter`, or an error

### Requirement: Page list binding
The host SHALL expose a `GetPageList` service binding returning pages for a chapter.

#### Scenario: Fetch pages
- **WHEN** the frontend calls `GetPageList` with a plugin id and chapter id
- **THEN** the host returns the ordered `[]Page`, or an error

### Requirement: Library toggle binding
The host SHALL expose a `ToggleLibraryItem` service binding that toggles a manga's library membership.

#### Scenario: Toggle library membership
- **WHEN** the frontend calls `ToggleLibraryItem` for a manga
- **THEN** the manga's `in_library` flag flips and persists

### Requirement: Plugin install binding
The host SHALL expose an `InstallPlugin` service binding that installs a plugin from a `.wasm` file path.

#### Scenario: Install a plugin
- **WHEN** the frontend calls `InstallPlugin` with a `.wasm` file path
- **THEN** the plugin is registered and becomes callable, or an error is returned

### Requirement: Local image proxy protocol
The host SHALL register a custom protocol handler (e.g. `manga-img://`) that fetches image bytes with the correct HTTP `Referer` and serves them to the frontend.

#### Scenario: Proxy a manga image
- **WHEN** the frontend requests a page image via the custom protocol
- **THEN** the host fetches the image bytes using the page's headers (including Referer) and returns them, bypassing CORS and hotlink restrictions

### Requirement: Cached image latency
Image loading via the custom protocol SHALL maintain sub-second latency for cached images.

#### Scenario: Serve a cached image
- **WHEN** an image is already cached
- **THEN** the host serves it in under one second
