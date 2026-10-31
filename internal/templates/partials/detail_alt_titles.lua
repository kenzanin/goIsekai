--  partials/detail_alt_titles.lua
--  The alternative-titles section of the enrichment panel: one badge per alt
--  title, each a click-to-set form. Returned as one HTML string ("") when
--  there are no alt titles.
--  data.PluginID, data.MangaID, data.CurrentTitle, data.AltTitles,
--  data.csrf_token
return function(data)
	local pluginID = data.PluginID or ""
	local mangaID = data.MangaID or ""
	local currentTitle = data.CurrentTitle or ""
	local altTitles = data.AltTitles or {}

	local body = ""

	-- Alt titles — click to set (no confirm)
	local atCount = #altTitles
	if atCount > 0 then
		body = body
			.. '<div class="mb-3">'
			.. '<span class="text-xs font-semibold text-neutral-400 uppercase">Alternative titles ('
			.. atCount
			.. ")</span>"
		local atBody = '<div class="flex flex-wrap gap-1.5 mt-1">'
		-- Current title badge
		atBody = atBody
			.. '<span class="inline-flex items-center gap-1.5 rounded-full bg-neutral-800 border border-indigo-700/60 pl-2.5 pr-1 py-0.5 text-xs">'
			.. '<span class="text-indigo-300" title="Current main title">'
			.. h(currentTitle)
			.. "</span>"
			.. '<span class="text-xs px-1.5 py-0.5 rounded-full bg-indigo-900/40 text-indigo-300">via '
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
				.. csrfInput(data.csrf_token or "")
				.. '<input type="hidden" name="title" value="'
				.. h(t.Title or "")
				.. '">'
				.. '<input type="hidden" name="_label" value="Set as main title">'
				.. '<span class="text-neutral-300 hover:text-indigo-300 cursor-pointer transition" @click="submitForm($el.closest(&apos;form&apos;))">'
				.. h(t.Title or "")
				.. "</span>"
				.. "</form>"
				.. (t.Source and t.Source ~= "" and ('<span class="text-xs px-1.5 py-0.5 rounded-full bg-neutral-700/50 text-neutral-400">via ' .. h(
					t.Source
				) .. "</span>") or "")
				.. '<form method="post" action="/action/remove-alt-title/'
				.. h(pluginID)
				.. "/"
				.. h(mangaID)
				.. '" class="inline-flex">'
				.. csrfInput(data.csrf_token or "")
				.. '<input type="hidden" name="title" value="'
				.. h(t.Title or "")
				.. '">'
				.. '<button type="submit" title="Remove alternative title" aria-label="Remove" class="size-4 inline-flex items-center justify-center rounded-full text-neutral-500 hover:text-red-400 hover:bg-neutral-700" data-confirm="Remove this alternative title?">&times;</button>'
				.. "</form></span>"
		end
		atBody = atBody .. "</div>"
		body = body .. atBody .. "</div>"
	end

	return body
end
