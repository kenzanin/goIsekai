## Purpose

Provides Alpine.js-based client-side interactivity and SPA navigation, replacing hand-written vanilla JS (toast system, confirm modal, data-confirm delegation) and dead HTMX imports with a unified reactive framework that makes page transitions instant and UI interactions declarative.

## ADDED Requirements

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

### Requirement: SPA client-side navigation
The system SHALL provide a client-side SPA router that intercepts internal link clicks and navigates via `fetch()` without full page reloads. The router SHALL extract the `<main>` content from the fetched HTML, morph the DOM, update `history.pushState`, and handle `popstate` for back/forward navigation. External links, reader pages, and API endpoints SHALL bypass the router and perform standard navigation.

#### Scenario: Internal link click
- **WHEN** a user clicks a link to `/view/library` from the search page
- **THEN** the router fetches `/view/library`, extracts `<main>`, morphs the DOM, and updates the URL bar — without a full page reload

#### Scenario: Back button
- **WHEN** the user presses the browser back button after SPA navigation
- **THEN** the `popstate` handler fetches the previous URL and restores its content

#### Scenario: External link bypass
- **WHEN** a link points to an external domain or `/view/manga/{plugin}/{id}` (reader)
- **THEN** the router performs standard full-page navigation

#### Scenario: Fetch error fallback
- **WHEN** a SPA navigation fetch returns a non-2xx status
- **THEN** the router falls back to standard full-page navigation and shows an error toast

### Requirement: Per-page Alpine.js components
Each view page SHALL use Alpine.js `x-data` for local interactive state. The following page-specific interactions SHALL be declarative Alpine.js components instead of hand-written JS:

- **Library**: sidebar collapse/expand toggle, stats accordion, duplicate groups expand/collapse
- **Search**: search input with debounced fetch, plugin dropdown filter that refreshes results via SPA router
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
- **THEN** the component calls `PUT /api/manga/{plugin}/{id}/title`, shows a success toast, and refreshes the page content via SPA morph

#### Scenario: Settings clear cache
- **WHEN** the user clicks "Clear All Cache" on settings
- **THEN** the confirm modal appears, and on confirm, `POST /action/clear-all-cache` is called, a success toast shows, and the cache stats update inline

### Requirement: Remove HTMX dependency
The system SHALL remove the HTMX CDN import and all dead HTMX error handlers from the frontend. No template SHALL contain `hx-*` attributes. The Alpine.js framework SHALL fully replace HTMX's role in the application.

#### Scenario: No HTMX in page source
- **WHEN** a client inspects the page source of any view
- **THEN** there are zero `hx-get`, `hx-post`, `hx-target`, or `hx-swap` attributes, and no HTMX script tag

#### Scenario: Error handling preserved
- **WHEN** a fetch-based action (SPA navigation or component action) fails
- **THEN** the error toast appears — same user-visible behavior as the old HTMX error handler
