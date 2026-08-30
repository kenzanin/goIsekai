// goIsekai frontend — shared utilities.
// Image loading + blob cache, console→Go logging bridge, and reader toolbar
// button classes. Pure format/convert helpers live in format.js (re-exported
// here so existing imports + the window.* bindings keep working).

import { bindings, call } from "./bindings.js";
import {
  toBytes,
  detectImageType,
  clamp,
  escapeHtml,
  getInitials,
  fallbackInitial,
  lookupPluginName,
  formatChapterNum,
  formatDate,
} from "./format.js";

/* ------------------------------------------------------------------ *
 * 1. Error handling — forward console errors/warns to the Go logger
 *    so they surface in the terminal.
 * ------------------------------------------------------------------ */
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
 * 2. Reader toolbar button classes (module-scoped, not exposed to the
 *    Alpine template scope — matches pre-split behavior).
 * ------------------------------------------------------------------ */
export const RBTN = 'px-3 py-1.5 rounded-card border border-border text-zinc-100 bg-black/30 hover:bg-white/10 transition-all text-sm font-medium';
export const RBTN_ACTIVE = 'px-3 py-1.5 rounded-card border border-accent text-accent bg-black/40 transition-all text-sm font-medium';

/* ------------------------------------------------------------------ *
 * 3. Image loading — GetImage bytes → Blob URL (cache per-URL).
 *    `blobCache` stays centralized here so it is never spread out.
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
    console.error('Failed to load image:', url, err?.message || err?.toString() || JSON.stringify(err));
    return null;
  }
}

// Expose helpers needed by Alpine inline expressions in index.html.
window.loadImage = loadImage;
window.getInitials = getInitials;
window.formatChapterNum = formatChapterNum;
window.formatDate = formatDate;
window.fallbackInitial = fallbackInitial;
window.RBTN = RBTN;
window.RBTN_ACTIVE = RBTN_ACTIVE;

export {
  blobCache,
  revokeAllBlobUrls,
  loadImage,
  toBytes,
  detectImageType,
  clamp,
  escapeHtml,
  getInitials,
  fallbackInitial,
  lookupPluginName,
  formatChapterNum,
  formatDate,
};
