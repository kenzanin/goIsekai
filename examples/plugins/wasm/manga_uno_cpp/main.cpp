// MangaUno (manga.uno) source plugin for goIsekai — C++17 WASM reactor.
//
// 1:1 port of ../manga_uno/main.go (standard-Go wasip1 reactor).
//
// manga.uno is a Next.js site with a JSON API:
//   search:   GET /api/search/advanced?title=...&genres=...&limit=24
//   genres:   GET /api/genres
//   detail:   GET /api/manga/<slug>       -> {"manga":{...},"chapters":[...]}
//   chapters: GET /api/manga/<slug>       -> top-level "chapters" (same response)
//   pages:    GET /api/chapter/<id>       -> {"chapter":{...},"pages":[...]}
//
// ID scheme: Manga.ID = slug; Chapter.ID = the site's numeric chapter id, so
// GetPageList can feed it straight back to /api/chapter/<id>.
//
// Covers and pages are served from the CDN origin (xz7.fstr-cdn.com), NOT
// manga.uno: manga.uno/covers/*.webp answers an HTML shell (text/html), which
// is what made every cover and reader image fail to load.
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
static const char *kCDN = "https://xz7.fstr-cdn.com";

// Resolve a path the way the site itself does: against the CDN origin, never
// manga.uno (which serves an HTML shell for /covers/*.webp).
static std::string cdn(const std::string &raw) {
    std::string p = trim(raw);
    if (p.empty()) return "";
    if (p.rfind("http://", 0) == 0 || p.rfind("https://", 0) == 0) return p;
    if (p.rfind("//", 0) == 0) return "https:" + p;
    if (p[0] == '/') return std::string(kCDN) + p;
    return std::string(kCDN) + "/" + p;
}

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
        add_string(m, "cover_url", cdn(cover), alloc);
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
    // The detail page reads cover_url from here; without it the hero image is
    // blank even though search shows a cover.
    add_string(m, "cover_url",
               cdn(m0.HasMember("cover") && m0["cover"].IsString() ? trim(m0["cover"].GetString()) : ""), alloc);
    add_string(m, "author", m0.HasMember("author") && m0["author"].IsString() ? trim(m0["author"].GetString()) : "",
               alloc);
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
    // The chapter list rides along on the detail response — /search/advanced
    // returns *other manga*, which is how this used to emit one fake chapter.
    if (!d.IsObject() || !d.HasMember("chapters") || !d["chapters"].IsArray()) return "[]";

    rapidjson::Document out;
    out.SetArray();
    auto &alloc = out.GetAllocator();
    for (auto &c : d["chapters"].GetArray()) {
        if (!c.IsObject()) continue;
        long cid = 0;
        if (c.HasMember("id") && c["id"].IsInt64()) cid = (long)c["id"].GetInt64();
        std::string number =
            c.HasMember("chapter_number") && c["chapter_number"].IsString() ? trim(c["chapter_number"].GetString()) : "";
        std::string title = c.HasMember("title") && c["title"].IsString() ? trim(c["title"].GetString()) : "";
        if (title.empty()) title = "Chapter " + number;
        std::string rel;
        if (c.HasMember("published_at") && c["published_at"].IsString()) rel = c["published_at"].GetString();
        if (rel.empty() && c.HasMember("created_at") && c["created_at"].IsString()) rel = c["created_at"].GetString();

        rapidjson::Value ch(rapidjson::kObjectType);
        add_string(ch, "id", std::to_string(cid), alloc);
        add_string(ch, "manga_id", slug, alloc);
        add_string(ch, "title", title, alloc);
        ch.AddMember("chapter_num", number.empty() ? 0 : std::strtod(number.c_str(), nullptr), alloc);
        add_string(ch, "released_at", parse_time(rel), alloc);
        add_string(ch, "url", std::string(kSite) + "/" + slug, alloc);
        out.PushBack(ch, alloc);
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
    if (!resp.IsObject()) return "[]";

    rapidjson::Document out;
    out.SetArray();
    auto &alloc = out.GetAllocator();

    // Page URLs sit at the TOP level of the response, not under "chapter".
    if (resp.HasMember("pages") && resp["pages"].IsArray()) {
        for (auto &p : resp["pages"].GetArray()) {
            if (!p.IsString() || p.GetStringLength() == 0) continue;
            rapidjson::Value pg(rapidjson::kObjectType);
            pg.AddMember("index", (int)out.Size(), alloc);
            add_string(pg, "url", p.GetString(), alloc);
            out.PushBack(pg, alloc);
        }
    }
    if (out.Size() > 0) return to_json(out);

    // pages omitted (uncached): those URLs all resolve to
    // /pages/<manga_id>/<chapter_id>/<n>.webp on the CDN, so synthesize them.
    if (resp.HasMember("chapter") && resp["chapter"].IsObject()) {
        auto &c = resp["chapter"];
        long mid = c.HasMember("manga_id") && c["manga_id"].IsInt64() ? (long)c["manga_id"].GetInt64() : -1;
        long cid = c.HasMember("id") && c["id"].IsInt64() ? (long)c["id"].GetInt64() : -1;
        int n = c.HasMember("page_count") && c["page_count"].IsInt() ? c["page_count"].GetInt() : 0;
        if (mid > 0 && cid > 0 && n > 0) {
            for (int i = 1; i <= n; i++) {
                rapidjson::Value pg(rapidjson::kObjectType);
                pg.AddMember("index", (int)out.Size(), alloc);
                add_string(pg, "url",
                           std::string(kCDN) + "/pages/" + std::to_string(mid) + "/" + std::to_string(cid) + "/" +
                               std::to_string(i) + ".webp",
                           alloc);
                out.PushBack(pg, alloc);
            }
        }
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
