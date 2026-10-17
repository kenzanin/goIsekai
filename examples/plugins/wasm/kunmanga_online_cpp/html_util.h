// html_util.h — reusable HTML-scraping helpers for goIsekai WASM plugins (C++17).
//
// Copy into a new plugin directory as-is. Depends on util.h.
// Regex-based extraction for server-rendered (Madara-like) themes:
// strip tags, decode the handful of HTML entities these sites emit, pull the
// first <h1>/og:* value, and read an <img> src from a tag fragment.
#pragma once

#include <string>
#include <regex>
#include <vector>
#include <algorithm>

#include "util.h"

// Both delegate to the host (host.text.strip_html / host.text.unescape), so the
// scraping vocabulary cannot drift from the Lua/JS runtimes. The host's
// strip_html already unescapes entities, so clean() = unescape(strip(x)) is
// idempotent — harmless.
static std::string strip_html(const std::string &s) { return host_str("text.strip_html", s); }

static std::string unescape_html(const std::string &s) { return host_str("text.unescape", s); }

// Trimmed first capture group of `re` inside `body`, or "".
static std::string first_sub(const std::regex &re, const std::string &body) {
    std::smatch m;
    if (!std::regex_search(body, m, re) || m.size() < 2) return "";
    return trim(m[1]);
}

// All capture-group-1 matches of `re`.
static std::vector<std::string> all_subs(const std::regex &re, const std::string &body, size_t max = 200) {
    std::vector<std::string> out;
    for (std::sregex_iterator it(body.begin(), body.end(), re), end; it != end && out.size() < max; ++it) {
        out.push_back((*it)[1].str());
    }
    return out;
}

// src (or data-src) attribute of an <img ...> tag fragment.
static std::string img_url(const std::string &tag) {
    std::smatch m;
    if (std::regex_search(tag, m, std::regex("data-src\\s*=\\s*\"([^\"]+)\""))) return trim(m[1]);
    if (std::regex_search(tag, m, std::regex("src\\s*=\\s*\"([^\"]+)\""))) return trim(m[1]);
    return "";
}

// Value of an arbitrary attribute in a tag fragment: attr(tag, "href").
static std::string attr(const std::string &tag, const char *name) {
    std::regex re(std::string(name) + "\\s*=\\s*\"([^\"]*)\"");
    std::smatch m;
    if (std::regex_search(tag, m, re)) return trim(m[1]);
    return "";
}

// Resolve a possibly-relative URL against a site origin (no path traversal beyond leading '/').
static std::string make_abs(const std::string &origin, const std::string &raw) {
    std::string u = trim(raw);
    if (u.empty()) return "";
    if (u.compare(0, 2, "//") == 0) return "https:" + u;
    if (u.rfind("http://", 0) == 0 || u.rfind("https://", 0) == 0) return u;
    if (!u.empty() && u[0] == '/') return origin + u;
    return origin + "/" + u;
}
