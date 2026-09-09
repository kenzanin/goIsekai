-- views/logs.lua
-- Live log viewer with WebSocket streaming and polling fallback.
-- Replaces views/logs.jet.
-- Called as: logs(data) -> string (body HTML only, layout wraps it)
-- data.Logs: []string, data.Limit: int, data.Limits: []int, data.Filter: string

return function(data)
	local logs = data.Logs or {}
	local limit = data.Limit or 500
	local limits = data.Limits or { 100, 250, 500, 1000, 2000 }
	local filter = data.Filter or "all"
	local parts = {}
	local function emit(s)
		parts[#parts + 1] = s
	end

	emit('<div class="flex items-center justify-between mb-4 gap-2 flex-wrap">')
	emit(
		'    <h1 class="text-xl font-semibold">Logs <span class="text-xs font-normal text-neutral-500">last '
			.. h(tostring(limit))
			.. " lines · ring buffer 2000</span></h1>"
	)
	emit('    <div class="flex items-center gap-2">')
	emit(
		'        <span id="log-paused" class="hidden text-xs px-2 py-1 rounded-md bg-amber-500/15 text-amber-400">Paused — selecting text</span>'
	)
	emit(
		'        <select id="log-filter" class="bg-neutral-900 border border-neutral-700 rounded-md px-2 py-1.5 text-xs">'
	)
	local function opt(val, label)
		local sel = filter == val and " selected" or ""
		emit('            <option value="' .. h(val) .. '"' .. sel .. ">" .. h(label) .. "</option>")
	end
	opt("all", "All sources")
	opt("app", "App only")
	opt("plugins", "Plugins only")
	emit("        </select>")
	emit(
		'        <select id="log-limit" class="bg-neutral-900 border border-neutral-700 rounded-md px-2 py-1.5 text-xs">'
	)
	for _, n in ipairs(limits) do
		local sel = n == limit and " selected" or ""
		emit('            <option value="' .. h(tostring(n)) .. '"' .. sel .. ">" .. tostring(n) .. " lines</option>")
	end
	emit("        </select>")
	emit(
		'        <button id="log-copy" type="button" class="border border-neutral-700 hover:bg-neutral-800 rounded-md px-3 py-1.5 text-xs font-medium">Copy</button>'
	)
	emit("    </div>")
	emit("</div>")

	emit(
		'<div id="logview" class="bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 mb-6 overflow-y-auto max-h-[70vh] font-mono text-xs">'
	)
	for i, line in ipairs(logs) do
		emit(
			'    <pre class="text-xs font-mono whitespace-pre-wrap break-all border-b border-neutral-900 py-0.5 select-text">'
				.. h(line)
				.. "</pre>"
		)
	end
	emit("</div>")

	emit("<script>")
	emit("(function() {")
	emit('  var view = document.getElementById("logview");')
	emit("  if (!view) return;")
	emit("  var selecting = false;")
	emit("  function hasSelection() { var s = window.getSelection(); return s && s.toString().length > 0; }")
	emit('  view.addEventListener("mousedown", function() { selecting = true; });')
	emit('  document.addEventListener("mouseup", function() { selecting = false; });')
	emit("  function colorLogLines() {")
	emit('    var pres = view.querySelectorAll("pre");')
	emit("    for (var i = 0; i < pres.length; i++) {")
	emit("      var el = pres[i];")
	emit("      var t = el.textContent;")
	emit('      el.classList.remove("text-amber-400", "text-red-400", "text-neutral-500", "text-neutral-300");')
	emit('      if (t.indexOf("WARN") !== -1) el.classList.add("text-amber-400");')
	emit('      else if (t.indexOf("ERROR") !== -1) el.classList.add("text-red-400");')
	emit('      else if (t.indexOf("DEBUG") !== -1) el.classList.add("text-neutral-500");')
	emit('      else el.classList.add("text-neutral-300");')
	emit("    }")
	emit("  }")
	emit("  colorLogLines();")
	emit("")
	emit("  function refresh() {")
	emit("    if (selecting || hasSelection()) return;")
	emit('    var limit = document.getElementById("log-limit").value;')
	emit('    var filter = document.getElementById("log-filter").value;')
	emit('    fetch("/view/logs?limit=" + limit + "&filter=" + filter + "&partial=1")')
	emit("      .then(function(r) { return r.text(); })")
	emit("      .then(function(html) {")
	emit('        var doc = new DOMParser().parseFromString(html, "text/html");')
	emit('        var fresh = doc.getElementById("logview");')
	emit("        if (fresh) { view.innerHTML = fresh.innerHTML; colorLogLines(); }")
	emit("      })")
	emit("      .catch(function() {});")
	emit("  }")
	emit("  var intervalId = setInterval(refresh, 2000);")
	emit("")
	emit("  var ws;")
	emit("  function startWebSocket() {")
	emit('    var protocol = (location.protocol === "https:" ? "wss://" : "ws://");')
	emit('    var wsUrl = protocol + location.host + "/api/logs/ws";')
	emit("    ws = new WebSocket(wsUrl);")
	emit('    ws.addEventListener("open", function() {')
	emit("      clearInterval(intervalId); intervalId = null;")
	emit("    });")
	emit('    ws.addEventListener("message", function(ev) {')
	emit("      var line = ev.data;")
	emit('      var filter = document.getElementById("log-filter").value;')
	emit('      if (filter === "app" && line.indexOf(" plugin=") !== -1) return;')
	emit('      if (filter === "plugins" && line.indexOf(" plugin=") === -1) return;')
	emit('      var pre = document.createElement("pre");')
	emit(
		'      pre.className = "text-xs font-mono whitespace-pre-wrap break-all border-b border-neutral-900 py-0.5 select-text";'
	)
	emit("      pre.textContent = line;")
	emit("      view.appendChild(pre);")
	emit("      colorLogLines();")
	emit('      var limit = parseInt(document.getElementById("log-limit").value, 10);')
	emit("      while (view.children.length > limit) { view.removeChild(view.firstChild); }")
	emit("    });")
	emit('    ws.addEventListener("error", function() { fallbackToPolling(); });')
	emit('    ws.addEventListener("close", function() { fallbackToPolling(); });')
	emit("  }")
	emit("  function fallbackToPolling() {")
	emit("    if (ws) { ws = null; }")
	emit("    if (!intervalId) { intervalId = setInterval(refresh, 2000); }")
	emit("  }")
	emit("  startWebSocket();")
	emit("")
	emit('  document.getElementById("log-copy").addEventListener("click", function() {')
	emit("    var text = view.innerText;")
	emit("    if (navigator.clipboard && navigator.clipboard.writeText) {")
	emit("      navigator.clipboard.writeText(text).then(function() {")
	emit('        var b = document.getElementById("log-copy");')
	emit('        b.textContent = "Copied ✓";')
	emit('        setTimeout(function() { b.textContent = "Copy"; }, 1200);')
	emit("      });")
	emit("    } else {")
	emit('      var ta = document.createElement("textarea");')
	emit("      ta.value = text;")
	emit("      document.body.appendChild(ta);")
	emit("      ta.select();")
	emit('      document.execCommand("copy");')
	emit("      document.body.removeChild(ta);")
	emit("    }")
	emit("  });")
	emit("")
	emit('  document.getElementById("log-limit").addEventListener("change", function() { refresh(); });')
	emit('  document.getElementById("log-filter").addEventListener("change", function() { refresh(); });')
	emit("})();")
	emit("</script>")

	return table.concat(parts, "\n")
end
