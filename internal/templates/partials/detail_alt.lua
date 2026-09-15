--- partials/detail_alt.lua
--  Detail page: Alpine.js-powered enrichment panel — interactive genres & related toggles.
--  data.PluginID, data.MangaID, data.CurrentTitle, data.AltTitles,
--  data.AltSummaries, data.Manga
--  data.Categories, data.Related, data.Genres (current active genres for highlighting)

return function(data)
	local pluginID = data.PluginID or ""
	local mangaID = data.MangaID or ""
	local currentTitle = data.CurrentTitle or ""
	local altTitles = data.AltTitles or {}
	local altSummaries = data.AltSummaries or {}
	local manga = data.Manga or {}
	local cats = data.Categories or {}
	local rels = data.Related or {}
	local genres = data.Genres or {}
	local pluginGenres = data.PluginGenres or {}
	local author = data.Author or ""

	-- Use &quot; for JSON double-quotes so they don't break the HTML attribute.
	-- The browser decodes &quot; → " before Alpine sees the x-data expression.
	local jq = '&quot;'

	-- Build x-data: genres list and related list for Alpine reactive highlighting.
	-- Individual values are HTML-escaped with h(); the JSON structure uses &quot;.
	local genresJSON = '['
	for i, g in ipairs(genres) do
		if i > 1 then genresJSON = genresJSON .. ',' end
		genresJSON = genresJSON .. jq .. h(g) .. jq
	end
	genresJSON = genresJSON .. ']'

	local relsJSON = '['
	for i, r in ipairs(rels) do
		if i > 1 then relsJSON = relsJSON .. ',' end
		relsJSON = relsJSON .. jq .. h(r.Value or '') .. jq
	end
	relsJSON = relsJSON .. ']'

	local xData = "x-data=\"{ loading: false, currentGenres: " .. genresJSON .. ", currentRelated: " .. relsJSON .. " }\""

	local chevOnClick =
		"var b=document.getElementById(&quot;enrichment-btn&quot;);var bd=document.getElementById(&quot;enrichment-body&quot;);bd.classList.toggle(&quot;hidden&quot;);b.querySelector(&quot;.chev&quot;).classList.toggle(&quot;rotate-90&quot;);try{localStorage.setItem(&quot;gsk:enrichment-open:&quot;+b.dataset.key,b.querySelector(&quot;.chev&quot;).classList.contains(&quot;rotate-90&quot;) ? &quot;1&quot; : &quot;0&quot;)}catch(e){}"

	local body = ""

	-- Enrichment collapsible header
	body = body
		.. '<div class="mb-4 border-t border-neutral-800 pt-4">'
		.. '<button type="button" id="enrichment-btn" data-key="'
		.. h(pluginID)
		.. ':'
		.. h(mangaID)
		.. '" onclick="'
		.. chevOnClick
		.. '" class="flex items-center gap-1.5 cursor-pointer group select-none">'
		.. '<svg class="size-3.5 text-neutral-500 chev transition-transform" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m9 18 6-6-6-6"/></svg>'
		.. '<span class="text-xs font-semibold text-neutral-300 uppercase tracking-wide">Enrichment</span>'
		.. "</button>"
		.. '<div id="enrichment-body" class="hidden" ' .. xData .. '>'

	-- Fetch button with spinner + Reset button side by side
	body = body
	.. (author ~= "" and ('<p class="text-sm text-neutral-400 mb-2">' .. "Author: " .. h(author) .. "</p>") or "")
	.. '<div class="mb-3 flex items-center gap-2">'
		.. '<form method="post" action="/action/fetch-enrichment/'
		.. h(pluginID)
		.. "/"
		.. h(mangaID)
		.. '">'
		.. '<input type="hidden" name="manga_title" value="'
		.. h(data.CurrentTitle or manga.Title or "")
		.. '">'
		.. '<button type="submit" class="border border-indigo-600/50 text-indigo-300 hover:bg-indigo-600/20 rounded-md px-3 py-1.5 text-xs font-medium inline-flex items-center gap-1.5" :disabled="loading">'
		.. '<svg x-show="loading" class="size-3.5 animate-spin" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 1 1-6.219-8.56"/><path d="M3.5 12h3.5"/><path d="M12 3.5v3.5"/></svg>'
		.. '<span x-text="loading ? &quot;Fetching...&quot; : &quot;Fetch Details&quot;">Fetch Details</span>'
		.. "</button>"
		.. "</form>"
		.. '<form method="post" action="/action/reset-enrichment/'
		.. h(pluginID)
		.. "/"
		.. h(mangaID)
		.. '">'
		.. '<button type="submit" class="border border-red-600/50 text-red-400 hover:bg-red-500/10 rounded-md px-3 py-1.5 text-xs font-medium" data-confirm="Reset all enrichment data to original plugin values?">Reset to Original</button>'
		.. '</form>'
		.. '</div>'

	-- Alt titles — click to set (no confirm)
	local atCount = #altTitles
	if atCount > 0 then
		body = body
			.. '<div class="mb-3">'
			.. '<span class="text-[11px] font-semibold text-neutral-400 uppercase">Alternative titles ('
			.. atCount
			.. ')</span>'
		local atBody = '<div class="flex flex-wrap gap-1.5 mt-1">'
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
				.. '<input type="hidden" name="_label" value="Set as main title">'
				.. '<span class="text-neutral-300 hover:text-indigo-300 cursor-pointer transition" @click="submitForm($el.closest(&apos;form&apos;))">'
				.. h(t.Title or "")
				.. "</span>"
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
				.. '<button type="submit" title="Remove alternative title" aria-label="Remove" class="size-4 inline-flex items-center justify-center rounded-full text-neutral-500 hover:text-red-400 hover:bg-neutral-700" data-confirm="Remove this alternative title?">&times;</button>'
				.. "</form></span>"
		end
		atBody = atBody .. "</div>"
		body = body .. atBody .. "</div>"
	end

	-- Alt synopses — click to set (no confirm)
	local asCount = #altSummaries
	if asCount > 0 then
		body = body
			.. '<div class="mb-3">'
			.. '<span class="text-[11px] font-semibold text-neutral-400 uppercase">Alternative synopses ('
			.. asCount
			.. ')</span>'
		for _, a in ipairs(altSummaries) do
			body = body
				.. '<div class="flex items-start gap-2 py-1">'
				.. '<form method="post" action="/action/set-summary/'
				.. h(pluginID)
				.. "/"
				.. h(mangaID)
				.. '" class="flex-1 min-w-0 group">'
				.. '<input type="hidden" name="description" value="'
				.. h(a.Description or "")
				.. '">'
				.. '<span class="w-full text-left text-sm text-neutral-300 hover:text-indigo-300 transition cursor-pointer" @click="submitForm($el.closest(&apos;form&apos;))">'
				.. h(a.Description or "")
				.. "</span>"
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
				.. '<button type="submit" title="Remove alternative summary" aria-label="Remove" class="size-4 inline-flex items-center justify-center rounded-full text-neutral-500 hover:text-red-400 hover:bg-neutral-700" data-confirm="Remove this alternative synopsis?">&times;</button>'
				.. "</form></div></div>"
		end
		body = body .. "</div>"
	end

	-- Genres — toggle via add/remove endpoints, highlighted if currently active
	if #cats > 0 then
		body = body .. '<div class="mb-3">'
		body = body .. '<span class="text-[11px] font-semibold text-neutral-400 uppercase">Categories / Genres</span>'
		body = body .. '<div class="flex flex-wrap gap-1.5 mt-1">'
		for _, c in ipairs(cats) do
			if not c.Value or c.Value == "" then goto next end
			local isCurrent = false
			for _, g in ipairs(pluginGenres) do
				if g == c.Value then isCurrent = true; break end
			end
			body = body
				.. '<form method="post" action="'
				.. (isCurrent and '/action/remove-category/' or '/action/add-category/')
				.. h(pluginID)
				.. "/"
				.. h(mangaID)
				.. '" class="inline">'
				.. '<input type="hidden" name="category" value="'
				.. h(c.Value)
				.. '">'
				.. '<span class="inline-flex items-center gap-1 rounded-full '
				.. (isCurrent
					and 'bg-indigo-500/20 border-indigo-600/60 text-indigo-300 cursor-pointer hover:bg-indigo-500/30'
					or 'bg-neutral-800 border-neutral-700 text-neutral-300 cursor-pointer hover:bg-neutral-700')
				.. '" @click="submitForm($el.closest(&apos;form&apos;))">'
				.. h(c.Value)
				.. '</span></form>'
			::next::
		end
		body = body .. "</div></div>"

	end

	-- Related manga — clickable, highlighted if in current related
	if #rels > 0 then
		body = body .. '<div class="mb-3" data-related-section>'
		body = body .. '<span class="text-[11px] font-semibold text-neutral-400 uppercase">Related / Recommended</span>'
		body = body .. '<div class="flex flex-wrap gap-1.5 mt-1">'
		for _, r in ipairs(rels) do
			local rName = r.Value or ""
			local isCur = false
			for _, g in ipairs(genres) do
				if g == rName then isCur = true; break end
			end
			body = body
				.. '<form method="post" action="/action/remove-related/'
				.. h(pluginID)
				.. "/"
				.. h(mangaID)
				.. '" class="inline">'
				.. '<input type="hidden" name="title" value="'
				.. h(rName)
				.. '">'
				.. '<span class="inline-flex items-center gap-1 rounded-full '
				.. (isCur
					and 'bg-emerald-500/20 border-emerald-600/60 text-emerald-300 cursor-pointer hover:bg-emerald-500/30'
					or 'bg-neutral-800 border-neutral-700 text-neutral-300 cursor-pointer hover:bg-neutral-700')
				.. '" @click="submitForm($el.closest(&apos;form&apos;))">'
				.. h(rName)
				.. '</span></form>'
		end
		body = body .. "</div></div>"
	end

	body = body .. "</div></div>"
	return body
end
