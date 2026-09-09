-- views/reader.lua
-- Reader shell (blank layout). Replaces views/reader.jet.
-- Called as: reader(data) -> string (body HTML only, blank layout wraps it)
-- data keys: PluginID, MangaID, Manga (types.Manga), ChapterID,
--   Chapters, CurrentChapter (types.Chapter), PrevChapterID, NextChapterID

return function(data)
	local pluginID = data.PluginID or ""
	local mangaID = data.MangaID or ""
	local chapterID = data.ChapterID or ""
	local manga = data.Manga or {}
	local mangaTitle = manga.Title or ""
	local currentChapter = data.CurrentChapter or {}
	local chNum = currentChapter.ChapterNum or 0
	local chTitle = currentChapter.Title or ""
	local prevChapterID = data.PrevChapterID or ""
	local nextChapterID = data.NextChapterID or ""

	local parts = {}
	local function emit(s)
		parts[#parts + 1] = s
	end

	-- Reader container with data-* attributes for reader.js
	emit('<div id="reader" class="relative w-full h-screen"')
	emit('  data-plugin-id="' .. h(pluginID) .. '"')
	emit('  data-manga-id="' .. h(mangaID) .. '"')
	emit('  data-chapter-id="' .. h(chapterID) .. '"')
	emit('  data-manga-title="' .. h(mangaTitle) .. '"')
	emit('  data-next-chapter-id="' .. h(nextChapterID) .. '"')
	emit('  data-prev-chapter-id="' .. h(prevChapterID) .. '">')

	-- Top bar
	emit("  <!-- Top bar: persistent (back + page counter + controls) -->")
	emit(
		'  <div id="top-bar" class="absolute top-0 left-0 right-0 z-30 flex flex-wrap items-center gap-2 bg-neutral-950/85 backdrop-blur-sm px-3 py-2 text-sm transition-transform duration-300 ease-in-out">'
	)
	emit(
		'  <a href="/view/manga/'
			.. h(pluginID)
			.. "/"
			.. h(mangaID)
			.. '" class="border border-neutral-700 hover:bg-neutral-800 active:scale-95 transition rounded-md px-3 py-1.5">← Back</a>'
	)
	emit('  <span class="ml-2 text-xs cursor-default" title="← → navigate · Space next · Esc back">⌨</span>')
	emit('    <span class="text-neutral-500 truncate max-w-[25%]">' .. h(mangaTitle) .. "</span>")
	emit('    <span id="page-counter" class="text-neutral-400">– / –</span>')
	emit('    <div class="flex-1 flex justify-center min-w-0">')
	emit('      <span id="chapter-title" class="text-neutral-300 truncate">')
	if chNum > 0 then
		emit("        Ch. " .. h(tostring(chNum)))
		if chTitle ~= "" then
			emit(" — " .. h(chTitle))
		end
	else
		emit("        " .. h(chTitle))
	end
	emit("      </span>")
	emit("    </div>")
	emit('    <div class="flex items-center gap-1">')
	emit(
		'      <button id="btn-fit" title="Fit mode" class="border border-neutral-700 hover:bg-neutral-800 active:scale-95 transition rounded-md px-3 py-1.5 text-xs">Fit W</button>'
	)
	emit(
		'      <button id="btn-zoom-out" title="Zoom out" class="border border-neutral-700 hover:bg-neutral-800 active:scale-95 transition rounded-md px-3 py-1.5 text-xs">−</button>'
	)
	emit(
		'      <button id="btn-zoom-in" title="Zoom in" class="border border-neutral-700 hover:bg-neutral-800 active:scale-95 transition rounded-md px-3 py-1.5 text-xs">＋</button>'
	)
	emit(
		'      <button id="btn-dir" title="Direction (RTL/LTR)" class="border border-neutral-700 hover:bg-neutral-800 active:scale-95 transition rounded-md px-3 py-1.5 text-xs">LTR</button>'
	)
	emit("    </div>")
	emit("  </div>")

	-- Click zones
	emit("  <!-- Click zones: left/right = page, top/bottom strip = chapter -->")
	emit(
		'  <div id="zone-top" class="absolute top-12 left-[35%] right-[35%] h-[18%] z-10 cursor-pointer" title="Previous chapter"></div>'
	)
	emit(
		'  <div id="zone-bottom" class="absolute bottom-14 left-[35%] right-[35%] h-[18%] z-10 cursor-pointer" title="Next chapter"></div>'
	)
	emit(
		'  <div id="zone-left" class="absolute top-12 bottom-14 left-0 w-[35%] z-10 cursor-pointer" title="Previous page"></div>'
	)
	emit(
		'  <div id="zone-right" class="absolute top-12 bottom-14 right-0 w-[35%] z-10 cursor-pointer" title="Next page"></div>'
	)

	-- Canvas
	emit("  <!-- Canvas -->")
	emit('  <canvas id="page-canvas" class="absolute inset-0 h-full w-full"></canvas>')

	-- Center click zone
	emit("  <!-- Center click zone: toggles bars visibility (before side zones in DOM so nav strips win overlaps) -->")
	emit(
		'  <div id="zone-center" class="absolute top-12 bottom-14 left-[35%] right-[35%] z-10 cursor-pointer" title="Toggle bars"></div>'
	)

	-- Spinner
	emit("  <!-- Spinner -->")
	emit('  <div id="spinner" class="absolute inset-0 z-20 hidden items-center justify-center bg-neutral-950/70">')
	emit('    <div class="h-10 w-10 animate-spin rounded-full border-2 border-neutral-700 border-t-indigo-500"></div>')
	emit("  </div>")

	-- Error panel
	emit(
		'  <!-- Error card: centered between toolbars, non-blocking. Inline display:none keeps the "Failed to load page" flash invisible until the stylesheet applies (Tailwind `hidden` alone leaves it visible pre-CSS). -->'
	)
	emit(
		'  <div id="error-panel" style="display:none" class="absolute top-12 bottom-14 left-0 right-0 z-40 hidden items-center justify-center pointer-events-none">'
	)
	emit(
		'    <div class="bg-black/85 backdrop-blur-sm rounded-lg p-6 max-w-md w-[90%] flex flex-col items-center gap-3 pointer-events-auto">'
	)
	emit('      <p class="text-sm text-red-400">Failed to load page</p>')
	emit('      <div class="flex gap-2">')
	emit(
		'        <button id="btn-retry" class="bg-indigo-600 hover:bg-indigo-500 rounded-md px-4 py-2 text-sm">↻ Retry</button>'
	)
	emit(
		'        <button id="btn-skip" class="border border-neutral-700 hover:bg-neutral-800 rounded-md px-4 py-2 text-sm">→ Skip</button>'
	)
	emit("      </div>")
	emit("    </div>")
	emit("  </div>")

	-- Bottom bar
	emit("  <!-- Bottom bar: chapter nav + page slider -->")
	emit(
		'  <div id="bottom-bar" class="absolute bottom-0 left-0 right-0 z-30 flex items-center gap-3 bg-neutral-950/85 backdrop-blur-sm px-3 py-2 text-sm transition-transform duration-300 ease-in-out">'
	)
	local prevStyle = prevChapterID == "" and ' style="display:none"' or ""
	emit(
		'    <a id="btn-prev-ch" href="/view/read/'
			.. h(pluginID)
			.. "/"
			.. h(mangaID)
			.. "/"
			.. h(prevChapterID)
			.. '" class="border border-neutral-700 hover:bg-neutral-800 active:scale-95 transition rounded-md px-3 py-1.5"'
			.. prevStyle
			.. ">← Prev ch.</a>"
	)
	emit(
		'    <button id="btn-prev-page" class="border border-neutral-700 hover:bg-neutral-800 active:scale-95 transition rounded-md px-3 py-1.5">‹</button>'
	)
	emit('    <input id="page-slider" type="range" min="1" value="1" class="h-2 flex-1 accent-indigo-500" />')
	emit(
		'    <button id="btn-next-page" class="border border-neutral-700 hover:bg-neutral-800 active:scale-95 transition rounded-md px-3 py-1.5">›</button>'
	)
	local nextStyle = nextChapterID == "" and ' style="display:none"' or ""
	emit(
		'    <a id="btn-next-ch" href="/view/read/'
			.. h(pluginID)
			.. "/"
			.. h(mangaID)
			.. "/"
			.. h(nextChapterID)
			.. '" class="border border-neutral-700 hover:bg-neutral-800 active:scale-95 transition rounded-md px-3 py-1.5"'
			.. nextStyle
			.. ">Next ch. →</a>"
	)
	emit("  </div>")

	-- Progress line
	emit("  <!-- Progress line: visible when bars are hidden -->")
	emit(
		'  <div id="progress-line" class="absolute bottom-0 left-0 right-0 z-40 h-2 bg-indigo-500 shadow-[0_0_8px_2px_rgba(99,102,241,0.55)] transition-opacity duration-300 ease-in-out opacity-0"></div>'
	)

	emit("</div>")

	-- reader.js
	emit('<script src="/static/lib/reader.js" defer></script>')
	emit("<script>")
	emit("(function() {")
	emit("  var title = '" .. h(mangaTitle) .. "';")
	emit("  var chNum = " .. tostring(chNum) .. ";")
	emit("  var chTitle = '" .. h(chTitle) .. "';")
	emit("  if (title) {")
	emit("    var label = title;")
	emit("    if (chNum > 0) { label += ' \\u2013 Ch. ' + chNum; if (chTitle) label += ': ' + chTitle; }")
	emit("    else if (chTitle) { label += ' \\u2013 ' + chTitle; }")
	emit("    document.title = label + ' \\u2013 goIsekai';")
	emit("  }")
	emit("")
	emit("  var dirBtn = document.getElementById('btn-dir');")
	emit("  if (dirBtn) {")
	emit("    var RTL_CLASSES = 'bg-indigo-600/20 border-indigo-500/40';")
	emit("    function applyDirStyle() {")
	emit("      if (localStorage.getItem('gi_direction') === 'rtl') dirBtn.classList.add(...RTL_CLASSES.split(' '));")
	emit("      else dirBtn.classList.remove(...RTL_CLASSES.split(' '));")
	emit("    }")
	emit("    applyDirStyle();")
	emit("    dirBtn.addEventListener('click', function() {")
	emit("      requestAnimationFrame(applyDirStyle);")
	emit("    });")
	emit("  }")
	emit("})();")
	emit("</script>")

	return table.concat(parts, "\n")
end
