-- ZeroScann plugin for goIsekai
-- WordPress mangareader-style theme. All pages SSR:
--   Search GET /manga?keyword={query} -> result cards
--   Detail  GET /manga/{slug}          -> manga meta + chapter links
--   Pages   GET /manga/{slug}/chapter-{n} -> <img> CDN links
--   Chapter IDs use decimal numbers (e.g. chapter-2.1).
--   Images require a Referer (403 without, 200 with).

PLUGIN = {
    contract_version = 1,
    name = "ZeroScann",
    site_url = "https://zeroscann.com",
    verify_url = "https://zeroscann.com",
    needs_human_verify = false,
    thumb_ratio = 0.667,
    logo = "logo.png",
}

BASE = "https://zeroscann.com"

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

local function slug_from_href(href)
    if href:match("^https?://") then
        return href:match("/manga/([^/?#]+)")
    end
    return href:match("/manga/([^/?#]+)")
end

local function chapter_num_from_href(href)
    return href:match("/manga/[^/]+/chapter%-(.+)")
end

function search_manga(arg)
    local args = host.json.decode(arg) or {}
    local query = host.text.url_encode(args.query or "")
    local page = args.page or 1
    local url = BASE .. "/manga?keyword=" .. query
    if page > 1 then
        url = url .. "&page=" .. page
    end

    local doc = fetch(url)
    if not doc then
        return host.json.encode({})
    end

    -- Result cards: div.zs-bg-3, title anchor a.zs-link (same selector => aligned lists)
    local manga_urls = host.html.find_list_attr(doc, ".zs-bg-3 a.zs-link", "href")
    local titles = host.html.find_list_text(doc, ".zs-bg-3 a.zs-link")

    local out = {}
    local seen = {}
    local by_slug = {}
    for i, href in ipairs(manga_urls) do
        local slug = slug_from_href(href)
        if slug and not seen[slug] then
            seen[slug] = true
            local title = titles[i] or slug
            local item = { id = slug, title = title, cover_url = "" }
            out[#out + 1] = item
            by_slug[slug] = item
        end
    end

    -- Covers: background-image on .v-image__image inside cards.
    -- Join on thumb filename (≈ slug), never by flat-list index.
    local cover_styles = host.html.find_list_attr(doc, ".zs-bg-3 .v-image__image", "style")
    local cover_by_slug = {}
    local cover_in_order = {}
    for _, st in ipairs(cover_styles) do
        local img = st:match("url%(([^)]+)%)")
        if img then
            img = img:gsub("&quot;", ""):gsub("'", ""):gsub('"', ""):gsub("^%s+", ""):gsub("%s+$", "")
            if img ~= "" then
                if not img:match("^https?://") then img = BASE .. img end
                cover_in_order[#cover_in_order + 1] = img
                local thumb = img:match("/thumb/([^/?#]+)")
                if thumb then thumb = thumb:gsub("%.webp$", ""):gsub("%.jpg$", ""):gsub("%.png$", "") end
                if thumb and thumb ~= "" then cover_by_slug[thumb] = img end
            end
        end
    end
    for _, item in ipairs(out) do
        item.cover_url = cover_by_slug[item.id] or ""
    end
    -- Fallback: document-order alignment for cards whose thumb name differs
    if #cover_in_order == #out then
        for idx, item in ipairs(out) do
            if item.cover_url == "" then item.cover_url = cover_in_order[idx] or "" end
        end
    end
    return host.json.encode(out)
end

function get_manga_detail(arg)
    local manga_id = host.json.decode(arg)
    local doc = fetch(BASE .. "/manga/" .. manga_id)
    if not doc then
        return host.json.encode({id=manga_id, title="", cover_url=""})
    end

    local genre_links = host.html.find_list_text(doc, "a[href*='/manga/genre/']")

    local cover = host.html.find_attr(doc, "meta[property='og:image']", "content")
    local status_raw = host.html.find_attr(doc, "meta[property='manga:status']", "content") or ""

    return host.json.encode({
        id = manga_id,
        title = host.html.find_attr(doc, "meta[property='og:title']", "content"),
        author = host.html.find_attr(doc, "meta[property='author']", "content") or "",
        description = host.html.find_attr(doc, "meta[name='description']", "content") or "",
        cover_url = cover,
        genres = genre_links,
        status = host.text.normalize_status(status_raw),
    })
end

function get_chapter_list(arg)
    local manga_id = host.json.decode(arg)
    local doc = fetch(BASE .. "/manga/" .. manga_id)
    if not doc then
        return host.json.encode({})
    end

    -- Chapter links: /manga/{slug}/chapter-{num} (slash before chapter)
    local urls = host.html.find_list_attr(doc, "a[href*='/chapter-']", "href")

    local out = {}
    local seen = {}
    for _, url in ipairs(urls) do
        -- keep only chapters of this manga (drop nav to other titles)
        if url:find("/manga/" .. manga_id .. "/chapter-", 1, true) then
            local num = chapter_num_from_href(url)
            -- store the path form for get_page_list (hrefs are absolute)
            local path = url:match("^https?://[^/]+(/.*)$") or url
            if num and path and not seen[path] then
                seen[path] = true
                out[#out + 1] = {
                    id = path,
                    title = "Chapter " .. num,
                    lang = "id",
                }
            end
        end
    end
    return host.json.encode(out)
end

function get_page_list(arg)
    local chapter_id = host.json.decode(arg)
    -- chapter_id is the full reader path: /manga/{slug}/chapter-{num}
    local url = BASE .. chapter_id

    local doc = fetch(url)
    if not doc then
        return host.json.encode({})
    end

    -- Pages: SSR <img> inside .pages.longstrip (CDN holdingyouclose.xyz)
    local imgs = host.html.find_list_attr(doc, ".pages.longstrip img", "src")
    if #imgs == 0 then
        imgs = host.html.find_list_attr(doc, "img[src*='holdingyouclose']", "src")
    end
    local out = {}
    for i, img_url in ipairs(imgs) do
        out[#out + 1] = { index = i - 1, url = img_url }
    end
    return host.json.encode(out)
end
