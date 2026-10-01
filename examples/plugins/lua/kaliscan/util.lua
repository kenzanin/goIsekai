-- util.lua — KaliScan HTML parsing helpers
-- Sibling module required by main.lua via require("util")

local util = {}

-- ─── Search result parsing ─────────────────────────────────────────────────
-- Parses one result set, from either source (both reuse the same anchor shape):
--   * /service/backend/search/?q=… fragment — novel__item cards, cover in
--     a plain <img src="COVER">
--   * /genres/<slug> archive pages — book-item cards with a lazy placeholder
--     <img src='…x.gif' data-src="COVER">
--
-- Structure per result (cover anchor):
--   <a title="TITLE" href="/manga/ID-SLUG"> <img … src-or-data-src="COVER" …>

function util.parse_search(html)
	local results = {}

	-- Match src= after the img tag: the lazy card's first double-quoted src=
	-- IS its data-src (the placeholder src= uses single quotes), and the
	-- fragment card's src= carries the real URL directly.
	local seen = {}
	for title, slug, cover in
		host.regex.gmatch(
			html,
			[[(?s)<a\s+title="([^"]*?)"[^>]*href="/manga/([^"]*?)"[^>]*>\s*<img[^>]*?src="([^"]*?)"]]
		)
	do
		if not seen[slug] then
			seen[slug] = true
			table.insert(results, {
				id = slug,
				title = title,
				cover_url = cover,
			})
		end
	end

	return {
		results = results,
	}
end

-- ─── Manga detail parsing ─────────────────────────────────────────────────
-- Parses /manga/SLUG HTML page.
--
-- Sections:
--   <h1>TITLE</h1>                                    in <div class="name box">
--   Authors :</strong> <a ...><span>AUTHOR</span>     in meta box
--   Status :</strong> <a ...><span>STATUS</span>
--   Cover: <div class="cover"> ... <img data-src="URL">
--   Description: <p class="content" ...>TEXT</p>      in summary tab panel

function util.parse_manga_detail(html, manga_id)
	local detail = { id = manga_id }

	-- Title: <h1>TITLE</h1>
	detail.title = host.regex.find(html, [[<h1>([^<]+)</h1>]]) or ""

	-- Author: Authors :</strong> <a ...><span>AUTHOR</span>
	detail.author = host.regex.find(html, [[Authors[^<]*</strong>\s*\n?\s*<a[^>]*>\s*<span>([^<]+)</span>]]) or ""

	-- Status
	detail.status = host.text.normalize_status(
		host.regex.find(html, [[Status[^<]*</strong>\s*\n?\s*<a[^>]*>\s*<span>([^<]+)</span>]]) or ""
	)

	-- Cover: the detail poster lives in <div class="img-cover"> (class value
	-- ENDS with "cover", so anchor the pattern on any class containing it)
	detail.cover_url = host.regex.find(html, [[(?s)class="[^"]*cover[^"]*"[^>]*>.*?<img[^>]*data-src="([^"]*?)"]]) or ""

	-- Description: <p class="content" ...>TEXT</p>
	local desc = host.text.strip_html(host.regex.find(html, [[(?s)<p class="content"[^>]*>(.*?)</p>]]) or "")
	detail.description = desc

	-- Genres
	local genres = {}
	for _, g in host.regex.gmatch(html, [[href="/genres/([^/"]+)/"[^>]*>\s*([^<]+)\s*]]) do
		genres[#genres + 1] = host.regex.replace(g, [[[\s,]+$]], "")
	end
	detail.genres = genres

	return detail
end

-- ─── Chapter list parsing ──────────────────────────────────────────────────
-- Parses /manga/SLUG HTML for the full chapter list.
--
-- Each chapter item:
--   <li id="c-NUMBER">
--     <a href="/manga/SLUG/chapter-NUMBER" title="...">
--       <div>
--         <strong class="chapter-title">TITLE</strong>
--         <time class="chapter-update">UPLOAD_TIME</time>

function util.parse_chapter_list(html, manga_id)
	local chapters = {}

	for number, href, title, upload_time in
		host.regex.gmatch(
			html,
			'id="c-([^"]*?)"[^>]*>\\s*'
				.. '<a[^>]*href="([^"]*?)"[^>]*title="([^"]*?)"[^>]*>\\s*'
				.. '<div>\\s*<strong class="chapter-title">([^<]*?)</strong>\\s*'
				.. "<time[^>]*>([^<]*)</time>"
		)
	do
		-- chapter_id: path after /manga/ with "/" swapped for ":" (e.g.
		-- "SLUG:chapter-98") — host routing treats the ID as one opaque
		-- segment, so a raw slash would 404 the reader URL.
		local chapter_id = host.regex.find(href, [[(?s)/manga/(.+)]])
		if not chapter_id then
			chapter_id = manga_id .. "/chapter-" .. number
		end
		chapter_id = chapter_id:gsub("/", ":")

		local entry = {
			id = chapter_id,
			manga_id = manga_id,
			chapter_num = tonumber(number) or 0,
			title = title,
			url = BASE .. href,
		}
		-- The site prints a relative time ("2 days ago"); host.text knows those
		-- as well as absolute dates. A row that still does not parse gets no
		-- released_at key at all, since an empty one fails the ABI decode.
		local iso = host.text.date_to_iso(upload_time)
		if iso ~= "" then
			entry.released_at = iso
		end
		chapters[#chapters + 1] = entry
	end

	-- ABI convention: newest-first (chapters[1] = newest) — the site HTML is
	-- already newest-first, so no reversal is applied.

	return chapters
end

-- ─── Page list parsing ─────────────────────────────────────────────────────
-- Parses the chapterServer HTML response.
-- Each page image: <div ... data-src="URL" ...>
--
-- TODO: verify against live site — the data-src extraction needs
-- confirmation that chapterServer returns this exact shape.

function util.parse_page_list(html, referer)
	local pages = {}
	local n = 0
	for url in host.regex.gmatch(html, [[data-src="([^"]*?)"]]) do
		-- Skip static assets / loading placeholders
		if not host.regex.match(url, [[\.svg$]]) and not host.regex.match(url, [[/static/]]) then
			n = n + 1
			pages[n] = {
				index = n - 1,
				url = url,
				headers = { ["Referer"] = referer }, -- image CDN needs the chapter page as Referer
			}
		end
	end
	return pages
end

return util
