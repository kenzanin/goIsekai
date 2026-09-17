// 1Manga (MangaHub) JS plugin for goIsekai
// Uses MangaHub GraphQL API at api.mghcdn.com.
// Source ID: mn03. The API answers 404 without Origin/Referer on the POST;
// mhub_access is only needed by the chapter (pages) resolver.

var PLUGIN = {
    contract_version: 1,
    name: "1Manga",
    site_url: "https://1manga.co",
    logo: "",
    verify_url: "https://1manga.co",
    needs_human_verify: false,
    thumb_ratio: 0.703,
    search_page_size: 30,
};

var GRAPHQL_URL = "https://api.mghcdn.com/graphql";
var SITE_URL = "https://1manga.co";
var IMG_CDN = "https://imgx.mghcdn.com/";
var THUMB_CDN = "https://thumb.mghcdn.com/";

// Module-level cached access key (refreshed on first call or after error).
var _cachedKey = null;

// Last refusal message reported by the GraphQL API (rate limit, expired key).
var _gqlError = null;

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// Fetch a fresh mhub_access cookie by visiting a chapter URL (not homepage).
// The Kotlin source (MangaHub.kt line 112) refreshes via chapter URL with Referer.
function _fetchAccessKey() {
    // A bare GET only hands back the key this session/IP already exhausted, so
    // every chapter comes back rate limited. Presenting a cookie value the
    // server has never issued makes it mint a genuinely new key, and
    // ?reloadKey=1 is what triggers the re-issue. Measured: each key so obtained
    // carries four chapters of quota, and an explicit Cookie header overrides
    // whatever the host cookie jar holds.
    var refreshUrl = SITE_URL + "/chapter/martial-peak/chapter-" + (1000 + Math.floor(Math.random() * 2000)) + "?reloadKey=1";
    var r = host.http.get(refreshUrl, {
        "Referer": SITE_URL + "/manga/martial-peak",
        "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
        "Sec-Fetch-Dest": "document",
        "Sec-Fetch-Mode": "navigate",
        "Sec-Fetch-Site": "same-origin",
        "Upgrade-Insecure-Requests": "1",
        "Cookie": "mhub_access=0000000000000000000000000000cafe",
    });
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

// Execute a GraphQL query, dropping the cached key and retrying once when the
// API refuses. MangaHub reports both expired keys and its "API rate limit
// excessed" refusal as HTTP 200 with an errors array and a null payload, so the
// status code alone cannot tell a good answer from a refused one. The message is
// kept in _gqlError so callers can surface it instead of a silent empty list.
function _graphqlQuery(query) {
    _gqlError = null;
    for (var attempt = 0; attempt < 2; attempt++) {
        var key = _cachedKey;
        if (!key) {
            key = _fetchAccessKey();
            if (key) _cachedKey = key;
        }
        if (!key) {
            log.error("1manga: no mhub_access key available");
            return null;
        }

        var r = host.http.post(GRAPHQL_URL, JSON.stringify({ query: query }), {
            "Content-Type": "application/json",
            "Origin": SITE_URL,
            "Referer": SITE_URL + "/",
            "x-mhub-access": key,
        });

        if (!r || r.status < 200 || r.status >= 300) {
            log.info("1manga: GraphQL status " + (r ? r.status : "null") + ", refreshing key");
            _cachedKey = null;
            continue;
        }

        var parsed;
        try {
            parsed = JSON.parse(r.body);
        } catch (e) {
            log.error("1manga: JSON parse error: " + e);
            return null;
        }
        if (!parsed) {
            log.error("1manga: empty GraphQL response");
            return null;
        }
        if (parsed.errors && parsed.errors.length) {
            _gqlError = parsed.errors[0].message || "GraphQL error";
        } else if (!parsed.data) {
            _gqlError = "GraphQL response carried no data";
        } else {
            return parsed;
        }
        log.warn("1manga: GraphQL refused (" + _gqlError + "), retrying with a fresh key");
        _cachedKey = null;
    }
    return null;
}

// Escape a string for embedding in a GraphQL string literal.
function _escapeGQL(s) {
    return s.replace(/\\/g, "\\\\").replace(/"/g, '\\"');
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

    // genres arrives as one comma-separated string; the ABI wants a list.
    var genres = (typeof m.genres === "string" ? m.genres.split(",") : (m.genres || []))
        .map(function (s) { return s.trim(); })
        .filter(function (s) { return s !== ""; });

    return JSON.stringify({
        id: m.slug || slug,
        title: m.title || "",
        cover_url: cover,
        author: m.author || "",
        description: m.description || "",
        status: host.text.normalize_status(status || ""),
        genres: genres,
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
            // MangaHub sends epoch seconds, epoch millis or a date string; the
            // host normalizes all three. An unparseable date leaves the key
            // out, so Go keeps time.Time zero instead of failing the decode.
            released_at: host.text.date_to_iso(ch.date) || undefined,
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
        // A refused query (rate limit, expired key) must fail loudly: returning
        // an empty list renders a blank chapter with no explanation.
        if (_gqlError) throw new Error("1manga: " + _gqlError);
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

