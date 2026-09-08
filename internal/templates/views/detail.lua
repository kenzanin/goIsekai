-- views/detail.lua
-- Manga detail page: cover, metadata, alt-titles, chapters with progress.
-- Replaces views/detail.jet.
-- Called as: detail(data) -> string (body HTML only, layout wraps it)
-- data.PluginID, data.PluginName, data.PluginIcon, data.MangaID
-- data.Manga: types.Manga, data.AltTitles: []AltTitle, data.CurrentTitle: string
-- data.AltTitleServers: []AltTitleServerEntry
-- data.Chapters: []types.Chapter, data.Progress: map[string]database.ChapterProgress
-- data.Continue: *ContinuePoint, data.InLibrary: bool, data.Challenge: bool
-- data.ChCurrentPage/ChTotalPages: int

local pagination = require("partials.pagination")

return function(data)
    local pluginID = data.PluginID or ""
    local pluginName = data.PluginName or ""
    local pluginIcon = data.PluginIcon or ""
    local mangaID = data.MangaID or ""
    local manga = data.Manga or {}
    local altTitles = data.AltTitles or {}
    local currentTitle = data.CurrentTitle or manga.Title or ""
    local altTitleServers = data.AltTitleServers or {}
    local altSummaries = data.AltSummaries or {}
    local altSummaryServers = data.AltSummaryServers or {}
    local chapters = data.Chapters or {}
    local progress = data.Progress or {}
    local continuePoint = data.Continue
    local inLibrary = data.InLibrary or false
    local challenge = data.Challenge or false
    local chPage = data.ChCurrentPage or 1
    local chTotalPages = data.ChTotalPages or 1
    local parts = {}
    local function emit(s) parts[#parts + 1] = s end

    -- Back button
    emit('<div class="mb-4">')
    emit('    <button onclick="if (history.length > 1) { history.back(); } else { window.location.href = \'/view/library\'; }" class="inline-flex items-center gap-1.5 text-sm text-neutral-400 hover:text-neutral-200 transition" aria-label="Back">')
    emit('        <svg xmlns="http://www.w3.org/2000/svg" class="size-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m15 18-6-6 6-6"/></svg>')
    emit('        Back')
    emit('    </button>')
    emit('</div>')

    -- Challenge warning
    if challenge then
        emit('<div class="bg-amber-500/15 border border-amber-500/40 text-amber-300 rounded-lg p-4 mb-6">This site needs human verification — go to <a href="/view/plugins" class="underline">Plugins → Human Verification</a> to paste your session cookies.</div>')
    end

    -- Manga header grid
    emit('<div class="grid grid-cols-1 md:grid-cols-4 gap-6 mb-8">')
    emit('    <div class="md:col-span-1">')
    if manga.CoverURL and manga.CoverURL ~= "" then
        emit('        <img src="/image?pluginID=' .. h(pluginID) .. '&amp;url=' .. h(manga.CoverURL) .. '" alt="' .. h(manga.Title or "") .. '" class="w-full md:max-w-[200px] aspect-[2/3] rounded-lg object-cover">')
    else
        emit('        <div class="w-full md:max-w-[200px] aspect-[2/3] bg-neutral-800 rounded-lg flex items-center justify-center text-neutral-500 text-4xl font-semibold">' .. h(getInitials(manga.Title or "")) .. '</div>')
    end
    emit('    </div>')
    emit('    <div class="md:col-span-3">')
    emit('        <h1 class="text-2xl font-semibold mb-2">' .. h(manga.Title or "") .. '</h1>')

    -- Plugin badge + status + genres
    emit('        <div class="flex flex-wrap gap-2 mb-3">')
    emit('            <span class="inline-flex items-center gap-1.5 text-xs px-2 py-0.5 rounded-full bg-neutral-800 text-neutral-500">')
    if pluginIcon ~= "" then
        emit('                <img src="' .. h(pluginIcon) .. '" alt="" class="h-3.5 w-3.5 rounded-sm object-cover">')
    end
    emit('                ' .. h(pluginName))
    emit('            </span>')
    if manga.Status and manga.Status ~= "" then
        emit('            <span class="text-xs px-2 py-0.5 rounded-full bg-neutral-700/50 text-neutral-400">' .. h(manga.Status) .. '</span>')
    end
    local genres = manga.Genres or {}
    for _, g in ipairs(genres) do
        emit('            <span class="text-xs px-2 py-0.5 rounded-full bg-indigo-500/15 text-indigo-400">' .. h(g) .. '</span>')
    end
    emit('        </div>')

    -- Author
    if manga.Author and manga.Author ~= "" then
        emit('        <p class="text-sm text-neutral-400 mb-2">' .. h(manga.Author) .. '</p>')
    end

    -- Alternative titles
    emit('        <div class="mb-4">')
    emit('            <div class="flex items-center gap-2 mb-1.5">')
    emit('                <span class="text-xs font-semibold text-neutral-300 uppercase tracking-wide">Alternative titles</span>')
    emit('            </div>')
    if #altTitles > 0 then
        emit('            <div class="flex flex-wrap gap-1.5 mb-2">')
        -- Current title badge
        emit('                <span class="inline-flex items-center gap-1.5 rounded-full bg-neutral-800 border border-indigo-700/60 pl-2.5 pr-1 py-0.5 text-xs">')
        emit('                    <span class="text-indigo-300" title="Current main title">' .. h(currentTitle) .. '</span>')
        emit('                    <span class="text-[10px] px-1.5 py-0.5 rounded-full bg-indigo-900/40 text-indigo-300">via ' .. h(pluginID) .. '</span>')
        emit('                </span>')
        for _, t in ipairs(altTitles) do
            emit('                <span class="inline-flex items-center gap-1.5 rounded-full bg-neutral-800 border border-neutral-700/60 pl-2.5 pr-1 py-0.5 text-xs">')
            emit('                    <form method="post" action="/action/set-title/' .. h(pluginID) .. '/' .. h(mangaID) .. '" class="inline">')
            emit('                        <input type="hidden" name="title" value="' .. h(t.Title or "") .. '">')
            emit('                        <button type="submit" title="Set as main title" class="text-neutral-300 hover:text-indigo-300" data-confirm="Set as main title?">' .. h(t.Title or "") .. '</button>')
            emit('                    </form>')
            if t.Source and t.Source ~= "" then
                emit('                    <span class="text-[10px] px-1.5 py-0.5 rounded-full bg-neutral-700/50 text-neutral-400">via ' .. h(t.Source) .. '</span>')
            end
            emit('                    <form method="post" action="/action/remove-alt-title/' .. h(pluginID) .. '/' .. h(mangaID) .. '" class="inline-flex">')
            emit('                        <input type="hidden" name="title" value="' .. h(t.Title or "") .. '">')
            emit('                        <button type="submit" title="Remove alternative title" aria-label="Remove" class="size-4 inline-flex items-center justify-center rounded-full text-neutral-500 hover:text-red-400 hover:bg-neutral-700" data-confirm="Are you sure?">&times;</button>')
            emit('                    </form>')
            emit('                </span>')
        end
        emit('            </div>')
    end

    -- Fetch alt-titles form
    if #altTitleServers > 0 then
        emit('            <form method="post" action="/action/fetch-alt-titles/' .. h(pluginID) .. '/' .. h(mangaID) .. '" class="flex flex-wrap items-center gap-2">')
        emit('                <select name="server" class="bg-neutral-900 border border-neutral-700 rounded-md px-2 py-1.5 text-xs text-neutral-300 focus:outline-none focus:border-indigo-500">')
        for _, s in ipairs(altTitleServers) do
            emit('                    <option value="' .. h(s.ServerID or "") .. '">' .. h(s.Name or "") .. '</option>')
        end
        emit('                </select>')
        emit('                <button type="submit" class="border border-neutral-700 hover:bg-neutral-800 rounded-md px-3 py-1.5 text-xs font-medium inline-flex items-center gap-1.5">')
        emit('                    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="size-3.5"><circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/></svg>')
        emit('                    Get alternative titles')
        emit('                </button>')
        emit('            </form>')
    else
        emit('            <p class="text-xs text-neutral-500">No alt-title providers available — install a plugin that declares alt-title servers.</p>')
    end
    emit('        </div>')

    -- Origin summary (from the source plugin — never deletable)
    if manga.Description and manga.Description ~= "" then
        emit('        <div class="mb-4 border-t border-neutral-800 pt-4">')
        emit('            <span class="text-xs font-semibold text-neutral-300 uppercase tracking-wide">Summary</span>')
        emit('            <p class="text-sm text-neutral-400 mt-1.5">' .. h(manga.Description) .. '</p>')
        emit('        </div>')
    end

    -- Alternative summaries (mirror of alternative titles; origin summary lives in mangas.description)
    emit('        <div class="mb-4 border-t border-neutral-800 pt-4">')
    emit('            <div class="flex items-center gap-2 mb-1.5">')
    emit('                <span class="text-xs font-semibold text-neutral-300 uppercase tracking-wide">Alternative summaries</span>')
    emit('            </div>')
    if #altSummaries > 0 then
        for _, a in ipairs(altSummaries) do
            emit('            <div class="flex items-start justify-between gap-2 py-1">')
            emit('                <div class="text-sm text-neutral-400 flex-1 min-w-0">' .. h(a.Description or "") .. '</div>')
            emit('                <div class="flex items-center gap-1.5 shrink-0">')
            if a.Source and a.Source ~= "" then
                emit('                    <span class="text-[10px] px-1.5 py-0.5 rounded-full bg-neutral-700/50 text-neutral-400">via ' .. h(a.Source) .. '</span>')
            end
            emit('                    <form method="post" action="/action/set-summary/' .. h(pluginID) .. '/' .. h(mangaID) .. '" class="inline-flex">')
            emit('                        <input type="hidden" name="description" value="' .. h(a.Description or "") .. '">')
            emit('                        <button type="submit" title="Use as main summary" class="text-[10px] px-1.5 py-0.5 rounded bg-indigo-900/40 text-indigo-300 hover:bg-indigo-800/50" data-confirm="Use this as the main summary?">Use</button>')
            emit('                    </form>')
            emit('                    <form method="post" action="/action/remove-alt-summary/' .. h(pluginID) .. '/' .. h(mangaID) .. '" class="inline-flex">')
            emit('                        <input type="hidden" name="description" value="' .. h(a.Description or "") .. '">')
            emit('                        <button type="submit" title="Remove alternative summary" aria-label="Remove" class="size-4 inline-flex items-center justify-center rounded-full text-neutral-500 hover:text-red-400 hover:bg-neutral-700" data-confirm="Remove this alternative summary?">&times;</button>')
            emit('                    </form>')
            emit('                </div>')
            emit('            </div>')
        end
    end

    -- Fetch alt-summaries form
    if #altSummaryServers > 0 then
        emit('            <form method="post" action="/action/fetch-alt-summaries/' .. h(pluginID) .. '/' .. h(mangaID) .. '" class="flex flex-wrap items-center gap-2 mt-1">')
        emit('                <select name="server" class="bg-neutral-900 border border-neutral-700 rounded-md px-2 py-1.5 text-xs text-neutral-300 focus:outline-none focus:border-indigo-500">')
        for _, s in ipairs(altSummaryServers) do
            emit('                    <option value="' .. h(s.ServerID or "") .. '">' .. h(s.Name or "") .. '</option>')
        end
        emit('                </select>')
        emit('                <button type="submit" class="border border-neutral-700 hover:bg-neutral-800 rounded-md px-3 py-1.5 text-xs font-medium inline-flex items-center gap-1.5">')
        emit('                    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="size-3.5"><circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/></svg>')
        emit('                    Get alternative summaries')
        emit('                </button>')
        emit('            </form>')
    else
        emit('            <p class="text-xs text-neutral-500">No alt-summary providers available — install a plugin that declares alt-summary servers.</p>')
    end
    emit('        </div>')

    -- Action buttons: library + continue reading
    emit('        <div class="flex flex-wrap items-center gap-2 mb-4">')
    emit('            <form method="post" action="/action/toggle-library/' .. h(pluginID) .. '/' .. h(mangaID) .. '">')
    if inLibrary then
        emit('                <button type="submit" class="border border-emerald-600/50 text-emerald-400 bg-emerald-500/10 hover:bg-emerald-500/20 rounded-md px-4 py-2 text-sm">✓ In Library</button>')
    else
        emit('                <button type="submit" class="bg-indigo-600 hover:bg-indigo-500 text-white rounded-md px-4 py-2 text-sm font-medium">+ Add to Library</button>')
    end
    emit('            </form>')
    if continuePoint then
        local cp = continuePoint
        local label, titleAttr
        if cp.Page and cp.Page > 1 then
            label = '▶ Continue · Ch.' .. formatChapterNum(cp.ChapterN)
            titleAttr = 'Continue: Ch. ' .. formatChapterNum(cp.ChapterN) .. ', page ' .. tostring(cp.Page)
        elseif cp.Started then
            label = '▶ Continue · Ch.' .. formatChapterNum(cp.ChapterN)
            titleAttr = 'Continue: Ch. ' .. formatChapterNum(cp.ChapterN)
        else
            label = '▶ Start Reading'
            titleAttr = 'Start Reading'
        end
        emit('            <a href="/view/read/' .. h(pluginID) .. '/' .. h(mangaID) .. '/' .. h(cp.ChapterID or "") .. '?page=' .. tostring(cp.Page or 1) .. '" class="bg-indigo-600 hover:bg-indigo-500 text-white rounded-md px-4 py-2 text-sm font-medium" title="' .. h(titleAttr) .. '">' .. h(label) .. '</a>')
    end
    emit('        </div>')
    emit('    </div>')
    emit('</div>')

    -- Chapters section
    emit('<h2 class="text-xl font-semibold mb-4">Chapters</h2>')
    if #chapters == 0 then
        emit('<div class="py-16 text-center text-neutral-500">No chapters yet</div>')
    else
        -- Bulk actions toolbar
        emit('<div class="flex flex-wrap items-center gap-2 mb-4">')
        emit('    <form id="mark-bulk" method="post" action="/action/mark-read-bulk">')
        emit('        <input type="hidden" name="pluginID" value="' .. h(pluginID) .. '">')
        emit('        <input type="hidden" name="mangaID" value="' .. h(mangaID) .. '">')
        emit('        <button type="submit" class="bg-indigo-600 hover:bg-indigo-500 text-white rounded-md px-3 py-1.5 text-sm font-medium">Mark selected as read</button>')
        emit('    </form>')
        emit('    <form method="post" action="/action/reset-progress-all/' .. h(pluginID) .. '/' .. h(mangaID) .. '">')
        emit('        <button type="submit" title="Clear read progress for every chapter" class="bg-transparent border border-red-900/60 text-red-400 hover:bg-red-950/40 rounded-md px-3 py-1.5 text-sm" data-confirm="Are you sure?">↺ Reset all read status</button>')
        emit('    </form>')
        emit('    <form method="post" action="/action/clear-cache/' .. h(pluginID) .. '/' .. h(mangaID) .. '" data-confirm="Clear cached images for this manga?">')
        emit('        <button type="submit" title="Delete cached image files for this manga" class="bg-transparent border border-red-900/60 text-red-400 hover:bg-red-950/40 rounded-md px-3 py-1.5 text-sm">🗑 Clear cached images</button>')
        emit('    </form>')
        emit('</div>')

        -- Chapter pagination (top)
        emit(pagination({ Pagination = { Base = "", Param = "ChPage", Current = chPage, Total = chTotalPages } }))

        -- Chapter list
        local firstID = ""
        emit('<div class="divide-y divide-neutral-800">')
        for i, c in ipairs(chapters) do
            local cID = c.ID or ""
            if i == 1 then firstID = cID end
            local cTitle = c.Title or ""
            local chapterNum = c.ChapterNum or 0
            local p = progress[cID] or {}
            local isDone = p.Done or false
            local totalPages = p.TotalPages or 0
            local lastPageRead = p.LastPageRead or 0
            local cachedPages = p.CachedPages or 0

            emit('    <div class="flex items-center gap-3 py-3 px-2 -mx-2 rounded hover:bg-neutral-900 transition">')
            emit('        <input type="checkbox" name="chapterIDs" value="' .. h(cID) .. '" form="mark-bulk" class="size-4 accent-indigo-600 shrink-0">')
            emit('        <a href="/view/read/' .. h(pluginID) .. '/' .. h(mangaID) .. '/' .. h(cID) .. '" class="flex-1 flex items-center justify-between gap-4 min-w-0">')
            emit('            <span class="text-sm">')
            emit('                <span class="text-neutral-400 mr-2">Ch. ' .. h(formatChapterNum(chapterNum)) .. '</span>')
            if isDone then
                emit('                <span class="line-through text-neutral-500">' .. h(cTitle) .. '</span>')
            else
                emit('                ' .. h(cTitle))
            end
            emit('            </span>')
            emit('            <span class="flex items-center gap-2 shrink-0">')
            if totalPages > 0 then
                emit('                <span class="text-xs px-2 py-0.5 rounded-full bg-neutral-800 text-neutral-400">' .. h(tostring(lastPageRead)) .. '/' .. h(tostring(totalPages)) .. ' read')
                if cachedPages > 0 then
                    emit(' · ' .. h(tostring(cachedPages)) .. ' cached')
                end
                emit('</span>')
            elseif cachedPages > 0 then
                emit('                <span class="text-xs px-2 py-0.5 rounded-full bg-neutral-800 text-neutral-400">' .. h(tostring(cachedPages)) .. ' cached</span>')
            end
            if isDone then
                emit('                <span class="text-emerald-400 text-xs">✓</span>')
            end
            -- Release date
            if c.ReleasedAt then
                local rawDate = ""
                if type(c.ReleasedAt) == "table" and c.ReleasedAt.Format then
                    rawDate = c.ReleasedAt:Format("2006-01-02T15:04:05")
                elseif type(c.ReleasedAt) == "string" then
                    rawDate = c.ReleasedAt
                end
                if rawDate ~= "" and string.sub(rawDate, 1, 4) ~= "0001" then
                    emit('                <span class="text-xs text-neutral-500">' .. h(formatDate(rawDate)) .. '</span>')
                end
            end
            emit('            </span>')
            emit('        </a>')
            -- Mark read button
            emit('        <form method="post" action="/action/mark-read/' .. h(pluginID) .. '/' .. h(mangaID) .. '/' .. h(cID) .. '">')
            emit('            <button type="submit" title="Mark read" class="size-7 inline-flex items-center justify-center rounded-md border border-neutral-700 hover:bg-neutral-800 text-sm" aria-label="Mark read"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="size-4"><polyline points="20 6 9 17 4 12"/></svg></button>')
            emit('        </form>')
            -- Reset progress button
            emit('        <form method="post" action="/action/reset-progress/' .. h(pluginID) .. '/' .. h(mangaID) .. '/' .. h(cID) .. '">')
            emit('            <button type="submit" title="Reset progress" class="size-7 inline-flex items-center justify-center rounded-md border border-neutral-700 hover:bg-neutral-800 text-sm" aria-label="Reset progress" data-confirm="Are you sure?"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="size-4"><path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/><path d="M3 3v5h5"/></svg></button>')
            emit('        </form>')
            -- Mark range button
            emit('        <form method="post" action="/action/mark-read-range/' .. h(pluginID) .. '/' .. h(mangaID) .. '/' .. h(firstID) .. '/' .. h(cID) .. '">')
            emit('            <button type="submit" title="Mark up to here" class="size-7 inline-flex items-center justify-center rounded-md border border-neutral-700 hover:bg-neutral-800 text-sm" aria-label="Mark up to here"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="size-4"><path d="M18 6L7 17l-5-5"/><path d="M22 10l-7.5 7.5L13 16"/></svg></button>')
            emit('        </form>')
            -- CBZ download button
            emit('        <form method="post" action="/action/export-cbz/' .. h(pluginID) .. '/' .. h(mangaID) .. '/' .. h(cID) .. '">')
            emit('            <input type="hidden" name="title" value="' .. h(manga.Title or "") .. ' - Ch. ' .. h(formatChapterNum(chapterNum)) .. '">')
            emit('            <button type="submit" title="Download this chapter as .cbz" class="h-7 px-2 inline-flex items-center justify-center rounded-md border border-neutral-700 hover:bg-neutral-800 text-sm whitespace-nowrap">⬇ cbz</button>')
            emit('        </form>')
            emit('    </div>')
        end
        emit('</div>')

        -- Chapter pagination (bottom)
        emit(pagination({ Pagination = { Base = "", Param = "ChPage", Current = chPage, Total = chTotalPages } }))
    end

    return table.concat(parts, '\n')
end
