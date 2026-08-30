// goIsekai frontend — entry point.
// Imports the Wails bindings, Alpine state, and view components, wires them
// into Alpine (store + components inside the alpine:init listener), then sets
// up global routing and keyboard shortcuts.

import { appStore } from "./lib/state.js";
import { viewComponents } from "./lib/views/index.js";

// ── Alpine store + component registration ────────────────────────────────
// Runs inside alpine:init so stores/components exist before Alpine processes
// the DOM. State (store) lives in state.js; view factories in views.js.
document.addEventListener('alpine:init', () => {
  Alpine.store('app', appStore);

  for (const [name, component] of Object.entries(viewComponents)) {
    Alpine.data(name, component);
  }
});

// ── Router + global keyboard shortcuts ───────────────────────────────────
window.addEventListener('hashchange', () => {
  if (window.Alpine) Alpine.store('app').updateRoute();
});

// Resolve the initial hash into currentView on first paint. hashchange never
// fires for the load-time hash, so without this a deep-link like #/read/... 
// leaves currentView on its default ('library') and the target section never mounts.
document.addEventListener('DOMContentLoaded', () => {
  if (window.Alpine) Alpine.store('app').updateRoute();
  // Global keyboard shortcuts (non-reader)
  document.addEventListener('keydown', (e) => {
    const inField = e.target.matches('input, select, textarea');
    if (e.ctrlKey && e.altKey && !inField) {
      if (e.key === '1') { e.preventDefault(); window.location.hash = '#/library'; return; }
      if (e.key === '2') { e.preventDefault(); window.location.hash = '#/search'; return; }
      if (e.key === '3') { e.preventDefault(); window.location.hash = '#/plugins'; return; }
    }
    if (e.key === '/' && !inField) { e.preventDefault(); document.getElementById('search-input')?.focus(); return; }
  });
});
