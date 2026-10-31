--  partials/search_scripts.lua
--  The two inline scripts on the search page: the plugin-picker dropdown
--  behaviour, and the recent-search history stored in localStorage. Split out
--  of views/search.lua so the view stays readable; both are returned as raw
--  HTML string literals, so the emitted bytes are unchanged.
return function(data)
	local pickerScript = [==[<script>(function(){
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
})();</script>]==]
	local historyScript = [==[<script>
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
</script>]==]
	return pickerScript, historyScript
end
