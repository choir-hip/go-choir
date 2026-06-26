# Overnight Autoradio Platform Checklist v0

## Purpose

This paradoc is the source program for an overnight, thread-native `/goal`
mission. The product forcing function is Autoradio: a screenless Choir surface
that continuously performs source-grounded, relevant material and accepts voice
interruption as first-class artifact input.

The execution order is deliberately constrained:

1. Object graph.
2. News / Universal Wire.
3. Choir-in-Choir self-development.
4. Nucleus capsules.
5. Choir Base.
6. Autoradio and Pipecat audio.

Autoradio is the north-star benchmark, but the mission must not build a
parallel audio toy. It must pull durable object, source, News, self-development,
sandboxing, and file-provider substrate forward in the order above, then use a
thin Autoradio vertical slice to prove that the substrate is useful.

## Required References

- `AGENTS.md`
- `docs/choir-doctrine.md`
- `docs/parallax-design-2026-06-11.md`
- `docs/worktree-review-2026-06-23.md`
- `docs/report-conceptual-refactor-2026-06-23.md`
- `docs/news-voice-autoradio-forward-plan-2026-06-06.md`
- `docs/paradoc-object-service-prototype.md`
- `docs/paradoc-qdrant-indexing-pipeline.md`
- `docs/paradoc-source-entity-migration.md`
- `docs/paradoc-universal-wire-diagnosis.md`
- `docs/mission-choir-in-choir-platform-pr-accelerator-v0.md`
- `docs/handoff-hybrid-computer-capsule-architecture-2026-06-10.md`
- `docs/mission-choir-base-reconciliation-kernel-v0.md`

## Thread Operating Model

This mission should be run with Codex thread tools. The current Codex app
surface, as discovered on 2026-06-26, exposes the needed thread primitives via
`codex_app` after tool discovery. The orchestration thread is the human-visible
conductor for the overnight run. It owns this checklist, the ledger, worktree
hygiene, dependency order, and final evidence packet. It does not self-certify
implementation work.

Thread roles:

- **Orchestration thread:** reads this paradoc, updates Parallax State, creates
  worker and verifier threads, receives callbacks, chooses the next move by
  expected variant decrease per budget, and performs final evidence synthesis.
- **Implementation worker threads:** receive one bounded work item with
  authority limits, mutation class, protected surfaces, admissible evidence,
  rollback path, and callback target. They may implement only their assigned
  slice and must report dirty paths, commits, tests, blockers, and residual
  risk back to orchestration.
- **Verifier threads:** start from a clear context, read the paradoc plus the
  assigned diff or artifact, run review/proof commands as needed, and report
  findings first with file/line references and verdict `accept`,
  `revise_before_continue`, `blocked`, or `supersede`.

Required thread primitives and current semantics:

- `list_projects` to choose the `go-choir` project before creating
  project-scoped threads.
- `create_thread` to start orchestration-owned implementation and verifier
  threads. Prefer project worktree threads for implementation work and local
  project threads for read-only verifier work when that preserves independence.
- `send_message_to_thread` for orchestration follow-ups to active worker or
  verifier threads. Worker/verifier assignment prompts must include the
  orchestration thread id or callback instructions if the worker is expected to
  report back through thread tools.
- `read_thread` and `list_threads` to reconnect verdicts, recover interrupted
  context, and audit thread status without depending on chat-visible callbacks.
- `handoff_thread` plus `get_handoff_status` to transfer an existing thread and
  its git state between its checkout and a Codex worktree on the current host
  when ownership or execution substrate must change. Handoff interrupts a
  running thread, so it is an ownership-transfer tool, not a routine callback
  path.
- `set_thread_title`, `set_thread_pinned`, and `set_thread_archived` for
  operator hygiene: name worker/verifier threads by work item id, pin live
  orchestration-critical threads, and archive settled or superseded threads only
  after their evidence is recorded.

If these tools are not exposed in the executing environment, the orchestration
thread must record that as a capability blocker. It may still perform local
planning or a single-thread checkpoint, but it must not claim a thread-native
overnight settlement or treat same-context review as independent proof.

Thread messages must be durable enough to resume: every worker assignment names
the paradoc path, ledger path, work item id, target files or surfaces, current
claim, admissible evidence, callback target, and stop condition.

## Overnight Work Items

### O0 - Preserve The Current WIP Queue

Goal: make the existing salvageable work durable before broad construction.

Checklist:

- [x] Record current main SHA and clean/dirty status.
- [x] Inventory all Codex/Cascade worktrees.
- [x] Mark the email-freeze worktrees superseded by main.
- [x] Preserve object-service prototype work before any cleanup.
- [x] Preserve Qdrant pipeline prototype work before any cleanup.
- [x] Preserve source-entity migration design state.
- [x] Preserve Universal Wire diagnosis state.
- [x] Extract PPTX prototype learning and mark prototype code disposable unless
  revived by a later mission.
- [x] Record recovery handles for any stash, branch, commit, or thread.

Evidence: `git worktree list --porcelain`, `git status --short` per worktree,
branch/commit/stash refs, and a ledger entry naming intentional source,
durable docs/evidence, temporary proof output, generated artifacts, and
unrelated WIP.

### O1 - Object Graph Foundation

Goal: land the smallest real object graph substrate that can carry source,
web-capture, media, run-sheet, Qdrant, and Base objects.

Checklist:

- [x] Review the object-service prototype for fit with current main.
- [x] Decide whether to land as `internal/objectgraph`, narrower package, or
  design-only successor.
- [x] Preserve stable object identity and content hash semantics.
- [x] Preserve edge storage semantics.
- [x] Add or retain focused tests for memory and SQLite stores.
- [x] Add the next missing integration test before claiming durable persistence.
- [x] Define minimal object kinds needed by News and Autoradio:
  `choir.source_entity`, `choir.source_ref`, `choir.web_capture`,
  `choir.media_item`, `choir.audio_recording`, `choir.transcript`,
  `choir.autoradio_run_sheet`.
- [x] Open a verifier thread before merge or settlement.

Acceptance: branch-level object graph foundation merged or a precise blocker
with review evidence. Platform behavior settlement requires the normal
commit/push/CI/deploy/staging loop if runtime behavior changes.

### O2 - Qdrant Derived Index Pipeline

Goal: make Qdrant a rebuildable derived index over object graph data, not a
parallel source of truth.

Checklist:

- [ ] Review the Qdrant prototype for alias-switch correctness.
- [ ] Verify against a real local Qdrant instance.
- [ ] Replace sample objects with object-service-backed inputs or explicitly
  defer that edge.
- [ ] Keep deterministic hash embedding only as a test embedder.
- [ ] Define production embedder/provider boundary without hard-coding role or
  provider assumptions.
- [ ] Record derived-index rebuild and rollback path.
- [ ] Open a verifier thread focused on schema, alias switch, and source-of-truth
  boundaries.

Acceptance: Qdrant build/switch can be tested locally and is documented as
derived/rebuildable. No staging claim without deployed proof.

### O3 - Source Entity Migration

Goal: migrate source truth into durable graph objects so News, Texture, and
Autoradio cite the same substrate.

Checklist:

- [ ] Start with Problem Documentation First if implementation reveals a new
  behavior problem.
- [ ] Get independent review of the existing design.
- [ ] Define source entity identity, citation carry-forward, and unused-source
  handling.
- [ ] Ensure source citation remains tri-state: cited, toolbar-only, unused.
- [ ] Keep Texture canonical writes protected.
- [ ] Add tests that fail on disappearing source entities.
- [ ] Verify that source refs are native objects, not prose links.
- [ ] Open a verifier thread before any red/orange landing claim.

Acceptance: source entity persistence and source refs survive the relevant
Texture/News path with focused tests, plus staging proof if behavior-changing
code lands.

### O4 - News / Universal Wire

Goal: make Universal Wire work as a News benchmark over durable source and
web-capture objects.

Checklist:

- [ ] Implement or wire `choir.web_capture`.
- [ ] Ingest sourcecycled/web/source items into graph objects.
- [ ] Build News/Wire feed from graph objects and source refs.
- [ ] Keep empty feed honest but diagnostic.
- [ ] Add acceptance for authenticated `/api/universal-wire/stories`.
- [ ] Add browser proof that the Universal Wire app renders real story cards.
- [ ] Verify source/citation links open to real source artifacts or Source
  Viewer/reader artifacts.
- [ ] Open independent verifier thread before claiming News benchmark.

Acceptance: on `https://choir.news`, authenticated Universal Wire returns and
renders non-empty, cited, source-grounded stories from durable graph/source
objects. Evidence includes deployed commit identity.

### O5 - Choir-in-Choir Self-Development

Goal: use the News/Universal Wire work as the first real self-development
payload rather than a toy task.

Checklist:

- [ ] Start from product path, not Codex-only edits.
- [ ] Use prompt bar / Texture / super path to create or continue a mission.
- [ ] Produce a reviewable PR, AppChangePackage, or precise blocker.
- [ ] Attach worker/candidate evidence to the artifact context.
- [ ] Attach verifier contract and verdict.
- [ ] Keep owner promotion and rollback boundaries explicit.
- [ ] Do not claim `promotion-level` without AppChangePackage adoption evidence
  plus owner review and promote/rollback evidence.

Acceptance: Choir-in-Choir creates a reviewable platform candidate for the News
path, with evidence strong enough that a separate review thread can evaluate it
without rediscovering the whole problem.

### O6 - Nucleus Capsules

Goal: start the sandboxing substrate after the News/self-development path has
a concrete workload that benefits from better isolation.

Checklist:

- [ ] Land or draft `CapsuleRunner` interface.
- [ ] Add fake runner and persisted result model.
- [ ] Add Nucleus strict-agent backend only after the interface is proven.
- [ ] Keep durable super outside Nucleus.
- [ ] Keep candidate VMs distinct from capsules.
- [ ] Use capsules for scratch/verifier/worker execution, not direct active
  state mutation.
- [ ] Define rollback and audit receipts for capsule execution.

Acceptance: one bounded worker or verifier command can run through a capsule
path with durable result evidence, or the blocker is precise enough to become
the next mission.

### O7 - Choir Base

Goal: begin the Dropbox-like foundation as a local reconciliation kernel, not
a premature File Provider implementation.

Checklist:

- [ ] Implement or refine `internal/base` value model.
- [ ] Model remote/local/synced trees.
- [ ] Use stable item IDs, immutable blobs, and journal-shaped metadata.
- [ ] Add deterministic scenarios for local edit, remote edit, delete/edit,
  move/edit, and conflict.
- [ ] Preserve ContentItem compatibility.
- [ ] Defer macOS File Provider and Wails product claims until the kernel proves
  convergence/conflict behavior.

Acceptance: focused kernel tests prove deterministic convergence/conflict
semantics. No Dropbox-like product claim yet.

### O8 - Autoradio And Pipecat Vertical Slice

Goal: prove Autoradio as a thin vertical slice over the substrate, then add
Pipecat as the realtime audio session layer.

Checklist:

- [ ] Define Autoradio station, queue, beat, and run-sheet object shapes.
- [ ] Generate a run sheet from News/Wire story objects.
- [ ] Render or stub TTS narration with artifact refs.
- [ ] Interleave narration with existing audio/podcast/media items.
- [ ] Store playback state and source refs as durable artifacts.
- [ ] Ingest user speech as recording -> transcript -> artifact -> index.
- [ ] Integrate Pipecat for realtime audio session control.
- [ ] Support spoken interruption that updates the queue and leaves evidence.
- [ ] Keep visual/text representation equivalent to audio behavior.

Acceptance: user can start a station, hear source-grounded material, interrupt
by voice, and see durable artifacts for run sheet, sources, transcript, and
queue update. If Pipecat is not reached overnight, settlement must name the
remaining audio transport gap precisely.

## Parallax State

status: working

mission conjecture: If an orchestration thread uses this checklist to preserve
current WIP, land or hand off the object graph substrate, then advance News,
self-development, Nucleus, Base, and Autoradio in dependency order with worker
and verifier threads, then Choir will materially advance toward an Autoradio
product benchmark without losing evidence or conflating local proxies with
staging/product proof.

deeper goal (G): Make Autoradio the concrete product benchmark for Choir's
self-improving mainframe: source-grounded information, durable artifacts,
self-development, sandboxed work, file-like persistence, and realtime audio
should converge into one usable experience.

witness/spec (A/S): This paradoc, its ledger, thread callbacks, implementation
branches/commits, verifier verdicts, CI/deploy/staging receipts, and any
accepted blockers or successor paradocs produced during the overnight run.

invariants / qualities / domain ramp (I/Q/D): Follow Choir Doctrine and
`AGENTS.md`; preserve authority boundaries; use Problem Documentation First for
new platform behavior problems; do not claim local proof for staging-only
surfaces; never treat same-context review as independent proof; keep Qdrant
derived and rebuildable; keep Pipecat/audio integrated with artifact graph; keep
thread messages resumable; use Codex thread tools for independent worker and
verifier evidence when available. Domain ramp: docs/checkpoint -> branch-level
tests -> local focused proof -> CI/deploy -> staging product acceptance.

variant (ranking function) V: 68 total obligations = 9 WIP-preservation
obligations + 8 object graph obligations + 7 Qdrant obligations + 8
source-entity obligations + 8 News/Universal Wire obligations + 7
self-development obligations + 7 Nucleus obligations + 6 Choir Base obligations
+ 8 Autoradio/Pipecat obligations. Current value: 51. Last Delta V: 8 for
branch-level O1 objectgraph foundation accepted by verifier and merged into
this mission branch. Variant total corrected from 67 to 68 because O0 contains
nine checklist obligations.

budget: overnight run, target 8-12 hours wall-clock, with orchestration
checkpoints at least every major work item and before any behavior-changing
merge. Solvency verdict: feasible only if workers batch bounded constructs and
verifiers run in separate threads; current thread-tool availability removes the
authoring-time capability blocker, but it remains unlikely to complete all
O0-O8 to staging.

authority / bounds: Orchestration may create worker/verifier threads, inspect
worktrees, make docs/checkpoint commits, and land reviewed code through the
repo's normal loop. Behavior-changing work must follow mutation-class
declarations and landing proof. Protected red surfaces require explicit
conjecture delta, protected surfaces, admissible evidence, rollback path, and
heresy delta before editing.

mutation class / protected surfaces: This paradoc creation is green. The
overnight mission will include yellow/orange/red slices: object persistence,
Texture/source refs, Universal Wire routes, Qdrant derived indexes,
self-development/candidate evidence, capsules, Base sync state, and audio
session artifacts. This update is green and only narrows the thread-tool
capability edge.

evidence packet: For each landed behavior-changing slice, record pushed commit
SHA, CI run, deploy status, staging health/build identity, deployed acceptance
command/result, verifier thread id/verdict, rollback refs, mutation class,
protected surfaces, heresy delta, conjecture delta, residual risks, and next
realism axis. For unlanded slices, record branch/stash/worktree handle and
blocker.

heresy delta: initial expected delta is `repaired` for WIP fragility and source
truth fragmentation, `discovered` for new blockers found by workers/verifiers,
and `introduced` only if a reviewer explicitly finds a new doctrine or behavior
regression.

position / live conjectures / open edges: Email is done. Object graph is next.
News depends on source/web objects. Choir-in-Choir should use News as its real
payload. Nucleus follows once there is a concrete worker/verifier isolation
need. Choir Base starts as local reconciliation kernel. Autoradio is the final
product forcing function, but Pipecat is an open implementation edge and not
yet represented in current code. The original authoring thread lacked visible
Codex thread primitives; this update confirms the current Codex app surface can
load `list_projects`, `create_thread`, `read_thread`, `list_threads`,
`send_message_to_thread`, `handoff_thread`, `get_handoff_status`, and thread
title/pin/archive controls. Remaining edge: the overnight runner must still
use actual worker/verifier verdicts as evidence before claiming thread-native
settlement. O0 worker thread: `019f0270-aad3-7001-a6df-d6bc21aec9ab`
(`O0 worker - Autoradio WIP inventory`). O0 verifier thread:
`019f0271-02d9-7391-a564-3ffc2dfce2cd` (`O0 verifier - Autoradio WIP
inventory`). Both were created project-scoped against `/Users/wiz/go-choir`,
titled, and pinned on 2026-06-26. The verifier returned `blocked` because the
worker thread was still `inProgress` and had not emitted its required final O0
report; the verifier also observed that orchestration edits changed the main
worktree dirty status during verification, so the worker must refresh dirty
status before finalizing. After worker refresh, the verifier returned `accept`:
root dirty state matched durable mission docs only, clean Codex/email-freeze
heads were contained in `main`, and the four Cascade branch heads were real
recovery handles not ancestors of `main`. O0 preservation commits created:
Universal Wire diagnosis `a246ab04` on
`preserve/o0-universal-wire-diagnosis-2026-06-26`; source-entity migration
`7a355806` on `preserve/o0-source-entity-migration-2026-06-26`; objectgraph
prototype `b6b45b60` on `preserve/o0-objectgraph-prototype-2026-06-26`;
Qdrant prototype `4c1b28be` on `preserve/o0-qdrant-prototype-2026-06-26`;
PPTX learning/prototype `4a687522` on
`preserve/o0-pptx-learning-2026-06-26`; docs-checker cleanup `238c7ce2` on
`preserve/o0-docs-checker-cleanup-2026-06-26`; orchestration mission state on
`preserve/o0-autoradio-mission-state-2026-06-26`. O0 is complete. O1 worker
thread: `019f0279-b855-7e52-b830-70a8eb4bbfe8` (`O1 worker - Object Graph
Foundation`) in `/Users/wiz/.codex/worktrees/3026/go-choir`. O1 verifier
thread: `019f027a-3434-7ef2-b813-f3f21213167f` (`O1 verifier - Object Graph
Foundation`). The verifier returned `blocked` because the worker had no final
report or diff yet. After worker completion, verifier returned `accept` with no
blocking findings. O1 branch `codex/o1-objectgraph-foundation` produced docs
checkpoint `fa06b718` and implementation `34ece272`; implementation was
cherry-picked into this mission branch as `a68bc801`. Focused objectgraph tests
passed from this branch: `nix develop -c go test ./internal/objectgraph`. O1 is
complete at branch level, with no main/staging/platform settlement claim. O2
worker thread: `019f0285-037b-7a21-b352-ece5b84efeca` (`O2 worker - Qdrant
Derived Index`) in `/Users/wiz/.codex/worktrees/fb93/go-choir`. O2 verifier
thread: `019f0285-e660-7cd1-a468-554e9b175825` (`O2 verifier - Qdrant Derived
Index`). O2 is active but no O2 obligation is complete yet.

next move: Use `read_thread` on the O2 worker until it produces its Qdrant
derived-index decision/implementation report, then use or follow up the O2
verifier for verdict `accept`, `revise_before_continue`, `blocked`, or
`supersede`. Incorporate that verdict before claiming any O2 checklist
progress.

ledger file: `docs/mission-overnight-autoradio-platform-checklist-v0.ledger.md`

version / lineage: v0 created after email-freeze landing. It supersedes the
loose queue ordering from `docs/worktree-review-2026-06-23.md` for overnight
execution order, while preserving that report as evidence.

learning state: Retained here until concrete work lands. Promote outward only
when the mission proves or falsifies thread-native orchestration, object graph
as substrate, News as self-development payload, or Autoradio as forcing
benchmark.

settlement: Exit only as `settled`, `open_handoff`, `blocked`, or
`superseded`. Full settlement requires thread-native orchestration receipts,
independent verifier verdicts, landed code/docs where behavior changed, CI,
deploy identity, and staging/product acceptance for any staging claim. Partial
overnight progress should exit as `open_handoff` with exact remaining V and
next worker/verifier assignment.

## Suggested Goal String

```text
Use Parallax on docs/mission-overnight-autoradio-platform-checklist-v0.md. Treat it as the source program for an overnight, thread-native mission. One orchestration thread owns the checklist, ledger, worker/verifier thread creation, dependency order, worktree hygiene, and evidence synthesis. Current Codex thread tools are available through codex_app after tool discovery: list_projects/create_thread to start bounded project-scoped implementation and verifier threads, send_message_to_thread for follow-ups and explicit callbacks, read_thread/list_threads to reconnect verdicts, handoff_thread/get_handoff_status for ownership-transfer cases, and set_thread_title/set_thread_pinned/set_thread_archived for operator hygiene. If thread tools are not exposed in a later execution environment, record that as a blocker to thread-native settlement and do not treat same-context review as independent proof. Execute in order: O0 preserve WIP, O1 object graph, O2 Qdrant derived index, O3 source entities, O4 News/Universal Wire, O5 Choir-in-Choir self-development, O6 Nucleus capsules, O7 Choir Base, O8 Autoradio/Pipecat vertical slice. Each worker assignment must name mutation class, protected surfaces, admissible evidence, rollback path, heresy delta, callback target, and stop condition. Follow AGENTS.md: Problem Documentation First for new behavior problems; behavior-changing landings require commit, push, CI, deploy identity, staging acceptance, verifier evidence, rollback refs, and residual risks. Update Parallax State in place and append to docs/mission-overnight-autoradio-platform-checklist-v0.ledger.md after each material pass. Exit only as settled, open_handoff, blocked, or superseded, with remaining V and next thread assignment explicit.
```
