-- views/logs.lua
-- Live log viewer with WebSocket streaming + polling fallback.
-- Called as: logs(data) -> string (body HTML only, layout wraps it)
-- data.Logs: []string (newest-first, index 0 = newest line)
-- data.Limit: int, data.Limits: []int, data.Filter: string

return function(data)
  local logs = data.Logs or {}
  local limit = data.Limit or 500
  local limits = data.Limits or { 100, 250, 500, 1000, 2000 }
  local filter = data.Filter or "all"
  local lanes = data.WorkerLanes or {}

  -- Build limit options
  local limitOpts = ""
  for _, n in ipairs(limits) do
    local sel = n == limit and " selected" or ""
    limitOpts = limitOpts .. "<option value=" .. h(tostring(n)) .. sel .. ">" .. tostring(n) .. " lines</option>"
  end

  -- Worker-lane cards, rendered server-side so the page shows real numbers
  -- immediately instead of flashing "loading lanes…" while the JS fetches
  -- /api/workers. The live refresh still overwrites these in place (same DOM
  -- shape renderLanes() builds), so this only removes the first-load flash.
  local function fmtMs(ms)
    if ms >= 1000 then
      return string.format("%.2fs", ms / 1000)
    end
    return tostring(ms) .. "ms"
  end
  local laneCards = ""
  for _, l in ipairs(lanes) do
    local dur = "no completed jobs yet"
    if l.HasDurations then
      dur = "p50 " .. fmtMs(l.MedianMS) .. " · p99 " .. fmtMs(l.P99MS) .. " (n=" .. tostring(l.Samples) .. ")"
    end
    local stats = "waiting " .. tostring(l.Waiting) .. " · running " .. tostring(l.Running) .. "/" .. tostring(l.Capacity)
      .. " · ok " .. tostring(l.Completed) .. " · fail " .. tostring(l.Failed) .. " · abandoned " .. tostring(l.Abandoned)
    laneCards = laneCards
      .. '<div data-lane="' .. h(l.Lane) .. '" class="bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 cursor-pointer hover:bg-neutral-800">'
      .. '<div class="font-semibold text-neutral-200 mb-1">' .. h(l.Lane) .. "</div>"
      .. '<div class="text-neutral-400">' .. h(stats) .. "</div>"
      .. '<div class="text-neutral-500">' .. h(dur) .. "</div>"
      .. "</div>"
  end
  if laneCards == "" then
    laneCards = '<div class="bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2">loading lanes…</div>'
  end

  -- Build log lines
  local logLines = ""
  for i, line in ipairs(logs) do
    local lane = extractLane(line)
    logLines = logLines .. '<pre class="text-sm font-mono whitespace-pre-wrap break-all border-b border-neutral-900 py-0.5 select-text" data-lane="' .. lane .. '">' .. h(line) .. "</pre>"
  end

  return [=[<div class="flex items-center justify-between mb-4 gap-2 flex-wrap">
    <h1 class="text-xl font-semibold">Logs <span class="text-xs font-normal text-neutral-500">last ]=] .. h(tostring(limit)) .. [=[ lines · ring buffer 2000</span></h1>
    <div class="flex items-center gap-2">
      <span id="log-paused" class="hidden text-xs px-2 py-1 rounded-md bg-amber-500/15 text-amber-400">Paused — auto-stream stopped</span>
      <button id="log-pause" class="bg-neutral-800 border border-neutral-600 rounded-md px-3 py-1.5 text-xs font-medium text-neutral-200 hover:bg-neutral-700">Pause</button>
      <select id="log-limit" class="bg-neutral-900 border border-neutral-700 rounded-md px-2 py-1.5 text-xs">]=] .. limitOpts .. [=[</select>
      <select id="log-filter" class="bg-neutral-900 border border-neutral-700 rounded-md px-2 py-1.5 text-xs">
        <option value="all"]=] .. (filter == "all" and " selected" or "") .. [=[>All sources</option>
        <option value="app"]=] .. (filter == "app" and " selected" or "") .. [=[>App only</option>
        <option value="plugins"]=] .. (filter == "plugins" and " selected" or "") .. [=[>Plugins</option>
        <option value="warn"=] .. (filter == "warn" and " selected" or "") .. [=[>Warnings</option>
        <option value="error"]=] .. (filter == "error" and " selected" or "") .. [=[>Errors</option>
        <option value="lane-fetch"]=] .. (filter == "lane-fetch" and " selected" or "") .. [=[>Fetch lane</option>
        <option value="lane-image"]=] .. (filter == "lane-image" and " selected" or "") .. [=[>Image lane</option>
        <option value="lane-interactive"=] .. (filter == "lane-interactive" and " selected" or "") .. [=[>Interactive lane</option>
        <option value="lane-maintenance"=] .. (filter == "lane-maintenance" and " selected" or "") .. [=[>Maintenance lane</option>
        <option value="lane-cdp"]=] .. (filter == "lane-cdp" and " selected" or "") .. [=[>CDP lane</option>
      </select>
    </div>
  </div>

  <div class="grid grid-cols-2 md:grid-cols-4 gap-2 mb-4 text-xs">
    <div id="worker-lanes" class="contents">]=] .. laneCards .. [=[</div>
  </div>

  <div id="logview" class="bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 mb-6 overflow-y-auto max-h-[70vh] font-mono text-sm">
]=] .. logLines .. [=[</div>

<script>
(function() {
  var view = document.getElementById("logview");
  if (!view) return;

  var selecting = false;
  var paused = false;

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

  // Lane values are lowercase, matching workers.Lane.String() and the lane=
  // attr on log lines. These values drive the ?lane= query param, the
  // lineMatchesLane test, and the worker-card highlight, so they must agree
  // with the server's logger.LaneOf exactly.
  var laneMap = {
    "lane-fetch": "fetch",
    "lane-image": "image",
    "lane-interactive": "interactive",
    "lane-maintenance": "maintenance",
    "lane-cdp": "cdp"
  };

  function getSelectedLaneName() {
    var el = document.getElementById("log-filter");
    var val = el ? el.value : "all";
    return laneMap[val] || null;
  }

  function lineMatchesLane(lineText, laneName) {
    if (!laneName) return true;
    return lineText.indexOf("lane=" + laneName) !== -1;
  }

  function buildFetchURL() {
    var limit = document.getElementById("log-limit").value;
    var filter = document.getElementById("log-filter").value;
    var url = "/view/logs?limit=" + limit + "&filter=" + filter;
    var laneName = getSelectedLaneName();
    if (laneName) {
      url += "&lane=" + laneName;
    }
    return url;
  }

  function refresh() {
    if (paused) return;
    if (selecting || hasSelection()) return;
    fetch(buildFetchURL(), { headers: { "X-Partial": "true" } })
      .then(function(r) { return r.text(); })
      .then(function(html) {
        var doc = new DOMParser().parseFromString(html, "text/html");
        var fresh = doc.getElementById("logview");
        if (fresh) {
          view.innerHTML = fresh.innerHTML;
          colorLogLines();
        }
      })
      .catch(function() {});
  }

  var intervalId = setInterval(refresh, 2000);
  var ws;

  function updatePauseButton() {
    var btn = document.getElementById("log-pause");
    if (!btn) return;
    var pausedSpan = document.getElementById("log-paused");
    if (paused) {
      btn.textContent = "Resume";
      if (pausedSpan) pausedSpan.classList.remove("hidden");
    } else {
      btn.textContent = "Pause";
      if (pausedSpan) pausedSpan.classList.add("hidden");
    }
  }

  document.getElementById("log-pause").addEventListener("click", function() {
    paused = !paused;
    updatePauseButton();
    if (paused) {
      if (intervalId) { clearInterval(intervalId); intervalId = null; }
      if (ws) { ws.close(); ws = null; }
    } else {
      if (!intervalId) { intervalId = setInterval(refresh, 2000); }
      startWebSocket();
      refresh();
    }
  });

  function startWebSocket() {
    if (paused) return;
    var protocol = (location.protocol === "https:" ? "wss://" : "ws://");
    var wsUrl = protocol + location.host + "/api/logs/ws";
    ws = new WebSocket(wsUrl);
    ws.addEventListener("open", function() {
      if (intervalId) { clearInterval(intervalId); intervalId = null; }
    });
    ws.addEventListener("message", function(ev) {
      if (paused) return;
      var line = ev.data;
      var filterVal = document.getElementById("log-filter").value;
      if (filterVal === "app" && line.indexOf(" plugin=") !== -1) return;
      if (filterVal === "plugins" && line.indexOf(" plugin=") === -1) return;
      if (filterVal === "warn" && line.indexOf(" WARN ") === -1) return;
      if (filterVal === "error" && line.indexOf(" ERROR ") === -1) return;
      var laneName = getSelectedLaneName();
      if (laneName && !lineMatchesLane(line, laneName)) return;
      var pre = document.createElement("pre");
      pre.className = "text-sm font-mono whitespace-pre-wrap break-all border-b border-neutral-900 py-0.5 select-text";
      pre.textContent = line;
      view.insertBefore(pre, view.firstChild);
      colorLogLines();
      var limit = parseInt(document.getElementById("log-limit").value, 10);
      while (view.children.length > limit) { view.removeChild(view.firstChild); }
    });
    ws.addEventListener("error", function() { fallbackToPolling(); });
    ws.addEventListener("close", function() { fallbackToPolling(); });
  }

  function fallbackToPolling() {
    if (ws) { ws = null; }
    if (!paused && !intervalId) { intervalId = setInterval(refresh, 2000); }
  }

  startWebSocket();

  var lanesEl = document.getElementById("worker-lanes");
  var currentLanesData = null;

  function fmtMs(ms) { return ms >= 1000 ? (ms / 1000).toFixed(2) + "s" : ms + "ms"; }

  function updateLaneCardHighlight() {
    if (!lanesEl) return;
    var cards = lanesEl.querySelectorAll("div[ data-lane] , div[data-lane]");
    for (var i = 0; i < cards.length; i++) {
      var card = cards[i];
      var laneName = card.dataset.lane;
      if (!laneName) continue;
      var matchKey = null;
      for (var key in laneMap) {
        if (laneMap[key] === laneName) { matchKey = key; break; }
      }
      var currentFilter = document.getElementById("log-filter").value;
      if (matchKey && currentFilter === matchKey) {
        card.classList.add("ring-2", "ring-amber-400");
      } else {
        card.classList.remove("ring-2", "ring-amber-400");
      }
    }
  }

  function renderLanes(data) {
    if (!lanesEl || !data || !data.lanes) return;
    lanesEl.textContent = "";
    currentLanesData = data;
    data.lanes.forEach(function (l) {
      var card = document.createElement("div");
      card.className = "bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 cursor-pointer hover:bg-neutral-800";
      card.dataset.lane = l.lane;

      var head = document.createElement("div");
      head.className = "font-semibold text-neutral-200 mb-1";
      head.textContent = l.lane;

      var stats = document.createElement("div");
      stats.className = "text-neutral-400";
      stats.textContent = "waiting " + l.waiting + " · running " + l.running + "/" + l.capacity + " · ok " + l.completed + " · fail " + l.failed + " · abandoned " + l.abandoned;

      var dur = document.createElement("div");
      dur.className = "text-neutral-500";
      dur.textContent = l.has_durations ? "p50 " + fmtMs(l.median_ms) + " · p99 " + fmtMs(l.p99_ms) + " (n=" + l.duration_samples + ")" : "no completed jobs yet";

      card.appendChild(head);
      card.appendChild(stats);
      card.appendChild(dur);

      card.addEventListener("click", function() {
        var filterEl = document.getElementById("log-filter");
        var matchKey = null;
        for (var key in laneMap) {
          if (laneMap[key] === l.lane) { matchKey = key; break; }
        }
        if (!matchKey) return;
        if (filterEl.value === matchKey) {
          filterEl.value = "all";
        } else {
          filterEl.value = matchKey;
        }
        refresh();
        updateLaneCardHighlight();
      });

      lanesEl.appendChild(card);
    });
    updateLaneCardHighlight();
  }

  function refreshLanes() {
    fetch("/api/workers")
      .then(function (r) { return r.json(); })
      .then(function(data) {
        currentLanesData = data;
        renderLanes(data);
      })
      .catch(function () {});
  }
  refreshLanes();
  setInterval(refreshLanes, 2000);

  document.getElementById("log-limit").addEventListener("change", function() { refresh(); });
  document.getElementById("log-filter").addEventListener("change", function() { refresh(); });
})();
</script>
]=]
end
