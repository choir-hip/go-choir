# Mission Ledger: Texture As A Long-Running Agent v0

Parallax ledger for `docs/mission-texture-long-running-agent-v0.md`. The paradoc
holds current Parallax State; this ledger records dated checkpoints, evidence,
and conjecture/heresy deltas.

## 2026-06-17 - Paradoc created (green)

Created the paramission from the deployed-fix falsification recorded in
`docs/mission-texture-product-loop-recovery-v0.ledger.md`. The owner-selected
direction after that falsification is to invert the core invariant: make the
Texture agent `texture:<docID>` a single long-running logical actor that writes an
immediate from-weights V1 and then deepens the document across many canonical
revisions as evidence streams in.

Belief state at creation:

- Confirmed by code review: one write per run is a hard DB-backed block
  (`tools_texture.go:566-573`, `store/texture.go:1753-1776`); Texture warm
  injection is disabled (`super_controller.go:434-436`, shipped in `68d09cc3`);
  the "persistent super" pattern is wake-driven ephemeral runs, not an immortal
  run; there is no park-and-wait primitive, a bare `maxToolLoopIterations=200`
  ceiling, no cumulative cost budget, and doc-delete does not cancel runs
  (`texture.go:1048-1061`).
- Confirmed by staging probe: prior increment `68d09cc3` deployed (staging health
  + sandbox both on the SHA) and produced V1-only at ~49s - necessary but not
  sufficient, and it reinforced the cap by disabling warm injection.

Mutation class of this checkpoint: green (docs only). Planned execution is red.

Remaining error / open edges: cost/runaway and cancellation are the top risks of
a long-lived actor; the budget kill-switch and the doc-delete->cancel gap must
close with the lifecycle change. The collapse of many revisions into one run must
keep trajectory/work-item attribution and the per-revision supervision narrative
legible (verifier and Trace at N:1).

Next move: hand the goal string to Codex to one-shot as far as it can safely
prove; then critically review (codex review + reading + deployed staging probe)
and iterate. Codex must leave precise, file-cited blockers for unfinished ramp
items.

Lineage: supersedes/folds in `mission-texture-product-loop-recovery-v0`. The
`68d09cc3` injector change is reverted/superseded by T1.
