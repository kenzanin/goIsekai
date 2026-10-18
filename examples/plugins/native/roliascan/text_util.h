// text_util.h — string, date and digest helpers for native plugins (C++17).
//
// OpenSSL provides MD5 because the site signs its chapter-list request with it;
// nothing here is security-sensitive.
#pragma once

#include <openssl/evp.h>

#include <cstdio>
#include <ctime>
#include <string>

static std::string trim(const std::string &s) {
	size_t a = s.find_first_not_of(" \t\r\n");
	if (a == std::string::npos) return "";
	size_t b = s.find_last_not_of(" \t\r\n");
	return s.substr(a, b - a + 1);
}

static std::string collapse_ws(const std::string &s) {
	std::string out;
	bool sp = false;
	for (char c : s) {
		if (c == ' ' || c == '\t' || c == '\r' || c == '\n') {
			sp = true;
			continue;
		}
		if (sp && !out.empty()) out += ' ';
		sp = false;
		out += c;
	}
	return out;
}

static std::string md5_hex(const std::string &in) {
	unsigned char dig[EVP_MAX_MD_SIZE];
	unsigned int len = 0;
	EVP_MD_CTX *ctx = EVP_MD_CTX_new();
	EVP_DigestInit_ex(ctx, EVP_md5(), nullptr);
	EVP_DigestUpdate(ctx, in.data(), in.size());
	EVP_DigestFinal_ex(ctx, dig, &len);
	EVP_MD_CTX_free(ctx);

	static const char *hex = "0123456789abcdef";
	std::string out;
	for (unsigned int i = 0; i < len; i++) {
		out += hex[dig[i] >> 4];
		out += hex[dig[i] & 0xf];
	}
	return out;
}

static std::string iso_utc(time_t t) {
	struct tm tm;
	gmtime_r(&t, &tm);
	char buf[32];
	strftime(buf, sizeof buf, "%Y-%m-%dT%H:%M:%SZ", &tm);
	return buf;
}

// relative_to_iso converts the chapter API's "2 days ago" into RFC3339. The
// host decodes ReleasedAt as a timestamp, so the raw text cannot be shipped.
static std::string relative_to_iso(const std::string &s) {
	long n = 0;
	char unit[32] = {0};
	if (sscanf(s.c_str(), "%ld %31s", &n, unit) != 2) return "";
	std::string u(unit);
	long secs;
	if (u.rfind("second", 0) == 0) secs = n;
	else if (u.rfind("minute", 0) == 0) secs = n * 60;
	else if (u.rfind("hour", 0) == 0) secs = n * 3600;
	else if (u.rfind("day", 0) == 0) secs = n * 86400;
	else if (u.rfind("week", 0) == 0) secs = n * 604800;
	else if (u.rfind("month", 0) == 0) secs = n * 2592000;
	else if (u.rfind("year", 0) == 0) secs = n * 31536000;
	else return "";
	return iso_utc(time(nullptr) - secs);
}
