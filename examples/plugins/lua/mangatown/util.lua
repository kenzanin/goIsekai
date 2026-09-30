-- util.lua — MangaTown (m.mangatown.com) HTML parsing helpers
-- Sibling module required by main.lua via require("util")
local util = {}

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

return util
