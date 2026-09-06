// search.js — MangaDex search ABI function.

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
