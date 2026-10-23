(() => {
  var root = document.getElementById('reader');
  var canvas = document.getElementById('page-canvas');
  var ctx = canvas.getContext('2d');
  var spinner = document.getElementById('spinner');
  var errPanel = document.getElementById('error-panel');
  var counter = document.getElementById('page-counter');
  var pageJumper = document.getElementById('page-jumper');
  var pageProgress = document.getElementById('page-progress');
  var pageDots = document.getElementById('page-dots');
  var chapterJumperBtn = document.getElementById('chapter-jumper');
  var chapterMenu = document.getElementById('chapter-menu');

  // State
  var pid = root.dataset.pluginId,
    mid = root.dataset.mangaId,
    cid = root.dataset.chapterId;
  var pages = [],
    current = 0,
    img = null,
    loading = false,
    _lastFailedRetry = null;
  // Server per-manga prefs (embedded by the view; "" = global defaults).
  // Server wins over the localStorage mirror so a synced device picks up
  // fresh values instead of a stale mirror.
  var srvPrefs = {};
  try {
    srvPrefs = JSON.parse(root.dataset.mangaPrefs || '{}') || {};
  } catch {
    srvPrefs = {};
  }
  var scope = `${pid}.${mid}`;
  function hasSrv(k) {
    return Object.prototype.hasOwnProperty.call(srvPrefs, k);
  }
  var viewKey = `gi_viewMode.${scope}`;
  var dirKey = `gi_direction.${scope}`;
  var viewMode = hasSrv('viewMode')
    ? srvPrefs.viewMode
    : localStorage.getItem(viewKey) || localStorage.getItem('gi_viewMode') || 'fitWidth';
  var direction = hasSrv('direction')
    ? srvPrefs.direction
    : localStorage.getItem(dirKey) || localStorage.getItem('gi_direction') || 'ltr';
  var smoothing = (localStorage.getItem('gi_renderMode') || 'smooth') === 'smooth';
  var zoom = 1,
    panX = 0,
    panY = 0,
    baseScale = 1,
    dpr = 1;
  var preloaded = {}; // page index -> Image; n{k}/p{k} = neighbor-chapter spill
  var nextData = null,
    nextPages = null, // neighbor reader-data, kept for instant chapter switch
    prevData = null,
    prevPages = null,
    prevWarmTried = false; // one attempt per chapter; a failing neighbor must not be hammered
  var imgFails = 0; // consecutive image-load failures (at-home nodes flake transiently)
  var readAhead = Math.max(
    0,
    Math.min(10, parseInt(localStorage.getItem('gi_readAhead'), 10) || 3),
  );
  var startPage = parseInt(new URLSearchParams(window.location.search).get('page'), 10);
  var nextChID = root.dataset.nextChapterId || '';
  var prevChID = root.dataset.prevChapterId || '';
  // Vertical strip mode: continuous downward scroll. When a chapter's pages run
  // out, the next chapter's pages are appended seamlessly (sites that split one
  // manga release across several chapter parts read as one flow).
  var stripKey = `gi_stripMode.${scope}`;
  var stripMode = hasSrv('strip')
    ? !!srvPrefs.strip
    : localStorage.getItem(stripKey) === '1' || localStorage.getItem(`gi_stripMode.${mid}`) === '1';
  var stripView = document.getElementById('strip-view');
  var stripLoading = false; // strip is appending a neighbor chapter
  // Two-page view for sites that split one original page into two images: a
  // single next/prev shows both halves at once instead of making the reader
  // click twice per original page. `spread` is the layout:
  //   ''      one page
  //   'side'  halves side by side (a page split left/right)
  //   'stack' one above the other (a page split top/bottom) — reads as one
  //           continuous page, and next shows the following pair.
  var spreadKey = `gi_spreadMode.${scope}`;
  var spreadStored = hasSrv('spread')
    ? srvPrefs.spread
    : localStorage.getItem(spreadKey) || localStorage.getItem(`gi_spreadMode.${mid}`);
  // Older builds stored a boolean ('1'/true) for the side-by-side view.
  var spread =
    spreadStored === true || spreadStored === 'true' || spreadStored === '1'
      ? 'side'
      : spreadStored === 'stack'
        ? 'stack'
        : '';
  var img2 = null; // second page of a two-page view; null in single-page view

  // Pages on screen, in reading order. Side by side is reversed for rtl manga
  // so the earlier page sits on the right; a vertical stack keeps its order.
  function viewPages() {
    var ims = [];
    if (img) ims.push(img);
    if (img2) ims.push(img2);
    return spread === 'side' && direction === 'rtl' ? ims.reverse() : ims;
  }

  // Bounding box of the pages on screen, as one unit so both halves scale
  // together — side by side adds widths, stacked adds heights.
  function viewBox() {
    var ims = viewPages();
    if (!ims.length) return null;
    var w = 0,
      h = 0;
    for (var i = 0; i < ims.length; i++) {
      if (spread === 'stack') {
        w = Math.max(w, ims[i].width);
        h += ims[i].height;
      } else {
        w += ims[i].width;
        h = Math.max(h, ims[i].height);
      }
    }
    return w && h ? { w: w, h: h } : null;
  }

  // Debug logger - sends to both browser console and backend server log
  var _debugSid = Math.random().toString(36).substring(2, 10);
  var _logToServer = true;
  function debugLog(msg, level) {
    level = level || 'log';
    var timestamp = new Date().toISOString().substring(11, 23);
    var formatted = `[${timestamp}] [reader:${_debugSid}] ${msg}`;
    console.log(formatted);
    if (_logToServer && typeof fetch !== 'undefined') {
      fetch('/api/frontend-log', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ tag: 'reader', level: level, msg: msg, sid: _debugSid }),
      }).catch(function() {});
    }
  }
  function debugWarn(msg) { debugLog(msg, 'warn'); }
  function debugError(msg) { debugLog(msg, 'error'); }

  var loadAbortController = null;
  // One controller per read-ahead direction. `new Image()` has no cancellation
  // handle, so read-ahead issues fetch() instead and is abandoned at the network
  // layer when the reader leaves the chapter — otherwise skipped chapters leave
  // orphaned requests holding image-lane worker slots.
  // CSRF token from the page meta tag. This progress write builds its own body
  // with no form to carry a hidden field, so the header is the only channel.
  var csrfToken = (() => {
    const el = document.querySelector('meta[name="csrf-token"]');
    return el ? el.getAttribute('content') || '' : '';
  })();
  // Debounced server save of the per-manga prefs blob. Wheel-zoom fires
  // per tick, so only the settled value is POSTed (600ms).
  var savePrefsTimer = null;
  function postPrefs(keepalive) {
    savePrefsTimer = null;
    fetch(`/action/reader-prefs/${[pid, mid].map(encodeURIComponent).join('/')}`, {
      method: 'POST',
      keepalive: keepalive,
      headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
      body: JSON.stringify({ viewMode, direction, strip: stripMode, spread: spread, zoom, zoomMode: viewMode }),
    }).catch(() => {});
  }
  function savePrefs() {
    clearTimeout(savePrefsTimer);
    savePrefsTimer = setTimeout(() => postPrefs(false), 600);
  }
  // Switching chapter reloads the page, which throws away a pending debounce:
  // the next chapter then opened with the previous settings and the change
  // looked lost. Flush the settled value before the page goes away.
  function flushPrefs() {
    if (savePrefsTimer === null) return;
    clearTimeout(savePrefsTimer);
    postPrefs(true);
  }
  window.addEventListener('pagehide', flushPrefs);
  document.addEventListener('visibilitychange', function() {
    if (document.visibilityState === 'hidden') flushPrefs();
  });
  var warmCtrl = { next: null, prev: null };
  // key -> true while a read-ahead fetch is in flight. prefetch() runs on every
  // page turn, so without this the same page would be re-requested until its
  // first fetch lands.
  var warming = {};
  // Bars stay visible only until the first page renders — after that the
  // immersive default (hidden until tapped) resumes.
  var initialLoad = true;

  // Auto-hide bars state
  var barsVisible = true;
  var topBar = document.getElementById('top-bar');
  var bottomBar = document.getElementById('bottom-bar');
  var zoneCenter = document.getElementById('zone-center');

  function setBarsVisible(visible) {
    barsVisible = visible;
    if (visible) {
      topBar.style.transform = 'translateY(0)';
      bottomBar.style.transform = 'translateY(0)';
      pageDots.style.opacity = '0';
      pageDots.classList.add('pointer-events-none');
    } else {
      topBar.style.transform = 'translateY(-100%)';
      bottomBar.style.transform = 'translateY(100%)';
      pageDots.style.opacity = '1';
      pageDots.classList.remove('pointer-events-none');
    }
  }

  // Update page jumper with progress bar + clickable numbered steps
  function updatePageJumper() {
    var total = pages.length || 1;
    if (!pageJumper || total === 0) return;

    // Clear existing content
    pageJumper.innerHTML = '';

    // Calculate step spacing (max 50 steps total)
    var maxSteps = Math.min(total, 50);
    var step = Math.ceil(total / maxSteps);
    if (step < 1) step = 1;

    for (var i = 0; i < total; i += step) {
      var dot = document.createElement('button');
      dot.className =
        'page-btn h-full flex-1 flex items-center justify-center text-[10px] leading-none cursor-pointer transition-all duration-200 rounded';
      dot.textContent = String(i + 1);
      dot.title = 'Page ' + (i + 1);
      dot.addEventListener('click', function(e) {
        e.stopPropagation();
        goToPage(parseInt(this.dataset.pageIndex, 10));
      });
      dot.dataset.pageIndex = i;

      // Numbered steps: passed/current = blue, upcoming = grey
      if (i <= current) {
        dot.className += ' bg-indigo-500 text-white shadow-[0_0_6px_1px_rgba(99,102,241,0.5)]';
      } else {
        dot.className += ' bg-neutral-700 text-neutral-300 hover:bg-neutral-600';
      }

      pageJumper.appendChild(dot);
    }

    // Progress fill + mini dots (shown when bars are hidden)
    var progressPct = ((current + 1) / total) * 100;
    pageProgress.style.width = `${progressPct}%`;
    updatePageDots(total);
  }

  // Mini clickable dots for the hidden-bars state (bottom indicator)
  function updatePageDots(total) {
    if (!pageDots || !total) return;
    pageDots.innerHTML = '';
    // Full width with larger dots
    var maxDots = Math.min(total, 50);
    var step = Math.ceil(total / maxDots);
    if (step < 1) step = 1;
    for (var i = 0; i < total; i += step) {
      var dot = document.createElement('button');
      dot.className =
        'w-4 h-4 rounded-full transition-all duration-150 flex items-center justify-center text-[9px] leading-none ' +
        (i === current
          ? 'bg-indigo-500 scale-125 shadow-[0_0_8px_2px_rgba(99,102,241,0.7)] text-white'
          : i <= current
            ? 'bg-indigo-600'
            : 'bg-neutral-600 hover:bg-neutral-500 text-neutral-300');
      dot.textContent = String(i + 1);
      dot.title = 'Page ' + (i + 1);
      dot.dataset.page = i;
      dot.addEventListener('click', function(e) {
        e.stopPropagation();
        goToPage(parseInt(this.dataset.page, 10));
      });
      pageDots.appendChild(dot);
    }
  }

  // Start with bars visible during loading so user can cancel
  setBarsVisible(true);

  // Unified page-jumper + dots indicator (shown when bars hidden)
  function updateUnifiedPageIndicator(total) {
    // Update the page-jumper step bar (shows when bars hidden)
    updatePageJumper();
  }

  function imageUrl(p, chapterID, prio) {
    var h = p.headers || {};
    var ref = h.Referer || h.referer || '';
    var prioParam = prio === 'high' ? '&prio=high' : '';
    return (
      '/image?pluginID=' +
      encodeURIComponent(pid) +
      '&url=' +
      encodeURIComponent(p.url) +
      '&mangaID=' +
      encodeURIComponent(mid) +
      '&chapterID=' +
      encodeURIComponent(chapterID || cid) +
      prioParam +
      (ref ? `&referer=${encodeURIComponent(ref)}` : '')
    );
  }

  function clampPan() {
    var d = viewBox();
    if (!d) return;
    var rect = canvas.getBoundingClientRect();
    var w = d.w * baseScale * zoom,
      h = d.h * baseScale * zoom;
    var cx = Math.max((rect.width - w) / 2, 0);
    var cy = Math.max((rect.height - h) / 2, 0);
    // Image pos = center + pan. Constrain so both edges are reachable.
    var minX = Math.min(0, rect.width - w - cx);
    var maxX = Math.max(0, -cx);
    var minY = Math.min(0, rect.height - h - cy);
    var maxY = Math.max(0, -cy);
    panX = Math.max(minX, Math.min(maxX, panX));
    panY = Math.max(minY, Math.min(maxY, panY));
  }

  function calcBaseScale() {
    var d = viewBox();
    if (!d) return;
    var rect = canvas.getBoundingClientRect();
    if (viewMode === 'fitWidth') baseScale = rect.width / d.w;
    else if (viewMode === 'fitHeight') baseScale = rect.height / d.h;
    else baseScale = 1; // original
  }

  function measure() {
    // Canvas must be sized while visible (hard-won lesson): call after paint.
    var rect = canvas.getBoundingClientRect();
    debugLog(`[reader] measure: canvas=${rect.width}x${rect.height}, DPR=${window.devicePixelRatio}`);
    if (rect.width === 0 || rect.height === 0) {
      debugWarn('[reader] measure: canvas has zero dimensions');
      return false;
    }
    dpr = window.devicePixelRatio || 1;
    canvas.width = Math.round(rect.width * dpr);
    canvas.height = Math.round(rect.height * dpr);
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    calcBaseScale();
    clampPan();
    return true;
  }

  function render() {
    var rect = canvas.getBoundingClientRect();
    ctx.clearRect(0, 0, rect.width, rect.height);
    var d = viewBox();
    if (!d) {
      debugWarn('[reader] render: no image loaded');
      return;
    }
    var w = d.w * baseScale * zoom,
      h = d.h * baseScale * zoom;
    debugLog(`[reader] render: ${d.w}x${d.h} -> ${Math.round(w)}x${Math.round(h)} on ${rect.width}x${rect.height}`);
    // Center when smaller, top-align when overflowing (top must never clip).
    var x = Math.max((rect.width - w) / 2, 0) + panX;
    var y = Math.max((rect.height - h) / 2, 0) + panY;
    ctx.imageSmoothingEnabled = smoothing;
    ctx.imageSmoothingQuality = 'high';
    // Both pages scale by the same factor. Side by side advances across, a
    // vertical stack advances down and is centred per row (the halves are the
    // same width, so the rows line up like one continuous page).
    var ims = viewPages();
    var cx = x,
      cy = y;
    for (var i = 0; i < ims.length; i++) {
      var iw = ims[i].width * baseScale * zoom,
        ih = ims[i].height * baseScale * zoom;
      if (spread === 'stack') {
        // Stacked: both halves are the same width, so centring each row lines
        // them up as one continuous page.
        ctx.drawImage(ims[i], x + (w - iw) / 2, cy, iw, ih);
        cy += ih;
      } else {
        ctx.drawImage(ims[i], cx, cy, iw, ih);
        cx += iw;
      }
    }
  }

  function showSpinner(v) {
    spinner.style.display = v ? 'flex' : 'none';
    loading = v;
  }

  function showError(v) {
    errPanel.style.display = v ? 'flex' : 'none';
  }

  function abortLoad() {
    if (loadAbortController) {
      loadAbortController.abort();
      loadAbortController = null;
    }
  }

  // Drop in-flight read-ahead for one direction (or both). The fetch rejects with
  // an AbortError, which warmImage swallows — abandonment is not a failure.
  function abortWarm(dir) {
    var c = warmCtrl[dir];
    if (c) {
      c.abort();
      warmCtrl[dir] = null;
    }
  }

  function abortAllWarm() {
    abortWarm('next');
    abortWarm('prev');
    warming = {};
  }

  // Read-ahead for one page. Fetching (rather than assigning an Image.src)
  // makes the request abortable; once the bytes are in the browser cache the
  // display path gets an Image built from the same URL, which resolves from
  // cache instead of re-requesting. The URL is identical to the display URL on
  // purpose — the browser cache keys on the full URL.
  function warmImage(store, key, url, dir) {
    if (store[key] || warming[key]) return;
    warming[key] = true;
    if (!warmCtrl[dir]) warmCtrl[dir] = new AbortController();
    var signal = warmCtrl[dir].signal;
    fetch(url, { signal: signal, credentials: 'same-origin' })
      .then((r) => {
        if (!r.ok) throw new Error(`HTTP ${r.status}`);
        return r.arrayBuffer();
      })
      .then(() => {
        const im = new Image();
        im.src = url;
        store[key] = im;
      })
      .catch(() => {
        /* abandoned or failed: the display path fetches on demand */
      })
      .finally(() => {
        delete warming[key];
      });
  }

  function showReaderError(msg) {
    var p = errPanel.querySelector('p');
    p.textContent = msg;
    p.classList.add('text-red-400');
    p.classList.remove('text-neutral-200');
    var btnRow = errPanel.querySelector('.flex.gap-2');
    if (btnRow) btnRow.style.display = '';
    showError(true);
  }
  function hideReaderError() {
    showError(false);
  }

  // End-of-series: friendly, persistent, no retry/skip (there's nothing to retry).
  function showEndOfSeries() {
    var p = errPanel.querySelector('p');
    p.textContent = "You've reached the end — no more chapters.";
    p.classList.remove('text-red-400');
    p.classList.add('text-neutral-200');
    var btnRow = errPanel.querySelector('.flex.gap-2');
    if (btnRow) btnRow.style.display = 'none';
    showError(true);
  }

  // ponytail: classify fetch errors into user-friendly text
  function friendlyError(err) {
    if (err instanceof TypeError) return 'Network error — check your connection';
    if (err instanceof SyntaxError) return 'Server returned invalid data';
    var m = err.message || String(err);
    if (/^HTTP \d+$/.test(m)) return `Server error (${m.slice(5)})`;
    return m;
  }

  // The server answers a failed reader-data request with the standard JSON error
  // envelope, and its "error" field carries the plugin's own message - the part
  // that says a page list could not be decrypted, or which upstream refused.
  // Collapsing that to "Server error (502)" hid every one of them, so the reader
  // just looked broken with nothing to act on.
  function errorFromBody(raw, status) {
    if (!raw) return `Server error (${status})`;
    var parsed = null;
    try {
      parsed = JSON.parse(raw);
    } catch {
      // Not the envelope. An HTML page here came from something between here and
      // the app, so show the status rather than dump markup into the panel.
      return raw.charAt(0) === '<' ? `Server error (${status})` : raw;
    }
    return parsed?.error ? String(parsed.error) : `Server error (${status})`;
  }

  function readerDataFailure(r) {
    return r
      .text()
      .catch(() => '')
      .then((body) => errorFromBody((body || '').trim(), r.status))
      .then((detail) =>
        // Peel the plumbing the host still adds, so what is left is the part a
        // reader can act on: the bridge, the runtime, and the plugin chunk
        // position say nothing they can do anything about.
        detail
          .replace(/^bridge:\s*/i, '')
          .replace(/^get page list:\s*/i, '')
          .replace(/^(lua|js|yaegi|wasm) plugin \S+\s+\w+:\s*/i, '')
          .replace(/\[string "[^"]+"\]:\d+:\s*/, '')
          .trim(),
      );
  }

  function showNotice(msg) {
    errPanel.querySelector('p').textContent = msg;
    errPanel.style.display = 'flex';
    setTimeout(() => {
      errPanel.style.display = 'none';
    }, 2000);
  }

  function updateCounter() {
    var total = pages.length || 1;
    // Two pages are on screen whenever a spread layout is active.
    var second = spread && !stripMode && current + 1 < total ? current + 2 : null;
    counter.textContent = second ? `${current + 1}-${second} / ${total}` : `${current + 1} / ${total}`;
    updatePageJumper();
  }

  function reportProgress() {
    var body = new URLSearchParams({
      pluginID: pid,
      mangaID: mid,
      chapterID: cid,
      page: String(current + 1),
    });
    fetch('/action/set-chapter-progress', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/x-www-form-urlencoded',
        'X-CSRF-Token': csrfToken,
      },
      body: body.toString(),
    })
      .then((resp) => {
        // A stale token (page rendered before a restart) is refused with
        // X-GoIsekai-Reload. This post is fire-and-forget, so without the reload
        // the progress write is silently dropped and the chapter never looks read.
        if (resp && !resp.ok && resp.headers.get('X-GoIsekai-Reload')) {
          // Same one-shot guard as alpine-components.js; duplicated because both
          // files are standalone IIFEs with no shared module.
          try {
            if (sessionStorage.getItem('gi_csrf_reloaded')) return;
            sessionStorage.setItem('gi_csrf_reloaded', '1');
          } catch {
            return;
          }
          location.reload();
        }
      })
      .catch(() => {});
  }

  // Read-ahead, server side. One request hands the whole window to the image
  // lane, which paces and orders it below the page on screen. The browser
  // cannot do that for itself: it holds at most six connections per origin and
  // the displayed page needs one of them, so per-page browser prefetch stalls
  // at ~5. Best effort — a dropped warm request just means the display path
  // fetches the page normally.
  function warmServer(chapterID, from, count) {
    if (count <= 0) return;
    fetch(`/api/reader-warm/${[pid, mid, chapterID].map(encodeURIComponent).join('/')}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ from: from, count: count }),
      credentials: 'same-origin',
    }).catch(() => {
      /* warm is best effort */
    });
  }

  function prefetch() {
    // Read-ahead: keep the next K pages warm in the browser cache.
    // Suwayomi-style spill: near the end of the chapter, the leftover
    // budget preloads the first pages of the NEXT chapter instead.
    var remaining = pages.length - 1 - current;
    var budget = Math.min(readAhead, remaining);
    for (let k = 1; k <= budget; k++) {
      const idx = current + k;
      warmImage(preloaded, idx, imageUrl(pages[idx], undefined, 'high'), 'next');
    }
    // Ask the server to warm the same window on its own lanes. The browser
    // requests above are capped at six connections per origin; the server is
    // not, and once it has the bytes in L2 those requests return instantly.
    warmServer(cid, current + 1, budget);
    var spill = readAhead - budget;
    // Evict bookkeeping for pages far behind the reading position; the
    // browser HTTP cache keeps the bytes, this only frees the Image handles.
    for (const idx in preloaded) {
      if (/^\d+$/.test(idx) && +idx < current - readAhead) delete preloaded[idx];
    }
    if (spill > 0 && nextChID) {
      if (nextPages) prefetchNext(spill);
      else warmNeighbor(nextChID, 'next');
    }
    // Warm the previous chapter as soon as this one is on screen. This used to
    // require current === 0, so retreating from the middle of a chapter found
    // prevPages empty and paid a cold reader-data fetch plus a cold image for
    // every page — which is exactly where Back feels slow.
    if (prevChID && !prevPages && !prevWarmTried && !loading) {
      prevWarmTried = true;
      warmNeighbor(prevChID, 'prev');
    }
  }

  function prefetchNext(n) {
    for (let k = 0; k < Math.min(n, nextPages.length); k++) {
      warmImage(preloaded, `n${k}`, imageUrl(nextPages[k], nextChID, 'high'), 'next');
    }
    warmServer(nextChID, 0, n);
  }

  function prefetchPrev(n) {
    for (let k = 0; k < Math.min(n, prevPages.length); k++) {
      warmImage(preloaded, `p${k}`, imageUrl(prevPages[k], prevChID, 'high'), 'prev');
    }
  }

  // Neighbor warmup: fetch the chapter's reader-data once so switchChapter can
  // commit instantly (page list + neighbors already in memory, images already
  // spilling into the browser cache).
  function warmNeighbor(chID, dir) {
    if (!chID || chID === cid) return;
    // Same controller as the neighbour's page read-ahead, so leaving the chapter
    // abandons the reader-data fetch too, not just the images.
    if (!warmCtrl[dir]) warmCtrl[dir] = new AbortController();
    fetch(`/api/reader-data/${[pid, mid, chID].map(encodeURIComponent).join('/')}`, {
      signal: warmCtrl[dir].signal,
      credentials: 'same-origin',
    })
      .then((r) => r.json())
      .then((d) => {
        if (!d || d.error) return;
        var list = d.pages || [];
        if (dir === 'next') {
          nextData = d;
          nextPages = list;
        } else {
          prevData = d;
          prevPages = list;
        }
        if (list.length) (dir === 'next' ? prefetchNext : prefetchPrev)(readAhead);
      })
      .catch(() => {
        /* neighbor warmup is best-effort */
      });
  }

  // ---- Vertical strip mode ------------------------------------------------
  // One <img> per page stacked vertically; wheel/space/keys scroll. When the
  // reader reaches the seam between chapter parts, the next chapter's page
  // list is fetched and its pages appended to the same column with a thin
  // divider, so split-up releases read continuously.
  var stripImgs = []; // pages currently in the column
  // Per-page chapter bookkeeping, parallel to stripImgs. The column spans every
  // chapter appended at the seam, so a page's position in stripImgs says nothing
  // about its page number within its own chapter — and `pages` only ever holds
  // one chapter's list.
  var stripMeta = []; // {chID, idx, pages, nextID, prevID, num, title}
  var stripChapters = []; // chapter ids already appended
  var curNum = 0, // loaded chapter's number/title, so a seam crossing can sync
    curTitle = ''; // the top bar back to the chapter it scrolled into

  // Read mode is one cycle button — the current mode is always a single letter
  // on screen:
  //   A  1 page per view (default)
  //   B  2 pages per view, stacked (one above the other, reads as one page)
  //   C  2 pages per view, side by side
  //   D  continuous (vertical strip)
  // Strip and spread are the same choice seen from two axes, so they are
  // mutually exclusive: picking any non-D mode leaves the strip column first.
  var READ_MODES = ['A', 'B', 'C', 'D'];

  function currentReadMode() {
    if (stripMode) return 'D';
    if (spread === 'stack') return 'B';
    if (spread === 'side') return 'C';
    return 'A';
  }

  function syncReadModeBtn() {
    var b = document.getElementById('btn-readmode');
    if (!b) return;
    var mode = currentReadMode();
    b.textContent = mode;
    b.classList.toggle('bg-indigo-600/20', mode !== 'A');
    b.classList.toggle('border-indigo-500/40', mode !== 'A');
  }

  function applyReadMode(mode) {
    if (mode === 'D') {
      if (!stripMode) enterStrip();
      syncReadModeBtn();
      return;
    }
    // Set the spread first: exitStrip redraws with whatever is in `spread`.
    spread = mode === 'B' ? 'stack' : mode === 'C' ? 'side' : '';
    localStorage.setItem(spreadKey, spread);
    savePrefs();
    if (stripMode) {
      exitStrip();
      return;
    }
    syncReadModeBtn();
    drawPage(current);
  }

  function enterStrip() {
    stripMode = true;
    localStorage.setItem(stripKey, '1');
    savePrefs();
    syncReadModeBtn();
    canvas.style.display = 'none';
    stripView.classList.remove('hidden');
    populateStrip();
    applyStripSizing();
    stripView.scrollTop = (current / Math.max(pages.length, 1)) * stripView.scrollHeight;
  }

  // (Re)build the strip column from the current chapter's pages. No-op when
  // already populated — chapter commits and seam appends manage it directly.
  function populateStrip() {
    if (stripImgs.length) return;
    stripChapters = [cid];
    pages.forEach((p, i) => {
      stripImgs.push(appendStripPage(p, i, cid));
      stripMeta.push({
        chID: cid,
        idx: i,
        pages: pages,
        nextID: nextChID,
        prevID: prevChID,
        num: curNum,
        title: curTitle,
      });
    });
    applyStripSizing();
  }

  function exitStrip() {
    stripMode = false;
    localStorage.setItem(stripKey, '0');
    savePrefs();
    stripView.classList.add('hidden');
    stripView.innerHTML = '';
    stripImgs = [];
    stripMeta = [];
    stripChapters = [];
    canvas.style.display = '';
    syncReadModeBtn();
    drawPage(current);
  }

  function appendStripPage(p, idx, chID) {
    var im = document.createElement('img');
    im.src = imageUrl(p, chID, 'low');
    im.loading = 'lazy';
    im.className = 'block select-none mx-auto';
    im.dataset.idx = idx;
    im.addEventListener('load', () => {
      if (stripMode && viewMode !== 'fitWidth') applyStripSizing();
    });
    stripView.appendChild(im);
    return im;
  }

  function appendStripDivider(num) {
    var d = document.createElement('div');
    d.className = 'flex items-center justify-center gap-3 py-6 text-xs text-neutral-500';
    d.textContent = `\u2014 Ch. ${num} \u2014`;
    stripView.appendChild(d);
  }

  // Append the next chapter's pages when the column bottom approaches.
  function maybeExtendStrip() {
    if (stripLoading) return;
    if (!nextChID || nextChID === cid || stripChapters.indexOf(nextChID) >= 0) return;
    var nearBottom = stripView.scrollTop + stripView.clientHeight > stripView.scrollHeight - 2500;
    if (!nearBottom) return;
    stripLoading = true;
    var warm =
      nextData && (nextData.pages || []).length
        ? Promise.resolve(nextData)
        : fetch(`/api/reader-data/${[pid, mid, nextChID].map(encodeURIComponent).join('/')}`).then(
            (r) => r.json(),
          );
    warm
      .then((d) => {
        stripLoading = false;
        if (!d || d.error || !(d.pages || []).length) return;
        if (stripChapters.indexOf(nextChID) >= 0) return;
        stripChapters.push(nextChID);
        appendStripDivider(d.chapterNum || '');
        // Rebase p{k}/n{k} image keys are per-chapter; column images carry
        // their own chapter id via imageUrl, so plain appends are safe. The
        // appended pages are numbered within THEIR chapter, not the column, so
        // each carries its own idx and page list.
        var chID = nextChID;
        d.pages.forEach((p, i) => {
          stripImgs.push(appendStripPage(p, i, chID));
          stripMeta.push({
            chID: chID,
            idx: i,
            pages: d.pages,
            nextID: d.nextChapterID || '',
            prevID: d.prevChapterID || '',
            num: d.chapterNum || 0,
            title: d.chapterTitle || '',
          });
        });
        applyStripSizing();
        // No adoption here: appended pages are below the viewport, so the reader
        // is still on the previous chapter's last page. trackStripProgress()
        // adopts each chapter when it actually scrolls into view, which is what
        // makes the counter and the title follow the page on screen.
      })
      .catch(() => {
        stripLoading = false;
      });
  }

  // Adopt a chapter the column scrolled into. The pages are already in the
  // column, so this only re-points the identity the rest of the reader reads
  // from — the counter's total, prev/next, the title and the progress write.
  function adoptSeam(meta) {
    cid = meta.chID;
    // Follow the chapter on screen: `pages` is what the counter, the jumper and
    // the progress write all read their total from, and the column holds several
    // chapters' lists at once.
    pages = meta.pages;
    curNum = meta.num;
    curTitle = meta.title;
    nextChID = meta.nextID;
    prevChID = meta.prevID;
    // The warm caches described the chapter just left, so drop them or the next
    // seam append would reuse those pages while labelling them as the new next.
    nextData = null;
    nextPages = null;
    prevData = null;
    prevPages = null;
    prevWarmTried = false;
    syncChapterNav({ chapterNum: meta.num, chapterTitle: meta.title });
    history.replaceState(
      {},
      '',
      `/view/read/${[pid, mid, cid].map(encodeURIComponent).join('/')}`,
    );
  }

  // Progress = the image crossing the viewport's vertical midpoint.
  //
  // The counter tracks the page number WITHIN the page's own chapter: `k` is a
  // position in the column, which keeps climbing across a seam, so the old
  // `k % pages.length` reported the previous chapter's last page for every page
  // of the next one (10, then 10 again, instead of resetting to 1).
  function trackStripProgress() {
    var acc = 0;
    for (let k = 0; k < stripImgs.length; k++) {
      acc += stripImgs[k].clientHeight + 8;
      if (acc >= stripView.scrollTop + stripView.clientHeight / 2) {
        var meta = stripMeta[k];
        if (!meta) break;
        // Swallow the seam: scrolled into a chapter that is not the one the rest
        // of the reader state points at. Adopt it so prev/next, the title and
        // the progress write all refer to the chapter actually on screen.
        var adopted = meta.chID !== cid;
        if (adopted) adoptSeam(meta);
        // Report on a chapter change even when the page number is unchanged: a
        // fast scroll can land on page 3 of the next chapter while `current` is
        // still 3 from the previous one, and that page must still be recorded.
        if (adopted || current !== meta.idx) {
          current = meta.idx;
          updateCounter();
          reportProgress();
        }
        break;
      }
    }
  }
  // The scroll handler must not run per scroll event: WebKit fires several per
  // frame, and trackStripProgress() walks every loaded image reading
  // .clientHeight (a forced layout) — doing that unthrottled, on a column that
  // keeps growing across chapter seams, is what made the strip scroll stutter.
  // Collapse the burst into one pass per animation frame instead.
  var stripScrollPending = false;
  stripView.addEventListener('scroll', () => {
    if (stripScrollPending) return;
    stripScrollPending = true;
    requestAnimationFrame(() => {
      stripScrollPending = false;
      maybeExtendStrip();
      trackStripProgress();
    });
  });
  stripView.addEventListener('click', () => setBarsVisible(!barsVisible));

  // Load pages[idx] into one slot of the view. Reuses the preloaded Image when
  // the bytes are already there; `done` fires once the slot is ready so a
  // spread can wait for both halves before the first paint.
  function loadSlot(idx, assign, done) {
    var cached = preloaded[idx];
    if (cached?.complete && cached.naturalWidth > 0) {
      debugLog(`[reader] Using cached image ${cached.naturalWidth}x${cached.naturalHeight}`);
      assign(cached);
      done();
      return;
    }
    var url = imageUrl(pages[idx], undefined, 'high');
    debugLog(`[reader] Image URL: ${url.substring(0, 80)}...`);
    var im = new Image();
    im.onload = () => {
      debugLog(`[reader] Image loaded: ${im.naturalWidth}x${im.naturalHeight}`);
      preloaded[idx] = im;
      assign(im);
      done();
    };
    im.onerror = () => {
      debugError(`[reader] Image load failed: ${url.substring(0, 80)}`);
      imgFails++;
      if (imgFails >= 4) {
        showReaderError('Failed to load image — try Retry');
        return;
      } // give up — manual Retry still available
      // At-home nodes flake transiently: retry the same URL once, then
      // re-fetch the page list (a fresh request may land on a healthy node).
      var reFetch = imgFails % 2 === 0;
      setTimeout(() => {
        if (reFetch) loadChapter(true);
        else drawPage(current);
      }, 1200);
    };
    im.src = url;
  }

  function drawPage(i) {
    debugLog(`[reader] drawPage: ${i} of ${pages.length}`);
    current = i;
    showError(false);
    updateCounter();
    // Hide spinner immediately before image load so user sees page faster
    showSpinner(false);
    reportProgress();
    prefetch();
    if (stripMode && !stripImgs.length) populateStrip();
    if (stripMode) stripView.scrollTop = (i / Math.max(pages.length, 1)) * stripView.scrollHeight;
    // Spread view shows the facing page too. A trailing odd page stands alone
    // rather than pairing with the next chapter's first. Strip mode owns its own
    // column, so it never loads a second page here.
    var secondIdx = spread && !stripMode && i + 1 < pages.length ? i + 1 : -1;
    img = null;
    img2 = null;
    var pending = secondIdx >= 0 ? 2 : 1;
    showSpinner(true);
    var done = () => {
      if (--pending > 0) return;
      showSpinner(false);
      afterImage();
    };
    loadSlot(i, (im) => { img = im; }, done);
    if (secondIdx >= 0) loadSlot(secondIdx, (im) => { img2 = im; }, done);
  }

  // rAF never fires in a hidden/background tab (Chrome throttles it), which
  // left the first draw pending forever. setTimeout(0) is the safe fallback.
  function nextFrame(fn) {
    var done = false;
    var run = () => {
      if (done) return;
      done = true;
      fn();
    };
    requestAnimationFrame(run);
    setTimeout(run, 50); // hidden tabs never fire rAF — this is the real path
  }

  function afterImage() {
    imgFails = 0;
    if (initialLoad) {
      initialLoad = false;
      debugLog('[reader] First image loaded, hiding bars');
      setBarsVisible(false);
    }
    if (!measure()) {
      debugWarn('[reader] measure() failed, retrying in nextFrame');
      nextFrame(() => {
        measure();
        restoreZoom();
      });
      return;
    }
    debugLog('[reader] Rendering image');
    restoreZoom();
  }

  function resetZoom() {
    zoom = 1;
    panX = 0;
    panY = 0;
    localStorage.setItem(`gi_zoom_${viewMode}.${scope}`, '1');
    savePrefs();
    calcBaseScale();
    render();
  }
  function restoreZoom() {
    // Local mirror first: every zoom change writes it immediately, while the
    // server blob is only as fresh as the last settled POST. Preferring the
    // server value made every page turn snap the zoom back to whatever the
    // chapter had opened with.
    var srvZoom = hasSrv('zoom') && srvPrefs.zoomMode === viewMode ? parseFloat(srvPrefs.zoom) : NaN;
    zoom =
      parseFloat(localStorage.getItem(`gi_zoom_${viewMode}.${scope}`)) ||
      parseFloat(localStorage.getItem(`gi_zoom_${viewMode}`)) ||
      srvZoom ||
      1;
    panX = 0;
    panY = 0;
    calcBaseScale();
    if (stripMode) applyStripSizing();
    else render();
  }

  // Strip pages size off viewMode + zoom: fit-width fills the column, 1:1
  // uses natural width capped to the viewport (no horizontal scroll),
  // fit-height scales per page to the visible height. zoom multiplies on top.
  function applyStripSizing() {
    var vw = canvas.clientWidth || window.innerWidth;
    var vh = canvas.clientHeight || window.innerHeight;
    for (let k = 0; k < stripImgs.length; k++) {
      const im = stripImgs[k];
      if (viewMode === 'fitHeight' && im.naturalHeight > 0) {
        const s = (vh / im.naturalHeight) * zoom;
        im.style.width = `${im.naturalWidth * s}px`;
      } else if (viewMode === 'original') {
        im.style.width = `${Math.min(im.naturalWidth || vw, vw) * zoom}px`;
      } else {
        im.style.width = `${vw * zoom}px`;
      }
    }
  }
  window.addEventListener('resize', () => {
    if (stripMode) applyStripSizing();
  });

  function goToPage(i) {
    debugLog(`goToPage: ${i} (of ${pages.length})`);
    if (i < 0 || i >= pages.length) {
      debugWarn(`goToPage: invalid page index ${i}, pages.length=${pages.length}`);
      return;
    }
    drawPage(i);
  }

  // One step is one view: two pages with a spread layout, one otherwise.
  function stepSize() {
    return spread ? 2 : 1;
  }

  function next() {
    if (stripMode) {
      debugLog('next: strip mode, scrolling');
      return;
    }
    if (current < pages.length - 1) {
      var target = Math.min(current + stepSize(), pages.length - 1);
      debugLog(`next: page ${current + 1} -> ${target + 1}`);
      goToPage(target);
      return;
    }
    // Last page → auto-advance to next chapter, unless we're at the end of the
    // series (no next chapter, or backend returned the current chapter as next).
    if (nextChID && nextChID !== cid) {
      debugLog(`next: last page, switching to next chapter: ${nextChID}`);
      switchChapter(nextChID, 1);
      return;
    }
    debugWarn('next: no more chapters');
    showEndOfSeries();
  }
  function prev() {
    if (stripMode) {
      debugLog('prev: strip mode, scrolling');
      return;
    }
    if (current > 0) {
      var target = Math.max(current - stepSize(), 0);
      debugLog(`prev: page ${current + 1} -> ${target + 1}`);
      goToPage(target);
      return;
    }
    // First page → auto-retreat to prev chapter last page
    if (prevChID) {
      debugLog(`prev: first page, switching to prev chapter: ${prevChID}`);
      switchChapter(prevChID, 'last');
      return;
    }
    debugWarn('prev: no previous chapter');
    showNotice('First page');
  }

  // Fetch-swap navigation: load a chapter in place without a full page reload.
  // A warmed neighbor commits instantly — no fetch, no spinner — so switching
  // chapters feels like switching pages.
  function switchChapter(targetCID, targetPage) {
    if (loading) {
      debugWarn(`switchChapter: already loading, ignoring ${targetCID}`);
      return;
    }
    debugLog(`switchChapter: ${targetCID}, page=${targetPage}`);
    if (targetCID === nextChID && nextData && (nextData.pages || []).length) {
      debugLog('switchChapter: using warmed next chapter');
      commitChapter(nextData, targetCID, targetPage);
      return;
    }
    if (targetCID === prevChID && prevData && (prevData.pages || []).length) {
      debugLog('switchChapter: using warmed prev chapter');
      commitChapter(prevData, targetCID, targetPage);
      return;
    }
    debugLog('switchChapter: cold fetch required');
    showSpinner(true);
    _lastFailedRetry = () => {
      switchChapter(targetCID, targetPage);
    };
    fetch(`/api/reader-data/${[pid, mid, targetCID].map(encodeURIComponent).join('/')}`)
      .then((r) => {
        debugLog(`switchChapter: response status=${r.status}`);
        if (!r.ok) return readerDataFailure(r).then((msg) => Promise.reject(new Error(msg)));
        return r.json();
      })
      .then((data) => {
        if (data.error) {
          debugError(`switchChapter: API error: ${data.error}`);
          showReaderError(data.error);
          showSpinner(false);
          return;
        }
        if (!(data.pages || []).length) {
          debugWarn('switchChapter: empty page list');
          showReaderError(
            'This chapter has no pages. The plugin returned an empty page list — ' +
              'the site may have changed how it names them. Check the log for the reason.',
          );
          showSpinner(false);
          return;
        }
        debugLog(`switchChapter: loaded ${data.pages.length} pages`);
        commitChapter(data, targetCID, targetPage);
      })
      .catch((err) => {
        debugError(`switchChapter: ${err.message}`);
        showReaderError(friendlyError(err));
        showSpinner(false);
      });
  }

  // Commit a chapter's fetched-or-warmed state and draw the target page.
  function commitChapter(data, targetCID, targetPage) {
    debugLog(`commitChapter: cid=${targetCID}, page=${targetPage}, pages=${data.pages?.length}`);
    // Leaving the old chapter: drop its in-flight read-ahead before the page
    // bookkeeping is reset, so nothing keeps warming bytes nobody will read.
    abortAllWarm();
    cid = targetCID;
    pages = data.pages;
    curNum = data.chapterNum || 0;
    curTitle = data.chapterTitle || '';
    nextChID = data.nextChapterID || '';
    prevChID = data.prevChapterID || '';
    preloaded = {};
    nextData = null;
    nextPages = null;
    prevData = null;
    prevPages = null;
    prevWarmTried = false;
    imgFails = 0;
    img = null;
    img2 = null;
    panX = 0;
    panY = 0;
    _lastFailedRetry = null;
    syncChapterNav(data);
    // Update the URL without a reload; back/forward still work.
    var url = `/view/read/${[pid, mid, targetCID].map(encodeURIComponent).join('/')}`;
    history.pushState({}, '', url + (targetPage === 'last' ? '?page=last' : ''));
    var initial = targetPage === 'last' ? pages.length - 1 : 0;
    showSpinner(false);
    if (stripMode) {
      for (const im of stripImgs) im.remove();
      stripImgs = [];
      stripMeta = [];
      stripChapters = [];
      populateStrip();
      stripView.scrollTop = targetPage === 'last' ? stripView.scrollHeight : 0;
    } else drawPage(initial);
  }

  // Reflect chapter state in the top-bar title + prev/next chapter controls.
  function syncChapterNav(data) {
    var t = document.getElementById('chapter-title');
    if (t) {
      const num = data.chapterNum || 0,
        title = data.chapterTitle || '';
      t.textContent = num > 0 ? `Ch. ${num}${title ? ` — ${title}` : ''}` : title;
    }
    var pb = document.getElementById('btn-prev-ch'),
      nb = document.getElementById('btn-next-ch');
    if (pb) {
      pb.style.display = prevChID ? '' : 'none';
      pb.href = prevChID
        ? `/view/read/${[pid, mid, prevChID].map(encodeURIComponent).join('/')}`
        : '#';
    }
    if (nb) {
      nb.style.display = nextChID ? '' : 'none';
      nb.href = nextChID
        ? `/view/read/${[pid, mid, nextChID].map(encodeURIComponent).join('/')}`
        : '#';
    }
  }

  // Zoom controls (cursor-anchored via wheel; buttons zoom about center).
  function zoomBy(factor, cx, cy) {
    if (stripMode) {
      zoom = Math.max(0.2, Math.min(5, zoom * factor));
      localStorage.setItem(`gi_zoom_${viewMode}.${scope}`, String(zoom));
      savePrefs();
      applyStripSizing();
      return;
    }
    var rect = canvas.getBoundingClientRect();
    var mx = cx === undefined ? rect.width / 2 : cx - rect.left;
    var my = cy === undefined ? rect.height / 2 : cy - rect.top;
    var nz = Math.max(0.2, Math.min(5, zoom * factor));
    var sc = nz / zoom;
    panX = mx - (mx - panX) * sc;
    panY = my - (my - panY) * sc;
    zoom = nz;
    localStorage.setItem(`gi_zoom_${viewMode}.${scope}`, String(zoom));
    savePrefs();
    clampPan();
    render();
  }

  // Events — wheel on document so it works over click zones + toolbars.
  document.addEventListener(
    'wheel',
    (e) => {
      e.preventDefault();
      if (stripMode) {
        stripView.scrollTop += e.deltaY;
        return;
      }
      if (e.ctrlKey) {
        zoomBy(e.deltaY > 0 ? 0.9 : 1.1, e.clientX, e.clientY);
        return;
      }
      panX -= e.deltaX;
      panY -= e.deltaY;
      clampPan();
      render();
    },
    { passive: false },
  );

  var dragging = false,
    moved = false,
    sx = 0,
    sy = 0;
  canvas.addEventListener('mousedown', (e) => {
    if (e.button !== 0) return;
    dragging = true;
    moved = false;
    sx = e.clientX;
    sy = e.clientY;
    canvas.style.cursor = 'grabbing';
  });
  window.addEventListener('mousemove', (e) => {
    if (!dragging) return;
    var dx = e.clientX - sx,
      dy = e.clientY - sy;
    if (Math.abs(dx) > 5 || Math.abs(dy) > 5) moved = true;
    if (moved) {
      panX += dx;
      panY += dy;
      sx = e.clientX;
      sy = e.clientY;
      clampPan();
      render();
    }
  });
  window.addEventListener('mouseup', () => {
    dragging = false;
    canvas.style.cursor = 'default';
  });
  canvas.addEventListener('dblclick', resetZoom);

  var prevCh = prevChID
    ? `/view/read/${[pid, mid, prevChID].map(encodeURIComponent).join('/')}`
    : null;
  var nextCh = nextChID
    ? `/view/read/${[pid, mid, nextChID].map(encodeURIComponent).join('/')}`
    : null;

  document.getElementById('zone-left').addEventListener('click', () => {
    if (!moved) (direction === 'rtl' ? next : prev)();
  });
  document.getElementById('zone-right').addEventListener('click', () => {
    if (!moved) (direction === 'rtl' ? prev : next)();
  });
  var zt = document.getElementById('zone-top');
  if (prevCh)
    zt.addEventListener('click', () => {
      if (!moved) switchChapter(prevChID, 'last');
    });
  var zb = document.getElementById('zone-bottom');
  if (nextCh)
    zb.addEventListener('click', () => {
      if (!moved) switchChapter(nextChID, 1);
    });
  zoneCenter.addEventListener('click', () => {
    if (!moved) setBarsVisible(!barsVisible);
  });

  // Prev/next chapter buttons in the bottom bar — fetch-swap, not full reload.
  var pb = document.getElementById('btn-prev-ch');
  if (pb)
    pb.addEventListener('click', (e) => {
      e.preventDefault();
      if (prevChID) switchChapter(prevChID, 'last');
    });
  var nb = document.getElementById('btn-next-ch');
  if (nb)
    nb.addEventListener('click', (e) => {
      e.preventDefault();
      if (nextChID) switchChapter(nextChID, 1);
    });

  document.getElementById('btn-prev-page').addEventListener('click', prev);
  document.getElementById('btn-next-page').addEventListener('click', next);
  document.getElementById('btn-retry').addEventListener('click', () => {
    imgFails = 0;
    hideReaderError();
    if (_lastFailedRetry) _lastFailedRetry();
    else loadChapter(true);
  });
  document.getElementById('btn-skip').addEventListener('click', next);
  document.getElementById('btn-zoom-in').addEventListener('click', () => {
    zoomBy(1.2);
  });
  document.getElementById('btn-zoom-out').addEventListener('click', () => {
    zoomBy(1 / 1.2);
  });

  var readModeBtn = document.getElementById('btn-readmode');
  if (readModeBtn)
    readModeBtn.addEventListener('click', () => {
      var i = READ_MODES.indexOf(currentReadMode());
      applyReadMode(READ_MODES[(i + 1) % READ_MODES.length]);
    });
  syncReadModeBtn();

  var fitBtn = document.getElementById('btn-fit');
  var fitLabels = { fitWidth: 'Fit W', fitHeight: 'Fit H', original: '1:1' };
  var fitOrder = ['fitWidth', 'fitHeight', 'original'];
  function syncFitLabel() {
    fitBtn.textContent = fitLabels[viewMode];
  }
  fitBtn.addEventListener('click', () => {
    viewMode = fitOrder[(fitOrder.indexOf(viewMode) + 1) % fitOrder.length];
    localStorage.setItem(viewKey, viewMode);
    savePrefs();
    syncFitLabel();
    restoreZoom();
  });
  syncFitLabel();

  var dirBtn = document.getElementById('btn-dir');
  function syncDirLabel() {
    dirBtn.textContent = direction.toUpperCase();
  }
  dirBtn.addEventListener('click', () => {
    direction = direction === 'ltr' ? 'rtl' : 'ltr';
    localStorage.setItem(dirKey, direction);
    savePrefs();
    syncDirLabel();
  });
  syncDirLabel();

  // Chapter jumper dropdown
  if (chapterJumperBtn && chapterMenu) {
    chapterJumperBtn.addEventListener('click', function(e) {
      e.stopPropagation();
      chapterMenu.classList.toggle('hidden');
    });

    // Close menu when clicking outside
    document.addEventListener('click', function(e) {
      if (!chapterJumperBtn.contains(e.target) && !chapterMenu.contains(e.target)) {
        chapterMenu.classList.add('hidden');
      }
    });

    // Handle chapter selection
    chapterMenu.addEventListener('click', function(e) {
      var btn = e.target.closest('button[data-chapter-id]');
      if (btn) {
        var targetCID = btn.dataset.chapterId;
        if (targetCID && targetCID !== cid) {
          chapterMenu.classList.add('hidden');
          window.location.href = `/view/read/${encodeURIComponent(pid)}/${encodeURIComponent(mid)}/${encodeURIComponent(targetCID)}`;
        }
      }
    });
  }

  document.addEventListener('keydown', (e) => {
    if (e.target.matches('input, select, textarea')) return;
    if (e.key === 'ArrowRight' || e.key === 'd') {
      e.preventDefault();
      (direction === 'rtl' ? prev : next)();
    } else if (e.key === 'ArrowLeft' || e.key === 'a') {
      e.preventDefault();
      (direction === 'rtl' ? next : prev)();
    } else if (e.key === ' ') {
      e.preventDefault();
      if (stripMode) stripView.scrollTop += canvas.clientHeight * 0.9;
      else next();
    } else if (e.key === 'Escape') {
      e.preventDefault();
      window.location.href = `/view/manga/${encodeURIComponent(pid)}/${encodeURIComponent(mid)}`;
    } else if (e.key === 'Home') {
      e.preventDefault();
      goToPage(0);
    } else if (e.key === 'End') {
      e.preventDefault();
      goToPage(pages.length - 1);
    } else if (e.key === 'ArrowUp' || e.key === 'PageUp') {
      e.preventDefault();
      panY += e.key === 'PageUp' ? canvas.clientHeight * 0.8 : 80;
      clampPan();
      render();
    } else if (e.key === 'ArrowDown' || e.key === 'PageDown') {
      e.preventDefault();
      panY -= e.key === 'PageDown' ? canvas.clientHeight * 0.8 : 80;
      clampPan();
      render();
    } else if (e.key === 'r' || e.key === 'R') {
      e.preventDefault();
      resetZoom();
    }
  });

  // Resize: recompute base scale only — keep user zoom/pan.
  window.addEventListener('resize', () => {
    if (measure()) render();
  });

  // Back/forward: the URL changed via pushState — reload that chapter in place.
  window.addEventListener('popstate', () => {
    var parts = location.pathname.split('/');
    // /view/read/{pluginID}/{mangaID}/{chapterID} → last segment is the chapter ID.
    var target = decodeURIComponent(parts[parts.length - 1] || '');
    if (target && target !== cid) {
      const page = new URLSearchParams(location.search).get('page');
      switchChapter(target, page === 'last' ? 'last' : 1);
    }
  });

  // Boot: fetch page data, then first draw after the canvas is visible.
  function loadChapter(resume) {
    debugLog('[reader] loadChapter:', { pid, mid, cid, resume });
    preloaded = {}; // stale at-home URLs 404 — drop cached Image objects
    _lastFailedRetry = () => {
      loadChapter(resume);
    };
    loadAbortController = new AbortController();
    fetch(`/api/reader-data/${[pid, mid, cid].map(encodeURIComponent).join('/')}`, {
      signal: loadAbortController.signal,
    })
      .then((r) => {
        debugLog('[reader] API response:', { ok: r.ok, status: r.status });
        if (!r.ok) throw new Error(`HTTP ${r.status}`);
        return r.json();
      })
      .then((data) => {
        debugLog('[reader] API data:', { chapterID: data.chapterID, pages: data.pages?.length, error: data.error });
        if (data.error) {
          debugError('[reader] API error:', data.error);
          showReaderError(data.error);
          return;
        }
        pages = data.pages || [];
        if (!pages.length) {
          debugWarn('[reader] No pages found');
          showReaderError('Empty chapter (plugin failed to fetch pages) — try Retry');
          return;
        }
        // Seed the chapter identity the seam bookkeeping restores when the
        // column scrolls back up into this chapter.
        curNum = data.chapterNum || 0;
        curTitle = data.chapterTitle || '';
        debugLog('[reader] Pages loaded:', pages.length);
        _lastFailedRetry = null;
        nextFrame(() => {
          // Honor ?page=N (or ?page=last from chapter advance); a retry keeps
          // the page the reader was already on.
          var sp = resume ? current + 1 : startPage;
          if (window.location.search.indexOf('page=last') >= 0) sp = pages.length;
          var initial = sp ? Math.max(1, Math.min(sp, pages.length)) - 1 : 0;
          debugLog('[reader] Drawing initial page:', initial);
          if (stripMode) {
            canvas.style.display = 'none';
            stripView.classList.remove('hidden');
          }
          current = initial;
          updateCounter();
          drawPage(initial);
        });
      })
      .catch((err) => {
        debugError(`[reader] loadChapter fetch failed: ${err.name} - ${err.message}`);
        if (err.name === 'AbortError') return;
        showReaderError(friendlyError(err));
      });
  }
  // Abort fetch on back navigation (clicking manga title Back link)
  document.querySelector('#top-bar a[href^="/view/manga/"]').addEventListener('click', () => {
    abortLoad();
  });

  loadChapter();
})();
