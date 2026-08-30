// Unit tests for the pure reader-hash helpers in readhash.js.
// Runnable with plain node (no browser/Alpine/Wails runtime):
//   node cmd/goisekai/frontend/lib/readhash.test.js
// Exits non-zero on any failure so `make test-reader` fails CI-style.

import { buildReadHash, nextChapterInList, parseReadHash, shouldReload } from './readhash.js';

const assert = (cond, msg) => {
  if (!cond) {
    console.error('FAIL:', msg);
    process.exitCode = 1;
  } else {
    console.log('ok -', msg);
  }
};

// --- parseReadHash ---
assert(JSON.stringify(parseReadHash('')) === 'null', 'empty hash -> null');
assert(JSON.stringify(parseReadHash('#/library')) === 'null', '#/library -> null');
assert(JSON.stringify(parseReadHash('#/manga/pid/mid')) === 'null', '#/manga/ -> null (non-read)');
assert(JSON.stringify(parseReadHash('#/read/')) === 'null', '#/read/ -> null (no segments)');
assert(
  JSON.stringify(parseReadHash('#/read/pid/mid')) === 'null',
  '#/read/pid/mid -> null (2 segs)',
);
assert(
  JSON.stringify(parseReadHash('#/read/pid//cid')) === 'null',
  '#/read/pid//cid -> null (empty mid)',
);
assert(
  JSON.stringify(parseReadHash('#/read//mid/cid')) === 'null',
  '#/read//mid/cid -> null (empty pid)',
);
assert(
  JSON.stringify(parseReadHash('#/read/pid/mid/cid')) ===
    JSON.stringify({ pid: 'pid', mid: 'mid', cid: 'cid', page: 0 }),
  '#/read/pid/mid/cid -> parsed, page 0',
);
assert(
  JSON.stringify(parseReadHash('#/read/pid/mid/cid?page=2')) ===
    JSON.stringify({ pid: 'pid', mid: 'mid', cid: 'cid', page: 2 }),
  'page param parsed',
);
assert(
  JSON.stringify(parseReadHash('#/read/pid/mid/cid?page=abc')) ===
    JSON.stringify({ pid: 'pid', mid: 'mid', cid: 'cid', page: 0 }),
  'NaN page -> 0',
);

const enc = parseReadHash('#/read/mangadex%20plug/7643e9f6/mid%20cid?page=3');
assert(
  enc &&
    enc.pid === 'mangadex plug' &&
    enc.mid === '7643e9f6' &&
    enc.cid === 'mid cid' &&
    enc.page === 3,
  'URI-encoded segments + page decoded',
);

// --- shouldReload (idempotency) ---
assert(
  shouldReload('pid|mid|cid|0', 'pid|mid|cid|0') === false,
  'identical target -> no reload (prevents clobber)',
);
assert(shouldReload('pid|mid|cid|0', '') === true, 'first load (empty last) -> reload');
assert(shouldReload('', 'pid|mid|cid|0') === false, 'empty new key -> no reload');
assert(shouldReload('', '') === false, 'both empty -> no reload');
assert(shouldReload('pid|mid|cid|0', 'pid|mid|cid|2') === true, 'different page -> reload');
assert(shouldReload('pid|mid|other|0', 'pid|mid|cid|0') === true, 'different chapter -> reload');

// --- nextChapterInList (reading-order next chapter) ---
const chapters = [{ id: 'ch1' }, { id: 'ch2' }, { id: 'ch3' }, { id: 'ch4' }];
assert(nextChapterInList(chapters, 'ch2', 'ltr') === 'ch3', 'ltr: next is ch3');
assert(nextChapterInList(chapters, 'ch2', 'rtl') === 'ch1', 'rtl: next is ch1 (previous in list)');
assert(nextChapterInList(chapters, 'ch1', 'rtl') === null, 'rtl at first chapter -> null');
assert(nextChapterInList(chapters, 'ch4', 'ltr') === null, 'ltr at last chapter -> null');
assert(nextChapterInList(chapters, 'missing', 'ltr') === null, 'current not in list -> null');
assert(nextChapterInList([], 'ch1', 'ltr') === null, 'empty list -> null');
assert(nextChapterInList(null, 'ch1', 'ltr') === null, 'null list -> null');
assert(nextChapterInList([{ id: 'only' }], 'only', 'ltr') === null, 'single chapter -> null');

// --- buildReadHash (inverse of parseReadHash) ---
assert(buildReadHash('pid', 'mid', 'cid') === '#/read/pid/mid/cid', 'buildReadHash basic');
assert(
  buildReadHash('pid', 'mid', 'cid', 2) === '#/read/pid/mid/cid?page=2',
  'buildReadHash with page',
);
assert(buildReadHash('pid', 'mid', 'cid', 0) === '#/read/pid/mid/cid', 'page 0 omitted');
assert(
  buildReadHash('mangadex plug', 'mid', 'cid with space') ===
    '#/read/mangadex%20plug/mid/cid%20with%20space',
  'buildReadHash URI-encodes',
);

// Round-trip: parseReadHash(buildReadHash(...)) must return the original ids.
const rt = parseReadHash(buildReadHash('pid', 'mid', 'cid', 4));
assert(
  rt && rt.pid === 'pid' && rt.mid === 'mid' && rt.cid === 'cid' && rt.page === 4,
  'buildReadHash/parseReadHash round-trip',
);

if (process.exitCode) {
  console.error('readhash.test.js: failures present');
  process.exit(process.exitCode);
}
console.log('\nreadhash.test.js: all assertions passed');
