## ADDED Requirements

### Requirement: Alt-title removal endpoint

The system SHALL expose alt-title removal under `/api/manga/{pluginID}/{mangaID}/alt-titles`: `DELETE` with `{"title"}` removes one stored row and responds with the manga's remaining stored titles. Removal of the current main title SHALL be rejected with a validation error, and a request without a title SHALL be rejected without removing anything.

#### Scenario: Delete a stored title
- **WHEN** `DELETE /api/manga/p/m/alt-titles` is called with a stored title
- **THEN** the response is `200` and the title no longer appears in subsequent listings

#### Scenario: Current main title not removable
- **WHEN** `DELETE /api/manga/p/m/alt-titles` is called with the title the manga currently shows as its main title
- **THEN** the response is a validation error and the stored titles are unchanged

#### Scenario: Missing title rejected
- **WHEN** `DELETE /api/manga/p/m/alt-titles` is called without a title
- **THEN** the response is a validation error and no stored row is removed

## REMOVED Requirements

### Requirement: Alt-title server discovery endpoint
**Reason**: `GET /api/alt-title-servers` is deleted. The list existed to populate a fetch endpoint that no longer exists, and the plugins it aggregated declared a capability that no longer exists.
**Migration**: Use the enrichment catalog for available metadata sources, and `POST /action/fetch-enrichment/{pluginID}/{mangaID}` (or the enrichment panel) to fetch. A client reading `GET /api/alt-title-servers` receives `404` and should switch to the catalog.

### Requirement: Alt-title management endpoints
**Reason**: This requirement covered both fetching from a named alt-title server and removing a stored row. The fetch half is deleted with the rest of the lookup API; the removal half survives, restated as the narrower "Alt-title removal endpoint" added by this change.
**Migration**: Fetch through the enrichment endpoint — `POST /action/fetch-enrichment/{pluginID}/{mangaID}` with a title and a source — and detach the alt-title fetch and alt-summary fetch actions. Removal is unchanged in behavior, but is documented as its own requirement.
