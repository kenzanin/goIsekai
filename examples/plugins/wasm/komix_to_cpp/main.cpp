// Komix.to (komix.to) source plugin for goIsekai — C++17 WASM reactor.
//
// Komix.to is Next.js + JSON API:
//   search: GET /api/search?q=<query>
//           → {"results":[{"id,title,slug,cover_image,status,type,...}]}
//           cover_image like "/api/images?path=<slug>%2Fcover.jpg"
//   detail: scrape /title/<slug> (og:image, og:description, /genres/* links)
//   pages:  scrape /title/<slug>/chapter/<n> — embedded "pages":[{image_url,enc}]
//           image_url empty → real URL /api/images?t=<enc>
//   genres: scrape /genres index for /genres/* links
//
// ID scheme:
//   Manga.ID   = slug, e.g. "isekai-mesu-yuusha"
//   Chapter.ID = "<mangaSlug>:<chapterNum>"
//
// Reuses the shared helpers: wasm_abi.h (host ABI), util.h (text/JSON),
// html_util.h (scraping). JSON via RapidJSON.

#include <string>
#include <vector>
#include <regex>

#include "html_util.h"

static const char *kSite = "https://komix.to";
static const char *kJSONHeaders = "{\"Accept\":\"application/json\",\"Referer\":\"https://komix.to/\"}";

// ─── search ─────────────────────────────────────────────────────────────────

static std::string genre_listing(const std::string &genre);

static std::string search_impl(const std::string &arg) {
    rapidjson::Document f;
    f.Parse(arg.c_str());
    std::string query = f.HasMember("query") && f["query"].IsString() ? f["query"].GetString() : "";
    std::vector<std::string> genres;
    if (f.HasMember("genres") && f["genres"].IsArray()) {
        for (auto &g : f["genres"].GetArray())
            if (g.IsString()) genres.push_back(g.GetString());
    }

    if (trim(query).empty() && genres.empty()) return "[]";
    if (trim(query).empty()) return genre_listing(genres[0]);

    std::string body;
    if (!http_body(std::string(kSite) + "/api/search?q=" + url_encode(query), kJSONHeaders, body)) return "[]";

    rapidjson::Document resp;
    resp.Parse(body.c_str());
    if (!resp.IsObject() || !resp.HasMember("results") || !resp["results"].IsArray()) return "[]";

    rapidjson::Document out;
    out.SetArray();
    auto &alloc = out.GetAllocator();
    for (auto &it : resp["results"].GetArray()) {
        if (!it.IsObject()) continue;
        std::string cover = it.HasMember("cover_image") && it["cover_image"].IsString() ? trim(it["cover_image"].GetString()) : "";
        rapidjson::Value m(rapidjson::kObjectType);
        add_string(m, "id", it.HasMember("slug") && it["slug"].IsString() ? trim(it["slug"].GetString()) : "", alloc);
        add_string(m, "title", it.HasMember("title") && it["title"].IsString() ? trim(it["title"].GetString()) : "", alloc);
        add_string(m, "cover_url", make_abs(kSite, cover), alloc);
        add_string(m, "status", it.HasMember("status") && it["status"].IsString() ? normalize_status(it["status"].GetString()) : "", alloc);
        out.PushBack(m, alloc);
    }
    return to_json(out);
}

static std::string genre_listing(const std::string &genre) {
    std::string slug = genre_slug(genre);
    std::string body;
    if (!get_body(std::string(kSite) + "/genres/" + url_encode(slug), body)) return "[]";

    std::vector<std::string> seen;
    rapidjson::Document out;
    out.SetArray();
    auto &alloc = out.GetAllocator();
    int count = 0;
    for (auto &s : all_subs(std::regex("/title/([a-z0-9-]+)"), body, 200)) {
        if (count >= 48) break;
        if (s.empty() || std::find(seen.begin(), seen.end(), s) != seen.end()) continue;
        seen.push_back(s);
        rapidjson::Value m(rapidjson::kObjectType);
        add_string(m, "id", s, alloc);
        add_string(m, "title", humanize(s), alloc);
        rapidjson::Value garr(rapidjson::kArrayType);
        garr.PushBack(rapidjson::Value(humanize(slug).c_str(), (rapidjson::SizeType)humanize(slug).size(), alloc), alloc);
        m.AddMember("genres", garr, alloc);
        out.PushBack(m, alloc);
        count++;
    }
    return to_json(out);
}

// ─── detail ─────────────────────────────────────────────────────────────────

static std::string detail_impl(const std::string &arg) {
    std::string slug = trim(decode_string(arg));
    if (slug.empty()) return "{}";
    std::string body;
    if (!get_body(std::string(kSite) + "/title/" + url_encode(slug), body)) return "{}";

    rapidjson::Document m;
    m.SetObject();
    auto &alloc = m.GetAllocator();
    add_string(m, "id", slug, alloc);

    std::string t = first_sub(std::regex("og:title\"\\s*content=\"([^\"]+)\""), body);
    const char *suffix = " - Read Free Online | Komix";
    size_t sl = strlen(suffix);
    if (t.size() >= sl && t.compare(t.size() - sl, sl, suffix) == 0) t = t.substr(0, t.size() - sl);
    add_string(m, "title", t.empty() ? humanize(slug) : t, alloc);

    std::string img = first_sub(std::regex("og:image\"\\s*content=\"([^\"]+)\""), body);
    if (!img.empty()) add_string(m, "cover_url", img, alloc);
    std::string desc = first_sub(std::regex("og:description\"\\s*content=\"([^\"]+)\""), body);
    if (!desc.empty()) add_string(m, "description", unescape_html(desc), alloc);

    rapidjson::Value garr(rapidjson::kArrayType);
    std::vector<std::string> seen;
    for (auto &g : all_subs(std::regex("/genres/([a-z0-9-]+)"), body, 40)) {
        std::string name = humanize(g);
        if (name.empty() || std::find(seen.begin(), seen.end(), name) != seen.end()) continue;
        seen.push_back(name);
        garr.PushBack(rapidjson::Value(name.c_str(), (rapidjson::SizeType)name.size(), alloc), alloc);
    }
    m.AddMember("genres", garr, alloc);
    return to_json(m);
}

// ─── chapters ───────────────────────────────────────────────────────────────

static std::string chapters_impl(const std::string &arg) {
    std::string slug = trim(decode_string(arg));
    if (slug.empty()) return "[]";
    std::string body;
    if (!get_body(std::string(kSite) + "/title/" + url_encode(slug), body)) return "[]";

    // Embedded JSON is escaped: \"chapters\":[{\"id\":...}]
    std::smatch m;
    if (!std::regex_search(body, m, std::regex("\\\\?\"chapters\\\\?\":\\[(.*?)\\]"))) return "[]";
    std::string raw = "[" + std::regex_replace(m[1].str(), std::regex("\\\\\""), "\"") + "]";

    rapidjson::Document chs;
    chs.Parse(raw.c_str());
    if (!chs.IsArray()) return "[]";

    rapidjson::Document out;
    out.SetArray();
    auto &alloc = out.GetAllocator();
    for (auto &c : chs.GetArray()) {
        if (!c.IsObject()) continue;
        double num = c.HasMember("chapter_number") && c["chapter_number"].IsNumber() ? c["chapter_number"].GetDouble() : 0;
        char numbuf[32];
        snprintf(numbuf, sizeof(numbuf), "%g", num);
        std::string numstr = numbuf;

        rapidjson::Value ch(rapidjson::kObjectType);
        add_string(ch, "id", slug + ":" + numstr, alloc);
        add_string(ch, "manga_id", slug, alloc);
        add_string(ch, "title", c.HasMember("title") && c["title"].IsString() ? trim(c["title"].GetString()) : "", alloc);
        ch.AddMember("chapter_num", num, alloc);
        add_string(ch, "released_at", c.HasMember("created_at") && c["created_at"].IsString() ? parse_time(c["created_at"].GetString()) : "", alloc);
        add_string(ch, "url", std::string(kSite) + "/title/" + slug + "/chapter/" + numstr, alloc);
        out.PushBack(ch, alloc);
    }
    return to_json(out);
}

// ─── pages ──────────────────────────────────────────────────────────────────

static std::string pages_impl(const std::string &arg) {
    std::string chID = trim(decode_string(arg));
    if (chID.empty()) return "[]";

    std::string page_url;
    size_t colon = chID.find(':');
    if (colon != std::string::npos) {
        page_url = std::string(kSite) + "/title/" + url_encode(chID.substr(0, colon)) + "/chapter/" + url_encode(chID.substr(colon + 1));
    } else {
        page_url = chID;
    }

    std::string body;
    if (!get_body(page_url, body)) return "[]";

    std::string raw = std::regex_replace(body, std::regex("\\\\\""), "\"");
    std::smatch m;
    if (!std::regex_search(raw, m, std::regex("\"pages\":\\[(.*?)\\]"))) return "[]";

    rapidjson::Document pgs;
    pgs.Parse(("[" + m[1].str() + "]").c_str());
    if (!pgs.IsArray()) return "[]";

    rapidjson::Document out;
    out.SetArray();
    auto &alloc = out.GetAllocator();
    int idx = 0;
    for (auto &p : pgs.GetArray()) {
        if (!p.IsObject()) continue;
        std::string u = p.HasMember("image_url") && p["image_url"].IsString() ? trim(p["image_url"].GetString()) : "";
        std::string enc = p.HasMember("enc") && p["enc"].IsString() ? trim(p["enc"].GetString()) : "";
        if (u.empty() && !enc.empty()) u = std::string(kSite) + "/api/images?t=" + enc;
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
    std::vector<std::string> names = {"Action", "Adventure", "Comedy", "Drama", "Fantasy", "Isekai", "Romance"};
    std::string body;
    if (get_body(std::string(kSite) + "/genres", body)) {
        std::vector<std::string> seen;
        names.clear();
        for (auto &g : all_subs(std::regex("/genres/([a-z0-9-]+)"), body, 80)) {
            std::string name = humanize(g);
            if (name.empty() || std::find(seen.begin(), seen.end(), name) != seen.end()) continue;
            seen.push_back(name);
            names.push_back(name);
        }
    }

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
