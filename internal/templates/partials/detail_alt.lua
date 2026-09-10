-- partials/detail_alt.lua
-- Detail page: alternative titles, alt summaries, origin summary, fetch forms.
-- Called as: detail_alt(data) -> string (fragment, lives in the info column)
-- data.PluginID, data.MangaID, data.CurrentTitle, data.AltTitles,
-- data.AltTitleServers, data.AltSummaries, data.AltSummaryServers, data.Manga

return function(data)
	local pluginID = data.PluginID or ""
	local mangaID = data.MangaID or ""
	local currentTitle = data.CurrentTitle or ""
	local altTitles = data.AltTitles or {}
	local altTitleServers = data.AltTitleServers or {}
	local altSummaries = data.AltSummaries or {}
	local altSummaryServers = data.AltSummaryServers or {}
	local manga = data.Manga or {}

	local body = ""

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
	body = body .. summariesFormHTML

	return body
end
