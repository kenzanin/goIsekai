-- helpers.lua — generic goIsekai Lua-plugin helpers.
-- Copy this file into any plugin folder unchanged. It pre-executes before
-- main.lua and registers the globals below. Order-independent: siblings may
-- call these at call time; no require() needed from other modules.
--
-- Globals provided:
--   normalizeStatus(s)  raw status -> canonical (Ongoing/Completed/Hiatus/
--                       Dropped/Upcoming); unknown passes through, empty -> "unknown"
--   url_encode(s)       percent-encode for query strings
--   lua_escape(s)       escape Lua pattern magic chars for string.match/gsub
--   decode_entities(s)  HTML entity decode (&amp; &#039; &quot; &apos; …)
--   titlecase(s)        first character to upper
--   http_get(url, opts) GET wrapper over http_request with logging
--
-- Site globals read at call time (optional; set them in main.lua):
--   UA    browser user-agent  (defaults to a generic Chrome UA below)
--   BASE  site origin used as the Referer for JSON calls (none when unset)
--
-- http_get opts (table; pass `true` as a shorthand for {json=true}):
--   json      bool   -> Accept: application/json
--   referer   string|false -> override the Referer (false = send none)
--   headers   table  -> extra headers merged last (wins over defaults)
--
-- If your plugin ships its own util.lua defining these names, drop yours or
-- merge — this file pre-executes and later-loaded definitions win in the VM.
-- Only sandbox globals are used (http_request, log); helpers has no deps.

-- normalizeStatus maps a raw status string to a canonical host value.
-- Canonical set: Ongoing, Completed, Hiatus, Dropped, Upcoming.
-- Unknown values pass through as-is.
function normalizeStatus(s)
    if not s or s == "" then return "unknown" end
    local raw = s:lower()
    if raw:find("ongo") or raw:find("releas") or raw:find("publish") then return "Ongoing" end
    if raw:find("complet") or raw:find("finish") then return "Completed" end
    if raw:find("hiatus") or raw:find("on.?hold") or raw:find("onhold") then return "Hiatus" end
    if raw:find("drop") or raw:find("cancel") then return "Dropped" end
    if raw:find("upcom") or raw:find("not.?publish") then return "Upcoming" end
    return s
end

-- url_encode percent-encodes everything outside unreserved chars.
function url_encode(s)
    return (s:gsub("([^%w%-%.%_%~])", function(c)
        return string.format("%%%02X", string.byte(c))
    end))
end

-- lua_escape escapes Lua pattern magic chars in literals interpolated into
-- patterns: [ - . + [ ] ( ) $ ^ % ? *
function lua_escape(s)
    return (s:gsub("[%-%.%+%[%]%(%)%$%^%%%?%*]", "%%%0"))
end

-- decode_entities decodes the common HTML entities. Also unwraps the
-- double-escaped forms some sites emit in <meta> tags ("&amp;#039;").
function decode_entities(s)
    if not s then return "" end
    s = s:gsub("&amp;#0?39;", "'"):gsub("&amp;quot;", '"')
    s = s:gsub("&#0?39;", "'"):gsub("&apos;", "'")
    s = s:gsub("&quot;", '"')
    s = s:gsub("&amp;", "&")
    return s
end

-- titlecase uppercases the first character only.
function titlecase(s)
    if s == nil then return "" end
    return s:sub(1, 1):upper() .. s:sub(2)
end

local defaultUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36"

-- http_get issues GET via the sandbox http_request global, logging failures.
-- Uses the global UA when the plugin set one, else defaultUA; JSON calls get
-- a Referer of BASE .. "/" unless overridden via opts.referer.
function http_get(url, opts)
    local o = opts
    if type(o) ~= "table" then
        o = (o == true) and {json = true} or {}
    end
    local headers = {
        ["User-Agent"] = UA or defaultUA,
        ["Accept"] = "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
        ["Accept-Language"] = "en-US,en;q=0.9"
    }
    if o.json then
        headers["Accept"] = "application/json"
    end
    if o.referer ~= false then
        local ref = o.referer
        if ref == nil and BASE ~= nil then ref = BASE .. "/" end
        if ref ~= nil and ref ~= "" then headers["Referer"] = ref end
    end
    if o.headers then
        for k, v in pairs(o.headers) do headers[k] = v end
    end
    local resp = http_request({url = url, method = "GET", headers = headers})
    if not resp then
        log.error("http_request returned nil for " .. url)
    elseif resp.status ~= 200 then
        log.error("http status " .. tostring(resp.status) .. " for " .. url)
    end
    return resp
end
