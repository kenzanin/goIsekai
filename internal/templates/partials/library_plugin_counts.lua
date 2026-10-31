--  partials/library_plugin_counts.lua
--  The "titles per plugin" card of the library sidebar: the All row plus one
--  row per plugin. Returned as the card's HTML ("") when the caller has no
--  plugin counts, so the sidebar keeps deciding when to render it.
--  data.PluginCounts, data.PluginID
return function(data)
	local pluginCounts = data.PluginCounts or {}

	local pluginCard = ""
	-- Plugin counts
	if #pluginCounts > 0 then
		local curPluginID = data.PluginID or ""
		local pluginRows = '<a href="/view/library" onclick="try{localStorage.removeItem(\'gsk:library-view\')}catch(e){}" class="flex items-center justify-between gap-2 text-sm rounded-md px-1.5 py-1 mb-1 border border-dashed '
			.. (curPluginID == "" and "border-indigo-500 bg-indigo-500/15" or "border-neutral-700")
			.. '"><span class="text-sm font-medium '
			.. (curPluginID == "" and "text-indigo-200" or "text-neutral-300")
			.. '">All</span><span class="text-xs text-neutral-500">all manga</span></a>'
		for _, p in ipairs(pluginCounts) do
			local active = (curPluginID ~= "" and p.PluginID == curPluginID)
			local iconHTML = ""
			if p.Icon and p.Icon ~= "" then
				iconHTML = '<img src="'
					.. h(p.Icon)
					.. '" alt="" class="h-3.5 w-3.5 rounded-sm object-cover shrink-0">'
			end
			pluginRows = pluginRows
				.. '<a href="/view/library?pluginID='
				.. h(p.PluginID or "")
				.. '" class="flex items-center justify-between gap-2 text-sm rounded-md px-1.5 py-1 '
				.. (active and "border border-indigo-500 bg-indigo-500/15" or "border border-transparent")
				.. '">'
				.. '<span class="flex items-center gap-1.5 min-w-0">'
				.. iconHTML
				.. '<span class="text-sm font-medium truncate '
				.. (active and "text-indigo-200" or "text-neutral-200")
				.. '">'
				.. h(p.Name or "")
				.. "</span>"
				.. "</span>"
				.. '<span class="text-sm font-bold shrink-0 '
				.. (active and "text-indigo-300" or "text-indigo-400")
				.. '">'
				.. h(tostring(p.Count or 0))
				.. "</span></a>"
		end
		pluginCard = [[
        <div class="bg-neutral-900 border border-neutral-800 rounded-lg px-4 py-3">
            <div class="text-xs font-medium text-neutral-400 mb-2">titles per plugin</div>
            <div class="flex flex-col gap-1.5">]]
			.. pluginRows
			.. [[</div>
        </div>]]
	end

	return pluginCard
end
