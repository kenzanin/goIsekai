import { bindings } from '../bindings.js';

// ── Logs component ────────────────────────────────────────────────
// Polls GetLogs (buffered Go + webview [ui] lines) and renders them live.
// The section uses x-show (stays mounted), so Alpine never fires destroy() —
// instead refresh() cheaply checks the hash and skips the fetch off-page.
export const logsView = () => {
  const MAX = 1000; // render cap to keep the DOM light
  return {
    lines: [],
    autoScroll: true,
    _timer: null,

    init() {
      this.refresh();
      this._timer = setInterval(() => this.refresh(), 1000);
    },

    async refresh() {
      // Skip the fetch (and network round-trip) unless the Logs page is active.
      if (!window.location.hash.startsWith('#/logs')) return;
      try {
        const all = (await bindings.getLogs()) || [];
        const keep = all.length > MAX ? all.slice(all.length - MAX) : all;
        const changed =
          keep.length !== this.lines.length ||
          keep[keep.length - 1] !== this.lines[this.lines.length - 1];
        if (changed) this.lines = keep;
        this.$nextTick(() => {
          if (this.autoScroll) {
            const el = this.$refs.logBox;
            if (el) el.scrollTop = el.scrollHeight;
          }
        });
      } catch {
        /* ignore transient poll errors */
      }
    },

    async clear() {
      this.lines = [];
      try {
        await bindings.clearLogs();
      } catch {
        /* ignore */
      }
    },
  };
};
