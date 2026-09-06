// alttitles.js — Optional enricher: resolve alternative titles via MangaDex.
// Returns {source, titles} — the host uses `source` as the "via X" badge.
// arg is JSON {"title": string, "server": string}.
// Self-contained: only depends on httpGet() from util.js.

function getAltTitles(arg) {
    var input = JSON.parse(arg);
    var title = input.title || "";
    var qs = "title=" + encodeURIComponent(title) + "&limit=5&includes[]=manga";
    var resp = httpGet(API_URL + "/manga?" + qs);
    var body = typeof resp === "string" ? JSON.parse(resp) : JSON.parse(resp.body);
    var data = body && body.data;
    if (!data || data.length === 0) {
        return JSON.stringify({ source: "MangaDex", titles: [] });
    }
    var attrs = data[0].attributes;
    var out = [];
    var seen = {};
    var keepLangs = { "en": true, "ja": true, "ja-ro": true, "ko": true, "ko-ro": true };
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
    return JSON.stringify({ source: "MangaDex", titles: out });
}
