// goIsekai frontend — view components (Alpine.data factories), excluding the
// reader which lives in reader.js.

import { bindings } from "./bindings.js";
import { loadImage } from "./utils.js";
import { settings, saveSetting } from "./state.js";
import { readerView } from "./reader.js";

// ── Library component ─────────────────────────────────────────────
export const libraryView = () => ({
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
});

// ── Search component ──────────────────────────────────────────────
export const searchView = () => ({
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
});

// ── Manga detail component ────────────────────────────────────────
export const detailView = () => ({
  pluginID: '',
  mangaID: '',
  manga: null,
  chapters: [],
  inLibrary: false,
  loading: false,
  error: null,
  coverUrl: '',

  async load(pid, mid) {
    console.log('[detail] load:', 'pid=' + pid + ' mid=' + mid + ' hash=' + (window.location.hash || ''));
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
});

// ── Plugins component ─────────────────────────────────────────────
export const pluginsView = () => ({
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
});

// ── Settings component ────────────────────────────────────────────
export const settingsView = () => {
  const s = settings;
  return {
    renderMode: s.renderMode,
    gpuCompositing: s.gpuCompositing,
    readAhead: s.readAhead,
    direction: s.direction,
    viewMode: s.viewMode,
    configPath: '',
    configStatus: '',

    init() {
      bindings.getConfigPath().then(p => { this.configPath = p; }).catch(() => {});
    },

    persist() {
      settings.renderMode = this.renderMode;
      settings.gpuCompositing = this.gpuCompositing;
      settings.readAhead = this.readAhead;
      settings.direction = this.direction;
      settings.viewMode = this.viewMode;
      saveSetting('renderMode', this.renderMode);
      saveSetting('gpuCompositing', String(this.gpuCompositing));
      saveSetting('readAhead', String(this.readAhead));
      saveSetting('direction', this.direction);
      saveSetting('viewMode', this.viewMode);
    },

    reloadConfig() {
      this.configStatus = 'Reloading...';
      bindings.reloadConfig()
        .then(() => { this.configStatus = 'Config reloaded ✓'; setTimeout(() => { this.configStatus = ''; }, 3000); })
        .catch(e => { this.configStatus = 'Error: ' + (e?.message || e); });
    },

    setReadAhead(v) {
      const n = parseInt(v, 10);
      this.readAhead = Number.isNaN(n) ? 0 : Math.max(0, Math.min(10, n));
    },
  };
};

// Aggregated map so app.js can register every view in one place.
export const viewComponents = {
  libraryView,
  searchView,
  detailView,
  readerView,
  pluginsView,
  settingsView,
};
