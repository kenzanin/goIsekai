--- partials/detail_alt.lua
--  Detail page: Alpine.js-powered enrichment panel — interactive genres & title toggles.
--  data.PluginID, data.MangaID, data.CurrentTitle, data.AltTitles,
--  data.AltSummaries, data.Manga
--  data.Categories, data.PluginGenres (current active genres for highlighting)

local detailAltTitles = require("partials.detail_alt_titles")

return function(data)
	local pluginID = data.PluginID or ""
	local mangaID = data.MangaID or ""
	local currentTitle = data.CurrentTitle or ""
	local altTitles = data.AltTitles or {}
	local altSummaries = data.AltSummaries or {}
	local manga = data.Manga or {}
	local cats = data.Categories or {}
	local pluginGenres = data.PluginGenres or {}
	local author = data.Author or ""

	local xData = 'x-data="{ loading: false }"'

	local chevOnClick =
		"var b=document.getElementById(&quot;enrichment-btn&quot;);var bd=document.getElementById(&quot;enrichment-body&quot;);bd.classList.toggle(&quot;hidden&quot;);b.querySelector(&quot;.chev&quot;).classList.toggle(&quot;rotate-90&quot;);try{localStorage.setItem(&quot;gsk:enrichment-open:&quot;+b.dataset.key,b.querySelector(&quot;.chev&quot;).classList.contains(&quot;rotate-90&quot;) ? &quot;1&quot; : &quot;0&quot;)}catch(e){}"

	local body = ""

	-- Enrichment collapsible header
	body = body
		.. '<div class="mb-4 border-t border-neutral-800 pt-4">'
		.. '<button type="button" id="enrichment-btn" data-key="'
		.. h(pluginID)
		.. ":"
		.. h(mangaID)
		.. '" onclick="'
		.. chevOnClick
		.. '" class="flex items-center gap-1.5 cursor-pointer group select-none">'
		.. '<svg class="size-3.5 text-neutral-500 chev transition-transform" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m9 18 6-6-6-6"/></svg>'
		.. '<span class="text-xs font-semibold text-neutral-300 uppercase tracking-wide">Enrichment</span>'
		.. "</button>"
		.. '<div id="enrichment-body" class="hidden" '
		.. xData
		.. ">"

	-- Fetch button with spinner + Reset button side by side
	body = body
		.. (author ~= "" and ('<p class="text-sm text-neutral-400 mb-2">' .. "Author: " .. h(author) .. "</p>") or "")
		.. '<div class="mb-3 flex items-center gap-2">'
		.. '<form method="post" action="/action/fetch-enrichment/'
		.. h(pluginID)
		.. "/"
		.. h(mangaID)
		.. '">'
		.. csrfInput(data.csrf_token or "")
		.. '<input type="hidden" name="manga_title" value="'
		.. h(data.CurrentTitle or manga.Title or "")
		.. '">'
		.. '<button type="submit" class="border border-indigo-600/50 text-indigo-300 hover:bg-indigo-600/20 rounded-md px-3 py-1.5 text-xs font-medium inline-flex items-center gap-1.5" @click="loading = true">'
		.. '<svg x-show="loading" class="size-3.5 animate-spin" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 1 1-6.219-8.56"/><path d="M3.5 12h3.5"/><path d="M12 3.5v3.5"/></svg>'
		.. '<span x-text="loading ? &quot;Fetching...&quot; : &quot;Fetch Details&quot;">Fetch Details</span>'
		.. "</button>"
		.. "</form>"
		.. '<form method="post" action="/action/reset-enrichment/'
		.. h(pluginID)
		.. "/"
		.. h(mangaID)
		.. '">'
		.. csrfInput(data.csrf_token or "")
		.. '<button type="submit" class="border border-red-600/50 text-red-400 hover:bg-red-500/10 rounded-md px-3 py-1.5 text-xs font-medium" data-confirm="Reset all enrichment data to original plugin values?">Reset to Original</button>'
		.. "</form>"
		.. "</div>"

	body = body .. detailAltTitles({
		csrf_token = data.csrf_token or "",
		PluginID = pluginID,
		MangaID = mangaID,
		CurrentTitle = currentTitle,
		AltTitles = altTitles,
	})

	-- Alt synopses — click to set (no confirm)
	local asCount = #altSummaries
	if asCount > 0 then
		body = body
			.. '<div class="mb-3">'
			.. '<span class="text-xs font-semibold text-neutral-400 uppercase">Alternative synopses ('
			.. asCount
			.. ")</span>"
		for _, a in ipairs(altSummaries) do
			body = body
				.. '<div class="flex items-start gap-2 py-1">'
				.. '<form method="post" action="/action/set-summary/'
				.. h(pluginID)
				.. "/"
				.. h(mangaID)
				.. '" class="flex-1 min-w-0 group">'
				.. csrfInput(data.csrf_token or "")
				.. '<input type="hidden" name="description" value="'
				.. h(a.Description or "")
				.. '">'
				.. '<span class="w-full text-left text-sm text-neutral-300 hover:text-indigo-300 transition cursor-pointer" @click="submitForm($el.closest(&apos;form&apos;))">'
				.. h(a.Description or "")
				.. "</span>"
				.. "</form>"
				.. '<div class="flex items-center gap-1.5 shrink-0 mt-0.5">'
				.. (a.Source and a.Source ~= "" and ('<span class="text-xs px-1.5 py-0.5 rounded-full bg-neutral-700/50 text-neutral-400">via ' .. h(
					a.Source
				) .. "</span>") or "")
				.. '<form method="post" action="/action/remove-alt-summary/'
				.. h(pluginID)
				.. "/"
				.. h(mangaID)
				.. '" class="inline-flex">'
				.. csrfInput(data.csrf_token or "")
				.. '<input type="hidden" name="description" value="'
				.. h(a.Description or "")
				.. '">'
				.. '<button type="submit" title="Remove alternative summary" aria-label="Remove" class="size-4 inline-flex items-center justify-center rounded-full text-neutral-500 hover:text-red-400 hover:bg-neutral-700" data-confirm="Remove this alternative synopsis?">&times;</button>'
				.. "</form></div></div>"
		end
		body = body .. "</div>"
	end

	-- Genres — toggle via add/remove endpoints, highlighted if currently active
	if #cats > 0 then
		body = body .. '<div class="mb-3">'
		body = body .. '<span class="text-xs font-semibold text-neutral-400 uppercase">Categories / Genres</span>'
		body = body .. '<div class="flex flex-wrap gap-1.5 mt-1">'
		for _, c in ipairs(cats) do
			if not c.Value or c.Value == "" then
				goto next
			end
			local isCurrent = false
			for _, g in ipairs(pluginGenres) do
				if g == c.Value then
					isCurrent = true
					break
				end
			end
			body = body
				.. '<form method="post" action="'
				.. (isCurrent and "/action/remove-category/" or "/action/add-category/")
				.. h(pluginID)
				.. "/"
				.. h(mangaID)
				.. '" class="inline">'
				.. csrfInput(data.csrf_token or "")
				.. '<input type="hidden" name="category" value="'
				.. h(c.Value)
				.. '">'
				.. '<span class="inline-flex items-center gap-1 rounded-full '
				.. (isCurrent and "bg-indigo-500/20 border-indigo-600/60 text-indigo-300 cursor-pointer hover:bg-indigo-500/30" or "bg-neutral-800 border-neutral-700 text-neutral-300 cursor-pointer hover:bg-neutral-700")
				.. '" @click="submitForm($el.closest(&apos;form&apos;))">'
				.. h(c.Value)
				.. "</span></form>"
			::next::
		end
		body = body .. "</div></div>"
	end

	body = body .. "</div></div>"
	return body
end
