-- views/history.lua
-- Reading history page. Replaces views/history.jet.
-- Called as: history(data) -> string (body HTML only, layout wraps it)

local pagination = require("partials.pagination")
local row = require("partials.manga_list_row")

return function(data)
	local history = data.History or {}
	local page = data.Page or 1
	local totalPages = data.TotalPages or 1

	if #history == 0 then
		return [[
<div class="py-16 text-center text-neutral-500">
    <p class="mb-2">No reading history yet</p>
    <a href="/" class="text-indigo-400 hover:text-indigo-300 text-sm">Go to library</a>
</div>]]
	end

	local rows = {}
	local lastDay = nil
	for _, entry in ipairs(history) do
		local lastReadAt = entry.LastReadAt or ""
		local tsAttr = ""
		local formattedDate = ""
		local dayKey = ""
		if lastReadAt ~= "" then
			tsAttr = h(tostring(lastReadAt))
			formattedDate = h(formatDate(tostring(lastReadAt)))
			dayKey = formatDate(tostring(lastReadAt))
		else
			formattedDate = "—"
		end

		if dayKey ~= "" and dayKey ~= lastDay then
			rows[#rows + 1] = row({ Which = "section", Label = dayKey })
			lastDay = dayKey
		end

		-- Badge: plugin icon + name, same shape as the updates badge slot.
		local iconHtml = ""
		if entry.PluginIcon ~= "" then
			iconHtml = string.format('<img src="%s" alt="" class="h-4 w-4 rounded-sm object-cover">', h(entry.PluginIcon))
		end
		local badge = iconHtml .. h(entry.PluginName or "")

		rows[#rows + 1] = row({
			Which = "row",
			Entry = {
				PluginID = entry.PluginID,
				SourceMangaID = entry.SourceMangaID,
				Title = entry.Title,
				CoverURL = entry.CoverURL,
				ReadChapters = entry.ReadChapters,
				TotalChapters = entry.TotalChapters,
				TsAttr = tsAttr,
				FormattedDate = formattedDate,
				Badge = badge,
			},
		})
	end

	local pag = ""
	if totalPages > 1 then
		pag = pagination({
			Pagination = { Base = "/view/history", Param = "page", Current = page, Total = totalPages },
		})
	end

	local listHTML = string.format(
		[[
<h1 class="text-xl font-semibold mb-6">History</h1>
%s
<div class="divide-y divide-neutral-800">
%s
</div>]],
		pag,
		table.concat(rows, "\n")
	)

	return listHTML .. pag
end
