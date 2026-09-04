(() => {
  var root = document.getElementById('reader');
  var canvas = document.getElementById('page-canvas');
  var ctx = canvas.getContext('2d');
  var spinner = document.getElementById('spinner');
  var errPanel = document.getElementById('error-panel');
  var counter = document.getElementById('page-counter');
  var slider = document.getElementById('page-slider');

  // State
  var pid = root.dataset.pluginId,
    mid = root.dataset.mangaId,
    cid = root.dataset.chapterId;
  var pages = [],
    current = 0,
    img = null,
    loading = false,
    _lastFailedRetry = null;
  var viewMode = localStorage.getItem('gi_viewMode') || 'fitWidth';
  var direction = localStorage.getItem('gi_direction') || 'ltr';
  var smoothing = (localStorage.getItem('gi_renderMode') || 'smooth') === 'smooth';
  var zoom = 1,
    panX = 0,
    panY = 0,
    baseScale = 1,
    dpr = 1;
  var preloaded = {}; // page index -> Image
  var nextPages = null; // next chapter page list — fetched lazily for read-ahead spill
  var imgFails = 0; // consecutive image-load failures (at-home nodes flake transiently)
  var readAhead = Math.max(
    0,
    Math.min(10, parseInt(localStorage.getItem('gi_readAhead'), 10) || 3),
  );
  var startPage = parseInt(new URLSearchParams(window.location.search).get('page'), 10);
  var nextChID = root.dataset.nextChapterId || '';
  var prevChID = root.dataset.prevChapterId || '';

  // Auto-hide bars state
  var barsVisible = true;
  var topBar = document.getElementById('top-bar');
  var bottomBar = document.getElementById('bottom-bar');
  var progressLine = document.getElementById('progress-line');
  var zoneCenter = document.getElementById('zone-center');

  function setBarsVisible(visible) {
    barsVisible = visible;
    if (visible) {
      topBar.style.transform = 'translateY(0)';
      bottomBar.style.transform = 'translateY(0)';
      progressLine.style.opacity = '0';
    } else {
      topBar.style.transform = 'translateY(-100%)';
      bottomBar.style.transform = 'translateY(100%)';
      progressLine.style.opacity = '1';
    }
  }

  function updateProgressLine() {
    var total = pages.length || 1;
    var pct = ((current + 1) / total) * 100;
    progressLine.style.width = `${pct}%`;
  }

  // Start with bars hidden
  setBarsVisible(false);

  function imageUrl(p, chapterID) {
    var h = p.headers || {};
    var ref = h.Referer || h.referer || '';
    return (
      '/image?pluginID=' +
      encodeURIComponent(pid) +
      '&url=' +
      encodeURIComponent(p.url) +
      '&mangaID=' +
      encodeURIComponent(mid) +
      '&chapterID=' +
      encodeURIComponent(chapterID || cid) +
      (ref ? `&referer=${encodeURIComponent(ref)}` : '')
    );
  }

  function clampPan() {
    if (!img) return;
    var rect = canvas.getBoundingClientRect();
    var w = img.width * baseScale * zoom,
      h = img.height * baseScale * zoom;
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
    if (!img) return;
    var rect = canvas.getBoundingClientRect();
    if (viewMode === 'fitWidth') baseScale = rect.width / img.width;
    else if (viewMode === 'fitHeight') baseScale = rect.height / img.height;
    else baseScale = 1; // original
  }

  function measure() {
    // Canvas must be sized while visible (hard-won lesson): call after paint.
    var rect = canvas.getBoundingClientRect();
    if (rect.width === 0 || rect.height === 0) return false;
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
    if (!img) return;
    var w = img.width * baseScale * zoom,
      h = img.height * baseScale * zoom;
    // Center when smaller, top-align when overflowing (top must never clip).
    var x = Math.max((rect.width - w) / 2, 0) + panX;
    var y = Math.max((rect.height - h) / 2, 0) + panY;
    ctx.imageSmoothingEnabled = smoothing;
    ctx.imageSmoothingQuality = 'high';
    ctx.drawImage(img, x, y, w, h);
  }

  function showSpinner(v) {
    spinner.style.display = v ? 'flex' : 'none';
    loading = v;
  }

  function showError(v) {
    errPanel.style.display = v ? 'flex' : 'none';
  }
  function showReaderError(msg) {
    errPanel.querySelector('p').textContent = msg;
    showError(true);
  }
  function hideReaderError() {
    showError(false);
  }

  // ponytail: classify fetch errors into user-friendly text
  function friendlyError(err) {
    if (err instanceof TypeError) return 'Network error — check your connection';
    if (err instanceof SyntaxError) return 'Server returned invalid data';
    var m = err.message || String(err);
    if (/^HTTP \d+$/.test(m)) return 'Server error (' + m.slice(5) + ')';
    return m;
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
    counter.textContent = `${current + 1} / ${total}`;
    slider.max = total;
    slider.value = current + 1;
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
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: body.toString(),
    }).catch(() => {});
  }

  function prefetch() {
    // Read-ahead: keep the next K pages warm in the browser cache.
    // Suwayomi-style spill: near the end of the chapter, the leftover
    // budget preloads the first pages of the NEXT chapter instead.
    var remaining = pages.length - 1 - current;
    var budget = Math.min(readAhead, remaining);
    for (let k = 1; k <= budget; k++) {
      const idx = current + k;
      if (!preloaded[idx]) {
        const im = new Image();
        im.src = imageUrl(pages[idx]);
        preloaded[idx] = im;
      }
    }
    var spill = readAhead - budget;
    if (spill > 0 && nextChID) {
      if (nextPages) {
        prefetchNext(spill);
        return;
      }
      // Lazily fetch the next chapter's page list once, then spill into it.
      fetch(`/api/reader-data/${[pid, mid, nextChID].map(encodeURIComponent).join('/')}`)
        .then((r) => r.json())
        .then((d) => {
          nextPages = d?.pages || [];
          if (nextPages.length) prefetchNext(spill);
        })
        .catch(() => {
          /* next-chapter warmup is best-effort */
        });
    }
  }

  function prefetchNext(n) {
    for (let k = 0; k < Math.min(n, nextPages.length); k++) {
      if (!preloaded[`n${k}`]) {
        const im = new Image();
        im.src = imageUrl(nextPages[k], nextChID);
        preloaded[`n${k}`] = im;
      }
    }
  }

  function drawPage(i) {
    current = i;
    showError(false);
    updateCounter();
    updateProgressLine();
    reportProgress();
    prefetch();
    var url = imageUrl(pages[i]);
    var cached = preloaded[i];
    if (cached?.complete && cached.naturalWidth > 0) {
      img = cached;
      afterImage();
      return;
    }
    showSpinner(true);
    var im = new Image();
    im.onload = () => {
      img = im;
      preloaded[i] = im;
      showSpinner(false);
      afterImage();
    };
    im.onerror = () => {
      showSpinner(false);
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
    if (!measure()) {
      nextFrame(() => {
        measure();
        restoreZoom();
      });
      return;
    }
    restoreZoom();
  }

  function resetZoom() {
    zoom = 1;
    panX = 0;
    panY = 0;
    localStorage.setItem(`gi_zoom_${viewMode}`, '1');
    calcBaseScale();
    render();
  }
  function restoreZoom() {
    zoom = parseFloat(localStorage.getItem(`gi_zoom_${viewMode}`)) || 1;
    panX = 0;
    panY = 0;
    calcBaseScale();
    render();
  }

  function goToPage(i) {
    if (i < 0 || i >= pages.length) return;
    drawPage(i);
  }

  function next() {
    if (current < pages.length - 1) {
      goToPage(current + 1);
      return;
    }
    // Last page → auto-advance to next chapter
    if (nextChID) {
      switchChapter(nextChID, 1);
      return;
    }
    showNotice('Last page');
  }
  function prev() {
    if (current > 0) {
      goToPage(current - 1);
      return;
    }
    // First page → auto-retreat to prev chapter last page
    if (prevChID) {
      switchChapter(prevChID, 'last');
      return;
    }
    showNotice('First page');
  }

  // Fetch-swap navigation: load a chapter in place without a full page reload.
  function switchChapter(targetCID, targetPage) {
    if (loading) return;
    showSpinner(true);
    _lastFailedRetry = function() { switchChapter(targetCID, targetPage); };
    fetch(`/api/reader-data/${[pid, mid, targetCID].map(encodeURIComponent).join('/')}`)
      .then((r) => {
        if (!r.ok) throw new Error(`HTTP ${r.status}`);
        return r.json();
      })
      .then((data) => {
        if (data.error) {
          showReaderError(data.error);
          showSpinner(false);
          return;
        }
        var newPages = data.pages || [];
        if (!newPages.length) {
          showReaderError('Empty chapter (plugin failed to fetch pages) — try Retry');
          showSpinner(false);
          return;
        }
        // Commit new chapter state.
        cid = targetCID;
        pages = newPages;
        nextChID = data.nextChapterID || '';
        prevChID = data.prevChapterID || '';
        preloaded = {};
        nextPages = null;
        imgFails = 0;
        img = null;
        panX = 0;
        panY = 0;
        _lastFailedRetry = null;
        syncChapterNav(data);
        // Update the URL without a reload; back/forward still work.
        var url = `/view/read/${[pid, mid, targetCID].map(encodeURIComponent).join('/')}`;
        history.pushState({}, '', url + (targetPage === 'last' ? '?page=last' : ''));
        var initial = targetPage === 'last' ? pages.length - 1 : 0;
        showSpinner(false);
        drawPage(initial);
      })
      .catch((err) => {
        showReaderError(friendlyError(err));
        showSpinner(false);
      });
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
    var rect = canvas.getBoundingClientRect();
    var mx = cx === undefined ? rect.width / 2 : cx - rect.left;
    var my = cy === undefined ? rect.height / 2 : cy - rect.top;
    var nz = Math.max(0.2, Math.min(5, zoom * factor));
    var sc = nz / zoom;
    panX = mx - (mx - panX) * sc;
    panY = my - (my - panY) * sc;
    zoom = nz;
    localStorage.setItem(`gi_zoom_${viewMode}`, String(zoom));
    clampPan();
    render();
  }

  // Events — wheel on document so it works over click zones + toolbars.
  document.addEventListener(
    'wheel',
    (e) => {
      e.preventDefault();
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

  var fitBtn = document.getElementById('btn-fit');
  var fitLabels = { fitWidth: 'Fit W', fitHeight: 'Fit H', original: '1:1' };
  var fitOrder = ['fitWidth', 'fitHeight', 'original'];
  function syncFitLabel() {
    fitBtn.textContent = fitLabels[viewMode];
  }
  fitBtn.addEventListener('click', () => {
    viewMode = fitOrder[(fitOrder.indexOf(viewMode) + 1) % fitOrder.length];
    localStorage.setItem('gi_viewMode', viewMode);
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
    localStorage.setItem('gi_direction', direction);
    syncDirLabel();
  });
  syncDirLabel();

  slider.addEventListener('input', () => {
    goToPage(parseInt(slider.value, 10) - 1);
  });

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
      next();
    } else if (e.key === 'Escape') {
      e.preventDefault();
      window.location.href = `/view/manga/${encodeURIComponent(pid)}/${encodeURIComponent(mid)}`;
    } else if (e.key === 'Home') {
      e.preventDefault();
      goToPage(0);
    } else if (e.key === 'End') {
      e.preventDefault();
      goToPage(pages.length - 1);
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
    preloaded = {}; // stale at-home URLs 404 — drop cached Image objects
    _lastFailedRetry = function() { loadChapter(resume); };
    fetch(`/api/reader-data/${[pid, mid, cid].map(encodeURIComponent).join('/')}`)
      .then((r) => {
        if (!r.ok) throw new Error(`HTTP ${r.status}`);
        return r.json();
      })
      .then((data) => {
        if (data.error) {
          showReaderError(data.error);
          return;
        }
        pages = data.pages || [];
        if (!pages.length) {
          showReaderError('Empty chapter (plugin failed to fetch pages) — try Retry');
          return;
        }
        _lastFailedRetry = null;
        nextFrame(() => {
          // Honor ?page=N (or ?page=last from chapter advance); a retry keeps
          // the page the reader was already on.
          var sp = resume ? current + 1 : startPage;
          if (window.location.search.indexOf('page=last') >= 0) sp = pages.length;
          var initial = sp ? Math.max(1, Math.min(sp, pages.length)) - 1 : 0;
          drawPage(initial);
        });
      })
      .catch((err) => {
        showReaderError(friendlyError(err));
      });
  }
  loadChapter();
})();
