// JS test fixture for the goja runtime.
var PLUGIN = {
    contract_version: 1,
    name: "JS test",
    verify_url: "https://example.com",
    needs_human_verify: true,
    thumb_ratio: 0.7,
};

function searchManga(arg) {
    var args = JSON.parse(arg);
    if (!args.query) return JSON.stringify([]);
    return JSON.stringify([
        { id: "J1", title: "JS test", cover_url: "https://example.com/cover.jpg" }
    ]);
}

function getMangaDetail(arg) {
    var id = JSON.parse(arg);
    return JSON.stringify({
        id: id, title: "Detail " + id, cover_url: "https://example.com/cover.jpg"
    });
}

function getChapterList(arg) {
    var id = JSON.parse(arg);
    return JSON.stringify([
        { id: id + ":c1", manga_id: id, title: "C1", chapter_num: 1, released_at: "2026-01-01T00:00:00Z", url: "https://example.com/c1" }
    ]);
}

function getPageList(arg) {
    var id = JSON.parse(arg);
    return JSON.stringify([
        { index: 0, url: "https://example.com/img/1.png" }
    ]);
}
