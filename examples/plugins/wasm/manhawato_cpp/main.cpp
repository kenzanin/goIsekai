// Manhwato (manhwato.com) source plugin for goIsekai — C++17 WASM reactor.
//
// Manhwato is plain server-rendered HTML (Madara-like theme):
//   genre listing: /manhwa-genre/<slug>
//   search:        /search?q=<query>
//   detail:        /manhwa/<slug>
//   chapter:       /manhwa/<slug>/<chapter-slug>  (e.g. chapter-0-ch264518)
//   pages:         <img> tags with /images/manga/<slug>/<chapter>/<n>.jpg
//
// ID scheme:
//   Manga.ID   = manhwa slug, e.g. "isekai-meikyuu-de-harem-wo"
//   Chapter.ID = "<mangaSlug>:<chapterSlug>"
//
// Reuses the shared helpers: wasm_abi.h (host ABI), util.h (text/JSON),
// html_util.h (scraping). Ported 1:1 from manhawato/main.go; Go's (?s) dotall
// is expressed here as [\s\S] since C++ std::regex has no inline flags.

#include <string>
#include <vector>
#include <regex>
#include <algorithm>

#include "html_util.h"

static const char *kSite = "https://manhwato.com";

// ─── patterns ───────────────────────────────────────────────────────────────

// NB: custom raw-string delimiter R"re(...)re" — these patterns contain `)"`
// (e.g. href="([^"]+)"), which would terminate the default R"( ... )" early.
static const std::regex reCard(R"re(<div class="visual">([\s\S]*?)</div>\s*</div>)re");
static const std::regex reCover(R"re(<div class="novel-cover">([\s\S]*?)</div>)re");
static const std::regex reTitleH3(R"re(<h3 class="title">([\s\S]*?)</h3>)re");
static const std::regex reAnchor(R"re(<a[^>]*>([\s\S]*?)</a>)re");
static const std::regex reHref(R"re(href="([^"]+)")re");
static const std::regex reTitleAt(R"re(title="([^"]+)")re");
static const std::regex reNum(R"re((\d+(\.\d+)?))re");
static const std::regex reH1(R"re(<h1[^>]*>([\s\S]*?)</h1>)re");
static const std::regex reChapUl(R"re(<ul class="chapter-list">([\s\S]*?)</ul>)re");
static const std::regex reChapLi(R"re(<li>([\s\S]*?)</li>)re");
static const std::regex reTime(R"re(datetime="([^"]+)")re");
static const std::regex reGenre(R"re(<a[^>]*href="/manhwa-genre/([^"/]+)/?"[^>]*>([^<]+)</a>)re");
static const std::regex reOGImage(R"re(<meta[^>]*property="og:image"[^>]*content="([^"]+)")re");
static const std::regex reOGDesc(R"re(<meta[^>]*property="og:description"[^>]*content="([^"]+)")re");
static const std::regex reStatus(R"re(status[^<]{0,80}(ongoing|completed|hiatus|cancelled))re", std::regex::icase);
static const std::regex reImg(R"re(<img\b[^>]*>)re", std::regex::icase);

// ─── helpers ────────────────────────────────────────────────────────────────

static std::string clean(const std::string &s) { return unescape_html(strip_html(s)); }

static std::string manga_url(const std::string &slug) {
    return std::string(kSite) + "/manhwa/" + url_encode(slug);
}

// Last path segment of a URL (query/fragment and trailing slash removed).
static std::string slug_of(const std::string &raw) {
    std::string p = raw;
    size_t q = p.find_first_of("?#");
    if (q != std::string::npos) p = p.substr(0, q);
    while (!p.empty() && p.back() == '/') p.pop_back();
    size_t i = p.find_last_of('/');
    return i == std::string::npos ? p : p.substr(i + 1);
}

// ─── search ─────────────────────────────────────────────────────────────────

static std::string parse_cards(const std::string &body) {
    rapidjson::Document out;
    out.SetArray();
    auto &alloc = out.GetAllocator();
    std::vector<std::string> seen;
    for (std::sregex_iterator it(body.begin(), body.end(), reCard), end; it != end; ++it) {
        std::string seg = (*it)[1].str();
        std::string h3 = first_sub(reTitleH3, seg);
        std::string href = first_sub(reHref, h3);
        if (href.empty()) href = first_sub(reHref, seg);
        std::string slug = slug_of(href);
        if (slug.empty() || href.find("/chapter-") != std::string::npos) continue;
        if (std::find(seen.begin(), seen.end(), slug) != seen.end()) continue;
        seen.push_back(slug);

        std::string title = clean(first_sub(reAnchor, h3));
        if (title.empty()) title = clean(first_sub(reTitleAt, h3));

        rapidjson::Value m(rapidjson::kObjectType);
        add_string(m, "id", slug, alloc);
        add_string(m, "title", title, alloc);
        add_string(m, "cover_url", make_abs(kSite, img_url(first_sub(reCover, seg))), alloc);
        out.PushBack(m, alloc);
    }
    return to_json(out);
}

static std::string search_impl(const std::string &arg) {
    rapidjson::Document f;
    f.Parse(arg.c_str());
    std::string q = f.HasMember("query") && f["query"].IsString() ? trim(f["query"].GetString()) : "";
    std::vector<std::string> genres;
    if (f.HasMember("genres") && f["genres"].IsArray())
        for (auto &g : f["genres"].GetArray())
            if (g.IsString()) genres.push_back(g.GetString());

    std::string url;
    if (q.empty() && !genres.empty()) url = std::string(kSite) + "/manhwa-genre/" + url_encode(lower(trim(genres[0])));
    else if (q.empty()) url = std::string(kSite) + "/manhwa-genre/isekai";
    else url = std::string(kSite) + "/search?q=" + url_encode(q);

    std::string body;
    if (!get_body(url, body)) return "[]";
    return parse_cards(body);
}

// ─── detail ─────────────────────────────────────────────────────────────────

static std::string detail_impl(const std::string &arg) {
    std::string slug = trim(decode_string(arg));
    rapidjson::Document m;
    m.SetObject();
    auto &alloc = m.GetAllocator();
    add_string(m, "id", slug, alloc);
    if (slug.empty()) return to_json(m);

    std::string body;
    if (!get_body(manga_url(slug), body)) return to_json(m);

    std::string cover = first_sub(reOGImage, body);
    if (!cover.empty()) add_string(m, "cover_url", cover, alloc);
    std::string desc = first_sub(reOGDesc, body);
    if (!desc.empty()) add_string(m, "description", unescape_html(desc), alloc);
    std::string h1 = clean(first_sub(reH1, body));
    if (!h1.empty()) add_string(m, "title", h1, alloc);
    std::string st = first_sub(reStatus, body);
    if (!st.empty()) add_string(m, "status", normalize_status(st), alloc);

    rapidjson::Value garr(rapidjson::kArrayType);
    std::vector<std::string> gseen;
    for (std::sregex_iterator it(body.begin(), body.end(), reGenre), end; it != end; ++it) {
        std::string name = clean((*it)[2].str());
        std::string key = lower(name);
        if (name.empty() || std::find(gseen.begin(), gseen.end(), key) != gseen.end()) continue;
        gseen.push_back(key);
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
    if (!get_body(manga_url(slug), body)) return "[]";

    std::string ul = first_sub(reChapUl, body);
    if (ul.empty()) ul = body;

    rapidjson::Document out;
    out.SetArray();
    auto &alloc = out.GetAllocator();
    std::vector<std::string> seen;
    for (std::sregex_iterator it(ul.begin(), ul.end(), reChapLi), end; it != end; ++it) {
        std::string li = (*it)[1].str();
        std::string href = first_sub(reHref, li);
        std::string cslug = slug_of(href);
        if (cslug.empty() || std::find(seen.begin(), seen.end(), cslug) != seen.end()) continue;
        seen.push_back(cslug);

        std::string label = clean(first_sub(reAnchor, li));
        if (label.empty()) label = clean(first_sub(reTitleAt, li));

        double num = 0;
        std::smatch nm;
        if (std::regex_search(label, nm, reNum)) num = atof(nm[1].str().c_str());

        std::string rel;
        std::string ds = first_sub(reTime, li);
        if (!ds.empty()) rel = parse_time(ds);

        rapidjson::Value ch(rapidjson::kObjectType);
        add_string(ch, "id", slug + ":" + cslug, alloc);
        add_string(ch, "manga_id", slug, alloc);
        add_string(ch, "title", label, alloc);
        ch.AddMember("chapter_num", num, alloc);
        add_string(ch, "released_at", rel, alloc);
        add_string(ch, "url", make_abs(kSite, href), alloc);
        out.PushBack(ch, alloc);
    }
    return to_json(out);
}

// ─── pages ──────────────────────────────────────────────────────────────────

static std::string pages_impl(const std::string &arg) {
    std::string id = trim(decode_string(arg));
    size_t colon = id.find(':');
    if (colon == std::string::npos) return "[]";
    std::string slug = id.substr(0, colon), cslug = id.substr(colon + 1);
    if (slug.empty() || cslug.empty()) return "[]";

    std::string body;
    if (!get_body(manga_url(slug) + "/" + url_encode(cslug), body)) return "[]";

    rapidjson::Document out;
    out.SetArray();
    auto &alloc = out.GetAllocator();
    int idx = 0;
    for (std::sregex_iterator it(body.begin(), body.end(), reImg), end; it != end; ++it) {
        std::string src = img_url(it->str());
        if (src.empty() || src.find("/images/manga/") == std::string::npos) continue;
        rapidjson::Value pg(rapidjson::kObjectType);
        pg.AddMember("index", idx++, alloc);
        add_string(pg, "url", make_abs(kSite, src), alloc);
        out.PushBack(pg, alloc);
    }
    return to_json(out);
}

// ─── genres ─────────────────────────────────────────────────────────────────

static std::string genres_impl() {
    rapidjson::Document out;
    out.SetArray();
    auto &alloc = out.GetAllocator();
    std::string body;
    if (!get_body(std::string(kSite) + "/", body)) return to_json(out);

    std::vector<std::string> seen;
    for (std::sregex_iterator it(body.begin(), body.end(), reGenre), end; it != end; ++it) {
        std::string slug = (*it)[1].str();
        std::string name = clean((*it)[2].str());
        if (slug.empty() || name.empty() || std::find(seen.begin(), seen.end(), slug) != seen.end()) continue;
        seen.push_back(slug);
        rapidjson::Value g(rapidjson::kObjectType);
        add_string(g, "name", name, alloc);
        add_string(g, "slug", slug, alloc);
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
