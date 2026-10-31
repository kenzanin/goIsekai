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

  local logsScript = require("partials.logs_script")({})
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

]=] .. logsScript .. [=[
]=]
end
