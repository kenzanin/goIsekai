-- helpers.lua — generic goIsekai Lua-plugin helpers.
-- Copy this file into any plugin folder unchanged. It pre-executes before
-- main.lua and registers the globals below. Order-independent: siblings may
-- call these at call time; no require() needed from other modules.
--
-- Globals provided:
--   normalizeStatus(s)  raw status -> canonical (Ongoing/Completed/Hiatus/
--                       Dropped/Upcoming); unknown passes through, empty -> "unknown"
--   http_get(url, opts) GET wrapper over host.http.get with logging
--
-- The escaping and status vocabulary live in the host (host.text.*); only the
-- site-shaped bits stay here.
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
-- Only sandbox globals are used (host, log); helpers has no deps.

-- normalizeStatus maps a raw status string to a canonical host value.
-- Canonical set: Ongoing, Completed, Hiatus, Dropped, Upcoming.
-- Unknown values pass through as-is.
function normalizeStatus(s)
    if not s or s == "" then return "unknown" end
    return host.text.normalize_status(s)
end

local defaultUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36"

-- http_get issues GET via host.http.get, which logs transport failures and
-- non-2xx responses. Uses the global UA when the plugin set one, else
-- defaultUA; JSON calls get a Referer of BASE .. "/" unless overridden via
-- opts.referer.
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
    return host.http.get(url, headers)
end
