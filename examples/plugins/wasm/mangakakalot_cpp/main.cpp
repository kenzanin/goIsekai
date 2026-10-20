// MangaKakalot (www.mangakakalove.com) source plugin for goIsekai — C++17 WASM reactor.
//
// 1:1 port of ../mangakakalot/main.go (standard-Go wasip1 reactor), itself a
// port of the keiyoushi MangaBox multisrc (mangakakalot).
//
//   search:   scrape /search/story/<slug>?page=N  (or /manga-list/hot-manga)
//   chapters: GET /api/manga/<slug>/chapters?limit=-1
//   detail:   scrape /manga/<slug> (h1/h2, cover, description, Status)
//   pages:    scrape chapter page — cdns[] / backupImage[] / chapterImages[],
//             else every <img> on the page
//   genres:   MangaBox hard-codes filters client-side → empty array
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

static const char *kSite = "https://www.mangakakalove.com";
// mangakakalove 403s any request without a Referer.
static const char *kRefHeader = "{\"Referer\":\"https://www.mangakakalove.com/\"}";
static const char *kRefJSONHeader =
    "{\"Accept\":\"application/json, text/plain, */*\",\"Referer\":\"https://www.mangakakalove.com/\"}";

static bool get_html(const std::string &u, std::string &out) { return http_body(u, kRefHeader, out); }
static bool get_json(const std::string &u, std::string &out) { return http_body(u, kRefJSONHeader, out); }

// std::regex has no inline flags: Go's `(?i)` → std::regex::icase, `(?s)` → [\s\S].
static std::regex re_i(const char *p) { return std::regex(p, std::regex::icase); }

// ─── helpers ────────────────────────────────────────────────────────────────

// clean(): tags → " ", unescape entities, collapse whitespace.
static std::string clean(const std::string &s) {
    std::string t = std::regex_replace(s, std::regex(R"re(<[^>]*>)re"), " ");
    t = unescape_html(t);
    t = std::regex_replace(t, std::regex(R"re(\s+)re"), " ");
    return trim(t);
}

// Trimmed 1st capture group, or "".
static std::string first_sub_or_empty(const std::regex &re, const std::string &s) {
    std::smatch m;
    if (std::regex_search(s, m, re) && m.size() > 1) return m[1].str();
    return "";
}

static std::string first_non_empty(const std::vector<std::string> &vals) {
    for (const auto &v : vals)
        if (!v.empty()) return v;
    return "";
}

// Last path segment of a URL, trailing slash trimmed.
static std::string slug_of(const std::string &raw) {
    std::string p = raw;
    size_t q = p.find_first_of("?#");
    if (q != std::string::npos) p = p.substr(0, q);
    size_t scheme = p.find("://");
    if (scheme != std::string::npos) {
        size_t slash = p.find('/', scheme + 3);
        p = (slash == std::string::npos) ? "" : p.substr(slash);
    }
    while (!p.empty() && p.back() == '/') p.pop_back();
    size_t i = p.rfind('/');
    return i == std::string::npos ? p : p.substr(i + 1);
}

// Resolve a possibly-relative URL against the site origin.
static std::string abs_url(const std::string &raw) {
    if (raw.empty()) return "";
    if (raw.rfind("http://", 0) == 0 || raw.rfind("https://", 0) == 0) return raw;
    if (raw.compare(0, 2, "//") == 0) return "https:" + raw;
    if (raw[0] == '/') return std::string(kSite) + raw;
    return std::string(kSite) + "/" + raw;
}

static double chapter_num(const std::string &label) {
    std::smatch m;
    if (std::regex_search(label, m, std::regex(R"re(\d+(?:\.\d+)?)re"))) return std::atof(m[0].str().c_str());
    return 0;
}

// MangaBox's status vocabulary is narrower than the shared helper's.
static std::string normalize_status_site(const std::string &raw) {
    std::string l = lower(trim(raw));
    if (l.find("ongoing") != std::string::npos) return "Ongoing";
    if (l.find("complete") != std::string::npos) return "Completed";
    if (l.find("hiatus") != std::string::npos) return "Hiatus";
    if (l.find("cancel") != std::string::npos || l.find("dropped") != std::string::npos) return "Cancelled";
    return "";
}

// normalizeQuery(): mirror MangaBox's change_alias diceware transform.
static std::string normalize_query(const std::string &q) {
    std::string s = lower(q);
    // Vietnamese diacritics → base ASCII letter, applied in the same order as the Go map.
    struct Fold { const char *chars; char to; };
    static const Fold folds[] = {
        {"àáạảãâầấậẩẫăằắặẳẵ", 'a'}, {"èéẹẻẽêềếệểễ", 'e'}, {"ìíịỉĩ", 'i'},
        {"òóọỏõôồốộổỗơờớợởỡ", 'o'}, {"ùúụủũưừứựửữ", 'u'}, {"ỳýỵỷỹ", 'y'}, {"đ", 'd'},
    };
    for (const auto &f : folds) {
        std::string from = f.chars, to(1, f.to);
        for (size_t i = 0; i < from.size();) {
            unsigned char c = (unsigned char)from[i];
            size_t len = (c < 0x80) ? 1 : (c < 0xE0 ? 2 : 3);  // UTF-8 lead-byte length
            s = std::regex_replace(s, std::regex(from.substr(i, len)), to);
            i += len;
        }
    }
    s = std::regex_replace(s, std::regex(R"re([!@%^*()+ =<>\?/,\.:;'"&#\[\]~_$-])re"), "_");
    s = std::regex_replace(s, std::regex(R"re(_+)re"), "_");
    return std::regex_replace(s, std::regex(R"re(^_+|_+$)re"), "");
}

// src / data-src / data-lazy-src of an <img ...> fragment.
static std::string img_url_of(const std::string &s) {
    for (const char *name : {"data-src", "data-lazy-src", "src"}) {
        std::regex re(std::string(R"re((?:\s|^))re") + name + R"re(="\s*([^"]+)")re");
        std::smatch m;
        if (std::regex_search(s, m, re)) {
            std::string v = trim(m[1].str());
            if (!v.empty()) return v;
        }
    }
    return "";
}

// cardIndex(): locate the card link whose href ends in slug.
static long card_index(const std::string &body, const std::string &slug) {
    for (const std::string &needle : {"/manga/" + slug + "\"", "/manga/" + slug + "/"}) {
        size_t i = body.find(needle);
        if (i != std::string::npos) return (long)i;
    }
    size_t i = body.find(slug);
    return i == std::string::npos ? -1 : (long)i;
}

// coverNear(): cover image of the card whose link contains slug. Prefer a
// following image whose URL embeds the slug; else the nearest preceding image.
static std::string cover_near(const std::string &body, const std::string &slug) {
    long i = card_index(body, slug);
    if (i < 0) return "";
    size_t j = body.find("<img", (size_t)i);
    if (j != std::string::npos) {
        size_t k = body.find('>', j);
        if (k != std::string::npos) {
            std::string u = img_url_of(body.substr(j, k - j + 1));
            if (u.find(slug) != std::string::npos) return u;
        }
    }
    size_t p = body.rfind("<img", (size_t)i);
    if (p == std::string::npos) return "";
    size_t k = body.find('>', p);
    if (k == std::string::npos) return "";
    return img_url_of(body.substr(p, k - p + 1));
}

// All whole-match occurrences of `re` (no capture groups needed).
static std::vector<std::string> full_matches(const std::regex &re, const std::string &body, size_t max = 500) {
    std::vector<std::string> out;
    for (std::sregex_iterator it(body.begin(), body.end(), re), end; it != end && out.size() < max; ++it) {
        out.push_back((*it)[0].str());
    }
    return out;
}

// extractArray(): parse a JS `name = ["a","b"]` array literal's strings.
static std::vector<std::string> extract_array(const std::string &s, const std::regex &re) {    std::smatch m;
    if (!std::regex_search(s, m, re) || m.size() < 2) return {};
    std::vector<std::string> out;
    std::string inner = m[1].str(), cur;
    for (size_t i = 0; i <= inner.size(); i++) {
        if (i == inner.size() || inner[i] == ',') {
            std::string p = trim(cur);
            cur.clear();
            if (!p.empty() && p.front() == '"') p.erase(0, 1);
            if (!p.empty() && p.back() == '"') p.pop_back();
            p = std::regex_replace(p, std::regex(R"re(\\/)re"), "/");
            if (!p.empty() && p.back() == '/') p.pop_back();
            if (!p.empty()) out.push_back(p);
        } else {
            cur += inner[i];
        }
    }
    return out;
}

// ─── search ─────────────────────────────────────────────────────────────────

static std::string search_impl(const std::string &arg) {
    rapidjson::Document f;
    f.Parse(arg.c_str());
    std::string query = f.HasMember("query") && f["query"].IsString() ? trim(f["query"].GetString()) : "";
    int page = f.HasMember("page") && f["page"].IsInt() ? f["page"].GetInt() : 1;
    if (page < 1) page = 1;

    std::string target = query.empty()
                             ? std::string(kSite) + "/manga-list/hot-manga?page=" + std::to_string(page)
                             : std::string(kSite) + "/search/story/" + normalize_query(query) + "?page=" + std::to_string(page);

    std::string body;
    if (!get_html(target, body)) return "[]";

    rapidjson::Document out;
    out.SetArray();
    auto &alloc = out.GetAllocator();
    std::vector<std::string> seen;
    std::regex cardRe(R"re([\s\S]<h3[^>]*>\s*<a[^>]*href="([^"]+)"[^>]*>([\s\S]*?)</a>)re");
    // No result cap: the Go twin emits every card on the page, and the host
    // paginates. A `count < 48` here silently dropped the last two results.
    for (std::sregex_iterator it(body.begin(), body.end(), cardRe), end; it != end; ++it) {
        std::string slug = slug_of((*it)[1].str());
        if (slug.empty() || std::find(seen.begin(), seen.end(), slug) != seen.end()) continue;
        seen.push_back(slug);
        rapidjson::Value m(rapidjson::kObjectType);
        add_string(m, "id", slug, alloc);
        add_string(m, "title", clean((*it)[2].str()), alloc);
        add_string(m, "cover_url", abs_url(cover_near(body, slug)), alloc);
        out.PushBack(m, alloc);
    }
    return to_json(out);
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
    if (!get_html(std::string(kSite) + "/manga/" + url_encode(slug), body)) return to_json(m);

    std::string title = first_non_empty({
        clean(first_sub_or_empty(re_i(R"re(<h1[^>]*>([\s\S]*?)</h1>)re"), body)),
        clean(first_sub_or_empty(re_i(R"re(<h2[^>]*>([\s\S]*?)</h2>)re"), body)),
    });
    if (!title.empty()) add_string(m, "title", title, alloc);

    std::smatch ct, cp;
    std::string coverTop = std::regex_search(body, ct, re_i(R"re(class="manga-info-pic"[\s\S]*?<img[^>]*>)re"))
                               ? ct[0].str() : "";
    std::string coverPanel = std::regex_search(body, cp, re_i(R"re(class="info-image"[\s\S]*?<img[^>]*>)re"))
                                 ? cp[0].str() : "";
    std::string cover = abs_url(img_url_of(first_non_empty({coverTop, coverPanel})));
    if (!cover.empty()) add_string(m, "cover_url", cover, alloc);

    std::string desc = clean(first_non_empty({
        first_sub_or_empty(re_i(R"re(id="noidungm"[^>]*>([\s\S]*?)</div>)re"), body),
        first_sub_or_empty(re_i(R"re(id="panel-story-info-description"[^>]*>([\s\S]*?)</div>)re"), body),
        first_sub_or_empty(re_i(R"re(id="contentBox"[^>]*>([\s\S]*?)</div>)re"), body),
    }));
    if (!desc.empty()) add_string(m, "description", desc, alloc);

    std::string st = first_sub_or_empty(
        re_i(R"re(\bStatus\b\s*:?\s*(?:<[^>]*>\s*)*([A-Za-z][A-Za-z ]{2,24}))re"), body);
    if (!st.empty()) add_string(m, "status", normalize_status_site(st), alloc);
    return to_json(m);
}

// ─── chapters ───────────────────────────────────────────────────────────────

static std::string chapters_impl(const std::string &arg) {
    std::string slug = trim(decode_string(arg));
    if (slug.empty()) return "[]";

    std::string body;
    if (!get_json(std::string(kSite) + "/api/manga/" + url_encode(slug) + "/chapters?limit=-1", body)) return "[]";

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
        std::string cslug = c.HasMember("chapter_slug") && c["chapter_slug"].IsString() ? c["chapter_slug"].GetString() : "";
        if (cslug.empty()) continue;
        std::string title = "Chapter";
        if (c.HasMember("chapter_name") && c["chapter_name"].IsString() && c["chapter_name"].GetStringLength() > 0)
            title = c["chapter_name"].GetString();
        double num = c.HasMember("chapter_num") && c["chapter_num"].IsNumber() ? c["chapter_num"].GetDouble() : 0;

        rapidjson::Value ch(rapidjson::kObjectType);
        add_string(ch, "id", slug + ":" + cslug, alloc);
        add_string(ch, "manga_id", slug, alloc);
        add_string(ch, "title", title, alloc);
        ch.AddMember("chapter_num", num, alloc);
        add_string(ch, "released_at",
                   c.HasMember("updated_at") && c["updated_at"].IsString() ? parse_time(c["updated_at"].GetString()) : "",
                   alloc);
        add_string(ch, "url", std::string(kSite) + "/manga/" + url_encode(slug) + "/" + cslug, alloc);
        out.PushBack(ch, alloc);
    }
    return to_json(out);
}

// ─── pages ──────────────────────────────────────────────────────────────────

static std::string pages_impl(const std::string &arg) {
    std::string v = trim(decode_string(arg));
    size_t colon = v.find(':');
    if (colon == std::string::npos || colon == 0 || colon + 1 >= v.size()) return "[]";
    std::string slug = v.substr(0, colon), chapterSlug = v.substr(colon + 1);

    std::string body;
    if (!get_html(std::string(kSite) + "/manga/" + url_encode(slug) + "/" + url_encode(chapterSlug), body)) return "[]";

    std::vector<std::string> cdns = extract_array(body, std::regex(R"re(cdns\s*=\s*\[([^]]+)])re"));
    for (auto &b : extract_array(body, std::regex(R"re(backupImage\s*=\s*\[([^]]+)])re"))) cdns.push_back(b);
    std::vector<std::string> images = extract_array(body, std::regex(R"re(chapterImages\s*=\s*\[([^]]+)])re"));

    rapidjson::Document out;
    out.SetArray();
    auto &alloc = out.GetAllocator();

    if (!images.empty() && !cdns.empty()) {
        // Base = origin of cdns[0]; relative image paths hang off it.
        std::string base = cdns[0];
        size_t scheme = base.find("://");
        std::string origin;
        if (scheme != std::string::npos) {
            size_t slash = base.find('/', scheme + 3);
            origin = (slash == std::string::npos) ? base : base.substr(0, slash);
        } else {
            origin = std::string(kSite);
        }
        for (const auto &p0 : images) {
            std::string p = p0;
            std::string full;
            if (p.rfind("http", 0) == 0) {
                full = p;
            } else {
                if (p.empty() || p[0] != '/') p = "/" + p;
                full = origin + p;
            }
            rapidjson::Value pg(rapidjson::kObjectType);
            pg.AddMember("index", (int)out.Size(), alloc);
            add_string(pg, "url", full, alloc);
            rapidjson::Value hdr(rapidjson::kObjectType);
            add_string(hdr, "Referer", kSite, alloc);
            pg.AddMember("headers", hdr, alloc);
            out.PushBack(pg, alloc);
        }
        return to_json(out);
    }

    for (auto &tag : full_matches(re_i(R"re(<img\b[^>]*>)re"), body)) {
        std::string src = img_url_of(tag);
        if (src.empty()) continue;
        rapidjson::Value pg(rapidjson::kObjectType);
        pg.AddMember("index", (int)out.Size(), alloc);
        add_string(pg, "url", abs_url(src), alloc);
        rapidjson::Value hdr(rapidjson::kObjectType);
        add_string(hdr, "Referer", kSite, alloc);
        pg.AddMember("headers", hdr, alloc);
        out.PushBack(pg, alloc);
    }
    return to_json(out);
}

// ─── genres ─────────────────────────────────────────────────────────────────

// MangaBox hard-codes its genre filters client-side → empty array (host contract happy).
static std::string genres_impl() { return "[]"; }

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
