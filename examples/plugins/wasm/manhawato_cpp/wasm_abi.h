// wasm_abi.h — reusable goIsekai WASM plugin ABI glue for C++17 (wasi-sdk).
//
// Copy this file into a new plugin directory as-is. It provides:
//   • the env.host_call import (JSON dispatch — see docs/plugin-wasm-helpers.md)
//   • read_string / pack / reply_json / alloc_mem / free_mem
//   • host_call_fn / http_body / get_body / http_post_body / get_json
//   • the contract_version export (always 1)
//
// The plugin's own main.cpp must export, with extern "C" __attribute__((export_name(...))):
//   Search, GetMangaDetail, GetChapterList, GetPageList  (+ optional GetGenres)
// using reply_json(...) to return each payload. malloc/free/_initialize come
// from libc++ automatically (link with -Wl,--export=malloc -Wl,--export=free).
//
// Build: /opt/wasi-sdk/bin/clang++ -std=c++17 -O2 -fno-exceptions -fno-rtti \
//        --target=wasm32-wasi -mexec-model=reactor -idirafter /usr/include \
//        main.cpp -o main.wasm -Wl,--export=malloc -Wl,--export=free \
//        -Wl,-z,stack-size=1048576
#pragma once

#include <cstdint>
#include <cstdlib>
#include <cstring>
#include <string>
#include <vector>

#include <rapidjson/document.h>
#include <rapidjson/stringbuffer.h>
#include <rapidjson/writer.h>

extern "C" uint64_t host_call(uint32_t ptr, uint32_t len)
    __attribute__((import_module("env"), import_name("host_call")));

// ─── memory ─────────────────────────────────────────────────────────────────

static void *alloc_mem(uint32_t n) {
    if (n == 0) n = 1;
    return std::malloc(n);
}
static void free_mem(void *p) { std::free(p); }

static uint64_t pack(uint32_t ptr, uint32_t len) { return (uint64_t)len << 32 | ptr; }

static std::string read_string(uint32_t ptr, uint32_t len) {
    if (ptr == 0 || len == 0) return "";
    return std::string((const char *)(uintptr_t)ptr, len);
}

static uint64_t reply_json(const std::string &json) {
    uint32_t ptr = (uint32_t)(uintptr_t)alloc_mem((uint32_t)json.size() + 1);
    if (ptr == 0) return pack(0, 0);
    std::memcpy((void *)(uintptr_t)ptr, json.c_str(), json.size() + 1);
    return pack(ptr, (uint32_t)json.size());
}

extern "C" __attribute__((export_name("contract_version"))) int32_t contract_version() { return 1; }

// ─── host helpers ───────────────────────────────────────────────────────────

static std::string host_call_fn(const std::string &fn, const std::vector<std::string> &args) {
    rapidjson::Document req;
    req.SetObject();
    auto &alloc = req.GetAllocator();
    req.AddMember("fn", rapidjson::Value(fn.c_str(), (rapidjson::SizeType)fn.size(), alloc), alloc);
    rapidjson::Value arr(rapidjson::kArrayType);
    for (const auto &a : args) {
        arr.PushBack(rapidjson::Value(a.c_str(), (rapidjson::SizeType)a.size(), alloc), alloc);
    }
    req.AddMember("args", arr, alloc);
    rapidjson::StringBuffer sb;
    rapidjson::Writer<rapidjson::StringBuffer> w(sb);
    req.Accept(w);

    std::string req_str = sb.GetString();
    uint32_t ptr = (uint32_t)(uintptr_t)alloc_mem((uint32_t)req_str.size() + 1);
    if (ptr == 0) return "";
    std::memcpy((void *)(uintptr_t)ptr, req_str.c_str(), req_str.size() + 1);

    uint64_t out = host_call(ptr, (uint32_t)req_str.size());
    free_mem((void *)(uintptr_t)ptr);

    uint32_t rptr = (uint32_t)out;
    uint32_t rlen = (uint32_t)(out >> 32);
    if (rptr == 0 || rlen == 0) return "";
    std::string resp = read_string(rptr, rlen);
    free_mem((void *)(uintptr_t)rptr);
    return resp;
}

// http.get_body returns a JSON string on 200, else null.
static bool http_body(const std::string &url, const std::string &headers_json, std::string &out) {
    std::string raw = host_call_fn("http.get_body", {url, headers_json});
    if (raw.empty() || raw == "null" || raw[0] == '{') return false;
    rapidjson::Document d;
    d.Parse(raw.c_str());
    if (!d.IsString()) return false;
    out = d.GetString();
    return !out.empty();
}

static bool get_body(const std::string &url, std::string &out) {
    return http_body(url, "", out);
}

// http.post_body returns a JSON string on 200, else null. Args: url, body,
// headersJSON. WordPress admin-ajax/Madara endpoints need this.
static bool http_post_body(const std::string &url, const std::string &body, const std::string &headers_json,
                           std::string &out) {
    std::string raw = host_call_fn("http.post_body", {url, body, headers_json});
    if (raw.empty() || raw == "null" || raw[0] == '{') return false;
    rapidjson::Document d;
    d.Parse(raw.c_str());
    if (!d.IsString()) return false;
    out = d.GetString();
    return !out.empty();
}

// Call a one-string host helper (e.g. "text.url_encode") and return its JSON
// string result, or "" on the {"error":...} envelope. Lets the shared headers
// delegate text logic to the host instead of re-implementing it.
static std::string host_str(const char *fn, const std::string &arg) {
    std::string raw = host_call_fn(fn, {arg});
    if (raw.empty() || raw == "null" || raw[0] == '{') return "";
    rapidjson::Document d;
    d.Parse(raw.c_str());
    return d.IsString() ? d.GetString() : "";
}

// Serialize a RapidJSON value (object/array) to a compact JSON string.
static std::string to_json(const rapidjson::Value &v) {
    rapidjson::StringBuffer sb;
    rapidjson::Writer<rapidjson::StringBuffer> w(sb);
    v.Accept(w);
    return sb.GetString();
}
