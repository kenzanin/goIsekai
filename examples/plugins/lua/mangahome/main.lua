-- main.lua — MangaHome (https://www.mangahome.com) plugin for goIsekai
-- MangaHome is a MangaHere mirror (same CDN: fmcdn.mangahere.com / zjcdn).
-- Pure SSR; chapter pages carry direct <img> tags (no chapterfun JS).
-- Chapter dates are "Apr 05,2024" — parsed in-plugin before sending to host.

PLUGIN = {
	contract_version = 1,
	name = "MangaHome",
	site_url = "https://www.mangahome.com",
	logo = "logo.png",
	thumb_ratio = 0.703,
	timeout = 60,
}

local BASE = "https://www.mangahome.com"

local function trim(s)
	return (string.gsub(s, "^%s+", ""):gsub("%s+$", ""))
end

local function html_decode(s)
	if not s then return "" end
	local ents = { ["&amp;"] = "&", ["&lt;"] = "<", ["&gt;"] = ">",
		["&quot;"] = '"', ["&#39;"] = "'", ["&#x27;"] = "'", ["&nbsp;"] = " " }
	for ent, ch in pairs(ents) do
		s = string.gsub(s, ent, ch)
	end
	return s
end

-- "Apr 05,2024" / "Apr 5, 2024" -> RFC3339 midnight UTC. The host unmarshals
-- released_at as time.Time (date-only strings are rejected silently and drop
-- the chapter list), and os.date is unavailable in the plugin runtime, so the
-- month lookup and formatting are done here.
local MONTHS = { Jan=1, Feb=2, Mar=3, Apr=4, May=5, Jun=6,
	Jul=7, Aug=8, Sep=9, Oct=10, Nov=11, Dec=12 }
local function site_date_to_iso(s)
	if not s or s == "" then return nil end
	local mon, day, year = string.match(s, "(%a%a%a)%s+(%d+)%s*,%s*(%d%d%d%d)")
	if not mon or not MONTHS[mon] then return nil end
	return string.format("%04d-%02d-%02dT00:00:00Z", tonumber(year), MONTHS[mon], tonumber(day))
end

-- ─── ABI: search ───────────────────────────────────────────────────────────
-- SSR cards: <a title="<title>" href="/manga/<slug>">...<img src="<cover>">.
-- Pagination: ?page=N (12 cards/page, ~2015 results for "dungeon").
-- Genre filter: /<genre-slug>?name=<q> (e.g. /action?name=dungeon) — the
-- directory's own browse path; slugs from get_genres.
function search(arg)
	local a = host.json.decode(arg)
	if type(a) == "string" then a = { query = a, page = 1 } end
	local query = (a and a.query) or ""
	local page = tonumber(a.page) or 1
	local genres = a and (a.genres or {})
	local url
	if #genres > 0 then
		-- Genre browse: first genre wins. Empty query is valid — the genre
		-- page lists everything in the genre.
		url = BASE .. "/" .. tostring(genres[1])
		if query ~= "" then
			url = url .. "?name=" .. host.text.url_encode(query)
		end
		if page > 1 then
			url = url .. (query ~= "" and "&" or "?") .. "page=" .. tostring(page)
		end
	else
		if query == "" then
			return host.json.encode({})
		end
		url = BASE .. "/search?name=" .. host.text.url_encode(query)
		if page > 1 then
			url = url .. "&page=" .. tostring(page)
		end
	end
	local body = host.http.get_body(url)
	if not body or body == "" then
		log.error("search fetch failed for: " .. url)
		return nil, "upstream_unavailable"
	end
	-- Cards: title anchor then the cover img; scan each /manga/ link and take
	-- the first following <img src="//...">.
	local out, seen = {}, {}
	local pos = string.find(body, 'href="/manga/', 1, true)
	while pos do
		local slug = string.match(body, 'href="/manga/([%w_]+)"', pos)
		if slug and not seen[slug] then
			-- the card's title attr sits BEFORE the href ("<a class=...
			-- title=... href=...>"), so look back; the cover img follows.
			local back = string.sub(body, math.max(1, pos - 300), pos)
			local fwd = string.sub(body, pos, pos + 600)
			local title = html_decode(trim(string.match(back, 'title="([^"]*)"') or ""))
			-- Lua patterns have no (?:...) groups; capture any quoted URL
			-- ending in .jpg and require the host prefix on the result.
			local cover = string.match(fwd, 'src="([^"]+%.jpg[^"]*)"')
			if title ~= "" and cover then
				seen[slug] = true
				if string.sub(cover, 1, 2) == "//" then
					cover = "https:" .. cover
				end
				out[#out + 1] = { id = slug, title = title, cover_url = cover }
			end
		end
		pos = string.find(body, 'href="/manga/', pos + 12, true)
	end
	log.info("search found " .. tostring(#out) .. " for " .. query)
	return host.json.encode(out)
end

-- ─── ABI: get_manga_detail ────────────────────────────────────────────────
-- <title> is "<name>Manga - Read ... For Free!"; genres are /directory links.
function get_manga_detail(arg)
	local id = host.json.decode(arg)
	if type(id) == "table" then id = id.id end
	if not id or id == "" then
		return host.json.encode({})
	end
	local body = host.http.get_body(BASE .. "/manga/" .. id)
	if not body or body == "" then
		log.error("detail fetch failed for: " .. id)
		return nil, "upstream_unavailable"
	end
	local raw = string.match(body, "<title[^>]*>([^<]*)</title>") or id
	-- "NameManga - Read Name Manga Online For Free!" -> strip suffixes
	raw = string.gsub(raw, "Manga%s*-%s*Read.*$", "")
	raw = string.gsub(raw, "Manga%s*$", "")
	local title = html_decode(trim(raw))
	local cover = string.match(body, '<img[^>]*class="[^"]*manga[^"]*"[^>]*src="((?:https?:)?//[^"]+)"')
		or string.match(body, 'property="og:image" content="([^"]+)"') or ""
	if string.sub(cover, 1, 2) == "//" then cover = "https:" .. cover end
	-- genres: chips linking into /directory
	local genres = {}
	for g in string.gmatch(body, 'href="/directory/[^"]*"[^>]*>([^<]+)</a>') do
		genres[#genres + 1] = trim(g)
	end
	local status = string.match(body, ">(Ongoing)<") and "ongoing"
		or (string.match(body, ">(Completed)<") and "completed" or "")
	local desc = html_decode(trim(string.match(body, '<div[^>]*class="[^"]*content[^"]*"[^>]*>(.-)</div>')
		or string.match(body, 'name="description" content="([^"]*)"') or ""))
	return host.json.encode({
		id = id,
		title = title,
		cover_url = cover,
		genres = genres,
		status = status,
		description = desc,
		is_library = true,
	})
end

-- ─── ABI: get_chapter_list ────────────────────────────────────────────────
-- Rows: <a href="/manga/<slug>/cNNN" name="NN.N"><span>label</span></a>
--       ... <span class="time">Apr 05,2024</span>  (date in a sibling cell)
function get_chapter_list(arg)
	local id = host.json.decode(arg)
	if type(id) == "table" then id = id.id end
	if not id or id == "" then
		return host.json.encode({})
	end
	local body = host.http.get_body(BASE .. "/manga/" .. id)
	if not body or body == "" then
		log.error("chapter fetch failed for: " .. id)
		return nil, "upstream_unavailable"
	end
	local out = {}
	local pos = string.find(body, '/c%d+%"', 1, false)
	while pos do
		local href = string.match(body, 'href="(/manga/' .. id .. '/c[%d.]+)"', pos - 200)
			or string.match(string.sub(body, math.max(1, pos - 200), pos + 60), 'href="(/manga/[%w_]+/c[%d.]+)"')
		local num = href and string.match(href, "/c([%d.]+)$")
		if href and num then
			-- read the row up to the next chapter link; date sits in a sibling
			local nxt = string.find(body, '/c%d+%"', pos + 6, false) or (pos + 1500)
			local row = string.sub(body, pos, nxt)
			local label = string.match(row, '<span class="pc%-none">Ch%.? ?([^<]*)</span>')
				or string.match(row, 'name="[^"]*">.-<span[^>]*>([^<]*)</span>')
			local date_s = string.match(row, '<span class="time">([^<]*)</span>')
			out[#out + 1] = {
				id = id .. ":" .. href:match("/c([%d.]+)$"),
				title = "Chapter " .. (label and trim(label) or num),
				chapter_num = tonumber(num) or 0,
				released_at = site_date_to_iso(date_s),
			}
			pos = nxt
		else
			pos = string.find(body, '/c%d+%"', pos + 4, false)
		end
	end
	if #out == 0 then
		log.error("no chapter rows found for " .. id)
		return nil, "no_chapters"
	end
	log.info("found " .. tostring(#out) .. " chapters for " .. id)
	return host.json.encode(out)
end

-- ─── ABI: get_page_list ───────────────────────────────────────────────────
-- Reader page: direct <img src="//zjcdn.mangahere.org/store/manga/...jpg">.
-- chapterID arrives as "<slug>:<num>" (see get_chapter_list).
function get_page_list(arg)
	local composite = host.json.decode(arg)
	if type(composite) ~= "string" or composite == "" then
		return host.json.encode({})
	end
	local slug, num = string.match(composite, "^(.-):c?([%d.]+)$")
	if not slug or not num then
		return host.json.encode({})
	end
	local url = BASE .. "/manga/" .. slug .. "/c" .. num
	local body = host.http.get_body(url)
	if not body or body == "" then
		log.error("page fetch failed for: " .. composite)
		return nil, "upstream_unavailable"
	end
	local out = {}
	-- Lua patterns have no (?:...) groups — capture any quoted .jpg URL and
	-- filter to the reader CDN below.
	for raw in string.gmatch(body, 'src="([^"]+%.jpg[^"]*)"') do
		local u = raw
		if string.sub(u, 1, 2) == "//" then u = "https:" .. u end
		-- skip avatars: reader pages live under /store/manga/
		if string.find(u, "/store/manga/", 1, true) then
			out[#out + 1] = { index = #out, url = u }
		end
	end
	if #out == 0 then
		log.error("no reader images found for " .. composite)
		return nil, "no_pages"
	end
	log.info("found " .. tostring(#out) .. " pages for " .. composite)
	return host.json.encode(out)
end

-- ─── ABI: get_genres ──────────────────────────────────────────────────────
-- Directory genre chips: <a href="/<slug>" title="<Name>">Name</a> under the
-- "Genres:" filter. Slugs double as browse paths (/action?name=<q>).
function get_genres()
	return host.json.encode({
		{ name = "Action",        slug = "action" },
		{ name = "Adventure",     slug = "adventure" },
		{ name = "Comedy",        slug = "comedy" },
		{ name = "Doujinshi",     slug = "doujinshi" },
		{ name = "Drama",         slug = "drama" },
		{ name = "Ecchi",         slug = "ecchi" },
		{ name = "Fantasy",       slug = "fantasy" },
		{ name = "Harem",         slug = "harem" },
		{ name = "Historical",    slug = "historical" },
		{ name = "Horror",        slug = "horror" },
		{ name = "Josei",         slug = "josei" },
		{ name = "Mature",        slug = "mature" },
		{ name = "Mecha",         slug = "mecha" },
		{ name = "Mystery",       slug = "mystery" },
		{ name = "Psychological", slug = "psychological" },
		{ name = "Romance",       slug = "romance" },
		{ name = "Sci-fi",        slug = "sci-fi" },
		{ name = "Seinen",        slug = "seinen" },
		{ name = "Shoujo",        slug = "shoujo" },
		{ name = "Shounen",       slug = "shounen" },
		{ name = "Sports",        slug = "sports" },
		{ name = "Supernatural",  slug = "supernatural" },
		{ name = "Tragedy",       slug = "tragedy" },
		{ name = "Yaoi",          slug = "yaoi" },
		{ name = "Yuri",          slug = "yuri" },
		{ name = "Manga",         slug = "manga" },
		{ name = "Manhwa",        slug = "manhwa" },
		{ name = "Manhua",        slug = "manhua" },
	})
end

return {}
