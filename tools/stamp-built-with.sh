#!/usr/bin/env bash
# Record which toolchain (and version) compiled a plugin's artifact, in the
# plugin.json beside it, as "built_with". The plugins page reads that field, so
# a user can tell a TinyGo build from a standard-Go one without stat-ing bytes.
#
# Usage: stamp-built-with.sh <plugin-dir> <tool> [version]
#   e.g. stamp-built-with.sh examples/plugins/wasm/manga_uno tinygo 0.43.0
# Other plugin.json fields are preserved; a missing file is created.
set -euo pipefail

dir="${1:?usage: stamp-built-with.sh <plugin-dir> <tool> [version]}"
tool="${2:?usage: stamp-built-with.sh <plugin-dir> <tool> [version]}"
ver="${3:-}"

pj="$dir/plugin.json"
[ -f "$pj" ] || echo '{}' >"$pj"

tmp="$(mktemp)"
python3 - "$pj" "$tool" "$ver" >"$tmp" <<'PY'
import json, sys

path, tool, ver = sys.argv[1], sys.argv[2], sys.argv[3]
try:
    with open(path, encoding="utf-8") as fh:
        meta = json.load(fh)
    if not isinstance(meta, dict):
        meta = {}
except (OSError, ValueError):
    meta = {}

meta["built_with"] = f"{tool} {ver}".strip()
json.dump(meta, sys.stdout, indent=2, ensure_ascii=False)
sys.stdout.write("\n")
PY

mv "$tmp" "$pj"
echo "stamped $pj: built_with=$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["built_with"])' "$pj")"
