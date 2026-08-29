// goIsekai frontend — Alpine.js + Tailwind CSS, no build step.
// All Wails v3 service calls funnel through the single `bindings` object below.

import { Call } from "/wails/runtime.js";

/* ------------------------------------------------------------------ *
 * 1. Bindings — centralized Wails v3 bridge access
 * ------------------------------------------------------------------ */
const SVC = 'goisekai/internal/bridge.AppService';

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
  togglePlugin: (id) => call('TogglePlugin', id),
  syncLibrary: () => call('SyncLibrary'),
  getImage: (pluginID, url, headers) => call('GetImage', pluginID, url, headers),
  log: (level, msg) => call('Log', level, msg),
};

function call(method, ...args) {
  return Call.ByName(SVC + '.' + method, ...args);
}

/* ------------------------------------------------------------------ *
 * 1b. Settings — persisted to localStorage with the `gi_` prefix
 * ------------------------------------------------------------------ */
function getSetting(key, def) {
  try { return localStorage.getItem('gi_' + key) ?? def; } catch (_) { return def; }
}
function saveSetting(key, value) {
  try { localStorage.setItem('gi_' + key, String(value)); } catch (_) {}
}
const settings = {
  renderMode: getSetting('renderMode', 'smooth'),   // 'smooth' | 'sharp'
  gpuCompositing: getSetting('gpuCompositing', 'false') === 'true',
  readAhead: (() => { const n = parseInt(getSetting('readAhead', '3'), 10); return Number.isNaN(n) ? 3 : Math.max(0, Math.min(10, n)); })(),
  direction: getSetting('direction', 'ltr'),        // 'ltr' | 'rtl'
  viewMode: getSetting('viewMode', 'fitWidth'),     // 'fitWidth' | 'fitHeight' | 'original'
};
window.settings = settings;

// Shared reader control-button classes (defined here so index.html templates can reference them).
const RBTN = 'px-3 py-1.5 rounded-card border border-border text-zinc-100 bg-black/30 hover:bg-white/10 transition-all text-sm font-medium';
const RBTN_ACTIVE = 'px-3 py-1.5 rounded-card border border-accent text-accent bg-black/40 transition-all text-sm font-medium';

// Forward console errors/warns to Go logger so they show in terminal.
const _origError = console.error, _origWarn = console.warn, _origLog = console.log;
function _fwd(level, args) {
  try { bindings.log(level, args.map(a => typeof a === 'string' ? a : JSON.stringify(a)).join(' ')); } catch (_) {}
}
console.error = (...a) => { _origError(...a); _fwd('error', a); };
console.warn  = (...a) => { _origWarn(...a);  _fwd('warn',  a); };
console.log   = (...a) => { _origLog(...a);   _fwd('debug', a); };
window.addEventListener('error', e => _fwd('error', [e.message + ' at ' + e.filename + ':' + e.lineno]));
window.addEventListener('unhandledrejection', e => _fwd('error', ['Unhandled: ' + (e.reason?.message || e.reason)]));

/* ------------------------------------------------------------------ *
 * 2. Image loading — GetImage bytes → Blob URL (cache per-URL)
 * ------------------------------------------------------------------ */
const blobCache = new Map();
const blobUrls = [];
const MAX_BLOBS = 50;

function revokeAllBlobUrls() {
  for (const url of blobUrls.splice(0)) URL.revokeObjectURL(url);
  blobCache.clear();
}

async function loadImage(pluginID, url, headers) {
  if (!url) return null;
  const cached = blobCache.get(url);
  if (cached) return cached;
  try {
    const bytes = toBytes(await call('GetImage', pluginID, url, headers || {}));
    if (!bytes) return null;
    const blob = new Blob([bytes], { type: detectImageType(bytes) });
    const blobUrl = URL.createObjectURL(blob);
    blobCache.set(url, blobUrl);
    blobUrls.push(blobUrl);
    while (blobUrls.length > MAX_BLOBS) URL.revokeObjectURL(blobUrls.shift());
    return blobUrl;
  } catch (err) {
    console.error('Failed to load image:', url, err);
    return null;
  }
}

function toBytes(v) {
  if (v == null) return null;
  if (v instanceof Uint8Array) return v;
  if (Array.isArray(v)) return new Uint8Array(v);
  if (typeof v === 'string') {
    const bin = atob(v);
    const out = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
    return out;
  }
  return null;
}

function detectImageType(bytes) {
  if (!bytes || bytes.length < 4) return 'image/jpeg';
  if (bytes[0] === 0xFF && bytes[1] === 0xD8) return 'image/jpeg';
  if (bytes[0] === 0x89 && bytes[1] === 0x50 && bytes[2] === 0x4e && bytes[3] === 0x47) return 'image/png';
  if (bytes[0] === 0x47 && bytes[1] === 0x49 && bytes[2] === 0x46) return 'image/gif';
  if (bytes[0] === 0x52 && bytes[1] === 0x49 && bytes[2] === 0x46 && bytes[3] === 0x46) return 'image/webp';
  return 'image/jpeg';
}

/* ------------------------------------------------------------------ *
 * 3. Helpers
 * ------------------------------------------------------------------ */
const clamp = (v, lo, hi) => Math.max(lo, Math.min(hi, v));

function escapeHtml(s) {
  return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

function getInitials(title) {
  const parts = String(title || '').trim().split(/\s+/).slice(0, 2);
  return parts.map((p) => p.charAt(0)).join('').toUpperCase() || '?';
}

function fallbackInitial(name) {
  return String(name || '?').charAt(0).toUpperCase() || '?';
}

function lookupPluginName(plugins, pluginID) {
  const found = plugins.find((p) => p.ID === pluginID);
  return found ? (found.Name || found.ID) : '';
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

// Expose helpers needed by Alpine inline expressions
window.loadImage = loadImage;
window.getInitials = getInitials;
window.formatChapterNum = formatChapterNum;
window.formatDate = formatDate;
window.fallbackInitial = fallbackInitial;

/* ------------------------------------------------------------------ *
 * 4. Alpine stores and components
 * ------------------------------------------------------------------ */
document.addEventListener('alpine:init', () => {

  // Global app store — routing, plugins, library cache
  Alpine.store('app', {
    currentView: 'library',
    plugins: [],
    libraryList: null,
    readProgress: {},
    // chapters sorted ascending (by chapter_num), keyed by `pluginID|mangaID`
    chaptersByManga: {},

    navigate(hash) {
      revokeAllBlobUrls();
      window.location.hash = hash;
    },

    updateRoute() {
      const hash = window.location.hash || '#/library';
      const [path, queryStr] = hash.split('?');
      const params = new URLSearchParams(queryStr || '');
      const segments = path.replace('#/', '').split('/');
      const view = segments[0] || 'library';

      if (view === 'read') {
        this.currentView = 'reader';
      } else if (view === 'manga') {
        this.currentView = 'detail';
      } else {
        this.currentView = view;
      }
      return { view, segments, params };
    },

    async loadPlugins() {
      try {
        this.plugins = await bindings.listPlugins();
      } catch (_) {
        this.plugins = [];
      }
    },

    async loadLibrary() {
      try {
        this.libraryList = await bindings.listLibrary();
      } catch (_) {
        this.libraryList = [];
      }
    },

    pluginName(pluginID) {
      return lookupPluginName(this.plugins, pluginID);
    },
  });

  // ── Library component ─────────────────────────────────────────────
  Alpine.data('libraryView', () => ({
    loading: false,
    error: null,
    syncing: false,

    async init() {
      this.$watch('$store.app.currentView', (v) => {
        if (v === 'library') this.load();
      });
      if (this.$store.app.currentView === 'library') this.load();
    },

    async load() {
      this.loading = true;
      this.error = null;
      try {
        await this.$store.app.loadLibrary();
      } catch (err) {
        this.error = 'Failed to load library';
      } finally {
        this.loading = false;
      }
    },

    async sync() {
      this.syncing = true;
      try {
        await bindings.syncLibrary();
        await this.load();
      } catch (e) {
        console.error('sync library', e);
      } finally {
        this.syncing = false;
      }
    },

    get items() {
      return this.$store.app.libraryList || [];
    },
  }));

  // ── Search component ──────────────────────────────────────────────
  Alpine.data('searchView', () => ({
    pluginID: '',
    query: '',
    page: 1,
    results: [],
    loading: false,
    error: null,
    searched: false,

    async init() {
      if (!this.$store.app.plugins.length) await this.$store.app.loadPlugins();
      if (this.$store.app.plugins.length === 1) this.pluginID = this.$store.app.plugins[0].ID;
      this.$watch('$store.app.currentView', (v) => {
        if (v === 'search' && !this.$store.app.plugins.length) this.$store.app.loadPlugins();
      });
    },

    async search() {
      if (!this.pluginID || !this.query.trim()) return;
      this.loading = true;
      this.error = null;
      this.searched = true;
      try {
        const items = await bindings.search(this.pluginID, { query: this.query.trim(), page: this.page });
        this.results = items || [];
      } catch (err) {
        this.error = 'Search failed';
        this.results = [];
      } finally {
        this.loading = false;
      }
    },

    nextPage() { this.page++; this.search(); },
    prevPage() { if (this.page > 1) { this.page--; this.search(); } },

    get hasResults() { return this.results.length > 0; },
    get showEmpty() { return this.searched && !this.loading && !this.error && this.results.length === 0; },
  }));

  // ── Manga detail component ────────────────────────────────────────
  Alpine.data('detailView', () => ({
    pluginID: '',
    mangaID: '',
    manga: null,
    chapters: [],
    inLibrary: false,
    loading: false,
    error: null,
    coverUrl: '',

    async load(pid, mid) {
      this.pluginID = pid;
      this.mangaID = mid;
      this.loading = true;
      this.error = null;
      this.manga = null;
      this.chapters = [];
      this.coverUrl = '';

      try {
        const [manga, chapters] = await bindings.mangaDetails(pid, mid);
        this.manga = manga;
        this.chapters = Array.isArray(chapters)
          ? chapters.slice().sort((a, b) => (b.chapter_num || 0) - (a.chapter_num || 0))
          : [];

        if (manga.cover_url) {
          const url = await loadImage(pid, manga.cover_url);
          if (url) this.coverUrl = url;
        }

        // Expose chapters (ascending by number) to the reader for next-chapter nav.
        const asc = this.chapters.slice().sort((a, b) => (a.chapter_num || 0) - (b.chapter_num || 0));
        this.$store.app.chaptersByManga[pid + '|' + mid] = asc;

        // Check library membership
        if (!this.$store.app.libraryList) await this.$store.app.loadLibrary();
        this.inLibrary = (this.$store.app.libraryList || []).some(
          (m) => m.PluginID === pid && m.SourceMangaID === mid
        );
      } catch (err) {
        this.error = 'Failed to load manga details';
      } finally {
        this.loading = false;
      }
    },

    async toggleLibrary() {
      try {
        await bindings.toggleLibrary(this.pluginID, this.mangaID);
        await this.$store.app.loadLibrary();
        this.inLibrary = !this.inLibrary;
      } catch (err) {
        console.error('toggle library failed:', err);
      }
    },

    chapterUrl(ch) {
      return `#/read/${encodeURIComponent(this.pluginID)}/${encodeURIComponent(this.mangaID)}/${encodeURIComponent(ch.id)}`;
    },

    readProgress(ch) {
      return this.$store.app.readProgress[ch.id] || null;
    },

    get chaptersAscending() {
      return this.chapters.slice().sort((a, b) => (a.chapter_num || 0) - (b.chapter_num || 0));
    },

    // Where the "Continue Reading" button points: first chapter, or the
    // furthest-read chapter (at its last page), or the next chapter once the
    // furthest-read one is complete.
    get startTarget() {
      const list = this.chaptersAscending;
      if (!list.length) return null;
      let best = null, bestPage = -1;
      for (const ch of list) {
        const p = this.readProgress(ch);
        if (p && p.lastPage > bestPage) { bestPage = p.lastPage; best = ch; }
      }
      if (!best) {
        return { chapter: list[0], page: 0, label: 'Start Reading' };
      }
      const p = this.readProgress(best);
      if (p.lastPage >= p.pageCount) {
        const idx = list.indexOf(best);
        const nextCh = list[idx + 1];
        if (nextCh) return { chapter: nextCh, page: 0, label: 'Continue Reading', num: nextCh.chapter_num };
        return null; // fully caught up
      }
      return { chapter: best, page: p.lastPage, label: 'Continue Reading', num: best.chapter_num };
    },

    startReading() {
      const t = this.startTarget;
      if (!t || !t.chapter) return;
      this.$store.app.navigate(
        `#/read/${encodeURIComponent(this.pluginID)}/${encodeURIComponent(this.mangaID)}` +
        `/${encodeURIComponent(t.chapter.id)}?page=${t.page}`
      );
    },
  }));

  // ── Reader component ──────────────────────────────────────────────
  Alpine.data('readerView', () => ({
    pluginID: '',
    mangaID: '',
    chapterID: '',
    title: '',
    pages: [],
    currentPage: 0,
    pageSrc: '',
    loading: false,
    error: false,
     complete: false,
     autoHideTimer: null,
    controlsVisible: true,
    _boundKeyHandler: null,
    // Settings mirrored from window.settings (updated live in load()).
    renderMode: 'smooth',
    gpu: false,
    viewMode: 'fitWidth',
    direction: 'ltr',
    showShortcuts: false,

    init() {
      const s = window.settings;
      this.renderMode = s.renderMode;
      this.gpu = s.gpuCompositing;
      this.viewMode = s.viewMode;
      this.direction = s.direction;
      // Auto-dismiss the shortcuts legend on any key or click (see index.html).
      this.$watch('showShortcuts', (open) => {
        if (!open) return;
        const close = () => { this.showShortcuts = false; document.removeEventListener('keydown', close); };
        document.addEventListener('keydown', close);
      });
    },

    async load(pid, mid, cid, page) {
      this.pluginID = pid;
      this.mangaID = mid;
      this.chapterID = cid;
      this.pages = [];
      this.currentPage = 0;
      this.pageSrc = '';
      this.loading = true;
      this.error = false;
       this.complete = false;
       this.renderMode = window.settings.renderMode;
       this.gpu = window.settings.gpuCompositing;
       this.viewMode = window.settings.viewMode;
       this.direction = window.settings.direction;

       try {
         const pages = await bindings.pageList(pid, cid);
        if (!pages || pages.length === 0) {
          this.loading = false;
          return;
        }
         this.pages = pages;

         this.currentPage = clamp(page || 0, 0, pages.length - 1);
        await this.goToPage(this.currentPage);
        this.scheduleAutoHide();
        this.bindKeys();
        this.prefetchNextChapters();
      } catch (err) {
        console.error('Failed to load chapter:', err);
        this.loading = false;
        this.error = true;
      }
    },

    async goToPage(n, forceReload) {
      if (!this.pages.length) return;
      const next = clamp(n, 0, this.pages.length - 1);
      const changed = next !== this.currentPage || forceReload;
      this.currentPage = next;
      this.error = false;
      this.complete = false;

      if (!changed && this.pageSrc) { this.loading = false; return; }

      this.loading = true;
      const page = this.pages[next];
      try {
        const url = await loadImage(this.pluginID, page.url, page.headers);
        if (url) {
          this.pageSrc = url;
          this.loading = false;
          this.preloadAdjacent();
          this.onProgress();
        } else {
          this.error = true;
          this.loading = false;
        }
      } catch (e) {
        console.error('page load failed', e);
        this.error = true;
        this.loading = false;
      }
    },

    onProgress() {
      bindings.recordRead(this.pluginID, this.mangaID, this.chapterID, this.currentPage)
        .catch((e) => console.error('recordRead failed', e));
      this.$store.app.readProgress[this.chapterID] = { lastPage: this.currentPage, pageCount: this.pages.length };

      if (this.currentPage >= this.pages.length - 1) {
        bindings.setChapterProgress(this.pluginID, this.mangaID, this.chapterID, this.pages.length - 1)
          .catch((e) => console.error('setChapterProgress failed', e));
        this.showComplete();
      }
    },

    preloadAdjacent() {
      for (const delta of [-1, 1]) {
        const i = this.currentPage + delta;
        if (i >= 0 && i < this.pages.length) {
          loadImage(this.pluginID, this.pages[i].url, this.pages[i].headers);
        }
      }
    },

    showComplete() {
      this.complete = true;
    },

    scheduleAutoHide() {
      clearTimeout(this.autoHideTimer);
      this.controlsVisible = true;
      this.autoHideTimer = setTimeout(() => { this.controlsVisible = false; }, 3000);
    },

    bindKeys() {
      // Remove previous handler if any
      if (this._boundKeyHandler) document.removeEventListener('keydown', this._boundKeyHandler);
      this._boundKeyHandler = (e) => {
        const inField = e.target.matches('input, select, textarea');
        // A shortcuts-legend open wins: any key closes it before anything else.
        if (this.showShortcuts) { e.preventDefault(); this.showShortcuts = false; return; }
        if (e.key === 'Escape') { e.preventDefault(); this.exit(); return; }
        if (e.key === '?') { e.preventDefault(); this.showShortcuts = true; return; }
        if (e.key === 'f' || e.key === 'F') { e.preventDefault(); this.toggleFullscreen(); return; }
        if (!inField && e.key === ' ') { e.preventDefault(); this.goNext(); return; }
        if (e.key === 'Home') { e.preventDefault(); this.goToPage(0); return; }
        if (e.key === 'End') { e.preventDefault(); this.goToPage(this.pages.length - 1); return; }
        // ArrowRight: LTR → next, RTL → prev. ArrowLeft: LTR → prev, RTL → next.
        if (!inField && e.key === 'ArrowRight') { e.preventDefault(); this.direction === 'rtl' ? this.goPrev() : this.goNext(); return; }
        if (!inField && e.key === 'ArrowLeft')  { e.preventDefault(); this.direction === 'rtl' ? this.goNext()  : this.goPrev(); return; }
        if (!inField && (e.key === 'a' || e.key === 'A')) { e.preventDefault(); this.goPrev(); return; }
        if (!inField && (e.key === 'd' || e.key === 'D')) { e.preventDefault(); this.goNext(); return; }
      };
      document.addEventListener('keydown', this._boundKeyHandler);
    },

    // Reading-order navigation: always +1 / -1 by page index. Which key maps to
    // these (ArrowLeft vs ArrowRight) is decided in bindKeys() by the direction.
    goNext() { this.goToPage(this.currentPage + 1); },
    goPrev() { this.goToPage(this.currentPage - 1); },

    setViewMode(v) {
      this.viewMode = v;
      window.settings.viewMode = v;
      saveSetting('viewMode', v);
    },

    toggleDirection() {
      this.direction = this.direction === 'ltr' ? 'rtl' : 'ltr';
      window.settings.direction = this.direction;
      saveSetting('direction', this.direction);
    },

    get progressPct() {
      if (!this.pages.length) return 0;
      if (this.pages.length === 1) return 100;
      return Math.round((this.currentPage / (this.pages.length - 1)) * 100);
    },

    get orderedChapters() {
      const list = this.$store.app.chaptersByManga[this.pluginID + '|' + this.mangaID];
      return Array.isArray(list) ? list : [];
    },

    // Chapter after the current one in reading order. LTR → higher chapter
    // number (index+1); RTL → lower (index-1).
    get nextChapterID() {
      const list = this.orderedChapters;
      const idx = list.findIndex(c => c.id === this.chapterID);
      if (idx === -1) return null;
      const target = list[this.direction === 'rtl' ? idx - 1 : idx + 1];
      return target ? target.id : null;
    },

    // Silently prefetch the next K chapters' page blobs so they open instantly.
    prefetchNextChapters() {
      const k = clamp(settings.readAhead, 0, 10);
      if (k <= 0) return;
      const idx = this.orderedChapters.findIndex(c => c.id === this.chapterID);
      if (idx === -1) return;
      let remaining = k;
      for (let i = idx + 1; i < this.orderedChapters.length && remaining > 0; i++) {
        const ch = this.orderedChapters[i];
        bindings.pageList(this.pluginID, ch.id)
          .then(pages => {
            if (Array.isArray(pages)) {
              for (const p of pages) loadImage(this.pluginID, p.url, p.headers);
            }
          })
          .catch(() => {});
        remaining--;
      }
    },

    unbindKeys() {
      if (this._boundKeyHandler) {
        document.removeEventListener('keydown', this._boundKeyHandler);
        this._boundKeyHandler = null;
      }
    },

    toggleFullscreen() {
      if (!document.fullscreenElement) {
        document.documentElement.requestFullscreen().catch(() => {});
      } else {
        document.exitFullscreen().catch(() => {});
      }
    },

    exit() {
      this.unbindKeys();
      if (this.pages.length) {
        bindings.setChapterProgress(this.pluginID, this.mangaID, this.chapterID, this.currentPage)
          .catch((e) => console.error('setChapterProgress failed', e));
      }
      this.$store.app.navigate(`#/manga/${encodeURIComponent(this.pluginID)}/${encodeURIComponent(this.mangaID)}`);
    },

    backToManga() {
      return `#/manga/${encodeURIComponent(this.pluginID)}/${encodeURIComponent(this.mangaID)}`;
    },

    get pageInfo() { return (this.currentPage + 1) + ' / ' + this.pages.length; },
    get sliderMax() { return Math.max(0, this.pages.length - 1); },
  }));

  // ── Plugins component ─────────────────────────────────────────────
  Alpine.data('pluginsView', () => ({
    loading: false,
    error: null,

    async init() {
      this.$watch('$store.app.currentView', (v) => {
        if (v === 'plugins') this.load();
      });
      if (this.$store.app.currentView === 'plugins') this.load();
    },

    async load() {
      this.loading = true;
      this.error = null;
      try {
        await this.$store.app.loadPlugins();
      } catch (err) {
        this.error = 'Failed to load plugins';
      } finally {
        this.loading = false;
      }
    },

    async togglePlugin(id) {
      try {
        await bindings.togglePlugin(id);
        await this.load();
      } catch (e) {
        console.error('toggle plugin', e);
      }
    },

    async installPlugin() {
      const input = document.getElementById('plugin-file-input');
      input.click();
    },

    async onFileSelected(e) {
      const file = e.target.files[0];
      if (!file) return;
      try {
        await bindings.installPlugin(file.name);
        e.target.value = '';
        await this.load();
      } catch (err) {
        console.error('install plugin failed:', err);
        alert('Failed to install plugin: ' + err);
      }
    },

    get plugins() { return this.$store.app.plugins; },
  }));

  // ── Settings component ────────────────────────────────────────────
  Alpine.data('settingsView', () => {
    const s = window.settings;
    return {
      renderMode: s.renderMode,
      gpuCompositing: s.gpuCompositing,
      readAhead: s.readAhead,
      direction: s.direction,
      viewMode: s.viewMode,

      persist() {
        window.settings.renderMode = this.renderMode;
        window.settings.gpuCompositing = this.gpuCompositing;
        window.settings.readAhead = this.readAhead;
        window.settings.direction = this.direction;
        window.settings.viewMode = this.viewMode;
        saveSetting('renderMode', this.renderMode);
        saveSetting('gpuCompositing', String(this.gpuCompositing));
        saveSetting('readAhead', String(this.readAhead));
        saveSetting('direction', this.direction);
        saveSetting('viewMode', this.viewMode);
      },

      setReadAhead(v) {
        const n = parseInt(v, 10);
        this.readAhead = Number.isNaN(n) ? 0 : Math.max(0, Math.min(10, n));
      },
    };
  });

});

/* ------------------------------------------------------------------ *
 * 5. Router + keyboard (global)
 * ------------------------------------------------------------------ */
window.addEventListener('hashchange', () => {
  if (window.Alpine) Alpine.store('app').updateRoute();
});

document.addEventListener('DOMContentLoaded', () => {
  // Global keyboard shortcuts (non-reader)
  document.addEventListener('keydown', (e) => {
    const inField = e.target.matches('input, select, textarea');
    if (e.ctrlKey && e.altKey && !inField) {
      if (e.key === '1') { e.preventDefault(); window.location.hash = '#/library'; return; }
      if (e.key === '2') { e.preventDefault(); window.location.hash = '#/search'; return; }
      if (e.key === '3') { e.preventDefault(); window.location.hash = '#/plugins'; return; }
    }
    if (e.key === '/' && !inField) { e.preventDefault(); document.getElementById('search-input')?.focus(); return; }
  });
});
