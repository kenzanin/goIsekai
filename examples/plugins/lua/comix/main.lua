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

local BASE = host.plugin.param("site_url")

PLUGIN = {
    id = "comix",
    name = "Comix",
    site_url = BASE,
    version = "1",
    contract_version = 1,
    search_page_size = 28,
    timeout = 60,
    logo = "logo.png"
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
    -- Chapters are client-rendered; use the browser. Link + span:
    --   <a class="mchap-row__primary" href="/title/<slug>/<id>-chapter-<num>">
    --     <span class="mchap-row__ch">Ch.43.2</span>
    --     ... <time datetime="YYYY-MM-DD">...</time>
    local out = {}
    for _, m in ipairs(find_all(html, [[(?s)<a class="mchap-row__primary" href="(/title/[^"]+/([^"]+))">.*?<span class="mchap-row__ch">([^<]+)</span>.*?<time datetime="([^"]+)">]]) or {}) do
        local href, chid, chtitle, raw_date = m[1], m[2], m[3], m[4]
        -- Chapter number from "Ch.43.2" or the URL tail.
        local num = string.match(chtitle, "Ch%.([%d%.]+)")
        if not num then
            num = string.match(chid, "chapter%-([%d%.]+)")
        end
        local ch = {
            id = slug .. ":" .. chid,
            chapter_num = tonumber(num) or 0,
            title = chtitle,
        }
        -- <time datetime="2026-07-04"> is already ISO-8601 date — host
        -- date_to_iso accepts "2006-01-02" directly. Mirrors atsumaru
        end pattern, skip if absent (host keeps zero value).
        local iso = host.text.date_to_iso(host.text.trim(raw_date or ""))
        if iso ~= "" then ch.released_at = iso end
        out[#out + 1] = ch
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
    -- comix.to's page list exists only in the browser: the chapter API returns an
    -- encrypted {"e":"..."} payload that the site's own JS decrypts, and each
    -- page's <img src> is injected lazily as it scrolls into view. Neither the API
    -- nor SSR contains the URLs — the rendered DOM is the only source of truth.
    --
    -- Proven recipe (verified against a 28-page chapter with a standalone rod probe,
    -- 28/28 images harvested; curl confirmed the CDN serves 200 with no Referer and
    -- 403 with any Referer — handled by the referer-ladder no-Referer candidate):
    --   1. initJS: read-modify-write localStorage['reader.webtoon.v3'].state.preload
    --      to "all" (EvalOnNewDocument, before page scripts — persists to origin).
    --      Verified live: a 75-page chapter went from 3 imgs (some) to 68 (all).
    --      Even "all" still defers pages until viewport touch, so scrolling is required.
    --   2. js: wait for .rpage-page slots, then per-element scrollIntoView (NOT
    --      window.scroll — .rpage is a scroll parent) with a 180ms dwell so the
    --      reader's IntersectionObserver fires and sets img src. Sweep unloaded
    --      slots across up to 2 rounds, then read img[src] directly.

    local initJS = [[
        try {
            const raw = localStorage.getItem('reader.webtoon.v3');
            const cur = raw ? JSON.parse(raw) : { state: {} };
            cur.state = cur.state || {};
            cur.state.preload = 'all';
            localStorage.setItem('reader.webtoon.v3', JSON.stringify(cur));
        } catch (e) {}
    ]]

    local js = [[async () => {
        for (let i = 0; i < 25; i++) {
            if (document.querySelector('.rpage-page')) break;
            await new Promise(r => setTimeout(r, 400));
        }
        const slots = () => Array.from(document.querySelectorAll('.rpage-page'));
        for (let round = 0; round < 2; round++) {
            let done = true;
            for (const p of slots()) {
                const img = p.querySelector('img');
                if (!img || !img.getAttribute('src')) {
                    done = false;
                    p.scrollIntoView({ block: 'center', inline: 'center' });
                    await new Promise(r => setTimeout(r, 180));
                }
            }
            if (done) break;
        }
        const pages = [];
        document.querySelectorAll('.rpage-page img[src]').forEach((img) => {
            pages.push({ url: img.currentSrc || img.src });
        });
        return JSON.stringify(pages);
    }]]

    local out = host.browser.evaluate_with_init(url, initJS, js)
    if not out or out == "" then
        log.error("comix: browser evaluate failed for pages: " .. composite)
        return nil, "upstream_unavailable"
    end
    local pages = host.json.decode(out)
    if not pages or #pages == 0 then
        log.error("comix: no page images in chapter: " .. composite)
        return nil, "no_pages"
    end
    -- Normalize to 0-based index, dropping any slot that yielded no URL.
    local result = {}
    for i, p in ipairs(pages) do
        if p.url then
            result[#result + 1] = { index = i - 1, url = p.url }
        end
    end
    log.info("comix: found " .. tostring(#result) .. " pages for " .. composite)
    return host.json.encode(result)
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
    -- Genres live under list.options.genres in the browse payload, each entry
    -- shaped {id, label, slug}. Reading `name` here matched nothing and shipped
    -- an empty list, which the host cached as "[]" — an empty selector with no
    -- error to show for it.
    local out = {}
    local function walk(v)
        if type(v) ~= "table" then return end
        local label = v.label or v.name
        if type(label) == "string" and type(v.slug) == "string" and #label < 40 then
            out[#out + 1] = { name = label, slug = v.slug }
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
