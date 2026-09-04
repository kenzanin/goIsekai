## Purpose

Add standard keyboard navigation to the manga reader for efficient page-by-page reading without mouse interaction.

## ADDED Requirements

### Requirement: Page navigation via keyboard
The reader SHALL support keyboard shortcuts for page navigation: ArrowLeft (previous page), ArrowRight (next page), Space (next page). Shortcuts SHALL be active when the reader canvas is visible and no input element is focused.

#### Scenario: Arrow right advances page
- **WHEN** the user presses ArrowRight while viewing the reader
- **THEN** the next page is displayed

#### Scenario: Arrow left goes to previous page
- **WHEN** the user presses ArrowLeft while viewing the reader
- **THEN** the previous page is displayed

#### Scenario: Space advances page
- **WHEN** the user presses Space while viewing the reader
- **THEN** the next page is displayed

#### Scenario: Shortcuts disabled during input focus
- **WHEN** an input or textarea element is focused
- **THEN** keyboard shortcuts do not trigger page navigation

### Requirement: Escape to exit reader
Pressing Escape SHALL navigate back to the manga detail page.

#### Scenario: Escape exits reader
- **WHEN** the user presses Escape while viewing the reader
- **THEN** the browser navigates to the manga detail page

### Requirement: Chapter boundary navigation
When the user is on the last page and presses next, the reader SHALL navigate to the next chapter (same as clicking the next-chapter button). When on the first page and presses previous, it SHALL navigate to the previous chapter.

#### Scenario: Next chapter via keyboard
- **WHEN** the user is on the last page and presses ArrowRight
- **THEN** the reader loads the next chapter's first page

#### Scenario: Previous chapter via keyboard
- **WHEN** the user is on the first page and presses ArrowLeft
- **THEN** the reader loads the previous chapter's last page
