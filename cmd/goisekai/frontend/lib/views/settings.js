import { bindings } from '../bindings.js';
import { saveSetting, settings } from '../state.js';

// ── Settings component ────────────────────────────────────────────
export const settingsView = () => {
  const s = settings;
  return {
    renderMode: s.renderMode,
    gpuCompositing: s.gpuCompositing,
    readAhead: s.readAhead,
    direction: s.direction,
    viewMode: s.viewMode,
    configPath: '',
    configStatus: '',

    init() {
      bindings
        .getConfigPath()
        .then((p) => {
          this.configPath = p;
        })
        .catch(() => {});
    },

    persist() {
      settings.renderMode = this.renderMode;
      settings.gpuCompositing = this.gpuCompositing;
      settings.readAhead = this.readAhead;
      settings.direction = this.direction;
      settings.viewMode = this.viewMode;
      saveSetting('renderMode', this.renderMode);
      saveSetting('gpuCompositing', String(this.gpuCompositing));
      saveSetting('readAhead', String(this.readAhead));
      saveSetting('direction', this.direction);
      saveSetting('viewMode', this.viewMode);
    },

    reloadConfig() {
      this.configStatus = 'Reloading...';
      bindings
        .reloadConfig()
        .then(() => {
          this.configStatus = 'Config reloaded ✓';
          setTimeout(() => {
            this.configStatus = '';
          }, 3000);
        })
        .catch((e) => {
          this.configStatus = `Error: ${e?.message || e}`;
        });
    },

    setReadAhead(v) {
      const n = parseInt(v, 10);
      this.readAhead = Number.isNaN(n) ? 0 : Math.max(0, Math.min(10, n));
    },
  };
};
