#!/usr/bin/env bash
# Audit every active plugin through the live host API (read-only).
#
# Usage:
#   ./tools/audit-live-plugins.sh [BASE_URL]
#
# Default BASE_URL is the sandbox instance (127.0.0.1:8081, started with
# -dataDir app_data_playground -port 8081). Pass http://127.0.0.1:8080 to
# audit production — read-only either way.
#
# Outputs (repo root):
#   live-plugin-audit.json  — raw per-plugin probe records
#   live-plugin-audit.md    — human table: per-feature WORKING/MISSING/BROKEN
#
# Feature classes:
#   WORKING  — expected data present
#   MISSING  — HTTP 200 but expected data absent (no genres, no released_at, no pages)
#   BROKEN   — HTTP error or host-level failure (403 challenge labeled separately)
set -u
# The audit writes its raw probe bodies to the session's scratch dir.
# TMPDIR may be unset when run from cron/wrappers — fall back to /tmp so probes
# never expand to an empty path (an empty out-file silently breaks all probes).
export TMPDIR="${TMPDIR:-/tmp}"
BASE="${1:-http://127.0.0.1:8081}"
OUT_JSON="$(pwd)/live-plugin-audit.json"
OUT_MD="$(pwd)/live-plugin-audit.md"
TIMEOUT=30
Q=hero

# q=hero misses some catalogs; fall back to q=manga for an empty search.
probe_search() { # $1=plugin -> writes $TMPDIR/audit_$1.search, sets code/search_ok
    local pid="$1"
    local code
    code=$(curl -s --max-time "$TIMEOUT" -o "$TMPDIR/audit_${pid}.search" -w '%{http_code}' "$BASE/api/sandbox/plugins/${pid}/search?q=${Q}")
    local ok
    ok=$(node -e 'try{const d=JSON.parse(require("fs").readFileSync(process.argv[1],"utf8"));console.log(Array.isArray(d)&&d.length>0?"1":"0")}catch(e){console.log("0")}' "$TMPDIR/audit_${pid}.search" 2>/dev/null)
    if [ "$ok" = "0" ] && [ "$Q" = "hero" ]; then
        Q=manga
        code=$(curl -s --max-time "$TIMEOUT" -o "$TMPDIR/audit_${pid}.search" -w '%{http_code}' "$BASE/api/sandbox/plugins/${pid}/search?q=${Q}")
        ok=$(node -e 'try{const d=JSON.parse(require("fs").readFileSync(process.argv[1],"utf8"));console.log(Array.isArray(d)&&d.length>0?"1":"0")}catch(e){console.log("0")}' "$TMPDIR/audit_${pid}.search" 2>/dev/null)
    fi
    printf '%s %s' "$code" "$ok"
}

classify() { # $1=http_code $2=ok_expr_evaluated_as_1_or_0
    local code="$1" ok="$2"
    case "$code" in
        403) echo "BROKEN-403(challenge)";;
        000) echo "BROKEN(timeout)";;
        5?*|4*) echo "BROKEN(http$code)";;
        200) if [ "$ok" = "1" ]; then echo "WORKING"; else echo "MISSING"; fi;;
        *) echo "BROKEN(http$code)";;
    esac
}

# urlencode — percent-encode a string for safe embedding in a single URL path
# segment. The sandbox routes accept exactly one segment per {var} (chi), so a
# chapter id containing '/' (fanfox: "slug/c005.5") must be encoded: chi decodes
# %2F back to '/' inside the param, but a literal '/' is a path separator and the
# request 404s at the router before Lua ever runs.
urlencode() { local s="$1" out=""; local i c; for ((i=0; i<${#s}; i++)); do c="${s:i:1}"; case "$c" in [a-zA-Z0-9.~_-]) out+="$c";; *) out+="$(printf '%%%02X' "'$c")";; esac; done; printf '%s' "$out"; }

echo "[" > "$OUT_JSON"
first=1
plugins="$(node -e 'fetch(process.argv[1]+"/api/plugins").then(r=>r.json()).then(d=>console.log(d.map(p=>p.id).filter(Boolean).join("\n")))' "$BASE")"
n=0
for pid in $plugins; do
    n=$((n+1))
    Q=hero
    # search: sandbox search returns a bare array of items.
    read -r code search_ok < <(probe_search "$pid")
    search_cls=$(classify "$code" "$search_ok")
    mid=""
    if [ "$search_ok" = "1" ]; then
        mid=$(node -e 'try{const d=JSON.parse(require("fs").readFileSync(process.argv[1],"utf8"));console.log(Array.isArray(d)&&d[0]?d[0].id:"")}catch(e){console.log("")}' "$TMPDIR/audit_${pid}.search" 2>/dev/null)
    fi
    ch=0; ccls=""; pcls=""; dcls=""
    if [ -n "$mid" ]; then
        code=$(curl -s --max-time "$TIMEOUT" -o "$TMPDIR/audit_${pid}.detail" -w '%{http_code}' "$BASE/api/sandbox/plugins/${pid}/detail/${mid}")
        g_ok=$(node -e 'try{const d=JSON.parse(require("fs").readFileSync(process.argv[1],"utf8"));console.log(Array.isArray(d.genres)&&d.genres.length>0?"1":"0")}catch(e){console.log("0")}' "$TMPDIR/audit_${pid}.detail" 2>/dev/null)
        dcls=$(classify "$code" "$g_ok")
        # chapters: dedicated sandbox endpoint carries released_at.
        ccode=$(curl -s --max-time "$TIMEOUT" -o "$TMPDIR/audit_${pid}.chapters" -w '%{http_code}' "$BASE/api/sandbox/plugins/${pid}/chapters/${mid}")
        ch=$(node -e 'try{const a=JSON.parse(require("fs").readFileSync(process.argv[1],"utf8"));console.log(Array.isArray(a)?a.filter(c=>c.released_at&&c.released_at!=="").length:"0")}catch(e){console.log("0")}' "$TMPDIR/audit_${pid}.chapters" 2>/dev/null)
        ccls=$(classify "$ccode" "1")
        cid=$(node -e 'try{const a=JSON.parse(require("fs").readFileSync(process.argv[1],"utf8"));console.log(Array.isArray(a)&&a[0]?a[0].id:"")}catch(e){console.log("")}' "$TMPDIR/audit_${pid}.chapters" 2>/dev/null)
        if [ -n "$cid" ]; then
            cid_enc="$(urlencode "$cid")"
            code=$(curl -s --max-time "$TIMEOUT" -o "$TMPDIR/audit_${pid}.pages" -w '%{http_code}' "$BASE/api/sandbox/plugins/${pid}/pages/${cid_enc}")
            pl_ok=$(node -e 'try{const j=JSON.parse(require("fs").readFileSync(process.argv[1],"utf8"));const p=j.pages||j;console.log(Array.isArray(p)&&p.length>0&&p[0].url?"1":"0")}catch(e){console.log("0")}' "$TMPDIR/audit_${pid}.pages" 2>/dev/null)
            pcls=$(classify "$code" "$pl_ok")
        else
            ccls="BROKEN(nochapter)"
            pcls="BROKEN(nochapter)"
        fi
    else
        dcls="BROKEN(noresult)"; ccls="BROKEN(noresult)"; pcls="BROKEN(noresult)"
    fi
    [ "$first" = "1" ] && first=0 || printf ",\n" >> "$OUT_JSON"
    printf '{"plugin":"%s","search":"%s","detail":"%s","chapters_with_date":%s,"pages":"%s"}' \
        "$pid" "$search_cls" "${dcls:-MISSING}" "${ch:-0}" "${pcls:-MISSING}" >> "$OUT_JSON"
    md_line="| $pid | $search_cls | ${dcls:-MISSING} | ${ch:-0} | ${pcls:-MISSING} |"
    if [ "$n" = "1" ]; then
        printf '| plugin | search | detail | chapters w/ date | pages |\n' > "$OUT_MD"
        printf '|---|---|---|---|---|\n' >> "$OUT_MD"
    fi
    printf '%s\n' "$md_line" >> "$OUT_MD"
done
printf '\n' >> "$OUT_JSON"
echo "]" >> "$OUT_JSON"
echo "audit written: $OUT_JSON" >> "$OUT_MD"
echo "audit done -> $OUT_JSON + $OUT_MD"
