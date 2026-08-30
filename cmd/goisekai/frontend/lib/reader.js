// goIsekai frontend — reader component logic: prefetch, keyboard handling,
// progress tracking, and page navigation.

import { bindings } from "./bindings.js";
import { loadImage, clamp } from "./utils.js";
import { settings, saveSetting } from "./state.js";
// Pure reader-hash helpers live in their own zero-browser-dependency module so
// they can be unit-tested with plain node (see readhash.test.js). Imported here
// and re-exported so reader logic uses the single source of truth.
import { parseReadHash, shouldReload, nextChapterInList, buildReadHash } from "./readhash.js";
export { parseReadHash, shouldReload, nextChapterInList, buildReadHash };

// `readerView` is the Alpine.data('readerView', readerView) factory.
export const readerView = () => ({
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
  deleteCacheOnRetry: false,
  lastReadHash: '',

  init() {
    const s = settings;
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
    // Handle a direct page-load at a read URL (no hashchange fires on refresh).
    // The hash may be empty/stale at mount if the router hasn\'t stamped the read
    // URL yet — recover on the next tick so the reader isn\'t stuck blank.
    this.reloadFromHash();
    setTimeout(() => {
      if (window.location.hash.startsWith('#/read/')) {
        console.log('[reader] mount recovery reloadFromHash:', window.location.hash);
        this.reloadFromHash();
      }
    }, 0);
  },

  async load(pid, mid, cid, page) {
    console.log('[reader] load:', 'pid=' + pid + ' mid=' + mid + ' cid=' + cid + ' page=' + (page || 0) + ' hash=' + (window.location.hash || ''));
    this.pluginID = pid;
    this.mangaID = mid;
    this.chapterID = cid;
    this.pages = [];
    this.currentPage = 0;
    this.pageSrc = '';
    this.loading = true;
    this.error = false;
    this.complete = false;
    this.renderMode = settings.renderMode;
    this.gpu = settings.gpuCompositing;
    this.viewMode = settings.viewMode;
    this.direction = settings.direction;

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
        // Reset scroll to top when changing pages.
        const wrap = document.querySelector('.reader-page')?.parentElement;
        if (wrap) wrap.scrollTop = 0;
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

  // Retry current page, optionally evicting cache first.
  async retryPage() {
    if (this.deleteCacheOnRetry) {
      const page = this.pages[this.currentPage];
      if (page) await bindings.evictImageCache(this.pluginID, page.url);
    }
    await this.goToPage(this.currentPage, true);
  },

  // Skip to next page without retrying.
  skipPage() {
    this.deleteCacheOnRetry = false;
    this.goNext();
  },

  // <img @error> handler: mark the page errored, stop the spinner, and log the
  // failing URL so broken-image failures are traceable in the Go stdout log.
  onImageError() {
    this.error = true;
    this.loading = false;
    console.error('[reader] image failed to load:', this.pageSrc, '(page ' + (this.currentPage + 1) + ')');
  },

  setViewMode(v) {
    this.viewMode = v;
    settings.viewMode = v;
    saveSetting('viewMode', v);
  },

  toggleDirection() {
    this.direction = this.direction === 'ltr' ? 'rtl' : 'ltr';
    settings.direction = this.direction;
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
    const id = nextChapterInList(list, this.chapterID, this.direction);
    if (id === null) {
      console.warn('[reader] nextChapterID: current chapter "' + this.chapterID + '" not found in reading-order list (len ' + list.length + ')');
      return null;
    }
    console.log('[reader] nextChapterID: idx ' + list.findIndex(c => c.id === this.chapterID) + ' (' + this.direction + ') -> "' + id + '"');
    return id;
  },

  // "Next Chapter" button (completion overlay). Navigates to the next chapter
  // in reading order; the reader re-loads via @window:hashchange -> reloadFromHash.
  goNextChapter() {
    const next = this.nextChapterID;
    console.log('[reader] next-chapter click: current "' + this.chapterID + '" next "' + next + '"');
    if (!next) {
      console.warn('[reader] next-chapter: no next chapter to navigate to');
      return;
    }
    this.$store.app.navigate(buildReadHash(this.pluginID, this.mangaID, next));
  },

  // Re-load the reader from the current URL. Covers reader->reader hash
  // navigation (Next Chapter, etc.), which the old x-effect couldn't trigger
  // because window.location.hash isn't reactive and currentView stays 'reader'
  // (so updateRoute() is a no-op). Also run on mount for a direct read URL.
  reloadFromHash() {
    const h = window.location.hash || '';
    const parsed = parseReadHash(h);
    if (!parsed) {
      if (!h) {
        console.warn('[reader] reloadFromHash: empty hash, mount recovery will retry');
      } else {
        console.warn('[reader] reloadFromHash: skipping non-read/non-canonical hash:', h);
      } 
      return;
    }
    const key = parsed.pid + '|' + parsed.mid + '|' + parsed.cid + '|' + parsed.page;
    if (key === this.lastReadHash) {
      console.log('[reader] reloadFromHash: already loaded, skip', key);
      return;
    }
    this.lastReadHash = key;
    console.log('[reader] reloadFromHash entry:', h);
    console.log('[reader] reloadFromHash:', h, '-> pid=' + parsed.pid + ' mid=' + parsed.mid + ' cid=' + parsed.cid + ' page=' + parsed.page);
    this.load(parsed.pid, parsed.mid, parsed.cid, parsed.page);
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
    // Never navigate to an empty-mid manga hash (e.g. #/manga//). If the reader
    // somehow has no manga id, fall back to the library — a detail view with an
    // empty mid would just re-fire the detail x-effect's bad-hash skip.
    if (!this.mangaID) {
      this.$store.app.navigate('#/library');
      return;
    }
    this.$store.app.navigate(`#/manga/${encodeURIComponent(this.pluginID)}/${encodeURIComponent(this.mangaID)}`);
  },

  backToManga() {
    if (!this.mangaID) return '#/library';
    return `#/manga/${encodeURIComponent(this.pluginID)}/${encodeURIComponent(this.mangaID)}`;
  },

  get pageInfo() { return (this.currentPage + 1) + ' / ' + this.pages.length; },
  get sliderMax() { return Math.max(0, this.pages.length - 1); },
});
