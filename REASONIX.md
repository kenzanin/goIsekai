# Reasonix — how agents work in this repo

## Language policy

- **Chat with the user: Indonesian**, casual and terse.
- **Everything else: English** — code comments, docs and specs, OpenSpec
  artifacts, commit messages, skill bodies, and any file written to disk.
- Keep identifiers, file paths, shell commands, and technical terms in their
  original form; never translate them.

## Subagent delegation

The `worker` subagent (`.reasonix/skills/worker/SKILL.md`) runs on `9router/free`:
a cheap model with a small step budget that never sees the orchestrator's
conversation. Strong at mechanical edits, weak at long autonomous runs.

**Split every delegation into the smallest task that still produces a verifiable
result** — one task = one package, one group of files, or one OpenSpec artifact.
Never hand over a whole change containing a dozen tasks at once.

- For a change with more than ~4 tasks, delegate in batches and continue the same
  transcript with `task(task_id: "sa_...")` instead of starting from scratch.
- **Always pass `max_steps: 120` explicitly on every call:**
  `task({profile: "worker", max_steps: 120, ...})`. The default is half the
  parent's cap, which is not enough for repo exploration plus edits. This is the
  only mechanism that works: `[agent].max_steps` is no longer used by Reasonix,
  and the skill frontmatter does not recognize the key. A frontmatter entry looks
  effective but is silently ignored — do not add one.
- Give mechanical instructions: exact file paths, the decision to follow, and the
  command that proves completion. The worker has no other context.
- **Verify the result before trusting it.** A cheap model reports success on code
  that does not compile — the worker once handed back a test with a syntax error
  and zero checkboxes ticked. Run the test it was supposed to run.
