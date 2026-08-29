# goIsekai UI Specification

> **Version**: 1.0  
> **Target**: Desktop manga reader (Go + Wails v3)  
> **Frontend**: Plain HTML/CSS/JS (no framework, no build step)  
> **Default window**: 1200×800px (from config)

---

## 1. Screen Inventory

| # | Screen | Purpose |
|---|--------|---------|
| 1 | **Library** | Default landing. Shows manga the user has added to their library. Grid of cover cards. |
| 2 | **Search** | Search a specific plugin/source for manga. Shows results as a grid. Has a plugin picker and search input. |
| 3 | **Manga Detail** | Shows cover, metadata, description, and chapter list for one manga. Entry point to reading. |
| 4 | **Reader** | Full-bleed image viewer for reading a chapter page-by-page. Prev/next navigation, keyboard shortcuts, progress persistence. |
| 5 | **Plugins** | Lists installed plugins. Allows installing a new plugin via file picker. |

---

## 2. Navigation Model

### 2.1 Hash Router (in-memory, no framework)

The app uses `window.location.hash` for routing. Each view is a hash fragment:

| Hash | View | Params |
|------|------|--------|
| `#/` or `#/library` | Library | — |
| `#/search` | Search | `?plugin=<id>&q=<query>&page=<n>` |
| `#/manga/<pluginID>/<mangaID>` | Manga Detail | pluginID and mangaID are URL-encoded |
| `#/read/<pluginID>/<mangaID>/<chapterID>?page=<n>` | Reader | page is 0-indexed, defaults to 0 |
| `#/plugins` | Plugins | — |

### 2.2 Router Implementation Pattern

```js
// app.js — router skeleton
function router() {
  const hash = window.location.hash || '#/library';
  const [path, queryStr] = hash.split('?');
  const params = new URLSearchParams(queryStr || '');
  const segments = path.replace('#/', '').split('/');

  // Hide all views
  document.querySelectorAll('.view').forEach(v => v.classList.remove('active'));

  switch (segments[0]) {
    case 'library': case '':
      showView('library-view');
      loadLibrary();
      break;
    case 'search':
      showView('search-view');
      loadSearch(params.get('plugin'), params.get('q'), parseInt(params.get('page') || '1'));
      break;
    case 'manga':
      showView('manga-view');
      loadMangaDetail(decodeURIComponent(segments[1]), decodeURIComponent(segments[2]));
      break;
    case 'read':
      showView('reader-view');
      loadReader(decodeURIComponent(segments[1]), decodeURIComponent(segments[2]),
                 decodeURIComponent(segments[3]), parseInt(params.get('page') || '0'));
      break;
    case 'plugins':
      showView('plugins-view');
      loadPlugins();
      break;
  }
}
window.addEventListener('hashchange', router);
window.addEventListener('DOMContentLoaded', router);
```

### 2.3 Navigation Flows

```
Library ──click card──→ Manga Detail ──click chapter──→ Reader
    │                       │                              │
    │                       │←── Esc / back button ────────│
    │                       │
    ├── nav "Search" ──→ Search ──click result──→ Manga Detail
    │                       │
    ├── nav "Plugins" ──→ Plugins
    │
Search ──nav "Library"──→ Library
```

- **Top nav bar** is always visible (except in Reader, where it's hidden).
- **Esc** in Reader returns to Manga Detail.
- **Browser-style back** works via hash history.

---

## 3. Layout & Component Hierarchy

### 3.1 Global Shell

Every view lives inside a global shell. The Reader replaces the shell content entirely (fullscreen mode).

```html
<body>
  <!-- Top Navigation Bar (hidden in Reader) -->
  <nav id="top-nav" class="top-nav">
    <div class="nav-brand">goIsekai</div>
    <div class="nav-links">
      <a href="#/library" class="nav-link" data-view="library">Library</a>
      <a href="#/search" class="nav-link" data-view="search">Search</a>
      <a href="#/plugins" class="nav-link" data-view="plugins">Plugins</a>
    </div>
  </nav>

  <!-- Main Content Area -->
  <main id="app-content">
    <section id="library-view" class="view">...</section>
    <section id="search-view" class="view">...</section>
    <section id="manga-view" class="view">...</section>
    <section id="reader-view" class="view">...</section>
    <section id="plugins-view" class="view">...</section>
  </main>
</body>
```

### 3.2 Library View

```
┌─────────────────────────────────────────────────┐
│  [Top Nav]                                       │
├─────────────────────────────────────────────────┤
│  <section id="library-view" class="view">        │
│    <div class="view-header">                     │
│      <h1>Library</h1>                            │
│    </div>                                        │
│    <div id="library-grid" class="manga-grid">    │
│      <!-- .manga-card repeated per item -->       │
│    </div>                                        │
│    <!-- Empty state -->                           │
│    <div id="library-empty" class="empty-state">  │
│      <div class="empty-icon">📚</div>            │
│      <h2>Your library is empty</h2>              │
│      <p>Search for manga and add them here.</p>  │
│      <a href="#/search" class="btn btn-primary"> │
│        Browse Sources                            │
│      </a>                                        │
│    </div>                                        │
│  </section>                                      │
└─────────────────────────────────────────────────┘
```

**Manga Card** (`.manga-card`):
```html
<a href="#/manga/{pluginID}/{sourceMangaID}" class="manga-card">
  <div class="manga-card-cover">
    <img src="" alt="" loading="lazy" />
    <div class="manga-card-overlay">
      <span class="manga-card-status">{status}</span>
    </div>
  </div>
  <div class="manga-card-info">
    <h3 class="manga-card-title">{title}</h3>
    <span class="manga-card-source">{pluginName}</span>
  </div>
</a>
```

### 3.3 Search View

```
┌─────────────────────────────────────────────────┐
│  [Top Nav]                                       │
├─────────────────────────────────────────────────┤
│  <section id="search-view" class="view">         │
│    <div class="search-controls">                 │
│      <select id="plugin-select" class="select">  │
│        <!-- <option> per installed plugin -->     │
│      </select>                                   │
│      <div class="search-input-wrap">             │
│        <input id="search-input" type="text"      │
│               placeholder="Search manga..."      │
│               class="input" />                   │
│        <button id="search-btn" class="btn btn-   │
│                primary">Search</button>           │
│      </div>                                      │
│    </div>                                        │
│    <div id="search-results" class="manga-grid">  │
│      <!-- .manga-card repeated per result -->     │
│    </div>                                        │
│    <div id="search-empty" class="empty-state">   │
│      <div class="empty-icon">🔍</div>            │
│      <h2>No results found</h2>                   │
│      <p>Try a different search term or source.</p>│
│    </div>                                        │
│    <div id="search-loading" class="loading-state">│
│      <div class="spinner"></div>                 │
│      <p>Searching...</p>                         │
│    </div>                                        │
│    <!-- Pagination -->                            │
│    <div id="search-pagination" class="pagination">│
│      <button id="prev-page" class="btn">← Prev</button>│
│      <span id="page-info">Page 1</span>          │
│      <button id="next-page" class="btn">Next →</button>│
│    </div>                                        │
│  </section>                                      │
└─────────────────────────────────────────────────┘
```

### 3.4 Manga Detail View

```
┌─────────────────────────────────────────────────┐
│  [Top Nav]                                       │
├─────────────────────────────────────────────────┤
│  <section id="manga-view" class="view">          │
│    <div class="manga-detail">                    │
│      <div class="manga-detail-header">           │
│        <div class="manga-detail-cover">          │
│          <img id="manga-cover" src="" alt="" />   │
│        </div>                                    │
│        <div class="manga-detail-meta">           │
│          <h1 id="manga-title"></h1>              │
│          <div class="manga-detail-badges">       │
│            <span id="manga-status" class="badge"></span>│
│            <span id="manga-author" class="badge"></span>│
│          </div>                                  │
│          <p id="manga-description"></p>           │
│          <div class="manga-detail-actions">      │
│            <button id="library-toggle" class="btn btn-primary">│
│              Add to Library                      │
│            </button>                             │
│          </div>                                  │
│        </div>                                    │
│      </div>                                      │
│      <div class="manga-detail-chapters">         │
│        <h2>Chapters</h2>                         │
│        <div id="chapter-list" class="chapter-list">│
│          <!-- .chapter-row repeated -->           │
│        </div>                                    │
│      </div>                                      │
│    </div>                                        │
│    <!-- Loading -->                               │
│    <div id="manga-loading" class="loading-state">│
│      <div class="spinner"></div>                 │
│      <p>Loading manga details...</p>             │
│    </div>                                        │
│  </section>                                      │
└─────────────────────────────────────────────────┘
```

**Chapter Row** (`.chapter-row`):
```html
<a href="#/read/{pluginID}/{mangaID}/{chapterID}" class="chapter-row">
  <span class="chapter-num">Ch. {chapterNum}</span>
  <span class="chapter-title">{title}</span>
  <span class="chapter-date">{releasedAt formatted}</span>
  <span class="chapter-progress">
    <!-- if read: ✓ checkmark; if partial: progress bar -->
  </span>
</a>
```

### 3.5 Reader View

```
┌─────────────────────────────────────────────────┐
│  <section id="reader-view" class="view reader">  │
│    <!-- Top bar (auto-hide, show on hover/move) -->│
│    <div id="reader-topbar" class="reader-topbar">│
│      <button id="reader-back" class="btn btn-ghost">│
│        ← Back                                    │
│      </button>                                   │
│      <span id="reader-title" class="reader-title">│
│        {manga title} — Ch. {num}                 │
│      </span>                                     │
│      <span id="reader-page-info" class="reader-page-info">│
│        1 / 20                                    │
│      </span>                                     │
│    </div>                                        │
│                                                  │
│    <!-- Image container (centered, scrollable) -->│
│    <div id="reader-image-wrap" class="reader-image-wrap">│
│      <img id="reader-image" src="" alt="" />      │
│    </div>                                        │
│                                                  │
│    <!-- Click zones for prev/next -->             │
│    <div id="reader-zone-prev" class="reader-zone reader-zone-prev">│
│    </div>                                        │
│    <div id="reader-zone-next" class="reader-zone reader-zone-next">│
│    </div>                                        │
│                                                  │
│    <!-- Bottom bar (page slider) -->              │
│    <div id="reader-bottombar" class="reader-bottombar">│
│      <input id="reader-slider" type="range"      │
│             min="0" max="0" value="0"            │
│             class="reader-slider" />             │
│    </div>                                        │
│                                                  │
│    <!-- Loading overlay -->                       │
│    <div id="reader-loading" class="reader-loading">│
│      <div class="spinner"></div>                 │
│    </div>                                        │
│  </section>                                      │
└─────────────────────────────────────────────────┘
```

### 3.6 Plugins View

```
┌─────────────────────────────────────────────────┐
│  [Top Nav]                                       │
├─────────────────────────────────────────────────┤
│  <section id="plugins-view" class="view">        │
│    <div class="view-header">                     │
│      <h1>Plugins</h1>                            │
│      <button id="install-plugin-btn" class="btn btn-primary">│
│        Install Plugin                            │
│      </button>                                   │
│    </div>                                        │
│    <div id="plugin-list" class="plugin-list">    │
│      <!-- .plugin-card repeated -->              │
│    </div>                                        │
│    <div id="plugins-empty" class="empty-state">  │
│      <div class="empty-icon">🧩</div>            │
│      <h2>No plugins installed</h2>               │
│      <p>Install a .wasm plugin to start reading.</p>│
│    </div>                                        │
│  </section>                                      │
└─────────────────────────────────────────────────┘
```

**Plugin Card** (`.plugin-card`):
```html
<div class="plugin-card">
  <div class="plugin-card-icon">
    <img src="{iconURL}" alt="" />
    <!-- fallback: first letter of name in a colored circle -->
  </div>
  <div class="plugin-card-info">
    <h3>{name}</h3>
    <span class="plugin-card-id">{id}</span>
    <span class="plugin-card-version">{version}</span>
  </div>
  <div class="plugin-card-status">
    <span class="badge badge-active">Active</span>
  </div>
</div>
```

---

## 4. Visual Design System

### 4.1 Color Palette (Dark Theme)

| Token | Hex | Usage |
|-------|-----|-------|
| `--bg-primary` | `#0f0f13` | Page background |
| `--bg-secondary` | `#1a1a24` | Card backgrounds, nav bar |
| `--bg-tertiary` | `#24243a` | Hover states, input backgrounds |
| `--bg-elevated` | `#2a2a45` | Modals, dropdowns, tooltips |
| `--text-primary` | `#e8e8f0` | Main text |
| `--text-secondary` | `#9898b0` | Secondary text, metadata |
| `--text-muted` | `#5a5a78` | Disabled text, placeholders |
| `--accent` | `#7c5cfc` | Primary buttons, links, active states |
| `--accent-hover` | `#9478ff` | Hover state for accent elements |
| `--accent-subtle` | `rgba(124, 92, 252, 0.12)` | Accent backgrounds, badges |
| `--success` | `#4ade80` | Read status, active badges |
| `--warning` | `#fbbf24` | Partial progress |
| `--error` | `#f87171` | Error states, failed loads |
| `--border` | `#2a2a3e` | Card borders, dividers |
| `--shadow` | `0 4px 24px rgba(0, 0, 0, 0.4)` | Card shadows |
| `--shadow-sm` | `0 2px 8px rgba(0, 0, 0, 0.3)` | Subtle shadows |

### 4.2 Typography

**Font Stack**:
```css
--font-display: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
--font-body: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
--font-mono: 'JetBrains Mono', 'Fira Code', monospace;
```

**Load from Google Fonts** (in `index.html` `<head>`):
```html
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=JetBrains+Mono:wght@400&display=swap" rel="stylesheet">
```

**Type Scale**:

| Token | Size | Weight | Line Height | Usage |
|-------|------|--------|-------------|-------|
| `--text-display` | 2rem (32px) | 700 | 1.2 | Page titles (h1) |
| `--text-heading` | 1.5rem (24px) | 600 | 1.3 | Section headings (h2) |
| `--text-subheading` | 1.125rem (18px) | 600 | 1.4 | Card titles, manga title |
| `--text-body` | 0.9375rem (15px) | 400 | 1.6 | Body text, descriptions |
| `--text-small` | 0.8125rem (13px) | 400 | 1.5 | Metadata, timestamps |
| `--text-caption` | 0.75rem (12px) | 500 | 1.4 | Badges, labels, page numbers |

### 4.3 Spacing Scale

| Token | Value | Usage |
|-------|-------|-------|
| `--space-xs` | 4px | Tight gaps (badge padding) |
| `--space-sm` | 8px | Inner card padding, icon gaps |
| `--space-md` | 16px | Card padding, section gaps |
| `--space-lg` | 24px | Section margins, grid gaps |
| `--space-xl` | 32px | Page margins, large gaps |
| `--space-2xl` | 48px | Top/bottom page padding |

### 4.4 Border Radius

| Token | Value | Usage |
|-------|-------|-------|
| `--radius-sm` | 4px | Badges, small elements |
| `--radius-md` | 8px | Cards, inputs, buttons |
| `--radius-lg` | 12px | Large cards, modals |
| `--radius-full` | 9999px | Avatars, circular elements |

### 4.5 Shadows

```css
--shadow-sm: 0 2px 8px rgba(0, 0, 0, 0.3);
--shadow-md: 0 4px 24px rgba(0, 0, 0, 0.4);
--shadow-lg: 0 8px 48px rgba(0, 0, 0, 0.5);
--shadow-glow: 0 0 20px rgba(124, 92, 252, 0.3);
```

### 4.6 Component Styles

**Buttons**:
```css
.btn {
  display: inline-flex;
  align-items: center;
  gap: var(--space-sm);
  padding: 10px 20px;
  border: none;
  border-radius: var(--radius-md);
  font-family: var(--font-body);
  font-size: var(--text-body);
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-primary {
  background: var(--accent);
  color: white;
}

.btn-primary:hover {
  background: var(--accent-hover);
  box-shadow: var(--shadow-glow);
}

.btn-ghost {
  background: transparent;
  color: var(--text-primary);
  border: 1px solid var(--border);
}

.btn-ghost:hover {
  background: var(--bg-tertiary);
  border-color: var(--text-muted);
}
```

**Inputs**:
```css
.input, .select {
  padding: 10px 16px;
  background: var(--bg-tertiary);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  color: var(--text-primary);
  font-family: var(--font-body);
  font-size: var(--text-body);
  transition: border-color 0.2s ease;
}

.input:focus, .select:focus {
  outline: none;
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-subtle);
}

.input::placeholder {
  color: var(--text-muted);
}
```

**Cards**:
```css
.manga-card {
  display: flex;
  flex-direction: column;
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  overflow: hidden;
  transition: transform 0.2s ease, box-shadow 0.2s ease;
  text-decoration: none;
  color: inherit;
}

.manga-card:hover {
  transform: translateY(-4px);
  box-shadow: var(--shadow-md);
  border-color: var(--accent);
}
```

**Badges**:
```css
.badge {
  display: inline-flex;
  align-items: center;
  padding: 4px 10px;
  border-radius: var(--radius-full);
  font-size: var(--text-caption);
  font-weight: 500;
  background: var(--accent-subtle);
  color: var(--accent);
}

.badge-active {
  background: rgba(74, 222, 128, 0.15);
  color: var(--success);
}
```

**Spinner**:
```css
.spinner {
  width: 40px;
  height: 40px;
  border: 3px solid var(--border);
  border-top-color: var(--accent);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}
```

### 4.7 States

**Loading State**:
```html
<div class="loading-state">
  <div class="spinner"></div>
  <p>Loading...</p>
</div>
```
```css
.loading-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: var(--space-md);
  padding: var(--space-2xl);
  color: var(--text-secondary);
}
```

**Empty State**:
```html
<div class="empty-state">
  <div class="empty-icon">📚</div>
  <h2>Title</h2>
  <p>Description text</p>
  <a href="#/..." class="btn btn-primary">Action</a>
</div>
```
```css
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: var(--space-md);
  padding: var(--space-2xl);
  text-align: center;
}

.empty-icon {
  font-size: 4rem;
  margin-bottom: var(--space-md);
}

.empty-state h2 {
  color: var(--text-primary);
  font-size: var(--text-heading);
}

.empty-state p {
  color: var(--text-secondary);
  max-width: 400px;
}
```

**Error State** (inline, for failed image loads or API errors):
```html
<div class="error-state">
  <div class="error-icon">⚠️</div>
  <p>Failed to load image</p>
  <button class="btn btn-ghost">Retry</button>
</div>
```
```css
.error-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: var(--space-sm);
  padding: var(--space-lg);
  color: var(--error);
}
```

---

## 5. Interaction Spec

### 5.1 Library View

| Element | Event | Action | Binding Call |
|---------|-------|--------|--------------|
| `.manga-card` | click | Navigate to `#/manga/{pluginID}/{sourceMangaID}` | — |
| `.manga-card` | hover | Lift card (translateY -4px), glow border | CSS only |
| Nav link "Library" | click | Navigate to `#/library` | — |
| Empty state "Browse Sources" | click | Navigate to `#/search` | — |

**Keyboard**: None specific (global nav shortcuts below).

### 5.2 Search View

| Element | Event | Action | Binding Call |
|---------|-------|--------|--------------|
| `#plugin-select` | change | Store selected plugin ID | — |
| `#search-input` | Enter key | Trigger search | `SearchManga(pluginID, {query, page: 1})` |
| `#search-btn` | click | Trigger search | `SearchManga(pluginID, {query, page: 1})` |
| `.manga-card` | click | Navigate to `#/manga/{pluginID}/{mangaID}` | — |
| `#prev-page` | click | Navigate to `#/search?plugin={id}&q={q}&page={n-1}` | `SearchManga(pluginID, {query, page: n-1})` |
| `#next-page` | click | Navigate to `#/search?plugin={id}&q={q}&page={n+1}` | `SearchManga(pluginID, {query, page: n+1})` |

**Keyboard**:
- `/` (when not in input): Focus search input
- `Enter` (in search input): Submit search

### 5.3 Manga Detail View

| Element | Event | Action | Binding Call |
|---------|-------|--------|--------------|
| `#library-toggle` | click | Toggle library status, update button text | `ToggleLibraryItem(pluginID, mangaID)` |
| `.chapter-row` | click | Navigate to `#/read/{pluginID}/{mangaID}/{chapterID}` | — |
| `.chapter-row` | hover | Highlight row background | CSS only |

**Button Text Logic**:
- If manga is in library: "In Library ✓" (green accent)
- If not: "Add to Library" (primary accent)

**Keyboard**:
- `Esc`: Navigate back (to previous hash or `#/library`)

### 5.4 Reader View

| Element | Event | Action | Binding Call |
|---------|-------|--------|--------------|
| `#reader-back` | click | Navigate to manga detail | — |
| `#reader-zone-prev` | click | Go to previous page | `RecordRead(...)` on page change |
| `#reader-zone-next` | click | Go to next page | `RecordRead(...)` on page change |
| `#reader-slider` | input | Jump to page | `RecordRead(...)` |
| `#reader-image` | load | Hide loading overlay | — |
| `#reader-image` | error | Show error state with retry | — |

**Keyboard** (global, only when Reader is active):
- `←` or `A`: Previous page
- `→` or `D`: Next page
- `Esc`: Exit reader → navigate to manga detail
- `Home`: First page
- `End`: Last page
- `F`: Toggle fullscreen (use Wails fullscreen API if available, otherwise CSS)

**Auto-hide Top/Bottom Bars**:
- Bars appear on mouse movement
- Bars hide after 3 seconds of inactivity
- Bars always visible when hovering over them

**Progress Persistence**:
- On every page change, call `RecordRead(pluginID, mangaID, chapterID, pageNum)`
- On reaching the last page, call `SetChapterProgress(pluginID, mangaID, chapterID, lastPage)`
- On exiting reader, call `SetChapterProgress(pluginID, mangaID, chapterID, currentPage)`

### 5.5 Plugins View

| Element | Event | Action | Binding Call |
|---------|-------|--------|--------------|
| `#install-plugin-btn` | click | Open file picker (accept `.wasm`) | `InstallPlugin(wasmPath)` |
| `.plugin-card` | hover | Subtle lift | CSS only |

**File Picker**:
```js
document.getElementById('install-plugin-btn').addEventListener('click', async () => {
  // Wails v3 doesn't expose a native file dialog by default in plain JS.
  // Use an <input type="file"> hidden element:
  const input = document.createElement('input');
  input.type = 'file';
  input.accept = '.wasm';
  input.onchange = async (e) => {
    const file = e.target.files[0];
    if (!file) return;
    // Wails v3 bindings: the path is the file's absolute path
    // In Wails v3, we need to use the runtime to get the file path
    // For now, show a loading state and call InstallPlugin
    try {
      await goisekai.bridge.AppService.InstallPlugin(file.name);
      loadPlugins(); // Refresh list
    } catch (err) {
      showError('Failed to install plugin: ' + err);
    }
  };
  input.click();
});
```

**Note**: Wails v3 file dialog integration may require using `window.runtime` or a dedicated dialog API. The fixer should check Wails v3 docs for the correct file-open dialog pattern. If unavailable, use the `<input type="file">` approach above.

### 5.6 Global Keyboard Shortcuts

| Shortcut | Action | Context |
|----------|--------|---------|
| `Ctrl+1` | Navigate to Library | Global |
| `Ctrl+2` | Navigate to Search | Global |
| `Ctrl+3` | Navigate to Plugins | Global |
| `/` | Focus search input | When not in Reader |
| `Esc` | Back / Exit reader | Context-dependent |

---

## 6. Data Flow

### 6.1 Wails v3 Binding Call Pattern

In Wails v3, bound Go methods are available on the `goisekai` global object. The exact namespace depends on how `application.NewService` exposes it. The fixer should verify the exact path, but it typically follows:

```js
// Pattern: goisekai.<packageName>.<ServiceName>.<MethodName>(...args)
// Returns a Promise.

// Example:
const manga = await goisekai.bridge.AppService.SearchManga(pluginID, filter);
```

**If the above doesn't work**, check `window['goisekai']` in the console. The fixer must verify the exact binding path at runtime and adjust all calls accordingly.

### 6.2 Image Display via GetImage

`GetImage(pluginID, url, headers)` returns raw bytes (`[]byte`). In Wails v3, this arrives as a `Uint8Array` in JS. Convert to a blob URL:

```js
async function loadImage(pluginID, url, headers = {}) {
  try {
    const bytes = await goisekai.bridge.AppService.GetImage(pluginID, url, headers);
    // bytes is Uint8Array
    const blob = new Blob([bytes], { type: 'image/jpeg' }); // assume JPEG; detect from response if possible
    const blobUrl = URL.createObjectURL(blob);
    return blobUrl;
  } catch (err) {
    console.error('Failed to load image:', url, err);
    return null;
  }
}

// Usage:
const imgEl = document.getElementById('manga-cover');
const url = await loadImage(pluginID, coverURL);
if (url) {
  imgEl.src = url;
} else {
  imgEl.parentElement.classList.add('error');
}
```

**Important**: Revoke blob URLs when no longer needed to avoid memory leaks:
```js
imgEl.addEventListener('load', () => {
  // Keep the blob URL alive while the image is displayed.
  // Revoke when navigating away or replacing the src.
});
// On view change:
URL.revokeObjectURL(oldBlobUrl);
```

### 6.3 Per-View Data Flow

#### Library View
```
1. Call: ListLibrary()
2. Returns: []database.Manga (has ID, PluginID, SourceMangaID, Title, CoverURL, Status)
3. For each manga:
   a. Call: loadImage(manga.PluginID, manga.CoverURL)
   b. Render card with cover, title, status badge
4. If empty: show empty state
```

#### Search View
```
1. On load: Call ListPlugins() to populate #plugin-select dropdown
2. On search submit:
   a. Show loading state
   b. Call: SearchManga(selectedPluginID, {query, page: currentPage})
   c. Returns: []types.Manga (has ID, Title, CoverURL, Author, Status, Genres)
   d. For each result:
      - Call: loadImage(pluginID, manga.CoverURL)
      - Render card
   e. If empty: show empty state
   f. Hide loading state
3. Pagination: update URL hash, re-call SearchManga with new page
```

#### Manga Detail View
```
1. Extract pluginID, mangaID from URL hash
2. Show loading state
3. Call: GetMangaDetails(pluginID, mangaID)
4. Returns: (types.Manga, []types.Chapter)
5. Render:
   a. Call: loadImage(pluginID, manga.CoverURL) for cover
   b. Set title, status, author, description
   c. Check if in library: call ListLibrary(), find by pluginID+sourceMangaID
   d. Set library toggle button state
   e. Render chapter list (sorted by ChapterNum descending — newest first)
6. Hide loading state
```

#### Reader View
```
1. Extract pluginID, mangaID, chapterID, page from URL hash
2. Show loading overlay
3. Call: GetPageList(pluginID, chapterID)
4. Returns: []types.Page (has Index, URL, Headers)
5. Set slider max to pages.length - 1
6. Load current page:
   a. Call: loadImage(pluginID, pages[currentPage].URL, pages[currentPage].Headers)
   b. Set #reader-image src
   c. Hide loading overlay
7. Preload adjacent pages (±1) in background
8. On page change:
   a. Update slider, page info text
   b. Load new page image
   c. Call: RecordRead(pluginID, mangaID, chapterID, newPageNum)
9. On exit:
   a. Call: SetChapterProgress(pluginID, mangaID, chapterID, currentPage)
   b. Revoke all blob URLs
```

#### Plugins View
```
1. Call: ListPlugins()
2. Returns: []database.Plugin (has ID, Name, Version, WasmPath, IsActive, IconURL)
3. Render plugin cards
4. If empty: show empty state
```

### 6.4 Error Handling Pattern

```js
async function safeCall(fn, ...args) {
  try {
    return await fn(...args);
  } catch (err) {
    console.error('API call failed:', err);
    throw err; // Re-throw for caller to handle
  }
}

// Usage in view loaders:
async function loadLibrary() {
  const grid = document.getElementById('library-grid');
  const empty = document.getElementById('library-empty');
  
  try {
    grid.innerHTML = ''; // Clear
    empty.style.display = 'none';
    
    const library = await goisekai.bridge.AppService.ListLibrary();
    
    if (library.length === 0) {
      empty.style.display = 'flex';
      return;
    }
    
    for (const manga of library) {
      const card = await createMangaCard(manga.PluginID, manga.SourceMangaID, 
                                          manga.Title, manga.CoverURL, manga.Status);
      grid.appendChild(card);
    }
  } catch (err) {
    grid.innerHTML = `<div class="error-state">
      <div class="error-icon">⚠️</div>
      <p>Failed to load library</p>
      <button class="btn btn-ghost" onclick="loadLibrary()">Retry</button>
    </div>`;
  }
}
```

---

## 7. File Structure

Create the following files under `cmd/goisekai/frontend/`:

```
cmd/goisekai/frontend/
├── index.html          # Main HTML shell, loads CSS and JS
├── styles.css          # All CSS (design system + component styles)
└── app.js              # All JS (router, data loading, rendering, interactions)
```

### 7.1 index.html

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>goIsekai</title>
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=JetBrains+Mono:wght@400&display=swap" rel="stylesheet">
  <link rel="stylesheet" href="styles.css" />
</head>
<body>
  <!-- Top Navigation Bar -->
  <nav id="top-nav" class="top-nav">
    <div class="nav-brand">
      <span class="brand-icon">📖</span>
      <span class="brand-text">goIsekai</span>
    </div>
    <div class="nav-links">
      <a href="#/library" class="nav-link" data-view="library">Library</a>
      <a href="#/search" class="nav-link" data-view="search">Search</a>
      <a href="#/plugins" class="nav-link" data-view="plugins">Plugins</a>
    </div>
  </nav>

  <!-- Main Content Area -->
  <main id="app-content">
    <!-- Library View -->
    <section id="library-view" class="view">
      <!-- ... as described in §3.2 ... -->
    </section>

    <!-- Search View -->
    <section id="search-view" class="view">
      <!-- ... as described in §3.3 ... -->
    </section>

    <!-- Manga Detail View -->
    <section id="manga-view" class="view">
      <!-- ... as described in §3.4 ... -->
    </section>

    <!-- Reader View -->
    <section id="reader-view" class="view">
      <!-- ... as described in §3.5 ... -->
    </section>

    <!-- Plugins View -->
    <section id="plugins-view" class="view">
      <!-- ... as described in §3.6 ... -->
    </section>
  </main>

  <!-- Hidden file input for plugin install -->
  <input type="file" id="plugin-file-input" accept=".wasm" style="display: none;" />

  <script src="app.js"></script>
</body>
</html>
```

### 7.2 styles.css

Structure:
```css
/* 1. CSS Custom Properties (design tokens) */
:root { /* all --bg-*, --text-*, --accent-*, --space-*, --radius-*, --shadow-* */ }

/* 2. Reset & Base */
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
body { /* font, bg, color */ }

/* 3. Layout: Top Nav */
.top-nav { /* ... */ }
.nav-brand { /* ... */ }
.nav-links { /* ... */ }
.nav-link { /* ... */ }
.nav-link.active { /* accent underline */ }

/* 4. Layout: Views */
.view { display: none; }
.view.active { display: block; }

/* 5. Components: Manga Grid */
.manga-grid { /* CSS Grid, responsive */ }
.manga-card { /* ... */ }
.manga-card-cover { /* ... */ }
.manga-card-info { /* ... */ }

/* 6. Components: Search Controls */
.search-controls { /* ... */ }
.search-input-wrap { /* ... */ }

/* 7. Components: Manga Detail */
.manga-detail { /* ... */ }
.manga-detail-header { /* flex row */ }
.manga-detail-cover { /* fixed width */ }
.manga-detail-meta { /* flex column */ }
.chapter-list { /* ... */ }
.chapter-row { /* ... */ }

/* 8. Components: Reader */
.reader { /* fullscreen */ }
.reader-topbar { /* fixed top, auto-hide */ }
.reader-image-wrap { /* centered, contain */ }
.reader-zone { /* click targets */ }
.reader-bottombar { /* fixed bottom */ }
.reader-slider { /* custom range input */ }

/* 9. Components: Plugins */
.plugin-list { /* ... */ }
.plugin-card { /* ... */ }

/* 10. Components: Shared */
.btn { /* ... */ }
.btn-primary { /* ... */ }
.btn-ghost { /* ... */ }
.input { /* ... */ }
.select { /* ... */ }
.badge { /* ... */ }
.spinner { /* ... */ }
.loading-state { /* ... */ }
.empty-state { /* ... */ }
.error-state { /* ... */ }

/* 11. Animations */
@keyframes spin { /* ... */ }
@keyframes fadeIn { /* ... */ }
@keyframes slideUp { /* ... */ }
```

### 7.3 app.js

Structure:
```js
// 1. Constants & State
const state = { /* currentView, pluginID, mangaID, chapterID, pages, currentPage, ... */ };

// 2. Router
function router() { /* hash-based routing */ }
window.addEventListener('hashchange', router);
window.addEventListener('DOMContentLoaded', router);

// 3. View Loaders
async function loadLibrary() { /* ... */ }
async function loadSearch(pluginID, query, page) { /* ... */ }
async function loadMangaDetail(pluginID, mangaID) { /* ... */ }
async function loadReader(pluginID, mangaID, chapterID, page) { /* ... */ }
async function loadPlugins() { /* ... */ }

// 4. Rendering Helpers
async function createMangaCard(pluginID, mangaID, title, coverURL, status) { /* ... */ }
async function createChapterRow(pluginID, mangaID, chapter) { /* ... */ }
async function createPluginCard(plugin) { /* ... */ }

// 5. Image Loading
async function loadImage(pluginID, url, headers) { /* blob URL pattern */ }
function revokeBlobUrl(url) { /* cleanup */ }

// 6. Reader Logic
function setupReader() { /* keyboard, click zones, slider, auto-hide bars */ }
function goToPage(pageNum) { /* ... */ }
function preloadAdjacentPages() { /* ... */ }

// 7. UI Helpers
function showView(viewID) { /* toggle .active */ }
function showLoading(elementID) { /* ... */ }
function hideLoading(elementID) { /* ... */ }
function showError(elementID, message) { /* ... */ }

// 8. Event Listeners
document.getElementById('search-btn').addEventListener('click', /* ... */);
document.getElementById('search-input').addEventListener('keydown', /* ... */);
document.getElementById('library-toggle').addEventListener('click', /* ... */);
document.getElementById('install-plugin-btn').addEventListener('click', /* ... */);
// ... etc.

// 9. Global Keyboard Shortcuts
document.addEventListener('keydown', (e) => { /* ... */ });
```

---

## 8. Edge Cases

### 8.1 Empty Library
- **Trigger**: `ListLibrary()` returns `[]`
- **Display**: Show `#library-empty` with icon, message, and "Browse Sources" button
- **Hide**: `#library-grid`

### 8.2 No Plugins Installed
- **Trigger**: `ListPlugins()` returns `[]`
- **Display**: Show `#plugins-empty` with icon, message
- **Search View**: Disable search button, show message "Install a plugin first"
- **Library View**: Show empty state with "Install a plugin to get started"

### 8.3 Search with No Results
- **Trigger**: `SearchManga()` returns `[]`
- **Display**: Show `#search-empty` with icon, message
- **Hide**: `#search-results`, `#search-pagination`

### 8.4 Image Load Failure
- **Trigger**: `GetImage()` throws or returns error
- **Display**: 
  - For covers: Show placeholder (gray box with manga title initials)
  - For reader: Show error overlay with "Failed to load page" and "Retry" button
- **Retry**: Re-call `loadImage()` for that specific URL

### 8.5 Chapter with Zero Pages
- **Trigger**: `GetPageList()` returns `[]`
- **Display**: Show message "This chapter has no pages" with "Back" button
- **Disable**: Navigation controls

### 8.6 Network/API Errors
- **Pattern**: Wrap all `goisekai.bridge.AppService.*` calls in try/catch
- **Display**: Show inline error message with "Retry" button
- **Log**: Console.error for debugging

### 8.7 Long Loading Times
- **Pattern**: Show loading spinner immediately on view switch
- **Timeout**: After 10 seconds, show "Still loading..." message
- **Cancel**: On view change, abort pending requests (use AbortController if supported, otherwise ignore stale responses)

### 8.8 Manga Not in Library (Detail View)
- **Trigger**: User opens manga detail for manga not yet in DB
- **Behavior**: `GetMangaDetails()` persists the manga as a side effect (in_library=0)
- **Display**: Library toggle shows "Add to Library"

### 8.9 Reader: Last Page Reached
- **Trigger**: `currentPage === pages.length - 1`
- **Behavior**: Call `SetChapterProgress(pluginID, mangaID, chapterID, lastPage)`
- **Display**: Show "Chapter Complete" overlay with options:
  - "Next Chapter" (if available)
  - "Back to Manga"

### 8.10 Blob URL Memory Management
- **Pattern**: Track all created blob URLs in an array
- **Cleanup**: On view change, revoke all tracked URLs
- **Limit**: If more than 50 blob URLs exist, revoke oldest first

---

## 9. Implementation Notes for Fixer

### 9.1 Wails v3 Binding Discovery

The exact binding path must be verified at runtime. Add this to `app.js` initialization:

```js
// Debug: log available bindings
console.log('Wails bindings:', window.goisekai);
console.log('AppService methods:', window.goisekai?.bridge?.AppService);
```

If `goisekai.bridge.AppService` doesn't exist, check:
- `window.runtime` (Wails v2 pattern)
- `window['goisekai']` (namespace)
- Browser console for available globals

### 9.2 CSS Grid for Manga Grid

```css
.manga-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: var(--space-lg);
  padding: var(--space-lg);
}
```

### 9.3 Reader Image Sizing

```css
.reader-image-wrap {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: calc(100vh - 120px); /* minus top/bottom bars */
  overflow: auto;
}

.reader-image-wrap img {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
}
```

### 9.4 Auto-hide Reader Bars

```js
let hideTimeout;
const topbar = document.getElementById('reader-topbar');
const bottombar = document.getElementById('reader-bottombar');

function showBars() {
  topbar.classList.add('visible');
  bottombar.classList.add('visible');
  clearTimeout(hideTimeout);
  hideTimeout = setTimeout(hideBars, 3000);
}

function hideBars() {
  topbar.classList.remove('visible');
  bottombar.classList.remove('visible');
}

document.addEventListener('mousemove', showBars);
topbar.addEventListener('mouseenter', () => clearTimeout(hideTimeout));
bottombar.addEventListener('mouseenter', () => clearTimeout(hideTimeout));
```

```css
.reader-topbar, .reader-bottombar {
  position: fixed;
  left: 0;
  right: 0;
  opacity: 0;
  transition: opacity 0.3s ease;
  pointer-events: none;
  z-index: 100;
}

.reader-topbar.visible, .reader-bottombar.visible {
  opacity: 1;
  pointer-events: auto;
}

.reader-topbar { top: 0; }
.reader-bottombar { bottom: 0; }
```

### 9.5 Date Formatting

```js
function formatDate(dateStr) {
  if (!dateStr) return '';
  const date = new Date(dateStr);
  return date.toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric'
  });
}
```

### 9.6 Chapter Sorting

```js
// Sort chapters by chapter number, descending (newest first)
chapters.sort((a, b) => b.chapter_num - a.chapter_num);
```

---

## 10. Accessibility

- All interactive elements must be keyboard-focusable
- Use semantic HTML (`<nav>`, `<main>`, `<section>`, `<a>`, `<button>`)
- Add `aria-label` to icon-only buttons
- Ensure color contrast meets WCAG AA (4.5:1 for text)
- Add `alt` text to all images (use manga title)
- Focus management: on view change, focus the main heading

---

## 11. Performance Considerations

- **Lazy loading**: Use `loading="lazy"` on all manga cover images
- **Debounce search**: Wait 300ms after typing before enabling search
- **Preload reader pages**: Load ±1 page in background
- **Virtual scrolling**: Not needed for initial implementation; revisit if library exceeds 1000 items
- **Image caching**: Backend caches via `imageCache` map; frontend should cache blob URLs in a `Map<url, blobUrl>`

---

## 12. Summary

This spec defines a dark-themed, desktop manga reader with five screens (Library, Search, Manga Detail, Reader, Plugins) connected by a hash-based router. The design uses a purple accent (#7c5cfc) on a near-black background (#0f0f13) with Inter typography. The Reader is the centerpiece: full-bleed images with auto-hiding controls, keyboard navigation (←/→/Esc), a page slider, and automatic progress persistence via `RecordRead`/`SetChapterProgress`. All data flows through Wails v3 bindings to the Go `AppService`, with images converted from raw bytes to blob URLs for display. The spec provides exact hex values, spacing tokens, DOM structures, and interaction mappings so the fixer can implement without design decisions.

**Spec path**: `docs/ui-spec.md`
