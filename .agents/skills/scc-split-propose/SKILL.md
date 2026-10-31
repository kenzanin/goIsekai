---
name: scc-split-propose
description: Propose an OpenSpec change that splits source files with 200+ lines of code into smaller files (move-only refactor) and completes unit tests for the split units. Use when the user says "scc split", "pecah file besar", "split large files", or wants an scc scan feeding an OpenSpec proposal. Triggers the openspec-propose workflow with the scan pre-filled.
allowed-tools: Bash(scc:*), Bash(openspec:*), Bash(awk:*), Bash(grep:*), Bash(sort:*), Bash(cut:*), Bash(head:*), Bash(cat:*)
license: MIT
compatibility: Requires scc + openspec CLI.
metadata:
  author: kenzanin
  version: "1.0"
  scope: project
---

Propose an OpenSpec change for splitting oversized source files, seeded by an `scc` scan. This is a thin, project-tuned wrapper around the `openspec-propose` workflow — after the scan, follow that skill's full artifact steps.

## Why this shortcut exists

The user splits files regularly. Every run repeats the same two steps: scan with scc, then hand-feed the results into an OpenSpec proposal. This skill freezes the project-specific scan command and the refactor conventions so nothing is re-derived per run.

## Step 1 — Scan

Run (from the repo root):

```bash
scc --by-file --no-cocomo --no-complexity -f csv internal/ cmd/ > "$TMPDIR/scc_split.csv"
grep -oE '^[^,]*,[^,]*,[^,]*,[2-9][0-9]{2,},' "$TMPDIR/scc_split.csv" \
  | grep -v "_test" \
  | sort -t, -k4 -rn \
  | cut -d',' -f4,2
```

Notes:
- CSV field 4 = lines, field 2 = path. The grep keeps counts of 200–999; extend the pattern if a file ever exceeds 999 lines (`[2-9][0-9]{2,},` already covers 200+).
- Test files and vendored/minified assets (`*.min.js`, generated `tailwind.css`) are excluded — never propose splitting them.
- If the user names other directories (e.g., `goisekai-qt6/`), scan those too.

## Step 2 — Propose via openspec-propose

Invoke the `openspec-propose` skill with a change name like `split-<area>-files` (kebab-case) and this description, pasting the scan table as scope. These project conventions are already decided — carry them into the artifacts verbatim instead of re-asking:

1. **Move-only**: functions/sections relocate verbatim; no new logic, no new abstractions, no renames.
2. **Pure refactor → `skip_specs: true`** in the change's `.openspec.yaml` (no spec deltas; openspec validate accepts zero deltas with that flag).
3. **Go**: same-package file moves only; name siblings after their concern (`views_library.go` → `views_library_filters.go`). `database/schema.go` migrations literal is intentionally kept — record the decision, don't split an ordered migration list.
4. **Lua templates**: extract top-level sections into `partials/<view>_<section>.lua` via the existing `require("partials.x")` contract; split at existing section banner comments, never mid-element. Verify with a byte-equality golden render test per split view.
5. **Unit tests**: add tests for each split Go unit as it lands, prioritizing files with thin coverage; one file split = one commit, `just check` green per commit.
6. **Exclusions**: `cmd/goisekai/frontend/lib/tailwind.css`, any `*.min.js`.

## Step 3 — Output

Present the artifacts as `openspec-propose` prescribes (status table, key decisions, "ready for review / run /openspec-apply-change"). Do NOT start apply — that boundary belongs to the apply workflow.
