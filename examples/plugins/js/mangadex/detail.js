// detail.js — MangaDex get_manga_detail ABI function.

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
