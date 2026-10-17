// MangaUno (manga.uno) source plugin for goIsekai — C++17 WASM reactor.
//
// 1:1 port of ../manga_uno/main.go (standard-Go wasip1 reactor).
//
// manga.uno is a Next.js site with a JSON API:
//   search:   GET /api/search/advanced?title=...&genres=...&limit=24
//   genres:   GET /api/genres
//   detail:   GET /api/manga/<slug>
//   chapters: GET /api/search/advanced?title=<slug>   (mirrors the Go plugin)
//   pages:    GET /api/chapter/<id>
//
// ID scheme: Manga.ID = slug; Chapter.ID = numeric index (as in the Go plugin).
//
// Reuses the shared helpers: wasm_abi.h (host ABI), util.h (text/JSON),
// html_util.h (scraping). JSON via RapidJSON.

#include <string>
#include <vector>

#include "html_util.h"

static const char *kSite = "https://manga.uno";
static const char *kAPI = "https://manga.uno/api";
static const char *kJSONHeaders =
    "{\"Accept\":\"application/json\",\"Referer\":\"https://manga.uno/\"}";

// ─── search ─────────────────────────────────────────────────────────────────

static std::string search_impl(const std::string &arg) {
    rapidjson::Document f;
    f.Parse(arg.c_str());
    std::string query = f.HasMember("query") && f["query"].IsString() ? f["query"].GetString() : "";
    std::string genre;
    if (f.HasMember("genres") && f["genres"].IsArray() && f["genres"].Size() > 0 && f["genres"][0].IsString())
        genre = f["genres"][0].GetString();

    std::string url = std::string(kAPI) + "/search/advanced?limit=24";
    if (!query.empty()) url += "&title=" + url_encode(query);
    if (!genre.empty()) url += "&genres=" + url_encode(genre);

    std::string body;
    if (!http_body(url, kJSONHeaders, body)) return "[]";

    rapidjson::Document resp;
    resp.Parse(body.c_str());
    if (!resp.IsObject() || !resp.HasMember("data") || !resp["data"].IsArray()) return "[]";

    rapidjson::Document out;
    out.SetArray();
    auto &alloc = out.GetAllocator();
    for (auto &it : resp["data"].GetArray()) {
        if (!it.IsObject()) continue;
        std::string slug = it.HasMember("slug") && it["slug"].IsString() ? trim(it["slug"].GetString()) : "";
        std::string cover = it.HasMember("cover") && it["cover"].IsString() ? trim(it["cover"].GetString()) : "";
        rapidjson::Value m(rapidjson::kObjectType);
        add_string(m, "id", slug, alloc);
        add_string(m, "title", it.HasMember("title") && it["title"].IsString() ? trim(it["title"].GetString()) : "", alloc);
        add_string(m, "cover_url", make_abs(kSite, cover), alloc);
        add_string(m, "status",
                   it.HasMember("status") && it["status"].IsString() ? normalize_status(it["status"].GetString()) : "", alloc);
        out.PushBack(m, alloc);
    }
    return to_json(out);
}

// ─── detail ─────────────────────────────────────────────────────────────────

static std::string detail_impl(const std::string &arg) {
    std::string slug = trim(decode_string(arg));
    if (slug.empty()) return "{}";

    std::string body;
    if (!http_body(std::string(kAPI) + "/manga/" + url_encode(slug), kJSONHeaders, body)) return "{}";

    rapidjson::Document resp;
    resp.Parse(body.c_str());
    if (!resp.IsObject() || !resp.HasMember("manga") || !resp["manga"].IsObject()) return "{}";
    auto &m0 = resp["manga"];

    rapidjson::Document m;
    m.SetObject();
    auto &alloc = m.GetAllocator();
    add_string(m, "id", m0.HasMember("slug") && m0["slug"].IsString() ? trim(m0["slug"].GetString()) : slug, alloc);
    add_string(m, "title", m0.HasMember("title") && m0["title"].IsString() ? trim(m0["title"].GetString()) : "", alloc);
    add_string(m, "description",
               m0.HasMember("synopsis") && m0["synopsis"].IsString() ? trim(m0["synopsis"].GetString()) : "", alloc);
    add_string(m, "status",
               m0.HasMember("status") && m0["status"].IsString() ? normalize_status(m0["status"].GetString()) : "", alloc);
    return to_json(m);
}

// ─── chapters ───────────────────────────────────────────────────────────────

static std::string chapters_impl(const std::string &arg) {
    std::string slug = trim(decode_string(arg));
    if (slug.empty()) return "[]";

    std::string body;
    if (!http_body(std::string(kAPI) + "/manga/" + url_encode(slug), kJSONHeaders, body)) return "[]";
    rapidjson::Document d;
    d.Parse(body.c_str());
    if (!d.IsObject() || !d.HasMember("manga") || !d["manga"].IsObject()) return "[]";

    // The Go plugin lists chapters via search/advanced keyed on the slug.
    body.clear();
    if (!http_body(std::string(kAPI) + "/search/advanced?title=" + url_encode(slug), kJSONHeaders, body)) return "[]";
    rapidjson::Document sresp;
    sresp.Parse(body.c_str());
    if (!sresp.IsObject() || !sresp.HasMember("data") || !sresp["data"].IsArray()) return "[]";

    rapidjson::Document out;
    out.SetArray();
    auto &alloc = out.GetAllocator();
    int i = 0;
    for (auto &it : sresp["data"].GetArray()) {
        if (!it.IsObject()) continue;
        std::string title = it.HasMember("title") && it["title"].IsString() ? trim(it["title"].GetString()) : "";
        std::string islug = it.HasMember("slug") && it["slug"].IsString() ? trim(it["slug"].GetString()) : "";
        rapidjson::Value ch(rapidjson::kObjectType);
        add_string(ch, "id", std::to_string(i), alloc);
        add_string(ch, "manga_id", slug, alloc);
        add_string(ch, "title", title, alloc);
        ch.AddMember("chapter_num", (double)i, alloc);
        add_string(ch, "url", std::string(kSite) + "/" + islug, alloc);
        out.PushBack(ch, alloc);
        i++;
    }
    return to_json(out);
}

// ─── pages ──────────────────────────────────────────────────────────────────

static std::string pages_impl(const std::string &arg) {
    std::string chID = trim(decode_string(arg));
    if (chID.empty()) return "[]";

    std::string body;
    if (!http_body(std::string(kAPI) + "/chapter/" + url_encode(chID), kJSONHeaders, body)) return "[]";

    rapidjson::Document resp;
    resp.Parse(body.c_str());
    if (!resp.IsObject() || !resp.HasMember("chapter") || !resp["chapter"].IsObject()) return "[]";
    auto &c = resp["chapter"];
    if (!c.HasMember("pages") || !c["pages"].IsArray()) return "[]";

    rapidjson::Document out;
    out.SetArray();
    auto &alloc = out.GetAllocator();
    int idx = 0;
    for (auto &p : c["pages"].GetArray()) {
        if (!p.IsString() || p.GetStringLength() == 0) continue;
        rapidjson::Value pg(rapidjson::kObjectType);
        pg.AddMember("index", idx++, alloc);
        add_string(pg, "url", p.GetString(), alloc);
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
    if (!resp.IsObject() || !resp.HasMember("genres") || !resp["genres"].IsArray()) return "[]";

    rapidjson::Document out;
    out.SetArray();
    auto &alloc = out.GetAllocator();
    for (auto &n : resp["genres"].GetArray()) {
        if (!n.IsString()) continue;
        std::string name = n.GetString();
        rapidjson::Value g(rapidjson::kObjectType);
        add_string(g, "name", name, alloc);
        add_string(g, "slug", name, alloc);
        out.PushBack(g, alloc);
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
