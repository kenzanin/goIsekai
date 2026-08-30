import { readerView } from '../reader.js';
import { detailView } from './detail.js';
import { libraryView } from './library.js';
import { logsView } from './logs.js';
import { pluginsView } from './plugins.js';
import { searchView } from './search.js';
import { settingsView } from './settings.js';

// Aggregated map so app.js can register every view in one place.
export const viewComponents = {
  libraryView,
  searchView,
  detailView,
  readerView,
  pluginsView,
  logsView,
  settingsView,
};
