// KSGroupScans (ksgroupscans.com) source plugin for goIsekai — C++17 WASM reactor.
//
// 1:1 port of ../ksgroupscans/main.go (standard-Go wasip1 reactor), itself a
// port of the keiyoushi KSGroupScans extension: a bare Madara subclass whose
// only override is chapterMode = MangaAjax.
//
//   search:   POST admin-ajax "madara_load_more" archive endpoint
//   chapters: POST <mangaURL>/ajax/chapters/
//   genres:   /manga/ listing page manga-genre links
//   detail:   scrape <mangaURL> (summary_image, author-content, genres-content)
//   pages:    chapter page wp-manga-chapter-img tags (`?style=list` if paged)
//
// ID scheme: Manga.ID = manga slug; Chapter.ID = "<mangaSlug>:<chapterSlug>"
//
// Reuses the shared helpers: wasm_abi.h (host ABI), util.h (text/JSON),
// html_util.h (scraping). JSON via RapidJSON.

#include <string>
#include <vector>
#include <regex>
#include <algorithm>

#include "html_util.h"

static const char *kSite = "https://ksgroupscans.com";
static const std::string kAjaxURL = std::string(kSite) + "/wp-admin/admin-ajax.php";
static const char *kMangaPath = "/manga/";  // Madara mangaSubString

static std::string manga_url(const std::string &slug) {
    return std::string(kSite) + kMangaPath + slug + "/";
}

// ─── encoding ───────────────────────────────────────────────────────────────
//
// Go's url.Values.Encode() (application/x-www-form-urlencoded): unreserved
// chars pass through, everything else is %XX, space becomes '+'. The host's
// text.url_encode is not form encoding, so build it here.
static std::string form_encode(const std::string &s) {
    static const char *hex = "0123456789ABCDEF";
    std::string out;
    for (unsigned char c : s) {
        if ((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_' ||
            c == '.' || c == '~') {
            out += (char)c;
        } else if (c == ' ') {
            out += '+';
        } else {
            out += '%';
            out += hex[c >> 4];
            out += hex[c & 0xF];
        }
    }
    return out;
}

// ─── parsing helpers ────────────────────────────────────────────────────────

// std::regex has no inline flags: Go `(?i)` → std::regex::icase, `(?s)` → [\s\S].
static std::regex re_i(const char *p) { return std::regex(p, std::regex::icase); }

// clean(): tags → " ", unescape entities, collapse whitespace.
static std::string clean(const std::string &s) {
    std::string t = std::regex_replace(s, std::regex(R"re(<[^>]*>)re"), " ");
    t = unescape_html(t);
    t = std::regex_replace(t, std::regex(R"re(\s+)re"), " ");
    return trim(t);
}

static std::string first_sub_or_empty(const std::regex &re, const std::string &s) {
    std::smatch m;
    if (std::regex_search(s, m, re) && m.size() > 1) return m[1].str();
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

static std::string abs_url(const std::string &raw) {
    if (raw.empty()) return "";
    if (raw.rfind("http://", 0) == 0 || raw.rfind("https://", 0) == 0) return raw;
    if (raw.compare(0, 2, "//") == 0) return "https:" + raw;
    if (raw[0] == '/') return std::string(kSite) + raw;
    return std::string(kSite) + "/" + raw;
}

// imgURL(): best lazy-aware image URL (attribute order matters).
static std::string img_url_of(const std::string &s) {
    for (const char *name : {"data-src", "data-lazy-src", "data-cfsrc", "data-manga-src", "src"}) {
        std::regex re(std::string(R"re((?:\s|^))re") + name + R"re(="\s*([^"]+)")re");
        std::smatch m;
        if (std::regex_search(s, m, re)) {
            std::string v = trim(m[1].str());
            if (!v.empty()) return v;
        }
    }
    return "";
}

static double chapter_num(const std::string &title) {
    std::smatch m;
    if (std::regex_search(title, m, std::regex(R"re(\d+(?:\.\d+)?)re"))) return std::atof(m[0].str().c_str());
    return 0;
}

// KSGroupScans' status vocabulary is narrower than the shared helper's.
static std::string normalize_status_site(const std::string &raw) {
    std::string l = lower(raw);
    if (l.find("ongoing") != std::string::npos || l.find("on going") != std::string::npos ||
        l.find("updating") != std::string::npos)
        return "Ongoing";
    if (l.find("complete") != std::string::npos) return "Completed";
    if (l.find("hiatus") != std::string::npos || l.find("hold") != std::string::npos ||
        l.find("pause") != std::string::npos)
        return "Hiatus";
    if (l.find("cancel") != std::string::npos) return "Cancelled";
    return "";
}

// All whole-match occurrences of `re` (no capture groups needed).
static std::vector<std::string> full_matches(const std::regex &re, const std::string &body, size_t max = 500) {
    std::vector<std::string> out;
    for (std::sregex_iterator it(body.begin(), body.end(), re), end; it != end && out.size() < max; ++it) {
        out.push_back((*it)[0].str());
    }
    return out;
}

// parseCards(): split an archive listing on the card marker and read each
// card's own href/title/thumbnail, so covers never bleed across cards.
static std::string parse_cards(const std::string &body) {
    rapidjson::Document out;
    out.SetArray();
    auto &alloc = out.GetAllocator();

    std::vector<size_t> locs;
    std::regex cardStart(R"re(class="page-item-detail)re");
    for (std::sregex_iterator it(body.begin(), body.end(), cardStart), end; it != end; ++it) {
        locs.push_back((size_t)it->position());
    }

    std::vector<std::string> seen;
    for (size_t i = 0; i < locs.size(); i++) {
        size_t start = locs[i];
        size_t end = (i + 1 < locs.size()) ? locs[i + 1] : body.size();
        std::string seg = body.substr(start, end - start);

        std::string slug = slug_of(first_sub_or_empty(std::regex(R"re(href="([^"]+)")re"), seg));
        if (slug.empty() || std::find(seen.begin(), seen.end(), slug) != seen.end()) continue;
        seen.push_back(slug);

        std::string title_block = first_sub_or_empty(std::regex(R"re(class="post-title[^"]*">([\s\S]*?)</div>)re"), seg);
        std::string title = clean(first_sub_or_empty(std::regex(R"re(<h[1-6][^>]*>([\s\S]*?)</h[1-6]>)re"), title_block));
        std::smatch im;
        std::string cover = std::regex_search(seg, im, re_i(R"re(<img\b[^>]*>)re")) ? img_url_of(im[0].str()) : "";

        rapidjson::Value m(rapidjson::kObjectType);
        add_string(m, "id", slug, alloc);
        add_string(m, "title", title, alloc);
        add_string(m, "cover_url", abs_url(cover), alloc);
        out.PushBack(m, alloc);
    }
    return to_json(out);
}

// ─── search ─────────────────────────────────────────────────────────────────

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

    // Madara admin-ajax form, key order mirroring url.Values.Encode().
    std::vector<std::pair<std::string, std::string>> form = {
        {"action", "madara_load_more"},
        {"page", std::to_string(page - 1)},
        {"template", "madara-core/content/content-archive"},
        {"vars[paged]", "1"},
        {"vars[template]", "archive"},
        {"vars[posts_per_page]", "25"},
        {"vars[post_type]", "wp-manga"},
        {"vars[post_status]", "publish"},
        {"vars[manga_archives_item_layout]", "big_thumbnail"},
    };
    if (!query.empty()) {
        form.push_back({"vars[s]", query});
    } else {
        form.push_back({"vars[orderby]", "meta_value_num"});
        form.push_back({"vars[meta_key]", "_wp_manga_views"});
        form.push_back({"vars[order]", "DESC"});
    }
    if (!genres.empty()) {
        form.push_back({"vars[tax_query][0][taxonomy]", "wp-manga-genre"});
        form.push_back({"vars[tax_query][0][field]", "slug"});
        for (size_t i = 0; i < genres.size(); i++)
            form.push_back({"vars[tax_query][0][terms][" + std::to_string(i) + "]", genres[i]});
    }

    std::string body_str;
    for (size_t i = 0; i < form.size(); i++) {
        if (i) body_str += "&";
        body_str += form_encode(form[i].first) + "=" + form_encode(form[i].second);
    }

    std::string full;
    if (!http_post_body(kAjaxURL, body_str,
                        R"re({"X-Requested-With":"XMLHttpRequest","Content-Type":"application/x-www-form-urlencoded"})re",
                        full))
        return "[]";
    return parse_cards(full);
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

    std::string title_block = first_sub_or_empty(std::regex(R"re(class="post-title[^"]*">([\s\S]*?)</div>)re"), body);
    std::string title = clean(first_sub_or_empty(std::regex(R"re(<h[1-6][^>]*>([\s\S]*?)</h[1-6]>)re"), title_block));
    if (!title.empty()) add_string(m, "title", title, alloc);

    std::smatch cm;
    std::string cover_block = std::regex_search(body, cm, std::regex(R"re(class="summary_image"[\s\S]*?</a>)re"))
                                  ? cm[0].str() : "";
    std::string cover = abs_url(img_url_of(cover_block));
    if (!cover.empty()) add_string(m, "cover_url", cover, alloc);

    std::string author = clean(first_sub_or_empty(std::regex(R"re(class="author-content">([\s\S]*?)</div>)re"), body));
    if (!author.empty()) add_string(m, "author", author, alloc);

    std::string desc = clean(first_sub_or_empty(
        std::regex(R"re(class="summary__content[^"]*">([\s\S]*?)<span\s+class="[^"]*content-readmore")re"), body));
    if (desc.empty())
        desc = clean(first_sub_or_empty(std::regex(R"re(class="summary__content[^"]*">([\s\S]*?)</div>)re"), body));
    if (!desc.empty()) add_string(m, "description", desc, alloc);

    std::string st = first_sub_or_empty(
        std::regex(R"re(Status\s*</h5>[\s\S]*?class="summary-content">\s*([^<]+))re"), body);
    if (!st.empty()) add_string(m, "status", normalize_status_site(st), alloc);

    rapidjson::Value garr(rapidjson::kArrayType);
    std::string gb = first_sub_or_empty(std::regex(R"re(class="genres-content">([\s\S]*?)</div>)re"), body);
    if (!gb.empty()) {
        std::regex anchorRe(R"re(<a[^>]*>([^<]+)</a>)re");
        for (std::sregex_iterator it(gb.begin(), gb.end(), anchorRe), end; it != end; ++it) {
            std::string n = clean((*it)[1].str());
            if (n.empty()) continue;
            garr.PushBack(rapidjson::Value(n.c_str(), (rapidjson::SizeType)n.size(), alloc), alloc);
        }
    }
    m.AddMember("genres", garr, alloc);
    return to_json(m);
}

// ─── chapters ───────────────────────────────────────────────────────────────

static std::string chapters_impl(const std::string &arg) {
    std::string slug = trim(decode_string(arg));
    if (slug.empty()) return "[]";

    std::string body;
    if (!http_post_body(manga_url(slug) + "ajax/chapters/", "",
                        R"re({"X-Requested-With":"XMLHttpRequest","Content-Type":"application/x-www-form-urlencoded"})re",
                        body))
        return "[]";

    rapidjson::Document out;
    out.SetArray();
    auto &alloc = out.GetAllocator();
    std::vector<std::string> seen;
    std::regex cardRe(R"re(<li[^>]*class="wp-manga-chapter[^"]*"[^>]*>([\s\S]*?)</li>)re");
    for (std::sregex_iterator it(body.begin(), body.end(), cardRe), end; it != end; ++it) {
        std::string inner = (*it)[1].str();
        std::string href = first_sub_or_empty(std::regex(R"re(href="([^"]+)")re"), inner);
        if (href.empty()) continue;
        std::string cslug = slug_of(href);
        if (cslug.empty() || std::find(seen.begin(), seen.end(), cslug) != seen.end()) continue;
        seen.push_back(cslug);
        std::string label = clean(first_sub_or_empty(std::regex(R"re(<a[^>]*>([\s\S]*?)</a>)re"), inner));

        rapidjson::Value ch(rapidjson::kObjectType);
        add_string(ch, "id", slug + ":" + cslug, alloc);
        add_string(ch, "manga_id", slug, alloc);
        add_string(ch, "title", label, alloc);
        ch.AddMember("chapter_num", chapter_num(label), alloc);
        add_string(ch, "url", abs_url(href), alloc);
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

    std::string chapter_url = manga_url(slug) + chapterSlug + "/";
    std::string body;
    if (!get_body(chapter_url, body)) return "[]";

    // Madara defaults to a paged reader; the list layout exposes the real <img> tags.
    if (body.find("id=\"single-pager\"") != std::string::npos) {
        std::string list_body;
        if (get_body(chapter_url + "?style=list", list_body)) body = list_body;
    }

    rapidjson::Document out;
    out.SetArray();
    auto &alloc = out.GetAllocator();
    for (auto &tag : full_matches(re_i(R"re(<img\b[^>]*>)re"), body)) {
        if (tag.find("wp-manga-chapter-img") == std::string::npos) continue;
        std::string src = img_url_of(tag);
        if (src.empty()) continue;
        rapidjson::Value pg(rapidjson::kObjectType);
        pg.AddMember("index", (int)out.Size(), alloc);
        add_string(pg, "url", abs_url(src), alloc);
        out.PushBack(pg, alloc);
    }
    return to_json(out);
}

// ─── genres ─────────────────────────────────────────────────────────────────

static std::string genres_impl() {
    std::string body;
    if (!get_body(std::string(kSite) + kMangaPath, body)) return "[]";

    rapidjson::Document out;
    out.SetArray();
    auto &alloc = out.GetAllocator();
    std::vector<std::string> seen;
    std::regex linkRe(R"re(<a[^>]*href="([^"]*manga-genre/([^"/]+)/?)"[^>]*>([\s\S]*?)</a>)re");
    for (std::sregex_iterator it(body.begin(), body.end(), linkRe), end; it != end; ++it) {
        std::string gslug = (*it)[2].str();
        std::string name = clean((*it)[3].str());
        if (gslug.empty() || name.empty() || std::find(seen.begin(), seen.end(), gslug) != seen.end()) continue;
        seen.push_back(gslug);
        rapidjson::Value g(rapidjson::kObjectType);
        add_string(g, "name", name, alloc);
        add_string(g, "slug", gslug, alloc);
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
