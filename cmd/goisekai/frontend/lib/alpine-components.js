(() => {
  // --- Constants (mirrored from old app.js) ---
  var ICONS = { success: '✓', error: '✕', info: 'ℹ' };
  var ACCENT = { success: 'bg-emerald-500', error: 'bg-red-500', info: 'bg-indigo-400' };
  var DURATION = { success: 3500, info: 3500, error: 6000 };

  // Inject animation CSS once (matches old toast-visible / toast-leave behavior)
  var _style = document.createElement('style');
  _style.textContent =
    '.toast-item{transition:transform 300ms ease,opacity 300ms ease;transform:translateX(100%);opacity:0}' +
    '.toast-enter{transform:translateX(0);opacity:1}' +
    '.toast-leave{transform:translateX(100%);opacity:0}';
  (document.head || document.documentElement).appendChild(_style);

  // Expand a collapsible section (used after fetch-alt-* actions so freshly
  // fetched items are visible instead of re-collapsed by the re-render).
  const expandSection = (id) => {
    const body = document.getElementById(id);
    if (!body) return;
    body.classList.remove('hidden');
    const btn = body.previousElementSibling;
    const ch = btn?.querySelector('.chev');
    if (ch) ch.classList.add('rotate-90');
  };

  // =====================================================================
  // Alpine stores — registered inside alpine:init so Alpine is guaranteed
  // to be present but hasn't processed the DOM yet.
  // =====================================================================
  document.addEventListener('alpine:init', () => {
    // ---- 4.1 Toast store ------------------------------------------------
    Alpine.store('toast', {
      items: [],
      _id: 0,
      _container: null,

      // show(msg, isError?) — isError: boolean or string type name
      show: function (msg, isError) {
        var type = typeof isError !== 'string' ? (isError ? 'error' : 'success') : isError;
        if (!(type in ACCENT)) type = 'info';

        var id = ++this._id;
        var item = { id: id, msg: String(msg), isError: type === 'error', _el: null, _timer: null };
        this.items.push(item);

        if (this._container) this._append(item, type);

        item._timer = setTimeout(() => {
          this.dismiss(id);
        }, DURATION[type]);

        // Cap stack at 4
        var old;
        while (this.items.length > 4) {
          old = this.items.shift();
          if (old._timer) clearTimeout(old._timer);
          if (old._el) old._el.remove();
        }
      },

      dismiss: function (id) {
        var idx = -1;
        var i;
        for (i = 0; i < this.items.length; i++) {
          if (this.items[i].id === id) {
            idx = i;
            break;
          }
        }
        if (idx === -1) return;
        var item = this.items.splice(idx, 1)[0];
        if (item._timer) clearTimeout(item._timer);
        var el;
        if (item._el) {
          el = item._el;
          el.classList.remove('toast-enter');
          el.classList.add('toast-leave');
          el.addEventListener('transitionend', function handler() {
            el.removeEventListener('transitionend', handler);
            if (el.parentNode) el.parentNode.removeChild(el);
          });
          setTimeout(() => {
            if (el.parentNode) el.parentNode.removeChild(el);
          }, 350);
        }
      },

      // Called from x-init on the toast container element.
      render: function (el) {
        this._container = el;

        this.items.forEach((item) => {
          if (item._el) return;
          this._append(item, item.isError ? 'error' : 'success');
        });
      },

      // --- private ---
      _append: function (item, type) {
        var el = document.createElement('div');
        el.setAttribute('data-toast-id', item.id);
        el.className =
          'toast-item pointer-events-auto flex items-start gap-2.5 w-80 max-w-[calc(100vw-2rem)] ' +
          'bg-neutral-900 border border-neutral-700 rounded-lg shadow-xl p-3 pr-2.5 ' +
          'relative overflow-hidden';
        el.setAttribute('role', 'status');

        // Accent bar
        var bar = document.createElement('span');
        bar.className = `absolute left-0 top-0 bottom-0 w-1 ${ACCENT[type]}`;
        el.appendChild(bar);

        // Icon badge
        var icon = document.createElement('span');
        icon.className =
          'shrink-0 flex items-center justify-center size-5 rounded-full text-xs font-bold text-neutral-100 ' +
          ACCENT[type];
        icon.textContent = ICONS[type];
        el.appendChild(icon);

        // Message text (textContent — never innerHTML for user data)
        var text = document.createElement('span');
        text.className = 'text-sm text-neutral-200 leading-snug break-words';
        text.textContent = item.msg;
        el.appendChild(text);

        var close = document.createElement('button');
        close.type = 'button';
        close.className =
          'ml-auto shrink-0 size-5 inline-flex items-center justify-center rounded text-neutral-500 hover:text-neutral-300';
        close.setAttribute('aria-label', 'Dismiss');
        close.textContent = '✕';
        close.addEventListener('click', () => {
          this.dismiss(item.id);
        });
        el.appendChild(close);

        this._container.appendChild(el);
        item._el = el;

        // Trigger enter animation on next frame
        requestAnimationFrame(() => {
          el.classList.add('toast-enter');
        });
      },
    });

    // ---- 4.1b Confirm store ---------------------------------------------
    Alpine.store('confirm', {
      open: false,
      message: '',
      _resolve: null,
      _container: null,
      _returnEl: null,
      _msgEl: null,
      _cancelBtn: null,

      ask: function (message) {
        return new Promise((resolve) => {
          this.message = message;
          this._resolve = resolve;
          this._returnEl = document.activeElement;
          this.open = true;

          if (this._container) {
            this._msgEl.textContent = message;
            this._container.classList.remove('hidden');
            this._container.style.display = ''; // overrides the inline display:none boot-flash guard
            document.body.style.overflow = 'hidden';
            setTimeout(() => {
              this._cancelBtn.focus();
            }, 0);
          }
        });
      },

      yes: function () {
        this._close(true);
      },
      no: function () {
        this._close(false);
      },

      _close: function (result) {
        var r = this._resolve;
        this._resolve = null;
        this.open = false;
        if (this._container) {
          this._container.classList.add('hidden');
          this._container.style.display = 'none';
          document.body.style.overflow = '';
        }
        if (r) r(result);
        if (result === false && this._returnEl && this._returnEl.focus) {
          this._returnEl.focus();
        }
      },

      // Called from x-init on the confirm container element.
      mount: function (el) {
        this._container = el;

        // Build modal DOM
        el.innerHTML = '';
        el.className = 'fixed inset-0 z-50 hidden';
        el.setAttribute('role', 'dialog');
        el.setAttribute('aria-modal', 'true');

        // Backdrop
        var backdrop = document.createElement('div');
        backdrop.className = 'absolute inset-0 bg-black/50';
        el.appendChild(backdrop);

        // Centre wrapper
        var wrapper = document.createElement('div');
        wrapper.className = 'relative flex items-center justify-center min-h-full p-4';
        el.appendChild(wrapper);

        // Panel
        var panel = document.createElement('div');
        panel.className =
          'bg-neutral-900 border border-neutral-700 rounded-xl shadow-2xl p-6 max-w-sm w-full';
        wrapper.appendChild(panel);

        // Message
        var msg = document.createElement('p');
        msg.className = 'text-neutral-200 text-sm';
        msg.textContent = this.message;
        panel.appendChild(msg);
        this._msgEl = msg;

        // Button row
        var btnRow = document.createElement('div');
        btnRow.className = 'mt-6 flex justify-end gap-3';
        panel.appendChild(btnRow);

        // Cancel
        var cancel = document.createElement('button');
        cancel.type = 'button';
        cancel.className =
          'px-4 py-2 text-sm font-medium rounded-lg bg-neutral-700 text-neutral-200 hover:bg-neutral-600 focus:outline-none focus:ring-2 focus:ring-neutral-500';
        cancel.textContent = 'Cancel';
        cancel.addEventListener('click', () => {
          this.no();
        });
        btnRow.appendChild(cancel);
        this._cancelBtn = cancel;

        // OK
        var ok = document.createElement('button');
        ok.type = 'button';
        ok.className =
          'px-4 py-2 text-sm font-medium rounded-lg bg-indigo-500 text-white hover:bg-indigo-400 focus:outline-none focus:ring-2 focus:ring-indigo-500';
        ok.textContent = 'OK';
        ok.addEventListener('click', () => {
          this.yes();
        });
        btnRow.appendChild(ok);

        // Backdrop click → cancel
        el.addEventListener('click', (e) => {
          if (e.target === el || e.target === backdrop) this.no();
        });

        // Escape → cancel (once per mount, cleaned up on teardown)
        this._keydownHandler = (e) => {
          if (e.key === 'Escape' && this.open) this.no();
        };
        document.addEventListener('keydown', this._keydownHandler);
      },
    });

    // ---- 4.3 data-confirm delegated handler ------------------------------
    // Submit interception (capture) — mirrors old app.js logic exactly
    document.addEventListener(
      'submit',
      (e) => {
        var form = e.target;
        var submitter = e.submitter;
        var msg = submitter?.getAttribute('data-confirm') || form.getAttribute('data-confirm');
        var list = form.getAttribute('data-confirm-actions');
        var field = form.querySelector('[name=action]');
        if (!msg && list && field && field.value && list.split(',').indexOf(field.value) !== -1) {
          msg = 'Are you sure?';
        }
        if (!msg) return;

        // Fail open if Alpine/store not ready
        if (typeof Alpine === 'undefined' || !Alpine.store('confirm')) return;

        if (form._confirmPassed) {
          form._confirmPassed = false;
          return;
        }
        e.preventDefault();
        // Mark the form so the SPA submit interceptor (4.5) ignores this
        // event — the confirmed re-submit below drives the actual request.
        form._confirmPending = true;
        Alpine.store('confirm')
          .ask(msg)
          .then((ok) => {
            if (!ok) {
              form._confirmPending = false;
              return;
            }
            form._confirmPassed = true;
            form._confirmPending = false;
            if (typeof form.requestSubmit === 'function') {
              form.requestSubmit(submitter);
            } else {
              form.submit();
            }
          });
      },
      true,
    );

    // Click interception for non-form [data-confirm] elements (capture)
    document.addEventListener(
      'click',
      (e) => {
        var el = e.target.closest('[data-confirm]');
        if (!el || el.closest('form')) return;

        // Fail open if Alpine/store not ready
        if (typeof Alpine === 'undefined' || !Alpine.store('confirm')) return;

        e.preventDefault();
        e.stopPropagation();
        Alpine.store('confirm')
          .ask(el.getAttribute('data-confirm'))
          .then((ok) => {
            if (ok && el.click) {
              el.removeAttribute('data-confirm');
              el.click();
            }
          });
      },
      true,
    );

    // ---- 4.5 SPA submit: same-page actions via fetch, history stays clean -
    // Detail-page action forms POST to /action/* which 303-redirects back to
    // the same /view/manga/* page. A plain form submit makes the browser push a
    // new history entry every time (alt-title clicks pile up), so Back ends up
    // stuck on the detail page instead of returning to search/library. Instead:
    // fetch the action with X-Partial, follow the redirect to the re-rendered
    // view content, and swap #content in place — no new history entry. Binary
    // downloads (export-cbz) and cross-page actions keep native navigation.
    document.addEventListener(
      'submit',
      (e) => {
        var form = e.target;
        if (!form?.tagName || form.tagName !== 'FORM' || e.isDefaultPrevented()) return;
        var method = (form.getAttribute('method') || 'get').toLowerCase();
        if (method !== 'post') return;
        var action = (form.getAttribute('action') || '').trim();
        if (action.indexOf('/action/') !== 0) return;
        // Scope to the manga detail page — other pages keep native navigation.
        if (window.location.pathname.indexOf('/view/manga/') !== 0) return;
        if (action.indexOf('export-cbz') !== -1) return; // binary download
        if (action.indexOf('save-verify') !== -1) return; // cookie form (plugins page)
        // If a confirm is pending, let the confirm flow drive the submit.
        if (form._confirmPending) return;
        if (form._confirmPassed) {
          form._confirmPassed = false;
        }
        e.preventDefault();

        var showErr = (m) => {
          if (typeof Alpine !== 'undefined' && Alpine.store('toast')) {
            Alpine.store('toast').show(m, 'error');
          }
        };

        // Follow redirects manually so we can keep X-Partial on the
        // re-fetch — fetch() auto-follow drops custom headers on 303.
        const followRedirects = (url, opts) =>
          fetch(url, { ...opts, redirect: 'manual' }).then((resp) => {
            if (resp.status >= 301 && resp.status <= 303) {
              const loc = resp.headers.get('Location') || url;
              return fetch(loc, {
                method: 'GET',
                headers: { 'X-Partial': 'true' },
                credentials: 'same-origin',
              });
            }
            return resp;
          });

        followRedirects(action, {
          method: 'POST',
          body: new FormData(form),
          headers: { 'X-Partial': 'true' },
          credentials: 'same-origin',
        })
          .then((resp) => {
            return resp.text().then((html) => {
              if (!resp.ok) {
                let m = html?.trim() || `Request failed (${resp.status})`;
                // Error bodies are plain text — don't toast raw HTML.
                if (/^</.test(m)) m = `Request failed (${resp.status})`;
                showErr(m);
                return;
              }
              const main = document.getElementById('content');
              if (!main || !html) return;
              // Parse the partial response to extract just the <main> content
              // in case the redirect returned a full page.
              const match = html.match(/<main[^>]*id="content"[\s\S]*?>([\s\S]*?)<\/main>/i);
              main.innerHTML = match ? match[1] : html;
              if (window.Alpine && Alpine.initTree) {
                Alpine.initTree(main);
              }
              if (window.syncCoverDim) syncCoverDim();
              // Sync the address bar to the canonical re-rendered URL without
              // adding a history entry — Back still leaves the detail page.
              const u = resp.url || action;
              if (u && u.indexOf(window.location.origin) === 0) {
                history.replaceState(history.state, '', u);
              }
              // After a fetch action, reveal the section that just grew.
              if (action.indexOf('fetch-alt-titles') !== -1) expandSection('alt-titles-body');
              if (action.indexOf('fetch-alt-summaries') !== -1) expandSection('alt-summaries-body');
            });
          })
          .catch(() => {
            showErr('Network error — check your connection');
          });
      },
      true,
    );

    // ---- 4.4 Global error → toast (replaces old htmx listeners) --------
    window.addEventListener('unhandledrejection', (e) => {
      var reason = e.reason;
      var msg = '';
      var text = '';

      if (reason instanceof TypeError) {
        text = String(reason.message || reason);
        if (/fetch|network|Failed to fetch|NetworkError/i.test(text)) {
          msg = 'Network error — check your connection';
        }
      } else if (reason instanceof Response) {
        if (reason.status >= 400) {
          msg = reason.statusText || `Request failed (${reason.status})`;
        }
      } else if (typeof reason === 'string' && /fetch|network/i.test(reason)) {
        msg = 'Network error — check your connection';
      }

      if (msg && typeof Alpine !== 'undefined' && Alpine.store('toast')) {
        Alpine.store('toast').show(msg, true);
      }
    });

    window.addEventListener('error', (e) => {
      // Suppress benign noise
      if (e.message && /ResizeObserver/i.test(e.message)) return;
      if (e.message && /Script error/i.test(e.message)) return;
      if (e.filename && e.filename.indexOf('extension') !== -1) return;

      var msg = e.message || '';
      if (msg && typeof Alpine !== 'undefined' && Alpine.store('toast')) {
        Alpine.store('toast').show(msg, true);
      }
    });

    // ---- URL param toast (?msg=) ----------------------------------------
    var url = new URL(window.location);
    var qmsg = url.searchParams.get('msg');
    if (qmsg) {
      Alpine.store('toast').show(qmsg, 'success');
      url.searchParams.delete('msg');
      history.replaceState(null, '', url);
    }
  });

  // =====================================================================
  // Cover-dim restore (shared by detail + library templates)
  // Dim state lives in localStorage as gsk:cover-dim:{pluginID}:{id}.
  // Templates no longer inline restore <script> tags: HTML entities inside
  // <script> are not decoded (JS syntax error) and innerHTML swaps skip
  // inline scripts entirely. This runs on load + after SPA content swaps.
  // =====================================================================
  const syncCoverDim = () => {
    document.querySelectorAll('[data-key]').forEach((wrap) => {
      const dim = wrap.querySelector('.lib-dim');
      if (!dim || !wrap.dataset.key) return;
      const on = localStorage.getItem(`gsk:cover-dim:${wrap.dataset.key}`) === '1';
      dim.style.display = on ? 'block' : 'none';
    });
    const btn = document.getElementById('cover-dim-btn');
    const wrap = document.getElementById('cover-wrap');
    if (btn && wrap?.dataset.key) {
      const on = localStorage.getItem(`gsk:cover-dim:${wrap.dataset.key}`) === '1';
      btn.dataset.on = on ? '1' : '0';
      btn.textContent = on ? 'Show cover' : 'Hide cover';
    }
  };
  window.syncCoverDim = syncCoverDim;
  syncCoverDim();

  // =====================================================================
  // Library view mode: grid ↔ list, persisted in localStorage (gsk:view-mode)
  // Reads the stored mode on load, toggles container + buttons, saves on change.
  // =====================================================================
  const syncViewMode = () => {
    const container = document.querySelector('.view-container');
    const toggle = document.querySelector('.view-mode-toggle');
    if (!container || !toggle) return;
    const mode = localStorage.getItem('gsk:view-mode') || 'grid';
    container.dataset.viewMode = mode;
    const gridBtn = document.getElementById('view-grid-btn');
    const listBtn = document.getElementById('view-list-btn');
    const setActive = (btn, on) => {
      btn.classList.toggle('bg-neutral-700', on);
      btn.setAttribute('aria-pressed', on ? 'true' : 'false');
      btn.querySelector('svg').style.color = on ? '#818cf8' : '';
    };
    if (gridBtn && listBtn) {
      setActive(gridBtn, mode === 'grid');
      setActive(listBtn, mode === 'list');
      gridBtn.onclick = () => {
        localStorage.setItem('gsk:view-mode', 'grid');
        container.dataset.viewMode = 'grid';
        setActive(gridBtn, true);
        setActive(listBtn, false);
      };
      listBtn.onclick = () => {
        localStorage.setItem('gsk:view-mode', 'list');
        container.dataset.viewMode = 'list';
        setActive(listBtn, true);
        setActive(gridBtn, false);
      };
    }
  };
  window.syncViewMode = syncViewMode;
  syncViewMode();

  // =====================================================================
  // Backward compat — window.showToast(msg, isError)
  // Defined outside alpine:init so it's available immediately; safely
  // no-ops if the store isn't mounted yet.
  // =====================================================================
  window.showToast = (msg, type) => {
    if (typeof Alpine !== 'undefined' && Alpine.store('toast')) {
      Alpine.store('toast').show(msg, type);
    }
  };
})();
