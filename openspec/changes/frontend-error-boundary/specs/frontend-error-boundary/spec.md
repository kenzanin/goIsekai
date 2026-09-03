## Purpose

Provides user-facing error feedback when the reader canvas or HTMX views encounter network, parse, or plugin errors, instead of showing blank/broken UI.

## ADDED Requirements

### Requirement: Reader fetch error handling
All fetch calls in reader.js SHALL be wrapped in try-catch. On network error, parse error, or plugin error response, the reader canvas SHALL display a visible error overlay with the error type and a retry button.

#### Scenario: Network error during reader-data fetch
- **WHEN** `/api/reader-data` returns a network error or non-200 status
- **THEN** the reader canvas shows an error overlay with "Network error" and a retry button

#### Scenario: Corrupt JSON response
- **WHEN** `/api/reader-data` returns invalid JSON
- **THEN** the reader canvas shows an error overlay with "Parse error" and a retry button

#### Scenario: Plugin error in response
- **WHEN** `/api/reader-data` returns a JSON error envelope `{"error": "..."}`
- **THEN** the reader canvas shows the plugin error message and a retry button

### Requirement: HTMX view error display
The base layout SHALL register an `htmx:responseError` event handler that displays a toast notification for failed HTMX requests (search, detail, library actions).

#### Scenario: HTMX request fails
- **WHEN** an HTMX `hx-get` or `hx-post` request returns a non-2xx status
- **THEN** a toast notification appears with the error message and auto-dismisses after 5 seconds

#### Scenario: Successful requests unaffected
- **WHEN** an HTMX request returns 200
- **THEN** no toast appears and the response is rendered normally
