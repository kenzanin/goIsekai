// goIsekai frontend — pure reader-hash helpers.
// Zero browser/Alpine dependencies so these can be unit-tested with plain node.
// reader.js re-exports them so the test exercises the real module.

// Parse a `#/read/<pid>/<mid>/<cid>?page=N` hash into {pid, mid, cid, page}.
// Returns null for empty / non-read / malformed / empty-segment hashes, which
// is exactly the empty-ID state that previously caused the reader to mount
// blank and never recover until a late hashchange.
export function parseReadHash(h) {
  const hash = h || '';
  if (!hash.startsWith('#/read/')) return null;
  const segs = hash.split('?')[0].replace('#/read/', '').split('/');
  if (segs.length < 3) return null;
  const pid = decodeURIComponent(segs[0]);
  const mid = decodeURIComponent(segs[1]);
  const cid = decodeURIComponent(segs[2]);
  if (!pid || !mid || !cid) return null;
  const params = new URLSearchParams(hash.split('?')[1] || '');
  const page = parseInt(params.get('page') || '0', 10) || 0;
  return { pid, mid, cid, page };
}

// Idempotency gate for reloadFromHash. Returns true only when the incoming
// chapter target differs from the last one actually loaded, so re-entrant
// re-fires (mount-time + hashchange racing) and empty/stale hashes no longer
// clobber an in-flight or successful chapter load.
export function shouldReload(newKey, lastReadHash) {
  return Boolean(newKey) && newKey !== lastReadHash;
}
