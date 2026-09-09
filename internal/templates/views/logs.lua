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

	-- Build limit options
	local limitOpts = ""
	for _, n in ipairs(limits) do
		local sel = n == limit and " selected" or ""
		limitOpts = limitOpts
			.. "            <option value="
			.. h(tostring(n))
			.. sel
			.. ">"
			.. tostring(n)
			.. " lines</option>\n"
	end

	-- Build log lines
	local logLines = ""
	for i, line in ipairs(logs) do
		logLines = logLines
			.. '    <pre class="text-sm font-mono whitespace-pre-wrap break-all border-b border-neutral-900 py-0.5 select-text">'
			.. h(line)
			.. "</pre>\n"
	end

	return [[<div class="flex items-center justify-between mb-4 gap-2 flex-wrap">
    <h1 class="text-xl font-semibold">Logs <span class="text-xs font-normal text-neutral-500">last ]] .. h(
		tostring(limit)
	) .. [[ lines · ring buffer 2000</span></h1>
    <div class="flex items-center gap-2">
        <span id="log-paused" class="hidden text-xs px-2 py-1 rounded-md bg-amber-500/15 text-amber-400">Paused — selecting text</span>
        <select id="log-filter" class="bg-neutral-900 border border-neutral-700 rounded-md px-2 py-1.5 text-xs">
            <option value="all"]] .. (filter == "all" and " selected" or "") .. [[>All sources</option>
            <option value="app"]] .. (filter == "app" and " selected" or "") .. [[>App only</option>
            <option value="plugins"]] .. (filter == "plugins" and " selected" or "") .. [[>Plugins only</option>
        </select>
        <select id="log-limit" class="bg-neutral-900 border border-neutral-700 rounded-md px-2 py-1.5 text-xs">
]] .. limitOpts .. [[        </select>
        <button id="log-copy" type="button" class="border border-neutral-700 hover:bg-neutral-800 rounded-md px-3 py-1.5 text-xs font-medium">Copy</button>
    </div>
</div>

<div id="logview" class="bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 mb-6 overflow-y-auto max-h-[70vh] font-mono text-sm">
]] .. logLines .. [[</div>

<script>
(function() {
  var view = document.getElementById("logview");
  if (!view) return;
  var selecting = false;
  function hasSelection() { var s = window.getSelection(); return s && s.toString().length > 0; }
  view.addEventListener("mousedown", function() { selecting = true; });
  document.addEventListener("mouseup", function() { selecting = false; });
  function colorLogLines() {
    var pres = view.querySelectorAll("pre");
    for (var i = 0; i < pres.length; i++) {
      var el = pres[i];
      var t = el.textContent;
      el.classList.remove("text-amber-400", "text-red-400", "text-neutral-500", "text-neutral-300");
      if (t.indexOf("WARN") !== -1) el.classList.add("text-amber-400");
      else if (t.indexOf("ERROR") !== -1) el.classList.add("text-red-400");
      else if (t.indexOf("DEBUG") !== -1) el.classList.add("text-neutral-500");
      else el.classList.add("text-neutral-300");
    }
  }
  colorLogLines();

  function refresh() {
    if (selecting || hasSelection()) return;
    var limit = document.getElementById("log-limit").value;
    var filter = document.getElementById("log-filter").value;
    fetch("/view/logs?limit=" + limit + "&filter=" + filter + "&partial=1")
      .then(function(r) { return r.text(); })
      .then(function(html) {
        var doc = new DOMParser().parseFromString(html, "text/html");
        var fresh = doc.getElementById("logview");
        if (fresh) { view.innerHTML = fresh.innerHTML; colorLogLines(); }
      })
      .catch(function() {});
  }
  var intervalId = setInterval(refresh, 2000);

  var ws;
  function startWebSocket() {
    var protocol = (location.protocol === "https:" ? "wss://" : "ws://");
    var wsUrl = protocol + location.host + "/api/logs/ws";
    ws = new WebSocket(wsUrl);
    ws.addEventListener("open", function() {
      clearInterval(intervalId); intervalId = null;
    });
    ws.addEventListener("message", function(ev) {
      var line = ev.data;
      var filter = document.getElementById("log-filter").value;
      if (filter === "app" && line.indexOf(" plugin=") !== -1) return;
      if (filter === "plugins" && line.indexOf(" plugin=") === -1) return;
      var pre = document.createElement("pre");
      pre.className = "text-xs font-mono whitespace-pre-wrap break-all border-b border-neutral-900 py-0.5 select-text";
      pre.textContent = line;
      view.appendChild(pre);
      colorLogLines();
      var limit = parseInt(document.getElementById("log-limit").value, 10);
      while (view.children.length > limit) { view.removeChild(view.firstChild); }
    });
    ws.addEventListener("error", function() { fallbackToPolling(); });
    ws.addEventListener("close", function() { fallbackToPolling(); });
  }
  function fallbackToPolling() {
    if (ws) { ws = null; }
    if (!intervalId) { intervalId = setInterval(refresh, 2000); }
  }
  startWebSocket();

  document.getElementById("log-copy").addEventListener("click", function() {
    var text = view.innerText;
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(function() {
        var b = document.getElementById("log-copy");
        b.textContent = "Copied ✓";
        setTimeout(function() { b.textContent = "Copy"; }, 1200);
      });
    } else {
      var ta = document.createElement("textarea");
      ta.value = text;
      document.body.appendChild(ta);
      ta.select();
      document.execCommand("copy");
      document.body.removeChild(ta);
    }
  });

  document.getElementById("log-limit").addEventListener("change", function() { refresh(); });
  document.getElementById("log-filter").addEventListener("change", function() { refresh(); });
})();
</script>
]]
end
