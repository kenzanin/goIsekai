(() => {
  // --- Constants (mirrored from old app.js) ---
  var ICONS = { success: '✓', error: '✕', info: 'ℹ' };
  var ACCENT = { success: 'bg-emerald-500', error: 'bg-red-500', info: 'bg-indigo-400' };
  var DURATION = { success: 3500, info: 3500, error: 6000 };

  // CSRF token minted by the server and emitted into the page's meta tag by
  // layouts/base.lua. Plain forms carry it in a hidden field, but the fetches
  // below build their own bodies, so they send it as a header.
  function csrfToken() {
    var el = document.querySelector('meta[name="csrf-token"]');
    return el ? el.getAttribute('content') || '' : '';
  }

  // The server sets X-GoIsekai-Reload on a CSRF refusal when the token we sent
  // was well formed but belongs to an earlier run — i.e. this page was rendered
  // before a restart. Reloading is the whole fix, so do it instead of showing a
  // dead error the user has to interpret.
  //
  // Guarded by sessionStorage: if the reloaded page still carries a stale token
  // (a cached document, a proxy holding HTML) we must not loop, so the second
  // refusal falls through to the normal error toast.
  var RELOAD_FLAG = 'gi_csrf_reloaded';
  function recoverFromStaleToken(resp) {
    if (!resp.headers.get('X-GoIsekai-Reload')) return false;
    try {
      if (sessionStorage.getItem(RELOAD_FLAG)) return false;
      sessionStorage.setItem(RELOAD_FLAG, '1');
    } catch {
      return false; // storage blocked: fall through to the error toast
    }
    location.reload();
    return true;
  }

  // A page that carries a token at all was rendered by a live server, so the
  // reload guard has done its job and can be cleared for the next stale page.
  (function clearStaleGuard() {
    try {
      if (document.querySelector('meta[name="csrf-token"]')) sessionStorage.removeItem(RELOAD_FLAG);
    } catch {}
  })();

  // Inject animation CSS once (matches old toast-visible / toast-leave behavior)
  var _style = document.createElement('style');
  _style.textContent =
    '.toast-item{transition:transform 300ms ease,opacity 300ms ease;transform:translateX(100%);opacity:0}' +
    '.toast-enter{transform:translateX(0);opacity:1}' +
    '.toast-leave{transform:translateX(100%);opacity:0}';
  (document.head || document.documentElement).appendChild(_style);

  // Global helper: surface a server-sent toast. Handlers pass the fetch
  // response; the message may ride the 303 as X-Toast or as ?toast= on the
  // redirect target (fetch follows redirects, so only the target survives).
  window.showToast = (resp, type) => {
    if (!resp) return;
    var msg = resp.headers?.get('X-Toast');
    var m;
    if (!msg && resp.url) {
      m = resp.url.match(/[?&]toast=([^&]+)/);
      if (m) msg = decodeURIComponent(m[1]);
    }
    if (!msg) return;
    if (typeof Alpine !== 'undefined' && Alpine.store('toast')) {
      Alpine.store('toast').show(msg, type || 'success');
    }
  };
  // List-page memory for the detail-page Back button: remember the last list
  // URL (search/library/updates/history) so the in-page Back returns exactly
  // where the user came from. history.back() is unreliable — the stack may
  // hold reader or stale entries — and the library fallback loses search
  // results. MangaDex-style back-to-list, no hack.
  window.rememberListPage = () => {
    try {
      if (/^\/view\/(search|library|updates|history)/.test(window.location.pathname)) {
        sessionStorage.setItem('gsk:last-list', window.location.pathname + window.location.search);
      }
    } catch {
      /* storage unavailable */
    }
  };
  window.backToList = () => {
    try {
      const u = sessionStorage.getItem('gsk:last-list');
      if (u && u.charAt(0) === '/') {
        window.location.href = u;
        return;
      }
    } catch {
      /* storage unavailable */
    }
    const ref = document.referrer;
    if (
      ref &&
      ref.indexOf(window.location.origin) === 0 &&
      /\/view\/(search|library|updates|history)/.test(ref)
    ) {
      window.location.href = ref;
      return;
    }
    window.location.href = '/view/library';
  };
  // Global helper: submit a form via SPA fetch (same as submit event handler).
  // Called as: @click="submitForm($el.closest('form'))"
  window.submitForm = (form) => {
    if (!form) return;
    var method = (form.getAttribute('method') || 'get').toLowerCase();
    if (method !== 'post') return;
    var action = (form.getAttribute('action') || '').trim();
    if (action.indexOf('/action/') !== 0) return;
    if (action.indexOf('export-cbz') !== -1) return;
    if (action.indexOf('save-verify') !== -1) return;

    fetch(action, {
      method: 'POST',
      body: new URLSearchParams(new FormData(form)),
      headers: { 'X-Partial': 'true', 'X-CSRF-Token': csrfToken() },
      credentials: 'same-origin',
    })
      .then((resp) => {
        if (!resp.ok && recoverFromStaleToken(resp)) return;
        return resp.text().then((html) => {
          if (!resp.ok) {
            let m = html?.trim() || `Request failed (${resp.status})`;
            if (/^</.test(m)) m = `Request failed (${resp.status})`;
            if (typeof Alpine !== 'undefined' && Alpine.store('toast')) {
              Alpine.store('toast').show(m, 'error');
            }
            return;
          }
          const main = document.getElementById('content');
          if (!main || !html) return;
          const match = html.match(/<main[^>]*id="content"[\s\S]*?>([\s\S]*?)<\/main>/i);
          main.innerHTML = match ? match[1] : html;
          // innerHTML does not execute <script> — re-raise script nodes so
          // page-specific inline scripts actually run.
          main.querySelectorAll('script').forEach((old) => {
            const s = document.createElement('script');
            s.textContent = old.textContent;
            old.replaceWith(s);
          });
          if (window.Alpine && Alpine.initTree) {
            Alpine.initTree(main);
          }
          if (window.syncEnrichmentPanel) syncEnrichmentPanel();
          rememberListPage();
          // An action mutates the current page, it does not navigate: the
          // address bar keeps the page URL. `/action/` is never a page, and
          // writing it in makes a reload or a back press land on the action.
          const u = resp.url || '';
          if (u.indexOf(window.location.origin) === 0 && u.indexOf('/action/') === -1) {
            history.replaceState(history.state, '', u);
          }
          showToast(resp, 'success');
        });
      })
      .catch(() => {
        if (typeof Alpine !== 'undefined' && Alpine.store('toast')) {
          Alpine.store('toast').show('Network error — check your connection', 'error');
        }
      });
  };

  // Job actions: sync / sync-manga / export-cbz POST JSON {status:"ok",job_id}
  // then stream progress over /jobs/ws (subscribe {"job_id":"<id>"}).
  // Contract field is job_id (snake_case). Server pushes
  // {job_id, status: queued|running|done|failed, path?} (path only on
  // export-cbz done). Usage: <form data-job-action="sync" ...> + delegated
  // submit listener below, or window.submitJobAction(form, {successMessage}).
  var JOB_DEFAULTS = {
    sync: {
      queued: 'Sync queued',
      running: 'Sync running',
      done: 'Sync done',
      failed: 'Sync failed',
    },
    'sync-manga': {
      queued: 'Manga sync queued',
      running: 'Manga sync running',
      done: 'Manga sync done',
      failed: 'Manga sync failed',
    },
    'export-cbz': {
      queued: 'CBZ export queued',
      running: 'CBZ export running',
      done: 'CBZ ready',
      failed: 'CBZ export failed',
    },
  };
  var jobToast = (msg, type) => {
    if (typeof Alpine !== 'undefined' && Alpine.store('toast')) {
      Alpine.store('toast').show(msg, type);
    }
  };
  var jobKind = (form) => {
    var hint = (form.getAttribute('data-job-action') || '').trim();
    if (hint && hint !== 'true') return hint;
    var action = form.getAttribute('action') || '';
    if (action.indexOf('export-cbz') !== -1) return 'export-cbz';
    if (action.indexOf('sync-manga') !== -1) return 'sync-manga';
    return 'sync';
  };
  window.submitJobAction = (form, opts) => {
    if (!form) return;
    opts = opts || {};
    var kind = jobKind(form);
    var defaults = JOB_DEFAULTS[kind] || JOB_DEFAULTS.sync;
    var queuedMsg = opts.queuedMessage || defaults.queued;
    var runningMsg = opts.runningMessage || defaults.running;
    var doneMsg = opts.successMessage || opts.doneMessage || defaults.done;
    var failedMsg = opts.failedMessage || defaults.failed;
    var btn = form.querySelector('button[type="submit"]') || form.querySelector('button');
    var origHTML = btn ? btn.innerHTML : null;
    if (btn) btn.disabled = true;
    var restoreBtn = () => {
      if (btn) {
        btn.disabled = false;
        if (origHTML !== null) btn.innerHTML = origHTML;
      }
    };
    var action = form.getAttribute('action') || form.action;
    var method = (form.getAttribute('method') || form.method || 'POST').toUpperCase();
    fetch(action, {
      method: method,
      body: new URLSearchParams(new FormData(form)),
      headers: method === 'POST' ? { 'X-CSRF-Token': csrfToken() } : {},
      credentials: 'same-origin',
    })
      .then((resp) => {
        if (!resp.ok && method === 'POST' && recoverFromStaleToken(resp)) return;
        if (!resp.ok) {
          return resp.text().then((t) => {
            var m = (t || '').trim() || `Request failed (${resp.status})`;
            if (/^</.test(m)) m = `Request failed (${resp.status})`;
            throw new Error(m);
          });
        }
        return resp.json();
      })
      .then((data) => {
        var jobID = data?.job_id;
        if (!jobID) throw new Error('Bad response: missing job_id');
        // export-cbz hands back the download link up front; the file itself only
        // exists once the job reports done.
        var resultURL = data?.url || '';
        jobToast(queuedMsg, 'info');
        var scheme = window.location.protocol === 'https:' ? 'wss://' : 'ws://';
        var ws = new WebSocket(`${scheme}${window.location.host}/jobs/ws`);
        ws.onopen = () => {
          ws.send(JSON.stringify({ job_id: jobID }));
        };
        ws.onmessage = (ev) => {
          var msg = null;
          try {
            msg = JSON.parse(ev.data);
          } catch {
            return;
          }
          if (!msg || msg.job_id !== jobID) return;
          if (msg.status === 'running') {
            jobToast(runningMsg, 'info');
          } else if (msg.status === 'done') {
            if (kind === 'export-cbz') {
              // Fetch the archive instead of only naming it. Previously this
              // branch toasted msg.path, which is a filesystem path like
              // app_data/cache/exports/... - nothing a browser can open, so the
              // button looked like it had done nothing at all.
              if (resultURL) {
                const link = document.createElement('a');
                link.href = resultURL;
                link.download = '';
                link.rel = 'noopener';
                document.body.appendChild(link);
                link.click();
                document.body.removeChild(link);
                jobToast(doneMsg, 'success');
              } else if (msg.path) {
                jobToast(`${doneMsg}: ${msg.path}`, 'success');
              } else {
                jobToast(failedMsg, 'error');
              }
            } else {
              jobToast(doneMsg, 'success');
            }
            restoreBtn();
            ws.close();
          } else if (msg.status === 'failed') {
            jobToast(failedMsg, 'error');
            restoreBtn();
            ws.close();
          }
        };
        ws.onerror = () => {
          jobToast(failedMsg, 'error');
          restoreBtn();
          try {
            ws.close();
          } catch {
            /* socket already closed */
          }
        };
      })
      .catch((err) => {
        jobToast(err?.message || 'Network error — check your connection', 'error');
        restoreBtn();
      });
  };
  // Delegated submit for job-action forms (capture so it wins over the
  // detail-page SPA interceptor; export-cbz is excluded there anyway).
  document.addEventListener(
    'submit',
    (e) => {
      var t = e.target;
      var form = t && t.tagName === 'FORM' ? t : t?.closest?.('form');
      if (!form || typeof form.getAttribute !== 'function') return;
      var hint = form.getAttribute('data-job-action');
      if (hint === null || hint === undefined) return;
      e.preventDefault();
      e.stopImmediatePropagation();
      window.submitJobAction(form, {
        successMessage: form.getAttribute('data-success-message') || undefined,
      });
    },
    true,
  );

  // Cover image error fallback: swap broken <img data-fallback="XY"> for an
  // initial-letter placeholder (kept outside inline onerror so the Lua template
  // never has to escape quotes into an HTML attribute).
  document.addEventListener(
    'error',
    (e) => {
      const img = e.target;
      if (!img || typeof img.getAttribute !== 'function') return;
      const fallback = img.getAttribute('data-fallback');
      if (fallback === null || fallback === undefined) return;
      const ph = document.createElement('div');
      ph.className =
        'w-full aspect-[2/3] bg-neutral-800 flex items-center justify-center text-neutral-500 text-2xl font-semibold';
      ph.textContent = fallback || '';
      img.replaceWith(ph);
    },
    true,
  );

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
        if (!msg && list && field?.value && list.split(',').indexOf(field.value) !== -1) {
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
        // Find the form: for native submit events bubbling from children
        // (e.g. data-confirm flow), e.target may be a child element.
        var form = e.target;
        if (form?.tagName !== 'FORM') {
          form = e.target?.closest('form');
        }
        if (!form?.tagName || form.tagName !== 'FORM' || e.defaultPrevented) return;
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

        // Auto-follow redirects: a 303 must be followed by the browser
        // itself. `redirect: 'manual'` yields an opaqueredirect (status 0,
        // unreadable headers) which cannot be consumed.
        fetch(action, {
          method: 'POST',
          // URL-encoded, not FormData: the action handlers call
          // r.ParseForm()+FormValue, which drops multipart bodies.
          body: new URLSearchParams(new FormData(form)),
          headers: { 'X-Partial': 'true', 'X-CSRF-Token': csrfToken() },
          credentials: 'same-origin',
        })
          .then((resp) => {
            if (!resp.ok && recoverFromStaleToken(resp)) return;
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
              main.querySelectorAll('script').forEach((old) => {
                const s = document.createElement('script');
                s.textContent = old.textContent;
                old.replaceWith(s);
              });
              if (window.Alpine && Alpine.initTree) {
                Alpine.initTree(main);
              }
              if (window.syncEnrichmentPanel) syncEnrichmentPanel();
              rememberListPage();
              // See submitForm: never leave an /action/ URL in the address bar.
              const u = resp.url || '';
              if (u.indexOf(window.location.origin) === 0 && u.indexOf('/action/') === -1) {
                history.replaceState(history.state, '', u);
              }
              showToast(resp, 'success');
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
  // Enrichment panel — form is now a plain HTMX POST in the template.
  // Kept as no-op for callers that still reference it.
  const syncEnrichmentPanel = () => {};

  // =====================================================================
  // =====================================================================
  window.syncEnrichmentPanel = syncEnrichmentPanel;
  syncEnrichmentPanel();

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

  // Toggle visibility of genre tags (show more/less)
  window.toggleGenreTags = (btnId, maxVisible) => {
    var tags = document.querySelectorAll('.genre-tag');
    var btn = document.getElementById(btnId);
    if (!btn) return;
    var showing = btn.dataset.show === '1';
    for (let i = maxVisible; i < tags.length; i++) {
      tags[i].style.display = showing ? 'none' : '';
    }
    btn.dataset.show = showing ? '0' : '1';
    btn.textContent = showing ? 'Show more' : 'Show less';
  };
})();

// =====================================================================
// Loading state helpers for buttons
// =====================================================================
window.setLoading = (btn, loading) => {
  if (!btn) return;
  btn.disabled = loading;
  if (loading) {
    btn.classList.add('opacity-70');
  } else {
    btn.classList.remove('opacity-70');
  }
};

// =====================================================================
// Relative timestamps: any element carrying data-ts shows "3h ago"
// =====================================================================
document.addEventListener('DOMContentLoaded', () => {
  rememberListPage();
  // Mermaid diagrams (About page): convert code.language-mermaid blocks to
  // divs mermaid renders as SVG, dark-themed to match the UI. No-op elsewhere.
  var mmd = document.querySelectorAll('code.language-mermaid');
  if (mmd.length && window.mermaid) {
    mmd.forEach((code) => {
      var div = document.createElement('pre');
      div.className = 'mermaid';
      div.textContent = code.textContent;
      code.closest('pre').replaceWith(div);
    });
    window.mermaid.initialize({ startOnLoad: false, theme: 'dark' });
    window.mermaid.run({ querySelector: '.mermaid' });
  }
  var m = window.location.search.match(/[?&]toast=([^&]+)/);
  if (m && typeof showToast === 'function') {
    showToast({ headers: null, url: window.location.href }, 'success');
    history.replaceState(history.state, '', window.location.pathname + window.location.hash);
  }
  if (!document.querySelector('[data-ts]')) return;
  const refresh = () => {
    document.querySelectorAll('[data-ts]').forEach((el) => {
      const mins = Math.floor((Date.now() - new Date(el.dataset.ts).getTime()) / 60000);
      if (mins < 60) el.textContent = `${mins}m ago`;
      else if (mins < 1440) el.textContent = `${Math.floor(mins / 60)}h ago`;
      else el.textContent = `${Math.floor(mins / 1440)}d ago`;
    });
  };
  refresh();
  setInterval(refresh, 60000);
});
