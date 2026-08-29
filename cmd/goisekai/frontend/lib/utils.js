// goIsekai frontend — shared utilities.
// Image loading + blob cache, format helpers, console→Go logging bridge,
// and reader toolbar button classes.

import { bindings } from "./bindings.js";

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
export const blobCache = new Map();
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
 * 4. Format + lookup helpers
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
