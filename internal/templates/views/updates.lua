-- views/updates.lua
-- Updates page: flat list of fresh chapter updates + recently added library titles.
-- Called as: updates(data) -> string (body HTML only, layout wraps it)

local pagination = require("partials.pagination")
local row = require("partials.manga_list_row")

return function(data)
	local allItems = data.Items or {}
	local page = data.Page or 1
	local totalPages = data.TotalPages or 1

	if #allItems == 0 then
		return [[
<div class="py-16 text-center text-neutral-500">
    <p class="mb-2">No updates yet</p>
    <a href="/view/library" class="text-indigo-400 hover:text-indigo-300 text-sm">Go to library</a>
</div>]]
	end

	local rows = {}
	local seenNew = false
	local seenRest = false
	for _, entry in ipairs(allItems) do
		local tsAttr = h(tostring(entry.Date or ""))
		local formattedDate = h(formatDate(tostring(entry.Date or "")))
		-- Badge follows the same flag that drives the [New] library badge.
		-- "New title" is just a recently-added row with no fresh chapters.
		local badge
		if entry.HasNew == true then
			badge =
				'<span class="shrink-0 text-xs px-2 py-0.5 rounded-full bg-red-500 text-white font-medium">New</span>'
		else
			badge =
				'<span class="shrink-0 text-xs px-2 py-0.5 rounded-full bg-indigo-500/15 text-indigo-400">New title</span>'
		end

		-- Two fixed sections matching the visible badge: rows that would show
		-- the red New badge first, everything else after. Headers emit once
		-- per section (repeating per page boundary).
		local isNew = entry.HasNew == true
		if isNew and not seenNew then
			rows[#rows + 1] = row({
				Which  = "section",
				Label  = "New chapters",
				Class  = "text-red-400",
				Inner  = '<span class="w-2 h-2 rounded-full bg-red-500 animate-pulse"></span> ',
			})
			seenNew = true
		elseif not isNew and not seenRest then
			rows[#rows + 1] = row({ Which = "section", Label = "Library updates" })
			seenRest = true
		end

		rows[#rows + 1] = row({
			Which = "row",
			Entry = {
				PluginID      = entry.PluginID,
				SourceMangaID = entry.MangaID,
				Title         = entry.Title,
				CoverURL      = entry.CoverURL,
				ReadChapters  = entry.ReadChapters,
				TotalChapters = entry.TotalChapters,
				TsAttr        = tsAttr,
				FormattedDate = formattedDate,
				Badge         = badge,
			},
			Opts = { onerrorCover = true },
		})
	end

	local pag = ""
	if totalPages > 1 then
		pag = pagination({
			Pagination = { Base = "/view/updates", Param = "page", Current = page, Total = totalPages },
		})
	end

	local stats = data.Stats or {}
	local chips = ""
	if next(stats) then
		local function chip(label, value)
			return string.format(
				'<span class="text-xs px-2.5 py-1 rounded-full bg-neutral-800/80 text-neutral-400">%s <span class="text-neutral-200 font-medium">%d</span></span>',
				label,
				value or 0
			)
		end
		chips = table.concat({
			chip("titles", stats.Total),
			chip("checked &lt;24h", stats.Checked24h),
			chip("new chapters", stats.Fresh),
		})
	end

	local listHTML = string.format(
		[[
<h1 class="text-xl font-semibold mb-3">Updates</h1>
<div class="flex flex-wrap gap-2 mb-6">%s</div>
%s
<div class="divide-y divide-neutral-800">
%s
</div>]],
		chips,
		pag,
		table.concat(rows, "\n")
	)

	return listHTML .. pag
end
