// Real-browser verification of the reader read-ahead abort (openspec
// harden-runtime-contention-and-cancel, task 4.3).
//
// The repo has no JS test runner and reader.js is a self-wiring IIFE exposing
// nothing, so the only honest check is a real browser. This drives headless
// Chrome over CDP using node's built-in WebSocket - no dependencies.
//
// Determinism comes from the Fetch domain: page 1 is let through so the reader
// renders, while every read-ahead image (page >= 2) is held open. That leaves
// read-ahead genuinely in flight, so a chapter switch has something to abort.
// Timing races are impossible; the abort is observed, not waited for.

const BASE = process.env.BASE || 'http://127.0.0.1:8090';
const READER = process.env.READER;
const OLD_CHAPTER_MARKER = process.env.OLD_CHAPTER_MARKER;
const HOLD_PAGES = Number(process.env.HOLD_PAGES || 3);

const wsUrl = process.argv[2];

const held = new Map(); // requestId -> { url, chapter } -- read-ahead in flight
// 1x1 PNG: any display page resolves instantly, so the only slow requests are
// the read-ahead images we deliberately hold.
const PNG = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==',
  'base64',
).toString('base64');
const CANNED_DATA = process.env.CANNED_DATA || '';
// The page's own reader-data must come from the real server: fulfilling it too
// would boot the reader on the canned chapter, so every later observation would
// describe the wrong chapter.
let readerDataSeen = 0;
const failures = [];
const sent = [];
let nextId = 1;
const pending = new Map();
const eventWaiters = [];

const ws = new WebSocket(wsUrl);
await new Promise((res, rej) => {
  ws.addEventListener('open', res, { once: true });
  ws.addEventListener('error', rej, { once: true });
});

let sessionId = null;
ws.addEventListener('message', (ev) => {
  const msg = JSON.parse(ev.data);
  if (msg.id && pending.has(msg.id)) {
    const { resolve, reject } = pending.get(msg.id);
    pending.delete(msg.id);
    if (msg.error) reject(new Error(JSON.stringify(msg.error)));
    else resolve(msg.result);
    return;
  }
  const method = msg.method;
  const p = msg.params || {};

  if (method === 'Target.attachedToTarget') {
    sessionId = p.sessionId;
    return;
  }
  if (sessionId && msg.sessionId !== sessionId) return;

  if (method === 'Network.requestWillBeSent') {
    sent.push({ id: p.requestId, url: p.request.url });
  }

  if (method === 'Network.loadingFailed') {
    failures.push({
      requestId: p.requestId,
      errorText: p.errorText,
      canceled: p.canceled === true,
    });
  }

  if (method === 'Fetch.requestPaused') {
    const url = p.request.url;

    if (url.includes('/api/reader-data/')) {
      readerDataSeen++;
      if (readerDataSeen === 1) {
        // Initial page load: let the real server answer it.
        send('Fetch.continueRequest', { requestId: p.requestId }).catch(() => {});
        return;
      }
      // A chapter switch: serve it instantly so commitChapter runs and there is
      // something to abort.
      const body = CANNED_DATA || '{}';
      send('Fetch.fulfillRequest', {
        requestId: p.requestId,
        responseCode: 200,
        responseHeaders: [
          { name: 'Content-Type', value: 'application/json' },
          { name: 'X-Partial', value: 'true' },
        ],
        body: Buffer.from(body).toString('base64'),
      }).catch(() => {});
      return;
    }

    // The reader passes the upstream page URL as ?url=...
    let upstream = '';
    try {
      upstream = new URL(url).searchParams.get('url') || '';
    } catch {}
    const m = upstream.match(/\/(\d+)\/(\d+)\.[a-z]+$/i);
    const pageNo = m ? Number(m[2]) : 0;

    if (pageNo <= 1) {
      // Display page: resolve instantly so the reader always renders.
      send('Fetch.fulfillRequest', {
        requestId: p.requestId,
        responseCode: 200,
        responseHeaders: [{ name: 'Content-Type', value: 'image/png' }],
        body: PNG,
      }).catch(() => {});
      return;
    }

    // Page >= 2 is read-ahead. Hold it open: this is what the chapter switch
    // has to abort.
    held.set(p.requestId, { url, upstream });
  }

  for (const w of eventWaiters.slice()) {
    if (w.match(method, p)) {
      eventWaiters.splice(eventWaiters.indexOf(w), 1);
      w.resolve({ method, params: p });
    }
  }
});

function send(method, params = {}, useSession = true) {
  const id = nextId++;
  const payload = { id, method, params };
  if (useSession && sessionId) payload.sessionId = sessionId;
  ws.send(JSON.stringify(payload));
  return new Promise((resolve, reject) => pending.set(id, { resolve, reject }));
}

function waitFor(method, match, timeoutMs, label) {
  return new Promise((resolve, reject) => {
    const w = { match, resolve };
    eventWaiters.push(w);
    setTimeout(() => {
      const i = eventWaiters.indexOf(w);
      if (i >= 0) eventWaiters.splice(i, 1);
      reject(new Error(`timeout waiting for ${label || method} (${timeoutMs}ms)`));
    }, timeoutMs);
  });
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// ---- attach to a fresh page target -------------------------------------
await send('Target.setDiscoverTargets', { discover: true }, false);
const { targetId } = await send(
  'Target.createTarget',
  { url: 'about:blank' },
  false,
);
await send(
  'Target.attachToTarget',
  { targetId, flatten: true },
  false,
);
await waitFor('Target.attachedToTarget', () => true, 5000, 'attach').catch(() => {});

// The attach event carries the sessionId; if the listener above already
// captured it we are good, otherwise read it from the pending attach.
for (let i = 0; i < 50 && !sessionId; i++) await sleep(20);
if (!sessionId) {
  const { sessionId: sid } = await send(
    'Target.attachToTarget',
    { targetId, flatten: true },
    false,
  );
  sessionId = sid;
}

await send('Page.enable');
await send('Runtime.enable');
await send('Network.enable');
// Intercept reader-data too. The chapter switch fetches the NEXT chapter's
// metadata before it commits; without this it would wait on the live plugin,
// commitChapter would never run, and there would be nothing to abort.
await send('Fetch.enable', {
  patterns: [
    { urlPattern: `${BASE}/image*`, requestStage: 'Request' },
    { urlPattern: `${BASE}/api/reader-data/*`, requestStage: 'Request' },
  ],
});

// ---- navigate and let read-ahead fill up ------------------------------
await send('Page.navigate', { url: READER });

const deadline = Date.now() + 60000;
while (held.size < HOLD_PAGES && Date.now() < deadline) await sleep(100);

console.log(
  JSON.stringify(
    {
      step: 'read-ahead in flight',
      heldCount: held.size,
      heldUpstreams: [...new Set([...held.values()].map((h) => h.upstream.slice(-40)))],
    },
    null,
    2,
  ),
);

if (held.size === 0) {
  console.log(JSON.stringify({ verdict: 'INCONCLUSIVE', why: 'no read-ahead observed' }, null, 2));
  await send('Target.closeTarget', { targetId }, false).catch(() => {});
  process.exit(2);
}

// ---- switch chapter through the real UI control ----------------------
await sleep(3000); // let the initial chapter settle before sampling

const beforeState = await send('Runtime.evaluate', {
  expression: `((document.querySelector('#page-counter') || {}).textContent || '').trim()`,
  returnByValue: true,
});
const beforeCounter = beforeState.result.value || '';

const clicked = await send('Runtime.evaluate', {
  expression: `(() => {
    const b = document.querySelector('#btn-next-ch');
    if (!b) return { ok: false, why: 'no #btn-next-ch' };
    b.click();
    return { ok: true };
  })()`,
  returnByValue: true,
});
console.log(JSON.stringify({ step: 'clicked next chapter', result: clicked.result.value }, null, 2));

// Everything held at this instant is the read-ahead the switch must abandon.
// Match on the upstream URL, not the requestId: Fetch.requestPaused and
// Network.loadingFailed do not share a requestId namespace in Chrome.
const upstreamOf = (url) => {
  try {
    return decodeURIComponent(new URL(url).searchParams.get('url') || '');
  } catch {
    return url || '';
  }
};
const inFlightAtClick = new Set([...held.values()].map((h) => h.upstream));
const sentUrlOf = (requestId) => {
  const req = sent.find((r) => r.id === requestId);
  return req ? upstreamOf(req.url) : '';
};

// ---- the switch must abort the in-flight read-ahead -------------------
const abortWindow = Date.now() + 15000;
while (Date.now() < abortWindow) {
  const aborted = failures.filter((f) => f.canceled);
  if (aborted.length > 0) break;
  await sleep(100);
}

// Give the switch time to commit and repaint the counter.
const switchDeadline = Date.now() + 20000;
while (Date.now() < switchDeadline) {
  const st = await send('Runtime.evaluate', {
    expression: `(document.querySelector('#page-counter') || {}).textContent || ''`,
    returnByValue: true,
  });
  if ((st.result.value || '').trim() !== beforeCounter) break;
  await sleep(150);
}

// Release everything still held so the browser can settle.
for (const id of [...held.keys()]) {
  await send('Fetch.continueRequest', { requestId: id }).catch(() => {});
}

// Map each aborted requestId back to the URL that was held, so we can prove the
// abort belonged to the OLD chapter's read-ahead rather than anything else.
const abortedReadAhead = failures.filter(
  (f) => f.canceled && inFlightAtClick.has(sentUrlOf(f.requestId)),
);

// ---- reader must not show an error after the transition --------------
const readerState = await send('Runtime.evaluate', {
  expression: `(() => {
    const t = document.body.innerText || '';
    return {
      failedText: t.includes('Failed to load'),
      counter: (document.querySelector('#page-counter') || {}).textContent || '',
      errorHidden: (() => {
        const e = document.querySelector('#error-panel');
        return e ? getComputedStyle(e).display : 'no-el';
      })(),
    };
  })()`,
  returnByValue: true,
});

const state = readerState.result.value;
const report = {
  abortedReadAheadAtClick: abortedReadAhead.length,
  abortedTotal: failures.filter((f) => f.canceled).length,
  errorTextsSeen: [...new Set(failures.map((f) => f.errorText))],
  // Diagnostics: without these it is impossible to tell an aborted read-ahead
  // apart from a page-1 request dropped at Browser.close().
  heldAtClick: inFlightAtClick.size,
  failuresDetail: failures.map((f) => {
      return {
      canceled: f.canceled,
      errorText: f.errorText,
      wasHeldAtClick: inFlightAtClick.has(sentUrlOf(f.requestId)),
      upstream: sentUrlOf(f.requestId).slice(-34),
    };
  }),
  readerVisibleError: state.failedText,
  readerErrorOverlay: state.errorHidden,
  pageCounter: state.counter.trim(),
  verdict: abortedReadAhead.length > 0 && !state.failedText ? 'PASS' : 'FAIL',
};
console.log(JSON.stringify({ step: 'result', ...report }, null, 2));

await send('Target.closeTarget', { targetId }, false).catch(() => {});
process.exit(report.verdict === 'PASS' ? 0 : 1);