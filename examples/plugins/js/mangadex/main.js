// MangaDex JS plugin for goIsekai
// Fetches manga, chapters, and pages from the MangaDex public API (api.mangadex.org).
// Ported from WASM — search returns ALL results; host handles pagination.

var PLUGIN = {
    contract_version: 1,
    name: "MangaDex",
    thumb_ratio: 0.703,
};

var API_URL = "https://api.mangadex.org";
var CDN_URL = "https://uploads.mangadex.org";
var LANG = "en";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

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
        status: md.attributes && md.attributes.status || "",
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

// ---------------------------------------------------------------------------
// ABI functions
// ---------------------------------------------------------------------------

function searchManga(arg) {
    var args = JSON.parse(arg);
    var query = args.query || "";
    var title = query.replace(/^\s+|\s+$/g, "");

    log.info("mangadex search: q=" + title);

    // Fetch ALL results by looping offset pages (limit=100 per request).
    // The host slices by search_page_size — we must not double-paginate.
    var all = [];
    var offset = 0;
    var total = 999999; // set on first response

    while (offset < total) {
        var qs = "limit=100&offset=" + offset;
        qs += "&includes[]=cover_art&includes[]=author";
        qs += "&availableTranslatedLanguage[]=" + LANG;
        if (title === "") {
            qs += "&order[followedCount]=desc";
        } else {
            qs += "&order[relevance]=desc&title=" + encodeURIComponent(title);
        }
        qs += "&" + contentRatingParams();

        var resp = httpGet(API_URL + "/manga?" + qs);
        if (!resp || resp.status < 200 || resp.status >= 300) {
            log.error("mangadex search: HTTP " + (resp ? resp.status : "null") + " at offset=" + offset);
            break;
        }

        var body;
        try {
            body = JSON.parse(resp.body);
        } catch (e) {
            log.error("mangadex search: JSON parse error: " + e);
            break;
        }

        if (body.total !== undefined) total = body.total;
        if (!body.data || body.data.length === 0) break;

        for (var i = 0; i < body.data.length; i++) {
            all.push(toManga(body.data[i]));
        }

        offset += body.data.length;
        if (body.data.length < 100) break; // last page

        // Safety cap: don't fetch more than 2000 results
        if (all.length >= 2000) break;
    }

    log.info("mangadex search: found " + all.length + " results for q=" + title + " (total=" + total + ")");
    return JSON.stringify(all);
}

function getMangaDetail(arg) {
    var mangaID = JSON.parse(arg);
    if (!mangaID) return JSON.stringify(null);

    log.info("mangadex detail: id=" + mangaID);

    var qs = "includes[]=cover_art&includes[]=author&includes[]=artist";
    var resp = httpGet(API_URL + "/manga/" + mangaID + "?" + qs);
    if (!resp || resp.status < 200 || resp.status >= 300) {
        return JSON.stringify(null);
    }

    var body;
    try {
        body = JSON.parse(resp.body);
    } catch (e) {
        return JSON.stringify(null);
    }

    if (!body.data) return JSON.stringify(null);
    return JSON.stringify(toManga(body.data));
}

function getChapterList(arg) {
    var mangaID = JSON.parse(arg);
    if (!mangaID) return JSON.stringify([]);

    log.info("mangadex chapters: id=" + mangaID);

    var all = [];
    var offset = 0;
    var total = 999999;

    while (offset < total) {
        var qs = "limit=500&offset=" + offset;
        qs += "&translatedLanguage[]=" + LANG;
        qs += "&order[volume]=asc&order[chapter]=asc";
        qs += "&includes[]=scanlation_group";
        qs += "&includeEmptyPages=0";
        qs += "&" + contentRatingParams();

        var resp = httpGet(API_URL + "/manga/" + mangaID + "/feed?" + qs);
        if (!resp || resp.status < 200 || resp.status >= 300) break;

        var body;
        try {
            body = JSON.parse(resp.body);
        } catch (e) {
            break;
        }

        if (body.total !== undefined) total = body.total;
        if (!body.data || body.data.length === 0) break;

        for (var i = 0; i < body.data.length; i++) {
            var cd = body.data[i];
            var a = cd.attributes;
            if (a.externalURL) continue;

            var chNum = parseFloatSafe(a.chapter);
            var volNum = parseFloatSafe(a.volume);

            var chTitle = "Chapter " + a.chapter;
            if (a.title) chTitle = a.title;
            if (!a.chapter && !a.title) chTitle = "Oneshot";

            all.push({
                id: cd.id,
                manga_id: mangaID,
                title: chTitle,
                chapter_num: chNum,
                volume_num: volNum,
                released_at: parseTime(a.publishAt),
                url: "https://mangadex.org/chapter/" + cd.id,
            });
        }

        offset += body.data.length;
        if (body.data.length < 500) break;
    }

    log.info("mangadex chapters: found " + all.length + " chapters for " + mangaID);
    return JSON.stringify(all);
}

function getPageList(arg) {
    var chapterID = JSON.parse(arg);
    if (!chapterID) return JSON.stringify([]);

    log.info("mangadex pages: chapter=" + chapterID);

    var resp = httpGet(API_URL + "/at-home/server/" + chapterID);
    if (!resp || resp.status < 200 || resp.status >= 300) {
        return JSON.stringify([]);
    }

    var body;
    try {
        body = JSON.parse(resp.body);
    } catch (e) {
        return JSON.stringify([]);
    }

    if (!body.chapter || !body.chapter.data) return JSON.stringify([]);

    var pages = [];
    for (var i = 0; i < body.chapter.data.length; i++) {
        pages.push({
            index: i,
            url: body.baseUrl + "/data/" + body.chapter.hash + "/" + body.chapter.data[i],
            headers: { Referer: CDN_URL + "/" },
        });
    }

    log.info("mangadex pages: found " + pages.length + " pages for " + chapterID);
    return JSON.stringify(pages);
}

// Optional enricher export: resolve alternative titles for ANY manga title.
// Returns {source, titles} — the host uses `source` as the "via X" badge.
function getAltTitles(arg) {
    var title = JSON.parse(arg);
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
