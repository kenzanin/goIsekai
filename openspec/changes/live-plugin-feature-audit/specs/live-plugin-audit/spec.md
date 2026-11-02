# live-plugin-audit

## ADDED Requirements

### Requirement: Audit every active plugin end-to-end through the live host

The audit SHALL exercise every plugin reported active by the running host's plugin list through the host HTTP API (not by loading plugin source directly), covering the required ABI surface: search, detail, chapter list, page list.

- The audit SHALL record per plugin: search HTTP status and result count, detail HTTP status and whether `genres` or `status` populated, chapter-list HTTP status and how many chapters carry `released_at`, page-list HTTP status and page count for one sampled chapter.
- The audit SHALL classify each plugin per feature as WORKING, MISSING (feature answered but data absent), or BROKEN (HTTP error/empty).
- The audit SHALL run read-only against the server: fetching through existing host endpoints only, never mutating plugin source or caches.

#### Scenario: Gated plugin returns failure without crashing audit
- **GIVEN** a plugin whose source site is unreachable or challenge-blocked
- **WHEN** the audit probes its search
- **THEN** the audit marks that plugin BROKEN for that feature
- **AND** continues auditing every other plugin without aborting

### Requirement: Fix broken plugins following established patterns

For each plugin the audit classifies BROKEN or MISSING on a feature, the remediation step SHALL patch the plugin source in `examples/plugins/` (and sync to `app_data/plugins/`) using the established patterns: balanced-object RSC parsing for JS chunk payloads, `date_to_iso` normalization for dates, optional getGenres export from catalog markup, per-page headers for gated image CDNs.

- Remediation SHALL NOT change the host, the ABI, or any other plugin.
- Remediation SHALL clear caches that record the absence of an optional export when adding one (e.g. `plugins.genres = NULL` after adding getGenres).
- After each remediation, the audit SHALL re-probe that plugin live and record the before/after status.

#### Scenario: Plugin missing search genres gains them
- **GIVEN** a plugin whose search returns items without a genres array although its detail payload carries one
- **WHEN** remediation applies the balanced-object pattern and refreshes the live plugin
- **THEN** a re-probe of that plugin's search returns items with populated genres
