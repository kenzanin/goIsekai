-- views/history.lua
-- Reading history page. Replaces views/history.jet.
-- Called as: history(data) -> string (body HTML only, layout wraps it)
-- data.History: []database.HistoryEntry (see internal/database/history.go)

return function(data)
    local history = data.History or {}
    local parts = {}
    local function emit(s) parts[#parts + 1] = s end

    emit('<h1 class="text-xl font-semibold mb-6">History</h1>')

    if #history == 0 then
        emit('<div class="py-16 text-center text-neutral-500">')
        emit('    <p class="mb-2">No reading history yet</p>')
        emit('    <a href="/" class="text-indigo-400 hover:text-indigo-300 text-sm">Go to library</a>')
        emit('</div>')
    else
        emit('<div class="divide-y divide-neutral-800">')
        for _, h in ipairs(history) do
            local title = h.Title or ""
            local pluginID = h.PluginID or ""
            local sourceMangaID = h.SourceMangaID or ""
            local coverURL = h.CoverURL or ""
            local pluginIcon = h.PluginIcon or ""
            local pluginName = h.PluginName or ""
            local readChapters = h.ReadChapters or 0
            local totalChapters = h.TotalChapters or 0
            local lastReadAt = h.LastReadAt or ""
            local tsAttr = ""
            local formattedDate = ""
            if lastReadAt ~= "" then
                tsAttr = h(tostring(lastReadAt))
                formattedDate = h(formatDate(tostring(lastReadAt)))
            else
                formattedDate = "—"
            end

            emit('    <a href="/view/manga/' .. h(pluginID) .. '/' .. h(sourceMangaID) .. '" class="flex items-center gap-4 py-4 -mx-2 px-2 rounded hover:bg-neutral-900 transition">')

            if coverURL ~= "" then
                emit('        <img src="/image?pluginID=' .. h(pluginID) .. '&amp;url=' .. h(coverURL) .. '" alt="' .. h(title) .. '" class="w-16 aspect-[2/3] object-cover rounded shrink-0" loading="lazy">')
            else
                emit('        <div class="w-16 aspect-[2/3] bg-neutral-800 rounded flex items-center justify-center text-neutral-500 text-sm font-semibold shrink-0">' .. h(getInitials(title)) .. '</div>')
            end

            emit('        <div class="flex-1 min-w-0">')
            emit('          <div class="flex items-center justify-between">')
            emit('            <div class="text-sm font-medium truncate">' .. h(title) .. '</div>')
            emit('            <div class="text-xs text-neutral-500 shrink-0 hidden sm:block" data-ts="' .. tsAttr .. '" title="' .. formattedDate .. '">' .. formattedDate .. '</div>')
            emit('          </div>')
            emit('          <div class="flex items-center gap-2 mt-1">')
            emit('            <span class="text-xs text-neutral-400">' .. h(tostring(readChapters)) .. '/' .. h(tostring(totalChapters)) .. ' chapters</span>')
            emit('            <span class="inline-flex items-center gap-1.5 text-xs px-2 py-0.5 rounded-full bg-neutral-800 text-neutral-500">')
            if pluginIcon ~= "" then
                emit('<img src="' .. h(pluginIcon) .. '" alt="" class="h-3.5 w-3.5 rounded-sm object-cover">')
            end
            emit(h(pluginName))
            emit('</span>')
            emit('            <div class="text-xs text-neutral-500 sm:hidden" data-ts="' .. tsAttr .. '" title="' .. formattedDate .. '">' .. formattedDate .. '</div>')
            emit('          </div>')
            emit('        </div>')
            emit('    </a>')
        end
        emit('</div>')
        emit('<script>')
        emit('function refreshRelativeTimes() {')
        emit("  document.querySelectorAll('[data-ts]').forEach(el => {")
        emit('    const d = new Date(el.dataset.ts);')
        emit('    const diff = Date.now() - d.getTime();')
        emit('    const mins = Math.floor(diff/60000);')
        emit("    if (mins < 60) el.textContent = mins + 'm ago';")
        emit("    else if (mins < 1440) el.textContent = Math.floor(mins/60) + 'h ago';")
        emit("    else el.textContent = Math.floor(mins/1440) + 'd ago';")
        emit('  });')
        emit('}')
        emit('refreshRelativeTimes();')
        emit('setInterval(refreshRelativeTimes, 60000);')
        emit('</script>')
    end

    return table.concat(parts, '\n')
end
