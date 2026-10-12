-- violetmanga.com plugin for goIsekai
--
-- WordPress mangareader-theme site. The reader page is JS-rendered:
-- the page list lives inside a `ts_reader.run({...})` call embedded in
-- the reader HTML — not in SSR markup. Use host.browser.evaluate to
-- run in-page JS that reads ts_reader's config and returns the image
-- URLs as JSON.
--
--   Search     GET /?s={query}            -> result cards
--   Detail     GET /comics/{slug}/         -> manga meta
--   Chapters   (same detail page)        -> chapter links
--   Pages      GET /{slug}-chapter-{N}/  -> ts_reader.run() config (sources[].images)

PLUGIN = {
    contract_version = 1,
    name = "VioletManga",
    site_url = "https://violetmanga.com",
    verify_url = "https://violetmanga.com",
    needs_human_verify = false,
    thumb_ratio = 0.667,
}

BASE = "https://violetmanga.com"

-- For static pages (search, detail, chapters) we can use plain HTTP.
-- Only the reader needs the browser because ts_reader() is JS-rendered.
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

function search_manga(arg)
    local args = host.json.decode(arg) or {}
    local query = host.text.url_encode(args.query or "")
    local page = args.page or 1
    local url = BASE .. "/?s=" .. query
    if page > 1 then
        url = url .. "&paged=" .. page
    end

    local doc = fetch(url)
    if not doc then
        return host.json.encode({})
    end

    -- bixbox/listupd theme: .bsx > a (href /comics/slug/) + .tt title + img[src]
    local hrefs = host.html.find_list_attr(doc, ".bsx > a", "href")
    local titles = host.html.find_list_text(doc, ".bsx .tt")
    local covers = host.html.find_list_attr(doc, ".bsx > a img", "src")

    local out = {}
    for i, url in ipairs(hrefs) do
        local slug = url:match("/comics/([^/]+)/?")
        if slug then
            out[#out + 1] = {
                id = slug,
                title = titles[i] or "",
                cover_url = covers[i] or "",
            }
        end
    end
    return host.json.encode(out)
end

function get_manga_detail(arg)
    local manga_id = host.json.decode(arg)
    local doc = fetch(BASE .. "/comics/" .. manga_id .. "/")
    if not doc then
        return host.json.encode({})
    end

    local genres = {}
    for _, name in ipairs(host.html.find_list_text(doc, ".genres-container .mgen a")) do
        genres[#genres + 1] = name
    end

    local statusRaw = host.html.find_text(doc, ".info-half .status i")
    return host.json.encode({
        id = manga_id,
        title = host.html.find_text(doc, "h1.entry-title"),
        author = host.html.find_text(doc, ".imptdt span.author i"),
        description = host.html.find_text(doc, ".summary .entry-content"),
        cover_url = host.html.find_attr(doc, ".thumb img", "src"),
        genres = genres,
        status = host.text.normalize_status(statusRaw),
    })
end

function get_chapter_list(arg)
    local manga_id = host.json.decode(arg)
    local doc = fetch(BASE .. "/comics/" .. manga_id .. "/")
    if not doc then
        return host.json.encode({})
    end

    -- detail page SSR: div.eplister#chapterlist, free chapters are a[href]
    -- (-> /{slug}-chapter-N/); locked (coin-gated) anchors have no href.
    local urls = host.html.find_list_attr(doc, "#chapterlist a[href]", "href")

    local out = {}
    for _, url in ipairs(urls) do
        -- store relative path: /view/read route needs URL-encode (#4076)
        local id = url:match("violetmanga%.com(/[^%?#]*)") or url
        local num = url:match("-chapter%-(.-)/?$") or ""
        local title = num ~= "" and ("Chapter " .. num) or ""
        out[#out + 1] = {
            id = id,
            manga_id = manga_id,
            title = title,
            url = id,
        }
    end
    return host.json.encode(out)
end

-- ts_reader.run() embeds the page URLs in a JS config object:
-- ts_reader.run({ ..., "sources": [{ "images": [...] }] }).
-- run is a function call, not a stored config — scan inline scripts for
-- the "images" array and return its URLs as JSON.
local PAGE_LIST_JS = [=[(function() {
    var scripts = document.querySelectorAll('script');
    for (var s = 0; s < scripts.length; s++) {
        var txt = scripts[s].textContent || '';
        if (txt.indexOf('ts_reader') !== -1) {
            var m = txt.match(/"images"\s*:\s*\[([^\]]*)\]/);
            if (m) {
                var urls = m[1].match(/https?:\/\/[^"']+/g);
                if (urls && urls.length) return JSON.stringify(urls);
            }
        }
    }
    return JSON.stringify([]);
})]=]

function get_page_list(arg)
    local chapter_id = host.json.decode(arg)
    local url = chapter_id
    if not url:match("^https?://") then
        url = BASE .. chapter_id
    end

    local body = host.http.get_body(url, {
        ["User-Agent"] = "Mozilla/5.0 (compatible; goIsekai-plugin/1.0)",
        ["Accept"] = "text/html,application/xhtml+xml",
    })
    if not body then
        return nil, "upstream_unavailable"
    end

    -- ts_reader.run({..."sources":[{"images":[...]}]}) — escaped slashes.
    local images = body:match('"images"%s*:%s*%[(.-)%]')
    if not images then
        log.error("violetmanga: no images array in chapter: " .. tostring(chapter_id))
        return nil, "no_pages"
    end
    local result = {}
    for raw in images:gmatch("https?:\\?/\\?/[^\"']+") do
        local clean = raw:gsub("\\/", "/")
        result[#result + 1] = { index = #result, url = clean }
    end
    if #result == 0 then
        log.error("violetmanga: no page images in chapter: " .. tostring(chapter_id))
        return nil, "no_pages"
    end
    log.info("violetmanga: found " .. tostring(#result) .. " pages for " .. tostring(chapter_id))
    return host.json.encode(result)
end
