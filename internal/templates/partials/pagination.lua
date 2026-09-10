-- partials/pagination.lua
-- Numbered page navigation. Replaces partials/pagination.jet.
-- Called as: pagination(data) -> string
-- data.Pagination mirrors the Go Pagination struct:
--   { Base, Param, Current, Total, Extra, Inner, Compact }
-- Inner (optional): raw (already-escaped) HTML rendered at the LEFT of the
-- page row so external controls (e.g. the chapter-actions dropdown) share the
-- same line as the page numbers. Without Inner the numbers stay centered.
-- Compact (optional): no outer margins + tighter buttons for inline toolbar use.

local BTN_BASE = "border border-neutral-700 hover:bg-neutral-800 rounded-md px-2 py-1 text-sm"
local BTN_NUM = "border border-neutral-700 hover:bg-neutral-800 text-neutral-300 rounded-md px-2.5 py-1 text-sm"
local BTN_CURRENT = "bg-indigo-500 text-white rounded-md px-2.5 py-1 text-sm"
local BTN_DISABLED =
	"border border-neutral-700 text-neutral-600 rounded-md px-2 py-1 text-sm opacity-50 cursor-not-allowed"
local ELLIPSIS = "px-1.5 text-neutral-500"

-- Tighter button classes for the compact (inline toolbar) variant.
local C_BASE = "border border-neutral-700 hover:bg-neutral-800 rounded px-1.5 py-0.5 text-xs"
local C_NUM = "border border-neutral-700 hover:bg-neutral-800 text-neutral-300 rounded px-1.5 py-0.5 text-xs"
local C_CURRENT = "bg-indigo-500 text-white rounded px-1.5 py-0.5 text-xs"
local C_DISABLED =
	"border border-neutral-700 text-neutral-600 rounded px-1.5 py-0.5 text-xs opacity-50 cursor-not-allowed"

return function(data)
	local p = data.Pagination
	if not p or not p.Total or p.Total <= 1 then
		return ""
	end

	local compact = p.Compact == true

	local function cls(standard, small)
		if compact then
			return small
		end
		return standard
	end

	local parts = {}
	local function emit(s)
		parts[#parts + 1] = s
	end

	local inner = p.Inner or ""
	if inner ~= "" then
		-- Two-zone toolbar: [inner controls] on the left, page numbers on the right.
		emit('<div class="flex items-center gap-3 mt-4 mb-4 flex-wrap">')
		emit('<div class="flex items-center gap-2 flex-wrap shrink-0">' .. inner .. "</div>")
		emit('<div class="ml-auto flex items-center gap-1.5 flex-wrap">')
	elseif compact then
		emit('<nav class="flex items-center gap-1" aria-label="Pagination">')
	else
		emit('<div class="flex items-center justify-center gap-2 mt-4 mb-4">')
	end

	-- prev
	if p.Current > 1 then
		emit('  <a href="' .. h(pageURL(p, p.Current - 1)) .. '" class="' .. h(cls(BTN_BASE, C_BASE)) .. '">←</a>')
	else
		emit('  <span class="' .. h(cls(BTN_DISABLED, C_DISABLED)) .. '">←</span>')
	end

	-- numbered pages
	local pages = pageWindow(p.Current, p.Total)
	for _, n in ipairs(pages) do
		if n == 0 then
			emit('  <span class="' .. h(ELLIPSIS) .. '">…</span>')
		elseif n == p.Current then
			emit('  <span class="' .. h(cls(BTN_CURRENT, C_CURRENT)) .. '">' .. tostring(n) .. "</span>")
		else
			emit(
				'  <a href="'
					.. h(pageURL(p, n))
					.. '" class="'
					.. h(cls(BTN_NUM, C_NUM))
					.. '" title="Page '
					.. tostring(n)
					.. '">'
					.. tostring(n)
					.. "</a>"
			)
		end
	end

	-- next
	if p.Current < p.Total then
		emit('  <a href="' .. h(pageURL(p, p.Current + 1)) .. '" class="' .. h(cls(BTN_BASE, C_BASE)) .. '">→</a>')
	else
		emit('  <span class="' .. h(cls(BTN_DISABLED, C_DISABLED)) .. '">→</span>')
	end

	if compact then
		emit("</nav>")
	else
		emit("</div>")
	end
	if inner ~= "" then
		emit("</div>")
	end
	return table.concat(parts, "\n")
end
