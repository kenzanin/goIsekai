// goIsekai frontend — application state.
// The `app` Alpine store ($store.app) plus persisted settings; the settings
// helpers live here so they stay together with the settings object.

import { bindings } from './bindings.js';
import { lookupPluginName, revokeAllBlobUrls } from './utils.js';

/* ------------------------------------------------------------------ *
 * Settings — persisted to localStorage with the `gi_` prefix
 * ------------------------------------------------------------------ */
function getSetting(key, def) {
  try {
    return localStorage.getItem(`gi_${key}`) ?? def;
  } catch (_) {
    return def;
  }
}
function saveSetting(key, value) {
  try {
    localStorage.setItem(`gi_${key}`, String(value));
  } catch (_) {}
}

const settings = {
  renderMode: getSetting('renderMode', 'smooth'), // 'smooth' | 'sharp'
  gpuCompositing: getSetting('gpuCompositing', 'false') === 'true',
  readAhead: (() => {
    const n = parseInt(getSetting('readAhead', '3'), 10);
    return Number.isNaN(n) ? 3 : Math.max(0, Math.min(10, n));
  })(),
  direction: getSetting('direction', 'ltr'), // 'ltr' | 'rtl'
  viewMode: getSetting('viewMode', 'fitWidth'), // 'fitWidth' | 'fitHeight' | 'original'
};
window.settings = settings;

export { getSetting, saveSetting, settings };

/* ------------------------------------------------------------------ *
 * App store — routing, plugins, library cache (registered as $store.app)
 * ------------------------------------------------------------------ */
export const appStore = {
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
};
