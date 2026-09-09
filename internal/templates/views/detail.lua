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

	-- Back button
	local body = [[<div class="mb-4">
    <button onclick="if (history.length > 1) { history.back(); } else { window.location.href = '/view/library'; }" class="inline-flex items-center gap-1.5 text-sm text-neutral-400 hover:text-neutral-200 transition" aria-label="Back">
        <svg xmlns="http://www.w3.org/2000/svg" class="size-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m15 18-6-6 6-6"/></svg>
        Back
    </button></div>
]]

	-- Challenge warning
	if challenge then
		body = body
			.. '<div class="bg-amber-500/15 border border-amber-500/40 text-amber-300 rounded-lg p-4 mb-6">This site needs human verification — go to <a href="/view/plugins" class="underline">Plugins → Human Verification</a> to paste your session cookies.</div>'
	end

	-- Manga header grid
	body = body .. '<div class="flex flex-col md:flex-row gap-6 mb-4 items-start">' .. '<div class="md:w-[200px] md:shrink-0">'

	-- Cover or placeholder
	local coverHTML = ""
	if manga.CoverURL and manga.CoverURL ~= "" then
		coverHTML = '<div id="cover-wrap" class="relative" data-key="'
			.. h(pluginID)
			.. ":"
			.. h(mangaID)
			.. '">'
			.. '<img src="/image?pluginID='
			.. h(pluginID)
			.. "&amp;url="
			.. h(manga.CoverURL)
			.. '" alt="'
			.. h(manga.Title or "")
			.. '" class="w-full aspect-[2/3] rounded-lg object-cover">'
			.. '<div id="cover-dim" class="lib-dim" style="display:none;position:absolute;top:0;left:0;right:0;bottom:0;border-radius:0.5rem;background:rgba(0,0,0,0.82);"></div>'
			.. "</div>"
			.. '<button type="button" id="cover-dim-btn" title="Dim the cover image" onclick="var w=document.getElementById(&quot;cover-wrap&quot;);var d=document.getElementById(&quot;cover-dim&quot;);var b=document.getElementById(&quot;cover-dim-btn&quot;);if(b.dataset.on===&quot;1&quot;){b.dataset.on=&quot;0&quot;;d.style.display=&quot;none&quot;;localStorage.setItem(&quot;gsk:cover-dim:&quot;+w.dataset.key,&quot;0&quot;);b.textContent=&quot;Hide cover&quot;;}else{b.dataset.on=&quot;1&quot;;d.style.display=&quot;block&quot;;localStorage.setItem(&quot;gsk:cover-dim:&quot;+w.dataset.key,&quot;1&quot;);b.textContent=&quot;Show cover&quot;;}" class="mt-2 inline-flex items-center text-xs text-neutral-400 hover:text-neutral-200 transition cursor-pointer">Hide cover</button>'
	else
		coverHTML = '<div class="w-full aspect-[2/3] bg-neutral-800 rounded-lg flex items-center justify-center text-neutral-500 text-4xl font-semibold">'
			.. h(getInitials(manga.Title or ""))
			.. "</div>"
	end
	body = body
		.. coverHTML
		.. "</div>"
		.. '<div class="flex-1 min-w-0">'
		.. '<h1 class="text-2xl font-semibold mb-2">'
		.. h(manga.Title or "")
		.. "</h1>"

	-- Plugin badge + status + genres
	local genresHTML = ""
	local genres = manga.Genres or {}
	for _, g in ipairs(genres) do
		genresHTML = genresHTML
			.. '<span class="text-xs px-2 py-0.5 rounded-full bg-indigo-500/15 text-indigo-400">'
			.. h(g)
			.. "</span>"
	end

	body = body
		.. '<div class="flex flex-wrap gap-2 mb-3">'
		.. '<span class="inline-flex items-center gap-1.5 text-xs px-2 py-0.5 rounded-full bg-neutral-800 text-neutral-500">'
		.. (pluginIcon ~= "" and ('<img src="' .. h(pluginIcon) .. '" alt="" class="h-3.5 w-3.5 rounded-sm object-cover">') or "")
		.. h(pluginName)
		.. "</span>"
		.. (manga.Status and manga.Status ~= "" and ('<span class="text-xs px-2 py-0.5 rounded-full bg-neutral-700/50 text-neutral-400">' .. h(
			manga.Status
		) .. "</span>") or "")
		.. genresHTML
		.. "</div>"

	-- Author
	if manga.Author and manga.Author ~= "" then
		body = body .. '<p class="text-sm text-neutral-400 mb-2">' .. h(manga.Author) .. "</p>"
	end

	-- Alternative titles (collapsible)
	local atCount = #altTitles
	local chevOnclick =
		"this.nextElementSibling.classList.toggle(&quot;hidden&quot;);this.querySelector(&quot;.chev&quot;).classList.toggle(&quot;rotate-90&quot;)"
	local atBody = ""

	if #altTitles > 0 then
		atBody = '<div class="flex flex-wrap gap-1.5 mb-2">'
		-- Current title badge
		atBody = atBody
			.. '<span class="inline-flex items-center gap-1.5 rounded-full bg-neutral-800 border border-indigo-700/60 pl-2.5 pr-1 py-0.5 text-xs">'
			.. '<span class="text-indigo-300" title="Current main title">'
			.. h(currentTitle)
			.. "</span>"
			.. '<span class="text-[10px] px-1.5 py-0.5 rounded-full bg-indigo-900/40 text-indigo-300">via '
			.. h(pluginID)
			.. "</span>"
			.. "</span>"
		for _, t in ipairs(altTitles) do
			atBody = atBody
				.. '<span class="inline-flex items-center gap-1.5 rounded-full bg-neutral-800 border border-neutral-700/60 pl-2.5 pr-1 py-0.5 text-xs">'
				.. '<form method="post" action="/action/set-title/'
				.. h(pluginID)
				.. "/"
				.. h(mangaID)
				.. '" class="inline">'
				.. '<input type="hidden" name="title" value="'
				.. h(t.Title or "")
				.. '">'
				.. '<button type="submit" title="Set as main title" class="text-neutral-300 hover:text-indigo-300" data-confirm="Set as main title?">'
				.. h(t.Title or "")
				.. "</button>"
				.. "</form>"
				.. (t.Source and t.Source ~= "" and ('<span class="text-[10px] px-1.5 py-0.5 rounded-full bg-neutral-700/50 text-neutral-400">via ' .. h(
					t.Source
				) .. "</span>") or "")
				.. '<form method="post" action="/action/remove-alt-title/'
				.. h(pluginID)
				.. "/"
				.. h(mangaID)
				.. '" class="inline-flex">'
				.. '<input type="hidden" name="title" value="'
				.. h(t.Title or "")
				.. '">'
				.. '<button type="submit" title="Remove alternative title" aria-label="Remove" class="size-4 inline-flex items-center justify-center rounded-full text-neutral-500 hover:text-red-400 hover:bg-neutral-700" data-confirm="Are you sure?">&times;</button>'
				.. "</form></span>"
		end
		atBody = atBody .. "</div>"
	end

	body = body
		.. '<div class="mb-4">'
		.. '<button type="button" onclick="'
		.. chevOnclick
		.. '" class="flex items-center gap-1.5 cursor-pointer group select-none">'
		.. '<svg class="size-3.5 text-neutral-500 chev transition-transform" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m9 18 6-6-6-6"/></svg>'
		.. '<span class="text-xs font-semibold text-neutral-300 uppercase tracking-wide">Alternative titles'
		.. (atCount > 0 and (' <span class="text-neutral-500 font-normal">(' .. atCount .. ")</span>") or "")
		.. "</span></button>"
		.. '<div id="alt-titles-body" class="'
		.. (atCount > 0 and "hidden" or "")
		.. '">'
		.. atBody
		.. "</div>"
		.. "</div>"

	-- Fetch alt-titles form
	local titlesFormHTML = ""
	if #altTitleServers > 0 then
		local optionsHTML = ""
		for _, s in ipairs(altTitleServers) do
			optionsHTML = optionsHTML
				.. '<option value="'
				.. h(s.ServerID or "")
				.. '">'
				.. h(s.Name or "")
				.. "</option>"
		end
		titlesFormHTML = '<form method="post" action="/action/fetch-alt-titles/'
			.. h(pluginID)
			.. "/"
			.. h(mangaID)
			.. '" class="flex flex-wrap items-center gap-2">'
			.. '<select name="server" class="bg-neutral-900 border border-neutral-700 rounded-md px-2 py-1.5 text-xs text-neutral-300 focus:outline-none focus:border-indigo-500">'
			.. optionsHTML
			.. "</select>"
			.. '<button type="submit" class="border border-neutral-700 hover:bg-neutral-800 rounded-md px-3 py-1.5 text-xs font-medium inline-flex items-center gap-1.5">'
			.. '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="size-3.5"><circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/></svg>'
			.. " Get alternative titles</button></form>"
	else
		titlesFormHTML =
			'<p class="text-xs text-neutral-500">No alt-title providers available — install a plugin that declares alt-title servers.</p>'
	end
	body = body .. titlesFormHTML

	-- Origin summary
	if manga.Description and manga.Description ~= "" then
		body = body
			.. '<div class="mb-4 border-t border-neutral-800 pt-4">'
			.. '<span class="text-xs font-semibold text-neutral-300 uppercase tracking-wide">Summary</span>'
			.. '<p class="text-sm text-neutral-400 mt-1.5">'
			.. h(manga.Description)
			.. "</p></div>"
	end

	-- Alternative summaries (collapsible)
	local asCount = #altSummaries
	local asBody = ""
	if #altSummaries > 0 then
		for _, a in ipairs(altSummaries) do
			asBody = asBody
				.. '<div class="flex items-start gap-2 py-1">'
				.. '<form method="post" action="/action/set-summary/'
				.. h(pluginID)
				.. "/"
				.. h(mangaID)
				.. '" class="flex-1 min-w-0 group" data-confirm="Set this as the main summary?">'
				.. '<input type="hidden" name="description" value="'
				.. h(a.Description or "")
				.. '">'
				.. '<button type="submit" title="Set as main summary" class="w-full text-left text-sm text-neutral-300 hover:text-indigo-300 transition">'
				.. h(a.Description or "")
				.. "</button>"
				.. "</form>"
				.. '<div class="flex items-center gap-1.5 shrink-0 mt-0.5">'
				.. (a.Source and a.Source ~= "" and ('<span class="text-[10px] px-1.5 py-0.5 rounded-full bg-neutral-700/50 text-neutral-400">via ' .. h(
					a.Source
				) .. "</span>") or "")
				.. '<form method="post" action="/action/remove-alt-summary/'
				.. h(pluginID)
				.. "/"
				.. h(mangaID)
				.. '" class="inline-flex">'
				.. '<input type="hidden" name="description" value="'
				.. h(a.Description or "")
				.. '">'
				.. '<button type="submit" title="Remove alternative summary" aria-label="Remove" class="size-4 inline-flex items-center justify-center rounded-full text-neutral-500 hover:text-red-400 hover:bg-neutral-700" data-confirm="Remove this alternative summary?">&times;</button>'
				.. "</form></div></div>"
		end
	end

	body = body
		.. '<div class="mb-4 border-t border-neutral-800 pt-4">'
		.. '<button type="button" onclick="'
		.. chevOnclick
		.. '" class="flex items-center gap-1.5 cursor-pointer group select-none">'
		.. '<svg class="size-3.5 text-neutral-500 chev transition-transform" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m9 18 6-6-6-6"/></svg>'
		.. '<span class="text-xs font-semibold text-neutral-300 uppercase tracking-wide">Alternative summaries'
		.. (asCount > 0 and (' <span class="text-neutral-500 font-normal">(' .. asCount .. ")</span>") or "")
		.. "</span></button>"
		.. '<div id="alt-summaries-body" class="'
		.. (asCount > 0 and "hidden" or "")
		.. '">'
		.. asBody
		.. "</div></div>"

	-- Fetch alt-summaries form
	local summariesFormHTML = ""
	if #altSummaryServers > 0 then
		local sOptions = ""
		for _, s in ipairs(altSummaryServers) do
			sOptions = sOptions .. '<option value="' .. h(s.ServerID or "") .. '">' .. h(s.Name or "") .. "</option>"
		end
		summariesFormHTML = '<form method="post" action="/action/fetch-alt-summaries/'
			.. h(pluginID)
			.. "/"
			.. h(mangaID)
			.. '" class="flex flex-wrap items-center gap-2 mt-1">'
			.. '<select name="server" class="bg-neutral-900 border border-neutral-700 rounded-md px-2 py-1.5 text-xs text-neutral-300 focus:outline-none focus:border-indigo-500">'
			.. sOptions
			.. "</select>"
			.. '<button type="submit" class="border border-neutral-700 hover:bg-neutral-800 rounded-md px-3 py-1.5 text-xs font-medium inline-flex items-center gap-1.5">'
			.. '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="size-3.5"><circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/></svg>'
			.. " Get alternative summaries</button></form>"
	else
		summariesFormHTML =
			'<p class="text-xs text-neutral-500">No alt-summary providers available — install a plugin that declares alt-summary servers.</p>'
	end
	body = body .. summariesFormHTML .. "</div></div>"

	-- Action buttons: library + continue reading
	local actionsHTML = '<div class="flex flex-wrap items-center gap-2 mb-4">'
		.. '<form method="post" action="/action/toggle-library/'
		.. h(pluginID)
		.. "/"
		.. h(mangaID)
		.. '">'
		.. (inLibrary and '<button type="submit" class="border border-emerald-600/50 text-emerald-400 bg-emerald-500/10 hover:bg-emerald-500/20 rounded-md px-4 py-2 text-sm">✓ In Library</button>' or '<button type="submit" class="bg-indigo-600 hover:bg-indigo-500 text-white rounded-md px-4 py-2 text-sm font-medium">+ Add to Library</button>')
		.. "</form>"

	if continuePoint then
		local cp = continuePoint
		local label, titleAttr
		if cp.Page and cp.Page > 1 then
			label = "▶ Continue · Ch." .. formatChapterNum(cp.ChapterN)
			titleAttr = "Continue: Ch. " .. formatChapterNum(cp.ChapterN) .. ", page " .. tostring(cp.Page)
		elseif cp.Started then
			label = "▶ Continue · Ch." .. formatChapterNum(cp.ChapterN)
			titleAttr = "Continue: Ch. " .. formatChapterNum(cp.ChapterN)
		else
			label = "▶ Start Reading"
			titleAttr = "Start Reading"
		end
		actionsHTML = actionsHTML
			.. '<a href="/view/read/'
			.. h(pluginID)
			.. "/"
			.. h(mangaID)
			.. "/"
			.. h(cp.ChapterID or "")
			.. "?page="
			.. tostring(cp.Page or 1)
			.. '" class="bg-indigo-600 hover:bg-indigo-500 text-white rounded-md px-4 py-2 text-sm font-medium" title="'
			.. h(titleAttr)
			.. '">'
			.. h(label)
			.. "</a>"
	end
	body = body .. actionsHTML .. "</div>"

	-- Chapters section
	body = body .. '<h2 class="text-xl font-semibold mb-4">Chapters</h2>'

	if #chapters == 0 then
		body = body .. '<div class="py-16 text-center text-neutral-500">No chapters yet</div>'
	else
		-- Bulk actions toolbar
		body = body
			.. '<div class="flex flex-wrap items-center gap-2 mb-4">'
			.. '<form id="mark-bulk" method="post" action="/action/mark-read-bulk">'
			.. '<input type="hidden" name="pluginID" value="'
			.. h(pluginID)
			.. '">'
			.. '<input type="hidden" name="mangaID" value="'
			.. h(mangaID)
			.. '">'
			.. '<button type="submit" class="bg-indigo-600 hover:bg-indigo-500 text-white rounded-md px-3 py-1.5 text-sm font-medium">Mark selected as read</button>'
			.. "</form>"
			.. '<form method="post" action="/action/reset-progress-all/'
			.. h(pluginID)
			.. "/"
			.. h(mangaID)
			.. '">'
			.. '<button type="submit" title="Clear read progress for every chapter" class="bg-transparent border border-red-900/60 text-red-400 hover:bg-red-950/40 rounded-md px-3 py-1.5 text-sm" data-confirm="Are you sure?">↺ Reset all read status</button>'
			.. "</form>"
			.. '<form method="post" action="/action/clear-cache/'
			.. h(pluginID)
			.. "/"
			.. h(mangaID)
			.. '" data-confirm="Clear cached images for this manga?">'
			.. '<button type="submit" title="Delete cached image files for this manga" class="bg-transparent border border-red-900/60 text-red-400 hover:bg-red-950/40 rounded-md px-3 py-1.5 text-sm">🗑 Clear cached images</button>'
			.. "</form></div>"

		-- Chapter pagination (top)
		body = body
			.. pagination({
				Pagination = { Base = "", Param = "ChPage", Current = chPage, Total = chTotalPages },
			})

		-- Chapter list
		local chaptersHTML = '<div class="divide-y divide-neutral-800">'
		local firstID = ""
		for i, c in ipairs(chapters) do
			local cID = c.ID or ""
			if i == 1 then
				firstID = cID
			end
			local cTitle = c.Title or ""
			local chapterNum = c.ChapterNum or 0
			local p = progress[cID] or {}
			local isDone = p.Done or false
			local totalPages = p.TotalPages or 0
			local lastPageRead = p.LastPageRead or 0
			local cachedPages = p.CachedPages or 0

			local rowHTML = '<div class="flex items-center gap-3 py-3 px-2 -mx-2 rounded hover:bg-neutral-900 transition">'
				.. '<input type="checkbox" name="chapterIDs" value="'
				.. h(cID)
				.. '" form="mark-bulk" class="size-4 accent-indigo-600 shrink-0">'
				.. '<a href="/view/read/'
				.. h(pluginID)
				.. "/"
				.. h(mangaID)
				.. "/"
				.. h(cID)
				.. '" class="flex-1 flex items-center justify-between gap-4 min-w-0">'
				.. '<span class="text-sm">'
				.. '<span class="text-neutral-400 mr-2">Ch. '
				.. h(formatChapterNum(chapterNum))
				.. "</span>"
				.. (isDone and ('<span class="line-through text-neutral-500">' .. h(cTitle) .. "</span>") or (" " .. h(
					cTitle
				)))
				.. "</span>"
				.. '<span class="flex items-center gap-2 shrink-0">'

			-- Page info
			if totalPages > 0 then
				rowHTML = rowHTML
					.. '<span class="text-xs px-2 py-0.5 rounded-full bg-neutral-800 text-neutral-400">'
					.. h(tostring(lastPageRead))
					.. "/"
					.. h(tostring(totalPages))
					.. " read"
					.. (cachedPages > 0 and (" · " .. h(tostring(cachedPages)) .. " cached") or "")
					.. "</span>"
			elseif cachedPages > 0 then
				rowHTML = rowHTML
					.. '<span class="text-xs px-2 py-0.5 rounded-full bg-neutral-800 text-neutral-400">'
					.. h(tostring(cachedPages))
					.. " cached</span>"
			end

			if isDone then
				rowHTML = rowHTML .. '<span class="text-emerald-400 text-xs">✓</span>'
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
					rowHTML = rowHTML
						.. '<span class="text-xs text-neutral-500">'
						.. h(formatDate(rawDate))
						.. "</span>"
				end
			end
			rowHTML = rowHTML .. "</span></a>"

			-- Mark read button
			rowHTML = rowHTML
				.. '<form method="post" action="/action/mark-read/'
				.. h(pluginID)
				.. "/"
				.. h(mangaID)
				.. "/"
				.. h(cID)
				.. '">'
				.. '<button type="submit" title="Mark read" class="size-7 inline-flex items-center justify-center rounded-md border border-neutral-700 hover:bg-neutral-800 text-sm" aria-label="Mark read"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="size-4"><polyline points="20 6 9 17 4 12"/></svg></button>'
				.. "</form>"

			-- Reset progress button
			rowHTML = rowHTML
				.. '<form method="post" action="/action/reset-progress/'
				.. h(pluginID)
				.. "/"
				.. h(mangaID)
				.. "/"
				.. h(cID)
				.. '">'
				.. '<button type="submit" title="Reset progress" class="size-7 inline-flex items-center justify-center rounded-md border border-neutral-700 hover:bg-neutral-800 text-sm" aria-label="Reset progress" data-confirm="Are you sure?"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="size-4"><path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/><path d="M3 3v5h5"/></svg></button>'
				.. "</form>"

			-- Mark range button
			rowHTML = rowHTML
				.. '<form method="post" action="/action/mark-read-range/'
				.. h(pluginID)
				.. "/"
				.. h(mangaID)
				.. "/"
				.. h(firstID)
				.. "/"
				.. h(cID)
				.. '">'
				.. '<button type="submit" title="Mark up to here" class="size-7 inline-flex items-center justify-center rounded-md border border-neutral-700 hover:bg-neutral-800 text-sm" aria-label="Mark up to here"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="size-4"><path d="M18 6L7 17l-5-5"/><path d="M22 10l-7.5 7.5L13 16"/></svg></button>'
				.. "</form>"

			-- CBZ download button
			rowHTML = rowHTML
				.. '<form method="post" action="/action/export-cbz/'
				.. h(pluginID)
				.. "/"
				.. h(mangaID)
				.. "/"
				.. h(cID)
				.. '">'
				.. '<input type="hidden" name="title" value="'
				.. h(manga.Title or "")
				.. " - Ch. "
				.. h(formatChapterNum(chapterNum))
				.. '">'
				.. '<button type="submit" title="Download this chapter as .cbz" class="h-7 px-2 inline-flex items-center justify-center rounded-md border border-neutral-700 hover:bg-neutral-800 text-sm whitespace-nowrap">⬇ cbz</button>'
				.. "</form></div>"

			chaptersHTML = chaptersHTML .. rowHTML
		end
		body = body .. chaptersHTML .. "</div>"

		-- Chapter pagination (bottom)
		body = body
			.. pagination({
				Pagination = { Base = "", Param = "ChPage", Current = chPage, Total = chTotalPages },
			})
	end

	return body
end
