-- views/history.lua
-- Reading history page. Replaces views/history.jet.
-- Called as: history(data) -> string (body HTML only, layout wraps it)

local pagination = require("partials.pagination")

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
		local title = entry.Title or ""
		local pluginID = entry.PluginID or ""
		local sourceMangaID = entry.SourceMangaID or ""
		local coverURL = entry.CoverURL or ""
		local pluginIcon = entry.PluginIcon or ""
		local pluginName = entry.PluginName or ""
		local readChapters = entry.ReadChapters or 0
		local totalChapters = entry.TotalChapters or 0
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
			rows[#rows + 1] = string.format(
				'<div class="sticky top-0 z-10 -mx-2 py-2 px-2 bg-neutral-950/95 backdrop-blur text-sm font-semibold text-neutral-300 border-b border-neutral-800">%s</div>',
				h(dayKey)
			)
			lastDay = dayKey
		end

		local coverHtml
		if coverURL ~= "" then
			coverHtml = string.format(
				'<img src="/image?pluginID=%s&amp;url=%s" alt="%s" class="w-24 aspect-[2/3] object-cover rounded shrink-0" loading="lazy">',
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
			iconHtml,
			h(pluginName),
			tsAttr,
			formattedDate,
			formattedDate
		)
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
