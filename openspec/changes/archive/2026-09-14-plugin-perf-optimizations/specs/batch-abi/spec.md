## Purpose

Allow plugins to return both manga detail and chapter list in a single invocation, halving HTTP roundtrips when both are needed simultaneously (e.g., detail page loads).

## ADDED Requirements

### Requirement: Batch detail+chapters plugin function
The plugin ABI SHALL include a `GetMangaDetailWithChapters(mangaID string) (string, error)` function that returns both manga detail and chapter list in a single response. The response SHALL be a JSON object containing `detail` and `chapters` fields.

#### Scenario: Plugin implements batch function
- **WHEN** a plugin exports `GetMangaDetailWithChapters`
- **THEN** the host invokes it instead of separate `GetMangaDetail` + `GetChapterList` calls when both are needed

#### Scenario: Plugin does not implement batch function
- **WHEN** a plugin does not export `GetMangaDetailWithChapters`
- **THEN** the host falls back to separate `GetMangaDetail` + `GetChapterList` calls

#### Scenario: Batch function returns valid response
- **WHEN** `GetMangaDetailWithChapters` is invoked and returns valid JSON with `detail` and `chapters`
- **THEN** the host parses both and returns them as separate typed results

#### Scenario: Batch function returns error
- **WHEN** `GetMangaDetailWithChapters` returns an error
- **THEN** the host returns the error without falling back to separate calls

### Requirement: Host batch invocation
The host SHALL invoke `GetMangaDetailWithChapters` when both detail and chapters are needed for the same manga (e.g., detail page load, library update). The host SHALL NOT invoke it when only detail or only chapters are requested.

#### Scenario: Detail page triggers batch call
- **WHEN** a user opens a manga detail page
- **THEN** the host invokes `GetMangaDetailWithChapters` if available, otherwise falls back to separate calls

#### Scenario: Library update uses batch call
- **WHEN** the library update process fetches detail+chapters for multiple manga
- **THEN** the host invokes `GetMangaDetailWithChapters` for each manga if available

#### Scenario: Single-field request does not trigger batch
- **WHEN** only `GetMangaDetail` or only `GetChapterList` is requested
- **THEN** the host does not invoke `GetMangaDetailWithChapters`

### Requirement: Backward compatible ABI
The batch function SHALL be optional. Existing plugins that do not implement it SHALL continue to work without modification. The host SHALL gracefully fall back to separate function calls.

#### Scenario: Legacy plugin unaffected
- **WHEN** a plugin does not export `GetMangaDetailWithChapters`
- **THEN** the host uses separate `GetMangaDetail` + `GetChapterList` calls as before
