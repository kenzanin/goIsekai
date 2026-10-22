// KunManga (kunmanga.online) source plugin for goIsekai — C++17 WASM reactor.
//
// 1:1 port of ../kunmanga_online/main.go (standard-Go wasip1 reactor).
//
// KunManga is WordPress Madara-like with a JSON API for search/chapters and
// HTML scraping for detail/pages/genres:
//   search:   GET /manga/searching?title=<query>
//   chapters: GET /api/comics/<slug>/chapters?page=&per_page=50&order=desc
//   genres:   scrape /manga-genre/<slug> listing for /manga-genre/* links
//   detail:   scrape /manga/<slug> (og:image, summary__content, genres-content)
//   reader:   /manga/<slug>/<chapter-slug>, images in data-src
//
// ID scheme: Manga.ID = slug; Chapter.ID = "<mangaSlug>:<chapterSlug>"
//
// Reuses the shared helpers: wasm_abi.h (host ABI), util.h (text/JSON),
// html_util.h (scraping). JSON via RapidJSON.

#include <string>
#include <vector>
#include <regex>
#include <algorithm>

#include "html_util.h"

// The site origin comes from plugin.json so a site move is one edit there.
// Lazy, not a global initializer: host_call only answers after _initialize.
static const std::string &site() {
    static const std::string v = [] {
        std::string s = host_str("plugin.param", "site_url");
        return s.empty() ? std::string("https://www.kunmanga.online") : s;
    }();
    return v;
}

// jsonHeaders() derives the Referer from the same origin.
static std::string jsonHeaders() {
    return "{\"Accept\":\"application/json\",\"Referer\":\"" + site() + "/\"}";
}

// Covers live on a CDN, not on the site. The API's image_original comes in
// three shapes — "//thumb/x.webp", "/thumb/x.webp", "thumb/x.webp" — and the
// site origin serves none of them (404); cdn.zinmanga1.com serves all three.
// "//thumb/x.webp" is the nasty one: it looks protocol-relative, so a plain
// make_abs() yields "https://thumb/x.webp" — a URL whose host is the literal
// directory name. Retarget every /thumb/ path at the CDN.
static std::string cover_url(const std::string &raw) {
    std::string u = trim(raw);
    if (u.empty()) return "";
    std::string cdn = host_str("plugin.param", "cdn");
    if (cdn.empty()) cdn = "https://cdn.zinmanga1.com";
    // Match "thumb/" without the leading slash: the API emits all three of
    // "//thumb/x", "/thumb/x" and bare "thumb/x", and only the first two have a
    // slash to anchor on. Everything from "thumb/" on is the CDN path.
    size_t at = u.find("thumb/");
    if (at != std::string::npos) return cdn + "/" + u.substr(at);
    return make_abs(site(), u);
}

// "a-b-c" → "a b c" (the Go plugin's ReplaceAll(slug, "-", " ") fallback).
static std::string dashes_to_spaces(const std::string &s) {
    std::string out = s;
    std::replace(out.begin(), out.end(), '-', ' ');
    return out;
}

// Comma-split, trimmed, non-empty (Go splitKinds).
static std::vector<std::string> split_kinds(const std::string &kind) {
    std::vector<std::string> out;
    size_t start = 0;
    while (start <= kind.size()) {
        size_t comma = kind.find(',', start);
        std::string k = trim(kind.substr(start, comma == std::string::npos ? std::string::npos : comma - start));
        if (!k.empty()) out.push_back(k);
        if (comma == std::string::npos) break;
        start = comma + 1;
    }
    return out;
}

// All 2-group matches of `re` in `body` (group1, group2), capped.
static std::vector<std::pair<std::string, std::string>> all_subs2(const std::regex &re, const std::string &body,
                                                                 size_t max = 100) {
    std::vector<std::pair<std::string, std::string>> out;
    for (std::sregex_iterator it(body.begin(), body.end(), re), end; it != end && out.size() < max; ++it) {
        out.push_back({(*it)[1].str(), (*it)[2].str()});
    }
    return out;
}

// ─── search ─────────────────────────────────────────────────────────────────

static std::string genre_listing(const std::string &genre, int page);

static std::string search_impl(const std::string &arg) {
    rapidjson::Document f;
    f.Parse(arg.c_str());
    std::string query = f.HasMember("query") && f["query"].IsString() ? trim(f["query"].GetString()) : "";
    int page = f.HasMember("page") && f["page"].IsInt() ? f["page"].GetInt() : 1;
    if (page < 1) page = 1;
    std::vector<std::string> genres;
    if (f.HasMember("genres") && f["genres"].IsArray()) {
        for (auto &g : f["genres"].GetArray())
            if (g.IsString()) genres.push_back(g.GetString());
    }

    if (query.empty() && genres.empty()) return "[]";
    if (query.empty()) return genre_listing(genres[0], page);

    std::string body;
    if (!http_body(site() + "/manga/searching?title=" + url_encode(query), jsonHeaders(), body)) return "[]";

    rapidjson::Document resp;
    resp.Parse(body.c_str());
    if (!resp.IsObject() || !resp.HasMember("success") || !resp["success"].IsBool() || !resp["success"].GetBool())
        return "[]";
    if (!resp.HasMember("data") || !resp["data"].IsArray()) return "[]";

    rapidjson::Document out;
    out.SetArray();
    auto &alloc = out.GetAllocator();
    for (auto &it : resp["data"].GetArray()) {
        if (!it.IsObject()) continue;
        std::string cover = it.HasMember("image_original") && it["image_original"].IsString()
                                ? trim(it["image_original"].GetString())
                                : "";
        // Covers resolve on the CDN, not the site origin — see cover_url().
        cover = cover_url(cover);

        rapidjson::Value m(rapidjson::kObjectType);
        add_string(m, "id", it.HasMember("slug") && it["slug"].IsString() ? trim(it["slug"].GetString()) : "", alloc);
        add_string(m, "title", it.HasMember("name") && it["name"].IsString() ? trim(it["name"].GetString()) : "", alloc);
        add_string(m, "cover_url", cover, alloc);
        add_string(m, "author", it.HasMember("author") && it["author"].IsString() ? trim(it["author"].GetString()) : "", alloc);
        add_string(m, "status",
                   it.HasMember("status") && it["status"].IsString() ? normalize_status(it["status"].GetString()) : "", alloc);
        if (it.HasMember("kind") && it["kind"].IsString()) {
            auto kinds = split_kinds(it["kind"].GetString());
            if (!kinds.empty()) {
                rapidjson::Value garr(rapidjson::kArrayType);
                for (auto &k : kinds) garr.PushBack(rapidjson::Value(k.c_str(), (rapidjson::SizeType)k.size(), alloc), alloc);
                m.AddMember("genres", garr, alloc);
            }
        }
        out.PushBack(m, alloc);
    }
    return to_json(out);
}

// Scrapes /manga-genre/<slug> cards (.page-item-detail).
static std::string genre_listing(const std::string &genre, int page) {
    std::string slug = genre_slug(genre);
    std::string u = site() + "/manga-genre/" + url_encode(slug);
    if (page > 1) u += "/page/" + std::to_string(page);
    std::string body;
    if (!get_body(u, body)) return "[]";

    std::regex cardRe(R"re([\s\S]<div class="page-item-detail[^>]*>([\s\S]*?)</div>\s*</div>\s*</div>)re");
    std::regex linkRe(R"re(<a href=["']?https?://www\.kunmanga\.online/manga/([^/"'>\s]+)["'\s>])re");
    std::regex titleRe(R"re(title="Read ([^"]+?) Manga Online)re");
    std::regex imgRe(R"re((?:data-src|src)="(https?://[^"]+\.(?:webp|jpg|jpeg|png))")re");

    rapidjson::Document out;
    out.SetArray();
    auto &alloc = out.GetAllocator();
    int count = 0;
    for (std::sregex_iterator it(body.begin(), body.end(), cardRe), end; it != end && count < 60; ++it) {
        std::string card = (*it)[1].str();
        std::smatch lm;
        if (!std::regex_search(card, lm, linkRe)) continue;
        std::string id = lm[1].str();

        std::string title;
        std::smatch tm;
        if (std::regex_search(card, tm, titleRe)) title = unescape_html(trim(tm[1].str()));
        else title = dashes_to_spaces(id);

        std::string cover;
        std::smatch im;
        if (std::regex_search(card, im, imgRe)) cover = im[1].str();

        std::string gname = humanize(slug);
        rapidjson::Value m(rapidjson::kObjectType);
        add_string(m, "id", id, alloc);
        add_string(m, "title", title, alloc);
        add_string(m, "cover_url", cover, alloc);
        rapidjson::Value garr(rapidjson::kArrayType);
        garr.PushBack(rapidjson::Value(gname.c_str(), (rapidjson::SizeType)gname.size(), alloc), alloc);
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
    if (!get_body(site() + "/manga/" + url_encode(slug), body)) return "{}";

    rapidjson::Document m;
    m.SetObject();
    auto &alloc = m.GetAllocator();
    add_string(m, "id", slug, alloc);

    std::string title = first_sub(std::regex(R"re(<h1[^>]*>([\s\S]*?)</h1>)re"), body);
    if (!title.empty()) title = unescape_html(strip_html(title));
    if (title.empty()) title = dashes_to_spaces(slug);
    add_string(m, "title", title, alloc);

    std::string cover = first_sub(std::regex(R"re(og:image"\s*content="([^"]+)")re"), body);
    if (!cover.empty()) add_string(m, "cover_url", cover, alloc);

    std::string desc = first_sub(std::regex(R"re([\s\S]summary__content[^>]*>([\s\S]*?)</div>)re"), body);
    if (!desc.empty()) add_string(m, "description", unescape_html(trim(strip_html(desc))), alloc);

    std::string status = first_sub(std::regex(R"re([\s\S]post-status[^>]*>[\s\S]*?<span[^>]*>\s*Status\s*</span>\s*([^<]+))re"), body);
    if (!status.empty()) add_string(m, "status", normalize_status(status), alloc);

    rapidjson::Value garr(rapidjson::kArrayType);
    std::regex genRe(R"re(manga-genre/([a-z0-9-]+)">([^<]+))re");
    std::string genres_block = first_sub(std::regex(R"re([\s\S]genres-content[^>]*>([\s\S]*?)</div>)re"), body);
    if (!genres_block.empty()) {
        for (auto &g : all_subs2(genRe, genres_block, 40)) {
            std::string name = unescape_html(trim(g.second));
            if (name.empty()) continue;
            garr.PushBack(rapidjson::Value(name.c_str(), (rapidjson::SizeType)name.size(), alloc), alloc);
        }
    }
    m.AddMember("genres", garr, alloc);
    return to_json(m);
}

// ─── chapters ───────────────────────────────────────────────────────────────

static std::string chapters_impl(const std::string &arg) {
    std::string slug = trim(decode_string(arg));
    if (slug.empty()) return "[]";

    rapidjson::Document out;
    out.SetArray();
    auto &alloc = out.GetAllocator();

    for (int page = 1;; page++) {
        std::string u = site() + "/api/comics/" + url_encode(slug) + "/chapters?page=" +
                        std::to_string(page) + "&per_page=50&order=desc";
        std::string body;
        if (!http_body(u, jsonHeaders(), body)) break;

        rapidjson::Document resp;
        resp.Parse(body.c_str());
        if (!resp.IsObject() || !resp.HasMember("success") || !resp["success"].IsBool() || !resp["success"].GetBool())
            break;
        if (!resp.HasMember("data") || !resp["data"].IsObject()) break;
        auto &d = resp["data"];
        if (d.HasMember("chapters") && d["chapters"].IsArray()) {
            for (auto &c : d["chapters"].GetArray()) {
                if (!c.IsObject()) continue;
                std::string cslug = c.HasMember("chapter_slug") && c["chapter_slug"].IsString()
                                        ? trim(c["chapter_slug"].GetString())
                                        : "";
                double num = c.HasMember("chapter_num") && c["chapter_num"].IsNumber() ? c["chapter_num"].GetDouble() : 0;
                rapidjson::Value ch(rapidjson::kObjectType);
                add_string(ch, "id", slug + ":" + cslug, alloc);
                add_string(ch, "manga_id", slug, alloc);
                add_string(ch, "title",
                           c.HasMember("chapter_name") && c["chapter_name"].IsString()
                               ? trim(c["chapter_name"].GetString())
                               : "", alloc);
                ch.AddMember("chapter_num", num, alloc);
                add_string(ch, "released_at",
                           c.HasMember("updated_at") && c["updated_at"].IsString()
                               ? parse_time(c["updated_at"].GetString())
                               : "", alloc);
                add_string(ch, "url", site() + "/manga/" + slug + "/" + cslug, alloc);
                out.PushBack(ch, alloc);
            }
        }
        int current = d.HasMember("current_page") && d["current_page"].IsInt() ? d["current_page"].GetInt() : 0;
        int last = d.HasMember("last_page") && d["last_page"].IsInt() ? d["last_page"].GetInt() : 0;
        if (current >= last || last == 0) break;
        if (page >= 40) break;  // ponytail: 2000-chapter ceiling; paginate further if needed
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
        page_url = site() + "/manga/" + url_encode(chID.substr(0, colon)) + "/" +
                   url_encode(chID.substr(colon + 1));
    } else {
        page_url = chID;
    }

    std::string body;
    if (!get_body(page_url, body)) return "[]";

    rapidjson::Document out;
    out.SetArray();
    auto &alloc = out.GetAllocator();
    std::vector<std::string> seen;
    for (auto &u : all_subs(std::regex(R"re(data-src=['"](https?://[^'"]+\.(?:webp|jpg|jpeg|png))['"])re"), body, 500)) {
        if (u.empty() || u.find("/thumb/") != std::string::npos) continue;
        if (std::find(seen.begin(), seen.end(), u) != seen.end()) continue;
        seen.push_back(u);
        rapidjson::Value pg(rapidjson::kObjectType);
        pg.AddMember("index", (int)out.Size(), alloc);
        add_string(pg, "url", u, alloc);
        out.PushBack(pg, alloc);
    }
    return to_json(out);
}

// ─── genres ─────────────────────────────────────────────────────────────────

static std::string genres_impl() {
    std::vector<std::string> names = {"Action", "Adventure", "Comedy", "Drama", "Fantasy", "Isekai", "Romance"};
    std::string body;
    if (get_body(site() + "/manga-genre/action", body)) {
        std::vector<std::string> seen;
        names.clear();
        for (auto &g : all_subs2(std::regex(R"re(manga-genre/([a-z0-9-]+)">([^<]+))re"), body, 100)) {
            std::string name = unescape_html(trim(g.second));
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
