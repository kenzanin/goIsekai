-- main.lua — MangaTown (https://m.mangatown.com) plugin for goIsekai
-- ABI: contract_version 1. Entry points take a JSON string and return a JSON
-- string. HTTP failures return an empty result with nil error (host
-- convention) and never surface as Lua errors.

PLUGIN = {
	contract_version = 1,
	name = "MangaTown",
	site_url = "https://m.mangatown.com",
	logo = "logo.png",
	thumb_ratio = 0.703,
	-- Page lists assemble from ~1 request per 2 pages (chapterfun pairs);
	-- a 34-page chapter needs ~18 round-trips. Extend the 15 s invoke default.
	timeout = 60,
}

local util = require("util")

local IMAGE_HEADERS = { Referer = "https://m.mangatown.com/" }

-- decode_id accepts the host's marshaled bare id ("kimi_no_knife") or an
-- object like {"id":"kimi_no_knife"}; returns the id string, or nil.
local function decode_id(arg)
	local v = host.json.decode(arg)
	if type(v) == "table" then
		v = v.id
	end
	return v
end

-- ─── search_manga ──────────────────────────────────────────────────────────
-- GET https://www.mangatown.com/search.php?name={query}&page={N} — desktop
-- site only; the mobile search path renders nothing (curl-verified).

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
	local url = "https://www.mangatown.com/search.php?name="
		.. host.text.url_encode(query)
		.. "&page="
		.. tostring(page)
	local html = host.http.get_body(url)
	if html == "" then
		return host.json.encode({}), nil
	end
	return host.json.encode(util.parse_search(html)), nil
end

-- ─── get_manga_detail ──────────────────────────────────────────────────────
-- GET https://m.mangatown.com/manga/{id}/ — mobile detail page carries
-- title, cover, author, status, genres, summary and the full chapter list.

function get_manga_detail(arg)
	local manga_id = decode_id(arg)
	if manga_id == nil or manga_id == "" then
		return host.json.encode({}), nil
	end
	local html = host.http.get_body("https://m.mangatown.com/manga/" .. manga_id .. "/")
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
	local html = host.http.get_body("https://m.mangatown.com/manga/" .. manga_id .. "/")
	if html == "" then
		return host.json.encode({}), nil
	end
	return host.json.encode(util.parse_chapter_list(html, manga_id)), nil
end

-- ─── get_page_list ─────────────────────────────────────────────────────────
-- GET https://m.mangatown.com/manga/{chapter_id}/{N}.html for N = 1..total.
-- Every reader page is server-rendered with <img id="image"> (verified on
-- pages 1 AND 2 — no packed-JS path on the mobile site). Chapter ids keep
-- the volume prefix when present ("kimi_no_kakera/v09/c011.5"). A
-- nonexistent chapter returns a 200 empty shell with no <img id="image">;
-- that page contributes nothing and an all-empty chapter returns [].

function get_page_list(arg)
	local chapter_id = decode_id(arg)
	if chapter_id == nil or chapter_id == "" then
		return host.json.encode({}), nil
	end
	-- Desktop chapterfun path: 2 images per request (a 34-page mobile
	-- chapter costs N round-trips and blows the 15 s invoke timeout).
	local pages = util.get_page_urls(function(url, headers)
		return host.http.get_body(url, headers) or ""
	end, chapter_id) or {}
	return host.json.encode(pages), nil
end
