// html_util.h — libxml2 helpers for native goIsekai plugins (C++17).
//
// Two jobs: turn a rendered HTML fragment into plain text, and read a field out
// of the page's schema.org block. Both go through a real HTML parser rather
// than regexes, because the pages are not well-formed.
#pragma once

#include <libxml/HTMLparser.h>
#include <libxml/tree.h>

#include <string>
#include <vector>

#include "json_util.h"
#include "text_util.h"

static htmlDocPtr parse_html(const std::string &html) {
	if (html.empty()) return nullptr;
	return htmlReadMemory(html.c_str(), (int)html.size(), nullptr, "UTF-8",
	                      HTML_PARSE_RECOVER | HTML_PARSE_NOERROR | HTML_PARSE_NOWARNING);
}

static void collect_text(xmlNodePtr node, std::string &out) {
	for (xmlNodePtr n = node; n; n = n->next) {
		if (n->type == XML_TEXT_NODE || n->type == XML_CDATA_SECTION_NODE) {
			if (n->content) out += (const char *)n->content;
			continue;
		}
		if (n->type != XML_ELEMENT_NODE) continue;
		bool block = xmlStrEqual(n->name, BAD_CAST "p") || xmlStrEqual(n->name, BAD_CAST "div") ||
		             xmlStrEqual(n->name, BAD_CAST "br") || xmlStrEqual(n->name, BAD_CAST "li") ||
		             xmlStrEqual(n->name, BAD_CAST "h1") || xmlStrEqual(n->name, BAD_CAST "h2") ||
		             xmlStrEqual(n->name, BAD_CAST "h3") || xmlStrEqual(n->name, BAD_CAST "h4") ||
		             xmlStrEqual(n->name, BAD_CAST "tr") || xmlStrEqual(n->name, BAD_CAST "blockquote");
		if (block) out += ' ';
		collect_text(n->children, out);
		if (block) out += ' ';
	}
}

static std::string html_to_text(const std::string &html) {
	htmlDocPtr doc = parse_html(html);
	if (!doc) return "";
	std::string out;
	if (xmlNodePtr root = xmlDocGetRootElement(doc)) collect_text(root, out);
	xmlFreeDoc(doc);
	return trim(collapse_ws(out));
}

// ld_json_field reads one field from the page's schema.org blocks.
//
// Two traps, both confirmed against live pages:
//  1. libxml2 does not reliably nest a <script type="application/ld+json"> body
//     into the script element — the JSON arrives as a bare CDATA node — so
//     walking elements misses it. Scanning text/CDATA nodes for a schema.org
//     document is immune to how the tree came out.
//  2. A page carries several blocks (WebSite, BreadcrumbList, ComicSeries). The
//     wanted field is usually in the last one, so every block is tried and the
//     first that actually has the field wins.
static std::string ld_json_field(const std::string &html, const char *field) {
	htmlDocPtr doc = parse_html(html);
	if (!doc) return "";

	std::vector<std::string> blocks;
	std::vector<xmlNodePtr> stack;
	if (xmlNodePtr root = xmlDocGetRootElement(doc)) stack.push_back(root);
	while (!stack.empty()) {
		xmlNodePtr n = stack.back();
		stack.pop_back();
		for (; n; n = n->next) {
			if ((n->type == XML_TEXT_NODE || n->type == XML_CDATA_SECTION_NODE) && n->content &&
			    xmlStrstr(n->content, BAD_CAST "\"@context\"")) {
				blocks.push_back((const char *)n->content);
			}
			if (n->children) stack.push_back(n->children);
		}
	}
	xmlFreeDoc(doc);

	for (const auto &block : blocks) {
		rapidjson::Document d;
		d.Parse(block.c_str(), block.size());
		if (d.HasParseError() || !d.IsObject()) continue;
		auto it = d.FindMember(field);
		if (it == d.MemberEnd() || !it->value.IsString()) continue;
		return it->value.GetString();
	}
	return "";
}
