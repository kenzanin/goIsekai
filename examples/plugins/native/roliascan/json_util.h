// json_util.h — RapidJSON helpers for native goIsekai plugins (C++17).
//
// The native counterpart of the wasm plugins' util.h: the ABI passes JSON
// strings in and expects a JSON string back, so this file owns the "byte
// buffer out" convention and the typed accessors every parser needs.
#pragma once

#include <cstdlib>
#include <cstring>
#include <string>

#include <rapidjson/document.h>
#include <rapidjson/stringbuffer.h>
#include <rapidjson/writer.h>

// dup_c copies into a malloc'd NUL-terminated buffer. The plugin owns every
// buffer it returns; the host copies it, then calls plugin_free on it.
static char *dup_c(const std::string &s) {
	char *p = (char *)malloc(s.size() + 1);
	if (!p) return nullptr;
	memcpy(p, s.c_str(), s.size() + 1);
	return p;
}

// error_json is the shared failure envelope: {"error":"code"}. The host turns a
// lone "error" key into a plugin error, so no separate return channel is needed.
static char *error_json(const char *code) {
	return dup_c(std::string("{\"error\":\"") + code + "\"}");
}

static char *document_json(const rapidjson::Document &d) {
	rapidjson::StringBuffer sb;
	rapidjson::Writer<rapidjson::StringBuffer> w(sb);
	d.Accept(w);
	return dup_c(sb.GetString());
}

static void add_str(rapidjson::Value &obj, const char *k, const std::string &v,
                    rapidjson::Document::AllocatorType &a) {
	obj.AddMember(rapidjson::Value(k, a), rapidjson::Value(v.c_str(), (rapidjson::SizeType)v.size(), a), a);
}

static std::string jstr(const rapidjson::Value &obj, const char *k) {
	if (!obj.IsObject()) return "";
	auto it = obj.FindMember(k);
	if (it == obj.MemberEnd() || !it->value.IsString()) return "";
	return it->value.GetString();
}

static int jint(const rapidjson::Value &obj, const char *k, int def) {
	if (!obj.IsObject()) return def;
	auto it = obj.FindMember(k);
	if (it == obj.MemberEnd() || !it->value.IsInt()) return def;
	return it->value.GetInt();
}

// parse_string_arg unwraps the ABI argument for the calls whose input is a bare
// string (a manga id, a chapter id) rather than an object.
static std::string parse_string_arg(const char *json) {
	rapidjson::Document d;
	d.Parse(json ? json : "");
	if (d.IsString()) return d.GetString();
	return "";
}
