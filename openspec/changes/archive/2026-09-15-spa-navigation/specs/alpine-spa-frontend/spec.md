## MODIFIED Requirements

### Requirement: SPA client-side navigation
The system SHALL provide a client-side SPA router that navigates between pages without a full page reload. The router SHALL capture internal link clicks and internal GET form submissions, fetch the target with `X-Partial: true`, swap the returned body into the page, and update `history.pushState`. It SHALL handle `popstate` so back and forward move between pages in place, and SHALL restore the scroll position recorded for a history entry. The router SHALL keep the highlighted navigation item matching the page now shown. External links, the reader page, and API endpoints SHALL bypass the router and perform standard navigation. When a navigation fetch fails, the router SHALL fall back to standard full-page navigation and report the failure.

#### Scenario: Internal link click
- **WHEN** a user clicks a link to `/view/library` from the search page
- **THEN** the router fetches `/view/library`, extracts `<main>`, morphs the DOM, and updates the URL bar — without a full page reload

#### Scenario: Back button
- **WHEN** the user presses the browser back button after SPA navigation
- **THEN** the `popstate` handler fetches the previous URL and restores its content

#### Scenario: External link bypass
- **WHEN** a link points to an external domain, or to the reader at `/view/read/{pluginID}/{mangaID}/{chapterID}`
- **THEN** the router performs standard full-page navigation

#### Scenario: Fetch error fallback
- **WHEN** a SPA navigation fetch returns a non-2xx status
- **THEN** the router falls back to standard full-page navigation and shows an error toast

#### Scenario: GET form navigates in place
- **WHEN** the user submits a GET form such as the search form targeting `/view/search`
- **THEN** the router navigates in place exactly as it does for a link to that URL, without a full page reload

#### Scenario: Navigation highlight follows the page
- **WHEN** the user navigates in place from the library page to the search page
- **THEN** the navigation bar highlights Search and no longer highlights Library

#### Scenario: Scroll position restored on back
- **WHEN** the user navigates away from a scrolled page and then presses back
- **THEN** the restored page returns to the scroll position it had when it was left

#### Scenario: Reader keeps standard navigation
- **WHEN** the user follows a link into the reader, or leaves it
- **THEN** the browser performs a standard full-page navigation, leaving the reader's own shell and history handling in charge

## ADDED Requirements

### Requirement: Self-hosted frontend assets
The page shell SHALL load its JavaScript framework and its webfont from the host's own asset root. No page SHALL depend on a third-party CDN for a script, a stylesheet, or a font file, so that a restricted, slow, or unavailable network cannot leave a page without the framework it needs to become interactive.

#### Scenario: No third-party asset reference
- **WHEN** a client inspects the page source of any view, including the reader
- **THEN** every `<script>` and `<link rel="stylesheet">` points at the host's own origin, and no asset request leaves for a third-party CDN

#### Scenario: Page stays interactive without a CDN
- **WHEN** a page is loaded with all third-party origins unreachable
- **THEN** the framework still loads and the page's interactive behaviour still works

#### Scenario: Vendored script and stylesheet are pre-compressed
- **WHEN** the frontend bundle is built
- **THEN** the vendored script and stylesheet sit alongside the other frontend assets and receive the same brotli pre-compression the asset handler serves to clients that accept it
