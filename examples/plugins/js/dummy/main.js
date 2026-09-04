// Dummy JS plugin for goIsekai
// ABI contract version: 1 (matches pkg/types ContractVersion)
//
// JS plugin contract (mirrors the Lua ABI with snake_case names):
//   - global PLUGIN object with metadata (contract_version, name, ...)
//   - exported functions: search_manga, get_manga_detail, get_chapter_list,
//     get_page_list — each takes one JSON string arg, returns a JSON string
//   - globals: http_request(jsonString) -> jsonString, log.debug/info/warn/error

var PLUGIN = {
    contract_version: 1,
    name: "Dummy JS",
    verify_url: "https://example.com",
    needs_human_verify: false,
    thumb_ratio: 0.703,
    search_page_size: 24,
};

// search_manga(arg) — arg is a JSON object: {"query":"...","page":1}
// Returns: array of {id, title, cover_url}
function searchManga(arg) {
    var args = JSON.parse(arg);
    log.debug("js search q=" + args.query + " page=" + args.page);
    return JSON.stringify([
        { id: "js-1", title: "Dummy JS Manga " + (args.query || ""), cover_url: "https://example.com/cover1.jpg" },
        { id: "js-2", title: "Second Result", cover_url: "https://example.com/cover2.jpg" },
    ]);
}

// get_manga_detail(arg) — arg is a JSON-encoded plain string (e.g. '"js-1"')
// Returns: {id, title, author, description, cover_url, genres, status}
function getMangaDetail(arg) {
    var mangaID = JSON.parse(arg);
    return JSON.stringify({
        id: mangaID,
        title: "Dummy JS Manga",
        author: "Test Author",
        description: "A dummy JS plugin for testing the goja runtime.",
        cover_url: "https://example.com/cover1.jpg",
        genres: ["action", "adventure"],
        status: "ongoing",
    });
}

// get_chapter_list(arg) — arg is a JSON-encoded plain string (manga id)
// Returns: array of {id, manga_id, title, chapter_num, released_at, url}
function getChapterList(arg) {
    var mangaID = JSON.parse(arg);
    var chapters = [];
    for (var i = 1; i <= 3; i++) {
        chapters.push({
            id: mangaID + ":chapter-" + i,
            manga_id: mangaID,
            title: "Chapter " + i,
            chapter_num: i,
            released_at: "2026-01-0" + i + "T00:00:00Z",
            url: "https://example.com/chapter/" + i,
        });
    }
    return JSON.stringify(chapters);
}

// get_page_list(arg) — arg is a JSON-encoded plain string (chapter id)
// Returns: array of {index, url, headers}
function getPageList(arg) {
    var chapterID = JSON.parse(arg);
    return JSON.stringify([
        { index: 0, url: "https://example.com/page1.jpg" },
        { index: 1, url: "https://example.com/page2.jpg" },
    ]);
}
