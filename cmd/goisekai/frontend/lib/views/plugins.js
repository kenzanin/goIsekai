import { bindings } from "../bindings.js";

// ── Plugins component ─────────────────────────────────────────────
export const pluginsView = () => ({
  loading: false,
  error: null,

  async init() {
    this.$watch('$store.app.currentView', (v) => {
      if (v === 'plugins') this.load();
    });
    if (this.$store.app.currentView === 'plugins') this.load();
  },

  async load() {
    this.loading = true;
    this.error = null;
    try {
      await this.$store.app.loadPlugins();
    } catch (err) {
      this.error = 'Failed to load plugins';
    } finally {
      this.loading = false;
    }
  },

  async togglePlugin(id) {
    try {
      await bindings.togglePlugin(id);
      await this.load();
    } catch (e) {
      console.error('toggle plugin', e);
    }
  },

  async installPlugin() {
    const input = document.getElementById('plugin-file-input');
    input.click();
  },

  async onFileSelected(e) {
    const file = e.target.files[0];
    if (!file) return;
    try {
      await bindings.installPlugin(file.name);
      e.target.value = '';
      await this.load();
    } catch (err) {
      console.error('install plugin failed:', err);
      alert('Failed to install plugin: ' + err);
    }
  },

  get plugins() { return this.$store.app.plugins; },
});
