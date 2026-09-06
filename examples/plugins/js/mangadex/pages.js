// pages.js — MangaDex get_page_list ABI function.

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
