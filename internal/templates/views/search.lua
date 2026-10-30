-- views/search.lua
-- Search form and results grid with plugin selector.
-- Replaces views/search.jet.
-- Called as: search(data) -> string (body HTML only, layout wraps it)
-- data.Plugins: []database.Plugin, data.Q: string, data.PluginID: string
-- data.Results: []types.Manga, data.Page: int, data.TotalPages: int
-- data.ThumbRatio: float, data.Challenge: bool

local pagination = require("partials.pagination")
local verifyWizard = require("partials.verify_wizard")

return function(data)
	local plugins = data.Plugins or {}
	local q = data.Q or ""
	local pluginID = data.PluginID or ""
	local results = data.Results or {}
	local page = data.Page or 1
	local totalPages = data.TotalPages or 1
	local challenge = data.Challenge or false
	local genres = data.Genres or {}
	local genre = data.Genre or ""
	-- GenresSelected is every genre the request asked for (genre is repeatable:
	-- ?genre=a&genre=b). `genre` is just the first one, kept for display.
	local genresSelected = data.GenresSelected or {}
	-- Cache-buster per cover URL, same contract the detail page uses: /image is
	-- cached for a week under an unversioned URL, so the grid must vary it.
	local coverVersions = data.CoverVersions or {}

	-- Build plugin options
	local pluginOpts = ""
	for _, p in ipairs(plugins) do
		if p.IsActive then
			local selected = (pluginID == p.ID) and " selected" or ""
			pluginOpts = pluginOpts .. " <option value=" .. h(p.ID) .. selected .. ">" .. h(p.Name) .. "</option>\n"
		end
	end

	-- Get plugin name + icon for badge
	local pluginName = ""
	local pluginIcon = ""
	for _, p in ipairs(plugins) do
		if p.ID == pluginID then
			pluginName = p.Name
			pluginIcon = p.IconURL or ""
			break
		end
	end
	local pluginBadge = ""
	if pluginName ~= "" then
		pluginBadge =
			'<span class="inline-flex items-center gap-1.5 text-xs px-2 py-0.5 rounded-full bg-neutral-800 text-neutral-500">'
		if pluginIcon ~= "" then
			pluginBadge = pluginBadge
				.. '<img src="'
				.. h(pluginIcon)
				.. '" alt="" class="h-3.5 w-3.5 rounded-sm object-cover">'
		end
		pluginBadge = pluginBadge .. h(pluginName) .. "</span>"
	end

	-- Build genre checkboxes (only when the selected plugin advertises genres).
	-- Rendered as a multi-column checkbox grid below the search bar; the area is
	-- hidden entirely for plugins without genre support (genreOpts == "").
	-- Browsers submit every checked [name="genre"] as separate ?genre=<slug> params.
	local genreBoxes = ""
	if pluginID ~= "" and #genres > 0 then
		for _, g in ipairs(genres) do
			local isSel = false
			for _, sel in ipairs(genresSelected) do
				if sel == g.Slug then
					isSel = true
					break
				end
			end
			genreBoxes = genreBoxes
				.. '<div class="flex items-center gap-2">'
				.. '<input type="checkbox" id="genre_'
				.. h(g.Slug)
				.. '" name="genre" value="'
				.. h(g.Slug)
				.. '"'
				.. (isSel and " checked" or "")
				.. ' class="rounded bg-neutral-800 border-neutral-600 text-indigo-600 focus:ring-indigo-500">'
				.. '<label for="genre_'
				.. h(g.Slug)
				.. '" class="text-sm text-neutral-300 cursor-pointer">'
				.. h(g.Name)
				.. "</label></div>"
		end
	end

	-- Build result rows
	local rows = {}
	for _, r in ipairs(results) do
		local title = r.Title or ""
		local coverURL = r.CoverURL or ""
		local mangaID = r.ID or ""

		local coverHTML
		if coverURL ~= "" then
			coverHTML = '<img src="/image?pluginID='
				.. h(pluginID)
				.. "&amp;url="
				.. h(coverURL)
				.. (coverVersions[coverURL] and coverVersions[coverURL] ~= "" and ("&amp;cb=" .. h(coverVersions[coverURL])) or "")
				.. '" alt="'
				.. h(title)
				.. '" class="w-full aspect-[2/3] object-cover" loading="lazy">'
		else
			coverHTML = '<div class="w-full aspect-[2/3] bg-neutral-800 flex items-center justify-center text-neutral-500 text-2xl font-semibold">'
				.. h(getInitials(title))
				.. "</div>"
		end

		rows[#rows + 1] = '<a href="/view/manga/'
			.. h(pluginID)
			.. "/"
			.. h(mangaID)
			.. '" class="flex flex-col bg-neutral-900 rounded-lg overflow-hidden hover:-translate-y-0.5 hover:shadow-lg hover:shadow-black/40 hover:ring-1 hover:ring-indigo-500 transition">'
			.. '<div class="relative overflow-hidden">'
			.. coverHTML
			.. "</div>"
			.. '<div class="p-3 flex-1 flex flex-col justify-between">'
			.. '<div class="font-medium text-sm mb-1 truncate" title="'
			.. h(title)
			.. '">'
			.. h(title)
			.. "</div>"
			.. '<div class="flex items-center gap-1">'
			.. (pluginBadge ~= "" and '<span class="text-xs text-neutral-500">' .. pluginBadge .. "</span>" or "")
			.. "</div>"
			.. "</div></a>"
	end

	local noResults = ""
	if #results == 0 and (q ~= "" or genre ~= "") then
		noResults = '<div class="py-16 text-center text-neutral-500">No results'
		if q ~= "" then
			noResults = noResults .. ' for "' .. h(q) .. '"'
		end
		if pluginName ~= "" then
			noResults = noResults .. " on " .. h(pluginName)
		end
		noResults = noResults .. "</div>"
	end

	-- Challenge warning
	local challengeHTML = ""
	if challenge then
		challengeHTML = '<div class="bg-amber-500/15 border border-amber-500/40 text-amber-300 rounded-lg p-4 mb-6">'
			.. 'This site needs human verification — go to <a href="/view/plugins" class="underline">Plugins → Human Verification</a>'
			.. " to paste your session cookies.</div>"
	end

	-- Human verification wizard modal (shared partial; renders "" when hidden).
	-- MangaID "" = search context: save keeps the query string.
	local verifyHTML = verifyWizard({
		csrf_token = data.csrf_token or "",
		PluginID = pluginID,
		MangaID = "",
		NeedsHumanVerify = data.NeedsHumanVerify or false,
		VerifyURL = data.VerifyURL or "",
		VerifyCookies = data.VerifyCookies or "",
		VerifyUserAgent = data.VerifyUserAgent or "",
		Challenge = challenge,
	})

	-- Results grid
	local resultsHTML = ""
	if #results > 0 then
		resultsHTML = '<div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-6 gap-4">'
			.. table.concat(rows, "\n")
			.. "</div>"
	end

	-- Top pagination
	local topPagination = ""
	if #results > 0 then
		topPagination = pagination({
			Pagination = {
				Base = "/view/search",
				Param = "page",
				Current = page,
				Total = totalPages,
				Extra = { "q", q, "pluginID", pluginID, "genre", genresSelected },
				Compact = true,
			},
		})
	end

	-- Bottom pagination
	local bottomPagination = ""
	if #results > 0 then
		bottomPagination = pagination({
			Pagination = {
				Base = "/view/search",
				Param = "page",
				Current = page,
				Total = totalPages,
				Extra = { "q", q, "pluginID", pluginID, "genre", genresSelected },
				Compact = false,
			},
		})
	end

	-- Plugin picker (button + hidden native select keeps form semantics)
	local selIcon, selName = "", pluginID
	for _, p in ipairs(plugins) do
		if p.ID == pluginID then
			selIcon = p.IconURL or ""
			selName = p.Name
			break
		end
	end
	local pickerBtn = '<button type="button" id="pluginPickerBtn" class="flex items-center gap-2 bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 text-sm min-w-[180px] text-left">'
		.. (selIcon ~= "" and ('<img src="' .. h(selIcon) .. '" alt="" class="w-5 h-5 rounded object-cover flex-shrink-0">') or "")
		.. '<span id="pluginPickerLabel" class="truncate">'
		.. h(selName)
		.. "</span></button>"
	local pickerDrop =
		'<div id="pluginPickerDrop" class="hidden absolute z-20 mt-1 w-56 bg-neutral-900 border border-neutral-700 rounded-md shadow-lg py-1 max-h-64 overflow-auto">'
	for _, p in ipairs(plugins) do
		if p.IsActive then
			local ic = p.IconURL or ""
			pickerDrop = pickerDrop
				.. '<button type="button" data-pick="'
				.. h(p.ID)
				.. '" data-pname="'
				.. h(p.Name)
				.. '" class="w-full flex items-center gap-2 px-3 py-2 text-sm text-left hover:bg-neutral-800">'
				.. (ic ~= "" and ('<img src="' .. h(ic) .. '" alt="" class="w-5 h-5 rounded object-cover flex-shrink-0">') or '<span class="w-5 h-5 rounded bg-neutral-700 flex-shrink-0"></span>')
				.. h(p.Name)
				.. "</button>"
		end
	end
	pickerDrop = pickerDrop .. "</div>"
	return [[<h1 class="text-xl font-semibold mb-6">Search]]
		.. pluginBadge
		.. [[</h1>]]
		.. '<form method="get" action="/view/search" id="searchForm" class="mb-6">'
		.. '<div class="flex gap-2 items-end">'
		.. '<div class="relative"><label class="block text-xs text-neutral-400 mb-1">Plugin</label>'
		.. pickerBtn
		.. pickerDrop
		.. '<input type="hidden" id="pluginID" name="pluginID" value="'
		.. h(pluginID)
		.. '" data-pname="'
		.. h(selName)
		.. '"><select id="pluginIDNative" class="hidden">'
		.. pluginOpts
		.. "</select></div>"
		.. '<div class="flex-1"><label for="q" class="block text-xs text-neutral-400 mb-1">&nbsp;</label><input id="q" name="q" type="text" value="'
		.. h(q)
		.. '" placeholder="Manga title..." class="w-full bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 text-sm"></div>'
		.. '<button type="submit" class="bg-indigo-600 hover:bg-indigo-500 text-white rounded-md px-4 py-2 text-sm font-medium">Search</button>'
		.. "</div>"
		-- Genre/tag selector area: only when the selected plugin advertises genres.
		-- Full-width multi-select below the search bar so category browsing has its
		-- own region, and is hidden entirely for plugins without genre support.
		.. (genreBoxes ~= "" and ('<div class="mt-4"><label class="block text-xs text-neutral-400 mb-1">Categories</label><div class="max-h-40 overflow-y-auto grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-x-4 gap-y-2 bg-neutral-900 border border-neutral-700 rounded-md px-3 py-3">' .. genreBoxes .. "</div></div>") or "")
		.. (topPagination ~= "" and ('<div class="mt-3">' .. topPagination .. "</div>") or "")
		.. "</form>"
		.. '<div id="search-history" class="mb-8"></div>'
		.. [[<script>(function(){
  var btn = document.getElementById('pluginPickerBtn'), drop = document.getElementById('pluginPickerDrop'), hid = document.getElementById('pluginID');
  if (btn && drop && hid) {
    btn.addEventListener('click', function () { drop.classList.toggle('hidden'); });
    drop.querySelectorAll('[data-pick]').forEach(function (el) {
      el.addEventListener('click', function () {
        hid.value = el.getAttribute('data-pick');
        hid.setAttribute('data-pname', el.getAttribute('data-pname') || el.getAttribute('data-pick'));
        document.getElementById('searchForm').submit();
      });
    });
    document.addEventListener('click', function (e) {
      if (!btn.contains(e.target) && !drop.contains(e.target)) drop.classList.add('hidden');
    });
  }
})();</script>]]
		.. resultsHTML
		.. "\n"
		.. noResults
		.. "\n"
		.. challengeHTML
		.. "\n"
		.. verifyHTML
		.. "\n"
		.. bottomPagination
		.. [[<script>
(function(){
  var KEY = "gi_search_history", MAX = 20, MAX_SHOW = 8;
  function load() {
    try { var v = JSON.parse(localStorage.getItem(KEY) || "[]"); return Array.isArray(v) ? v : []; }
    catch (e) { return []; }
  }
  function save(h) { try { localStorage.setItem(KEY, JSON.stringify(h.slice(0, MAX))); } catch (e) {} }
  var qEl = document.getElementById("q"), pEl = document.getElementById("pluginID"), gEl = document.getElementById("genre");
  // Genres = checkbox selections, collected so old single-value history entries
  // keep working. Submitted as separate ?genre=<slug> params by the browser.
  var cbxs = gEl ? gEl.querySelectorAll('input[name="genre"]') : null;
  var genres = [];
  if (cbxs) {
    for (var i = 0; i < cbxs.length; i++) {
      if (cbxs[i].checked && cbxs[i].value) genres.push(cbxs[i].value);
    }
  }
  // Joined so old single-value history entries keep working unchanged.
  var q = qEl ? qEl.value : "", pid = pEl ? pEl.value : "", genre = genres.join(",");
  var pname = (pEl && pEl.dataset && pEl.dataset.pname) ? pEl.dataset.pname : pid;
  var h = load();
  if (q || genre) {
    h = h.filter(function(e) { return !(e.q === q && e.pid === pid && e.g === genre); });
    h.unshift({ q: q, pid: pid, pname: pname || "", g: genre || "" });
    save(h);
  }
  if (q || genre || !h.length) return;
  var box = document.getElementById("search-history");
  if (!box) return;
  var label = document.createElement("div");
  label.className = "text-xs text-neutral-400 mb-2";
  label.textContent = "Recent searches";
  box.appendChild(label);
  var grid = document.createElement("div");
  grid.className = "grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-2";
  box.appendChild(grid);
  h.slice(0, MAX_SHOW).forEach(function (e) {
    var a = document.createElement("a");
    var url = "/view/search?pluginID=" + encodeURIComponent(e.pid || "") + "&q=" + encodeURIComponent(e.q || "");
    // e.g holds the genres joined by "," (one entry per selected genre).
    if (e.g) url += e.g.split(",").filter(Boolean).map(function (g) { return "&genre=" + encodeURIComponent(g); }).join("");
    a.href = url;
    a.className = "block truncate text-xs px-3 py-1.5 rounded-md bg-neutral-800 border border-neutral-700 text-neutral-300 hover:bg-neutral-700";
    a.title = (e.q || e.g) + (e.pname || e.pid ? " via " + (e.pname || e.pid) : "");
    var src = e.pname || e.pid || "";
    a.textContent = (e.q || e.g) + (src ? " via " + src : "");
    grid.appendChild(a);
  });
})();
</script>]]
end
