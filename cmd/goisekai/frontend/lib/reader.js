// goIsekai frontend — reader component logic: prefetch, keyboard handling,
// progress tracking, and page navigation.

import { bindings } from './bindings.js';
// Pure reader-hash helpers live in their own zero-browser-dependency module so
// they can be unit-tested with plain node (see readhash.test.js). Imported here
// and re-exported so reader logic uses the single source of truth.
import { buildReadHash, nextChapterInList, parseReadHash, shouldReload } from './readhash.js';
import { saveSetting, settings } from './state.js';
import { clamp, loadImage } from './utils.js';

export { buildReadHash, nextChapterInList, parseReadHash, shouldReload };

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

  // Canvas rendering state
  _canvas: null,
  _ctx: null,
  _img: null,
  _baseScale: 1,
  _zoom: 1,
  _panX: 0,
  _panY: 0,
  _isDragging: false,
  _dragStartX: 0,
  _dragStartY: 0,
  _dragMoved: false,
  _dragThreshold: 5,

  init() {
    const s = settings;
    this.renderMode = s.renderMode;
    this.gpu = s.gpuCompositing;
    this.viewMode = s.viewMode;
    this.direction = s.direction;
    // Auto-dismiss the shortcuts legend on any key or click (see index.html).
    this.$watch('showShortcuts', (open) => {
      if (!open) return;
      const close = () => {
        this.showShortcuts = false;
        document.removeEventListener('keydown', close);
      };
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

    // Initialize canvas after Alpine has rendered the DOM
    this.$nextTick(() => {
      this._initCanvas();
    });

    // Handle window resize
    window.addEventListener('resize', () => {
      this._updateCanvasSize();
      this._render();
    });
  },

  _initCanvas() {
    this._canvas = document.getElementById('reader-canvas');
    if (!this._canvas) {
      console.error('[reader] Canvas element not found');
      return;
    }
    this._ctx = this._canvas.getContext('2d');
    this._setupCanvasEvents();
    this._setupGlobalMouseEvents();
    this._updateCanvasSize();
  },

  _setupCanvasEvents() {
    if (!this._canvas) return;

    // Mouse wheel: scroll the image by default; Ctrl+wheel zooms (cursor-anchored).
    this._canvas.addEventListener(
      'wheel',
      (e) => {
        e.preventDefault();
        if (e.ctrlKey) {
          // Zoom (cursor-anchored)
          const rect = this._canvas.getBoundingClientRect();
          const mouseX = e.clientX - rect.left;
          const mouseY = e.clientY - rect.top;
          const zoomFactor = e.deltaY > 0 ? 0.9 : 1.1;
          const newZoom = Math.max(0.2, Math.min(5, this._zoom * zoomFactor));
          const scaleChange = newZoom / this._zoom;
          this._panX = mouseX - (mouseX - this._panX) * scaleChange;
          this._panY = mouseY - (mouseY - this._panY) * scaleChange;
          this._zoom = newZoom;
          this._render();
          return;
        }
        // Scroll: pan the image, clamped to the viewport.
        this._panX -= e.deltaX;
        this._panY -= e.deltaY;
        this._clampPan();
        this._render();
      },
      { passive: false },
    );

    // Mouse drag for panning - only on canvas itself
    this._canvas.addEventListener('mousedown', (e) => {
      if (e.button !== 0) return; // Only left mouse button
      this._isDragging = true;
      this._dragMoved = false;
      this._dragStartX = e.clientX;
      this._dragStartY = e.clientY;
      this._canvas.style.cursor = 'grabbing';
    });

    // Double-click to reset zoom
    this._canvas.addEventListener('dblclick', () => {
      this._resetZoom();
    });
  },

  // Global mouse events for drag (attached once)
  _setupGlobalMouseEvents() {
    document.addEventListener('mousemove', (e) => {
      if (!this._isDragging) return;

      const dx = e.clientX - this._dragStartX;
      const dy = e.clientY - this._dragStartY;

      // Check if we've moved beyond threshold
      if (Math.abs(dx) > this._dragThreshold || Math.abs(dy) > this._dragThreshold) {
        this._dragMoved = true;
      }

      if (this._dragMoved) {
        this._panX += dx;
        this._panY += dy;
        this._dragStartX = e.clientX;
        this._dragStartY = e.clientY;
        this._render();
      }
    });

    document.addEventListener('mouseup', () => {
      if (this._isDragging) {
        this._isDragging = false;
        if (this._canvas) this._canvas.style.cursor = 'grab';
      }
    });
  },

  _updateCanvasSize() {
    if (!this._canvas) return;
    const dpr = window.devicePixelRatio || 1;
    const rect = this._canvas.getBoundingClientRect();

    // Set canvas backing store size
    this._canvas.width = rect.width * dpr;
    this._canvas.height = rect.height * dpr;

    // Scale context for high DPI
    this._ctx.scale(dpr, dpr);

    // Recalculate base scale when size changes
    this._calculateBaseScale();
  },

  _calculateBaseScale() {
    if (!this._img || !this._canvas) return;

    const canvasWidth = this._canvas.width / (window.devicePixelRatio || 1);
    const canvasHeight = this._canvas.height / (window.devicePixelRatio || 1);
    const imgWidth = this._img.naturalWidth;
    const imgHeight = this._img.naturalHeight;

    if (imgWidth === 0 || imgHeight === 0) return;

    switch (this.viewMode) {
      case 'fitWidth':
        this._baseScale = canvasWidth / imgWidth;
        break;
      case 'fitHeight':
        this._baseScale = canvasHeight / imgHeight;
        break;
      case 'original':
        this._baseScale = 1;
        break;
      default:
        this._baseScale = canvasWidth / imgWidth;
    }

    // Reset zoom and pan when view mode changes
    this._zoom = 1;
    this._panX = 0;
    this._panY = 0;
  },

  _resetZoom() {
    this._zoom = 1;
    this._panX = 0;
    this._panY = 0;
    this._render();
  },

  _clampPan() {
    if (!this._canvas || !this._img) return;
    const dpr = window.devicePixelRatio || 1;
    const cw = this._canvas.width / dpr;
    const ch = this._canvas.height / dpr;
    const scale = this._baseScale * this._zoom;
    const iw = this._img.naturalWidth * scale;
    const ih = this._img.naturalHeight * scale;
    // If the image fits fully, force it back to centered (no panning needed).
    if (iw <= cw) this._panX = 0;
    else this._panX = Math.min(0, Math.max(this._panX, cw - iw));
    if (ih <= ch) this._panY = 0;
    else this._panY = Math.min(0, Math.max(this._panY, ch - ih));
  },

  zoomBy(factor) {
    const newZoom = Math.max(0.2, Math.min(5, this._zoom * factor));
    const scaleChange = newZoom / this._zoom;
    const cw = (this._canvas?.width || 0) / (window.devicePixelRatio || 1);
    const ch = (this._canvas?.height || 0) / (window.devicePixelRatio || 1);
    this._panX = cw / 2 - (cw / 2 - this._panX) * scaleChange;
    this._panY = ch / 2 - (ch / 2 - this._panY) * scaleChange;
    this._zoom = newZoom;
    this._clampPan();
    this._render();
  },

  _render() {
    if (!this._ctx || !this._img || !this._canvas) return;

    const ctx = this._ctx;
    const canvas = this._canvas;
    const dpr = window.devicePixelRatio || 1;
    const canvasWidth = canvas.width / dpr;
    const canvasHeight = canvas.height / dpr;

    // Clear canvas
    ctx.clearRect(0, 0, canvasWidth, canvasHeight);

    // Set image smoothing based on render mode
    ctx.imageSmoothingEnabled = this.renderMode === 'smooth';
    if (this.renderMode === 'smooth') {
      ctx.imageSmoothingQuality = 'high';
    }

    // Calculate final scale
    const scale = this._baseScale * this._zoom;

    // Calculate image position. Center horizontally; vertically align to the
    // top when the image overflows the viewport so the page's top isn't cut
    // off (center-anchoring a tall page pushes its top out of frame).
    const imgWidth = this._img.naturalWidth * scale;
    const imgHeight = this._img.naturalHeight * scale;
    const x = (canvasWidth - imgWidth) / 2 + this._panX;
    const y = Math.max((canvasHeight - imgHeight) / 2, 0) + this._panY;

    // Draw image
    ctx.drawImage(this._img, x, y, imgWidth, imgHeight);
  },

  async load(pid, mid, cid, page) {
    console.log(
      '[reader] load:',
      'pid=' +
        pid +
        ' mid=' +
        mid +
        ' cid=' +
        cid +
        ' page=' +
        (page || 0) +
        ' hash=' +
        (window.location.hash || ''),
    );
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

    // The canvas is sized at mount via $nextTick, but the reader section is
    // hidden (x-show) at app start, so getBoundingClientRect() returned 0x0
    // and the backing store is permanently zero-sized. Re-measure now that
    // the section is visible so rendered pages actually draw.
    this.$nextTick(() => this._updateCanvasSize());

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

    if (!changed && this.pageSrc) {
      this.loading = false;
      return;
    }

    this.loading = true;
    const page = this.pages[next];
    try {
      const url = await loadImage(
        this.pluginID,
        page.url,
        page.headers,
        this.mangaID,
        this.chapterID,
      );
      if (url) {
        this.pageSrc = url;

        // Load image into canvas
        const img = new Image();
        img.onload = () => {
          this._img = img;
          this._calculateBaseScale();
          this._render();
          this.loading = false;
          this.preloadAdjacent();
          this.onProgress();
        };
        img.onerror = () => {
          this.error = true;
          this.loading = false;
          console.error('[reader] image failed to load:', url, `(page ${this.currentPage + 1})`);
        };
        img.src = url;
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
    bindings
      .recordRead(this.pluginID, this.mangaID, this.chapterID, this.currentPage)
      .catch((e) => console.error('recordRead failed', e));
    this.$store.app.readProgress[this.chapterID] = {
      lastPage: this.currentPage,
      pageCount: this.pages.length,
    };

    if (this.currentPage >= this.pages.length - 1) {
      bindings
        .setChapterProgress(this.pluginID, this.mangaID, this.chapterID, this.pages.length - 1)
        .catch((e) => console.error('setChapterProgress failed', e));
      this.showComplete();
    }
  },

  preloadAdjacent() {
    for (const delta of [-1, 1]) {
      const i = this.currentPage + delta;
      if (i >= 0 && i < this.pages.length) {
        loadImage(
          this.pluginID,
          this.pages[i].url,
          this.pages[i].headers,
          this.mangaID,
          this.chapterID,
        );
      }
    }
  },

  showComplete() {
    this.complete = true;
  },

  scheduleAutoHide() {
    // Toolbars are kept persistent (requested: back button + page number must
    // stay visible). Moving the mouse still re-asserts visibility.
    clearTimeout(this.autoHideTimer);
    this.controlsVisible = true;
    /* ponytail: auto-hide disabled per user request; re-enable with a
       setTimeout(() => { this.controlsVisible = false; }, 3000) if wanted. */
  },

  bindKeys() {
    // Remove previous handler if any
    if (this._boundKeyHandler) document.removeEventListener('keydown', this._boundKeyHandler);
    this._boundKeyHandler = (e) => {
      const inField = e.target.matches('input, select, textarea');
      // A shortcuts-legend open wins: any key closes it before anything else.
      if (this.showShortcuts) {
        e.preventDefault();
        this.showShortcuts = false;
        return;
      }
      if (e.key === 'Escape') {
        e.preventDefault();
        this.exit();
        return;
      }
      if (e.key === '?') {
        e.preventDefault();
        this.showShortcuts = true;
        return;
      }
      if (e.key === 'f' || e.key === 'F') {
        e.preventDefault();
        this.toggleFullscreen();
        return;
      }
      if (!inField && e.key === ' ') {
        e.preventDefault();
        this.goNext();
        return;
      }
      if (e.key === 'Home') {
        e.preventDefault();
        this.goToPage(0);
        return;
      }
      if (e.key === 'End') {
        e.preventDefault();
        this.goToPage(this.pages.length - 1);
        return;
      }
      // ArrowRight: LTR → next, RTL → prev. ArrowLeft: LTR → prev, RTL → next.
      if (!inField && e.key === 'ArrowRight') {
        e.preventDefault();
        this.direction === 'rtl' ? this.goPrev() : this.goNext();
        return;
      }
      if (!inField && e.key === 'ArrowLeft') {
        e.preventDefault();
        this.direction === 'rtl' ? this.goNext() : this.goPrev();
        return;
      }
      if (!inField && (e.key === 'a' || e.key === 'A')) {
        e.preventDefault();
        this.goPrev();
        return;
      }
      if (!inField && (e.key === 'd' || e.key === 'D')) {
        e.preventDefault();
        this.goNext();
        return;
      }
      if (!inField && (e.key === 'r' || e.key === 'R')) {
        e.preventDefault();
        this._resetZoom();
        return;
      }
    };
    document.addEventListener('keydown', this._boundKeyHandler);
  },

  // Reading-order navigation: always +1 / -1 by page index. Which key maps to
  // these (ArrowLeft vs ArrowRight) is decided in bindKeys() by the direction.
  goNext() {
    this.goToPage(this.currentPage + 1);
  },
  goPrev() {
    this.goToPage(this.currentPage - 1);
  },

  // Handle click on left/right zones with drag disambiguation
  handleZoneClick(direction, _event) {
    // If we were dragging, don't navigate
    if (this._dragMoved) return;

    // Navigate based on direction
    if (direction === 'left') {
      this.goPrev();
    } else {
      this.goNext();
    }
  },

  // Retry current page, optionally evicting cache first.
  async retryPage() {
    if (this.deleteCacheOnRetry) {
      const page = this.pages[this.currentPage];
      if (page)
        await bindings.evictImageCache(this.pluginID, page.url, this.mangaID, this.chapterID);
    }
    await this.goToPage(this.currentPage, true);
  },

  // Skip to next page without retrying.
  skipPage() {
    this.deleteCacheOnRetry = false;
    this.goNext();
  },

  // Error handling is now done in the img.onerror callback in goToPage()

  setViewMode(v) {
    this.viewMode = v;
    settings.viewMode = v;
    saveSetting('viewMode', v);
    // Recalculate base scale for new view mode
    this._calculateBaseScale();
    this._render();
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
    const list = this.$store.app.chaptersByManga[`${this.pluginID}|${this.mangaID}`];
    return Array.isArray(list) ? list : [];
  },

  // Chapter after the current one in reading order. LTR → higher chapter
  // number (index+1); RTL → lower (index-1).
  get nextChapterID() {
    const list = this.orderedChapters;
    const id = nextChapterInList(list, this.chapterID, this.direction);
    if (id === null) {
      console.warn(
        '[reader] nextChapterID: current chapter "' +
          this.chapterID +
          '" not found in reading-order list (len ' +
          list.length +
          ')',
      );
      return null;
    }
    console.log(
      '[reader] nextChapterID: idx ' +
        list.findIndex((c) => c.id === this.chapterID) +
        ' (' +
        this.direction +
        ') -> "' +
        id +
        '"',
    );
    return id;
  },

  // "Next Chapter" button (completion overlay). Navigates to the next chapter
  // in reading order; the reader re-loads via @window:hashchange -> reloadFromHash.
  goNextChapter() {
    const next = this.nextChapterID;
    console.log(`[reader] next-chapter click: current "${this.chapterID}" next "${next}"`);
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
    const key = `${parsed.pid}|${parsed.mid}|${parsed.cid}|${parsed.page}`;
    if (key === this.lastReadHash) {
      console.log('[reader] reloadFromHash: already loaded, skip', key);
      return;
    }
    this.lastReadHash = key;
    console.log('[reader] reloadFromHash entry:', h);
    console.log(
      '[reader] reloadFromHash:',
      h,
      `-> pid=${parsed.pid} mid=${parsed.mid} cid=${parsed.cid} page=${parsed.page}`,
    );
    this.load(parsed.pid, parsed.mid, parsed.cid, parsed.page);
  },

  // Silently prefetch the next K chapters' page blobs so they open instantly.
  prefetchNextChapters() {
    const k = clamp(settings.readAhead, 0, 10);
    if (k <= 0) return;
    const idx = this.orderedChapters.findIndex((c) => c.id === this.chapterID);
    if (idx === -1) return;
    let remaining = k;
    for (let i = idx + 1; i < this.orderedChapters.length && remaining > 0; i++) {
      const ch = this.orderedChapters[i];
      bindings
        .pageList(this.pluginID, ch.id)
        .then((pages) => {
          if (Array.isArray(pages)) {
            for (const p of pages) loadImage(this.pluginID, p.url, p.headers, this.mangaID, ch.id);
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
      bindings
        .setChapterProgress(this.pluginID, this.mangaID, this.chapterID, this.currentPage)
        .catch((e) => console.error('setChapterProgress failed', e));
    }
    // Never navigate to an empty-mid manga hash (e.g. #/manga//). If the reader
    // somehow has no manga id, fall back to the library — a detail view with an
    // empty mid would just re-fire the detail x-effect's bad-hash skip.
    if (!this.mangaID) {
      this.$store.app.navigate('#/library');
      return;
    }
    this.$store.app.navigate(
      `#/manga/${encodeURIComponent(this.pluginID)}/${encodeURIComponent(this.mangaID)}`,
    );
  },

  backToManga() {
    if (!this.mangaID) return '#/library';
    return `#/manga/${encodeURIComponent(this.pluginID)}/${encodeURIComponent(this.mangaID)}`;
  },

  get pageInfo() {
    return `${this.currentPage + 1} / ${this.pages.length}`;
  },
  get sliderMax() {
    return Math.max(0, this.pages.length - 1);
  },
});
