-- dle.lua — shared DLE guard + page-shape helpers (Lua / Lunar VM).
-- Copied into both batcave and manga-bay plugin folders and loaded via
-- require("dle"). The guard is DLE's own proof-of-work (the host's browser
-- path only recognises Cloudflare markers, so the solve has to happen here).

local dle = {}

-- HTTP headers the host sends on behalf of a plugin.
dle.UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36"
dle.ACCEPT_LANG = "en-US,en;q=0.9"

-- Headers table the host wants as a flat map (NOT {headers={…}}).
function dle.headers(extra)
    local h = { ["User-Agent"] = dle.UA, ["Accept-Language"] = dle.ACCEPT_LANG }
    if extra then
        for k, v in pairs(extra) do h[k] = v end
    end
    return h
end

-- pow_nonce finds a nonce whose sha256(token..":"..nonce) has a hex prefix
-- of "00", the difficulty the DLE guard checks. The token is the raw string
-- from the page (url-decoded tokens do NOT match).
function dle.pow_nonce(token)
    if not token then return nil end
    for i = 0, 60000 do
        local h = host.crypto.sha256_hex(token .. ":" .. i)
        if string.sub(h, 1, 2) == "00" then
            return i, h
        end
    end
    return nil
end

-- solve_guard runs the handshake: GET target → 404 challenge page → pow →
-- POST /_v → trust cookie lands in the host's per-plugin cookie jar. Returns
-- true when the challenge was recognised and (apparently) solved.
function dle.solve_guard(site, body)
    -- pow_nonce only set on the challenge page; a normal 200 carries no
    -- "pow_nonce" token, so this is a no-op there.
    if not body or not string.find(body, "pow_nonce", 1, true) then
        return false
    end
    local tok = host.regex.find(body, [[token:\s*"([^"]+)"]])
    if not tok then return false end
    local nonce, hash = dle.pow_nonce(tok)
    if not nonce then return false end
    local form = "token=" .. host.text.url_encode(tok)
        .. "&mode=modern&workTime=55&iterations=110&hasCrypto=1&gms=-1&fi=1&rtc=98"
        .. "&rtip=" .. dle.client_ip()
        .. "&pow_nonce=" .. nonce .. "&pow_hash=" .. hash
        .. "&webdriver=0&touch=1&screen_w=1856&screen_h=1044&screen_cd=24"
        .. "&wgv=AMD&wgr=Radeon&tz=-420&dpr=1&cdp=0&cdpf="
    -- The host injects a Referer to the challenge URL and an Origin, both
    -- derived from the site, matching the form the browser sends.
    host.http.post(site .. "/_v", form, dle.headers({
        ["Origin"] = site,
    }))
    return true
end

-- client_ip is a placeholder the server does not actually validate: any
-- non-empty value is accepted, but an empty rtip escalates to Turnstile. A
-- stable fake keeps the handshake from being skipped.
function dle.client_ip()
    local ip = host.plugin.param("rtip")
    if ip and ip ~= "" then return ip end
    return "1.2.3.4"
end

-- get fetches site .. path and solves the guard if the response is the
-- 404 challenge page, returning the real body on the second attempt.
function dle.get(site, path)
    local h = dle.headers()
    local resp = host.http.get(site .. path, h)
    local body = resp and resp.body or ""
    if dle.solve_guard(site, body) then
        resp = host.http.get(site .. path, h)
        body = resp and resp.body or ""
    end
    return body, resp
end

-- json_blob_from extracts window.X = <json> (or the first <var ... = <json>;)
-- and decodes it, returning nil when the marker or payload is absent.
function dle.json_blob(html, marker)
    local blob = host.text.json_blob(html, marker)
    if not blob then return nil end
    local ok, data = pcall(host.json.decode, blob)
    if not ok or type(data) ~= "table" then return nil end
    return data
end

-- dle_date reformats "18.10.2023" (d.m.yyyy) to "2023-10-18", because
-- host.text.date_to_iso has no dotted layout in its accepted layouts.
function dle.dle_date(s)
    if not s then return "" end
    local d, m, y = string.match(s, "(%d+)%.(%d+)%.(%d+)")
    if not y then return "" end
    return string.format("%04d-%02d-%02dT00:00:00Z", tonumber(y), tonumber(m), tonumber(d))
end

-- slug_id pulls the numeric id and slug out of a DLE permalink
-- (e.g. https://site/x/y/29706-thor-1966-2011.html → 29706, thor-1966-2011).
function dle.slug_id(href)
    if not href then return nil, nil end
    local id, slug = string.match(href, "/(%d+)-([^/]-)%.html")
    return id and tonumber(id), slug
end

-- comic_series finds the ComicSeries node in a JSON-LD @graph block.
function dle.comic_series(html)
    local blocks = host.regex.find_all(html, [[(?s)<script type="application/ld\+json">(.*?)</script>]]) or {}
    for _, row in ipairs(blocks) do
        local ok, doc = pcall(host.json.decode, row[1])
        if ok and type(doc) == "table" and type(doc["@graph"]) == "table" then
            for _, node in ipairs(doc["@graph"]) do
                if type(node) == "table" and node["@type"] == "ComicSeries" then
                    return node
                end
            end
        end
    end
    return nil
end

-- reader_url builds the /reader/<news_id>/<chapter_id> URL the image
-- Referer must point at.
function dle.reader_url(site, news_id, chapter_id)
    return site .. "/reader/" .. tostring(news_id) .. "/" .. tostring(chapter_id)
end

return dle
