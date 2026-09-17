-- MangaKatana plugin for goIsekai (Lua / Lunar VM)
-- Site: https://mangakatana.com
-- ABI contract version: 1 (matches pkg/types ContractVersion)
--
-- Everything network- or text-shaped is host.*; there is no local helper file.
-- The one thing host.html cannot express is page_urls() below.

PLUGIN = {
    contract_version = 1,
    name = "MangaKatana",
    site_url = "https://mangakatana.com",
    logo = "logo.png",
    verify_url = "https://mangakatana.com",
    needs_human_verify = false,
    thumb_ratio = 0.703,
    search_page_size = 20,
}

BASE = "https://mangakatana.com"

local HEADERS = {
    ["User-Agent"] = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36",
    ["Accept"] = "text/html,application/xhtml+xml",
}

-- Ids are pasted into host URL paths (/view/manga/{mangaID} and
-- /view/read/{mangaID}/{chapterID}), and a bare path segment cannot hold a
-- slash: a manga id is the slug and a chapter id is "<manga slug>:<cXXX>".
-- Absolute URLs still work here so a test can point the plugin at a stub host.
local function url_for(kind, id)
    if string.match(id, "^https?://") then
        return id
    end
    return BASE .. "/" .. kind .. "/" .. id
end

-- "https://mangakatana.com/manga/naruto.1205" and "/manga/naruto.1205" both
-- reduce to "naruto.1205".
local function slug_of(id)
    return string.match(id, "/manga/([^/?#]+)") or id
end

-- "naruto.1205:c700" -> the chapter's absolute URL.
local function chapter_url(id)
    if string.match(id, "^https?://") then
        return id
    end
    local slug, tail = string.match(id, "^([^:]+):(.+)$")
    if not slug then
        return id
    end
    return url_for("manga", slug) .. "/" .. tail
end

-- get returns the response body, or nil when the request failed.
local function get(url)
    return host.http.get_body(url, HEADERS)
end

local function doc_for(url)
    local body = get(url)
    if not body then
        return nil
    end
    return host.html.parse(body)
end

-- The reader page ships no image URLs in its markup: every <img> carries
-- data-src="#" and a load handler copies the real URLs out of a JavaScript
-- array (var thzq=[...]) into them. No selector can read that, so read the
-- array — the longest one on the page wins, which keeps a rename of the
-- obfuscated variable from breaking us.
local function page_urls(html)
    local best = {}
    for array in string.gmatch(html, "%[(.-)%]") do
        local urls = {}
        for url in string.gmatch(array, "'(https?://[^']+)'") do
            urls[#urls + 1] = url
        end
        if #urls > #best then
            best = urls
        end
    end
    return best
end

-- ─── ABI: search_manga(arg) ────────────────────────────────────────────────
-- arg: {"query":"...","page":1}  ->  array of {id, title, cover_url}
-- Page 1 is the bare query string; later pages move the path to /page/N.
function search_manga(arg)
    local args = host.json.decode(arg)
    local q = host.text.url_encode(args.query or "")
    local page = tonumber(args.page) or 1

    local url = BASE .. "/?search=" .. q .. "&search_by=book_name"
    if page > 1 then
        url = BASE .. "/page/" .. page .. "?search=" .. q .. "&search_by=book_name"
    end

    local doc = doc_for(url)
    if not doc then
        return host.json.encode({})
    end

    -- Every card carries a title link and one cover <img> (results without a
    -- cover use /imgs/no-cover.png), so the lists stay index-aligned.
    local hrefs = host.html.find_list_attr(doc, "#book_list .item h3.title a", "href")
    local titles = host.html.find_list_text(doc, "#book_list .item h3.title a")
    local covers = host.html.find_list_attr(doc, "#book_list .item .media img", "src")

    local out = {}
    for i, href in ipairs(hrefs) do
        out[#out + 1] = { id = slug_of(href), title = titles[i] or "", cover_url = covers[i] or "" }
    end
    log.debug("mangakatana: " .. #out .. " results for q=" .. tostring(args.query))
    return host.json.encode(out)
end

-- ─── ABI: get_manga_detail(arg) ────────────────────────────────────────────
-- arg: JSON-encoded manga id  ->  {id, title, author, description, cover_url, genres, status}
function get_manga_detail(arg)
    local manga_id = slug_of(host.json.decode(arg))
    local doc = doc_for(url_for("manga", manga_id))
    if not doc then
        return host.json.encode({ id = manga_id })
    end

    -- #related sits inside #single_book and repeats the card shape, so every
    -- lookup below is scoped to stay off it.
    local genres = {}
    for _, name in ipairs(host.html.find_list_text(doc, "#single_book .genres a")) do
        genres[#genres + 1] = name
    end

    -- The description is one <p> whose line breaks are <br> tags. find_text
    -- walks past the void element and glues the words together, so read the
    -- text nodes instead: they are split exactly where the <br>s are. A run of
    -- two collapses to one break, which is a line rather than a blank one.
    local description = table.concat(
        host.html.xpath_list_text(doc, "//div[@id='single_book']//div[@class='summary']/p/text()"),
        "\n"
    )

    return host.json.encode({
        id = manga_id,
        title = host.html.find_text(doc, "h1.heading"),
        author = host.html.find_text(doc, "#single_book .authors a.author"),
        description = description,
        cover_url = host.html.find_attr(doc, "#single_book .cover img", "src"),
        -- The status cell reads "Completed" / "Ongoing" / "Cancelled"; the host
        -- owns the canonical vocabulary.
        status = host.text.normalize_status(host.html.find_text(doc, "#single_book div.value.status")),
        genres = genres,
    })
end

-- ─── ABI: get_chapter_list(arg) ────────────────────────────────────────────
-- arg: JSON-encoded manga id  ->  array of {id, manga_id, title, chapter_num, url}
function get_chapter_list(arg)
    local manga_id = slug_of(host.json.decode(arg))
    local doc = doc_for(url_for("manga", manga_id))
    if not doc then
        return host.json.encode({})
    end

    local selector = "div.chapters table tbody tr td div.chapter a"
    local urls = host.html.find_list_attr(doc, selector, "href")
    local titles = host.html.find_list_text(doc, selector)

    -- The table is newest-first already, which is the ABI convention.
    local out = {}
    for i, url in ipairs(urls) do
        out[#out + 1] = {
            -- .../c247, .../c230.5 and .../c0-v2 all end in the segment to keep.
            id = manga_id .. ":" .. (string.match(url, "/([^/]+)$") or url),
            manga_id = manga_id,
            -- The number is the tail of the href: .../c247, .../c230.5, .../c0-v2
            chapter_num = tonumber(string.match(url, "/c([%d%.]+)")) or 0,
            title = titles[i] or "",
            url = url,
        }
    end
    log.debug("mangakatana: " .. #out .. " chapters for " .. tostring(manga_id))
    return host.json.encode(out)
end

-- ─── ABI: get_page_list(arg) ───────────────────────────────────────────────
-- arg: JSON-encoded chapter id ("<manga slug>:<cXXX>")  ->  array of {index, url}
function get_page_list(arg)
    local chapter_id = chapter_url(host.json.decode(arg))
    local body = get(chapter_id)
    if not body then
        return host.json.encode({})
    end

    local out = {}
    for i, url in ipairs(page_urls(body)) do
        out[#out + 1] = { index = i, url = url }
    end
    log.debug("mangakatana: " .. #out .. " pages for " .. tostring(chapter_id))
    return host.json.encode(out)
end
