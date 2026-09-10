-- partials/detail_chapters.lua
-- Detail page: chapter list with a bulk-action dropdown, per-row actions and pagination.
-- Called as: detail_chapters(data) -> string (fragment)
-- data.PluginID, data.MangaID, data.Manga, data.Chapters, data.Progress,
-- data.ChPage, data.ChTotalPages

local pagination = require("partials.pagination")

return function(data)
	local pluginID = data.PluginID or ""
	local mangaID = data.MangaID or ""
	local manga = data.Manga or {}
	local chapters = data.Chapters or {}
	local progress = data.Progress or {}
	local chPage = data.ChPage or 1
	local chTotalPages = data.ChTotalPages or 1

	local body = ""

	-- Chapters section
	body = body .. '<h2 class="text-xl font-semibold mb-4">Chapters</h2>'

	if #chapters == 0 then
		body = body .. '<div class="py-16 text-center text-neutral-500">No chapters yet</div>'
	else
		-- Action dropdown (bulk progress + cache). Rendered inside the top
		-- pagination row via its Inner slot so the dropdown shares a line with the
		-- page numbers instead of sitting in its own row above them.
		local actionsInner = '<form id="chapter-actions" method="post" action="/action/chapter-actions" class="flex flex-wrap items-center gap-2" data-confirm-actions="mark-selected-unread,clear-up-to,mark-all-unread,clear-cache">'
			.. '<input type="hidden" name="pluginID" value="'
			.. h(pluginID)
			.. '">'
			.. '<input type="hidden" name="mangaID" value="'
			.. h(mangaID)
			.. '">'
			.. '<select name="action" class="bg-neutral-900 border border-neutral-700 text-neutral-200 rounded-md px-3 py-1.5 text-sm">'
			.. '<option value="mark-selected-read">Mark selected as read</option>'
			.. '<option value="mark-selected-unread">Mark selected as unread</option>'
			.. '<option value="mark-up-to">Mark up to selected</option>'
			.. '<option value="clear-up-to">Clear up to selected</option>'
			.. '<option value="mark-all-read">Mark all as read</option>'
			.. '<option value="mark-all-unread">Mark all as unread</option>'
			.. '<option value="clear-cache">Clear cache</option>'
			.. "</select>"
			.. '<button type="submit" class="bg-indigo-600 hover:bg-indigo-500 text-white rounded-md px-3 py-1.5 text-sm font-medium">GO</button>'
			.. "</form>"

		-- Chapter pagination (top) — the dropdown is inline on the left, numbers on the right.
		-- Fallback: with only one chapter page the pagination (and its Inner slot)
		-- would render nothing, so the standalone form row is kept for that case.
		if chTotalPages > 1 then
			body = body
				.. pagination({
					Pagination = {
						Base = "",
						Param = "ChPage",
						Current = chPage,
						Total = chTotalPages,
						Inner = actionsInner,
					},
				})
		else
			body = body .. '<div class="flex mb-4">' .. actionsInner .. "</div>"
		end

		-- Chapter list
		local chaptersHTML = '<div class="divide-y divide-neutral-800">'
		for i, c in ipairs(chapters) do
			local cID = c.ID or ""
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
				.. '" form="chapter-actions" class="size-4 accent-indigo-600 shrink-0">'
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
