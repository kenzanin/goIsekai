--  partials/logs_script.lua
--  The inline <script> block driving the live log viewer: WebSocket stream
--  with polling fallback, pause/limit/filter controls, lane cards. Moved out
--  of views/logs.lua verbatim; the emitted bytes are identical.
return function(data)
	return [==[<script>
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
]==]
end
