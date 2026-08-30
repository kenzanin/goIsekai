import { bindings } from "../bindings.js";
import { loadImage } from "../utils.js";

// ── Library component ─────────────────────────────────────────────
export const libraryView = () => ({
  loading: false,
  error: null,
  syncing: false,

  async init() {
    this.$watch('$store.app.currentView', (v) => {
      if (v === 'library') this.load();
    });
    if (this.$store.app.currentView === 'library') this.load();
  },

  async load() {
    this.loading = true;
    this.error = null;
    try {
      await this.$store.app.loadLibrary();
    } catch (err) {
      this.error = 'Failed to load library';
    } finally {
      this.loading = false;
    }
  },

  async sync() {
    this.syncing = true;
    try {
      await bindings.syncLibrary();
      await this.load();
    } catch (e) {
      console.error('sync library', e);
    } finally {
      this.syncing = false;
    }
  },

  get items() {
    return this.$store.app.libraryList || [];
  },
});
