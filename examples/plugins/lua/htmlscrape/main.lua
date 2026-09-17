-- HTML-scraping example plugin for goIsekai
--
-- What this shows: reading a site with host.html.* — parse the response once,
-- then query the handle with a CSS selector or an XPath expression — instead of
-- string.match over raw markup. The same nine functions exist in the JS and
-- Yaegi runtimes with the same names and the same argument order, so the
-- lookups below port across unchanged.
--
--   local doc = host.html.parse(host.http.get_body(url))
--   host.html.find_text(doc, "h1.title")            first match text (trimmed)
--   host.html.find_attr(doc, "img.cover", "src")    first match attribute
--   host.html.find_list_text(doc, "a.chapter")      every match, document order
--   host.html.find_list_attr(doc, "img.page", "src")
--   host.html.xpath_text(doc, "//h1[@class='title']")
--   host.html.xpath_list_attr(doc, "//img[@class='page']", "src")
--
-- A lookup that matches nothing returns "" or an empty list. An unparseable
-- selector or XPath expression is an error naming the offending value, so pass
-- "" for the argument you want spell-checked. Malformed markup never errors.
--
-- This is not a real site: the target is example.invalid and the selectors match
-- the stub page that TestLuaHTMLScrapeExample serves. Copy the shape, replace
-- BASE and the selectors.
--
-- Layout: real site plugins also ship a helpers.lua (http_get + normalizeStatus)
-- that pre-executes before this file — see examples/plugins/lua/mangabuddy/.
-- This example calls host.http.get directly so it stays one file.

PLUGIN = {
    contract_version = 1,
    name = "HTML Scrape Demo",
    site_url = "https://example.invalid",
    verify_url = "https://example.invalid",
    needs_human_verify = false,
    thumb_ratio = 0.667,
}

BASE = "https://example.invalid"

-- Ids may be full URLs, so this example can be pointed at any host (the test
-- serves a stub on 127.0.0.1) without editing the file.
local function url_for(kind, id)
    if host.regex.match(id, "^https?://") then
        return id
    end
    return BASE .. "/" .. kind .. "/" .. id
end

-- fetch returns a parsed document handle, or nil when the request failed.
local function fetch(url)
    local body = host.http.get_body(url, {
        ["User-Agent"] = "Mozilla/5.0 (compatible; goIsekai-plugin/1.0)",
        ["Accept"] = "text/html,application/xhtml+xml",
    })
    if not body then
        return nil
    end
    return host.html.parse(body)
end

-- ─── ABI: search_manga(arg) ────────────────────────────────────────────────
-- arg: {"query":"...","page":1}  ->  array of {id, title, cover_url}
function search_manga(arg)
    local args = host.json.decode(arg) or {}
    local doc = fetch(BASE .. "/search?q=" .. host.text.url_encode(args.query or ""))
    if not doc then
        return host.json.encode({})
    end

    -- One <a class="card"> per result, each with a .title and an <img>. The three
    -- lists line up because every card carries both; a site where some cards
    -- omit the cover would need an XPath over the card nodes instead.
    local hrefs = host.html.find_list_attr(doc, "a.card", "href")
    local titles = host.html.find_list_text(doc, "a.card .title")
    local covers = host.html.find_list_attr(doc, "a.card img", "src")

    local out = {}
    for i, href in ipairs(hrefs) do
        out[#out + 1] = { id = href, title = titles[i] or "", cover_url = covers[i] or "" }
    end
    return host.json.encode(out)
end

-- ─── ABI: get_manga_detail(arg) ────────────────────────────────────────────
-- arg: JSON-encoded manga id  ->  {id, title, author, description, cover_url, genres, status}
function get_manga_detail(arg)
    local manga_id = host.json.decode(arg)
    local doc = fetch(url_for("manga", manga_id))
    if not doc then
        return host.json.encode({})
    end

    local genres = {}
    for _, name in ipairs(host.html.find_list_text(doc, "a.genre")) do
        genres[#genres + 1] = name
    end

    return host.json.encode({
        id = manga_id,
        title = host.html.find_text(doc, "h1.title"),
        author = host.html.find_attr(doc, "a.author", "href"),
        description = host.html.find_text(doc, "div.summary"),
        cover_url = host.html.find_attr(doc, "img.cover", "src"),
        -- The raw status sits in data-status; the host maps it to the canonical
        -- vocabulary, so the plugin does not carry the source's words itself.
        status = host.text.normalize_status(host.html.find_attr(doc, "span.status", "data-status")),
        genres = genres,
    })
end

-- ─── ABI: get_chapter_list(arg) ────────────────────────────────────────────
-- arg: JSON-encoded manga id  ->  array of {id, title, chapter_num, url}
function get_chapter_list(arg)
    local manga_id = host.json.decode(arg)
    local doc = fetch(url_for("manga", manga_id))
    if not doc then
        return host.json.encode({})
    end

    -- XPath reaches the rows by position and by text content, which a CSS
    -- selector cannot express; the CSS list lookups above stay index-aligned.
    local urls = host.html.xpath_list_attr(doc, "//ul[@class='chapters']/li/a", "href")
    local titles = host.html.xpath_list_text(doc, "//ul[@class='chapters']/li/a")

    local out = {}
    for i, url in ipairs(urls) do
        out[#out + 1] = {
            id = url,
            manga_id = manga_id,
            title = titles[i] or "",
            -- The number is the position in the list, so no second parse pass.
            chapter_num = i,
            url = url,
        }
    end
    return host.json.encode(out)
end

-- ─── ABI: get_page_list(arg) ───────────────────────────────────────────────
-- arg: JSON-encoded chapter id  ->  array of {index, url}
-- Page images live in data-src (a lazy-load attribute), so the lookup asks for
-- that rather than src.
function get_page_list(arg)
    local chapter_id = host.json.decode(arg)
    local doc = fetch(url_for("chapter", chapter_id))
    if not doc then
        return host.json.encode({})
    end

    local out = {}
    for i, url in ipairs(host.html.find_list_attr(doc, "img.page", "data-src")) do
        out[#out + 1] = { index = i, url = url }
    end
    log.debug("htmlscrape: " .. #out .. " pages for chapter " .. tostring(chapter_id))
    return host.json.encode(out)
end
