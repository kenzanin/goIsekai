-- MangaBuddy plugin for goIsekai
-- Site: https://mangabuddy1.co.uk (mangabuddy.com redirects here; .com is a
-- status/mirror tracker page, the reader lives on the .co.uk mirror)
-- ABI contract version: 1 (matches pkg/types ContractVersion)
--
-- Site profile: custom Tailwind/Laravel reader (not Madara). No Cloudflare JS
-- challenge — plain GETs with a browser UA return 200. Data sources:
--   search:  GET /api/search?search=Q   -> JSON {comics:[{slug_hash,title,image,status}]}
--                                       (found in search-modal.js; /home?keyword= does NOT
--                                       filter server-side — it renders the home feed)
--   detail:  GET /series/{slug}.{zid} -> HTML; og:title/og:image, meta description,
--                                       Status/Author label rows, /genre/{g} chips
--   chapter: GET /get-chapter-list?slug={bare-slug}  -> clean JSON, all chapters,
--                                       newest-first, has chapter_num + updated_at
--   pages:   GET /series/{slug}.{zid}/{chapter-slug} -> HTML; data-src CDN webp imgs

PLUGIN = {
    contract_version = 1,
    name = "MangaBuddy",
    site_url = "https://mangabuddy1.co.uk",
    logo = "logo.png",
    verify_url = "https://mangabuddy1.co.uk",
    needs_human_verify = false,
    thumb_ratio = 0.667,
    search_page_size = 24,
    alt_title_servers = {{id = "mangadex", name = "MangaDex"}}
}

BASE = "https://mangabuddy1.co.uk"
CDN = "https://cdn1.love4awalk.xyz"
UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36"

-- normalizeStatus maps a raw status string to a canonical host value.
-- Canonical set: Ongoing, Completed, Hiatus, Dropped, Upcoming.
-- Unknown values pass through as-is.
local function normalizeStatus(s)
    if not s or s == "" then return "unknown" end
    local raw = s:lower()
    if raw:find("ongo") or raw:find("releas") or raw:find("publish") then return "Ongoing" end
    if raw:find("complet") or raw:find("finish") then return "Completed" end
    if raw:find("hiatus") or raw:find("on.?hold") or raw:find("onhold") then return "Hiatus" end
    if raw:find("drop") or raw:find("cancel") then return "Dropped" end
    if raw:find("upcom") or raw:find("not.?publish") then return "Upcoming" end
    return s
end

-- ─── helpers ───────────────────────────────────────────────────────────────

function url_encode(s)
    return (s:gsub("([^%w%-%.%_%~])", function(c)
        return string.format("%%%02X", string.byte(c))
    end))
end

-- Escape Lua pattern magic chars in literals interpolated into patterns.
function lua_escape(s)
    return (s:gsub("[%-%.%+%[%]%(%)%$%^%%%?%*]", "%%%0"))
end

function http_get(url, accept_json)
    local headers = {
        ["User-Agent"] = UA,
        ["Accept"] = "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
        ["Accept-Language"] = "en-US,en;q=0.9"
    }
    if accept_json then
        headers["Accept"] = "application/json"
        headers["Referer"] = BASE .. "/"
    end
    local resp = http_request({url = url, method = "GET", headers = headers})
    if not resp then
        log.error("http_request returned nil for " .. url)
    elseif resp.status ~= 200 then
        log.error("http status " .. tostring(resp.status) .. " for " .. url)
    end
    return resp
end

function decode_entities(s)
    if not s then return "" end
    -- site double-escapes some entities in meta description: "&amp;#039;"
    s = s:gsub("&amp;#0?39;", "'"):gsub("&amp;quot;", '"')
    s = s:gsub("&#0?39;", "'"):gsub("&apos;", "'")
    s = s:gsub("&quot;", '"')
    s = s:gsub("&amp;", "&")
    return s
end

function titlecase(s)
    if s == nil then return "" end
    return s:sub(1, 1):upper() .. s:sub(2)
end

-- Full manga id is "{slug}.{zid}"; several endpoints want the bare slug.
function bare_slug(manga_id)
    return (manga_id:match("^(.-)%."))
end

-- Info rows on the detail page carry their value as a link target, not a
-- text node: <a href="/series?status=Ongoing"> and <a href="/author/NAME">.
function label_value(html, label, href_prefix)
    -- no ">" anchor: the label is preceded by a newline+indent in real markup
    local pos = string.match(html, label .. "%s*</h1>()")
    if not pos then return "" end
    local seg = string.sub(html, pos, pos + 400)
    return string.match(seg, 'href="' .. lua_escape(href_prefix) .. '([^"]+)"') or ""
end

-- ─── ABI: search_manga(arg) ────────────────────────────────────────────────
-- arg: {"query":"...","page":1}  ->  array of {id, title, cover_url}
function search_manga(arg)
    local args = json.decode(arg)
    local query = args.query or ""
    log.debug("search q=" .. query)
    -- /home?keyword= renders the plain home feed (no server-side filtering);
    -- the search modal calls /api/search?search=Q instead.
    local resp = http_get(BASE .. "/api/search?search=" .. url_encode(query), true)
    if not resp or resp.status ~= 200 then
        return json.encode({})
    end
    local ok, data = pcall(json.decode, resp.body)
    if not ok or not data or type(data.comics) ~= "table" then
        return json.encode({})
    end
    local out = {}
    for _, c in ipairs(data.comics) do
        if c.slug_hash and c.slug_hash ~= "" then
            out[#out + 1] = {
                id = c.slug_hash,
                title = c.title or "",
                cover_url = c.image or ""
            }
        end
    end
    log.debug("search: found " .. #out .. " results for q=" .. query)
    return json.encode(out)
end

-- ─── ABI: get_manga_detail(arg) ────────────────────────────────────────────
-- arg: '"slug.ZID"'  ->  {id, title, author, description, cover_url, genres, status}
function get_manga_detail(arg)
    local manga_id = json.decode(arg)
    local resp = http_get(BASE .. "/series/" .. manga_id)
    if not resp or resp.status ~= 200 then
        return json.encode({id = manga_id}) -- detail must stay an OBJECT
    end
    local html = resp.body

    local title = string.match(html, '<meta property="og:title" content="([^"]*)"') or ""
    title = title:gsub("^Read%s+", "")
    title = title:gsub("%s*|%s*MangaBuddy%s*$", "")
    title = title:gsub("%s+Online$", "")
    title = title:gsub("%s+(Manga|Manhwa|Comic)$", "")
    title = decode_entities(title)

    local cover = string.match(html, '<meta property="og:image" content="([^"]*)"') or ""

    local desc = string.match(html, '<meta name="description" content="([^"]*)"') or ""
    desc = desc:gsub("^Read%s+[^.]+%.%s*", ""):gsub("%s*Read the latest chapters online for free at MangaBuddy%.?%s*$", "")
    desc = decode_entities(desc)

    local status = label_value(html, "Status", "/series?status=")
    local author = label_value(html, "Author", "/author/")
    if author == "Unknown" then author = "" end

    local genres = {}
    local seen_g = {}
    for g in string.gmatch(html, 'href="/genre/([a-z0-9%-]+)"') do
        if not seen_g[g] then
            seen_g[g] = true
            genres[#genres + 1] = g:gsub("%-", " "):gsub("(%a)([%w']*)", function(a, b)
                return a:upper() .. b
            end)
        end
    end

    local detail = {
        id = manga_id,
        title = title,
        author = author,
        description = desc,
        cover_url = cover,
        genres = genres,
        status = normalizeStatus(status)
    }
    log.debug("detail: " .. detail.title .. " | " .. detail.status)
    return json.encode(detail)
end

-- ─── ABI: get_chapter_list(arg) ────────────────────────────────────────────
-- arg: '"slug.ZID"'  ->  array of {id, manga_id, chapter_num, title, url, uploaded_at}
-- Chapters are newest-first (descending number) per ABI convention.
function get_chapter_list(arg)
    local manga_id = json.decode(arg)
    local slug = bare_slug(manga_id)
    if not slug then
        return json.encode({})
    end
    -- JSON endpoint wants the bare slug (no .zid suffix)
    local resp = http_get(BASE .. "/get-chapter-list?slug=" .. slug)
    if not resp or resp.status ~= 200 then
        return json.encode({})
    end
    local ok, body = pcall(json.decode, resp.body)
    if not ok or not body or not body.success or type(body.data) ~= "table" then
        log.error("get-chapter-list bad payload for " .. slug)
        return json.encode({})
    end

    local chapters = {}
    for _, ch in ipairs(body.data) do
        local num = ch.chapter_num or tonumber((ch.chapter_slug or ""):match("(%d+)$"))
        if num then
            chapters[#chapters + 1] = {
                id = manga_id .. ":" .. (ch.chapter_slug or ("chapter-" .. num)),
                manga_id = manga_id,
                chapter_num = num,
                title = ch.chapter_name or ("Chapter " .. tostring(num)),
                url = BASE .. "/series/" .. manga_id .. "/" .. (ch.chapter_slug or ("chapter-" .. num)),
                released_at = ch.updated_at or ""
            }
        end
    end
    table.sort(chapters, function(a, b) return a.chapter_num > b.chapter_num end)
    log.debug("chapters: " .. #chapters .. " for " .. manga_id)
    return json.encode(chapters)
end

-- ─── ABI: get_page_list(arg) ───────────────────────────────────────────────
-- arg: '"slug.ZID:N"' (chapter id from get_chapter_list)  ->  array of {url}
function get_page_list(arg)
    local chapter_id = json.decode(arg) -- "slug.ZID:chapter_slug"
    local manga_id, chslug = string.match(chapter_id, "^(.-):(.+)$")
    if not manga_id or not chslug then
        return json.encode({})
    end
    local resp = http_get(BASE .. "/series/" .. manga_id .. "/" .. chslug)
    if not resp or resp.status ~= 200 then
        return json.encode({})
    end
    local html = resp.body

    -- Page images: data-src="https://cdn1.love4awalk.xyz/{slug}/{ch}/{i}.webp"
    -- (chapter cover /thumb/ images and the discord gif don't match this shape).
    local pages = {}
    local seen = {}
    for u in string.gmatch(html, 'data%-src="(https://cdn1%.love4awalk%.xyz/[^"]-/%d+/%d+%.webp)"') do
        if not seen[u] then
            seen[u] = true
            pages[#pages + 1] = {url = u}
        end
    end
    log.debug("pages: " .. #pages .. " for chapter " .. manga_id .. ":" .. chslug)
    return json.encode(pages)
end

-- ─── ABI: getAltTitles(arg) ────────────────────────────────────────────────
-- Each plugin carries its own alt-title source (MangaDex API) so no plugin
-- depends on another. arg: {"title":"...","server":"..."} -> {source, titles}
function getAltTitles(arg)
    local input = json.decode(arg)
    local title = input.title or ""
    local url = "https://api.mangadex.org/manga?title=" .. url_encode(title) ..
        "&limit=5&includes[]=manga"
    local resp = http_request({url = url, method = "GET", headers = {}})
    if not resp or resp.status ~= 200 then
        return json.encode({source = "MangaDex", titles = {}})
    end
    local ok, body = pcall(json.decode, resp.body)
    if not ok or not body or not body.data or #body.data == 0 then
        return json.encode({source = "MangaDex", titles = {}})
    end
    local attrs = body.data[1].attributes
    local out = {}
    local seen = {}
    local keep_langs = {en = true, ja = true, ["ja-ro"] = true, ko = true, ["ko-ro"] = true}
    if attrs.altTitles then
        for _, alt in ipairs(attrs.altTitles) do
            for lang, val in pairs(alt) do
                if keep_langs[lang] and val and not seen[val] then
                    seen[val] = true
                    out[#out + 1] = val
                end
            end
        end
    end
    return json.encode({source = "MangaDex", titles = out})
end
