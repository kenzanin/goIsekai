-- main.lua — MangaHere (https://www.mangahere.cc) plugin for goIsekai
-- ABI: contract_version 1. Entry points take a JSON string and return a JSON
-- string. HTTP failures return an empty result with nil error (host
-- convention) and never surface as Lua errors.
PLUGIN = {
	contract_version = 1,
	name = "MangaHere",
	site_url = "https://www.mangahere.cc",
	logo = "logo.png",
	thumb_ratio = 0.703,
	-- Page lists assemble from ~1 request per 2 pages (chapterfun pairs);
	-- a long chapter needs many round-trips. Extend the 15 s invoke default.
	timeout = 60,
}
local util = require("util")

-- decode_id accepts the host's marshaled bare id ("kumo_desu_ga_nani_ka") or
-- an object like {"id":"kumo_desu_ga_nani_ka"}; returns the id string, or nil.
local function decode_id(arg)
	local v = host.json.decode(arg)
	if type(v) == "table" then
		v = v.id
	end
	return v
end

-- ─── search_manga ──────────────────────────────────────────────────────────
-- Text search:  GET /search?title={query}&page={N} — server-rendered rows.
-- Genre search: GET /search?title={query}&genres={id}&nogenres=&sort=... — the
--   genre param is a numeric id from the GENRES table below; with an empty
--   title the endpoint still returns genre-filtered cards (HAR-confirmed).
function search_manga(arg)
	local a = host.json.decode(arg)
	if type(a) == "string" then
		a = { query = a, page = 1 }
	end
	local query = a and (a.query or a.q or "")
	local page = tonumber(a.page) or 1
	local genres = a and (a.genres or {})
	log.debug("mangahere search q=" .. query .. " page=" .. tostring(page) .. " genres=" .. tostring(#genres))

	local url
	if #genres > 0 then
		-- First genre wins (^A combine pattern). Empty title is valid: the
		-- endpoint returns genre-only results.
		url = "https://www.mangahere.cc/search?title=" .. host.text.url_encode(query)
			.. "&genres=" .. tostring(genres[1]) .. "&nogenres=&sort=name"
		if page > 1 then url = url .. "&page=" .. tostring(page) end
	else
		if query == "" then
			return host.json.encode({}), nil
		end
		url = "https://www.mangahere.cc/search?title=" .. host.text.url_encode(query) .. "&page=" .. tostring(page)
	end
	local html = host.http.get_body(url)
	if html == "" then
		return host.json.encode({}), nil
	end
	return host.json.encode(util.parse_search(html)), nil
end

-- ─── get_manga_detail ──────────────────────────────────────────────────────
-- GET https://www.mangahere.cc/manga/{id}/ — title, cover, author, status,
-- genres, full summary (the hidden .fullcontent paragraph).
function get_manga_detail(arg)
	local manga_id = decode_id(arg)
	if manga_id == nil or manga_id == "" then
		return host.json.encode({}), nil
	end
	local html = host.http.get_body("https://www.mangahere.cc/manga/" .. manga_id .. "/")
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
	local html = host.http.get_body("https://www.mangahere.cc/manga/" .. manga_id .. "/")
	if html == "" then
		return host.json.encode({}), nil
	end
	return host.json.encode(util.parse_chapter_list(html, manga_id)), nil
end

-- ─── get_page_list ─────────────────────────────────────────────────────────
-- chapter_id keeps the slug prefix ("kumo_desu_ga_nani_ka/c007.5");
-- util.get_page_urls resolves cid from the reader page and walks
-- chapterfun.ashx in pairs. Chapter ids are marshaled with ":" between
-- manga slug and chapter ("slug:c007.5") — restore the slash.
function get_page_list(arg)
	local chapter_id = decode_id(arg)
	if chapter_id == nil or chapter_id == "" then
		return host.json.encode({}), nil
	end
	chapter_id = chapter_id:gsub(":", "/")
	local urls = util.get_page_urls(host.http.get_body, chapter_id)
	if urls == nil then
		return host.json.encode({}), nil
	end
	return host.json.encode(urls), nil
end

-- ─── get_genres ──────────────────────────────────────────────────────────────
-- Genre names + numeric ids for /search?genres=<id>. Harvested from the
-- search-page genre dropdown markup: <a data-val="N" title="Name">Name</a>
-- (ids 19 and 29 are absent from the site, so the table skips them).
local GENRES = {
	{ name = "Action",          slug = "1" },
	{ name = "Adventure",       slug = "2" },
	{ name = "Comedy",          slug = "3" },
	{ name = "Fantasy",         slug = "4" },
	{ name = "Historical",      slug = "5" },
	{ name = "Horror",          slug = "6" },
	{ name = "Martial Arts",    slug = "7" },
	{ name = "Mystery",         slug = "8" },
	{ name = "Romance",         slug = "9" },
	{ name = "Shounen Ai",      slug = "10" },
	{ name = "Supernatural",    slug = "11" },
	{ name = "Drama",           slug = "12" },
	{ name = "Shounen",         slug = "13" },
	{ name = "School Life",     slug = "14" },
	{ name = "Shoujo",          slug = "15" },
	{ name = "Gender Bender",   slug = "16" },
	{ name = "Josei",           slug = "17" },
	{ name = "Psychological",   slug = "18" },
	{ name = "Slice of Life",   slug = "20" },
	{ name = "Sci-fi",          slug = "21" },
	{ name = "Ecchi",           slug = "22" },
	{ name = "Harem",           slug = "23" },
	{ name = "Shoujo Ai",       slug = "24" },
	{ name = "Yuri",            slug = "25" },
	{ name = "Mature",          slug = "26" },
	{ name = "Tragedy",         slug = "27" },
	{ name = "Yaoi",            slug = "28" },
	{ name = "Sports",          slug = "30" },
	{ name = "Adult",           slug = "31" },
	{ name = "One Shot",        slug = "32" },
	{ name = "Smut",            slug = "33" },
	{ name = "Mecha",           slug = "34" },
	{ name = "Shotacon",        slug = "35" },
	{ name = "Lolicon",         slug = "36" },
	{ name = "Webtoons",        slug = "37" },
}

function get_genres()
	local genres = {}
	for _, g in ipairs(GENRES) do
		genres[#genres + 1] = { name = g.name, slug = g.slug }
	end
	return host.json.encode(genres)
end
