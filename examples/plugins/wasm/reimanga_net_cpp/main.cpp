// ReiManga (reimanga.net) source plugin for goIsekai — C++17 WASM reactor.
//
// 1:1 port of ../reimanga_net/main.go (standard-Go wasip1 reactor).
//
// reimanga.net exposes a clean JSON API — no HTML scraping:
//   search:   GET /api/manga/search/advanced?esOnly=1&page=N&limit=24&sort=viewed&order=desc&title=...&genres=...
//   detail:   GET /api/manga/<id>                  → {"manga":{...},"similar":[...]}
//   chapters: GET /api/chapters/manga/<id>?limit=N → {"data":[...],"pagination":{...}}
//   pages:    GET /api/chapters/<chapterID>/images → {"chapter_id":..,"images":[...]}
//
// ID scheme: Manga.ID = numeric id as string ("82201")
//            Chapter.ID = "<mangaID>:<chapterID>"
//
// Reuses the shared helpers: wasm_abi.h (host ABI), util.h (text/JSON),
// html_util.h (scraping). JSON via RapidJSON.

#include <string>
#include <vector>

#include "html_util.h"

static const char *kSite = "https://reimanga.net";
static const char *kJSONHeaders =
    "{\"Accept\":\"application/json\",\"Referer\":\"https://reimanga.net/\"}";

// reimanga's `status` is an integer code, not a label.
static std::string status_from_int(int s) {
    switch (s) {
        case 0: return "Ongoing";
        case 1: return "Completed";
        default: return "";
    }
}

static std::string cover_of(long id) {
    return std::string(kSite) + "/covers/" + std::to_string(id) + "/thumbnail.png";
}

// ─── search ─────────────────────────────────────────────────────────────────

static std::string search_impl(const std::string &arg) {
    rapidjson::Document f;
    f.Parse(arg.c_str());
    std::string query = f.HasMember("query") && f["query"].IsString() ? f["query"].GetString() : "";
    int page = f.HasMember("page") && f["page"].IsInt() ? f["page"].GetInt() : 1;
    if (page < 1) page = 1;
    std::vector<std::string> genres;
    if (f.HasMember("genres") && f["genres"].IsArray()) {
        for (auto &g : f["genres"].GetArray())
            if (g.IsString()) genres.push_back(g.GetString());
    }

    std::string url = std::string(kSite) + "/api/manga/search/advanced?esOnly=1&page=" +
                      std::to_string(page) + "&limit=24&sort=viewed&order=desc";
    if (!query.empty()) url += "&title=" + url_encode(query);
    if (!genres.empty()) {
        std::string joined;
        for (size_t i = 0; i < genres.size(); i++) {
            if (i) joined += ",";
            joined += genres[i];
        }
        url += "&genres=" + url_encode(joined);
    }

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
        long id = it.HasMember("id") && it["id"].IsInt() ? it["id"].GetInt() : 0;
        rapidjson::Value m(rapidjson::kObjectType);
        add_string(m, "id", std::to_string(id), alloc);
        add_string(m, "title", it.HasMember("title") && it["title"].IsString() ? trim(it["title"].GetString()) : "", alloc);
        add_string(m, "cover_url", cover_of(id), alloc);
        add_string(m, "status",
                   it.HasMember("status") && it["status"].IsInt() ? status_from_int(it["status"].GetInt()) : "", alloc);
        out.PushBack(m, alloc);
    }
    return to_json(out);
}

// ─── detail ─────────────────────────────────────────────────────────────────

static std::string detail_impl(const std::string &arg) {
    std::string id = trim(decode_string(arg));
    if (id.empty()) return "{}";

    std::string body;
    if (!http_body(std::string(kSite) + "/api/manga/" + url_encode(id), kJSONHeaders, body)) return "{}";

    rapidjson::Document resp;
    resp.Parse(body.c_str());
    // /api/manga/<id> answers {"manga":{...},"similar":[...]}. There is no
    // "data" envelope here — that is the /api/chapters/manga/<id> shape.
    if (!resp.IsObject() || !resp.HasMember("manga") || !resp["manga"].IsObject()) return "{}";
    auto &d = resp["manga"];

    long mid = d.HasMember("id") && d["id"].IsInt() ? d["id"].GetInt() : 0;
    rapidjson::Document m;
    m.SetObject();
    auto &alloc = m.GetAllocator();
    add_string(m, "id", std::to_string(mid), alloc);
    add_string(m, "title", d.HasMember("title") && d["title"].IsString() ? trim(d["title"].GetString()) : "", alloc);
    add_string(m, "cover_url", cover_of(mid), alloc);
    add_string(m, "description",
               d.HasMember("description") && d["description"].IsString() ? trim(d["description"].GetString()) : "", alloc);
    add_string(m, "status",
               d.HasMember("status") && d["status"].IsInt() ? status_from_int(d["status"].GetInt()) : "", alloc);

    // genres/tags are objects ({"id","name","slug"}), not bare strings.
    rapidjson::Value garr(rapidjson::kArrayType);
    if (d.HasMember("genres") && d["genres"].IsArray()) {
        for (auto &g : d["genres"].GetArray()) {
            if (!g.IsObject() || !g.HasMember("name") || !g["name"].IsString()) continue;
            std::string n = trim(g["name"].GetString());
            if (!n.empty()) garr.PushBack(rapidjson::Value(n.c_str(), (rapidjson::SizeType)n.size(), alloc), alloc);
        }
    }
    m.AddMember("genres", garr, alloc);

    std::string author;
    if (d.HasMember("authors") && d["authors"].IsArray()) {
        for (auto &a : d["authors"].GetArray()) {
            if (!a.IsObject() || !a.HasMember("name") || !a["name"].IsString()) continue;
            std::string n = trim(a["name"].GetString());
            if (n.empty()) continue;
            if (!author.empty()) author += ", ";
            author += n;
        }
    }
    add_string(m, "author", author, alloc);
    return to_json(m);
}

// ─── chapters ───────────────────────────────────────────────────────────────

static std::string chapters_impl(const std::string &arg) {
    std::string mangaID = trim(decode_string(arg));
    if (mangaID.empty()) return "[]";

    // Chapters live at /api/chapters/manga/<id>, not under /api/manga/<id>.
    // limit=1000 because the default page is 50: a partial list looks like the
    // manga ends there (144 chapters would arrive as 3 pages of 50).
    std::string body;
    if (!http_body(std::string(kSite) + "/api/chapters/manga/" + url_encode(mangaID) + "?limit=1000", kJSONHeaders,
                   body))
        return "[]";

    rapidjson::Document resp;
    resp.Parse(body.c_str());
    if (!resp.IsObject() || !resp.HasMember("data") || !resp["data"].IsArray()) return "[]";

    rapidjson::Document out;
    out.SetArray();
    auto &alloc = out.GetAllocator();
    for (auto &c : resp["data"].GetArray()) {
        if (!c.IsObject()) continue;
        long cid = c.HasMember("id") && c["id"].IsInt() ? c["id"].GetInt() : 0;
        std::string number =
            c.HasMember("chapter_number") && c["chapter_number"].IsString() ? trim(c["chapter_number"].GetString()) : "";
        std::string title = c.HasMember("name") && c["name"].IsString() ? trim(c["name"].GetString()) : "";
        if (title.empty()) title = "Chapter " + number;
        double num = number.empty() ? 0 : std::strtod(number.c_str(), nullptr);

        rapidjson::Value ch(rapidjson::kObjectType);
        add_string(ch, "id", mangaID + ":" + std::to_string(cid), alloc);
        add_string(ch, "manga_id", mangaID, alloc);
        add_string(ch, "title", title, alloc);
        ch.AddMember("chapter_num", num, alloc);
        add_string(ch, "released_at",
                   c.HasMember("created_at") && c["created_at"].IsString() ? parse_time(c["created_at"].GetString()) : "",
                   alloc);
        out.PushBack(ch, alloc);
    }
    return to_json(out);
}

// ─── pages ──────────────────────────────────────────────────────────────────

static std::string pages_impl(const std::string &arg) {
    std::string chID = trim(decode_string(arg));
    size_t colon = chID.find(':');
    if (colon == std::string::npos || colon == 0 || colon + 1 >= chID.size()) return "[]";
    std::string chapter = chID.substr(colon + 1);

    // Page URLs come from /api/chapters/<id>/images — a bare /api/chapters/<id>
    // returns chapter metadata with no "pages" array at all.
    std::string body;
    if (!http_body(std::string(kSite) + "/api/chapters/" + url_encode(chapter) + "/images", kJSONHeaders, body))
        return "[]";

    rapidjson::Document resp;
    resp.Parse(body.c_str());
    if (!resp.IsObject() || !resp.HasMember("images") || !resp["images"].IsArray()) return "[]";

    rapidjson::Document out;
    out.SetArray();
    auto &alloc = out.GetAllocator();
    int idx = 0;
    for (auto &p : resp["images"].GetArray()) {
        if (!p.IsObject() || !p.HasMember("image_url") || !p["image_url"].IsString()) continue;
        std::string u = trim(p["image_url"].GetString());
        if (u.empty()) continue;
        rapidjson::Value pg(rapidjson::kObjectType);
        pg.AddMember("index", idx++, alloc);
        add_string(pg, "url", u, alloc);
        out.PushBack(pg, alloc);
    }
    return to_json(out);
}

// ─── genres ─────────────────────────────────────────────────────────────────

static std::string genres_impl() {
    // There *is* a genres endpoint; the old hardcoded list made the filter row
    // a different set from what the site actually filters on.
    std::string body;
    if (http_body(std::string(kSite) + "/api/genres", kJSONHeaders, body)) {
        rapidjson::Document resp;
        resp.Parse(body.c_str());
        if (resp.IsArray()) {
            rapidjson::Document out;
            out.SetArray();
            auto &alloc = out.GetAllocator();
            for (auto &g : resp.GetArray()) {
                if (!g.IsObject() || !g.HasMember("name") || !g["name"].IsString()) continue;
                std::string name = trim(g["name"].GetString());
                if (name.empty()) continue;
                std::string slug = g.HasMember("slug") && g["slug"].IsString() ? trim(g["slug"].GetString()) : "";
                rapidjson::Value v(rapidjson::kObjectType);
                add_string(v, "name", name, alloc);
                add_string(v, "slug", slug.empty() ? genre_slug(name) : slug, alloc);
                out.PushBack(v, alloc);
            }
            if (out.Size() > 0) return to_json(out);
        }
    }

    // Fallback when /api/genres is unreachable, so the filter row is never empty.
    std::vector<std::string> names = {"Fantasy", "Action",     "Adventure", "Comedy",  "Drama",
                                      "Romance", "Mystery",    "Thriller",  "Horror",  "Isekai",
                                      "Sci-Fi",  "Slice of Life", "Supernatural"};
    rapidjson::Document out;
    out.SetArray();
    auto &alloc = out.GetAllocator();
    for (const auto &n : names) {
        rapidjson::Value g(rapidjson::kObjectType);
        add_string(g, "name", n, alloc);
        add_string(g, "slug", n, alloc);
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
