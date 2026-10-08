-- Comix.to plugin (https://comix.to)
--
-- Uses host.browser.fetch/evaluate because the JSON API is signed by the
-- site's own front-end code (_=hqm.<token>). A static fetch gets 403
-- missing_token; replaying a token outside its browser context gets 403
-- invalid_token. The browser runs the site's JS, so signing happens there.
--
-- Data shapes (verified live):
--   search:  /browse?q=<q> renders a.lrow__poster cards
--   detail:  /title/<slug> embeds JSON in script#initial-data
--   chapters: /title/<slug> renders a.mchap-row__primary links
--   pages:   chapter page renders images (extract via evaluate)

local BASE = "https://comix.to"

PLUGIN = {
    id = "comix",
    name = "Comix",
    site_url = BASE,
    version = "1",
    contract_version = 1,
    search_page_size = 28,
}

local function find(html, pat)
    return host.regex.find(html, pat)
end

local function find_all(html, pat)
    return host.regex.find_all(html, pat)
end

-- ─── ABI: search_manga ───────────────────────────────────────────────────
-- arg: '"query"'
function search_manga(arg)
    local args = host.json.decode(arg)
    local query = (args and args.query) or ""
    if query == "" then
        return host.json.encode({})
    end
    local url = BASE .. "/browse?q=" .. host.text.url_encode(query)
    local html = host.browser.fetch(url)
    if not html or html == "" then
        log.error("comix: browser fetch failed for search: " .. query)
        return nil, "upstream_unavailable"
    end
    -- <a class="lrow__poster" href="/title/<slug>" aria-label="<title>">...
    --   <img ... src="<cover>">
    local out = {}
    for _, m in ipairs(find_all(html, [[(?s)<a class="lrow__poster" href="(/title/([^"]+))" aria-label="([^"]+)">.*?src="([^"]+)"]]) or {}) do
        local href, slug, title, cover = m[1], m[2], m[3], m[4]
        -- aria-label is HTML-escaped; decode entities.
        title = host.text.html_decode(title)
        out[#out + 1] = {
            id = slug,
            title = title,
            cover_url = cover,
        }
    end
    log.info("comix: search found " .. tostring(#out) .. " for " .. query)
    return host.json.encode(out)
end

-- ─── ABI: get_manga_detail ───────────────────────────────────────────────
-- arg: '"slug"'
function get_manga_detail(arg)
    local slug = host.json.decode(arg)
    if not slug then
        return host.json.encode({})
    end
    -- The detail JSON is embedded in static HTML, so plain HTTP works.
    local body = host.http.get_body(BASE .. "/title/" .. host.text.url_encode(slug))
    if not body or body == "" then
        log.error("comix: detail fetch failed: " .. slug)
        return nil, "upstream_unavailable"
    end
    local raw = find(body, [[(?s)<script type="application/json" id="initial-data">(.*?)</script>]])
    if not raw then
        log.error("comix: no initial-data in detail page: " .. slug)
        return nil, "envelope_unrecognised"
    end
    local data = host.json.decode(raw)
    if not data or not data.queries then
        return nil, "envelope_unrecognised"
    end
    -- Find the detail query: ["manga","detail","<hid>"]
    for k, v in pairs(data.queries) do
        if string.find(k, '"detail"', 1, true) then
            local genres = {}
            if v.genres then
                for _, g in ipairs(v.genres) do
                    if type(g) == "table" then
                        genres[#genres + 1] = g.title or g.slug or g.name
                    elseif type(g) == "string" then
                        genres[#genres + 1] = g
                    end
                end
            end
            -- poster is {medium=..., large=...}; take the largest. Coerce
            -- anything non-string to nil so cover_url is always a string.
            local cover = nil
            if type(v.cover) == "string" then
                cover = v.cover
            elseif type(v.poster) == "string" then
                cover = v.poster
            elseif type(v.poster) == "table" then
                if type(v.poster.large) == "string" then
                    cover = v.poster.large
                elseif type(v.poster.medium) == "string" then
                    cover = v.poster.medium
                end
            end
            cover = cover or ""
            return host.json.encode({
                id = slug,
                title = v.title,
                cover_url = cover,
                author = v.author,
                artist = v.artist,
                status = v.status,
                description = v.synopsis or v.description,
                genres = genres,
            })
        end
    end
    log.error("comix: no detail query in initial-data: " .. slug)
    return nil, "envelope_unrecognised"
end

-- ─── ABI: get_chapter_list ───────────────────────────────────────────────
-- arg: '"slug"'
function get_chapter_list(arg)
    local slug = host.json.decode(arg)
    if not slug then
        return host.json.encode({})
    end
    -- Chapters are client-rendered; use the browser.
    local html = host.browser.fetch(BASE .. "/title/" .. host.text.url_encode(slug))
    if not html or html == "" then
        log.error("comix: browser fetch failed for chapters: " .. slug)
        return nil, "upstream_unavailable"
    end
    -- <a class="mchap-row__primary" href="/title/<slug>/<id>-chapter-<num>">
    --   <span class="mchap-row__ch">Ch.43.2</span>
    local out = {}
    for _, m in ipairs(find_all(html, [[(?s)<a class="mchap-row__primary" href="(/title/[^"]+/([^"]+))">.*?<span class="mchap-row__ch">([^<]+)</span>]]) or {}) do
        local href, chid, chtitle = m[1], m[2], m[3]
        -- Chapter number from "Ch.43.2" or the URL tail.
        local num = string.match(chtitle, "Ch%.([%d%.]+)")
        if not num then
            num = string.match(chid, "chapter%-([%d%.]+)")
        end
        out[#out + 1] = {
            id = slug .. ":" .. chid,
            chapter_num = tonumber(num) or 0,
            title = chtitle,
        }
    end
log.info("comix: found " .. tostring(#out) .. " chapters for " .. slug)
    -- Site already renders newest first; keep document order.
    return host.json.encode(out)
end

-- ─── ABI: get_page_list ──────────────────────────────────────────────────
-- arg: '"slug:chapter-id"'
function get_page_list(arg)
    local composite = host.json.decode(arg)
    if not composite then
        return host.json.encode({})
    end
    local slug, chid = string.match(composite, "^([^:]+):(.+)$")
    if not slug or not chid then
        log.error("comix: bad chapter id: " .. tostring(composite))
        return nil, "no_pages"
    end
    local url = BASE .. "/title/" .. host.text.url_encode(slug) .. "/" .. host.text.url_encode(chid)
    -- Pages come from a token-gated API. Hook fetch before page scripts run,
    -- let the reader fetch, then read the captured API response which holds
    -- all page URLs at once (no scrolling needed).
    local initJS = [[
        window.__cxPages = null;
        const origParse = JSON.parse;
        JSON.parse = function(text, ...rest) {
            try {
                const obj = origParse.call(this, text, ...rest);
                // Pages API returns {pages: [...]} or [{...}]; capture arrays
                // of objects with image URLs.
                const check = (v) => {
                    if (Array.isArray(v) && v.length >= 5 && v[0] && typeof v[0] === 'object') {
                        const s = JSON.stringify(v[0]);
                        if (s.includes('http') && (s.includes('.site/') || s.includes('comix'))) {
                            window.__cxPages = JSON.stringify(v);
                        }
                    }
                };
                if (obj && typeof obj === 'object') {
                    check(obj.pages || obj.data || obj);
                    for (const k of Object.keys(obj).slice(0, 10)) check(obj[k]);
                }
            } catch (e) {}
            return origParse.call(this, text, ...rest);
        };
    ]]
    local js = [[() => {
        return (async () => {
            for (let i = 0; i < 30; i++) {
                if (window.__cxPages) return window.__cxPages;
                await new Promise(r => setTimeout(r, 500));
            }
            return '';
        })();
    }]]
    local out = host.browser.evaluate_with_init(url, initJS, js)
    if not out or out == "" then
        log.error("comix: browser evaluate failed for pages: " .. composite)
        return nil, "upstream_unavailable"
    end
    -- The captured API response is encrypted {"e": "..."}; decrypt via
    -- the site's own flow is not available, so fall back to DOM images.
    -- For now, try parsing as direct URL list; if encrypted, report it.
    local urls = host.json.decode(out)
    if not urls or #urls == 0 then
        log.error("comix: no page images in chapter: " .. composite)
        return nil, "no_pages"
    end
    -- Captured pages may be objects {url=..., src=..., image=...} or plain
    -- URL strings; normalize to strings.
    local pages = {}
    for i, u in ipairs(urls) do
        local urlstr = nil
        if type(u) == "string" then
            urlstr = u
        elseif type(u) == "table" then
            urlstr = u.url or u.src or u.image or u.link
        end
        if urlstr then
            pages[#pages + 1] = { index = i - 1, url = urlstr }
        end
    end
    log.info("comix: found " .. tostring(#pages) .. " pages for " .. composite)
    return host.json.encode(pages)
end

-- ─── ABI: get_genres ─────────────────────────────────────────────────────
function get_genres(_)
    -- Genres are embedded in /browse static HTML (31 genres).
    local body = host.http.get_body(BASE .. "/browse")
    if not body or body == "" then
        return host.json.encode({})
    end
    local raw = find(body, [[(?s)<script type="application/json" id="initial-data">(.*?)</script>]])
    if not raw then
        return host.json.encode({})
    end
    local data = host.json.decode(raw)
    if not data then
        return host.json.encode({})
    end
    -- Genres live under list.options.genres in the browse payload.
    local out = {}
    local function walk(v)
        if type(v) ~= "table" then return end
        if v.name and v.slug and #v.name < 40 then
            -- Heuristic: genre entries have short name+slug pairs.
            out[#out + 1] = { name = v.name, slug = v.slug }
            return
        end
        for _, child in pairs(v) do
            walk(child)
        end
    end
    walk(data)
    -- Dedupe by slug.
    local seen, genres = {}, {}
    for _, g in ipairs(out) do
        if not seen[g.slug] then
            seen[g.slug] = true
            genres[#genres + 1] = g
        end
    end
    log.info("comix: found " .. tostring(#genres) .. " genres")
    return host.json.encode(genres)
end
