-- main.lua — MangaDE (https://mangade.io) plugin for goIsekai
-- ABI: contract_version 1. Entry points take a JSON string and return a JSON
-- string. HTTP failures return an empty result with nil error (host
-- convention) and never surface as Lua errors.
--
-- MangaDE exposes a documented JSON API on api.mangade.io (/api-docs.html):
--   GET /api/comics?page=N&size=M&name=QUERY      → search (totalCount/totalPage)
--   GET /api/comics/{id}/view                     → detail + news_chapters
--   GET /api/chapters/{id}/view                   → chapter_images (public, no auth)
-- All responses: { success, message, data }. Decimal chapter numbers are
-- strings in chapter_number ("59", "17.5"). News_chapters arrive
-- newest-first already; the explicit sort keeps the ABI invariant.

PLUGIN = {
	contract_version = 1,
	name = "MangaDE",
	site_url = "https://mangade.io",
	logo = "logo.png",
	thumb_ratio = 0.703,
}

-- decode_id accepts the host's marshaled bare id ("46959") or an object
-- like {"id":"46959"}; returns the id string, or nil.
local function decode_id(arg)
	local v = host.json.decode(arg)
	if type(v) == "table" then
		v = v.id
	end
	return v
end

-- api_get fetches {url} and returns the decoded data table, or nil on any
-- failure (HTTP error, non-success envelope, malformed JSON).
local function api_get(url)
	local body = host.http.get_body(url)
	if body == "" then
		return nil
	end
	local ok, decoded = pcall(host.json.decode, body)
	if not ok or type(decoded) ~= "table" or decoded.success ~= true or type(decoded.data) ~= "table" then
		return nil
	end
	return decoded.data
end

-- ─── search_manga ──────────────────────────────────────────────────────────
-- GET https://api.mangade.io/api/comics?page={N}&size=20&name={query}
-- The name filter is applied server-side. The API pges via page/size; the
-- host slices results per search_page_size, so 20 per call is plenty.

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
	local url = "https://api.mangade.io/api/comics?page="
		.. tostring(page)
		.. "&size=20&name="
		.. host.text.url_encode(query)
	local data = api_get(url)
	if data == nil or type(data.list) ~= "table" then
		return host.json.encode({}), nil
	end

	local results = {}
	for _, item in ipairs(data.list) do
		results[#results + 1] = {
			id = tostring(item.id or ""),
			title = host.text.trim(item.name or ""),
			cover_url = item.image or "",
		}
	end
	return host.json.encode(results), nil
end

-- ─── get_manga_detail ──────────────────────────────────────────────────────
-- GET https://api.mangade.io/api/comics/{id}/view — no author/artist field
-- exists on the API; status is a raw string; genres[] carry {name}.

function get_manga_detail(arg)
	local manga_id = decode_id(arg)
	if manga_id == nil or manga_id == "" then
		return host.json.encode({}), nil
	end
	local data = api_get("https://api.mangade.io/api/comics/" .. manga_id .. "/view")
	if data == nil then
		-- Failure shape for the object export: bare {id} (api.go never caches
		-- it, so the next call retries). An [] here would not decode.
		return host.json.encode({ id = manga_id }), nil
	end

	local genres = {}
	for _, g in ipairs(data.genres or {}) do
		if type(g) == "table" and g.name and g.name ~= "" then
			genres[#genres + 1] = g.name
		end
	end

	return host.json.encode({
		id = manga_id,
		title = host.text.trim(data.name or ""),
		cover_url = data.image or "",
		status = host.text.normalize_status(data.status or ""),
		description = data.description or "",
		genres = genres,
	}),
		nil
end

-- ─── get_chapter_list ──────────────────────────────────────────────────────
-- Same /view response; news_chapters[] carries every chapter (59 entries for
-- a 59-chapter series). chapter_number is a decimal-safe string.

function get_chapter_list(arg)
	local manga_id = decode_id(arg)
	if manga_id == nil or manga_id == "" then
		return host.json.encode({}), nil
	end
	local data = api_get("https://api.mangade.io/api/comics/" .. manga_id .. "/view")
	if data == nil or type(data.news_chapters) ~= "table" then
		return host.json.encode({}), nil
	end

	local chapters = {}
	for _, ch in ipairs(data.news_chapters) do
		local num = tonumber(ch.chapter_number)
		if num ~= nil then
			chapters[#chapters + 1] = {
				id = tostring(ch.id or ""),
				manga_id = manga_id,
				chapter_num = num,
				title = host.text.trim(ch.name or ""),
				url = "https://mangade.io/chapter/" .. tostring(ch.id or ""),
			}
		end
	end

	-- Newest-first ABI order regardless of API order.
	table.sort(chapters, function(a, b)
		return a.chapter_num > b.chapter_num
	end)
	return host.json.encode(chapters), nil
end

-- ─── get_page_list ─────────────────────────────────────────────────────────
-- GET https://api.mangade.io/api/chapters/{id}/view → data.chapter_images[]
-- ({image, ...}); public, no auth. Items arrive page-ordered; index is
-- assigned from position to guarantee 0-based monotonicity. No special
-- image headers: plain https://s3-load.ttr.group URLs.

function get_page_list(arg)
	local chapter_id = decode_id(arg)
	if chapter_id == nil or chapter_id == "" then
		return host.json.encode({}), nil
	end
	local data = api_get("https://api.mangade.io/api/chapters/" .. chapter_id .. "/view")
	if data == nil or type(data.chapter_images) ~= "table" then
		return host.json.encode({}), nil
	end

	local pages = {}
	for i, img in ipairs(data.chapter_images) do
		local url = type(img) == "table" and img.image or nil
		if url and url ~= "" then
			pages[#pages + 1] = {
				index = i - 1,
				url = url,
			}
		end
	end
	return host.json.encode(pages), nil
end
