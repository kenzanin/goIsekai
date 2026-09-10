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

local detailAlt = require("partials.detail_alt")
local detailChapters = require("partials.detail_chapters")

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
	body = body
		.. '<div class="flex flex-col md:flex-row gap-6 mb-4 items-start">'
		.. '<div class="md:w-[200px] md:shrink-0">'

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

	-- Alternative titles / summaries + fetch forms (info column)
	body = body
		.. detailAlt({
			PluginID = pluginID,
			MangaID = mangaID,
			CurrentTitle = currentTitle,
			AltTitles = altTitles,
			AltTitleServers = altTitleServers,
			AltSummaries = altSummaries,
			AltSummaryServers = altSummaryServers,
			Manga = manga,
		})
		.. "</div></div>"

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
	body = body
		.. detailChapters({
			PluginID = pluginID,
			MangaID = mangaID,
			Manga = manga,
			Chapters = chapters,
			Progress = progress,
			ChPage = chPage,
			ChTotalPages = chTotalPages,
		})

	return body
end
