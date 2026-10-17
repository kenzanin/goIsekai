// MangaK (mangak.io) source plugin for goIsekai — C++17 WASM reactor.
//
// 1:1 port of ../mangak/main.go (standard-Go wasip1 reactor).
//
// mangak.io is a Next.js front end over a JSON API at https://api.mangak.io:
//   search:   GET /titles/search?q=..&page=..&genre=..
//   detail:   GET /titles/<hsid>
//   chapters: GET /titles/<hsid>/chapters
//   pages:    GET /titles/<hsid>/chapters/<chapterId>   (images inline)
//   genres:   GET /genres
//
// All plain JSON behind a Referer gate — no HTML scraping.
//
// ID scheme: Manga.ID = API title id (hsid); Chapter.ID = "<hsid>:<chapterId>"
//
// Reuses the shared helpers: wasm_abi.h (host ABI), util.h (text/JSON),
// html_util.h (scraping). JSON via RapidJSON.

#include <string>
#include <vector>
#include <regex>

#include "html_util.h"

static const char *kSite = "https://mangak.io";
static const char *kAPI = "https://api.mangak.io";
static const char *kJSONHeaders =
    "{\"Accept\":\"application/json, text/plain, */*\",\"Referer\":\"https://mangak.io/\"}";

// First number in a string (Go chapterNum).
static double chapter_num(const std::string &title) {
    std::smatch m;
    if (std::regex_search(title, m, std::regex(R"re(\d+(?:\.\d+)?)re"))) {
        return std::atof(m[0].str().c_str());  // -fno-exceptions: no std::stod try/catch
    }
    return 0;
}

// Reads data.genres / data.authors — an array of {name,slug} (or plain names).
static void add_named_array(rapidjson::Value &obj, const char *key, const rapidjson::Value &arr,
                            rapidjson::Document::AllocatorType &alloc, bool names_only) {
    rapidjson::Value out(rapidjson::kArrayType);
    if (arr.IsArray()) {
        for (auto &g : arr.GetArray()) {
            if (!g.IsObject()) continue;
            if (!g.HasMember("name") || !g["name"].IsString()) continue;
            std::string name = g["name"].GetString();
            if (name.empty()) continue;
            if (names_only) {
                out.PushBack(rapidjson::Value(name.c_str(), (rapidjson::SizeType)name.size(), alloc), alloc);
            } else {
                rapidjson::Value gv(rapidjson::kObjectType);
                add_string(gv, "name", name, alloc);
                add_string(gv, "slug", g.HasMember("slug") && g["slug"].IsString() ? g["slug"].GetString() : "", alloc);
                out.PushBack(gv, alloc);
            }
        }
    }
    obj.AddMember(rapidjson::Value(key, (rapidjson::SizeType)strlen(key), alloc), out, alloc);
}

// ─── search ─────────────────────────────────────────────────────────────────

static std::string search_impl(const std::string &arg) {
    rapidjson::Document f;
    f.Parse(arg.c_str());
    std::string query = f.HasMember("query") && f["query"].IsString() ? trim(f["query"].GetString()) : "";
    int page = f.HasMember("page") && f["page"].IsInt() ? f["page"].GetInt() : 1;
    if (page < 1) page = 1;
    std::string genre;
    if (f.HasMember("genres") && f["genres"].IsArray() && f["genres"].Size() > 0 && f["genres"][0].IsString())
        genre = f["genres"][0].GetString();

    // An empty query returns the site's default popular listing (browse view relies on it).
    std::string url = std::string(kAPI) + "/titles/search?q=" + url_encode(query) + "&page=" + std::to_string(page);
    if (!genre.empty()) url += "&genre=" + url_encode(genre);

    std::string body;
    if (!http_body(url, kJSONHeaders, body)) return "[]";

    rapidjson::Document resp;
    resp.Parse(body.c_str());
    if (!resp.IsObject() || !resp.HasMember("data") || !resp["data"].IsObject()) return "[]";
    auto &d = resp["data"];
    if (!d.HasMember("items") || !d["items"].IsArray()) return "[]";

    rapidjson::Document out;
    out.SetArray();
    auto &alloc = out.GetAllocator();
    for (auto &it : d["items"].GetArray()) {
        if (!it.IsObject()) continue;
        std::string id = it.HasMember("id") && it["id"].IsString() ? it["id"].GetString() : "";
        if (id.empty()) continue;
        rapidjson::Value m(rapidjson::kObjectType);
        add_string(m, "id", id, alloc);
        add_string(m, "title", it.HasMember("name") && it["name"].IsString() ? it["name"].GetString() : "", alloc);
        add_string(m, "cover_url",
                   it.HasMember("cover") && it["cover"].IsString() ? it["cover"].GetString() : "", alloc);
        add_string(m, "status",
                   it.HasMember("status") && it["status"].IsString() ? normalize_status(it["status"].GetString()) : "", alloc);
        out.PushBack(m, alloc);
    }
    return to_json(out);
}

// ─── detail ─────────────────────────────────────────────────────────────────

static std::string detail_impl(const std::string &arg) {
    std::string id = trim(decode_string(arg));
    if (id.empty()) return "{}";

    std::string body;
    if (!http_body(std::string(kAPI) + "/titles/" + url_encode(id), kJSONHeaders, body)) return "{}";

    rapidjson::Document resp;
    resp.Parse(body.c_str());
    if (!resp.IsObject() || !resp.HasMember("data") || !resp["data"].IsObject()) return "{}";
    auto &d = resp["data"];
    if (!d.HasMember("title") || !d["title"].IsObject()) return "{}";
    auto &t = d["title"];

    rapidjson::Document m;
    m.SetObject();
    auto &alloc = m.GetAllocator();
    add_string(m, "id", t.HasMember("id") && t["id"].IsString() ? t["id"].GetString() : "", alloc);
    add_string(m, "title", t.HasMember("name") && t["name"].IsString() ? t["name"].GetString() : "", alloc);
    add_string(m, "cover_url", t.HasMember("cover") && t["cover"].IsString() ? t["cover"].GetString() : "", alloc);
    add_string(m, "description",
               t.HasMember("summary") && t["summary"].IsString() ? trim(t["summary"].GetString()) : "", alloc);
    add_string(m, "status",
               t.HasMember("status") && t["status"].IsString() ? normalize_status(t["status"].GetString()) : "", alloc);

    static const rapidjson::Value kEmptyArr(rapidjson::kArrayType);
    add_named_array(m, "genres", t.HasMember("genres") ? t["genres"] : kEmptyArr, alloc, true);

    std::string author;
    if (t.HasMember("authors") && t["authors"].IsArray() && t["authors"].Size() > 0) {
        auto &a0 = t["authors"][0];
        if (a0.IsObject() && a0.HasMember("name") && a0["name"].IsString()) author = trim(a0["name"].GetString());
    }
    add_string(m, "author", author, alloc);
    return to_json(m);
}

// ─── chapters ───────────────────────────────────────────────────────────────

static std::string chapters_impl(const std::string &arg) {
    std::string id = trim(decode_string(arg));
    if (id.empty()) return "[]";

    std::string body;
    if (!http_body(std::string(kAPI) + "/titles/" + url_encode(id) + "/chapters", kJSONHeaders, body)) return "[]";

    rapidjson::Document resp;
    resp.Parse(body.c_str());
    if (!resp.IsObject() || !resp.HasMember("data") || !resp["data"].IsObject()) return "[]";
    auto &d = resp["data"];
    if (!d.HasMember("chapters") || !d["chapters"].IsArray()) return "[]";

    rapidjson::Document out;
    out.SetArray();
    auto &alloc = out.GetAllocator();
    for (auto &c : d["chapters"].GetArray()) {
        if (!c.IsObject()) continue;
        std::string cid = c.HasMember("id") && c["id"].IsString() ? c["id"].GetString() : "";
        if (cid.empty()) continue;
        std::string title = c.HasMember("name") && c["name"].IsString() ? trim(c["name"].GetString()) : "";
        if (title.empty()) title = "Chapter";

        rapidjson::Value ch(rapidjson::kObjectType);
        add_string(ch, "id", id + ":" + cid, alloc);
        add_string(ch, "manga_id", id, alloc);
        add_string(ch, "title", title, alloc);
        ch.AddMember("chapter_num", chapter_num(title), alloc);
        add_string(ch, "released_at",
                   c.HasMember("updated_at") && c["updated_at"].IsString() ? parse_time(c["updated_at"].GetString()) : "",
                   alloc);
        add_string(ch, "url",
                   std::string(kSite) + (c.HasMember("url") && c["url"].IsString() ? c["url"].GetString() : ""), alloc);
        out.PushBack(ch, alloc);
    }
    return to_json(out);
}

// ─── pages ──────────────────────────────────────────────────────────────────

static std::string pages_impl(const std::string &arg) {
    std::string v = trim(decode_string(arg));
    size_t colon = v.find(':');
    if (colon == std::string::npos || colon == 0 || colon + 1 >= v.size()) return "[]";
    std::string id = v.substr(0, colon), chID = v.substr(colon + 1);

    std::string body;
    if (!http_body(std::string(kAPI) + "/titles/" + url_encode(id) + "/chapters/" + url_encode(chID), kJSONHeaders, body))
        return "[]";

    rapidjson::Document resp;
    resp.Parse(body.c_str());
    if (!resp.IsObject() || !resp.HasMember("data") || !resp["data"].IsObject()) return "[]";
    auto &d = resp["data"];
    if (!d.HasMember("chapter") || !d["chapter"].IsObject()) return "[]";
    auto &ch = d["chapter"];
    if (!ch.HasMember("images") || !ch["images"].IsArray()) return "[]";

    rapidjson::Document out;
    out.SetArray();
    auto &alloc = out.GetAllocator();
    int idx = 0;
    for (auto &img : ch["images"].GetArray()) {
        if (!img.IsString() || img.GetStringLength() == 0) continue;
        rapidjson::Value pg(rapidjson::kObjectType);
        pg.AddMember("index", idx++, alloc);
        add_string(pg, "url", img.GetString(), alloc);
        out.PushBack(pg, alloc);
    }
    return to_json(out);
}

// ─── genres ─────────────────────────────────────────────────────────────────

static std::string genres_impl() {
    std::string body;
    if (!http_body(std::string(kAPI) + "/genres", kJSONHeaders, body)) return "[]";

    rapidjson::Document resp;
    resp.Parse(body.c_str());
    if (!resp.IsObject() || !resp.HasMember("data") || !resp["data"].IsObject()) return "[]";
    auto &d = resp["data"];
    if (!d.HasMember("items") || !d["items"].IsArray()) return "[]";

    rapidjson::Document out;
    out.SetArray();
    auto &alloc = out.GetAllocator();
    for (auto &g : d["items"].GetArray()) {
        if (!g.IsObject() || !g.HasMember("name") || !g["name"].IsString() || g["name"].GetStringLength() == 0)
            continue;
        rapidjson::Value gv(rapidjson::kObjectType);
        add_string(gv, "name", g["name"].GetString(), alloc);
        add_string(gv, "slug", g.HasMember("slug") && g["slug"].IsString() ? g["slug"].GetString() : "", alloc);
        out.PushBack(gv, alloc);
    }
    return to_json(out);
}

// ─── exports ────────────────────────────────────────────────────────────────

extern "C" __attribute__((export_name("Search"))) uint64_t Search(uint32_t ptr, uint32_t len) {
    return reply_json(search_impl(read_string(ptr, len)));
}
extern "C" __attribute__((export_name("GetMangaDetail"))) uint64_t GetMangaDetail(uint32_t ptr, uint32_t len) {
    return reply_json(detail_impl(read_string(ptr, len)));
}
extern "C" __attribute__((export_name("GetChapterList"))) uint64_t GetChapterList(uint32_t ptr, uint32_t len) {
    return reply_json(chapters_impl(read_string(ptr, len)));
}
extern "C" __attribute__((export_name("GetPageList"))) uint64_t GetPageList(uint32_t ptr, uint32_t len) {
    return reply_json(pages_impl(read_string(ptr, len)));
}
extern "C" __attribute__((export_name("GetGenres"))) uint64_t GetGenres(uint32_t ptr, uint32_t len) {
    (void)ptr; (void)len;
    return reply_json(genres_impl());
}
