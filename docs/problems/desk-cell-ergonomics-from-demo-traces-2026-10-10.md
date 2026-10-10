# Desk mistakes in the demo traces: what wasted cells, and why (2026-10-10)

Source: all-agents traces of the Minesweeper and arXiv demo reruns (Texture,
management, research; scratchpad trace_all.sh snapshots, 18:05–18:40Z).

## Not the model's fault

- Every management cell ran twice under one call id. The duplicate's failure
  read as an API error after the original had already committed
  (docs/problems/responses-stream-duplicates-tool-calls-2026-10-10.md).
- Every delegated cast was reaped before its spawn, so management's correct
  `choir.Cast("engineering", …)` never produced engineering work
  (docs/problems/delegated-cast-reaped-before-spawn-2026-10-10.md).

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
  worker twice.

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
