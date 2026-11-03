# A stale management loop closes the next round; management cells fail on natural shapes

Date: 2026-11-03. Seen on: live deepseek arm, calendar round 1 (computer
computer-e80d229b0d78c35d47656b8c66426a43, Texture doc 29c0be04, trajectory
f704082c). Source: the round's trace review (finding 2) and
docs/evidence/selfdev-trace-review-synthesis-2026-11-03.md.

## Problem 1: a run end closes a round its run never saw (about 8 min a round)

Evidence (ledger):

- 20:21:43 management loop b3fe1201-6345-4fcf-84b9-80e8f6978607 starts,
  left over from the previous (Files) round.
- 20:26:16 Texture's Cast for the calendar request reaches management
  (record cell:703aafbb-237e-4425-9017-5ba50b4e5b8a:chatcmpl-tool-8e170cc6b9296765:cast:tray-1,
  round 1 of conv:management:...|texture:29c0be04-...).
- 20:26:29.212 b3fe1201 emits loop.completed.
- 20:26:29.295 the platform writes
  conv:management:0416df78-...|texture:29c0be04-...:r1:run-completed, a
  system closure of the new round.
- 20:26:29.352 loop cce191db starts, reads the Cast, and at 20:29:42 its
  engineering Cast is refused: "engineering cast: the round ... opened
  (round 1 of ...) is not open". Texture re-casts at 20:31:37; engineering
  starts at 20:32:43.

Cause (from code, confirms the trace's reading): `closeRoundsOfEndedRun`
(internal/agentcore/inbox_records.go, B6) closes every open round addressed
to the desk when a management run ends without a final Report and no other
run of the desk is active. It does not ask whether the ended run could have
seen the round. The calendar Cast arrived 13 s before b3fe1201 ended, was
never shown to it (above management's cursor), and opened a new
conversation; the run end closed it anyway. The re-check in the same
function then correctly woke a new loop for the unread Cast, which found
its round closed.

Fix direction: a run end leaves open a round with an unread record
committed after the run began. That is the re-check's own wake predicate,
so a held round always has a loop coming to read it.

Class: red (conversation rounds, run acceptance).

## Problem 2: management cells fail on shapes a model naturally writes (about 3 min)

Evidence (same round): `5:163: undefined: truncate`; `2:1: constant
definition loop | r := recs[0]` with the existing hint; `println(v.ExitCode)`
printed `exit 0xc9d4ab23020` (engineering, 20:34:49 and 20:45:00).

Findings:

- `truncate` is documented nowhere (no prompt, module or Help text); the
  model invented it. Prompt fix: say there is none and give the
  `s[:min(n, len(s))]` form (min and max are predeclared).
- yaegi refuses a top-level `x := v[i]`, `x := v.F.G` and `a, ok := ...` on
  many values, and `var x = v[i]` fails the same way, so suggesting the
  `var` form would be wrong. The management prompt already names the shape;
  the model wrote it anyway. The substrate fix of recompiling the cell with
  its tail inside `func() { ... }()` after the refusal is NOT safe: tried
  while building, yaegi then re-ran the rewritten statements on every later
  cell of the session (a staged Cast would repeat). Wrapping before the
  first compile would change cells that do compile (their names stop
  persisting). Fix: the refusal hands back the corrected cell, rewritten
  from the first such definition into `func() { ... }()`, for the desk to
  send as is.
- `JobView.ExitCode` is `*int` (nil while the job runs). Doc fix in the
  engineering prompt: dereference after the nil check, or print the job.

Class: orange (yaegi refusal text), green (prompts).
