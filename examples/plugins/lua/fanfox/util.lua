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
	detail.cover_url = host.text.unescape(host.regex.find(html, [[class="manga-detail">\s*<img src="([^"]+)"]]) or "")

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

return util
