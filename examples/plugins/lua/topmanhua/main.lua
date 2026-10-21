-- TopManhua plugin for goIsekai (Lua / Lunar VM)
-- Site: https://www.topmanhua.fan (Madara-family theme, Cloudflare-fronted)
-- ABI contract version: 1
--
-- Grounded against live responses (2026-10-11):
--   search   GET /manga/searching?action=wp-manga-search-manga&title=Q
--            -> {"success":true,"data":[{comic_id,slug,name,image_original,
--               author,status,kind,detail,...}]}; max 6 hits and the page param
--            is ignored (the host paginates whatever comes back).
--   detail   /manhua/{slug} -> inline `var comicId = NNN;` feeds the chapter
--            API; no author block is rendered anywhere on the page.
--   chapters GET /Comic/Services/ComicService.asmx/ProcessChapterList?comicId=N
--            -> {"success":true,"chapters":[{chapterId,name,url}]} where url
--            lacks the /manhua prefix and the slugs are inconsistent
--            (chapter-9-2 vs chapter-10.1) — always take the API's last path
--            segment, never rebuild it from the chapter name.
--   reader   /manhua/{slug}/{chapter-slug} -> <img class='wp-manga-chapter-img'>
--            on cdn[23].zinmanga1.com; both the page CDNs and the cover thumbs
--            403 any Referer that is not the site root WITH trailing slash.

PLUGIN = {
	contract_version = 1,
	name = "TopManhua",
	-- Trailing slash is load-bearing: image CDNs reject "https://www.topmanhua.fan"
	-- (403) but accept "https://www.topmanhua.fan/" (200), and the host's cover
	-- fetcher offers this string as the fallback Referer.
	site_url = "https://www.topmanhua.fan/",
	logo = "logo.png",
	verify_url = "https://www.topmanhua.fan",
	needs_human_verify = false,
	thumb_ratio = 0.694, -- covers render at 193x278
}

BASE = host.plugin.param("base")

-- Covers: the search JSON carries only a relative path (thumb/<slug>.webp) plus
-- a base_cdn number (4->img-r1, 6->img-r2), but img-r1 served every tested
-- thumb across both base_cdn values while img-r2 503'd the base_cdn=4 ones —
-- so one cover host for all.
CDN_COVER = host.plugin.param("cdn")

local trim = host.text.trim
local unescape = host.text.unescape
local find = host.regex.find
local gmatch = host.regex.gmatch
local match = host.regex.match

-- Search is a JSON endpoint; replicate the browser's XHR headers. The
-- X-CSRF-Token the browser attaches (from <meta name="csrf-token">) is NOT
-- enforced — a live probe without it returned the identical payload — so no
-- meta-tag round trip is needed here.
local function search_json(query)
	return host.http.get_body(
		BASE .. "/manga/searching?action=wp-manga-search-manga&title=" .. host.text.url_encode(query),
		{
			["X-Requested-With"] = "XMLHttpRequest",
			["Referer"] = BASE .. "/manhua",
		}
	)
end

local function cover_from(r)
	if not r.image_original or r.image_original == "" then
		return ""
	end
	return CDN_COVER .. r.image_original
end

-- The detail page renders no author block at all; the search JSON carries the
-- author per slug, so one slug-matched lookup (the host caches detail 24h)
-- recovers it. Any failure degrades to "" which the host then fills from
-- enrichment data when available.
local function lookup_author(slug, title)
	if title == "" then
		return ""
	end
	local body = search_json(title)
	if not body or body == "" then
		return ""
	end
	local decoded = host.json.decode(body)
	for _, r in ipairs(decoded and decoded.data or {}) do
		if r.slug == slug then
			return unescape(r.author or "")
		end
	end
	return ""
end

-- Genre archives are server-rendered with page-listing-item cards.
local function parse_genre_cards(body)
	local results = {}
	local seen = {}
	for card in gmatch(body, [[(?s)<div[^>]*class="[^"]*page-listing-item[^"]*".*?post-title.*?</h[23]>]]) do
		local slug = find(card, [[href="[^"]*/manhua/([a-z0-9-]+)"']])
		if slug and slug ~= "" and not seen[slug] then
			seen[slug] = true
			local title = find(card, [[(?s)<h[23][^>]*>\s*<a[^>]*>(.*?)</a>]])
			local cover = find(card, [[data-src="[\t\n\s]*(https?://[^"]+)"]])
				or find(card, [[src="[\t\n\s]*(https?://[^"]+)"]])
			results[#results + 1] = {
				id = slug,
				title = trim(unescape(host.text.strip_html(title or ""))),
				cover_url = cover and trim(cover) or "",
			}
		end
	end
	return results
end

-- ─── ABI: search_manga ──────────────────────────────────────────────────────

function search_manga(arg)
	local args = host.json.decode(arg)
	local query = trim(args.query or "")
	local page = tonumber(args.page) or 1
	local genres = args.genres or {}

	-- The host submits the genre SLUG (the search form's option value is
	-- g.Slug), so it can go straight into the archive path.
	if #genres > 0 then
		local gslug = host.text.url_encode(genres[1])
		local gurl = BASE .. "/manhua-genre/" .. gslug .. "/"
		if page > 1 then
			-- a bare /page/N/ 302s to an http:// URL that then 502s; the
			-- orderby param keeps the redirect away
			gurl = BASE .. "/manhua-genre/" .. gslug .. "/page/" .. tostring(page) .. "/?orderby=latest"
		end
		local body = host.http.get_body(gurl)
		if not body or body == "" then
			log.error("topmanhua genre archive unreachable: " .. gurl)
			return nil, "upstream_unavailable"
		end
		local genre_results = parse_genre_cards(body)
		log.debug("topmanhua genre " .. genres[1] .. " page=" .. page .. " found=" .. tostring(#genre_results))
		return host.json.encode(genre_results)
	end

	local body = search_json(query)
	if not body or body == "" then
		log.error("topmanhua search unreachable: q=" .. query)
		return nil, "upstream_unavailable"
	end
	local decoded = host.json.decode(body)

	local results = {}
	for _, r in ipairs(decoded and decoded.data or {}) do
		local slug = r.slug
		if slug and slug ~= "" then
			results[#results + 1] = {
				id = slug,
				title = unescape(r.name or ""),
				cover_url = cover_from(r),
				author = unescape(r.author or ""),
			}
		end
	end

	log.debug("topmanhua search q=" .. query .. " page=" .. page .. " found=" .. tostring(#results))
	return host.json.encode(results)
end

-- ─── ABI: get_manga_detail ──────────────────────────────────────────────────

function get_manga_detail(arg)
	local slug = trim(host.json.decode(arg or '""'))
	if slug == "" then return host.json.encode({ id = "" }) end

	local body = host.http.get_body(BASE .. "/manhua/" .. host.text.url_encode(slug))
	if not body or body == "" then
		-- empty table encodes as []; detail must stay an OBJECT — emit {id} only
		return host.json.encode({ id = slug })
	end

	local detail = {
		id = slug,
		title = "",
		author = "",
		description = "",
		cover_url = "",
		genres = {},
		status = "",
	}

	detail.title = unescape(trim(find(body, [[(?s)<div class="post-title">\s*<h1>(.*?)</h1>]]) or ""))

	local img = find(body, [[(?s)<div class="summary_image".*?</a>]]) or ""
	detail.cover_url = trim(
		find(img, [[data-src="[\t\n\s]*(https?://[^"]+)"]])
			or find(img, [[src="[\t\n\s]*(https?://[^"]+)"]])
			or ""
	)

	local gblock = find(body, [[(?s)class="genres-content">(.*?)</div>]]) or ""
	for g in gmatch(gblock, [[<a[^>]*>([^<]+)</a>]]) do
		local name = unescape(trim(g))
		if name ~= "" then detail.genres[#detail.genres + 1] = name end
	end

	local s = find(body, [[(?s)Status\s*</h5>.*?class="summary-content">\s*([^<]+)]])
	if s then detail.status = host.text.normalize_status(trim(s)) end

	-- .summary__content paragraphs; the first <p> just repeats the title here.
	-- trim runs after unescape so a bare &nbsp; paragraph collapses to "".
	local dblock = find(body, [[(?s)class="summary__content[^"]*">(.*?)</div>]]) or ""
	local paras = {}
	for p in gmatch(dblock, [[(?s)<p[^>]*>(.*?)</p>]]) do
		local t = trim(unescape(host.regex.replace(host.text.strip_html(p), [[\s+]], " ")))
		if t ~= "" and t ~= detail.title then paras[#paras + 1] = t end
	end
	detail.description = table.concat(paras, "\n")

	detail.author = lookup_author(slug, detail.title)

	return host.json.encode(detail)
end

-- ─── ABI: get_chapter_list ──────────────────────────────────────────────────
-- Chapter rows come from the ComicService JSON API; comicId lives only in the
-- detail page's inline script.

function get_chapter_list(arg)
	local slug = trim(host.json.decode(arg or '""'))
	if slug == "" then return host.json.encode({}) end

	local body = host.http.get_body(BASE .. "/manhua/" .. host.text.url_encode(slug))
	if not body or body == "" then
		return nil, "upstream_unavailable"
	end
	-- RE2 syntax (host coregex engine), not Lua patterns: \s / \d, never %s/%d
	local comic_id = find(body, [[var comicId\s*=\s*(\d+)]]) or find(body, [[id="comic_id"\s+value="(\d+)"]])
	if not comic_id then
		log.error("topmanhua: comicId missing on detail page for " .. slug)
		return nil, "envelope_unrecognised"
	end

	local api = host.http.get_body(
		BASE .. "/Comic/Services/ComicService.asmx/ProcessChapterList?comicId=" .. comic_id,
		{ ["Referer"] = BASE .. "/manhua/" .. slug }
	)
	if not api or api == "" then
		return nil, "upstream_unavailable"
	end
	local decoded = host.json.decode(api)

	local chapters = {}
	local seen = {}
	for _, c in ipairs(decoded and decoded.chapters or {}) do
		local seg = c.url and find(c.url, [[/([^/]+)/?$]])
		if seg and seg ~= "" and not seen[seg] then
			seen[seg] = true
			chapters[#chapters + 1] = {
				id = slug .. ":" .. seg,
				manga_id = slug,
				chapter_num = host.text.chapter_num(c.name or ""),
				title = c.name or "",
				url = BASE .. "/manhua/" .. host.text.url_encode(slug) .. "/" .. host.text.url_encode(seg),
			}
		end
	end

	log.debug("topmanhua chapters slug=" .. slug .. " count=" .. tostring(#chapters))
	return host.json.encode(chapters)
end

-- ─── ABI: get_page_list ─────────────────────────────────────────────────────

function get_page_list(arg)
	local path = trim(host.json.decode(arg or '""'))
	local slug, cslug = path:match("^([^:]+):(.+)$")
	if not slug or not cslug then return host.json.encode({}) end

	local body = host.http.get_body(
		BASE .. "/manhua/" .. host.text.url_encode(slug) .. "/" .. host.text.url_encode(cslug)
	)
	if not body or body == "" then
		return host.json.encode({})
	end

	local pages = {}
	local pos = 1
	while true do
		local s = host.regex.find_index(body, "<img", pos)
		if not s then break end
		local e = host.regex.find_index(body, ">", s)
		if not e then break end
		local tag = body:sub(s, e)
		if match(tag, "wp-manga-chapter-img") then
			-- attributes are single-quoted on this site; src == data-src and
			-- data-backup points at the second CDN. The Referer header is
			-- required: cdn3.zinmanga1.com answers 403/4547 bytes without one
			-- and 200/117738 with the site root. Level-1 long brackets below:
			-- each pattern ends in a "]" that would close a [[ ]] literal.
			local src = find(tag, [=[src=['"][\t\n\s]*(https?://[^'"]+)['"]]=])
				or find(tag, [=[data-src=['"][\t\n\s]*(https?://[^'"]+)['"]]=])
				or find(tag, [=[data-backup=['"][\t\n\s]*(https?://[^'"]+)['"]]=])
			if src then
				pages[#pages + 1] = {
					index = #pages,
					url = trim(src),
					headers = { Referer = BASE .. "/" },
				}
			end
		end
		pos = e + 1
	end

	log.debug("topmanhua pages " .. slug .. ":" .. cslug .. " count=" .. tostring(#pages))
	if #pages == 0 then
		-- the reader page loaded but carries no reader images: broken chapter
		return nil, "no_pages"
	end
	return host.json.encode(pages)
end

-- ─── get_genres (optional export) ───────────────────────────────────────────
-- Scraped from the /manhua nav so the list cannot drift from the theme.

function get_genres()
	local body = host.http.get_body(BASE .. "/manhua")
	local genres = {}
	if not body or body == "" then return host.json.encode(genres) end

	local seen = {}
	for slug, name in gmatch(body, [[<a href="/manhua-genre/([a-z0-9_-]+)"[^>]*>([^<]+)</a>]]) do
		local label = unescape(trim(name))
		if slug ~= "" and label ~= "" and not seen[slug] then
			seen[slug] = true
			genres[#genres + 1] = { name = label, slug = slug }
		end
	end

	log.debug("topmanhua genres found=" .. tostring(#genres))
	return host.json.encode(genres)
end
