// goIsekai frontend — pure format/convert helpers.
// Zero browser/Alpine/Wails dependencies so they can be unit-tested with plain
// node (see format.test.js). utils.js imports and re-exports these so the
// window.* bindings and existing imports keep working.

// Convert a Go-bridge value into a Uint8Array. Wails marshals Go []byte as a
// base64 string; the array/typed-array forms cover local/test callers.
export function toBytes(v) {
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

// Sniff an image's MIME type from its magic bytes (defaults to jpeg).
export function detectImageType(bytes) {
  if (!bytes || bytes.length < 4) return 'image/jpeg';
  if (bytes[0] === 0xff && bytes[1] === 0xd8) return 'image/jpeg';
  if (bytes[0] === 0x89 && bytes[1] === 0x50 && bytes[2] === 0x4e && bytes[3] === 0x47)
    return 'image/png';
  if (bytes[0] === 0x47 && bytes[1] === 0x49 && bytes[2] === 0x46) return 'image/gif';
  if (bytes[0] === 0x52 && bytes[1] === 0x49 && bytes[2] === 0x46 && bytes[3] === 0x46)
    return 'image/webp';
  return 'image/jpeg';
}

export const clamp = (v, lo, hi) => Math.max(lo, Math.min(hi, v));

export function escapeHtml(s) {
  return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

// Up to 2 initials from a title's first two words, or '?'.
export function getInitials(title) {
  const parts = String(title || '')
    .trim()
    .split(/\s+/)
    .slice(0, 2);
  return (
    parts
      .map((p) => p.charAt(0))
      .join('')
      .toUpperCase() || '?'
  );
}

export function fallbackInitial(name) {
  return (
    String(name || '?')
      .charAt(0)
      .toUpperCase() || '?'
  );
}

// Resolve a plugin name from the PascalCase plugin list (p.ID / p.Name).
export function lookupPluginName(plugins, pluginID) {
  const found = plugins.find((p) => p.ID === pluginID);
  return found ? found.Name || found.ID : '';
}

// Format a chapter number: integers as-is, floats with trailing zeros stripped,
// non-numeric / null / empty -> em dash.
export function formatChapterNum(num) {
  if (num === null || num === undefined || num === '') return '—';
  const n = Number(num);
  if (Number.isNaN(n)) return '—';
  return Number.isInteger(n) ? String(n) : String(n).replace(/\.?0+$/, '');
}

export function formatDate(dateStr) {
  if (!dateStr) return '';
  const date = new Date(dateStr);
  if (Number.isNaN(date.getTime())) return '';
  return date.toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric' });
}
