// Unit tests for the pure format/convert helpers in format.js.
// Runnable with plain node (no browser/Alpine/Wails runtime):
//   node cmd/goisekai/frontend/lib/format.test.js
// Exits non-zero on any failure so `make test-frontend` fails CI-style.

import {
  clamp,
  detectImageType,
  escapeHtml,
  fallbackInitial,
  formatChapterNum,
  formatDate,
  getInitials,
  lookupPluginName,
  toBytes,
} from './format.js';

const assert = (cond, msg) => {
  if (!cond) {
    console.error('FAIL:', msg);
    process.exitCode = 1;
  } else {
    console.log('ok -', msg);
  }
};

// --- toBytes ---
assert(toBytes(null) === null, 'toBytes(null) -> null');
assert(toBytes(undefined) === null, 'toBytes(undefined) -> null');
assert(toBytes(42) === null, 'toBytes(number) -> null');
{
  const u8 = new Uint8Array([1, 2, 3]);
  assert(toBytes(u8) === u8, 'toBytes(Uint8Array) -> same instance');
}
{
  const a = toBytes([1, 2, 3]);
  assert(
    a instanceof Uint8Array && a[0] === 1 && a[2] === 3 && a.length === 3,
    'toBytes(array) -> Uint8Array',
  );
}
{
  const a = toBytes('');
  assert(a instanceof Uint8Array && a.length === 0, "toBytes('') -> empty Uint8Array");
}

// --- detectImageType (magic-byte sniffing) ---
assert(detectImageType(null) === 'image/jpeg', 'detectImageType(null) -> jpeg default');
assert(detectImageType(new Uint8Array([0xff])) === 'image/jpeg', 'too-short -> jpeg default');
assert(detectImageType(new Uint8Array([0xff, 0xd8, 0xff, 0xe0])) === 'image/jpeg', 'jpeg magic');
assert(detectImageType(new Uint8Array([0x89, 0x50, 0x4e, 0x47])) === 'image/png', 'png magic');
assert(detectImageType(new Uint8Array([0x47, 0x49, 0x46, 0x38])) === 'image/gif', 'gif magic');
assert(detectImageType(new Uint8Array([0x52, 0x49, 0x46, 0x46])) === 'image/webp', 'webp magic');
assert(
  detectImageType(new Uint8Array([0x00, 0x00, 0x00, 0x00])) === 'image/jpeg',
  'unknown -> jpeg default',
);

// --- clamp ---
assert(clamp(5, 0, 10) === 5, 'clamp in-range unchanged');
assert(clamp(-5, 0, 10) === 0, 'clamp below -> lo');
assert(clamp(15, 0, 10) === 10, 'clamp above -> hi');

// --- escapeHtml ---
assert(escapeHtml('<a & b>') === '&lt;a &amp; b&gt;', 'escapeHtml escapes < & >');

// --- getInitials ---
assert(getInitials('Solo Leveling') === 'SL', 'getInitials two words');
assert(getInitials('  one two three  ') === 'OT', 'getInitials trims + first two words');
assert(getInitials('') === '?', 'getInitials empty -> ?');
assert(getInitials(null) === '?', 'getInitials null -> ?');
assert(getInitials('x') === 'X', 'getInitials single char -> upper');

// --- fallbackInitial ---
assert(fallbackInitial('manga') === 'M', 'fallbackInitial first char upper');
assert(fallbackInitial('') === '?', 'fallbackInitial empty -> ?');
assert(fallbackInitial(null) === '?', 'fallbackInitial null -> ?');

// --- lookupPluginName (PascalCase plugin list: p.ID / p.Name) ---
{
  const plugins = [
    { ID: 'mangadex', Name: 'MangaDex' },
    { ID: 'dummy', Name: 'Dummy' },
  ];
  assert(lookupPluginName(plugins, 'mangadex') === 'MangaDex', 'lookupPluginName by ID -> Name');
  assert(lookupPluginName(plugins, 'unknown') === '', "lookupPluginName unknown -> ''");
}
assert(
  lookupPluginName([{ ID: 'noid' }], 'noid') === 'noid',
  'lookupPluginName missing Name -> ID fallback',
);

// --- formatChapterNum ---
assert(formatChapterNum(5) === '5', "formatChapterNum integer -> '5'");
assert(formatChapterNum(5.0) === '5', "formatChapterNum 5.0 -> '5'");
assert(formatChapterNum(5.5) === '5.5', "formatChapterNum 5.5 -> '5.5'");
assert(formatChapterNum('7') === '7', "formatChapterNum '7' -> '7'");
assert(
  formatChapterNum('7.20') === '7.2',
  "formatChapterNum '7.20' -> '7.2' (trailing zero stripped)",
);
assert(formatChapterNum('abc') === '—', 'formatChapterNum non-numeric -> em dash');
assert(formatChapterNum(null) === '—', 'formatChapterNum null -> em dash');
assert(formatChapterNum(undefined) === '—', 'formatChapterNum undefined -> em dash');
assert(formatChapterNum('') === '—', 'formatChapterNum empty string -> em dash');

// --- formatDate ---
assert(formatDate('') === '', "formatDate empty -> ''");
assert(formatDate(null) === '', "formatDate null -> ''");
assert(formatDate('not-a-date') === '', "formatDate invalid -> ''");
{
  const d = formatDate('2024-03-05');
  assert(d === 'Mar 5, 2024', `formatDate ISO -> 'Mar 5, 2024' (got: ${d})`);
}

if (process.exitCode) {
  console.error('format.test.js: failures present');
  process.exit(process.exitCode);
}
console.log('\nformat.test.js: all assertions passed');
