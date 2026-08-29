// goIsekai frontend — vanilla JS, no build step / no framework.
// All Wails v3 service calls funnel through the single `bindings` object below
// (§6.1 / §9.1): the invocation shape lives in one place, so the bridge path can
// be verified / adjusted without touching view code.

'use strict';

/* ------------------------------------------------------------------ *
 * 1. Bindings — centralized Wails v3 bridge access (§6.1 / §9.1)
 * ------------------------------------------------------------------ */
// ponytail: Wails v3 exposes bound Go services on window.<appName>.<pkg>.<Service>.
// The spec documents goisekai.bridge.AppService; the exact namespace is resolved
// once here so a runtime mismatch is a one-line fix, not a file-wide search.
const AppService = (function () {
  const svc = window.goisekai && window.goisekai.bridge && window.goisekai.bridge.AppService;
  // Debug: log available bindings (§9.1)
  console.log('Wails bindings:', window.goisekai);
  console.log('AppService methods:', svc);
  return svc || null;
})();

const bindings = {
  search: (pluginID, filter) => call('SearchManga', pluginID, filter),
  mangaDetails: (pluginID, mangaID) => call('GetMangaDetails', pluginID, mangaID),
  pageList: (pluginID, chapterID) => call('GetPageList', pluginID, chapterID),
  toggleLibrary: (pluginID, mangaID) => call('ToggleLibraryItem', pluginID, mangaID),
  installPlugin: (wasmPath) => call('InstallPlugin', wasmPath),
  recordRead: (pluginID, mangaID, chapterID, pageNum) => call('RecordRead', pluginID, mangaID, chapterID, pageNum),
  setChapterProgress: (pluginID, mangaID, chapterID, lastPage) => call('SetChapterProgress', pluginID, mangaID, chapterID, lastPage),
  listLibrary: () => call('ListLibrary'),
  listPlugins: () => call('ListPlugins'),
};

// Thin wrapper: rejects on Go error return; callers handle via try/catch (§6.4).
function call(method, ...args) {
  if (!AppService) {
    return Promise.reject(new Error('Wails bridge not available (expected window.goisekai.bridge.AppService)'));
  }
  return AppService[method](...args);
}

/* ------------------------------------------------------------------ *
 * 2. State
 * ------------------------------------------------------------------ */
const state = {
  currentView: null,
  // reader
  reader: { pluginID: null, mangaID: null, chapterID: null, pages: [], currentPage: 0, title: '' },
  // search
  searchPlugin: '',
  searchQuery: '',
  searchPage: 1,
  // detail
  detail: { pluginID: null, mangaID: null, inLibrary: false },
  chapters: [],
  // library cache (reused by detail membership check + source labels)
  libraryList: null,
  // plugin registry (id -> name) for card source labels
  plugins: [],
  // local read tracking (chapterID -> { lastPage, pageCount })
  readProgress: {},
  // blob URL bookkeeping (§8.10): tracked list for revocation + url->blob cache
  blobUrls: [],
  blobCache: new Map(),
};

const $ = (sel) => document.querySelector(sel);

const clamp = (v, lo, hi) => Math.max(lo, Math.min(hi, v));

/* ------------------------------------------------------------------ *
 * 3. Router (§2.2)
 * ------------------------------------------------------------------ */
function router() {
  const hash = window.location.hash || '#/library';
  const [path, queryStr] = hash.split('?');
  const params = new URLSearchParams(queryStr || '');
  const segments = path.replace('#/', '').split('/');

  document.querySelectorAll('.view').forEach((v) => v.classList.remove('active'));
  document.getElementById('top-nav').style.display = '';

  switch (segments[0]) {
    case 'library':
    case '':
      showView('library-view');
      loadLibrary();
      break;
    case 'search':
      showView('search-view');
      loadSearch(params.get('plugin'), params.get('q'), parseInt(params.get('page') || '1', 10));
      break;
    case 'manga':
      showView('manga-view');
      loadMangaDetail(decodeURIComponent(segments[1]), decodeURIComponent(segments[2]));
      break;
    case 'read':
      showView('reader-view');
      document.getElementById('top-nav').style.display = 'none';
      loadReader(decodeURIComponent(segments[1]), decodeURIComponent(segments[2]),
        decodeURIComponent(segments[3]), parseInt(params.get('page') || '0', 10));
      break;
    case 'plugins':
      showView('plugins-view');
      loadPlugins();
      break;
    default:
      showView('library-view');
      loadLibrary();
  }
}
window.addEventListener('hashchange', router);
window.addEventListener('DOMContentLoaded', router);

function showView(viewID) {
  // Revoke blob URLs from the previous view to avoid leaks (§8.10).
  revokeAllBlobUrls();
  state.currentView = viewID;
  document.querySelectorAll('.view').forEach((v) => v.classList.remove('active'));
  const view = document.getElementById(viewID);
  if (view) view.classList.add('active');
  updateNavActive();
  // Focus management (§10): move focus to the view's primary heading.
  const heading = view && view.querySelector('h1, h2');
  if (heading) { heading.setAttribute('tabindex', '-1'); heading.focus({ preventScroll: true }); }
}

function updateNavActive() {
  const map = { 'library-view': 'library', 'search-view': 'search', 'plugins-view': 'plugins' };
  const link = document.querySelector(`.nav-link[data-view="${map[state.currentView]}"]`);
  document.querySelectorAll('.nav-link').forEach((l) => l.classList.toggle('active', l === link));
}

/* ------------------------------------------------------------------ *
 * 4. Rendering helpers
 * ------------------------------------------------------------------ */
async function createMangaCard(pluginID, mangaID, title, coverURL, status) {
  const a = document.createElement('a');
  a.className = 'manga-card';
  a.href = `#/manga/${encodeURIComponent(pluginID)}/${encodeURIComponent(mangaID)}`;

  const cover = document.createElement('div');
  cover.className = 'manga-card-cover';

  const img = document.createElement('img');
  img.alt = title || 'manga cover';
  img.loading = 'lazy';
  img.src = '';
  if (coverURL) {
    const url = await loadImage(pluginID, coverURL);
    if (url) img.src = url;
  }
  img.onerror = () => cover.classList.add('cover-error');
  cover.appendChild(img);

  // ponytail: placeholder for failed covers (§8.4) — initials on a gray box.
  const fallback = document.createElement('div');
  fallback.className = 'manga-card-fallback';
  fallback.textContent = getInitials(title);
  cover.appendChild(fallback);

  const overlay = document.createElement('div');
  overlay.className = 'manga-card-overlay';
  if (status) {
    const badge = document.createElement('span');
    badge.className = 'manga-card-status';
    badge.textContent = status;
    overlay.appendChild(badge);
  }
  cover.appendChild(overlay);

  const info = document.createElement('div');
  info.className = 'manga-card-info';
  const h3 = document.createElement('h3');
  h3.className = 'manga-card-title';
  h3.textContent = title || 'Untitled';
  const source = document.createElement('span');
  source.className = 'manga-card-source';
  source.textContent = lookupPluginName(pluginID) || pluginID;
  info.appendChild(h3);
  info.appendChild(source);

  a.appendChild(cover);
  a.appendChild(info);
  return a;
}

function createChapterRow(pluginID, mangaID, chapter) {
  const num = chapter.chapterNum || 0;
  const row = document.createElement('a');
  row.className = 'chapter-row';
  row.href = `#/read/${encodeURIComponent(pluginID)}/${encodeURIComponent(mangaID)}/${encodeURIComponent(chapter.id)}`;

  const numEl = document.createElement('span');
  numEl.className = 'chapter-num';
  numEl.textContent = `Ch. ${formatChapterNum(num)}`;

  const titleEl = document.createElement('span');
  titleEl.className = 'chapter-title';
  titleEl.textContent = chapter.title || `Chapter ${formatChapterNum(num)}`;

  const dateEl = document.createElement('span');
  dateEl.className = 'chapter-date';
  dateEl.textContent = formatDate(chapter.releasedAt);

  const progressEl = document.createElement('span');
  progressEl.className = 'chapter-progress';
  const prog = state.readProgress[chapter.id];
  if (prog) {
    const pct = Math.min(100, Math.round((prog.lastPage / Math.max(1, prog.pageCount)) * 100));
    if (prog.lastPage >= prog.pageCount) {
      progressEl.textContent = '✓ Read';
    } else {
      progressEl.innerHTML =
        '<span class="chapter-progress-bar"><span style="width:' + pct + '%"></span></span>' + pct + '%';
    }
  }

  row.appendChild(numEl);
  row.appendChild(titleEl);
  row.appendChild(dateEl);
  row.appendChild(progressEl);
  return row;
}

function createPluginCard(plugin) {
  const card = document.createElement('div');
  card.className = 'plugin-card';

  const icon = document.createElement('div');
  icon.className = 'plugin-card-icon';
  if (plugin.iconURL) {
    const img = document.createElement('img');
    img.src = plugin.iconURL;
    img.alt = (plugin.name || 'plugin') + ' icon';
    img.onerror = () => { icon.innerHTML = fallbackInitial(plugin.name); };
    icon.appendChild(img);
  } else {
    icon.innerHTML = fallbackInitial(plugin.name);
  }

  const info = document.createElement('div');
  info.className = 'plugin-card-info';
  const h3 = document.createElement('h3');
  h3.textContent = plugin.name || plugin.id;
  const idEl = document.createElement('span');
  idEl.className = 'plugin-card-id';
  idEl.textContent = plugin.id;
  const verEl = document.createElement('span');
  verEl.className = 'plugin-card-version';
  verEl.textContent = plugin.version || 'v0.0.0';
  info.appendChild(h3);
  info.appendChild(idEl);
  info.appendChild(verEl);

  const status = document.createElement('div');
  status.className = 'plugin-card-status';
  const badge = document.createElement('span');
  badge.className = 'badge badge-active';
  badge.textContent = plugin.isActive ? 'Active' : 'Inactive';
  status.appendChild(badge);

  card.appendChild(icon);
  card.appendChild(info);
  card.appendChild(status);
  return card;
}

/* ------------------------------------------------------------------ *
 * 5. Image loading (§6.2) — GetImage bytes → Blob URL
 * ------------------------------------------------------------------ */
function loadImage(pluginID, url, headers) {
  return makeBlobUrl(pluginID, url, headers);
}

function makeBlobUrl(pluginID, url, headers) {
  // ponytail: cache one Blob URL per source url (§11) so ±1 preloads and
  // revisits reuse bytes; revocation list bounds live memory (§8.10).
  return (async () => {
    if (!url) return null;
    const cached = state.blobCache.get(url);
    if (cached) return cached;
    try {
      const bytes = await call('GetImage', pluginID, url, headers || {});
      if (!bytes) return null;
      const blob = new Blob([bytes], { type: detectImageType(bytes) });
      const blobUrl = URL.createObjectURL(blob);
      state.blobCache.set(url, blobUrl);
      state.blobUrls.push(blobUrl);
      while (state.blobUrls.length > 50) {
        URL.revokeObjectURL(state.blobUrls.shift());
      }
      return blobUrl;
    } catch (err) {
      console.error('Failed to load image:', url, err);
      return null;
    }
  })();
}

// ponytail: sniff magic bytes instead of trusting a content-type GetImage strips.
function detectImageType(bytes) {
  if (!bytes || bytes.length < 4) return 'image/jpeg';
  if (bytes[0] === 0xFF && bytes[1] === 0xD8) return 'image/jpeg';
  if (bytes[0] === 0x89 && bytes[1] === 0x50 && bytes[2] === 0x4e && bytes[3] === 0x47) return 'image/png';
  if (bytes[0] === 0x47 && bytes[1] === 0x49 && bytes[2] === 0x46) return 'image/gif';
  if (bytes[0] === 0x52 && bytes[1] === 0x49 && bytes[2] === 0x46 && bytes[3] === 0x46) return 'image/webp';
  return 'image/jpeg';
}

function revokeBlobUrl(url) {
  if (url && state.blobUrls.indexOf(url) !== -1) {
    state.blobUrls = state.blobUrls.filter((u) => u !== url);
    URL.revokeObjectURL(url);
  }
}

function revokeAllBlobUrls() {
  for (const url of state.blobUrls.slice()) URL.revokeObjectURL(url);
  state.blobUrls = [];
  state.blobCache.clear();
}

/* ------------------------------------------------------------------ *
 * 6. View loaders
 * ------------------------------------------------------------------ */
async function loadLibrary() {
  const grid = $('#library-grid');
  const empty = $('#library-empty');
  grid.innerHTML = '';
  empty.style.display = 'none';
  try {
    state.libraryList = await bindings.listLibrary();
    if (!state.libraryList || state.libraryList.length === 0) {
      empty.style.display = 'flex';
      return;
    }
    for (const manga of state.libraryList) {
      // database.Manga marshals as PascalCase JSON keys (§contract).
      const card = await createMangaCard(
        manga.PluginID, manga.SourceMangaID, manga.Title, manga.CoverURL, manga.Status || ''
      );
      grid.appendChild(card);
    }
  } catch (err) {
    grid.replaceChildren(errorState('Failed to load library', loadLibrary));
  }
}

async function loadSearch(pluginID, query, page) {
  // Populate the plugin picker on first entry (§6.3).
  if (!state.plugins || state.plugins.length === 0) {
    try { state.plugins = await bindings.listPlugins(); } catch (_) { state.plugins = []; }
  }
  populatePluginSelect();

  const select = $('#plugin-select');
  if (pluginID && state.plugins.some((p) => p.id === pluginID)) {
    select.value = pluginID;
    state.searchPlugin = pluginID;
  } else if (!state.searchPlugin && state.plugins.length > 0) {
    state.searchPlugin = select.value;
  }

  const input = $('#search-input');
  if (query != null) input.value = query;
  state.searchQuery = input.value;

  const results = $('#search-results');
  const empty = $('#search-empty');
  const loading = $('#search-loading');
  const pagination = $('#search-pagination');

  if (!state.searchPlugin) {
    empty.querySelector('h2').textContent = 'Install a source first';
    empty.querySelector('p').textContent = 'Install a .wasm plugin to start searching.';
    empty.style.display = 'flex';
    return;
  }
  if (!state.searchQuery) {
    empty.querySelector('h2').textContent = 'Search a source';
    empty.querySelector('p').textContent = 'Type a title and press Enter or click Search.';
    empty.style.display = 'flex';
    pagination.style.display = 'none';
    return;
  }

  state.searchPage = page;
  results.innerHTML = '';
  empty.style.display = 'none';
  loading.style.display = 'flex';
  pagination.style.display = 'none';

  try {
    const items = await bindings.search(state.searchPlugin, { query: state.searchQuery, page });
    loading.style.display = 'none';
    if (!items || items.length === 0) {
      empty.querySelector('h2').textContent = 'No results found';
      empty.querySelector('p').textContent = 'Try a different search term or source.';
      empty.style.display = 'flex';
      return;
    }
    for (const m of items) {
      // types.Manga marshals as camelCase JSON keys (§contract).
      const card = await createMangaCard(state.searchPlugin, m.id, m.title, m.cover_url, m.status || '');
      results.appendChild(card);
    }
    pagination.style.display = 'flex';
    $('#page-info').textContent = `Page ${page}`;
  } catch (err) {
    loading.style.display = 'none';
    results.replaceChildren(errorState('Search failed', () => loadSearch(state.searchPlugin, state.searchQuery, state.searchPage)));
  }
}

function populatePluginSelect() {
  const select = $('#plugin-select');
  const current = select.value;
  select.innerHTML = '<option value="">Select a source…</option>';
  for (const p of state.plugins) {
    const opt = document.createElement('option');
    opt.value = p.id;
    opt.textContent = p.name || p.id;
    select.appendChild(opt);
  }
  if (current && state.plugins.some((p) => p.id === current)) select.value = current;
  else if (state.plugins.length === 1) select.value = state.plugins[0].id;
}

async function loadMangaDetail(pluginID, mangaID) {
  state.detail = { pluginID, mangaID, inLibrary: false };
  const detail = $('#manga-detail');
  const loading = $('#manga-loading');
  detail.style.display = 'none';
  loading.style.display = 'flex';

  try {
    const [manga, chapters] = await bindings.mangaDetails(pluginID, mangaID);
    state.chapters = Array.isArray(chapters) ? chapters : [];
    state.reader.title = manga.title;

    const cover = $('#manga-cover');
    cover.alt = manga.title || 'manga cover';
    const url = await loadImage(pluginID, manga.cover_url);
    cover.src = url || '';

    $('#manga-title').textContent = manga.title || 'Untitled';
    const statusBadge = $('#manga-status');
    statusBadge.textContent = manga.status || '';
    statusBadge.style.display = manga.status ? '' : 'none';
    const authorBadge = $('#manga-author');
    authorBadge.textContent = 'By ' + (manga.author || 'Unknown');
    $('#manga-description').textContent = manga.description || 'No description available.';

    checkLibrary(pluginID, mangaID);

    const list = $('#chapter-list');
    list.innerHTML = '';
    const sorted = state.chapters.slice().sort((a, b) => b.chapterNum - a.chapterNum); // newest first (§9.6)
    for (const chapter of sorted) {
      list.appendChild(createChapterRow(pluginID, mangaID, chapter));
    }
    loading.style.display = 'none';
    detail.style.display = 'block';
  } catch (err) {
    loading.style.display = 'none';
    detail.style.display = 'block';
    detail.replaceChildren(errorState('Failed to load manga detail', () => loadMangaDetail(pluginID, mangaID)));
  }
}

function checkLibrary(pluginID, mangaID) {
  const done = () => verifyMembership();
  if (!state.libraryList) {
    bindings.listLibrary().then((list) => { state.libraryList = list || []; done(); })
      .catch(() => { state.libraryList = []; done(); });
  } else {
    done();
  }
}

function verifyMembership() {
  const found = (state.libraryList || []).some(
    (m) => m.PluginID === state.detail.pluginID && m.SourceMangaID === state.detail.mangaID
  );
  state.detail.inLibrary = found;
  renderLibraryButton();
}

function renderLibraryButton() {
  const btn = $('#library-toggle');
  if (state.detail.inLibrary) {
    btn.textContent = 'In Library ✓';
    btn.classList.remove('btn-primary');
    btn.classList.add('btn-ghost');
  } else {
    btn.textContent = 'Add to Library';
    btn.classList.remove('btn-ghost');
    btn.classList.add('btn-primary');
  }
}

async function toggleLibrary() {
  const { pluginID, mangaID } = state.detail;
  try {
    await bindings.toggleLibrary(pluginID, mangaID);
    try { state.libraryList = await bindings.listLibrary(); } catch (_) {}
    state.detail.inLibrary = !state.detail.inLibrary;
    renderLibraryButton();
  } catch (err) {
    console.error('toggle library failed:', err);
  }
}

async function loadReader(pluginID, mangaID, chapterID, page) {
  try {
    const pages = await bindings.pageList(pluginID, chapterID);
    // §8.5: zero pages.
    if (!pages || pages.length === 0) {
      showReaderNoPages(pluginID, mangaID, chapterID);
      return;
    }
    setupReader();
    state.reader = {
      pluginID, mangaID, chapterID,
      pages,
      currentPage: clamp(page, 0, pages.length - 1),
      title: state.reader.title,
    };
    $('#reader-title').textContent =
      (state.reader.title || 'Manga') + ' — Ch. ' + chapterID;
    goToPage(state.reader.currentPage, false);
  } catch (err) {
    showReaderError('Failed to load chapter', () => loadReader(pluginID, mangaID, chapterID, 0));
  }
}

function showReaderNoPages(pluginID, mangaID, chapterID) {
  const wrap = $('#reader-image-wrap');
  wrap.innerHTML = '';
  const box = errorState('This chapter has no pages', () =>
    window.location.hash = `#/manga/${encodeURIComponent(pluginID)}/${encodeURIComponent(mangaID)}`);
  const back = box.querySelector('button');
  back.textContent = 'Back';
  wrap.appendChild(box);
  const slider = $('#reader-slider');
  slider.max = '0';
  slider.value = '0';
  $('#reader-page-info').textContent = '0 / 0';
  $('#reader-complete').style.display = 'none';
}

function showReaderError(message, retry) {
  const wrap = $('#reader-image-wrap');
  wrap.innerHTML = '';
  wrap.appendChild(errorState(message, retry));
  $('#reader-complete').style.display = 'none';
}

/* ------------------------------------------------------------------ *
 * 6.a Search controls + debounce (§11)
 * ------------------------------------------------------------------ */
function setupSearchControls() {
  const input = $('#search-input');
  const btn = $('#search-btn');
  let debounce = null;

  input.addEventListener('input', () => {
    state.searchQuery = input.value;
    // ponytail: enable the search button only after 300ms of idle typing (§11).
    clearTimeout(debounce);
    debounce = setTimeout(() => { btn.disabled = input.value.trim().length === 0; }, 300);
  });

  input.addEventListener('keydown', (e) => { if (e.key === 'Enter') runSearch(); });
  btn.addEventListener('click', runSearch);

  $('#plugin-select').addEventListener('change', (e) => { state.searchPlugin = e.target.value; });

  $('#prev-page').addEventListener('click', () => { if (state.searchPage > 1) goToSearchPage(state.searchPage - 1); });
  $('#next-page').addEventListener('click', () => goToSearchPage(state.searchPage + 1));
}

function runSearch() {
  state.searchQuery = $('#search-input').value.trim();
  state.searchPlugin = $('#plugin-select').value || state.searchPlugin;
  if (!state.searchPlugin || !state.searchQuery) return;
  window.location.hash =
    `#/search?plugin=${encodeURIComponent(state.searchPlugin)}&q=${encodeURIComponent(state.searchQuery)}&page=1`;
}

function goToSearchPage(page) {
  window.location.hash =
    `#/search?plugin=${encodeURIComponent(state.searchPlugin)}&q=${encodeURIComponent($('#search-input').value)}&page=${page}`;
}

/* ------------------------------------------------------------------ *
 * 6.b Plugin install (§5.5)
 * ------------------------------------------------------------------ */
function setupPluginInstall() {
  const btn = $('#install-plugin-btn');
  const input = $('#plugin-file-input');
  btn.addEventListener('click', () => input.click());
  input.addEventListener('change', async (e) => {
    const file = e.target.files[0];
    if (!file) return;
    try {
      await bindings.installPlugin(file.name);
      input.value = '';
      loadPlugins();
    } catch (err) {
      console.error('install plugin failed:', err);
      alert('Failed to install plugin: ' + err);
    }
  });
}

/* ------------------------------------------------------------------ *
 * 7. Reader logic (§5.4, §6.3)
 * ------------------------------------------------------------------ */
function setupReader() {
  const view = $('#reader-view');
  const wrap = $('#reader-image-wrap');

  $('#reader-back').addEventListener('click', () => exitReader());
  $('#reader-retry').addEventListener('click', () => {
    $('#reader-error').style.display = 'none';
    goToPage(state.reader.currentPage, true);
  });
  $('#next-chapter-btn').addEventListener('click', goToNextChapter);

  $('#reader-zone-prev').addEventListener('click', () => goToPage(state.reader.currentPage - 1));
  $('#reader-zone-next').addEventListener('click', () => goToPage(state.reader.currentPage + 1));

  wrap.addEventListener('mousemove', scheduleAutoHide);
  ['reader-topbar', 'reader-bottombar'].forEach((id) => {
    const bar = $('#' + id);
    bar.addEventListener('mousemove', scheduleAutoHide);
    bar.addEventListener('mouseover', () => { clearTimeout(view._autoHideTimer); view.classList.remove('auto-hide'); });
    bar.addEventListener('mouseout', scheduleAutoHide);
  });

  scheduleAutoHide();
}

function scheduleAutoHide() {
  const view = $('#reader-view');
  if (!view) return;
  clearTimeout(view._autoHideTimer);
  view.classList.remove('auto-hide');
  // ponytail: hide bars after 3s idle; mouse movement restarts the timer (§5.4).
  view._autoHideTimer = setTimeout(() => view.classList.add('auto-hide'), 3000);
}

async function goToPage(pageNum, forceReload) {
  const r = state.reader;
  if (!r.pages.length) return;
  const next = clamp(pageNum, 0, r.pages.length - 1);
  const changed = next !== r.currentPage || forceReload;
  r.currentPage = next;

  const img = $('#reader-image');
  const loading = $('#reader-loading');
  const error = $('#reader-error');
  error.style.display = 'none';
  $('#reader-complete').style.display = 'none';

  updateReaderChrome();
  if (!changed && img.src) { loading.style.display = 'none'; return; }

  loading.style.display = 'flex';
  const page = r.pages[next];
  const prevSrc = img.src;
  let url = null;
  try {
    url = await makeBlobUrl(r.pluginID, page.url, page.headers);
  } catch (e) {
    console.error('page load failed', e);
  }

  const finish = () => {
    loading.style.display = 'none';
    preloadAdjacentPages();
    onReadProgress(); // records §5.4 + last-page §8.9
  };
  if (url) {
    img.src = url;
    if (prevSrc && state.blobUrls.indexOf(prevSrc) === -1) revokeBlobUrl(prevSrc);
    img.onload = finish;
    img.onerror = () => { error.style.display = 'flex'; finish(); };
  } else {
    error.style.display = 'flex';
    finish();
  }
}

function onReadProgress() {
  const r = state.reader;
  const last = r.pages.length - 1;
  bindings.recordRead(r.pluginID, r.mangaID, r.chapterID, r.currentPage)
    .catch((e) => console.error('recordRead failed', e));
  state.readProgress[r.chapterID] = { lastPage: r.currentPage, pageCount: r.pages.length };

  if (r.currentPage >= last) {
    bindings.setChapterProgress(r.pluginID, r.mangaID, r.chapterID, last)
      .catch((e) => console.error('setChapterProgress failed', e));
    showChapterComplete();
  }
}

function preloadAdjacentPages() {
  // ponytail: preload ±1 through GetImage (never hotlink §constraint), letting
  // the blob cache serve repeat hits for free.
  const r = state.reader;
  for (const delta of [-1, 1]) {
    const i = r.currentPage + delta;
    if (i >= 0 && i < r.pages.length) {
      loadImage(r.pluginID, r.pages[i].url, r.pages[i].headers);
    }
  }
}

function updateReaderChrome() {
  const r = state.reader;
  const slider = $('#reader-slider');
  slider.min = '0';
  slider.max = String(Math.max(0, r.pages.length - 1));
  slider.value = String(r.currentPage);
  $('#reader-page-info').textContent = (r.currentPage + 1) + ' / ' + r.pages.length;
}

function showChapterComplete() {
  const box = $('#reader-complete');
  const nextBtn = $('#next-chapter-btn');
  // Find the next chapter by chapter_num (§8.9).
  const current = state.chapters.find((c) => c.id === state.reader.chapterID);
  const sorted = state.chapters.slice().sort((a, b) => a.chapterNum - b.chapterNum);
  let next = null;
  if (current) {
    const idx = sorted.findIndex((c) => c.id === current.id);
    if (idx !== -1 && idx < sorted.length - 1) next = sorted[idx + 1];
  }
  if (next) {
    nextBtn.style.display = '';
    nextBtn.onclick = () => {
      window.location.hash = `#/read/${encodeURIComponent(state.reader.pluginID)}/${encodeURIComponent(state.reader.mangaID)}/${encodeURIComponent(next.id)}`;
    };
  } else {
    nextBtn.style.display = 'none';
  }
  $('#back-to-manga-btn').href = `#/manga/${encodeURIComponent(state.reader.pluginID)}/${encodeURIComponent(state.reader.mangaID)}`;
  box.style.display = 'flex';
}

function goToNextChapter() { /* wired via onclick in showChapterComplete */ }

function exitReader() {
  const r = state.reader;
  // Persist current position on exit (§5.4).
  if (r.pages.length) {
    bindings.setChapterProgress(r.pluginID, r.mangaID, r.chapterID, r.currentPage)
      .catch((e) => console.error('setChapterProgress failed', e));
  }
  window.location.hash = `#/manga/${encodeURIComponent(r.pluginID)}/${encodeURIComponent(r.mangaID)}`;
}

/* ------------------------------------------------------------------ *
 * 8. Global keyboard shortcuts (§5.6, §5.4)
 * ------------------------------------------------------------------ */
function setupKeyboard() {
  document.addEventListener('keydown', (e) => {
    const inField = e.target.matches('input, select, textarea');

    if (e.ctrlKey && e.altKey && !inField) {
      if (e.key === '1') { e.preventDefault(); window.location.hash = '#/library'; return; }
      if (e.key === '2') { e.preventDefault(); window.location.hash = '#/search'; return; }
      if (e.key === '3') { e.preventDefault(); window.location.hash = '#/plugins'; return; }
    }

    const inReader = state.currentView === 'reader-view';
    if (inReader) {
      if (e.key === 'Escape') { e.preventDefault(); exitReader(); return; }
      if (e.key === 'f' || e.key === 'F') { e.preventDefault(); toggleFullscreen(); return; }
      if (e.key === 'Home') { e.preventDefault(); goToPage(0); return; }
      if (e.key === 'End') { e.preventDefault(); goToPage(state.reader.pages.length - 1); return; }
      if (!inField && (e.key === 'ArrowLeft' || e.key === 'a' || e.key === 'A')) { e.preventDefault(); goToPage(state.reader.currentPage - 1); return; }
      if (!inField && (e.key === 'ArrowRight' || e.key === 'd' || e.key === 'D')) { e.preventDefault(); goToPage(state.reader.currentPage + 1); return; }
      return;
    }

    if (e.key === '/' && !inField) { e.preventDefault(); $('#search-input').focus(); return; }
  });
}

function toggleFullscreen() {
  // ponytail: HTML5 fullscreen API — Wails exposes no JS fullscreen handle,
  // and this is the spec's "otherwise CSS" fallback (§5.4).
  if (!document.fullscreenElement) {
    document.documentElement.requestFullscreen().catch(() => {});
  } else {
    document.exitFullscreen().catch(() => {});
  }
}

/* ------------------------------------------------------------------ *
 * 9. UI helpers
 * ------------------------------------------------------------------ */
function errorState(message, retry) {
  const div = document.createElement('div');
  div.className = 'error-state';
  div.innerHTML = '<div class="error-icon">⚠️</div><p>' + escapeHtml(message) + '</p>';
  const btn = document.createElement('button');
  btn.className = 'btn btn-ghost';
  btn.textContent = 'Retry';
  if (retry) btn.addEventListener('click', retry);
  div.appendChild(btn);
  return div;
}

function escapeHtml(s) {
  return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

function getInitials(title) {
  const parts = String(title || '').trim().split(/\s+/).slice(0, 2);
  return parts.map((p) => p.charAt(0)).join('').toUpperCase() || '?';
}

function fallbackInitial(name) {
  return '<span class="fallback">' + (String(name || '?').charAt(0).toUpperCase() || '?') + '</span>';
}

function lookupPluginName(pluginID) {
  const found = state.plugins.find((p) => p.id === pluginID);
  return found ? (found.name || found.id) : '';
}

function formatChapterNum(num) {
  const n = Number(num);
  if (Number.isNaN(n)) return '—';
  return Number.isInteger(n) ? String(n) : String(n).replace(/\.?0+$/, '');
}

function formatDate(dateStr) {
  if (!dateStr) return '';
  const date = new Date(dateStr);
  if (isNaN(date.getTime())) return '';
  return date.toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric' });
}

/* ------------------------------------------------------------------ *
 * 10. Init
 * ------------------------------------------------------------------ */
document.addEventListener('DOMContentLoaded', () => {
  setupSearchControls();
  setupPluginInstall();
  setupKeyboard();
});
