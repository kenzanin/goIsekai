// Real-browser verification that a genuine read-ahead failure is reported and
// does not block the chapter on screen (openspec
// harden-runtime-contention-and-cancel, task 4.5).
//
// Every read-ahead image (page >= 2) is fulfilled with 502 - a real upstream
// failure - while the display page is fulfilled with a 1x1 PNG. If read-ahead
// failures could block or error the chapter being read, the display image would
// never paint or the error panel would appear.

const BASE = process.env.BASE || 'http://127.0.0.1:8090';
const READER = process.env.READER;
const wsUrl = process.argv[2];

const PNG = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==',
  'base64',
).toString('base64');

// Negative-check switch: fail the display page too, which must turn the verdict
// FAIL. Without it a broken probe would still pass.
const FAIL_DISPLAY = process.env.FAIL_DISPLAY === '1';

const sent = [];
let nextId = 1;
const pending = new Map();
const eventWaiters = [];
let sessionId = null;
let targetId = null;
let readAheadSeen = 0;

const ws = new WebSocket(wsUrl);
await new Promise((res, rej) => {
  ws.addEventListener('open', res, { once: true });
  ws.addEventListener('error', rej, { once: true });
});

ws.addEventListener('message', (ev) => {
  const msg = JSON.parse(ev.data);
  if (msg.id && pending.has(msg.id)) {
    const { resolve, reject } = pending.get(msg.id);
    pending.delete(msg.id);
    if (msg.error) reject(new Error(JSON.stringify(msg.error)));
    else resolve(msg.result);
    return;
  }
  if (msg.method === 'Target.attachedToTarget') {
    sessionId = msg.sessionId;
    return;
  }
  if (sessionId && msg.sessionId !== sessionId) return;

  if (msg.method === 'Network.requestWillBeSent') {
    sent.push({ id: msg.requestId, url: msg.params.request.url });
  }

  if (msg.method === 'Fetch.requestPaused') {
    const url = msg.params.request.url;
    if (url.includes('/api/reader-data/')) {
      send('Fetch.continueRequest', { requestId: msg.params.requestId }).catch(() => {});
      return;
    }
    let upstream = '';
    try {
      upstream = new URL(url).searchParams.get('url') || '';
    } catch {}
    const m = upstream.match(/\/(\d+)\/(\d+)\.[a-z]+$/i);
    const pageNo = m ? Number(m[2]) : 0;

    if (pageNo <= 1) {
      send('Fetch.fulfillRequest', {
        requestId: msg.params.requestId,
        responseCode: FAIL_DISPLAY ? 502 : 200,
        responseHeaders: [{ name: 'Content-Type', value: FAIL_DISPLAY ? 'text/plain' : 'image/png' }],
        body: FAIL_DISPLAY ? Buffer.from('display failure').toString('base64') : PNG,
      }).catch(() => {});
      return;
    }
    // Read-ahead: fail it the way a broken upstream would.
    readAheadSeen++;
    send('Fetch.fulfillRequest', {
      requestId: msg.params.requestId,
      responseCode: 502,
      responseHeaders: [{ name: 'Content-Type', value: 'text/plain' }],
      body: Buffer.from('upstream read-ahead failure').toString('base64'),
    }).catch(() => {});
  }

  for (const w of eventWaiters.slice()) {
    if (w.match(msg.method, msg.params || {})) {
      eventWaiters.splice(eventWaiters.indexOf(w), 1);
      w.resolve();
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

function waitFor(method, match, timeoutMs) {
  return new Promise((resolve, reject) => {
    const w = { match, resolve };
    eventWaiters.push(w);
    setTimeout(() => {
      const i = eventWaiters.indexOf(w);
      if (i >= 0) eventWaiters.splice(i, 1);
      reject(new Error(`timeout waiting for ${method}`));
    }, timeoutMs);
  });
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

const { targetId: tid } = await send('Target.createTarget', { url: 'about:blank' }, false);
targetId = tid;
await send('Target.attachToTarget', { targetId, flatten: true }, false);
await waitFor('Target.attachedToTarget', () => true, 5000).catch(() => {});
for (let i = 0; i < 50 && !sessionId; i++) await sleep(20);
// If the attach event landed before the listener was ready, attach again and
// take the sessionId from the reply - otherwise every page command would go to
// the browser target and fail with "Page.enable wasn't found".
if (!sessionId) {
  const r = await send('Target.attachToTarget', { targetId, flatten: true }, false);
  sessionId = r.sessionId;
}

await send('Page.enable');
await send('Runtime.enable');
await send('Network.enable');
await send('Fetch.enable', {
  patterns: [{ urlPattern: `${BASE}/image*`, requestStage: 'Request' }],
});
await send('Page.navigate', { url: READER });

// Wait until read-ahead has actually been attempted and failed.
const deadline = Date.now() + 45000;
while (readAheadSeen < 3 && Date.now() < deadline) await sleep(150);
await sleep(2500); // let any error handling settle

const PROBE_EXPR = [
  '(() => {',
  '  // drawPage builds a detached new Image() and paints it to the canvas, so',
  '  // there is no <img> in the DOM to inspect. The spinner is hidden only from',
  '  // im.onload, and the canvas is sized only once a frame painted - both are',
  '  // honest signals that the display page really loaded.',
  "  const panel = document.querySelector('#error-panel');",
  "  const spinner = document.querySelector('#spinner');",
  "  const canvas = document.querySelector('#page-canvas');",
  '  const t = document.body.innerText || "";',
  '  let painted = false;',
  '  if (canvas && canvas.width > 0 && canvas.height > 0) {',
  '    try {',
  "      const g = canvas.getContext('2d');",
  '      const d = g.getImageData(0, 0, Math.min(canvas.width, 8), Math.min(canvas.height, 8)).data;',
  '      painted = Array.prototype.some.call(d, (v) => v !== 0);',
  '    } catch {',
  '      painted = canvas.width > 1;',
  '    }',
  '  }',
  '  return {',
  "    pageCounter: ((document.querySelector('#page-counter') || {}).textContent || '').trim(),",
  '    spinnerDisplay: spinner ? getComputedStyle(spinner).display : "no-el",',
  '    canvasPainted: painted,',
  '    canvasSize: canvas ? canvas.width + "x" + canvas.height : "none",',
  '    errorPanelDisplay: panel ? getComputedStyle(panel).display : "no-el",',
  '    failedTextShown: t.indexOf("Failed to load page") !== -1,',
  '  };',
  '})()',
].join('\n');

const probe = await send('Runtime.evaluate', {
  expression: PROBE_EXPR,
  returnByValue: true,
});

const st = probe.result.value;
// The chapter is intact when the display page finished loading (spinner hidden,
// canvas painted) and no reader error is showing, despite every read-ahead
// request having failed.
const verdict =
  readAheadSeen >= 3 &&
  st.spinnerDisplay === 'none' &&
  st.canvasPainted &&
  !st.failedTextShown &&
  st.errorPanelDisplay !== 'flex'
    ? 'PASS'
    : 'FAIL';

console.log(
  JSON.stringify(
    {
      readAheadRequestsFailed: readAheadSeen,
      ...st,
      verdict,
    },
    null,
    2,
  ),
);

await send('Target.closeTarget', { targetId }, false).catch(() => {});
process.exit(verdict === 'PASS' ? 0 : 1);