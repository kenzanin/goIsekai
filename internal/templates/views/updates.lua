-- views/updates.lua
-- Updates page: flat list of fresh chapter updates + recently added library titles.
-- Called as: updates(data) -> string (body HTML only, layout wraps it)

local pagination = require("partials.pagination")

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
		local title = entry.Title or ""
		local pluginID = entry.PluginID or ""
		local sourceMangaID = entry.MangaID or ""
		local coverURL = entry.CoverURL or ""
		local pluginIcon = entry.PluginIcon or ""
		local readChapters = entry.ReadChapters or 0
		local totalChapters = entry.TotalChapters or 0
		local badge
		local tsAttr
		local formattedDate

		tsAttr = h(tostring(entry.Date or ""))
		formattedDate = h(formatDate(tostring(entry.Date or "")))
		-- Badge follows the same flag that drives the [New] library badge.
		-- "New title" is just a recently-added row with no fresh chapters.
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
			rows[#rows + 1] = [[<div class="sticky top-0 z-10 -mx-2 py-2 px-2 bg-neutral-950/95 backdrop-blur text-sm font-semibold text-red-400 border-b border-neutral-800 flex items-center gap-2">
    <span class="w-2 h-2 rounded-full bg-red-500 animate-pulse"></span>New chapters</div>]]
			seenNew = true
		elseif not isNew and not seenRest then
			rows[#rows + 1] = [[<div class="sticky top-0 z-10 -mx-2 py-2 px-2 bg-neutral-950/95 backdrop-blur text-sm font-semibold text-neutral-300 border-b border-neutral-800">Library updates</div>]]
			seenRest = true
		end

		local coverHtml
		if coverURL ~= "" then
			coverHtml = string.format(
				'<img src="/image?pluginID=%s&amp;url=%s" alt="%s" class="w-24 aspect-[2/3] object-cover rounded shrink-0" loading="lazy" onerror="this.onerror=null;this.src=\'/static/img/placeholder.svg\';">',
				h(pluginID),
				h(coverURL),
				h(title)
			)
		else
			coverHtml = string.format(
				'<div class="w-24 aspect-[2/3] bg-neutral-800 rounded flex items-center justify-center text-neutral-500 text-base font-semibold shrink-0">%s</div>',
				h(getInitials(title))
			)
		end

		local iconHtml = ""
		if pluginIcon ~= "" then
			iconHtml = string.format('<img src="%s" alt="" class="h-4 w-4 rounded-sm object-cover">', h(pluginIcon))
		end

		rows[#rows + 1] = string.format(
			[[
    <a href="/view/manga/%s/%s" class="flex items-center gap-4 py-4 -mx-2 px-2 rounded hover:bg-neutral-900 transition">
        %s
        <div class="flex-1 min-w-0">
          <div class="flex items-center justify-between">
            <div class="text-base font-medium truncate">%s</div>
            <div class="text-sm text-neutral-500 shrink-0 hidden sm:block" data-ts="%s" title="%s">%s</div>
          </div>
          <div class="flex items-center gap-2 mt-1">
            <span class="text-sm text-neutral-400">%s/%s chapters</span>
            <span class="inline-flex items-center gap-1.5 text-sm px-2.5 py-1 rounded-full bg-neutral-800 text-neutral-500">%s%s</span>
            <div class="text-sm text-neutral-500 sm:hidden" data-ts="%s" title="%s">%s</div>
          </div>
        </div>
    </a>]],
			h(pluginID),
			h(sourceMangaID),
			coverHtml,
			h(title),
			tsAttr,
			formattedDate,
			formattedDate,
			h(tostring(readChapters)),
			h(tostring(totalChapters)),
			badge,
			iconHtml,
			tsAttr,
			formattedDate,
			formattedDate
		)
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
