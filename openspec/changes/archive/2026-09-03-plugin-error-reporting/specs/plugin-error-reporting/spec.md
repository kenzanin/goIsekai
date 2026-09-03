## Purpose

Surfaces plugin errors to the user with clear error messages, retry buttons, and per-plugin health indicators instead of silently showing empty results.

## ADDED Requirements

### Requirement: Per-plugin error state in views
When a plugin call fails, the affected view (search results, manga detail) SHALL display the error message with the plugin name and a retry button instead of showing empty or misleading content.

#### Scenario: Search plugin fails
- **WHEN** a search request to a plugin returns an error
- **THEN** the search view shows "Plugin error: [message]" with a retry button for that plugin

#### Scenario: Detail plugin fails
- **WHEN** a manga detail request fails
- **THEN** the detail view shows the error message with a retry button

### Requirement: Consistent challenge detection
All views SHALL handle ChallengeError uniformly — showing the "Site needs human verification" banner with a link to the Plugins screen verification panel.

#### Scenario: Challenge on any view
- **WHEN** any plugin call returns a ChallengeError
- **THEN** the view shows the verification banner regardless of which view it is

### Requirement: Plugin health indicator
The Plugins page SHALL show per-plugin health: last successful call timestamp and error count since last success.

#### Scenario: Plugin health visible
- **WHEN** the user views the Plugins screen
- **THEN** each plugin card shows "Last success: [time]" and "Errors: [count]" since last success

#### Scenario: Error count resets on success
- **WHEN** a plugin call succeeds after previous failures
- **THEN** the error count resets to 0 and last success updates