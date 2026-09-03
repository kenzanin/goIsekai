// Mangzio JS plugin for goIsekai
// Parses Next.js RSC flight data from SSR HTML pages.
// Contract version: 1 (matches pkg/types ContractVersion)

var PLUGIN = {
    contract_version: 1,
    name: "Mangzio",
    verify_url: "https://www.mangzio.com",
    needs_human_verify: false,
    thumb_ratio: 0.703,
    search_page_size: 24,
};

var BASE = "https://www.mangzio.com";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function httpGet(url) {
    var resp = http_request(JSON.stringify({ method: "GET", url: url, headers: {} }));
    return typeof resp === "string" ? JSON.parse(resp) : resp;
}

// Extract all RSC flight data chunks from a Next.js SSR HTML page.
// Returns an array of decoded strings.
function extractRSCChunks(html) {
    var chunks = [];
    var re = /self\.__next_f\.push\(\[1,"(.+?)"\]\)/g;
    var m;
    while ((m = re.exec(html)) !== null) {
        // Decode unicode escapes
        var s = m[1].replace(/\\u([0-9a-fA-F]{4})/g, function (_, hex) {
            return String.fromCharCode(parseInt(hex, 16));
        });
        s = s.replace(/\\"/g, '"').replace(/\\\\/g, "\\");
        chunks.push(s);
    }
    return chunks;
}

// Find the first chunk containing a target key and extract a JSON object
// that contains that key. Uses a brace-matching approach.
function findJSONObject(chunks, key) {
    for (var i = 0; i < chunks.length; i++) {
        var idx = chunks[i].indexOf(key);
        if (idx < 0) continue;
        // Scan backward to find the opening {
        var start = chunks[i].lastIndexOf("{", idx);
        if (start < 0) continue;
        var depth = 0;
        var end = start;
        for (var j = start; j < Math.min(start + 10000, chunks[i].length); j++) {
            if (chunks[i][j] === "{") depth++;
            else if (chunks[i][j] === "}") depth--;
            if (depth === 0) { end = j + 1; break; }
        }
        var raw = chunks[i].substring(start, end);
        try {
            return JSON.parse(raw);
        } catch (e) {
            // try next chunk
        }
    }
    return null;
}

// Strip HTML tags from a string.
function stripHTML(s) {
    if (!s) return "";
    return s.replace(/<[^>]*>/g, "").trim();
}

// ---------------------------------------------------------------------------
// ABI functions
// ---------------------------------------------------------------------------

function searchManga(arg) {
    var args = JSON.parse(arg);
    var query = args.query || "";
    var page = args.page || 1;

    log.info("mangzio search: q=" + query + " page=" + page);

    // Mangzio search is server-rendered at /en?q={query}
    // All results are on the first page; pagination not supported by the site.
    var resp = httpGet(BASE + "/en?q=" + encodeURIComponent(query));
    if (!resp || resp.status !== 200) {
        log.error("mangzio search: HTTP " + (resp ? resp.status : "null"));
        return JSON.stringify([]);
    }

    var html = resp.body;
    var chunks = extractRSCChunks(html);
    var results = [];

    // The manga list is in a chunk containing multiple manga objects with "slug" and "coverImageUrl"
    for (var ci = 0; ci < chunks.length; ci++) {
        var chunk = chunks[ci];
        // Each manga object has "slug":..., "title":..., "coverImageUrl":...
        // Find all objects that have coverImageUrl (manga cards)
        var re = /"slug":"([^"]+)","baseSlug":"[^"]*","title":"([^"]+)","alternativeTitle":"[^"]*","coverImageUrl":"([^"]+)"/g;
        var m;
        while ((m = re.exec(chunk)) !== null) {
            var slug = m[1];
            var title = m[2];
            var cover = m[3];
            // Deduplicate by slug
            var exists = false;
            for (var k = 0; k < results.length; k++) {
                if (results[k].id === slug) { exists = true; break; }
            }
            if (!exists && slug !== "manga") {
                results.push({
                    id: slug,
                    title: title,
                    cover_url: cover.indexOf("http") === 0 ? cover : BASE + cover,
                });
            }
        }
    }

    // Mangzio embeds its full catalog in SSR RSC data but does search client-side.
    // Filter locally by title substring (case-insensitive).
    if (query) {
        var q = query.toLowerCase();
        var filtered = [];
        for (var fi = 0; fi < results.length; fi++) {
            if (results[fi].title.toLowerCase().indexOf(q) >= 0) {
                filtered.push(results[fi]);
            }
        }
        results = filtered;
    }
    // Return ALL filtered results — the host handles pagination via search_page_size.
    log.info("mangzio search: found " + results.length + " results for query=" + query);
    return JSON.stringify(results);
}

function getMangaDetail(arg) {
    var slug = JSON.parse(arg);
    log.info("mangzio detail: slug=" + slug);

    var resp = httpGet(BASE + "/en/" + encodeURIComponent(slug));
    if (!resp || resp.status !== 200) {
        log.error("mangzio detail: HTTP " + (resp ? resp.status : "null"));
        return JSON.stringify(null);
    }

    var html = resp.body;
    var chunks = extractRSCChunks(html);

    // Find the manga object with the full metadata
    var manga = findJSONObject(chunks, "\"slug\":\"" + slug + "\"");
    if (!manga) {
        log.error("mangzio detail: manga object not found for " + slug);
        return JSON.stringify(null);
    }

    // Also extract synopsis from JSON-LD as fallback
    var synopsis = manga.synopsis || "";
    if (!synopsis) {
        var ldMatch = html.match(/"description":"([^"]{10,})","image"/);
        if (ldMatch) synopsis = ldMatch[1];
    }

    var result = {
        id: manga.slug || slug,
        title: manga.title || "",
        author: manga.author || "",
        description: stripHTML(synopsis),
        cover_url: manga.coverImageUrl ? (manga.coverImageUrl.indexOf("http") === 0 ? manga.coverImageUrl : BASE + manga.coverImageUrl) : "",
        genres: manga.genres || [],
        status: (manga.status || "").toLowerCase(),
    };

    return JSON.stringify(result);
}

function getChapterList(arg) {
    var slug = JSON.parse(arg);
    log.info("mangzio chapters: slug=" + slug);

    var resp = httpGet(BASE + "/en/" + encodeURIComponent(slug));
    if (!resp || resp.status !== 200) {
        log.error("mangzio chapters: HTTP " + (resp ? resp.status : "null"));
        return JSON.stringify([]);
    }

    var html = resp.body;
    var chunks = extractRSCChunks(html);
    // Concatenate all chunks — the chapters array may span chunk boundaries
    var combined = chunks.join("");

    var chapters = [];
    var allChIdx = combined.indexOf('\"chapters\":[');
    if (allChIdx >= 0) {
        var arrStart = combined.indexOf("[", allChIdx);
        if (arrStart >= 0) {
            var depth = 0;
            var arrEnd = arrStart;
            for (var j = arrStart; j < combined.length; j++) {
                if (combined[j] === "[") depth++;
                else if (combined[j] === "]") depth--;
                if (depth === 0) { arrEnd = j + 1; break; }
            }
                try {
                    var arr = JSON.parse(combined.substring(arrStart, arrEnd));
                    for (var k = 0; k < arr.length; k++) {
                        var ch = arr[k];
                        chapters.push({
                            id: slug + ":chapter-" + ch.chapterNumber,
                            manga_id: slug,
                            title: ch.chapterTitle || "Chapter " + ch.chapterNumber,
                            chapter_num: ch.chapterNumber,
                            released_at: ch.releaseDate || undefined,
                            url: "/en/" + slug + "-en-chapter-" + ch.chapterNumber,
                        });
                    }
                    // ABI convention: newest chapter first (site order is ascending).
                    chapters.sort(function (a, b) { return b.chapter_num - a.chapter_num; });
            } catch (e) {
                log.error("mangzio chapters: parse error: " + e);
            }
        }
    }

    log.info("mangzio chapters: found " + chapters.length + " chapters");
    return JSON.stringify(chapters);
}

function getPageList(arg) {
    var chapterID = JSON.parse(arg);
    // chapterID is "slug:chapter-N"
    var parts = chapterID.split(":chapter-");
    var slug = parts[0];
    var chapterNum = parts[1];
    log.info("mangzio pages: slug=" + slug + " ch=" + chapterNum);

    var chapterURL = BASE + "/en/" + slug + "-en-chapter-" + chapterNum;
    var resp = httpGet(chapterURL);
    if (!resp || resp.status !== 200) {
        log.error("mangzio pages: HTTP " + (resp ? resp.status : "null"));
        return JSON.stringify([]);
    }

    var html = resp.body;
    var chunks = extractRSCChunks(html);

    // Find pageImageUrls array in the RSC data
    for (var ci = 0; ci < chunks.length; ci++) {
        var chunk = chunks[ci];
        var key = "\"pageImageUrls\"";
        var idx = chunk.indexOf(key);
        if (idx < 0) continue;

        // Extract the array after the key
        var arrStart = chunk.indexOf("[", idx);
        if (arrStart < 0) continue;

        var depth = 0;
        var arrEnd = arrStart;
        for (var j = arrStart; j < Math.min(arrStart + 200000, chunk.length); j++) {
            if (chunk[j] === "[") depth++;
            else if (chunk[j] === "]") depth--;
            if (depth === 0) { arrEnd = j + 1; break; }
        }

        try {
            var urls = JSON.parse(chunk.substring(arrStart, arrEnd));
            var pages = [];
            for (var k = 0; k < urls.length; k++) {
                pages.push({
                    index: k,
                    url: urls[k],
                    headers: {},
                });
            }
            log.info("mangzio pages: found " + pages.length + " pages");
            return JSON.stringify(pages);
        } catch (e) {
            log.error("mangzio pages: parse error: " + e);
        }
    }

    log.error("mangzio pages: pageImageUrls not found");
    return JSON.stringify([]);
}
