-- util.lua — MangaDex shared helpers (Lua / Lunar VM).
-- Loaded via require("util") from main.lua. Keep this file dependency-free so
-- it can be copied into another plugin unchanged.

local util = {}

util.API_URL = "https://api.mangadex.org"
util.CDN_URL = "https://uploads.mangadex.org"
util.LANG = "en"

-- MangaDex image endpoints reject requests without a Referer.
function util.http_get(url, headers)
    local h = headers or {}
    h["Referer"] = util.CDN_URL .. "/"
    return host.http.get(url, h)
end

-- first_locale returns m[LANG] when present, else any non-empty value. MangaDex
-- localisation maps are unordered, so "first" means "first found", and the
-- English entry is preferred explicitly before falling back.
function util.first_locale(m)
    if not m then return "" end
    if m[util.LANG] then return m[util.LANG] end
    for _, v in pairs(m) do
        if v and v ~= "" then return v end
    end
    return ""
end

-- altTitles is a list of single-key localisation maps, and most entries carry
-- no English at all, so every map is probed before giving up.
function util.first_title(attrs)
    attrs = attrs or {}
    local t = attrs.title and attrs.title[util.LANG]
    if t then return t end
    for _, at in ipairs(attrs.altTitles or {}) do
        if at[util.LANG] then return at[util.LANG] end
    end
    for _, at in ipairs(attrs.altTitles or {}) do
        local v = util.first_locale(at)
        if v ~= "" then return v end
    end
    return util.first_locale(attrs.title)
end

function util.cover_url(md)
    for _, r in ipairs(md.relationships or {}) do
        if r.type == "cover_art" and r.attributes and r.attributes.fileName then
            return util.CDN_URL .. "/covers/" .. md.id .. "/" .. r.attributes.fileName .. ".256.jpg"
        end
    end
    return ""
end

function util.normalize_status(raw)
    local mapped = host.text.normalize_status(raw or "")
    if not mapped or mapped == "" then return "unknown" end
    return mapped
end

function util.to_manga(md)
    local attrs = md.attributes or {}
    local genres = {}
    for _, tag in ipairs(attrs.tags or {}) do
        local name = util.first_locale(tag.attributes and tag.attributes.name)
        if name ~= "" then genres[#genres + 1] = name end
    end
    return {
        id = md.id,
        title = util.first_title(attrs),
        cover_url = util.cover_url(md),
        description = util.first_locale(attrs.description),
        status = util.normalize_status(attrs.status),
        genres = genres,
    }
end

-- MangaDex sends chapter/volume as strings and leaves them null for oneshots.
function util.number(s)
    return tonumber(s) or 0
end

-- The API hides these ratings unless they are named explicitly.
function util.content_rating_params()
    return "contentRating[]=safe&contentRating[]=suggestive&contentRating[]=erotica"
end

return util
