-- main.lua — Comico (https://comico.moe) plugin for goIsekai
-- ABI: contract_version 1. Lua globals: search, get_manga_detail,
-- get_chapter_list, get_page_list, get_genres.
--
-- comico.moe is a Nuxt aggregator: catalogue metadata mirrors comick,
-- reader pages are proxied per-chapter. No auth/cookies needed.
--
-- API base: https://comico.moe
--   GET  /api/search?source=comick&q=Q&comickTags=TAG&limit=N&offset=O
--   GET  /api/manga/{uuid}
--   GET  /api/manga/{uuid}/chapters?language=en&order=desc&sort=number
--   GET  /api/tags?source=comick-metadata
--   GET  /api/reader/access/{chapterId}          -> {token}
--   POST /api/reader/chapters/{chapterId}/open?d=ENCODED_TOKEN
--        body {"proxyMethod":"all"}              -> {sessionId, pages:[rel paths]}
--   GET  https://comico.moe{page path}          -> image bytes

PLUGIN = {
	contract_version = 1,
	name = "Comico",
	site_url = "https://comico.moe",
	thumb_ratio = 0.7,
	paginates = false,
}

local SITE = "https://comico.moe"
local API = SITE .. "/api"
local UA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
local HEADERS = { ["User-Agent"] = UA, ["Accept"] = "application/json" }

-- decode_id accepts a bare string or {"id":"..."}; returns id, or nil.
local function decode_id(arg)
	local v = host.json.decode(arg)
	if type(v) == "table" then
		v = v.id
	end
	return v
end

-- http_get fetches url and returns decoded table, or nil on any failure.
local function http_get(url)
	local body = host.http.get_body(url, HEADERS)
	if body == nil or body == "" then return nil end
	local ok, data = pcall(host.json.decode, body)
	if not ok or type(data) ~= "table" then return nil end
	return data
end

-- abs_url prefixes the site host on comico's relative asset paths.
local function abs_url(u)
	if type(u) ~= "string" or u == "" then return "" end
	if u:sub(1, 4) == "http" then return u end
	return SITE .. u
end

-- tags_to_names flattens [{id,name,group}] into a name list.
local function tags_to_names(tags)
	local out = {}
	for _, t in ipairs(tags or {}) do
		if type(t) == "table" and t.name then
			out[#out + 1] = t.name
		end
	end
	return out
end

-- ─── search ────────────────────────────────────────────────────────────────
-- GET /api/search?source=comick&q=Q&limit=100&offset=O
-- items: [{id, sourceId, sourceMangaId, title, coverUrl, status,
--          contentRating, format, year, tags[], comickTags}]
--
-- The host owns paging (plugin.json has paginates:false), so we must return
-- every match: fetch in 100-row slices until total is reached. limit>100 400s.

function search(arg)
	local f = host.json.decode(arg) or {}
	local query = f.query or ""
	local genres = f.genres or {}

	local base = API .. "/search?source=comick&sort=relevance&limit=100"
		.. "&contentRating=safe&contentRating=suggestive"
	if query ~= "" then
		base = base .. "&q=" .. host.text.url_encode(query)
	end
	if #genres > 0 then
		base = base .. "&comickTags=" .. host.text.url_encode(genres[1])
	end

	local results = {}
	local offset = 0
	for _ = 1, 20 do
		local data = http_get(base .. "&offset=" .. tostring(offset))
		if not data or not data.items or #data.items == 0 then break end
		for _, item in ipairs(data.items) do
			results[#results + 1] = {
				id             = tostring(item.id or ""),
				title          = host.text.trim(item.title or ""),
				cover_url      = abs_url(item.coverUrl),
				description    = item.description or "",
				status         = item.status or "",
				latest_chapter = "",
				tags           = tags_to_names(item.tags),
			}
		end
		offset = offset + #data.items
		if offset >= (tonumber(data.total) or 0) then break end
	end
	return host.json.encode(results), nil
end

-- ─── get_manga_detail ──────────────────────────────────────────────────────
-- GET /api/manga/{uuid} -> single object with altTitles/authors/tags.

function get_manga_detail(arg)
	local manga_id = decode_id(arg)
	if not manga_id or manga_id == "" then
		return host.json.encode({}), nil
	end
	local data = http_get(API .. "/manga/" .. manga_id)
	if not data then
		return host.json.encode({ id = manga_id }), nil
	end

	local author = ""
	for _, a in ipairs(data.authors or {}) do
		if type(a) == "table" and a.name then author = a.name; break end
		if type(a) == "string" then author = a; break end
	end

	return host.json.encode({
		id          = tostring(data.id or manga_id),
		title       = host.text.trim(data.title or ""),
		cover_url   = abs_url(data.coverUrl),
		description = data.description or "",
		status      = data.status or "",
		genres      = tags_to_names(data.tags),
		author      = author,
	}), nil
end

-- ─── get_chapter_list ──────────────────────────────────────────────────────
-- GET /api/manga/{uuid}/chapters?language=en&order=desc&sort=number
-- `source` is optional; omitting it returns every source's chapters.
-- items: [{id, mangaId, sourceId, title, volume, number, pageCount,
--          scanlationGroup, publishedAt}]

function get_chapter_list(arg)
	local manga_id = decode_id(arg)
	if not manga_id or manga_id == "" then
		return host.json.encode({}), nil
	end

	-- The API rejects limit > 100 (400), so page through in 100s.
	local base = API .. "/manga/" .. manga_id
		.. "/chapters?language=en&order=desc&sort=number&limit=100"
	local chapters = {}
	local offset = 0
	for _ = 1, 20 do
		local data = http_get(base .. "&offset=" .. tostring(offset))
		if not data or not data.items or #data.items == 0 then break end
		for _, ch in ipairs(data.items) do
			chapters[#chapters + 1] = {
				id          = tostring(ch.id or ""),
				manga_id    = tostring(manga_id),
				title       = host.text.trim(ch.title or ""),
				chapter_num = tonumber(ch.number) or 0,
				released_at = ch.publishedAt or "",
				url         = SITE .. "/manga/" .. manga_id,
				scanlator   = ch.scanlationGroup or "",
			}
		end
		offset = offset + #data.items
		if offset >= (tonumber(data.total) or 0) then break end
	end
	return host.json.encode(chapters), nil
end

-- ─── get_page_list ─────────────────────────────────────────────────────────
-- Two-step reader flow (see header): access token, then a POST to /open.
-- The response carries ready-to-use relative page paths that already embed
-- the session token in their query string.

function get_page_list(arg)
	local chapter_id = decode_id(arg)
	if not chapter_id or chapter_id == "" then
		return host.json.encode({}), nil
	end

	local access = http_get(API .. "/reader/access/" .. chapter_id)
	if not access or not access.token then return host.json.encode({}), nil end

	local open_url = API .. "/reader/chapters/" .. chapter_id
		.. "/open?d=" .. host.text.url_encode(access.token)
	local body = host.http.post_body(open_url, '{"proxyMethod":"all"}', {
		["User-Agent"] = UA,
		["Accept"] = "application/json",
		["Content-Type"] = "application/json",
		["Origin"] = SITE,
		["Referer"] = SITE .. "/reader/" .. chapter_id,
	})
	if body == nil or body == "" then return host.json.encode({}), nil end

	local ok, opened = pcall(host.json.decode, body)
	if not ok or type(opened) ~= "table" then return host.json.encode({}), nil end

	local pages = {}
	for i, p in ipairs(opened.pages or {}) do
		pages[#pages + 1] = {
			index = i - 1,
			url   = abs_url(p),
			headers = { Referer = SITE .. "/" },
		}
	end
	return host.json.encode(pages), nil
end

-- ─── get_genres ────────────────────────────────────────────────────────────
-- GET /api/tags?source=comick-metadata -> [{id, name, group}]

local cached_genres = nil
function get_genres()
	if cached_genres then return cached_genres end
	local data = http_get(API .. "/tags?source=comick-metadata")
	if not data then
		cached_genres = host.json.encode({})
		return cached_genres
	end

	local out = {}
	for _, t in ipairs(data) do
		if type(t) == "table" and t.name then
			out[#out + 1] = { name = t.name, slug = t.id or t.name }
		end
	end
	cached_genres = host.json.encode(out)
	return cached_genres
end
