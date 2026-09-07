(() => {
  // ---- Toasts ------------------------------------------------------------
  var stack;

  function ensureStack() {
    if (stack) return stack;
    stack = document.getElementById('toast-stack');
    return stack;
  }

  var ICONS = {
    success: '✓',
    error: '✕',
    info: 'ℹ',
  };
  var ACCENT = {
    success: 'bg-emerald-500',
    error: 'bg-red-500',
    info: 'bg-indigo-400',
  };
  var DURATION = { success: 3500, info: 3500, error: 6000 };

  // showToast(message, type) — type: 'success'|'error'|'info'. Truthy non-string
  // second arg is treated as 'error' for backward compatibility with the old
  // showToast(m, isError) signature.
  window.showToast = (m, type) => {
    if (typeof type !== 'string') type = type ? 'error' : 'success';
    if (ACCENT[type] === undefined) type = 'info';

    var container = ensureStack();
    if (!container) return;

    var el = document.createElement('div');
    el.className =
      'toast-item pointer-events-auto flex items-start gap-2.5 w-80 max-w-[calc(100vw-2rem)] ' +
      'bg-neutral-900 border border-neutral-700 rounded-lg shadow-xl p-3 pr-2.5 ' +
      'relative overflow-hidden';
    el.setAttribute('role', 'status');

    var bar = document.createElement('span');
    bar.className = `absolute left-0 top-0 bottom-0 w-1 ${ACCENT[type]}`;
    el.appendChild(bar);

    var icon = document.createElement('span');
    icon.className =
      'shrink-0 flex items-center justify-center size-5 rounded-full text-xs font-bold text-neutral-100 ' +
      ACCENT[type];
    icon.textContent = ICONS[type];
    el.appendChild(icon);

    var text = document.createElement('span');
    text.className = 'text-sm text-neutral-200 leading-snug break-words';
    text.textContent = m;
    el.appendChild(text);

    var close = document.createElement('button');
    close.type = 'button';
    close.className =
      'ml-auto shrink-0 size-5 inline-flex items-center justify-center rounded text-neutral-500 hover:text-neutral-300';
    close.setAttribute('aria-label', 'Dismiss');
    close.textContent = '✕';
    close.addEventListener('click', () => {
      dismiss(el);
    });
    el.appendChild(close);

    container.appendChild(el);

    // enter animation
    requestAnimationFrame(() => {
      el.classList.add('toast-visible');
    });

    el._hideTimer = setTimeout(() => {
      dismiss(el);
    }, DURATION[type]);

    while (container.children.length > 4) {
      container.removeChild(container.firstChild);
    }
  };

  function dismiss(el) {
    if (el._dismissed) return;
    el._dismissed = true;
    clearTimeout(el._hideTimer);
    el.classList.remove('toast-visible');
    el.classList.add('toast-leave');
    el.addEventListener('transitionend', () => {
      if (el.parentNode) el.parentNode.removeChild(el);
    });
    setTimeout(() => {
      if (el.parentNode) el.parentNode.removeChild(el);
    }, 350);
  }

  // ---- Confirm modal -----------------------------------------------------
  var modal, modalMsg, confirmBtn, cancelBtn, activeResolve, activeReturnEl;

  function ensureModal() {
    if (modal) return true;
    modal = document.getElementById('confirm-modal');
    if (!modal) return false;
    modalMsg = document.getElementById('confirm-message');
    confirmBtn = document.getElementById('confirm-ok');
    cancelBtn = document.getElementById('confirm-cancel');
    modal.addEventListener('click', (e) => {
      if (e.target === modal) resolveConfirm(false);
    });
    confirmBtn.addEventListener('click', () => {
      resolveConfirm(true);
    });
    cancelBtn.addEventListener('click', () => {
      resolveConfirm(false);
    });
    document.addEventListener('keydown', (e) => {
      if (e.key === 'Escape' && activeResolve) resolveConfirm(false);
    });
    return true;
  }

  window.showConfirm = (m) => {
    if (!ensureModal()) return Promise.resolve(true); // no modal markup: allow
    modalMsg.textContent = m;
    modal.classList.remove('hidden');
    activeReturnEl = document.activeElement;
    cancelBtn.focus();
    document.body.style.overflow = 'hidden';
    return new Promise((resolve) => {
      activeResolve = resolve;
    });
  };

  function resolveConfirm(result) {
    if (!activeResolve) return;
    var r = activeResolve;
    activeResolve = null;
    modal.classList.add('hidden');
    document.body.style.overflow = '';
    r(result);
    if (result === false && activeReturnEl && activeReturnEl.focus) {
      activeReturnEl.focus();
    }
  }

  // ---- data-confirm delegation ------------------------------------------

  // Intercept submits whose form (or submitter) declares data-confirm.
  document.addEventListener(
    'submit',
    (e) => {
      var form = e.target;
      var submitter = e.submitter;
      var msg = submitter?.getAttribute('data-confirm') || form.getAttribute('data-confirm');
      if (!msg) return;
      if (form._confirmPassed) {
        form._confirmPassed = false;
        return;
      }
      e.preventDefault();
      window.showConfirm(msg).then((ok) => {
        if (!ok) return;
        form._confirmPassed = true;
        if (typeof form.requestSubmit === 'function') {
          form.requestSubmit(submitter);
        } else {
          form.submit();
        }
      });
    },
    true,
  );

  // Intercept clicks on any [data-confirm] not inside a form (defensive).
  document.addEventListener(
    'click',
    (e) => {
      var el = e.target.closest('[data-confirm]');
      if (!el || el.closest('form')) return;
      e.preventDefault();
      e.stopPropagation();
      window.showConfirm(el.getAttribute('data-confirm')).then((ok) => {
        if (ok && el.click) {
          el.removeAttribute('data-confirm');
          el.click();
        }
      });
    },
    true,
  );

  // ---- URL param toast (?msg=) ------------------------------------------
  var url = new URL(window.location);
  var qmsg = url.searchParams.get('msg');
  if (qmsg) {
    window.showToast(qmsg, 'success');
    url.searchParams.delete('msg');
    history.replaceState(null, '', url);
  }

  // ---- HTMX error handling ----------------------------------------------
  document.addEventListener('htmx:responseError', (e) => {
    var xhr = e.detail?.xhr;
    var m = '';
    var t = '';
    if (xhr?.responseText) {
      t = String(xhr.responseText)
        .replace(/<[^>]*>/g, ' ')
        .replace(/\s+/g, ' ')
        .trim();
      if (t.length > 0 && t.length <= 120) m = t;
    }
    if (!m) m = xhr?.statusText || 'Request failed';
    window.showToast(m, 'error');
  });
  document.addEventListener('htmx:sendError', () => {
    window.showToast('Network error — check your connection', 'error');
  });
})();
