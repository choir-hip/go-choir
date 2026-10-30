# Desk mistakes in the demo traces: what wasted cells, and why (2026-10-10)

Source: all-agents traces of the Minesweeper and arXiv demo reruns (Texture,
management, research; scratchpad trace_all.sh snapshots, 18:05–18:40Z).

## Not the model's fault

- Every management cell ran twice under one call id. The duplicate's failure
  read as an API error after the original had already committed
  (docs/archive/problems/responses-stream-duplicates-tool-calls-2026-10-10.md).
- Every delegated cast was reaped before its spawn, so management's correct
  `choir.Cast("engineering", …)` never produced engineering work
  (docs/problems/delegated-cast-authority-dies-with-the-caster-turn-2026-10-10.md).

## Kernel ergonomics (every desk)

- K1. Missing imports. "undefined: fmt" and "undefined: json" appeared in
  management and Texture, and a grouped import after a failed compile gave
  "json/_.go redeclared". About 1–3 cells per activation.
- K2. Values never shown. A cell ending in a bare expression (`updates`,
  `choir.Help("Report")`) returned an empty stdout. Models fell back to
  `panic(string(b))` or to guessing methods (`u.MarshalJSON`, `ups[i].Kind`,
  `ups[i].String()`, three failed cells in one Texture run).
- K3. A yaegi compile panic (compDefineX on `b, _ := json.Marshal(…)` with
  json undefined) is not recovered and crashes the desk worker process.
- K4. (open) "variable definition loop" at runtime when a later cell
  re-declares a top-level name (`objective := …`) poisoned management's
  worker twice. Cause (reproduced by replaying the traced cells): cells that
  begin with `import "fmt"` take the import+statement path, which hoists
  each new `x := e` to a package-level `var x = e` and drops rebinds into a
  synthetic main. In the second such cell, `ref, err := choir.Cast(…,
  objective, spec)` was hoisted above the body that reassigns objective and
  spec. That reorders the cell, and yaegi refuses it as a definition loop at
  execution, which poisons. A rebind like `ref, err :=` with err bound also
  stayed local to main, so ref never reached the next cell.
- K5. (open) yaegi's top-level statement mode reports "undefined: b" for a
  valid `b, err := json.Marshal(x)` inside a top-level range body (Texture
  run b5506da4, 2026-10-10 rerun). The same code inside a function compiles.
- K6. (open, upstream) `x := u[0]` at top level, where u is the
  `choir.Updates()` slice, fails "constant definition loop" at compile
  (session preserved). Ranging over u, or indexing inside a function, works.
- K7. (open) a cell ending in `func() { … }()` shows `=> 0x…`, the closure
  call's meaningless value.
- K8. (open) a top-level `y := y * 2` on a bound name in statement mode
  declares a fresh, zero y.

- K9. (open) A one-line parenthesized import followed by statements,
  `import ("encoding/json"; "fmt")` then `dump := func(…) {…}`, fails
  `expected declaration, found dump`. The normalizer recognizes only
  `import "x"` lines and a multi-line `import (` block. Engineering run
  `run:delegated-sha256:b66e551d…` (arXiv demo, 21:14:39 and 21:14:43) paid
  two failed cells, then sent the import block alone.
- K10. (open, introduced by the K8 fix) Redefining a bound helper with a new
  type, `sh := func(cmd string, args []string, env []string) {…}` after `sh`
  held a `func(string, ...string)`, fails `cannot use type … in
  assignment`. The K8 rewrite turns every fully bound top-level `:=` into
  `=`. Same run, 21:18:08.
- K11. (open) Compile errors give only `_.go:4:176: missing ','` without the
  offending line. In the same run the model misread its own `"]` typo (for
  `"}`) as "long lines seem to be getting truncated by the transport"
  (21:19:52), and the same typo failed three cells (21:18:59, 21:19:47,
  21:27:14).

## Prompt gaps (management)

- M1. The management prompt was two paragraphs. It never said how to open
  engineering from a Texture request or how to answer Texture. Management
  tried `choir.Report("texture:<doc>")` (unknown desk), then
  `ReportPacket("texture")` with guessed packet shapes ("claim",
  "coordination_record", "execution_request_ack").
- M2. No rule against re-running a cell whose acts already committed. With
  the duplicate bug that produced five casts for one request.

## Texture (minor)

- T1. Put `file_mutation` at the top level of ApplyTexture, and used
  "decision" for decision_kind. One retry each; the rejections already name
  the valid fields.
- T2. (open) Strict decode errors give no location. Texture runs d878d267
  and 1803835a (arXiv demo, 21:17-21:21) lost five cells to
  `unknown field "decision"`, `"rationale"` and `"questions_note"`. The
  hint lists the top-level fields, and `rationale` is valid there; it was
  misplaced inside a nested object. So the message read as a
  contradiction.

- T3. (open) Texture controls aimed at research work fail with `target
  work does not bind the open target obligation and caller provenance`.
  Six distinct work items, across Texture runs 536ee5ef, 4d7e15d3,
  78f029d3, 93c9760d, d878d267 and 1803835a (arxiv, arxiv5-7), drew 26
  refusals (counted across overlapping trace snapshots). The pattern
  (hypothesis from the traces; the work rows were not read): Texture reads
  a research report whose update carries `work_item_id`, then follows up
  with a control on that `target_work_item_id`. The work was opened by an
  earlier Texture run of the same document, and
  `lifecycleTargetWorkRequestedByTexture`
  (`internal/store/lifecycle_texture_target.go`) requires the caller run
  to be the creating run. That run-level provenance is deliberate (the
  August 8 test "same channel is not provenance"; kept intact in
  docs/archive/evidence/effects-red-texture-caller-reactivate-2026-08-17.md), so
  this record does not change it. The defect is the message. One error
  string covers seven conditions (wrong id, scope, trajectory, closed
  status, assignee, authority profile, provenance) and says nothing about
  what Texture may do instead, so Texture retried the same control up to
  three times per run. Fix direction: name the failing condition and the
  alternative (a new research request, which this run then owns). Whether
  Texture provenance should be per document desk rather than per run is an
  open owner question: the persistent-management findings of this day point
  the same way.

- T4. (open, low) `choir.Updates()` lists every pending report, but a
  Texture turn may dispose only those at or before its
  `scheduled_message_seq`. Research-only demo, Texture run 1d9c40b0 at
  22:48:50: a disposition naming a report that arrived mid-turn was
  refused ("its message seq 15 is after this activation's scheduled seq
  13"); the next cell dropped it and committed. Cost: one cell. Fix
  direction: mark such updates in the cell view as arriving for the next
  turn.

## Research (tools)

- R1. (closed 2026-10-13: `choir.SourceSearch` and sourcecycled were deleted
  in Cut 2 unit 2b) `choir.SourceSearch` fails `dial tcp 10.200.57.1:8787:
  connection refused`. The source service (go-choir-sourcecycled) is
  deliberately default-off on Node B since 2026-10-05, but research is
  still offered the verb. Research run 64f09172 spent a cell on it, and
  its report blamed "the source service was unavailable".
- R2. (traced; cause revised) Two `choir.WebSearch` calls in one cell
  failed with `context deadline exceeded` (research run 64f09172 at
  21:17:09). The deadline was the cell's own: research set
  `timeout_ms: 2000` on a cell making two searches. The providers were not
  at fault. Across the saved demo traces (arxiv5-7, ms5), research ran 11
  cells calling WebSearch or FetchURL. Six died on a `timeout_ms` it chose
  itself (2000, 3000, 4000 or 5000 ms). The five that completed took 0.2,
  3.3, 4.8, 4.8 and 10.5 s; one finished 0.2 s inside a 5 s limit. Every
  timed-out result read `{"error":"context deadline exceeded","duration":0}`.
  It did not name the cell's limit, how long the cell ran, or that
  `DeskSessionWorker.EvalCell` kills the session worker on that deadline,
  so every value defined in earlier cells is gone. The tool schema
  describes `timeout_ms` only as "Evaluation timeout in milliseconds".
  Fix direction: the timed-out result names the limit, the elapsed time,
  the lost session, and the observed cost of host calls; the schema says
  host calls take seconds and that leaving `timeout_ms` unset is
  normal.
- R3. (observed, desk behavior) Research reported two candidate papers.
  Both need a GPU or a 7B model, which does not fit "very little compute".
  The cast reached engineering before research reported, so engineering
  picked arXiv:2209.10652 on its own. It is a 2022 paper, which does not
  fit "recent". Hypothesis, not yet traced: management cast in parallel
  with research instead of waiting for its pick.

