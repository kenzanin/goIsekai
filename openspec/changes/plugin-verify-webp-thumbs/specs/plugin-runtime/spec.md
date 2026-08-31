## Purpose

Extends the plugin runtime with host-side human-verification support, WebP image caching, and plugin-declared thumbnail aspect ratios.

## ADDED Requirements

### Requirement: Human verification session capture
The host SHALL store user-pasted verification cookies and an optional browser User-Agent per plugin id. When present, the host SHALL inject those cookies (and override the User-Agent) into the plugin's HTTP client before every request. Storage SHALL be a `plugin_verify` table keyed by plugin id.

#### Scenario: Paste cookies after challenge
- **WHEN** a plugin is flagged `needs_human_verify` and the user pastes cookies in the Plugins screen verification panel
- **THEN** subsequent host HTTP requests for that plugin carry the pasted cookies and User-Agent

#### Scenario: Tolerant cookie parsing
- **WHEN** the paste is a full `Cookie:` header, a single `name=value` pair, or a bare value
- **THEN** the host parses it into cookie pairs and stores them without error

### Requirement: Challenge detection surfaces verification banner
The host SHALL detect challenge responses (HTTP 403 with challenge markers such as `cf-challenge` or "Just a moment") and the affected view (search/detail) SHALL render a banner instructing the user to verify via the Plugins screen.

#### Scenario: Search hits a challenge
- **WHEN** a search request to a protected site returns a challenge response
- **THEN** the search view renders a "Site needs human verification" banner instead of silently showing no results

### Requirement: Image cache stored as WebP
The host SHALL convert cached images to WebP (quality ~85) at disk-cache write time, except animated or already-WebP images. Served responses SHALL carry the correct `Content-Type`. Existing cache entries SHALL NOT be migrated or invalidated.

#### Scenario: JPEG page cached as WebP
- **WHEN** a JPEG page image is written to the disk cache
- **THEN** it is stored as a `.webp` file at reduced size and served with `Content-Type: image/webp`

### Requirement: Plugin-declared thumbnail aspect ratio
Plugins SHALL be able to declare an optional `thumb_ratio` (width/height) in their Init response. The host SHALL persist it and search/library cards SHALL use it via `aspect-ratio` styling. Plugins without the field SHALL fall back to 2:3.

#### Scenario: MangaDex covers uncropped
- **WHEN** the mangadex plugin declares `thumb_ratio` 0.703
- **THEN** search and library cards render covers at that ratio without cropping

### Requirement: Verification metadata in plugin ABI
The plugin Init response SHALL support optional `verify_url` and `needs_human_verify` fields, persisted at registration, exposed to the Plugins screen.

#### Scenario: Plugin declares verification need
- **WHEN** a plugin's Init response contains `needs_human_verify: true` and a `verify_url`
- **THEN** the Plugins screen shows a Human Verification panel with an "Open verification page" link targeting that URL
