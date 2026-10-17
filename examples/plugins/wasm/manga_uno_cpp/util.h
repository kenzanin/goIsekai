// util.h — reusable text/JSON helpers for goIsekai WASM plugins (C++17).
//
// Copy into a new plugin directory as-is. Depends on wasm_abi.h (RapidJSON).
// Trim, URL-encode, JSON decode, HTML-ish humanize, status normalization and
// the add_string JSON-object helper shared by every plugin.
#pragma once

#include <cctype>
#include <cstdint>
#include <cstring>
#include <ctime>
#include <string>
#include <vector>
#include <algorithm>

#include <rapidjson/document.h>

#include "wasm_abi.h"

static std::string trim(const std::string &s) {
    size_t a = s.find_first_not_of(" \t\r\n");
    if (a == std::string::npos) return "";
    size_t b = s.find_last_not_of(" \t\r\n");
    return s.substr(a, b - a + 1);
}

// The host passes every export argument as a JSON string literal.
static std::string decode_string(const std::string &arg) {
    rapidjson::Document d;
    d.Parse(arg.c_str());
    return d.IsString() ? d.GetString() : "";
}

static std::string lower(const std::string &s) {
    std::string l = s;
    std::transform(l.begin(), l.end(), l.begin(), [](unsigned char c) { return (char)std::tolower(c); });
    return l;
}

// "some-slug" → "Some Slug"
static std::string humanize(const std::string &slug) {
    std::string out;
    bool cap = true;
    for (char c : slug) {
        if (c == '-' || c == '_') { out += ' '; cap = true; }
        else if (cap && c >= 'a' && c <= 'z') { out += (char)(c - 32); cap = false; }
        else { out += c; cap = false; }
    }
    return out;
}

// "Sci-Fi" → "sci-fi" (spaces to dashes, lowercased)
static std::string genre_slug(const std::string &g) {
    std::string s = trim(g);
    std::transform(s.begin(), s.end(), s.begin(), [](unsigned char c) {
        if (c == ' ') return '-';
        return (char)std::tolower(c);
    });
    return s;
}

// Delegates to the host so the logic cannot drift from the Lua/JS runtimes.
// ponytail: substring-based like the original (catches "notongoing" too);
// switch to host.text.normalize_status (exact-key map) if upstream gets noisier.
static std::string normalize_status(const std::string &raw) {
    std::string l = lower(raw);
    auto has = [&](const char *k) { return l.find(k) != std::string::npos; };
    if (has("ongoing") || has("on going") || has("updating")) return "Ongoing";
    if (has("complete") || has("finished")) return "Completed";
    if (has("hiatus") || has("hold") || has("pause")) return "Hiatus";
    if (has("cancel") || has("drop")) return "Cancelled";
    return "";
}

static std::string url_encode(const std::string &s) { return host_str("text.url_encode", s); }

static std::string url_decode(const std::string &s) { return host_str("text.url_decode", s); }

// RFC3339 in UTC via host.text.date_to_iso; "" when unparsable.
static std::string parse_time(const std::string &s) { return s.empty() ? "" : host_str("text.date_to_iso", s); }

static void add_string(rapidjson::Value &obj, const char *key, const std::string &val,
                       rapidjson::Document::AllocatorType &alloc) {
    obj.AddMember(rapidjson::Value(key, (rapidjson::SizeType)strlen(key), alloc),
                  rapidjson::Value(val.c_str(), (rapidjson::SizeType)val.size(), alloc), alloc);
}
