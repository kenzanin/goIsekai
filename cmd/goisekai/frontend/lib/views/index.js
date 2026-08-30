import { libraryView } from "./library.js";
import { searchView } from "./search.js";
import { detailView } from "./detail.js";
import { pluginsView } from "./plugins.js";
import { settingsView } from "./settings.js";
import { readerView } from "../reader.js";

// Aggregated map so app.js can register every view in one place.
export const viewComponents = {
  libraryView,
  searchView,
  detailView,
  readerView,
  pluginsView,
  settingsView,
};
