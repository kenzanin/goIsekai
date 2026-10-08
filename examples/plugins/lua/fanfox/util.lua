-- util.lua — FanFox (m.fanfox.net) HTML parsing helpers
-- Sibling module required by main.lua via require("util")
local util = {}

-- ─── Search result parsing ─────────────────────────────────────────────────
-- GET https://fanfox.net/search?title=...&page=N (desktop site). Each result
-- renders twice (cover row then title row); both anchors carry title="...".
-- Bare numeric links (/manga/10891) are redirects and are skipped.

function util.parse_search(html)
	local results = {}
	local seen = {}

	local function add(slug, title, cover)
		-- tonumber() rejects numeric redirect slugs like "10891".
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

	for href, title, cover in
		host.regex.gmatch(html, [[href="(/manga/[^"/]+/)" title="([^"]+)">\s*<img[^>]*src="([^"]+)"]])
	do
		add(host.regex.find(href, [[^/manga/([^/]+)/]]), title, cover)
	end

	for href, title in host.regex.gmatch(html, [[href="(/manga/[^"/]+/)" title="([^"]+)"]]) do
		add(host.regex.find(href, [[^/manga/([^/]+)/]]), title, "")
	end

	return results
end

-- ─── Manga detail parsing ─────────────────────────────────────────────────
-- GET https://m.fanfox.net/manga/{slug}/ (mobile detail page). The first
-- <div class="title"> is the manga name; a later widget with the same class
-- is a login prompt, so first-match-wins is load-bearing.

function util.parse_manga_detail(html, manga_id)
	local detail = { id = manga_id }

	detail.title = host.text.trim(host.text.unescape(host.regex.find(html, [[class="title">([^<]+)<]]) or ""))
	-- Real markup: <div class="manga-detail"> <div class="manga-detail-top"> <p class="title"> then
	-- <img src="//fmcdn..." ... class="detail-cover" /> — match the unique detail-cover
	-- img itself (class comes AFTER src), then normalize the protocol-relative URL.
	local cover = host.regex.find(html, [[<img src="([^"]+)"[^>]*class="detail-cover"]]) or ""
	cover = host.text.unescape(cover)
	if cover:sub(1, 2) == "//" then
		cover = "https:" .. cover
	end
	detail.cover_url = cover

	-- Author(s): <a ...>NAME</a> lives on the same <p> as Artist(s), so cut
	-- at <br (or </p> when there is no artist row).
	detail.author = host.text.strip_html(host.regex.find(html, [[Author\(s\):\s*(.*?)(?:<br|</p>)]]) or "")

	detail.status = host.text.normalize_status(host.regex.find(html, [[Status:\s*([^<]+)<]]) or "")

	detail.description =
		host.text.strip_html(host.regex.find(html, [[(?s)<div class="manga-summary">(.*?)</div>]]) or "")

	local genres = {}
	local genre_block = host.regex.find(html, [[(?s)<div class="manga-genres">(.*?)</div>]]) or ""
	for g in host.regex.gmatch(genre_block, [[<a[^>]*>(.*?)</a>]]) do
		local name = host.text.strip_html(g)
		if name ~= "" then
			genres[#genres + 1] = name
		end
	end
	detail.genres = genres

	return detail
end

-- ─── Chapter list parsing ──────────────────────────────────────────────────
-- Same detail page. Links: href="//m.fanfox.net/manga/{slug}/[vNN/]cNNN/1.html"
-- Chapter id is the path after /manga/ minus the /1.html suffix, e.g.
-- "blue_lock/c161" or "blue_lock/v01/c001". Decimal chapters (c288.5) are
-- first-class: the number regex is c([0-9][0-9.]*)$ (anchored to the path
-- end), never c(\d+). Row text is "Ch N" plus nbsp padding and a date span,
-- so the title is rebuilt from the number instead of scraped.

function util.parse_chapter_list(html, manga_id)
	local chapters = {}
	local seen = {}
	-- Scope to this manga's links so other series' /1.html links never leak in.
	local pattern = [[href="//m\.fanfox\.net/manga/]] .. manga_id .. [[/([^"]+)/1\.html"]]

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
				url = "https://m.fanfox.net/manga/" .. manga_id .. "/" .. rest .. "/1.html",
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
-- Reader page: https://m.fanfox.net/manga/{chapter_id}/{N}.html. The page
-- count comes from every {N}.html reference for THIS chapter (pager links
-- show a window, the <select class="mangaread-page"> lists all pages).
-- The current page's image is <img ... id="image" src="//zjcdn.../kNNN.jpg?token=...">;
-- attribute order varies and the URL carries &amp; entities.

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
	-- Whole-tag match keeps src/id order-agnostic; the hidden "next" img
	-- (display:none) has no id="image" and never matches.
	local tag = host.regex.find(html, [[<img[^>]*id="image"[^>]*>]]) or ""
	local src = host.text.unescape(host.regex.find(tag, [[src="([^"]+)"]]) or "")
	-- Page images are protocol-relative (//zjcdn.mangafox.me/...).
	if src:sub(1, 2) == "//" then
		src = "https:" .. src
	end
	return src
end

-- ─── Desktop chapterfun path (batched, 2 images per request) ───────────────
-- The mobile path costs one request PER page (0.9 s each; a 49-page chapter
-- is ~45 s, far over the 15 s invoke default). The desktop reader
-- (fanfox.net, same DM5 stack as mangahere) exposes var chapterid /
-- var imagecount and chapterfun.ashx returns a Dean-Edwards packed JS with
-- pix (folder, decimal-truncated: "02-008." serves "02-008.0") plus 2
-- token-stamped filenames per call; replies overlap ([K,K+1]), so dedupe
-- by filename. An empty key works (curl-verified).

-- Dean-Edwards unpacker for the p,a,c,k,e,d payload chapterfun returns.
local function unpack_packer(js)
	-- Full packer tail: }('PAYLOAD',BASE,COUNT,'w|w'.split('|'),0,{})
	local payload, base, words = host.regex.find(js, [[(?s)\}\('(.*)',(\d+),\d+,'(.*)'\.split\('\|'\),0,\{\}\)]])
	if payload == nil then
		return nil
	end
	local n = tonumber(base)
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
	local function enc(x)
		local digits = ""
		while true do
			local r = x % n
			local c
			if r < 10 then
				c = tostring(r)
			elseif r < 36 then
				c = string.char(87 + r)
			else
				c = string.char(29 + r)
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
	-- "slug/c008" → "0"; "slug/v02/c008.5" → "5"
	local num = host.regex.find(chapter_id, [[c([0-9][0-9.]*)$]]) or ""
	if host.regex.find(num, [[\.([0-9]+)$]]) ~= nil then
		return host.regex.find(num, [[([0-9]+)$]])
	end
	return "0"
end

function util.get_page_urls(get_body, chapter_id)
	local base = "https://fanfox.net/manga/" .. chapter_id .. "/"
	local first = get_body(base .. "1.html")
	if first == "" or first == nil then
		return nil
	end
	local cid = host.regex.find(first, [[var chapterid\s*=\s*([0-9]+);]])
	local total = tonumber(host.regex.find(first, [[var imagecount\s*=\s*(\d+);]]) or "")
	if cid == nil then
		return nil
	end
	if total == nil or total < 1 then
		total = 1000
	end

	local suffix = chapter_num_suffix(chapter_id)
	local seen = {}
	local urls = {}
	local k = 1
	while #urls < total and k <= total do
		local packed = get_body(
			"https://fanfox.net/chapterfun.ashx?cid=" .. cid .. "&page=" .. tostring(k) .. "&key=",
			{ Referer = base .. tostring(math.ceil(k / 2)) .. ".html" }
		)
		if packed == "" or packed == nil then
			break
		end
		local js = unpack_packer(packed)
		if js == nil then
			break
		end
		-- Re-insert the truncated decimal into the folder ("02-008." →
		-- "02-008.5"); the /compressed tail stays after it.
		local pix = (host.regex.find(js, [[pix="([^"]+)"]]) or ""):gsub("([0-9])%./", "%1." .. suffix .. "/", 1)
		for fname in host.regex.gmatch(js, [["(/[^"]+\.jpg[^"]*)"]]) do
			if not seen[fname] then
				seen[fname] = true
				urls[#urls + 1] = {
					index = #urls,
					url = "https:" .. pix .. fname,
					headers = { Referer = "https://fanfox.net/" },
				}
			end
		end
		k = k + 2
	end
	return urls
end

return util
