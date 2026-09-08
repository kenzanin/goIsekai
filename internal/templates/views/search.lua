-- views/search.lua
-- Search form and results grid. Replaces views/search.jet.
-- Called as: search(data) -> string (body HTML only, layout wraps it)

local pagination = require("partials.pagination")

return function(data)
    local q = data.Q or ""
    local pluginID = data.PluginID or ""
    local pluginName = data.PluginName or ""
    local pluginIcon = data.PluginIcon or ""
    local results = data.Results or {}
    local page = data.Page or 1
    local totalPages = data.TotalPages or 1
    local thumbRatio = data.ThumbRatio or 0
    local challenge = data.Challenge or false
    local plugins = data.Plugins or {}

    local parts = {}
    local function emit(s) parts[#parts + 1] = s end

    emit('<h1 class="text-xl font-semibold mb-6">Search</h1>')

    -- Search form
    emit('<form method="get" action="/view/search" class="flex gap-2 items-end mb-8">')
    emit('    <div>')
    emit('        <label for="pluginID" class="block text-xs text-neutral-400 mb-1">Plugin')
    if pluginIcon ~= "" then
        emit(' <img src="' .. h(pluginIcon) .. '" alt="" class="inline h-3.5 w-3.5 rounded-sm object-cover align-[-1px]">')
    end
    if pluginName ~= "" then
        emit(' <span class="text-neutral-500 font-normal">· ' .. h(pluginName) .. '</span>')
    end
    emit('</label>')
    emit('        <select id="pluginID" name="pluginID" class="bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 text-sm">')
    for _, p in ipairs(plugins) do
        if p.IsActive then
            local selected = ""
            if pluginID == p.ID then
                selected = " selected"
            end
            emit('            <option value="' .. h(p.ID) .. '"' .. selected .. '>' .. h(p.Name) .. '</option>')
        end
    end
    emit('        </select>')
    emit('    </div>')
    emit('    <div class="flex-1 max-w-md">')
    emit('        <label for="q" class="block text-xs text-neutral-400 mb-1">Query</label>')
    emit('        <input id="q" name="q" type="text" value="' .. h(q) .. '" placeholder="Manga title..." class="w-full bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 text-sm">')
    emit('    </div>')
    emit('    <button type="submit" class="bg-indigo-600 hover:bg-indigo-500 text-white rounded-md px-4 py-2 text-sm font-medium">Search</button>')
    emit('</form>')

    -- Results
    if #results > 0 then
        emit('<div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-6 gap-4">')
        for _, r in ipairs(results) do
            local title = r.Title or ""
            local coverURL = r.CoverURL or ""
            local mangaID = r.ID or ""

            emit('    <a href="/view/manga/' .. h(pluginID) .. '/' .. h(mangaID) .. '" class="bg-neutral-900 rounded-lg overflow-hidden hover:-translate-y-0.5 hover:shadow-lg hover:shadow-black/40 hover:ring-1 hover:ring-indigo-500 transition">')

            if coverURL ~= "" then
                if thumbRatio > 0 then
                    emit('        <img src="/image?pluginID=' .. h(pluginID) .. '&amp;url=' .. h(coverURL) .. '" alt="' .. h(title) .. '" style="aspect-ratio: ' .. h(tostring(thumbRatio)) .. '" class="w-full object-cover" loading="lazy">')
                else
                    emit('        <img src="/image?pluginID=' .. h(pluginID) .. '&amp;url=' .. h(coverURL) .. '" alt="' .. h(title) .. '" class="w-full aspect-[2/3] object-cover" loading="lazy">')
                end
            else
                emit('        <div class="w-full aspect-[2/3] bg-neutral-800 flex items-center justify-center text-neutral-500 text-2xl font-semibold">' .. h(getInitials(title)) .. '</div>')
            end

            emit('        <div class="p-3">')
            emit('            <div class="text-sm font-medium line-clamp-2" title="' .. h(title) .. '">' .. h(title) .. '</div>')
            if pluginName ~= "" then
                emit('            <div class="mt-1.5">')
                emit('                <span class="inline-flex items-center gap-1.5 text-[10px] px-2 py-0.5 rounded-full bg-neutral-800 text-neutral-500">')
                if pluginIcon ~= "" then
                    emit('<img src="' .. h(pluginIcon) .. '" alt="" class="h-3.5 w-3.5 rounded-sm object-cover">')
                end
                emit(h(pluginName))
                emit('</span>')
                emit('            </div>')
            end
            emit('        </div>')
            emit('    </a>')
        end
        emit('</div>')

        -- Pagination
        if page and page > 1 then
            emit(pagination({
                Pagination = {
                    Base = "/view/search",
                    Param = "page",
                    Current = page,
                    Total = totalPages,
                    Extra = { "q", q, "pluginID", pluginID },
                },
            }))
        elseif totalPages > 1 then
            emit(pagination({
                Pagination = {
                    Base = "/view/search",
                    Param = "page",
                    Current = page,
                    Total = totalPages,
                    Extra = { "q", q, "pluginID", pluginID },
                },
            }))
        end
    else
        if q ~= "" then
            emit('<div class="py-16 text-center text-neutral-500">No results for "' .. h(q) .. '"</div>')
        end
    end

    -- Challenge warning
    if challenge then
        emit('<div class="bg-amber-500/15 border border-amber-500/40 text-amber-300 rounded-lg p-4 mb-6">⚠ This site needs human verification — go to <a href="/view/plugins" class="underline">Plugins &rarr; Human Verification</a> to paste your session cookies.</div>')
    end

    return table.concat(parts, '\n')
end
