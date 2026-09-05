// JS test fixture with alt-title-server capability.
var PLUGIN = {
    contract_version: 1,
    name: "Alt Titles Test",
    alt_title_servers: [{ id: "testserver", name: "Test Server" }],
};

function searchManga(arg) {
    var args = JSON.parse(arg);
    if (!args.query) return JSON.stringify([]);
    return JSON.stringify([
        { id: "A1", title: "Alt test", cover_url: "https://example.com/cover.jpg" }
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

// Optional alt-title enricher: parses {"title","server"} and returns fixed data.
function getAltTitles(arg) {
    var input = JSON.parse(arg);
    return JSON.stringify({ source: "TestProvider", titles: ["Alt A", "Alt B"] });
}
