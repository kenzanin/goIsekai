# Spec Delta

## Purpose

Desktop Qt6 client page-level behavior: navigation between pages, manga detail rendering from the server API, chapter progress display, and back-to-library navigation.

## ADDED Requirements

### Requirement: Manga detail page renders from server API

The Qt6 client SHALL fetch manga detail from `GET /api/manga/{pluginID}/{mangaID}` and render: cover image, title, description, status, author/artist, genres/tags, alt titles, and chapter count. The page SHALL show a loading state while fetching and an error state if the request fails.

#### Scenario: Detail page opens from library card click
- **WHEN** the user clicks a manga card in the library grid
- **THEN** the client navigates to the manga detail page in-process (no browser opens) and fetches detail from the server API

#### Scenario: API request fails
- **WHEN** the detail API returns non-200 or unreachable
- **THEN** the page displays an error message with a retry action

### Requirement: Chapter list with progress

The detail page SHALL display the chapter list with per-chapter read/unread state from the server progress data. Chapters SHALL be paginated when the total exceeds one page.

#### Scenario: Chapters with read progress
- **WHEN** the detail page loads and the manga has chapters with read progress
- **THEN** each chapter row shows its read/unread state matching the server data

### Requirement: Back-to-library navigation

The detail page SHALL provide a back control that returns the user to the library tab with the previously applied filters and page position intact.

#### Scenario: Back button pressed
- **WHEN** the user clicks the back button on the detail page
- **THEN** the client returns to the library tab with the same filter state as before navigating away
