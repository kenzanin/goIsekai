// goIsekai frontend — centralized Wails v3 bridge access.
// All service calls funnel through `bindings` / `call`.

import { Call } from '/wails/runtime.js';

export const SVC = 'goisekai/internal/bridge.AppService';

export const bindings = {
  search: (pluginID, filter) => call('SearchManga', pluginID, filter),
  mangaDetails: (pluginID, mangaID) => call('GetMangaDetails', pluginID, mangaID),
  pageList: (pluginID, chapterID) => call('GetPageList', pluginID, chapterID),
  toggleLibrary: (pluginID, mangaID) => call('ToggleLibraryItem', pluginID, mangaID),
  installPlugin: (wasmPath) => call('InstallPlugin', wasmPath),
  recordRead: (pluginID, mangaID, chapterID, pageNum) =>
    call('RecordRead', pluginID, mangaID, chapterID, pageNum),
  setChapterProgress: (pluginID, mangaID, chapterID, lastPage) =>
    call('SetChapterProgress', pluginID, mangaID, chapterID, lastPage),
  listLibrary: () => call('ListLibrary'),
  listPlugins: () => call('ListPlugins'),
  togglePlugin: (id) => call('TogglePlugin', id),
  syncLibrary: () => call('SyncLibrary'),
  getImage: (pluginID, url, headers, mangaID, chapterID) =>
    call('GetImage', pluginID, url, headers, mangaID, chapterID),
  log: (level, msg) => call('Log', level, msg),
  reloadConfig: () => call('ReloadConfig'),
  getConfigPath: () => call('GetConfigPath'),
  evictImageCache: (pluginID, url, mangaID, chapterID) =>
    call('EvictImageCache', pluginID, url, mangaID, chapterID),
};

export function call(method, ...args) {
  return Call.ByName(`${SVC}.${method}`, ...args);
}
