-- .luacheckrc — Lua template lint config
-- Templates embed HTML + Lua, so line length and string-embedded var usage
-- are expected. Runtime globals (h, formatDate, etc.) are injected by lua_engine.

-- This list MUST mirror the helpers registered in
-- internal/templates/lua_helpers.go:luaHelpers. A helper added there without
-- being declared here shows up as W113 "accessing undefined variable" at every
-- call site, which is how `ue` went missing.
max_line_length = 800
exclude_files = {
    "internal/templates/partials/toast.lua",
}
read_globals = {
    "h",
    "ue",
    "csrfInput",
    "host",
    "formatDate",
    "getInitials",
    "formatBytes",
    "formatChapterNum",
    "pageURL",
    "pageWindow",
}
