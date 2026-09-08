// enrich.js — generic alt-title/alt-summary enrichment providers (JS).
// Copy this file into any JS plugin folder unchanged and require() it from
// main.js BEFORE the PLUGIN declaration, e.g.:
//   require("./enrich.js");
// Then declare the servers you want in PLUGIN.alt_title_servers:
//   alt_title_servers: [
//       { id: "mangadex", name: "MangaDex", kind: "titles" },
//       { id: "mangaupdates", name: "MangaUpdates", kind: "both" },
//   ],
// kind: "titles" (default) = appears in the alt-title picker only;
//       "summaries"         = alt-summary picker only;
//       "both"              = shows up in both pickers.
//
// Self-contained: only the sandbox global http_request is used (no httpGet,
// no per-site globals like API_URL — enrichment hits external APIs directly).
// require() runs this file into the plugin's own goja VM, so the functions
// below become globals; strip any inline getAltTitles/getAltSummary from
// your main.js before requiring this file (a later definition would win).
//
// ABI exports provided:
//   getAltTitles(arg)   arg {"title","server"} -> {source, titles[]}
//   getAltSummary(arg)  arg {"title","server"} -> {source, summaries[]}
//     server values handled: "mangadex", "mangaupdates"
//     unknown server -> empty result for the requested kind

// Local GET/POST over the sandbox http_request global.
function enrichHttp(url, opts) {
    var method = (opts && opts.method) || "GET";
    var headers = (opts && opts.headers) || {};
    var body = (opts && opts.body) || "";
    var resp = http_request(JSON.stringify({
        method: method,
        url: url,
        headers: headers,
        body: body
    }));
    if (typeof resp === "string") {
        try { resp = JSON.parse(resp); } catch (e) { return null; }
    }
    return resp;
}

// Search the MangaUpdates v1 API by title; returns best-match record or null.
function mangaupdatesSearch(title) {
    var resp = enrichHttp("https://api.mangaupdates.com/v1/series/search", {
        method: "POST",
        headers: { "Content-Type": "application/json", "Accept": "application/json" },
        body: JSON.stringify({ search: title, stype: "title", perpage: 5 })
    });
    if (!resp || resp.status !== 200) return null;
    try {
        var body = JSON.parse(resp.body);
        if (!body || !body.results || body.results.length === 0) return null;
        return body.results[0].record; // API sorts by relevance
    } catch (e) { return null; }
}

// Fetch the full MangaUpdates series record (has associated[] alt titles).
function mangaupdatesDetail(seriesId) {
    var resp = enrichHttp("https://api.mangaupdates.com/v1/series/" + seriesId);
    if (!resp || resp.status !== 200) return null;
    try { return JSON.parse(resp.body); } catch (e) { return null; }
}

function mangadexAltTitles(title) {
    var qs = "title=" + encodeURIComponent(title) + "&limit=5&includes[]=manga";
    var resp = enrichHttp("https://api.mangadex.org/manga?" + qs);
    if (!resp || resp.status < 200 || resp.status >= 300) {
        return JSON.stringify({ source: "MangaDex", titles: [] });
    }
    var body;
    try { body = JSON.parse(resp.body); } catch (e) {
        return JSON.stringify({ source: "MangaDex", titles: [] });
    }
    var data = body && body.data;
    if (!data || data.length === 0) {
        return JSON.stringify({ source: "MangaDex", titles: [] });
    }
    var attrs = data[0].attributes;
    var out = [], seen = {};
    var keepLangs = { "en": true, "ja": true, "ja-ro": true, "ko": true, "ko-ro": true };
    if (attrs && attrs.altTitles) {
        for (var i = 0; i < attrs.altTitles.length; i++) {
            var alt = attrs.altTitles[i];
            var keys = Object.keys(alt);
            for (var j = 0; j < keys.length; j++) {
                var lang = keys[j];
                if (keepLangs[lang] && alt[lang] && !seen[alt[lang]]) {
                    seen[alt[lang]] = true;
                    out.push(alt[lang]);
                }
            }
        }
    }
    return JSON.stringify({ source: "MangaDex", titles: out });
}

function mangaupdatesAltTitles(title) {
    var record = mangaupdatesSearch(title);
    if (!record) return JSON.stringify({ source: "MangaUpdates", titles: [] });
    var detail = mangaupdatesDetail(record.series_id);
    if (!detail || !detail.associated) {
        return JSON.stringify({ source: "MangaUpdates", titles: [] });
    }
    var out = [], seen = {};
    for (var i = 0; i < detail.associated.length; i++) {
        var t = detail.associated[i].title;
        if (t && !seen[t]) { seen[t] = true; out.push(t); }
    }
    return JSON.stringify({ source: "MangaUpdates", titles: out });
}

// getAltTitles(arg {"title","server"}) -> {"source", "titles":[]}
function getAltTitles(arg) {
    var input = JSON.parse(arg);
    var title = input.title || "";
    var server = input.server || "mangadex";

    if (server === "mangaupdates") {
        return mangaupdatesAltTitles(title);
    }
    // default: MangaDex
    return mangadexAltTitles(title);
}

// getAltSummary(arg {"title","server"}) -> {"source", "summaries":[]}
// Only MangaUpdates is a summary provider today.
function getAltSummary(arg) {
    var input = JSON.parse(arg);
    var title = input.title || "";
    var server = input.server || "mangaupdates";

    if (server !== "mangaupdates") {
        return JSON.stringify({ source: server, summaries: [] });
    }
    var record = mangaupdatesSearch(title);
    if (!record) return JSON.stringify({ source: "MangaUpdates", summaries: [] });
    var desc = record.description || "";
    if (!desc) {
        var detail = mangaupdatesDetail(record.series_id);
        if (detail) desc = detail.description || "";
    }
    if (!desc) return JSON.stringify({ source: "MangaUpdates", summaries: [] });
    return JSON.stringify({ source: "MangaUpdates", summaries: [desc] });
}
