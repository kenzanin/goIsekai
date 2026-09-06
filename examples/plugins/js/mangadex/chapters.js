// chapters.js — MangaDex get_chapter_list ABI function.

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
