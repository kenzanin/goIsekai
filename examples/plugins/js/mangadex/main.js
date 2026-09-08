// MangaDex JS plugin for goIsekai — entry point.
// Sibling modules are loaded via the sandboxed require() provided by the
// host (internal/pluginmanager/js.go). Each module declares top-level
// functions that become globals in the plugin's own goja VM.
// Search returns ALL results; the host handles pagination.

require("./util.js");
require("./search.js");
require("./detail.js");
require("./chapters.js");
require("./pages.js");
require("./enrich.js");

var PLUGIN = {
    contract_version: 1,
    name: "MangaDex",
    site_url: "https://mangadex.org",
    logo: "https://mangadex.org/favicon.ico",
    thumb_ratio: 0.703,
    alt_title_servers: [
        { id: "mangadex", name: "MangaDex", kind: "titles" },
        { id: "mangaupdates", name: "MangaUpdates", kind: "both" },
    ],
};
