-- main.lua — Fenyk (https://fenyk.org) plugin for goIsekai
-- ABI: contract_version 1. Lua globals: search, get_manga_detail,
-- get_chapter_list, get_page_list. All errors propagate as nil result,
-- nil error (host convention) so transient failures never surface.
--
-- API base: https://fenyk.org/api/v1
--   GET /search?q=QUERY&filter=popular&tags=TAG&limit=N&page_no=P
--   GET /content/series/{id}
--   GET /content/series/{id}/chapters
--   GET /content/chapters/{id}/pages?copy={copy_id}
--   GET /tags  (genres + tags catalog)
-- Page URLs are served from https://img.fenyk.org/<path>; use that as Referer.

PLUGIN = {
	contract_version = 1,
	name = "Fenyk",
	site_url = "https://fenyk.org",
	thumb_ratio = 0.71,
	paginates = false,
}

local API = "https://fenyk.org/api/v1"

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
	local body = host.http.get_body(url)
	if body == "" then return nil end
	local ok, data = pcall(host.json.decode, body)
	if not ok or type(data) ~= "table" then return nil end
	return data
end

-- ─── search ────────────────────────────────────────────────────────────────
-- GET /api/v1/search?q=QUERY&filter=popular&tags=TAG&limit=20&page_no=P
-- The tags param is a *name* (not id). The first matching genre wins for
-- browse-only mode (no query, only genres).
-- SearchFilter.page maps to page_no; limit 20 per call is enough (host slices).
--
-- result shape per item: {id, title, description, content_rating, status,
--   tags, chapters_count, latest_chapter, last_chapter_at, cover_url, ...}

function search(arg)
	local f = host.json.decode(arg) or {}
	local query = f.query or ""
	local page = tonumber(f.page) or 1
	local genres = f.genres or {}

	log.debug("search q=" .. tostring(query) .. " page=" .. tostring(page))

	-- Build URL
	local url = API .. "/search?limit=20&page_no=" .. tostring(page)
	if query and query ~= "" then
		url = url .. "&q=" .. host.text.url_encode(query)
		url = url .. "&filter=popular"
	end
	if #genres > 0 then
		-- First genre wins as a tag filter (Fenyk accepts name-based tags).
		url = url .. "&tags=" .. host.text.url_encode(genres[1])
	end
	if query == "" and #genres == 0 then
		-- Browse: no query, no tags → fallback to most popular.
		url = url .. "&filter=popular"
	end

	local data = http_get(url)
	if not data or not data.results then return host.json.encode({}), nil end

	local results = {}
	for _, item in ipairs(data.results) do
		results[#results + 1] = {
			id            = tostring(item.id or ""),
			title         = host.text.trim(item.title or ""),
			cover_url     = item.cover_url or "",
			description   = item.description or "",
			status        = item.status or "",
			chapters_count= item.chapters_count or 0,
			latest_chapter= item.latest_chapter or "",
			last_chapter_at= item.last_chapter_at or "",
			tags          = item.tags or {},
		}
	end
	return host.json.encode(results), nil
end

-- ─── get_manga_detail ──────────────────────────────────────────────────────
-- GET /api/v1/content/series/{id}
-- Returns {detail: <Series>, chapters: [<Chapter>]} when available.
-- The detail carries authors[] and artists[] (namespace/name). Tags[] are
-- strings (genres already returned by search). Status, cover, description
-- map directly.

function get_manga_detail(arg)
	local series_id = decode_id(arg)
	if not series_id or series_id == "" then
		return host.json.encode({}), nil
	end
	local data = http_get(API .. "/content/series/" .. series_id)
	if not data then
		return host.json.encode({ id = series_id }), nil
	end

	-- Authors: pick first author namespace, else first artist.
	local author = ""
	for _, a in ipairs(data.authors or {}) do
		if a.namespace == "author" and a.name then
			author = a.name; break
		end
	end
	if author == "" then
		for _, a in ipairs(data.artists or {}) do
			if a.name then author = a.name; break end
		end
	end

	-- Tags are [{namespace, name, id}] objects; extract just the names.
	local genres = {}
	for _, t in ipairs(data.tags or {}) do
		if type(t) == "table" and t.name then
			genres[#genres + 1] = t.name
		end
	end

	return host.json.encode({
		id          = tostring(data.id or series_id),
		title       = host.text.trim(data.title or ""),
		cover_url   = data.cover_url or "",
		description = data.description or "",
		status      = data.status or "",
		genres      = genres,
		author      = author,
	}), nil
end

-- ─── get_chapter_list ──────────────────────────────────────────────────────
-- GET /api/v1/content/series/{id}/chapters
-- Returns array of chapter objects. Each has: title, chapter_no, id,
-- published_at, copies[{id, group, main}].
--
-- Build one Chapter per main copy per chapter (main copy preferred; when
-- none is marked main, fall back to the first copy). chapter_no is parsed
-- as a float; decimal chapters are natural (e.g. 2.10 → 2.1). Order: newest
-- first (as the API returns).
--
-- The ABI expects chapter_num as a float. published_at may be nil for some
-- entries (use last_chapter_at from detail as fallback at top level; per
-- chapter we still emit what we have).

function get_chapter_list(arg)
	local series_id = decode_id(arg)
	if not series_id or series_id == "" then
		return host.json.encode({}), nil
	end
	local data = http_get(API .. "/content/series/" .. series_id .. "/chapters")
	if not data or type(data) ~= "table" then return host.json.encode({}), nil end

	-- Sort newest-first by published_at (ISO string, sortable).
	table.sort(data, function(a, b)
		return tostring(a.published_at or "") > tostring(b.published_at or "")
	end)

	local chapters = {}
	for _, ch in ipairs(data) do
		local chap_no = tonumber(ch.chapter_no) or 0
		-- Pick the main copy if any; otherwise the first copy.
		local chosen_copy = nil
		for _, c in ipairs(ch.copies or {}) do
			if c.main then chosen_copy = c; break end
		end
		if not chosen_copy then
			chosen_copy = ch.copies[1]
		end

		chapters[#chapters + 1] = {
			id         = tostring(ch.id or ""),
			manga_id   = tostring(series_id),
			title      = host.text.trim(ch.title or ""),
			chapter_num= chap_no,
			released_at= ch.published_at or "",
			url        = "https://fenyk.org/series/" .. series_id,
			scanlator  = chosen_copy and (chosen_copy.group or "") or "",
		}
	end
	return host.json.encode(chapters), nil
end

-- ─── get_page_list ─────────────────────────────────────────────────────────
-- GET /api/v1/content/chapters/{chapter_id}/pages?copy={copy_id}
-- Returns {pages: [{page_number, page_index, url, width, height}],
--           total_pages, state}.
--
-- Fenyk stores each scanlation-copy's pages under a keyed path inside the
-- chapter. We pick the copy-id by matching chapter.id → found in the chapter
-- list returned by get_chapter_list. The plugin must carry enough context:
-- the chapter_id alone doesn't identify which copy to use; we look up the
-- copy from the series/chapters cache or default to the main copy.
--
-- Since get_page_list is called independently (no direct series context), we
-- accept both {"chapter_id":"..."} and {"chapter_id":"...", "copy_id":"..."}
-- — the latter skips lookup. When copy_id is omitted, we request the chapter
-- metadata to find the main copy id ourselves.

function get_page_list(arg)
	-- The host passes a bare chapter-id JSON string, not an object.
	local chapter_id = decode_id(arg)
	if not chapter_id or chapter_id == "" then
		return host.json.encode({}), nil
	end

	-- Fenyk's pages endpoint is inconsistent: the no-copy URL and per-copy
	-- URLs can each return state=unavailable while another succeeds (cold
	-- cache / rate limit server-side). Try no-copy first, then every copy.
	local function pages_of(url)
		local data = http_get(url)
		if not data then return nil end
		local out = {}
		for _, p in ipairs(data.pages or {}) do
			out[#out + 1] = {
				index = p.page_index or (p.page_number or 1) - 1,
				url   = p.url or "",
				headers = { Referer = "https://fenyk.org/" },
			}
		end
		return out
	end

	local base = API .. "/content/chapters/" .. chapter_id
	local pages = pages_of(base .. "/pages")
	if not pages or #pages == 0 then
		-- Chapter metadata carries copies[{id}] — try each in turn.
		local cd = http_get(base)
		if cd and type(cd.copies) == "table" then
			for _, c in ipairs(cd.copies) do
				if type(c) == "table" and c.id then
					pages = pages_of(base .. "/pages?copy=" .. c.id)
					if pages and #pages > 0 then break end
				end
			end
		end
	end
	if not pages then return host.json.encode({}), nil end
	return host.json.encode(pages), nil
end

-- ─── get_genres ────────────────────────────────────────────────────────────
-- GET /api/v1/tags → {genres:[{name,count}], tags:[{name,count}]}
-- Genres and tags are both strings the series detail carries. Return a flat
-- catalog that the host can expose as browseable filters. Genres win as
-- top-level; tags are appended.

local cached_genres = nil
function get_genres()
	if cached_genres then return cached_genres end
	local data = http_get(API .. "/tags")
	if not data then cached_genres = host.json.encode({}); return cached_genres end

	local out = {}
	for _, g in ipairs(data.genres or {}) do
		out[#out + 1] = { name = g.name or "", slug = g.name or "" }
	end
	for _, t in ipairs(data.tags or {}) do
		out[#out + 1] = { name = t.name or "", slug = t.name or "" }
	end
	cached_genres = host.json.encode(out)
	return cached_genres
end
