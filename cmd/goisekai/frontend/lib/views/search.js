import { bindings } from "../bindings.js";

// ── Search component ──────────────────────────────────────────────
export const searchView = () => ({
  pluginID: '',
  query: '',
  page: 1,
  results: [],
  loading: false,
  error: null,
  searched: false,

  async init() {
    if (!this.$store.app.plugins.length) await this.$store.app.loadPlugins();
    if (this.$store.app.plugins.length === 1) this.pluginID = this.$store.app.plugins[0].ID;
    this.$watch('$store.app.currentView', (v) => {
      if (v === 'search' && !this.$store.app.plugins.length) this.$store.app.loadPlugins();
    });
  },

  async search() {
    if (!this.pluginID || !this.query.trim()) return;
    this.loading = true;
    this.error = null;
    this.searched = true;
    try {
      const items = await bindings.search(this.pluginID, { query: this.query.trim(), page: this.page });
      this.results = items || [];
    } catch (err) {
      this.error = 'Search failed';
      this.results = [];
    } finally {
      this.loading = false;
    }
  },

  nextPage() { this.page++; this.search(); },
  prevPage() { if (this.page > 1) { this.page--; this.search(); } },

  get hasResults() { return this.results.length > 0; },
  get showEmpty() { return this.searched && !this.loading && !this.error && this.results.length === 0; },
});
