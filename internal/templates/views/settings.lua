-- views/settings.lua
-- Settings page. Replaces views/settings.jet.
-- Called as: settings(data) -> string (body HTML only, layout wraps it)
-- data.Config: *config.Config, data.Path: string, data.CacheBytes: int64

return function(data)
    local cfg = data.Config
    local path = data.Path or ""
    local cacheBytes = data.CacheBytes or 0

    local parts = {}
    local function emit(s) parts[#parts + 1] = s end

    emit('<h1 class="text-xl font-semibold mb-1">Settings</h1>')
    emit('<p class="text-xs text-neutral-500 mb-6">Config: <code class="text-xs bg-neutral-800 rounded px-1.5 py-0.5">' .. h(path) .. '</code></p>')

    if cfg then
        -- Server section
        emit('<form method="post" action="/action/save-settings" class="space-y-4 max-w-xl">')
        emit('    <div class="border border-neutral-800 rounded-lg p-4">')
        emit('        <h2 class="text-sm font-medium text-neutral-300 mb-3">Server</h2>')
        emit('        <div class="space-y-4">')
        emit('            <div>')
        emit('                <label for="title" class="block text-sm font-medium mb-1">Title</label>')
        emit('                <input id="title" name="title" type="text" value="' .. h(cfg.Title or "") .. '" class="w-full bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 text-sm">')
        emit('            </div>')
        emit('            <div class="grid grid-cols-2 gap-4">')
        emit('                <div>')
        emit('                    <label for="host" class="block text-sm font-medium mb-1">Host</label>')
        emit('                    <input id="host" name="host" type="text" value="' .. h(cfg.Host or "") .. '" class="w-full bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 text-sm">')
        emit('                </div>')
        emit('                <div>')
        emit('                    <label for="port" class="block text-sm font-medium mb-1">Port</label>')
        emit('                    <input id="port" name="port" type="number" value="' .. h(tostring(cfg.Port or "")) .. '" class="w-full bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 text-sm">')
        emit('                </div>')
        emit('            </div>')
        emit('            <div>')
        emit('                <label for="log_level" class="block text-sm font-medium mb-1">Log Level</label>')
        emit('                <select id="log_level" name="log_level" class="w-full bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 text-sm">')
        local logLevel = cfg.LogLevel or ""
        emit('                    <option value="debug"' .. (logLevel == "debug" and ' selected' or '') .. '>debug</option>')
        emit('                    <option value="info"' .. (logLevel == "info" and ' selected' or '') .. '>info</option>')
        emit('                    <option value="warning"' .. (logLevel == "warning" and ' selected' or '') .. '>warning</option>')
        emit('                </select>')
        emit('            </div>')
        emit('        </div>')
        emit('    </div>')

        -- HTTP Client section
        emit('    <div class="border border-neutral-800 rounded-lg p-4">')
        emit('        <h2 class="text-sm font-medium text-neutral-300 mb-3">HTTP Client</h2>')
        emit('        <div class="space-y-4">')
        emit('            <div>')
        emit('                <label for="user_agent" class="block text-sm font-medium mb-1">User Agent</label>')
        emit('                <input id="user_agent" name="user_agent" type="text" value="' .. h(cfg.UserAgent or "") .. '" class="w-full bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 text-sm">')
        emit('            </div>')
        emit('            <div>')
        emit('                <label for="accept_language" class="block text-sm font-medium mb-1">Accept Language</label>')
        emit('                <input id="accept_language" name="accept_language" type="text" value="' .. h(cfg.AcceptLanguage or "") .. '" class="w-full bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 text-sm">')
        emit('            </div>')
        emit('            <div>')
        emit('                <label for="referer" class="block text-sm font-medium mb-1">Referer</label>')
        emit('                <input id="referer" name="referer" type="text" value="' .. h(cfg.Referer or "") .. '" class="w-full bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 text-sm">')
        emit('            </div>')
        emit('        </div>')
        emit('    </div>')

        -- Reader section
        emit('    <div class="border border-neutral-800 rounded-lg p-4">')
        emit('        <h2 class="text-sm font-medium text-neutral-300 mb-3">Reader</h2>')
        emit('        <div class="space-y-4">')
        emit('            <div>')
        emit('                <label for="read_ahead" class="block text-sm font-medium mb-1">Read Ahead (pages to prefetch)</label>')
        emit('                <input id="read_ahead" name="read_ahead" type="number" min="0" max="10" value="3" class="w-full bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 text-sm">')
        emit('                <span class="text-xs text-neutral-500">(stored in this browser)</span>')
        emit('            </div>')
        emit('        </div>')
        emit('    </div>')

        emit('    <button type="submit" class="bg-indigo-600 hover:bg-indigo-500 text-white rounded-md px-4 py-2 text-sm font-medium">Save</button>')
        emit('</form>')

        -- Image Cache section
        emit('<div class="max-w-xl mt-10 p-4 border border-neutral-800 rounded-lg">')
        emit('    <h2 class="text-sm font-semibold mb-1">Image Cache</h2>')
        emit('    <p class="text-xs text-neutral-400 mb-3">Cached images: ' .. h(formatBytes(cacheBytes)) .. '</p>')
        emit('    <form method="post" action="/action/clear-cache-all" data-confirm="Delete ALL cached images?">')
        emit('        <button type="submit" class="bg-neutral-700 hover:bg-red-600 text-white rounded-md px-4 py-2 text-sm font-medium">🗑 Clear all cached images</button>')
        emit('    </form>')
        emit('</div>')

        -- read_ahead localStorage script
        emit('<script>')
        emit('(function () {')
        emit("  var ra = document.getElementById('read_ahead');")
        emit("  ra.value = localStorage.getItem('gi_readAhead') || '3';")
        emit("  ra.closest('form').addEventListener('submit', function () {")
        emit("    localStorage.setItem('gi_readAhead', ra.value);")
        emit('  });')
        emit('})();')
        emit('</script>')
    else
        emit('<div class="py-16 text-center text-neutral-500">Failed to load configuration</div>')
    end

    return table.concat(parts, '\n')
end
