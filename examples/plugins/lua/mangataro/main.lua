-- mangataro.org plugin for goIsekai
--
-- WordPress (mangapeak theme). Search/detail server-rendered; the detail page
-- lazy-loads its full chapter list into a .chapter-list div via admin-ajax,
-- and the reader page lazy-loads its images via IntersectionObserver. Neither
-- the chapter index nor the page images appear in the SSR markup, so both the
-- chapter list and the page list go through host.browser.evaluate (a scroll
-- poll-harvest; rod awaits the returned promise).
--
-- Search   GET /?s={query}       -> /manga/{slug} cards  (static)
-- Detail   GET /manga/{slug}     -> h1, og:image, genres, status  (static)
-- Chapters evaluate(detail)     -> poll .chapter-list, scroll, harvest a[href*="/read/"]
-- Pages    evaluate(reader)     -> poll img[src]/data-src, scroll, harvest .webp chapter URLs
-- No Referer special-case: mangataro.yachts/storage serves 200 with or without Referer.

PLUGIN = {
  contract_version = 1,
  name = "Mangataro",
  site_url = "https://mangataro.org",
  verify_url = "https://mangataro.org",
  needs_human_verify = false,
  thumb_ratio = 0.667,
  logo = "logo.png",
}

BASE = host.plugin.param("site_url")
UA = "Mozilla/5.0 (compatible; goIsekai-plugin/1.0)"

local function abs(url)
  if not url or url == "" then return "" end
  if url:sub(1, 7) == "http://" or url:sub(1, 8) == "https://" then return url end
  if url:sub(1, 1) == "/" then return BASE .. url end
  -- bare slug from search results (memory #4076: full URLs in IDs 404 the route)
  return BASE .. "/manga/" .. url
end

local function http_get(url)
  return host.http.get_body(url, { ["User-Agent"] = UA, ["Accept"] = "text/html,application/xhtml+xml" })
end

function search_manga(arg)
  local q = host.json.decode(arg)
  local genres = q.genres or {}
  local body
  if #genres > 0 then
    -- Genre browse: WordPress tag archives at /tag/{slug}/ use the same
    -- card template as search results.
    body = http_get(BASE .. "/tag/" .. host.text.url_encode(genres[1]) .. "/")
  else
    body = http_get(BASE .. "/?s=" .. host.text.url_encode(q.query))
  end
  if not body then
    return nil, "fetch_failed"
  end
  local doc = host.html.parse(body)
  -- Each result is an <a href="/manga/{slug}"> wrapping an <img> whose alt is
  -- the title and whose src is the cover. No status/latest in these cards.
  local card_sel = "a[href*='/manga/']"
  local hrefs = host.html.find_list_attr(doc, card_sel, "href")
  local titles = host.html.find_list_attr(doc, card_sel .. " img", "alt")
  local covers = host.html.find_list_attr(doc, card_sel .. " img", "src")
  local out = {}
  for i = 1, #hrefs do
    local href = hrefs[i]
    -- bare slug IDs (memory #4076: full URLs in IDs 404 the /view/manga route)
    local id = href and href:match("/manga/([^/]+)/?%s*$") or nil
    if id then
      out[#out + 1] = {
        id = id,
        title = titles[i] or "",
        cover_url = covers[i] or "",
      }
    end
  end
  return host.json.encode(out)
end

function get_manga_detail(arg)
  local manga_id = host.json.decode(arg)
  local body = http_get(abs(manga_id))
  if not body then
    return nil, "fetch_failed"
  end
  local doc = host.html.parse(body)
  local genres = {}
  -- HAR mangataro_lengkap: genre links live at /tag/ paths (memory #4757)
  for _, name in ipairs(host.html.find_list_text(doc, "a[href*='/tag/']")) do
    genres[#genres + 1] = name
  end
  local status = host.html.find_attr(doc, "[data-status]", "data-status")
  if not status or status == "" then
    status = host.html.find_text(doc, ".comic-status, .status")
  end
  local description = host.html.find_attr(doc, "meta[name='description']", "content")
  if not description or description == "" then
    description = host.html.find_text(doc, ".entry-content, .description-summary")
  end
  return host.json.encode({
    id = manga_id,
    -- HAR: two h1s (site title + manga title); the manga one carries line-clamp-2
    title = host.html.find_text(doc, "h1.line-clamp-2"),
    -- HAR: no meta author; author sits in the div above the "Author & Artist" label
    author = "",
    description = description,
    cover_url = host.html.find_attr(doc, "meta[property='og:image']", "content"),
    genres = genres,
    status = host.text.normalize_status(status),
    first_reader_url = abs(host.html.find_attr(doc, "a[href*='/read/']", "href")),
  })
end

function get_chapter_list(arg)
  local manga_id = host.json.decode(arg)
  -- mangataro loads chapters via an AJAX endpoint:
  --   GET /auth/manga-chapters?manga_id=ID&_t=TOKEN&_ts=TIMESTAMP
  -- The token is generated client-side by generateToken():
  --   md5(timestamp + "mng_ch_" + YYYYMMDDHH)[:16]
  -- where YYYYMMDDHH is the current UTC hour. We reproduce it plugin-side
  -- to avoid a browser round-trip. (os.date() ignores its format arg and
  -- returns local RFC3339, so the UTC hour comes from host.text.date_to_iso.)
  local detail = http_get(abs(manga_id))
  if not detail or detail == "" then
    return nil, "fetch_failed"
  end
  local doc = host.html.parse(detail)
  local numeric_id = host.html.find_attr(doc, "[data-manga-id]", "data-manga-id")
  if not numeric_id or numeric_id == "" then
    return nil, "no_manga_id"
  end
  local ts = os.time()
  -- os.date() ignores its format arg (returns local RFC3339), so derive the
  -- UTC hour from date_to_iso, which parses unix seconds and formats UTC.
  local iso = host.text.date_to_iso(tostring(ts))
  if not iso or iso == "" then
    return nil, "fetch_failed"
  end
  local hour = iso:gsub("%D", ""):sub(1, 10)
  local token = host.crypto.md5_hex(tostring(ts) .. "mng_ch_" .. hour):sub(1, 16)
  local api_url = BASE .. "/auth/manga-chapters?manga_id=" .. numeric_id .. "&_t=" .. token .. "&_ts=" .. tostring(ts)
  local api_body = host.http.get_body(api_url, { ["User-Agent"] = UA, ["Accept"] = "application/json, text/plain, */*", ["Accept-Language"] = "en-US,en;q=0.9", ["Referer"] = abs(manga_id), ["Origin"] = "https://mangataro.org", ["sec-fetch-dest"] = "empty", ["sec-fetch-mode"] = "cors", ["sec-fetch-site"] = "same-origin" })
  if not api_body or api_body == "" then
    return nil, "fetch_failed"
  end
  local data = host.json.decode(api_body)
  if not data or not data.chapters then
    return nil, "no_chapters"
  end
  local out = {}
  for _, ch in ipairs(data.chapters) do
    out[#out + 1] = {
      id = ch.id,
      manga_id = manga_id,
      title = ch.title or "",
      chapter_num = host.text.chapter_num(ch.chapter),
      url = ch.url,
    }
  end
  return host.json.encode(out)
end

function get_page_list(arg)
  -- Chapter id is the plain numeric string stored in the DB (e.g. "9355"),
  -- which is exactly the chapter_id the content API wants. Also accept a
  -- reader URL and pull the trailing id after the last hyphen.
  local id = arg
  local decoded = host.json.decode(arg)
  if type(decoded) == "table" then
    id = decoded.id or decoded.url or ""
  elseif type(decoded) == "string" then
    id = decoded
  end
  if type(id) ~= "string" then id = tostring(id or "") end
  local num = id:match("([%d]+)%s*$")
  if not num then
    return nil, "no_pages"
  end
  -- The chapter-content endpoint returns the full deterministic image list —
  -- no token, no browser scroll-harvest needed.
  local body = http_get(BASE .. "/auth/chapter-content?chapter_id=" .. num)
  if not body or body == "" then
    return nil, "fetch_failed"
  end
  local data = host.json.decode(body)
  if not data or not data.images or #data.images == 0 then
    return nil, "no_pages"
  end
  local out = {}
  for i, url in ipairs(data.images) do
    out[#out + 1] = { index = i - 1, url = url }
  end
  return host.json.encode(out)
end

-- ─── genres ────────────────────────────────────────────────────────────────

-- Genre vocabulary harvested from the mangataro.org tag page (WordPress tags
-- at /tag/{slug}/, filter by appending the slug). Display names preserve
-- the site's exact casing.
local GENRES = {
    { name = "Action",          slug = "action" },
    { name = "Adaptation",      slug = "adaptation" },
    { name = "Adventure",       slug = "adventure" },
    { name = "Award Winning",   slug = "award-winning" },
    { name = "Ecchi",           slug = "ecchi" },
    { name = "Isekai",          slug = "isekai" },
    { name = "Martial Arts",    slug = "martial-arts" },
    { name = "Mecha",           slug = "mecha" },
    { name = "Mythology",       slug = "mythology" },
    { name = "Reincarnation",   slug = "reincarnation" },
    { name = "Romance",         slug = "romance" },
    { name = "Shounen",         slug = "shounen" },
    { name = "Slice of Life",   slug = "slice-of-life" },
    { name = "Team Sports",     slug = "team-sports" },
    { name = "Video Game",      slug = "video-game" },
}

function get_genres()
    local genres = {}
    for _, g in ipairs(GENRES) do
        genres[#genres + 1] = { name = g.name, slug = g.slug }
    end
    return host.json.encode(genres)
end
