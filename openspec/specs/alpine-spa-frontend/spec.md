## Purpose

Provides Alpine.js-based client-side interactivity, replacing hand-written vanilla JS (toast system, confirm modal, data-confirm delegation) and dead HTMX imports with a unified reactive framework that makes UI interactions declarative. Page-to-page navigation is left to the browser; only mutating form actions are posted in place.

## Requirements

### Requirement: Global toast notification store
The system SHALL provide a global Alpine.js `$store.toast` that displays stacking toast notifications with auto-dismiss, color-coded by type (success, error, info). Toasts SHALL slide in from the top-right, stack vertically (max 4 visible), and auto-dismiss after a configurable duration. The store SHALL expose a `show(message, type)` method callable from any component or `htmx:responseError` equivalent.

#### Scenario: Success toast
- **WHEN** any component calls `Alpine.store('toast').show('Saved!', 'success')`
- **THEN** a green-bordered toast appears in the top-right corner, displays for 3.5 seconds, and slides out

#### Scenario: Error toast
- **WHEN** any component calls `Alpine.store('toast').show('Failed to load', 'error')`
- **THEN** a red-bordered toast appears, displays for 6 seconds, and can be dismissed by clicking

#### Scenario: Stacking limit
- **WHEN** 5 toasts are triggered in rapid succession
- **THEN** only the 4 most recent are visible; older toasts are removed from DOM

### Requirement: Global confirm modal store
The system SHALL provide a global `Alpine.store('confirm')` that displays a centered modal dialog with a message, confirm button, and cancel button. The store SHALL expose a `show(message)` method that returns a Promise resolving to `true` (confirmed) or `false` (cancelled). Components SHALL use this to replace all `confirm()` calls.

#### Scenario: Confirm accepted
- **WHEN** a component calls `await Alpine.store('confirm').show('Clear all cache?')` and the user clicks Confirm
- **THEN** the modal closes and the Promise resolves to `true`

#### Scenario: Confirm cancelled
- **WHEN** the modal is displayed and the user clicks Cancel or presses Escape
- **THEN** the modal closes and the Promise resolves to `false`

#### Scenario: Confirm triggered by attribute
- **WHEN** a button has `data-confirm="Clear all cache?"` and is clicked
- **THEN** the confirm modal appears before the button's default action proceeds (only proceeds on confirm)

### Requirement: Browser-native navigation
Internal links SHALL NOT be intercepted. Every link click SHALL perform a standard browser navigation so that history, the back/forward buttons, scroll restoration, and link affordances (middle-click, ctrl-click, "open in new tab") behave as the browser intends, and the page a reader returns to is the page they actually came from.

#### Scenario: Internal link click
- **WHEN** a user clicks a link to `/view/library` from the search page
- **THEN** the browser performs a normal navigation with no click interception

#### Scenario: Back button restores the origin page
- **WHEN** the user opens a manga detail page from a search result, performs actions there, and presses the browser back button
- **THEN** the browser returns to the search URL with the original query and results intact

#### Scenario: Shell differences are handled by the server
- **WHEN** the user moves between a nav-bearing page and the nav-less reader
- **THEN** the destination page is rendered and served complete by the server, so the nav bar is present on every page that has one

### Requirement: In-place action submission
Mutating form actions posted to `/action/` from a manga detail page SHALL be submitted with `fetch()` and have their response swapped into `#content`, so that repeated clicks (genre chips, library toggle, enrichment) never reload the page and never add a history entry. The address bar SHALL keep the page URL and SHALL NOT be updated to the `/action/` URL.

#### Scenario: Repeated genre clicks
- **WHEN** the user clicks several genre chips in a row on a detail page
- **THEN** each click updates the page in place, with no reload and no change to the address bar

#### Scenario: Library toggle
- **WHEN** the user clicks the library toggle on a detail page
- **THEN** the button flips state in place and the address bar still shows `/view/manga/...`

#### Scenario: Fetch error fallback
- **WHEN** an in-place action fetch returns a non-2xx status
- **THEN** an error toast is shown and the page is left unchanged

### Requirement: Per-page Alpine.js components
Each view page SHALL use Alpine.js `x-data` for local interactive state. The following page-specific interactions SHALL be declarative Alpine.js components instead of hand-written JS:

- **Library**: sidebar collapse/expand toggle, stats accordion, duplicate groups expand/collapse
- **Search**: search input with debounced fetch, plugin dropdown filter that refreshes results
- **Detail**: alt-title dropdown with add/remove/promote via fetch, cache clear with confirm
- **Plugins**: profile dropdown with test button (shows toast result), reset pin button
- **Settings**: clear cache with confirm modal
- **Logs**: WebSocket connection state, poll fallback, filter controls, auto-scroll toggle

#### Scenario: Library sidebar toggle
- **WHEN** the user clicks the sidebar collapse button on the library page
- **THEN** the sidebar collapses with a CSS transition and the main content area expands

#### Scenario: Search debounce
- **WHEN** the user types in the search input
- **THEN** the search triggers a fetch 300ms after the last keystroke, not on every keypress

#### Scenario: Plugin profile test
- **WHEN** the user clicks "Test" on a plugin's TLS profile card
- **THEN** the button shows a loading state, calls `POST /action/test-profile/{id}`, and shows a green "OK 200 via firefox_148" toast on success or a red "403 blocked" toast on failure

### Requirement: Alpine.js data-bind on forms
Forms and interactive elements SHALL use Alpine.js `x-model`, `x-on:click`, `x-show`, and `x-for` instead of inline `<script>` blocks or `data-confirm` delegation. Each form SHALL handle its own submission via `fetch()` and display results via `$store.toast`.

#### Scenario: Alt-title promote
- **WHEN** the user selects a new main title from the alt-title dropdown on the detail page
- **THEN** the form is posted to `/action/set-title/{plugin}/{id}`, the response is swapped into `#content`, and a success toast is shown

#### Scenario: Settings clear cache
- **WHEN** the user clicks "Clear All Cache" on settings
- **THEN** the confirm modal appears, and on confirm, `POST /action/clear-all-cache` is called, a success toast shows, and the cache stats update inline

### Requirement: Remove HTMX dependency
The system SHALL remove the HTMX CDN import and all dead HTMX error handlers from the frontend. No template SHALL contain `hx-*` attributes. The Alpine.js framework SHALL fully replace HTMX's role in the application.

#### Scenario: No HTMX in page source
- **WHEN** a client inspects the page source of any view
- **THEN** there are zero `hx-get`, `hx-post`, `hx-target`, or `hx-swap` attributes, and no HTMX script tag

#### Scenario: Error handling preserved
- **WHEN** a fetch-based action fails
- **THEN** the error toast appears — same user-visible behavior as the old HTMX error handler
