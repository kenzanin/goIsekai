-- util.lua — MangaHere (www.mangahere.cc) HTML parsing helpers
-- Sibling module required by main.lua via require("util")
local util = {}

-- Referer required by chapterfun and every reader image (zjcdn.mangahere.org).
function util.image_headers()
	return { Referer = "https://www.mangahere.cc/" }
end

-- ─── Search result parsing ─────────────────────────────────────────────────
-- GET https://www.mangahere.cc/search?title={q}&page={N} — server-rendered
-- result rows:
--   <a href="/manga/{slug}/" title="T"><img class="manga-list-4-cover" src="COVER"
--   <p class="manga-list-4-item-title"><a href="/manga/{slug}/" title="T">T</a>

function util.parse_search(html)
	local results = {}
	local seen = {}

	local function add(slug, title, cover)
		if not slug or slug == "" then
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
		host.regex.gmatch(
			html,
			[[<a href="(/manga/[^"/]+/)" title="([^"]+)"><img class="manga-list-4-cover" src="([^"]+)"]]
		)
	do
		add(host.regex.find(href, [[^/manga/([^/]+)/]]), title, cover)
	end

	for href, title in
		host.regex.gmatch(html, [[<p class="manga-list-4-item-title"><a href="(/manga/[^"/]+/)" title="([^"]+)"]])
	do
		add(host.regex.find(href, [[^/manga/([^/]+)/]]), title, "")
	end

	return results
end

-- ─── Manga detail parsing ──────────────────────────────────────────────────
-- GET https://www.mangahere.cc/manga/{slug}/:
--   <span class="detail-info-right-title-font">TITLE</span>
--   <span class="detail-info-right-title-tip">Ongoing</span>
--   <p class="detail-info-right-say">Author: <a ...>NAME</a></p>
--   <p class="detail-info-right-tag-list"><a ...>Genre</a>...</p>
--   <p style="display:none" class="fullcontent">FULL SUMMARY</p>
--   <img class="detail-info-cover-img" src="COVER"

function util.parse_manga_detail(html, manga_id)
	local detail = { id = manga_id }

	detail.title =
		host.text.trim(host.text.unescape(host.regex.find(html, [[detail-info-right-title-font">([^<]+)<]]) or ""))
	detail.cover_url = host.text.unescape(host.regex.find(html, [[detail-info-cover-img" src="([^"]+)"]]) or "")
	detail.author =
		host.text.strip_html(host.regex.find(html, [[detail-info-right-say">Author:\s*<a[^>]*>(.*?)</a>]]) or "")
	detail.status = host.text.normalize_status(host.regex.find(html, [[detail-info-right-title-tip">([^<]+)<]]) or "")
	detail.description = host.text.strip_html(host.regex.find(html, [[(?s)class="fullcontent">(.*?)</p>]]) or "")

	local genres = {}
	local genre_block = host.regex.find(html, [[(?s)detail-info-right-tag-list">(.*?)</p>]]) or ""
	for g in host.regex.gmatch(genre_block, [[title="[^"]*">(.*?)</a>]]) do
		local name = host.text.strip_html(g)
		if name ~= "" then
			genres[#genres + 1] = name
		end
	end
	detail.genres = genres

	return detail
end

-- ─── Chapter list parsing ──────────────────────────────────────────────────
-- Same detail page. Links: href="/manga/{slug}/c080/1.html" with title
-- "Kumo desu ga, nani ka? Ch.080" and a date in <p class="title2">Nov 18,2016 </p>.
-- The number is rebuilt from the path (c([0-9.]+)) — padded/decimal safe.

function util.parse_chapter_list(html, manga_id)
	local chapters = {}
	local seen = {}
	local pattern = [[href="/manga/]] .. manga_id .. [[/(c[0-9.]+)/1\.html" title="[^"]*"]]

	for rest in host.regex.gmatch(html, pattern) do
		local num = tonumber(host.regex.find(rest, [[c([0-9][0-9.]*)$]]))
		if num ~= nil and not seen[rest] then
			seen[rest] = true
			chapters[#chapters + 1] = {
				id = manga_id .. "/" .. rest,
				manga_id = manga_id,
				chapter_num = num,
				title = string.format("Chapter %g", num),
				url = "https://www.mangahere.cc/manga/" .. manga_id .. "/" .. rest .. "/1.html",
			}
		end
	end

	-- Newest-first ABI order regardless of DOM order.
	table.sort(chapters, function(a, b)
		return a.chapter_num > b.chapter_num
	end)
	return chapters
end

-- ─── Dean-Edwards unpacker for the chapterfun p,a,c,k,e,d payload ──────────
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

-- ─── Page list via chapterfun (2 images per request) ───────────────────────
-- Entry: DESKTOP reader page /manga/{chapter_id}/1.html carries
-- var chapterid={cid} and var imagecount={total}. chapterfun.ashx
-- accepts an EMPTY key (unlike the browser, which derives dm5_key from a
-- packed eval) — verified live. Each call returns a packed JS with pix
-- (folder) + 2 filenames; replies overlap ([K,K+1]); dedupe by filename.
-- The packed pix truncates the folder decimal ("007." serves "007.5") and
-- KEEPS the "/compressed" tail (unlike mangatown, whose pix ends at the
-- folder), so the suffix is inserted INTO the folder, not appended.

local function chapter_num_suffix(chapter_id)
	local num = host.regex.find(chapter_id, [[c([0-9][0-9.]*)$]]) or ""
	if host.regex.find(num, [[\.([0-9]+)$]]) ~= nil then
		return host.regex.find(num, [[([0-9]+)$]])
	end
	return "0"
end

function util.get_page_urls(get_body, chapter_id)
	local base = "https://www.mangahere.cc/manga/" .. chapter_id .. "/"
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
			"https://www.mangahere.cc/manga/"
				.. chapter_id
				.. "/chapterfun.ashx?cid="
				.. cid
				.. "&page="
				.. tostring(k)
				.. "&key=",
			{ Referer = base .. tostring(math.ceil(k / 2)) .. ".html" }
		)
		if packed == "" or packed == nil then
			break
		end
		local js = unpack_packer(packed)
		if js == nil then
			break
		end
		local pix = (host.regex.find(js, [[pix="([^"]+)"]]) or ""):gsub("([0-9])%./", "%1." .. suffix .. "/", 1)
		for fname in host.regex.gmatch(js, [["(/[^"]+\.jpg)"]]) do
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
