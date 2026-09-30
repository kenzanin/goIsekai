-- util.lua — MangaTown (m.mangatown.com) HTML parsing helpers
-- Sibling module required by main.lua via require("util")
local util = {}

-- Referer required by every reader/chapterfun image (zjcdn.mangahere.org).
function util.image_headers()
	return { Referer = "https://m.mangatown.com/" }
end

-- ─── Search result parsing ─────────────────────────────────────────────────
-- GET https://www.mangatown.com/search.php?name=...&page=N (desktop site;
-- the mobile search path renders nothing). Each result row:
--   <a class="manga_cover" href="/manga/{slug}/" title="T">
--     <img src="https://fmcdn.mangahere.com/...">
--   ...<P class="title"><a href="/manga/{slug}/" title="T">
-- The cover anchor carries the image; the title anchor is plain. Numeric
-- redirect slugs are skipped like fanfox.

function util.parse_search(html)
	local results = {}
	local seen = {}

	local function add(slug, title, cover)
		if not slug or slug == "" or tonumber(slug) ~= nil then
			return
		end
		if seen[slug] then
			return
		end
		seen[slug] = true
		results[#results + 1] = {
			id = slug,
			title = host.text.trim(host.text.unescape(title or "")),
			cover_url = host.text.unescape(cover or ""),
		}
	end

	-- Pass 1: cover rows with images.
	for href, title, cover in
		host.regex.gmatch(
			html,
			[[<a class="manga_cover" href="(/manga/[^"/]+/)" title="([^"]+)">\s*<img[^>]*src="([^"]+)"]]
		)
	do
		add(host.regex.find(href, [[^/manga/([^/]+)/]]), title, cover)
	end

	-- Pass 2: title rows (covers may be lazy/absent) — dedupe by slug.
	for href, title in host.regex.gmatch(html, [[href="(/manga/[^"/]+/)" title="([^"]+)"]]) do
		add(host.regex.find(href, [[^/manga/([^/]+)/]]), title, "")
	end

	return results
end

-- ─── Manga detail parsing ──────────────────────────────────────────────────
-- GET https://m.mangatown.com/manga/{slug}/ (mobile detail page):
--   <div class="title">Kimi no Knife</div>
--   <img src="..." class="detail-cover">
--   <p><span>Author(s): </span><a ...>NAME</a></p>
--   <p><span>Status: </span> Completed</p>
--   <p><span>Genre(s):</span> <a title="Drama">Drama</a> ...</p>
--   <span id="hide">short...</span><span id="show" style="display: none;">FULL</span>
-- The "show" span carries the untruncated summary; "hide" is the teaser.

function util.parse_manga_detail(html, manga_id)
	local detail = { id = manga_id }

	detail.title = host.text.trim(host.text.unescape(host.regex.find(html, [[class="title">([^<]+)<]]) or ""))
	detail.cover_url = host.text.unescape(host.regex.find(html, [[<img src="([^"]+)"[^>]*class="detail-cover"]]) or "")

	detail.author = host.text.strip_html(host.regex.find(html, [[Author\(s\):\s*</span>(.*?)</p>]]) or "")

	detail.status = host.text.normalize_status(host.regex.find(html, [[Status:\s*</span>\s*([^<]+)]]) or "")

	-- Prefer the full "show" span; fall back to the "hide" teaser.
	local summary = host.regex.find(html, [[(?s)<span id="show"[^>]*>(.*?)</span>]])
		or host.regex.find(html, [[(?s)<span id="hide">(.*?)</span>]])
		or ""
	detail.description = host.text.strip_html(summary)

	local genres = {}
	local genre_block = host.regex.find(html, [[(?s)Genre\(s\):(.*?)</p>]]) or ""
	for g in host.regex.gmatch(genre_block, [[<a[^>]*title="[^"]*">(.*?)</a>]]) do
		local name = host.text.strip_html(g)
		if name ~= "" then
			genres[#genres + 1] = name
		end
	end
	detail.genres = genres

	return detail
end

-- ─── Chapter list parsing ──────────────────────────────────────────────────
-- Same detail page. Links look like (volume prefix only on some series):
--   href="/manga/{slug}/v09/c011.5/"   (kimi_no_kakera)
--   href="/manga/{slug}/c070/"         (kimi_no_knife)
-- Row text "C.71" is padded; the number is rebuilt from the path instead.
-- Decimal safety: c([0-9][0-9.]*)$ anchored to the path end (rule: [\d.]+,
-- never \d+ — c011.5 and c012.1 exist).

function util.parse_chapter_list(html, manga_id)
	local chapters = {}
	local seen = {}
	local pattern = [[href="/manga/]] .. manga_id .. [[/((?:v[0-9]+/)?c[0-9.]+)/"]]

	for rest in host.regex.gmatch(html, pattern) do
		local num_str = host.regex.find(rest, [[c([0-9][0-9.]*)$]])
		local num = tonumber(num_str)
		if num ~= nil and not seen[rest] then
			seen[rest] = true
			local volume = tonumber(host.regex.find(rest, [[^v([0-9]+)]]))
			chapters[#chapters + 1] = {
				id = manga_id .. "/" .. rest,
				manga_id = manga_id,
				chapter_num = num,
				volume_num = volume,
				title = string.format("Chapter %g", num),
				url = "https://m.mangatown.com/manga/" .. manga_id .. "/" .. rest .. "/1.html",
			}
		end
	end

	-- Newest-first ABI order regardless of DOM order.
	table.sort(chapters, function(a, b)
		return a.chapter_num > b.chapter_num
	end)
	return chapters
end

-- ─── Page count + image parsing ────────────────────────────────────────────
-- Reader page: https://m.mangatown.com/manga/{chapter_id}/{N}.html. Every
-- page is server-rendered with its own <img id="image"> (verified p1 + p2,
-- no packed JS on the mobile site — that is the desktop path). The page
-- count comes from the max {N}.html in the page <select> (windows in the
-- pager undercount). NOTE: nonexistent chapters return a 200 empty shell
-- with no image at all, so callers must treat "" as a miss.

function util.parse_page_count(html, chapter_id)
	local max_page = 1
	local pattern = "/manga/" .. chapter_id .. [[/([0-9]+)\.html]]
	for n in host.regex.gmatch(html, pattern) do
		local v = tonumber(n)
		if v ~= nil and v > max_page then
			max_page = v
		end
	end
	return max_page
end

function util.parse_page_image(html)
	local tag = host.regex.find(html, [[<img[^>]*id="image"[^>]*>]]) or ""
	local src = host.text.unescape(host.regex.find(tag, [[src="([^"]+)"]]) or "")
	if src:sub(1, 2) == "//" then
		src = "https:" .. src
	end
	return src
end

-- ─── Page list via desktop chapterfun (2 images per request) ──────────
-- A long mobile chapter is N round-trips (34-page chapter ≈ 18-30 s, over
-- the 15 s invoke timeout — live-verified). The desktop path returns 2
-- pages per request from a Dean-Edwards packed JS blob:
--   1. mobile {chapter_id}/1.html   → page count (max N.html in select)
--   2. desktop /manga/{chapter_id}/ → var chapter_id={cid}
--   3. chapterfun.ashx?cid={cid}&page=K&key= (K=1,3,5..) → packed JS with
--      pix (folder) + 2 filenames
-- The packed pix truncates the folder's decimal suffix ("070." serves
-- "070.0", "09-011." serves "09-011.5"): re-append the chapter's decimal
-- part (integer chapters get "0"). DM5 replies overlap ([K,K+1]); dedupe
-- by filename, order preserved.

-- Dean-Edwards unpacker for the p,a,c,k,e,d payload chapterfun returns.
local function unpack_packer(js)
	-- Full packer tail: }('PAYLOAD',BASE,COUNT,'w|w'.split('|'),0,{})
	local payload, base, words = host.regex.find(js, [[(?s)\}\('(.*)',(\d+),\d+,'(.*)'\.split\('\|'\),0,\{\}\)]])
	if payload == nil then
		return nil
	end
	local n = tonumber(base)
	-- Words include EMPTY entries; gmatch("[^|]+") would drop them and
	-- shift every later index, so split manually.
	local list = {}
	local start = 1
	while true do
		local pipe = words:find("|", start, true)
		if pipe == nil then
			list[#list + 1] = words:sub(start)
			break
		end
		list[#list + 1] = words:sub(start, pipe - 1)
		start = pipe + 1
	end
	-- base-N index encoder (Dean-Edwards e function): 0-9, then a-z
	-- (toString(36)) for 10-35, then A-Z (fromCharCode(c+29)) for 36-61.
	local function enc(x)
		local digits = ""
		while true do
			local r = x % n
			local c
			if r < 10 then
				c = tostring(r)
			elseif r < 36 then
				c = string.char(87 + r) -- 'a' + (r - 10)
			else
				c = string.char(29 + r) -- 'A' + (r - 36)
			end
			digits = c .. digits
			x = math.floor(x / n)
			if x == 0 then
				break
			end
		end
		return digits
	end
	local dict = {}
	for i, w in ipairs(list) do
		dict[enc(i - 1)] = w
	end
	return payload:gsub("%w+", dict)
end

local function chapter_num_suffix(chapter_id)
	-- "kimi_no_knife/c070" → "0"; "kimi_no_kakera/v09/c011.5" → "5"
	local num = host.regex.find(chapter_id, [[c([0-9][0-9.]*)$]]) or ""
	if num:find("%.", 1, true) then
		return num:match("%.([0-9]+)$") or "0"
	end
	return "0"
end

-- get_page_urls builds the full page list; get_body is the host fetch
-- closure (url, headers) handed over from main.lua. Entry is the DESKTOP
-- reader alone: one fetch carries both var chapter_id (cid) and
-- var total_pages (the mobile pager is a 9-link window, unusable for
-- totals, and skipping it saves a round-trip inside the 15 s invoke
-- budget — a 34-page chapter costs 1 + 17 chapterfun calls).
function util.get_page_urls(get_body, chapter_id)
	local mob_base = "https://m.mangatown.com/manga/" .. chapter_id .. "/"
	local desk_base = "https://www.mangatown.com/manga/" .. chapter_id .. "/"
	local desk = get_body(desk_base)
	if desk == "" or desk == nil then
		return nil
	end
	local cid = host.regex.find(desk, [[var chapter_id=([0-9]+);]])
	local total = tonumber(host.regex.find(desk, [[var total_pages=(\d+);]]) or "")
	if cid == nil or total == nil or total < 1 then
		-- No desktop reader data: fall back to per-page mobile scraping.
		local first = get_body(mob_base .. "1.html")
		if first == "" then
			return nil
		end
		local pages = {}
		for i = 1, util.parse_page_count(first, chapter_id) do
			local html = (i == 1) and first or get_body(mob_base .. tostring(i) .. ".html")
			local src = util.parse_page_image(html)
			if src ~= "" then
				pages[#pages + 1] = { index = i - 1, url = src, headers = util.image_headers() }
			end
		end
		return pages
	end

	local suffix = chapter_num_suffix(chapter_id)
	local seen = {}
	local urls = {}
	local k = 1
	while #urls < total and k <= total do
		local packed = get_body(
			"https://www.mangatown.com/chapterfun.ashx?cid=" .. cid .. "&page=" .. tostring(k) .. "&key=",
			{ Referer = desk_base }
		)
		if packed == "" or packed == nil then
			break
		end
		local js = unpack_packer(packed)
		if js == nil then
			break
		end
		-- Re-append the truncated decimal folder suffix, then join.
		local pix = (host.regex.find(js, [[pix="([^"]+)"]]) or "") .. suffix
		for fname in host.regex.gmatch(js, [["(/[^"]+%.jpg)"]]) do
			if not seen[fname] then
				seen[fname] = true
				urls[#urls + 1] = {
					index = #urls,
					url = "https:" .. pix .. fname,
					headers = util.image_headers(),
				}
			end
		end
		k = k + 2
	end
	return urls
end

return util
