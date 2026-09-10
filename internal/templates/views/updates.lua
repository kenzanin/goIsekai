-- Updates page: fresh chapter updates + recently added library titles.
return function(data)
	local updates = data.Updates or {}
	local recent = data.Recent or {}

	local body = [[<div class="mb-6">
    <h1 class="text-2xl font-semibold">Updates</h1>
    <p class="text-sm text-neutral-400 mt-1">]] .. h(tostring(data.UpdatesCount)) .. [[ manga with new chapters · ]] .. h(
		tostring(data.RecentCount)
	) .. [[ added in the last 7 days</p>
</div>]]

	local function rowCard(m, badge)
		local icon = ""
		if m.PluginIcon and m.PluginIcon ~= "" then
			icon = '<img src="' .. h(m.PluginIcon) .. '" alt="" class="h-3.5 w-3.5 rounded-sm object-cover shrink-0">'
		end
		local cover = ""
		if m.CoverURL and m.CoverURL ~= "" then
			cover = '<img src="/image?pluginID=' .. h(m.PluginID) .. "&amp;url=" .. h(m.CoverURL) .. '" alt="" class="w-full h-full object-cover">'
		else
			cover = '<div class="w-full h-full bg-neutral-800 flex items-center justify-center text-neutral-500 text-lg font-semibold">' .. h(
				getInitials(m.Title or "")
			) .. "</div>"
		end
		local stats = ""
		if m.TotalChapters and m.TotalChapters > 0 then
			stats = '<span class="text-sm"><span class="font-semibold text-indigo-400">' .. h(tostring(m.ReadChapters or 0))
				.. '</span><span class="text-neutral-500">/</span><span class="text-neutral-300">' .. h(tostring(m.TotalChapters)) .. "</span></span>"
		end
		return '<a href="/view/manga/' .. h(m.PluginID) .. "/" .. h(m.MangaID) .. '" class="flex items-center gap-3 bg-neutral-900 border border-neutral-800 rounded-lg p-2.5 hover:border-neutral-700 transition">'
			.. '<div class="w-12 h-16 rounded-md overflow-hidden shrink-0">' .. cover .. "</div>"
			.. '<div class="flex-1 min-w-0">'
			.. '<div class="flex items-center gap-2">'
			.. (badge or "")
			.. '<span class="text-sm font-medium truncate">' .. h(m.Title or "") .. "</span></div>"
			.. '<div class="flex items-center gap-1.5 mt-1 text-xs text-neutral-400">'
			.. icon
			.. "<span>" .. h(m.PluginName or "") .. "</span>"
			.. (stats ~= "" and ('<span class="text-neutral-600">·</span>' .. stats) or "")
			.. "</div></div></a>"
	end

	-- Updates section
	if #updates > 0 then
		body = body .. '<h2 class="text-lg font-semibold mb-3">New chapters</h2><div class="grid grid-cols-1 md:grid-cols-2 gap-2 mb-8">'
		for _, m in ipairs(updates) do
			body = body
				.. rowCard(m, '<span class="shrink-0 text-[10px] px-1.5 py-0.5 rounded-full bg-red-500 text-white font-medium">New</span>')
		end
		body = body .. "</div>"
	else
		body = body
			.. '<div class="mb-8 text-sm text-neutral-500">No new chapters — updates appear here after a library sync finds new chapters.</div>'
	end

	-- Recently added section
	if #recent > 0 then
		body = body .. '<h2 class="text-lg font-semibold mb-3">Added recently</h2><div class="grid grid-cols-1 md:grid-cols-2 gap-2">'
		for _, m in ipairs(recent) do
			body = body
				.. rowCard(m, '<span class="shrink-0 text-[10px] px-1.5 py-0.5 rounded-full bg-indigo-500/15 text-indigo-400">New title</span>')
		end
		body = body .. "</div>"
	end

	return body
end
