// 1Manga (MangaHub) JS plugin for goIsekai
// Uses MangaHub GraphQL API at api.mghcdn.com.
// Source ID: mn03 — requires mhub_access cookie for authentication.

var PLUGIN = {
    contract_version: 1,
    name: "1Manga",
    site_url: "https://1manga.co",
    logo: "",
    verify_url: "https://1manga.co",
    needs_human_verify: false,
    thumb_ratio: 0.703,
    search_page_size: 30,
    alt_title_servers: [{id: "mangadex", name: "MangaDex"}],
};

var GRAPHQL_URL = "https://api.mghcdn.com/graphql";
var SITE_URL = "https://1manga.co";
var IMG_CDN = "https://imgx.mghcdn.com";
var THUMB_CDN = "https://thumb.mghcdn.com";

// Module-level cached access key (refreshed on first call or after error).
var _cachedKey = null;

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// Fetch a fresh mhub_access cookie by visiting a chapter URL (not homepage).
// The Kotlin source (MangaHub.kt line 112) refreshes via chapter URL with Referer.
function _fetchAccessKey() {
    var refreshUrl = SITE_URL + "/chapter/martial-peak/chapter-" + (1000 + Math.floor(Math.random() * 2000));
    var resp = http_request(JSON.stringify({
        method: "GET",
        url: refreshUrl,
        headers: {
            "Referer": SITE_URL + "/manga/martial-peak",
            "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
            "Sec-Fetch-Dest": "document",
            "Sec-Fetch-Mode": "navigate",
            "Sec-Fetch-Site": "same-origin",
            "Upgrade-Insecure-Requests": "1",
        },
    }));
    var r = typeof resp === "string" ? JSON.parse(resp) : resp;
    if (!r || !r.headers) return null;

    // Go flattens multi-valued headers with ", " — find mhub_access=VALUE
    var sc = r.headers["Set-Cookie"] || "";
    var idx = sc.indexOf("mhub_access=");
    if (idx < 0) return null;
    var start = idx + "mhub_access=".length;
    var end = sc.indexOf(";", start);
    if (end < 0) end = sc.length;
    return sc.substring(start, end);
}

// Execute a GraphQL query with retry on auth failure.
function _graphqlQuery(query) {
    var key = _cachedKey;
    if (!key) {
        key = _fetchAccessKey();
        if (key) _cachedKey = key;
    }
    if (!key) {
        log.error("1manga: no mhub_access key available");
        return null;
    }

    var resp = http_request(JSON.stringify({
        method: "POST",
        url: GRAPHQL_URL,
        headers: {
            "Content-Type": "application/json",
            "x-mhub-access": key,
        },
        body: JSON.stringify({ query: query }),
    }));
    var r = typeof resp === "string" ? JSON.parse(resp) : resp;

    if (!r || r.status < 200 || r.status >= 300) {
        // Token expired — refresh and retry once.
        log.info("1manga: GraphQL status " + (r ? r.status : "null") + ", refreshing key");
        _cachedKey = null;
        key = _fetchAccessKey();
        if (key) _cachedKey = key;
        if (!key) return null;

        resp = http_request(JSON.stringify({
            method: "POST",
            url: GRAPHQL_URL,
            headers: {
                "Content-Type": "application/json",
                "x-mhub-access": key,
            },
            body: JSON.stringify({ query: query }),
        }));
        r = typeof resp === "string" ? JSON.parse(resp) : resp;
        if (!r || r.status < 200 || r.status >= 300) return null;
    }

    try {
        return JSON.parse(r.body);
    } catch (e) {
        log.error("1manga: JSON parse error: " + e);
        return null;
    }
}

// Escape a string for embedding in a GraphQL string literal.
function _escapeGQL(s) {
    return s.replace(/\\/g, "\\\\").replace(/"/g, '\\"');
}

// Normalize MangaHub's date field to an RFC3339 string (or undefined when
// unparseable, so the key is omitted and Go keeps time.Time zero). MangaHub
// sends either epoch seconds, epoch millis, or a parseable date string.
function _toISO(dateVal) {
    if (!dateVal) return undefined;
    var ms;
    if (typeof dateVal === "number") {
        ms = dateVal > 1e12 ? dateVal : dateVal * 1000; // ms vs seconds
    } else {
        ms = Date.parse(dateVal);
    }
    if (isNaN(ms)) return undefined;
    return new Date(ms).toISOString();
}

// Normalize a source status string to the host's canonical vocabulary
// (Ongoing/Completed/Hiatus/Dropped/Upcoming). Unrecognized values pass through.
function normalizeStatus(s) {
    var raw = (s || "").toLowerCase().replace(/-/g, "");
    if (raw.indexOf("ongo") === 0 || raw.indexOf("releas") === 0 || raw.indexOf("publish") === 0) return "Ongoing";
    if (raw.indexOf("complet") === 0 || raw.indexOf("finish") === 0) return "Completed";
    if (raw.indexOf("hiatus") === 0 || raw.indexOf("onhold") === 0) return "Hiatus";
    if (raw.indexOf("drop") === 0 || raw.indexOf("cancel") === 0) return "Dropped";
    if (raw.indexOf("upcom") === 0) return "Upcoming";
    return s || "";
}

// ---------------------------------------------------------------------------
// ABI functions
// ---------------------------------------------------------------------------

function searchManga(arg) {
    var args = JSON.parse(arg);
    var query = args.query || "";
    var page = args.page || 1;
    var offset = (page - 1) * 30;

    log.info("1manga search: q=" + query + " page=" + page);

    var gql = '{search(x: mn03, q: "' + _escapeGQL(query)
        + '", genre: "all", mod: POPULAR, offset: '
        + offset + ') {rows {title, slug, image}}}';

    var data = _graphqlQuery(gql);
    if (!data || !data.data || !data.data.search) {
        return JSON.stringify([]);
    }

    var rows = data.data.search.rows || [];
    var results = [];
    for (var i = 0; i < rows.length; i++) {
        var r = rows[i];
        var cover = r.image || "";
        if (cover && cover.indexOf("http") !== 0) {
            cover = THUMB_CDN + cover;
        }
        results.push({
            id: r.slug,
            title: r.title || "",
            cover_url: cover,
        });
    }

    log.info("1manga search: found " + results.length + " results for q=" + query);
    return JSON.stringify(results);
}

function getMangaDetail(arg) {
    var slug = JSON.parse(arg);
    if (!slug) return JSON.stringify(null);

    log.info("1manga detail: slug=" + slug);

    var gql = '{manga(x: mn03, slug: "' + _escapeGQL(slug)
        + '") {title, slug, status, image, author, artist, genres, description, alternativeTitle}}';

    var data = _graphqlQuery(gql);
    if (!data || !data.data || !data.data.manga) {
        return JSON.stringify(null);
    }

    var m = data.data.manga;
    var cover = m.image || "";
    if (cover && cover.indexOf("http") !== 0) {
        cover = THUMB_CDN + cover;
    }

    // status may come as an enum string or numeric code — coerce so the host
    // (types.Manga.Status string) can unmarshal it.
    var status = m.status;
    if (status !== undefined && status !== null && typeof status !== "string") {
        status = String(status);
    }

    return JSON.stringify({
        id: m.slug || slug,
        title: m.title || "",
        cover_url: cover,
        author: m.author || "",
        description: m.description || "",
        status: normalizeStatus(status) || "",
        genres: m.genres || [],
    });
}

function getChapterList(arg) {
    var slug = JSON.parse(arg);
    if (!slug) return JSON.stringify([]);

    log.info("1manga chapters: slug=" + slug);

    var gql = '{manga(x: mn03, slug: "' + _escapeGQL(slug)
        + '") {chapters {number, title, date}}}';

    var data = _graphqlQuery(gql);
    if (!data || !data.data || !data.data.manga || !data.data.manga.chapters) {
        return JSON.stringify([]);
    }

    var raw = data.data.manga.chapters;
    var chapters = [];
    for (var i = 0; i < raw.length; i++) {
        var ch = raw[i];
        var num = ch.number;
        if (typeof num === "string") num = parseFloat(num);
        if (isNaN(num)) num = 0;

        chapters.push({
            id: slug + ":chapter-" + ch.number,
            manga_id: slug,
            title: ch.title || ("Chapter " + ch.number),
            chapter_num: num,
            released_at: _toISO(ch.date),
            url: "https://1manga.co/" + slug + "/" + ch.number,
        });
    }

    // GraphQL returns ascending — reverse for newest-first (ABI convention).
    chapters.sort(function (a, b) { return b.chapter_num - a.chapter_num; });

    log.info("1manga chapters: found " + chapters.length + " chapters for " + slug);
    return JSON.stringify(chapters);
}

function getPageList(arg) {
    var chapterID = JSON.parse(arg);
    if (!chapterID) return JSON.stringify([]);

    // chapterID is "SLUG:chapter-NUMBER"
    var parts = chapterID.split(":chapter-");
    var slug = parts[0];
    var number = parts[1];
    log.info("1manga pages: slug=" + slug + " ch=" + number);

    var gql = '{chapter(x: mn03, slug: "' + _escapeGQL(slug)
        + '", number: ' + number + ') {pages, mangaID, number}}';

    var data = _graphqlQuery(gql);
    if (!data || !data.data || !data.data.chapter) {
        return JSON.stringify([]);
    }

    var ch = data.data.chapter;
    var pagesJSON;
    try {
        pagesJSON = JSON.parse(ch.pages);
    } catch (e) {
        log.error("1manga pages: failed to parse pages JSON: " + e);
        return JSON.stringify([]);
    }

    var prefix = pagesJSON.p || "";
    var images = pagesJSON.i || [];
    var pages = [];
    for (var i = 0; i < images.length; i++) {
        pages.push({
            index: i,
            url: IMG_CDN + prefix + images[i],
            headers: {},
        });
    }

    log.info("1manga pages: found " + pages.length + " pages for " + chapterID);
    return JSON.stringify(pages);
}

// Optional enricher export: resolve alternative titles via MangaDex.
function getAltTitles(arg) {
    var input = JSON.parse(arg);
    var title = input.title || "";
    var qs = "title=" + encodeURIComponent(title) + "&limit=5&includes[]=manga";
    var resp = http_request(JSON.stringify({ method: "GET", url: "https://api.mangadex.org/manga?" + qs, headers: {} }));
    resp = typeof resp === "string" ? JSON.parse(resp) : resp;
    if (!resp || resp.status < 200 || resp.status >= 300) {
        return JSON.stringify({ source: "MangaDex", titles: [] });
    }
    var body;
    try {
        body = JSON.parse(resp.body);
    } catch (e) {
        return JSON.stringify({ source: "MangaDex", titles: [] });
    }
    var data = body && body.data;
    if (!data || data.length === 0) {
        return JSON.stringify({ source: "MangaDex", titles: [] });
    }
    var attrs = data[0].attributes;
    var out = [];
    var seen = {};
    var keepLangs = { "en": true, "ja": true, "ja-ro": true, "ko": true, "ko-ro": true };
    for (var i = 0; i < (attrs.altTitles || []).length; i++) {
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
