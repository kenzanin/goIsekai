-- main.lua — FanFox (https://m.fanfox.net) plugin for goIsekai
-- ABI: contract_version 1. Entry points take a JSON string and return a JSON
-- string. HTTP failures return an empty result with nil error (host
-- convention) and never surface as Lua errors.

PLUGIN = {
	contract_version = 1,
	name = "FanFox",
	site_url = "https://m.fanfox.net",
	logo = "logo.png",
	thumb_ratio = 0.703,
}

local util = require("util")

local IMAGE_HEADERS = { Referer = "https://m.fanfox.net/" }

-- decode_id accepts the host's marshaled bare id ("blue_lock") or an object
-- like {"id":"blue_lock"}; returns the id string, or nil.
local function decode_id(arg)
	local v = host.json.decode(arg)
	if type(v) == "table" then
		v = v.id
	end
	return v
end

-- ─── search_manga ──────────────────────────────────────────────────────────
-- GET https://fanfox.net/search?title={query}&page={N} — desktop site only;
-- the mobile search endpoint is a JS shell that returns nothing.

function search_manga(arg)
	local a = host.json.decode(arg)
	if type(a) == "string" then
		a = { query = a, page = 1 }
	end
	local query = a and (a.query or a.q)
	if query == nil or query == "" then
		return host.json.encode({}), nil
	end
	local page = tonumber(a.page) or 1
	local url = "https://fanfox.net/search?title=" .. host.text.url_encode(query) .. "&page=" .. tostring(page)
	local html = host.http.get_body(url)
	if html == "" then
		return host.json.encode({}), nil
	end
	return host.json.encode(util.parse_search(html)), nil
end

-- ─── get_manga_detail ──────────────────────────────────────────────────────
-- GET https://m.fanfox.net/manga/{id}/ — mobile detail page carries title,
-- cover, author, status, genres, summary and the full chapter list.

function get_manga_detail(arg)
	local manga_id = decode_id(arg)
	if manga_id == nil or manga_id == "" then
		return host.json.encode({}), nil
	end
	local html = host.http.get_body("https://m.fanfox.net/manga/" .. manga_id .. "/")
	if html == "" then
		-- Failure shape for the object export: bare {id} (api.go never caches
		-- it, so the next call retries). An [] here would not decode.
		return host.json.encode({ id = manga_id }), nil
	end
	return host.json.encode(util.parse_manga_detail(html, manga_id)), nil
end

-- ─── get_chapter_list ──────────────────────────────────────────────────────
-- Same detail page as get_manga_detail; parsed separately per the ABI.

function get_chapter_list(arg)
	local manga_id = decode_id(arg)
	if manga_id == nil or manga_id == "" then
		return host.json.encode({}), nil
	end
	local html = host.http.get_body("https://m.fanfox.net/manga/" .. manga_id .. "/")
	if html == "" then
		return host.json.encode({}), nil
	end
	return host.json.encode(util.parse_chapter_list(html, manga_id)), nil
end

-- ─── get_page_list ─────────────────────────────────────────────────────────
-- GET https://m.fanfox.net/manga/{chapter_id}/{N}.html for N = 1..total.
-- Image URLs are per-request tokenized (token + ttl), so every page is
-- fetched at call time and the host caches the downloaded images, not URLs.

function get_page_list(arg)
	local chapter_id = decode_id(arg)
	if chapter_id == nil or chapter_id == "" then
		return host.json.encode({}), nil
	end
	local base = "https://m.fanfox.net/manga/" .. chapter_id .. "/"
	local first = host.http.get_body(base .. "1.html")
	if first == "" then
		return host.json.encode({}), nil
	end

	local pages = {}
	for n = 1, util.parse_page_count(first, chapter_id) do
		local html = first
		if n > 1 then
			html = host.http.get_body(base .. tostring(n) .. ".html")
		end
		local src = util.parse_page_image(html)
		if src ~= "" then
			pages[#pages + 1] = {
				index = n - 1,
				url = src,
				headers = IMAGE_HEADERS,
			}
		end
	end
	return host.json.encode(pages), nil
end
