# Mission: Texture As A Long-Running Agent v0

## Summary

Texture is currently a one-canonical-write-per-run, wake-driven actor that goes
dormant between deliveries. Staging measurement proves this caps the product at a
single revision (V1) that lands ~49-60s after the prompt with nothing shown in
between. That shape cannot deliver the three things Texture is for: a fast first
paint, a document that deepens across many revisions ("deep research" rather than
one chatbot answer), and a live control plane for supervising long-running coding
agent hierarchies (a fresh revision roughly every interval over hours/days).

This mission inverts the core invariant. The Texture agent `texture:<docID>`
becomes a single long-running logical actor: it writes an immediate from-weights
V1, then deepens the document across many canonical revisions as researcher and
super evidence streams in. When evidence pauses it parks cheaply; when the
sandbox restarts it sleeps and resumes as the same logical actor. The wake /
reconcile / one-write scaffolding that existed only to simulate continuity across
ephemeral runs is removed in favor of one uniform long-running-actor foundation
(park-and-wait + a per-actor budget) reusable by other roles.

This supersedes the cadence-fix increment shipped as `68d09cc3`, which disabled
Texture warm injection and thereby reinforced the very one-revision cap this
mission removes. The deployed probe (recorded in
`docs/mission-texture-product-loop-recovery-v0.ledger.md`) falsified that
increment as necessary-but-not-sufficient.

## Problem

Three coupled facts, all confirmed by code review and staging measurement:

1. One write per run is a hard, DB-backed block, not just a prompt. Each Texture
   run creates one `AgentMutation` row (`pending`); the first
   `patch_texture`/`rewrite_texture` flips it to `completed`
   (`internal/runtime/tools_texture.go:566-573`,
   `internal/store/texture.go:1753-1776`). A second write in the same run is
   rejected. So N revisions require N runs.
2. Texture deliberately opts out of warm injection
   (`internal/runtime/super_controller.go:434-436`, shipped in `68d09cc3`), so a
   live run cannot incorporate a second findings packet.
3. The first findings packet arrives late (~49s on staging) and the researcher
   is not checkpointing early despite its overlay mandating it
   (`internal/runtime/runtimeprompts/overlays/researcher_runtime.yaml:8,17-23`).
   First paint is gated on the first packet, not on the wake debounce.

The "persistent super" pattern is not an immortal run either: it is durable
identity + wake-driven ephemeral runs + warm injection while resident +
completion-chained inbox drain. Texture already mirrors it. The honest target is
therefore one logical actor per article, backed by a real park-and-wait + budget
primitive, because the runtime today has no cheap idle-wait, a bare
`maxToolLoopIterations=200` ceiling, and no cumulative cost budget.

## Owner Direction

- Texture is a long-running agent, not a dormant-then-woken one. The Texture
  agent should do multiple revisions per run; one run per Texture agent is the
  target minimal model.
- The first revision (V1) is an immediate from-weights draft produced before any
  researcher retrieval, then grounded and deepened by retrieval in later
  revisions.
- This is one paramission. How much lands in the first pass is open; Codex
  one-shots as far as it can and leaves precise blockers, then the owner-side
  reviewer (Cursor agent) critically reviews and iterates.

## Scope / Domain Ramp

The variant to burn down, roughly in dependency order. Codex should land as much
as is safely provable in one pass and record precise blockers for the rest.

- T1 Revision-cadence core. Remove the per-run one-write `AgentMutation` gate so
  a resident Texture run can write multiple canonical revisions; re-enable warm
  injection for Texture (supersede the `nil` short-circuit at
  `super_controller.go:434-436`); track consumed worker updates per revision, not
  by one per-run seq watermark (`runtime.go:2944-3019`,
  `markTextureWorkerUpdatesDelivered`).
- T2 From-weights V1. On activation, emit an immediate model-prior initial
  revision before researcher retrieval; flag it model-prior/interim in revision
  metadata so it is never mistaken for grounded; amend the grounding doctrine and
  overlays (`textureprompts/overlays/revision_policy.yaml`, `run_system.yaml`,
  `tools_coagent.go` guidance) to authorize this explicitly.
- T3 Long-running-actor foundation (harness, uniform). A park-and-wait primitive
  so an actor blocks with no billed provider calls until a new packet/signal or a
  budget/idle deadline; a cumulative per-actor cost/time budget + kill-switch
  replacing the bare `maxToolLoopIterations=200` for parked actors. Keep it
  role-uniform per AGENTS.md harness-minimalism; do not branch the core loop for
  Texture alone.
- T4 One-run-per-agent lifecycle. `texture:<docID>` owns at most one resident
  run; it parks when idle instead of ending. Remove or collapse the
  wake/reconcile/completion-chain/residency scaffolding it replaces
  (`texture_controller.go`, `reconcileCompletedTextureRun`) where the parked
  resident run now covers that role.
- T5 Passivation-as-sleep. vmctl refresh sleeps the actor; rewarm resumes the
  same logical actor from the run-memory snapshot (`run_memory.go:74-123`),
  with correct stale-mutation handling for multi-write runs
  (`runtime.go:1264-1312`).
- T6 Cancellation gaps. Document deletion must cancel the actor's run
  (`texture.go:1048-1061` currently does not); `CancelAgent` ends the parked run.
- T7 Verifier / tests / docs / heresy. Relax the verifier 1:1 revision-run-write
  causality to N:1 (`texture_workflow_verifier.go:527-593`); update comprehensive
  runtime tests, frontend specs, the doctrine in
  `docs/texture-agentic-invariants-2026-06-13.md` (Rule 4 and the cold/warm
  delivery contract), and the `initialTextureToolChoice`/`WithInitialToolChoice`
  heresy detectors (`cmd/doccheck/main.go:1135`, `docs/heresy-detectors.md`).
- T8 Deploy + staging proof. Fix the cadence probe's hang and the Playwright
  import portability flagged by Codex, then push -> CI -> staging deploy identity
  -> re-run `scripts/texture_revision_cadence_probe.mjs` and record a
  RunAcceptanceRecord.

## Suggested Goal String

```text
/goal Use Parallax on docs/mission-texture-long-running-agent-v0.md. Make the Texture agent texture:<docID> a single long-running logical actor that writes an immediate from-weights V1 (flagged model-prior/interim in revision metadata) and then deepens the document across many canonical revisions as researcher/super findings packets arrive, replacing today's one-canonical-write-per-run, wake-driven-dormant model. Ground truth and rationale are in docs/mission-texture-long-running-agent-v0.md and the falsification of the prior increment 68d09cc3 in docs/mission-texture-product-loop-recovery-v0.ledger.md (deployed staging probe: V1-only at ~49s). This is one paramission; land as much as you can safely prove in one pass and leave precise, file-cited blockers for the rest. Work the ramp in dependency order: (T1) remove the per-run one-write AgentMutation gate (internal/runtime/tools_texture.go:566-573, internal/store/texture.go:1753-1776) so a resident run can write multiple canonical revisions, re-enable warm injection for Texture by superseding the nil short-circuit at internal/runtime/super_controller.go:434-436, and track consumed worker updates per revision rather than by one per-run seq watermark (internal/runtime/runtime.go:2944-3019); (T2) emit an immediate from-weights initial revision before researcher retrieval, flagged model-prior/interim, and amend the grounding doctrine/overlays (textureprompts/overlays/revision_policy.yaml, run_system.yaml) to authorize it; (T3) add a uniform, role-agnostic long-running-actor foundation: a park-and-wait primitive that blocks with no billed provider calls until a new packet/signal or a budget/idle deadline, plus a cumulative per-actor cost/time budget and kill-switch replacing the bare maxToolLoopIterations=200 for parked actors (internal/runtime/toolloop.go) - do not branch the core loop for Texture alone (AGENTS.md harness-minimalism); (T4) make texture:<docID> own at most one resident run that parks when idle, and remove or collapse the wake/reconcile/completion-chain/residency scaffolding it replaces (internal/runtime/texture_controller.go, reconcileCompletedTextureRun); (T5) treat vmctl-refresh passivation as sleep and rewarm the same logical actor from the run-memory snapshot (internal/runtime/run_memory.go:74-123, runtime.go:1264-1312) with correct stale-mutation handling for multi-write runs; (T6) make document deletion cancel the actor's run (internal/runtime/texture.go:1048-1061) and CancelAgent end the parked run; (T7) relax the verifier to N:1 revision-run-write causality (internal/runtime/texture_workflow_verifier.go:527-593), update comprehensive runtime tests, frontend specs, docs/texture-agentic-invariants-2026-06-13.md (Rule 4 + cold/warm delivery contract), and the initialTextureToolChoice/WithInitialToolChoice heresy detectors (cmd/doccheck/main.go:1135, docs/heresy-detectors.md). Mutation class red: protected surfaces are Texture canonical writes, the run tool loop / park-and-wait / budget, passivation/rewarm, worker-update consumption bookkeeping, the verifier, Trace, and deployment routing. Name the conjecture delta, protected surfaces, admissible evidence class, rollback path, and heresy delta before touching orange/red surfaces. Preserve these invariants: canonical integrity (monotonic versions, no lost foreground updates, no duplicate/garbled writes); grounding honesty (from-weights V1 explicitly flagged, grounded revisions cite evidence); bounded cost (per-actor budget + kill-switch, parked time costs no LLM calls); restart safety (sleep/resume same logical actor); harness-minimalism (one uniform park-and-wait primitive, not a Texture branch); observability (per-revision Trace/Texture boundaries within one run). Do not add compatibility shims or semantic role-choreography decision trees. Verify with focused internal/runtime tests for multi-write-per-run, warm-injection-on-for-Texture, park-and-wait, budget kill-switch, and N:1 verifier causality; run scripts/go-test-runtime-shards; frontend build if touched; then commit -> push origin main -> CI -> Node B deploy identity (host + sandbox upstream commit) -> deployed proof on https://choir.news via scripts/texture_revision_cadence_probe.mjs showing from-weights first paint well under the ~49s baseline and multiple grounded revisions (V2+) that track findings packets. Fix the probe's hang and its gitignored Playwright import before relying on it. Record receipts and a RunAcceptanceRecord at staging-smoke-level. Rollback is revert of the mission commits, restoring the AgentMutation gate and the injector nil.
```

## Parallax State

status: planned

mission conjecture: if Texture becomes a single long-running logical actor that
drafts an immediate from-weights V1 and deepens the document across many
canonical revisions as evidence streams in - backed by a uniform park-and-wait +
per-actor budget foundation and sleep/resume across restarts - then Choir gets a
fast first paint, deep multi-revision answers, and a live supervision control
plane for long-running agent hierarchies, while the wake/one-write scaffolding
that only simulated continuity is removed.

deeper goal (G): Texture is Choir's human-supervision narrative and artifact
control plane. A continuously-updating long-running actor is the substrate for
supervising hours/days-long coding-agent hierarchies through a revision stream,
not a one-shot draft pane.

witness/spec (A/S): a deployed staging run where the prompt yields a from-weights
V1 well under the ~49s baseline, multiple grounded revisions (V2..Vn) that track
findings packets rather than collapsing into one write, one logical actor per
doc, survival of a vmctl refresh as sleep/resume without lost doc or duplicate
revisions, bounded per-actor cost, and cancellation on doc delete.

invariants / qualities / domain ramp (I/Q/D):

- I: canonical integrity - monotonic version numbers, no lost foreground
  updates, no duplicate or garbled revisions.
- I: grounding honesty - the from-weights V1 is explicitly flagged
  model-prior/interim in revision metadata; grounded revisions cite evidence;
  priors never masquerade as grounded.
- I: bounded cost - a cumulative per-actor cost/time budget plus kill-switch
  replaces the bare maxToolLoopIterations ceiling; parked time costs no LLM
  calls.
- I: restart safety - passivation is sleep; rewarm resumes the same logical
  actor from the run-memory snapshot; multi-write stale-mutation cleanup is
  correct.
- I: harness-minimalism - park-and-wait and the budget are uniform, role-agnostic
  primitives, not Texture-only core-loop branches.
- I: no compatibility shims, no semantic role-choreography decision trees in the
  cadence.
- Q: acceptance proves the product path (prompt-bar + public APIs), not manual
  worker invocation.
- Q: per-revision Trace/Texture boundaries remain legible within one run;
  trajectory/work-item attribution preserved.
- D: land cadence + from-weights V1 under focused local verification first, then
  the park-and-wait foundation, then deployed staging proof.

variant (ranking function) V: large and open. V descends as each ramp item lands
with evidence: multi-write-per-run proven; warm injection on for Texture;
per-revision consumption tracking; from-weights V1 flagged and shown fast;
park-and-wait blocks without billed calls; per-actor budget enforced with a
kill-switch; one-run-per-agent lifecycle with scaffolding removed; sleep/resume
across vmctl refresh; doc-delete cancels; verifier N:1; tests/docs/heresy updated;
deployed probe shows fast from-weights first paint and V2+ tracking findings.

budget: one broad red-surface paramission executed iteratively (Codex one-shot ->
critical review -> iterate). Broad change is authorized; there are no real users
yet and the owner values the correct long-running-agent architecture over
incremental compatibility.

authority / bounds: document creation here is green. Execution is red: Texture
canonical writes, the run tool loop / park-and-wait / budget, passivation and
rewarm, worker-update consumption bookkeeping, the verifier, Trace, and
deployment routing. Before touching orange/red surfaces the executor must name
the conjecture delta, protected surfaces, admissible evidence class, rollback
path, and heresy delta.

mutation class / protected surfaces: planned execution mutation class red.
Protected surfaces: Texture canonical writes, run lifecycle / tool loop, the new
park-and-wait and budget primitives, passivation/rewarm, worker-update
consumption/pending bookkeeping, the Texture workflow verifier, Trace/evidence,
and deployment routing.

evidence packet: focused local tests for multi-write-per-run,
warm-injection-on-for-Texture, park-and-wait blocking without billed calls,
budget kill-switch, sleep/resume, and N:1 verifier causality; go-test-runtime
shards; frontend build if touched; CI run; deploy run; staging health host +
sandbox upstream commit identity; deployed probe output (from-weights first-paint
time, revision count, findings-packet correlation); prompt-bar submission id,
conductor run id, doc id, Texture run id; RunAcceptanceRecord at
staging-smoke-level; residual risk note. Rollback ref is revert of the mission
commits.

heresy delta: discovered - the one-write-per-run + warm-injection-off shape (the
latter introduced by 68d09cc3) caps Texture at a single late revision and is the
root of the V1-only product defect; the runtime has no cheap park-and-wait, no
cumulative cost budget, and a doc-delete that does not cancel runs. To be
repaired by this mission. Do not count these newly named heresies as regressions;
do not count their naming as repair.

position / live conjectures / open edges:

- C1 active: one-write-per-run is the artificial cap. Removing the AgentMutation
  gate plus enabling warm injection lets a resident run deepen the doc; this is
  the core lever.
- C2 active: first paint is gated by the first findings packet, not the wake
  debounce. A from-weights V1 before retrieval is the intended fast-paint fix;
  it requires a doctrine change to allow a flagged model-prior initial revision.
- C3 active: "one run per agent" is more minimal as a model but requires a real
  park-and-wait + budget; without them a long run either idles on billed calls or
  hits the 200-iteration ceiling. The park-and-wait must be role-uniform.
- C4 active: passivation means even "one run" is one logical actor across
  physical runs; sleep/resume from the run-memory snapshot is the continuity
  mechanism, not an immortal process.
- C5 open edge: cost/runaway and cancellation are the top risks of a long-lived
  actor; the budget kill-switch and the doc-delete->cancel gap must close before
  or with the lifecycle change.
- C6 open edge: collapsing many revisions into one run must not muddy
  trajectory/work-item attribution or the per-revision supervision narrative;
  verifier and Trace projection must stay legible at N:1.

lineage: supersedes and folds in
`docs/mission-texture-product-loop-recovery-v0.md` (the V1-only cadence defect
and the `68d09cc3` falsification are its direct lineage). Sits on the portfolio
spine after the Texture product-loop work and before continuation deletion (M4);
this mission's lifecycle/passivation work should be reconciled with M4 and the
durable-actors rearchitecture.

learning state: prior increment `68d09cc3` deployed and falsified on staging
(V1-only at ~49s), proving warm-injection-off was necessary-but-not-sufficient
and reinforced the cap. The redesign direction (long-running actor + from-weights
V1) was owner-selected after that falsification.

settlement requirement: settles only with deployed staging proof of a
from-weights first paint well under the ~49s baseline and multiple grounded
revisions (V2+) that track findings packets, one logical actor per doc surviving
a vmctl refresh as sleep/resume, an enforced per-actor budget, doc-delete
cancellation, updated verifier/tests/docs/heresy detectors, and a
RunAcceptanceRecord at staging-smoke-level (or higher).
