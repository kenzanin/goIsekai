-- .luacheckrc — Lua template lint config
-- Templates embed HTML + Lua, so line length and string-embedded var usage
-- are expected. Runtime globals (h, formatDate, etc.) are injected by lua_engine.

max_line_length = 800
exclude_files = {
    "internal/templates/partials/toast.lua",
}
read_globals = {
    "h",
    "host",
    "formatDate",
    "getInitials",
    "formatBytes",
    "formatChapterNum",
    "pageURL",
    "pageWindow",
}
