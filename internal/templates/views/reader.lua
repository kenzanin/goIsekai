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
	local readerPrefs = data.ReaderPrefs or ""

	local prevStyle = prevChapterID == "" and ' style="display:none"' or ""
	local nextStyle = nextChapterID == "" and ' style="display:none"' or ""

	-- Chapter title display
	local chapterDisplay
	if chNum > 0 then
		chapterDisplay = "Ch. " .. h(tostring(chNum))
		if chTitle ~= "" then
			chapterDisplay = chapterDisplay .. " — " .. h(chTitle)
		end
	else
		chapterDisplay = h(chTitle)
	end

	-- Build chapter menu items
	local chapterMenuItems = ""
	for _, c in ipairs(data.Chapters or {}) do
		local num = c.ChapterNum or 0
		local title = c.Title or ""
		local display = num > 0 and "Ch. " .. num or title
		if title ~= "" and num <= 0 then
			display = title
		elseif title ~= "" then
			display = display .. " — " .. title
		end
		local isSelected = c.ID == chapterID and "bg-indigo-600/20" or ""
		chapterMenuItems = chapterMenuItems .. '<button class="block w-full text-left px-3 py-2 hover:bg-indigo-600/20 ' .. isSelected .. '" data-chapter-id="' .. h(c.ID) .. '">' .. h(display) .. '</button>'
	end

	return [[<div id="reader" class="relative w-full h-screen"
  data-plugin-id="]] .. h(pluginID) .. [["
  data-manga-id="]] .. h(mangaID) .. [["
  data-chapter-id="]] .. h(chapterID) .. [["
  data-manga-title="]] .. h(mangaTitle) .. [["
  data-next-chapter-id="]] .. h(nextChapterID) .. [["
  data-prev-chapter-id="]] .. h(prevChapterID) .. [["
  data-manga-prefs="]] .. h(readerPrefs) .. [[">
  <!-- Top bar: persistent (back + page counter + controls) -->
  <div id="top-bar" class="absolute top-0 left-0 right-0 z-30 flex flex-wrap items-center gap-2 bg-neutral-950/85 backdrop-blur-sm px-3 py-2 text-sm transition-transform duration-300 ease-in-out">
  <a href="/view/manga/]] .. h(pluginID) .. "/" .. h(mangaID) .. [[" class="border border-neutral-700 hover:bg-neutral-800 active:scale-95 transition rounded-md px-3 py-1.5">← Back</a>
  <span class="ml-2 text-xs cursor-default" title="← → navigate · Space next · Esc back">⌨</span>
    <span class="text-neutral-500 truncate max-w-[25%]">]] .. h(mangaTitle) .. [[</span>
    <span id="page-counter" class="text-neutral-400">– / –</span>
    <div class="flex-1 flex justify-center min-w-0">
      <div class="relative group">
        <button id="chapter-jumper" class="text-neutral-300 truncate text-left cursor-pointer hover:bg-neutral-800 rounded px-2 py-1">
          ]] .. chapterDisplay .. [[
        </button>
        <div id="chapter-menu" class="absolute top-full left-0 mt-1 w-64 bg-neutral-900 border border-neutral-700 rounded-md shadow-xl z-50 hidden max-h-[60vh] overflow-y-auto">
          ]] .. chapterMenuItems .. [[
        </div>
      </div>
    </div>
    <div class="flex items-center gap-1">
      <button id="btn-fit" title="Fit mode" class="border border-neutral-700 hover:bg-neutral-800 active:scale-95 transition rounded-md px-3 py-1.5 text-xs">Fit W</button>
      <button id="btn-zoom-out" title="Zoom out" class="border border-neutral-700 hover:bg-neutral-800 active:scale-95 transition rounded-md px-3 py-1.5 text-xs">−</button>
      <button id="btn-zoom-in" title="Zoom in" class="border border-neutral-700 hover:bg-neutral-800 active:scale-95 transition rounded-md px-3 py-1.5 text-xs">＋</button>
      <button id="btn-strip" title="Vertical strip (continuous scroll)" class="border border-neutral-700 hover:bg-neutral-800 active:scale-95 transition rounded-md px-3 py-1.5 text-xs">⇕</button>
      <button id="btn-dir" title="Direction (RTL/LTR)" class="border border-neutral-700 hover:bg-neutral-800 active:scale-95 transition rounded-md px-3 py-1.5 text-xs">LTR</button>
    </div>
  </div>

  <!-- Click zones -->
  <div id="zone-top" class="absolute top-12 left-[35%] right-[35%] h-[18%] z-10 cursor-pointer" title="Previous chapter"></div>
  <div id="zone-bottom" class="absolute bottom-14 left-[35%] right-[35%] h-[18%] z-10 cursor-pointer" title="Next chapter"></div>
  <div id="zone-left" class="absolute top-12 bottom-14 left-0 w-[35%] z-10 cursor-pointer" title="Previous page"></div>
  <div id="zone-right" class="absolute top-12 bottom-14 right-0 w-[35%] z-10 cursor-pointer" title="Next page"></div>

  <!-- Canvas -->
  <canvas id="page-canvas" class="absolute inset-0 h-full w-full"></canvas>

  <!-- Vertical strip view (continuous scroll; next chapter parts append seamlessly) -->
  <div id="strip-view" class="absolute inset-0 overflow-y-auto overscroll-contain hidden pt-12 pb-16 bg-neutral-950"></div>

  <!-- Center click zone -->
  <div id="zone-center" class="absolute top-12 bottom-14 left-[35%] right-[35%] z-10 cursor-pointer" title="Toggle bars"></div>

  <!-- Spinner -->
  <div id="spinner" style="display:none" class="absolute inset-0 z-20 hidden items-center justify-center bg-neutral-950/70">
    <div class="h-10 w-10 animate-spin rounded-full border-2 border-neutral-700 border-t-indigo-500"></div>
  </div>

  <!-- Error panel -->
  <div id="error-panel" style="display:none" class="absolute top-12 bottom-14 left-0 right-0 z-40 hidden items-center justify-center pointer-events-none">
    <div class="bg-black/85 backdrop-blur-sm rounded-lg p-6 max-w-md w-[90%] flex flex-col items-center gap-3 pointer-events-auto">
      <p class="text-sm text-red-400">Failed to load page</p>
      <div class="flex gap-2">
        <button id="btn-retry" class="bg-indigo-600 hover:bg-indigo-500 rounded-md px-4 py-2 text-sm">↻ Retry</button>
        <button id="btn-skip" class="border border-neutral-700 hover:bg-neutral-800 rounded-md px-4 py-2 text-sm">→ Skip</button>
      </div>
    </div>
  </div>

  <!-- Bottom bar -->
  <div id="bottom-bar" class="absolute bottom-0 left-0 right-0 z-30 flex items-center gap-3 bg-neutral-950/85 backdrop-blur-sm px-3 py-2 text-sm transition-transform duration-300 ease-in-out">
    <a id="btn-prev-ch" href="/view/read/]] .. h(pluginID) .. [[" class="border border-neutral-700 hover:bg-neutral-800 active:scale-95 transition rounded-md px-3 py-1.5" ]] .. prevStyle .. [[>← Prev ch.</a>
    <button id="btn-prev-page" class="border border-neutral-700 hover:bg-neutral-800 active:scale-95 transition rounded-md px-3 py-1.5">‹</button>
    <div id="page-jumper" class="flex-1 h-8 flex items-center justify-center gap-0.5 cursor-pointer" title="Click to jump to page">
      <div class="flex-1 h-full relative bg-neutral-800 rounded overflow-hidden">
        <div id="page-progress" class="absolute top-0 left-0 h-full bg-indigo-500 transition-all duration-200"></div>
      </div>
    </div>
    <button id="btn-next-page" class="border border-neutral-700 hover:bg-neutral-800 active:scale-95 transition rounded-md px-3 py-1.5">›</button>
    <a id="btn-next-ch" href="/view/read/]] .. h(pluginID) .. [[" class="border border-neutral-700 hover:bg-neutral-800 active:scale-95 transition rounded-md px-3 py-1.5" ]] .. nextStyle .. [[>Next ch. →</a>
  </div>

  <!-- Progress line -->
  <div id="progress-line" class="absolute bottom-0 left-0 right-0 z-40 h-2 bg-indigo-500 shadow-[0_0_8px_2px_rgba(99,102,241,0.55)] transition-opacity duration-300 ease-in-out opacity-0"></div>

  <!-- Mini page dots — visible when bars are hidden, clickable page jumpers -->
  <div id="page-dots" class="absolute bottom-1 left-0 right-0 z-30 flex items-center justify-center gap-1 px-4 transition-opacity duration-300 opacity-0 pointer-events-none">
  </div>

</div>

<script src="/static/lib/reader.js" defer></script>
<script>
(function() {
  var title = ']] .. h(mangaTitle) .. [[';
  var chNum = ]] .. tostring(chNum) .. [[;
  var chTitle = ']] .. h(chTitle) .. [[';
  if (title) {
    var label = title;
    if (chNum > 0) { label += ' \\u2013 Ch. ' + chNum; if (chTitle) label += ': ' + chTitle; }
    else if (chTitle) { label += ' \\u2013 ' + chTitle; }
    document.title = label + ' \\u2013 goIsekai';
  }

  var dirBtn = document.getElementById('btn-dir');
  if (dirBtn) {
    var RTL_CLASSES = 'bg-indigo-600/20 border-indigo-500/40';
    function applyDirStyle() {
      var _root = document.getElementById('reader');
      var _scope = _root ? (_root.dataset.pluginId + '.' + _root.dataset.mangaId) : '';
      var _rtl = localStorage.getItem('gi_direction.' + _scope) === 'rtl' || localStorage.getItem('gi_direction') === 'rtl';
      if (_rtl) dirBtn.classList.add(...RTL_CLASSES.split(' '));
      else dirBtn.classList.remove(...RTL_CLASSES.split(' '));
    }
    applyDirStyle();
    dirBtn.addEventListener('click', function() {
      requestAnimationFrame(applyDirStyle);
    });
  }
})();
</script>
]]
end
