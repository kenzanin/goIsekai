-- partials/library_cards.lua
-- Library page: manga grid/list cards with badges, stats and pagination.
-- Called as: library_cards(data) -> string
-- data.Mangas, data.LibraryStats, data.Ratios, data.Page, data.TotalPages

local pagination = require("partials.pagination")

return function(data)
	local mangas = data.Mangas or {}
	local libraryStats = data.LibraryStats or {}
	local ratios = data.Ratios or {}
	local page = data.Page or 1
	local totalPages = data.TotalPages or 1

	-- Build manga cards
	local mangaCards = ""
	if #mangas == 0 then
		mangaCards = [[<div class="py-16 text-center text-neutral-500">
    <svg class="w-14 h-14 mx-auto mb-4 text-neutral-700" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M5 5h6v14H5zM13 5h6v3h-6z"/></svg>
    <p class="mb-4">Your library is empty — search for manga first</p>
    <a href="/view/search" class="inline-block bg-indigo-600 hover:bg-indigo-500 text-white rounded-md px-4 py-2 text-sm font-medium">Search manga</a>
</div>]]
	else
		mangaCards = '<div class="view-container grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-4">'
		for _, m in ipairs(mangas) do
			local mid = m.ID or ""
			local pluginID = m.PluginID or ""
			local sourceMangaID = m.SourceMangaID or ""
			local title = m.Title or ""
			local coverURL = m.CoverURL or ""
			local status = m.Status or ""
			local statsObj = libraryStats[mid]
			local ratio = ratios[pluginID] or 0

			local coverHTML
			if coverURL ~= "" then
				if ratio > 0 then
					coverHTML = '<img src="/image?pluginID='
						.. h(pluginID)
						.. "&amp;url="
						.. h(coverURL)
						.. '" alt="'
						.. h(title)
						.. '" style="aspect-ratio: '
						.. h(tostring(ratio))
						.. '" class="w-full object-cover" loading="lazy">'
				else
					coverHTML = '<img src="/image?pluginID='
						.. h(pluginID)
						.. "&amp;url="
						.. h(coverURL)
						.. '" alt="'
						.. h(title)
						.. '" class="w-full aspect-[2/3] object-cover" loading="lazy">'
				end
			else
				coverHTML = '<div class="w-full aspect-[2/3] bg-neutral-800 flex items-center justify-center text-neutral-500 text-2xl font-semibold">'
					.. h(getInitials(title))
					.. "</div>"
			end

			local pluginName = statsObj and (statsObj.PluginName or "") or ""
			local pluginIcon = statsObj and (statsObj.PluginIcon or "") or ""

			-- Badges overlaid on the thumbnail
			local statusBadge = status ~= ""
					and ('<span class="absolute top-2 left-2 bg-black/60 backdrop-blur rounded-full px-2 py-0.5 text-xs text-neutral-200">' .. h(
						status
					) .. "</span>")
				or ""
			local pluginBadge = ""
			if pluginName ~= "" then
				local pIcon = pluginIcon ~= ""
						and ('<img src="' .. h(pluginIcon) .. '" alt="" class="h-3 w-3 rounded-sm object-cover">')
					or ""
				pluginBadge = '<span class="absolute bottom-2 left-2 bg-black/60 backdrop-blur rounded-full px-2 py-0.5 text-[11px] text-neutral-200 flex items-center gap-1 max-w-[90%]">'
					.. pIcon
					.. '<span class="truncate">'
					.. h(pluginName)
					.. "</span></span>"
			end

			-- read/total in the title area
			local statsBadge = ""
			if statsObj then
				statsBadge = '<span class="text-sm font-semibold text-indigo-400">'
					.. h(tostring(statsObj.ReadChapters or 0))
					.. '</span><span class="text-xs text-neutral-500">/</span><span class="text-sm font-medium text-neutral-300">'
					.. h(tostring(statsObj.TotalChapters or 0))
					.. "</span>"
					.. (
						statsObj.HasNew
							and ' <span class="text-[10px] px-1.5 py-0.5 rounded-full bg-red-500 text-white font-medium align-middle">New</span>'
						or ""
					)
			end

			mangaCards = mangaCards
				.. '<a href="/view/manga/'
				.. h(pluginID)
				.. "/"
				.. h(sourceMangaID)
				.. '" class="view-item bg-neutral-900 rounded-lg overflow-hidden relative hover:-translate-y-0.5 hover:shadow-lg hover:shadow-black/40 transition flex flex-col">'
				.. '<div class="relative flex-1 min-w-0" data-key="'
				.. h(pluginID)
				.. ":"
				.. h(sourceMangaID)
				.. '">'
				.. coverHTML
				.. '<div class="lib-dim" style="display:none;position:absolute;inset:0;background:rgba(0,0,0,0.82);border-radius:0.5rem;"></div>'
				.. statusBadge
				.. pluginBadge
				.. "</div>"
				.. '<div class="p-3 flex flex-col gap-1">'
				.. '<div class="text-sm font-medium line-clamp-2" title="'
				.. h(title)
				.. '">'
				.. h(title)
				.. "</div>"
				.. '<div class="flex items-center gap-1">'
				.. statsBadge
				.. "</div>"
				.. '<div class="view-meta hidden items-center gap-1.5 text-xs flex-wrap">'
				.. (status ~= "" and ('<span class="px-1.5 py-0.5 rounded-full bg-neutral-700/50 text-neutral-400">' .. h(
					status
				) .. "</span>") or "")
				.. (pluginName ~= "" and ('<span class="inline-flex items-center gap-1 text-neutral-400">' .. (pluginIcon ~= "" and ('<img src="' .. h(
					pluginIcon
				) .. '" alt="" class="h-3 w-3 rounded-sm object-cover shrink-0">') or "") .. h(pluginName) .. "</span>") or "")
				.. "</div>"
				.. "</div></a>"
		end
		mangaCards = mangaCards
			.. "</div>"
			.. pagination({
				Pagination = { Base = "/", Param = "page", Current = page, Total = totalPages },
			})
	end

	return mangaCards
end
