-- partials/detail_chapters.lua
-- Detail page: chapter list with a bulk-action dropdown, per-row actions and pagination.
-- Called as: detail_chapters(data) -> string (fragment)
-- data.PluginID, data.MangaID, data.Manga, data.Chapters, data.Progress,
-- data.ChPage, data.ChTotalPages

local pagination = require("partials.pagination")
local detailChapterRow = require("partials.detail_chapter_row")

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
		local actionsInner = '<form id="chapter-actions" method="post" action="/action/chapter-actions" class="flex flex-wrap items-center gap-2" data-confirm-actions="mark-selected-unread,set-up-to-read,set-up-to-unread,set-down-to-read,set-down-to-unread,mark-all-unread,clear-cache">'
			.. csrfInput(data.csrf_token or "")
			.. '<input type="hidden" name="pluginID" value="'
			.. h(pluginID)
			.. '">'
			.. '<input type="hidden" name="mangaID" value="'
			.. h(mangaID)
			.. '">'
			.. '<select name="action" class="bg-neutral-900 border border-neutral-700 text-neutral-200 rounded-md px-3 py-1.5 text-sm">'
			-- Grouped, because these write three unrelated flags (is_read,
			-- is_skipped, and the page-image cache) and a flat list of nine read as
			-- one operation. Labels name the flag and the scope, since the row
			-- buttons already call is_skipped "Skip".
			.. '<optgroup label="Ticked chapters">'
			.. '<option value="mark-selected-read">Mark ticked as read</option>'
			.. '<option value="mark-selected-unread">Mark ticked as unread</option>'
			.. '<option value="skip-selected">Skip ticked (ignore when continuing)</option>'
			.. '<option value="unskip-selected">Unskip ticked</option>'
			.. "</optgroup>"
			.. '<optgroup label="Ticked chapter and everything before it">'
			.. '<option value="set-up-to-read">Set up to as read</option>'
			.. '<option value="set-up-to-unread">Set up to as unread</option>'
			.. "</optgroup>"
			.. '<optgroup label="Ticked chapter and everything after it">'
			.. '<option value="set-down-to-read">Set down to as read</option>'
			.. '<option value="set-down-to-unread">Set down to as unread</option>'
			.. "</optgroup>"
			.. '<optgroup label="Whole manga (ticked chapters ignored)">'
			.. '<option value="mark-all-read">Mark all as read</option>'
			.. '<option value="mark-all-unread">Mark all as unread</option>'
			.. "</optgroup>"
			.. '<optgroup label="Storage">'
			.. '<option value="clear-cache">Clear this manga\'s cached page images</option>'
			.. "</optgroup>"
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
		local rowHTML = detailChapterRow({
			csrf_token = data.csrf_token or "",
			PluginID = pluginID,
			MangaID = mangaID,
			Manga = manga,
			Progress = progress,
			Chapter = c,
		})
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
