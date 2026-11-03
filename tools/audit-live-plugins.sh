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
# Outputs (next to this script's cwd, i.e. repo root):
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

probe() { # $1=url $2=out-file -> echoes http_code; body saved to $2
    curl -s --max-time "$TIMEOUT" -o "$2" -w '%{http_code}' "$1"
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

# has_true "pattern" file — 1 if regex grep matches (genreal data, not just key)
has() { rg -q "$1" "$2" && echo 1 || echo 0; }

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
    # search
    code=$(probe "$BASE/api/search?pluginID=$pid&q=hero" "$TMPDIR/audit_${pid}.search" 2>/dev/null) || code=000
    search_ok=$(has '"title"' "$TMPDIR/audit_${pid}.search")
    search_cls=$(classify "$code" "$search_ok")
    # detail: take first search result id
    mid=$(node -e 'try{const d=JSON.parse(require("fs").readFileSync(process.argv[1],"utf8"));console.log((d.results&&d.results[0]&&d.results[0].id)||"")}catch(e){console.log("")}' "$TMPDIR/audit_${pid}.search" 2>/dev/null)
    ch=0; gm=0; ch_ok=0; g_ok=0; pl_ok=0; dcls=""; ccls=""; pcls=""
    if [ -n "$mid" ]; then
        code=$(probe "$BASE/api/manga/$pid/$mid" "$TMPDIR/audit_${pid}.detail") || code=000
        g_ok=$(has '"genres":\[' "$TMPDIR/audit_${pid}.detail")
        ch=$(has '"chapter_num"' "$TMPDIR/audit_${pid}.detail" >/dev/null 2>&1 && rg -c '"released_at":"[^"]+"' "$TMPDIR/audit_${pid}.detail" || echo 0)
        dcls=$(classify "$code" "$g_ok")
        # pages: first chapter id
        cid=$(node -e 'try{const d=JSON.parse(require("fs").readFileSync(process.argv[1],"utf8"));console.log((d.chapters&&d.chapters[0]&&d.chapters[0].id)||"")}catch(e){console.log("")}' "$TMPDIR/audit_${pid}.detail" 2>/dev/null)
        if [ -n "$cid" ]; then
            # Encode the chapter id: multi-segment ids (fanfox "slug/c005.5")
            # must stay a single chi {chapterID} segment, or the request 404s
            # at the router before Lua runs and the row is misclassified.
            cid_enc="$(urlencode "$cid")"
            code=$(probe "$BASE/api/sandbox/plugins/$pid/pages/$cid_enc" "$TMPDIR/audit_${pid}.pages")
            pl_ok=$(has '"url"' "$TMPDIR/audit_${pid}.pages")
            pcls=$(classify "$code" "$pl_ok")
            ccls=$(classify 200 "$ch")
        else
            ccls="BROKEN(nochapterFAQ)"
            pcls="BROKEN(nochapter)"
        fi
    else
        dcls="BROKEN(noresult)"; ccls="BROKEN(noresult)"; pcls="BROKEN(noresult)"
    fi
    [ "$first" = "1" ] && first=0 || printf ",\n" >> "$OUT_JSON"
    printf '{"plugin":"%s","search":"%s","detail":"%s","chapters_with_date":%s,"pages":"%s"}' \
        "$pid" "$search_cls" "$dcls" "${ch:-0}" "$pcls" >> "$OUT_JSON"
    # md row
    md_line="| $pid | $search_cls | $dcls | ${ch:-0} | $pcls |"
    if [ "$n" = "1" ]; then
        printf '%s\n|---|---|---|---|---|\n' \
"| plugin | search | detail | chapters w/ date | pages |" > "$OUT_MD"
    fi
    printf '%s\n' "$md_line" >> "$OUT_MD"
done
printf '\n' >> "$OUT_JSON"
echo "]" >> "$OUT_JSON"
echo "audit written: $OUT_JSON" >> "$OUT_MD"
echo "audit done -> $OUT_JSON + $OUT_MD"
