// util.js — MangaDex shared helpers.
// Loaded via require() from main.js before other modules. All top-level
// declarations become globals in the plugin VM (goja). Keep this file
// dependency-free so it can be copy-pasted into other plugins.

var API_URL = "https://api.mangadex.org";
var CDN_URL = "https://uploads.mangadex.org";
var LANG = "en";

function httpGet(url, headers) {
    var h = headers || {};
    h["Referer"] = CDN_URL + "/";
    var resp = http_request(JSON.stringify({ method: "GET", url: url, headers: h }));
    return typeof resp === "string" ? JSON.parse(resp) : resp;
}

function firstTitle(attrs) {
    var t = attrs.title && attrs.title[LANG];
    if (t) return t;
    if (attrs.altTitles) {
        for (var i = 0; i < attrs.altTitles.length; i++) {
            var at = attrs.altTitles[i];
            if (at[LANG]) return at[LANG];
        }
        for (var i = 0; i < attrs.altTitles.length; i++) {
            var keys = Object.keys(attrs.altTitles[i]);
            for (var j = 0; j < keys.length; j++) {
                var v = attrs.altTitles[i][keys[j]];
                if (v) return v;
            }
        }
    }
    if (attrs.title) {
        var keys = Object.keys(attrs.title);
        for (var i = 0; i < keys.length; i++) {
            if (attrs.title[keys[i]]) return attrs.title[keys[i]];
        }
    }
    return "";
}

function firstLang(m) {
    if (!m) return "";
    if (m[LANG]) return m[LANG];
    var keys = Object.keys(m);
    for (var i = 0; i < keys.length; i++) {
        if (m[keys[i]]) return m[keys[i]];
    }
    return "";
}

function coverURL(md) {
    if (!md.relationships) return "";
    for (var i = 0; i < md.relationships.length; i++) {
        var r = md.relationships[i];
        if (r.type === "cover_art" && r.attributes && r.attributes.fileName) {
            return CDN_URL + "/covers/" + md.id + "/" + r.attributes.fileName + ".256.jpg";
        }
    }
    return "";
}

// normalizeStatus maps a raw status string to a canonical host value.
// Canonical set: Ongoing, Completed, Hiatus, Dropped, Upcoming.
// Unknown values pass through as-is.
function normalizeStatus(s) {
    var raw = (s || "").toLowerCase();
    if (raw.indexOf("ongo") === 0 || raw.indexOf("releas") === 0 || raw.indexOf("publish") === 0) return "Ongoing";
    if (raw.indexOf("complet") === 0 || raw.indexOf("finish") === 0) return "Completed";
    if (raw.indexOf("hiatus") === 0 || raw.indexOf("on hold") === 0 || raw.indexOf("onhold") === 0) return "Hiatus";
    if (raw.indexOf("drop") === 0 || raw.indexOf("cancel") === 0) return "Dropped";
    if (raw.indexOf("upcom") === 0) return "Upcoming";
    return s || "";
}

function toManga(md) {
    var tags = [];
    if (md.attributes && md.attributes.tags) {
        for (var i = 0; i < md.attributes.tags.length; i++) {
            var n = firstLang(md.attributes.tags[i].attributes && md.attributes.tags[i].attributes.name);
            if (n) tags.push(n);
        }
    }
    return {
        id: md.id,
        title: firstTitle(md.attributes || {}),
        cover_url: coverURL(md),
        description: firstLang(md.attributes && md.attributes.description),
        status: normalizeStatus(md.attributes && md.attributes.status || ""),
        genres: tags,
    };
}

function parseFloatSafe(s) {
    var f = parseFloat(s);
    return isNaN(f) ? 0 : f;
}

function parseTime(s) {
    if (!s) return "";
    return s; // ISO8601 string, Go side parses it
}

function contentRatingParams() {
    return "contentRating[]=safe&contentRating[]=suggestive&contentRating[]=erotica";
}
