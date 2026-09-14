## MODIFIED Requirements

### Requirement: Provider discovery
The system SHALL expose a JSON list of all available alt-title lookup servers, aggregated from the host's built-in enrichment providers and from plugins that declare the alt-title enricher capability. The list SHALL contain, per entry, the provider identifier, the server identifier, and a display name. Built-in MangaDex and MangaUpdates providers SHALL always be present; plugin-declared servers SHALL be added without the host hardcoding them.

#### Scenario: Plugin declares multiple servers
- **WHEN** a plugin declares two lookup servers in its metadata
- **THEN** `GET /api/alt-title-servers` returns both entries tagged with that plugin's id alongside the built-in providers

#### Scenario: No provider installed
- **WHEN** no active plugin exposes the alt-title capability
- **THEN** `GET /api/alt-title-servers` returns the built-in MangaDex and MangaUpdates entries and no error

### Requirement: Fetch alternative titles
The system SHALL fetch alternative titles for a library manga by resolving the user-selected server to either a host-built-in provider (MangaDex, MangaUpdates) or a plugin-declared provider, invoking it with the manga's current title, and merging the returned titles into persistent storage tagged with the provider-reported source label. Titles already stored for that manga SHALL be skipped (no duplicates). An empty result SHALL NOT delete existing rows.

#### Scenario: Fetch with a chosen server
- **WHEN** `POST /api/manga/{pluginID}/{mangaID}/alt-titles` is called with a server id offered by an installed or built-in provider
- **THEN** new titles returned by the provider are stored with their source label and pre-existing titles are unchanged

#### Scenario: Fetch from a built-in provider
- **WHEN** the request names the `mangadex` or `mangaupdates` server and no plugin declares it
- **THEN** the host resolves the title through its built-in provider and stores the returned titles

#### Scenario: Unknown server
- **WHEN** the request names a server id not present in the aggregated server list
- **THEN** the API responds with an error indicating the server is unavailable
