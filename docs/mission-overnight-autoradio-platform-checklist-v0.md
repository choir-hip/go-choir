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
surface, as discovered and re-confirmed on 2026-06-26, exposes the needed
thread primitives via `codex_app` after tool discovery. The orchestration
thread is the human-visible conductor for the overnight run. It owns this
checklist, the ledger, worktree hygiene, dependency order, and final evidence
packet. It does not self-certify implementation work.

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

Observed Codex app behavior on 2026-06-26:

- The active orchestration environment can discover the thread tools with
  targeted tool search. Available primitives include `list_projects`,
  `create_thread`, `read_thread`, `list_threads`, `send_message_to_thread`,
  `handoff_thread`, `get_handoff_status`, and thread title/pin/archive
  controls.
- Worktree `create_thread` may return a `pendingWorktreeId` before the actual
  thread id is visible. Treat the pending handle as a launch receipt, not a
  worker identity. Reconnect with `list_threads` by cwd, title/work-item text,
  or the pending handle, then record the resolved thread id, cwd, branch/HEAD,
  and title/pin state once materialized.
- A verifier may correctly return `blocked` if it runs before the worker thread
  or final report exists. If the worker later materializes, record the earlier
  verifier result as stale launch-order evidence and send a follow-up with the
  resolved worker id, cwd, commits, diff/test scope, and non-claims.
- Tool discovery is progressive. A broad thread-tool search exposed
  `list_projects`, `create_thread`, `read_thread`, `list_threads`,
  `send_message_to_thread`, `handoff_thread`, and title/pin/archive controls;
  a targeted handoff search exposed `get_handoff_status`. Future orchestration
  should search for the specific thread primitive before declaring it absent.

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

- [x] Review the Qdrant prototype for alias-switch correctness.
- [x] Verify against a real local Qdrant instance. Nix Qdrant `1.18.1` ran on
  `localhost:6333`; uncached live build/switch/rollback test passed.
- [x] Replace sample objects with object-service-backed inputs or explicitly
  defer that edge.
- [x] Keep deterministic hash embedding only as a test embedder.
- [x] Define production embedder/provider boundary without hard-coding role or
  provider assumptions.
- [x] Record derived-index rebuild and rollback path.
- [x] Open a verifier thread focused on schema, alias switch, and source-of-truth
  boundaries.

Acceptance: Qdrant build/switch can be tested locally and is documented as
derived/rebuildable. No staging claim without deployed proof.

### O3 - Source Entity Migration

Goal: migrate source truth into durable graph objects so News, Texture, and
Autoradio cite the same substrate.

Checklist:

- [ ] Start with Problem Documentation First if implementation reveals a new
  behavior problem.
- [x] Get independent review of the existing design.
- [x] Define source entity identity, citation carry-forward, and unused-source
  handling.
- [x] Ensure source citation remains tri-state: cited, toolbar-only, unused.
- [x] Keep Texture canonical writes protected. O3 Phase 1 store boundary
  accepted: source graph writes happen inside the Texture revision transaction
  before guarded head advancement.
- [x] Add tests that fail on disappearing source entities. Phase 1 covers a
  missing source entity/version rollback before document head advancement;
  Phase 2 adds focused Texture tool producer tests proving source entity graph
  records are shadow-written while legacy revision reads continue to work;
  Phase 3 adds source_ref graph-edge tests and head-stability coverage for
  unresolved graph refs.
- [x] Verify that source refs are native objects, not prose links. Phase 1 adds
  `texture_source_refs` records behind objectgraph-compatible IDs; Phase 3
  makes the selected Texture tool path shadow-write pinned `choir.source_ref`
  records. Phase 4 exposes graph-backed `source_entity_objects` and
  `source_refs` wrapper arrays additively on Texture revision reads; Phase 5
  adapts frontend source-open derivation so graph wrappers feed the existing
  native `source_ref` rendering and `sourceEntityLaunchPayload` path when
  legacy `source_entities` is absent.
- [x] Open a verifier thread before any red/orange landing claim. Phase 1
  verifier thread `019f02b0-47a4-74b2-b78a-44d13bdd958d` returned `accept`;
  Phase 3 verifier thread `019f02d4-80e7-7c73-8085-bc1c52beebf2` returned
  `accept` for branch-level continuation; Phase 4 verifier thread
  `019f02ed-d05e-78f1-975c-1de2df51451b` returned `accept` after a
  revision-list batching repair; Phase 5 verifier thread
  `019f031a-9eb9-7301-9db8-62bbb84e727a` returned `accept` for frontend
  graph-wrapper source derivation.

Acceptance: source entity persistence and source refs survive the relevant
Texture/source-open and News paths with focused tests, plus staging proof if
behavior-changing code lands to the platform. Current branch evidence covers
Texture store/runtime/API/frontend helper slices only; browser/product proof and
the News path remain open.

### O4 - News / Universal Wire

Goal: make Universal Wire work as a News benchmark over durable source and
web-capture objects.

Checklist:

- [x] Implement or wire `choir.web_capture`. O4 Phase 1 adds a typed
  `choir.web_capture.v1` objectgraph metadata contract, validation,
  `Service.CreateWebCapture`, object body storage for extracted text, and
  focused objectgraph tests. O4 Phase 2 adds an accepted branch-level Universal
  Wire fallback projection for existing graph-backed `choir.web_capture`
  objects through `/api/universal-wire/stories`. O4 Phase 3 adds accepted
  branch-level additive source/open identity fields for graph-backed fallback
  cards. O4 Phase 4 adds accepted branch-level frontend/browser proof that
  graph-backed capture cards render source-open controls and route Source
  Viewer/Web Lens through the existing source policy. O4 Phase 5 adds accepted
  branch-level sourcecycled/web/source ingestion into durable `choir.web_capture`
  graph objects with provenance edges; native Texture `source_ref` citation
  carry-forward and staging product proof remain open.
- [x] Ingest sourcecycled/web/source items into graph objects. O4 Phase 5 writes
  eligible sourcecycled `sources.Item` rows into `choir.web_capture` objects,
  creates `choir.source_entity` endpoints and `captured_from` provenance edges,
  and wires `cmd/sourcecycled` to opt-in objectgraph DB paths. This is accepted
  branch-level local code/test proof only; platform/deploy objectgraph DB
  configuration and staging product proof remain open.
- [x] Build News/Wire feed from graph objects and source refs. O4 Phase 7 adds
  accepted and incorporated branch-level Universal Wire graph fallback
  enrichment that reads `captured_from` edges from `choir.web_capture` objects
  to graph `choir.source_entity` provenance objects and exposes those source
  entities as Wire manifest context. It does not claim native Texture body
  `source_ref` citations, publication/export, deployed source artifacts,
  staging, or full News benchmark acceptance.
- [x] Keep empty feed honest but diagnostic. O4 Phase 8 adds accepted and
  incorporated branch-level empty-only Universal Wire diagnostics plus UI
  rendering. Empty responses stay empty and may include safe substrate
  diagnostic states; non-empty Texture/graph responses omit diagnostics. This
  does not claim staging, provider/search, Qdrant, publication/export,
  run-acceptance, promotion/rollback, or native Texture `source_ref`
  behavior.
- [x] Add acceptance for authenticated `/api/universal-wire/stories`. O4 Phase 6
  adds accepted branch-level authenticated public-route API proof that
  sourcecycled writes graph-backed `choir.web_capture` objects to the
  runtime-derived objectgraph DB path and a runtime opened on the same store path
  reads them through `GET /api/universal-wire/stories`. This does not prove
  staging/deploy/runtime daemon configuration.
- [x] Add browser proof that the Universal Wire app renders real story cards.
  O4 Phase 4 proves a graph-backed capture story card through the public
  `/api/universal-wire/stories` route mock in the authenticated desktop shell,
  with Source Viewer default and explicit Web Lens source opening.
- [ ] Verify source/citation links open to real source artifacts or Source
  Viewer/reader artifacts. O4 Phase 4 proves frontend routing for graph-backed
  fallback cards. O4 Phase 9 adds accepted and incorporated branch-level API/UI
  proof that graph-backed capture source handles carry durable reader snapshots
  and open Source Viewer/reader artifact text by default while Web Lens remains
  an explicit live/original action. Native Texture `source_ref` citation
  carry-forward and deployed/live source artifact proof remain open.
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
+ 8 Autoradio/Pipecat obligations. Current value: 31. Last Delta V: 0 for O4
Phase 9 accepted branch-level graph-capture Source Viewer proof because native
Texture citation opening and deployed source artifact proof remain open; the
last decrement was 1 for O4 Phase 8 verifier acceptance and root incorporation.
O4 Phase 1 closed the first O4 checklist obligation by adding a tested
`choir.web_capture` objectgraph
foundation. O4 Phase 2 adds an accepted branch-level fallback projection from
graph-backed web captures into `/api/universal-wire/stories`, but it does not
close the broader News/Wire feed-from-graph-and-source-refs obligation because
native Texture `source_ref` citation carry-forward, staging, deploy, and product
acceptance remain open.
O4 Phase 3 adds an accepted and incorporated additive DTO source/open identity
slice for graph capture cards; it does not claim native Texture `source_ref`,
frontend opening, or staging proof. O4 Phase 4 adds accepted and incorporated
frontend/browser proof that graph-backed capture cards render source controls
and route Source Viewer/Web Lens through existing source policy; it does not
claim live ingestion, native Texture citation carry-forward, publication/export,
or staging proof. O4 Phase 5 adds accepted and incorporated sourcecycled
ingestion into durable `choir.web_capture` objects with source entity provenance
edges; it does not claim platform/deploy objectgraph DB configuration, staging,
native Texture citation carry-forward, publication/export, or Qdrant.
O4 Phase 7 adds accepted and incorporated graph `captured_from`
`choir.source_entity` provenance carry-forward into Universal Wire manifest
context; it does not claim native Texture body `source_ref` citations,
publication/export, staging, or full News benchmark acceptance.
O4 Phase 8 adds accepted and incorporated empty-only diagnostics for Universal
Wire responses plus UI rendering; it does not claim staging, provider/search,
Qdrant, publication/export, run-acceptance, promotion/rollback, or native
Texture `source_ref` behavior.
O4 Phase 9 adds accepted and incorporated branch-level graph-capture Source
Viewer reader-snapshot proof: checkpoint `42d47423`, implementation
`fcde783a`, and docs-only provenance repair `f7e8fced` were accepted by
verifier thread `019f03f2-bb27-7d80-90a1-e172558b9c61`; root incorporated the
checkpoint as `afe8e70d` and the implementation as `9ac7d6c2`. The repair was
empty in root because the root ledger already distinguished the worker thread
from the source/orchestration thread. O4 Phase 9 does not close root V because
native Texture body `source_ref` citation opening and deployed/live source
artifact proof remain open, and it does not claim staging, publication/export,
Qdrant, provider/search, promotion/rollback, or run acceptance.
Variant total corrected from 67
to 68 because O0 contains nine checklist obligations.

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
session artifacts. The current O4 Phase 9 incorporated change is orange: it
additively exposes bounded reader snapshots on Universal Wire source items and
passes them to existing Source Viewer/reader UI behavior while preserving the
stated boundary against synthesized stories/source refs/source entities,
staging, publication/export, Qdrant, auth/session, provider/gateway, promotion,
rollback, run acceptance, and native Texture citation claims.

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

position / live conjectures / open edges: Email and O1 objectgraph foundation are
done at branch level. O2 branch `codex/o2-qdrant-derived-index` in
`/Users/wiz/.codex/worktrees/fb93/go-choir` produced implementation commit
`d90d8a84`, incorporated into this orchestration branch as `b02d43d5` after
docs checkpoint `dae88f60`. Qdrant
prototype `4c1b28be` was reviewed: its `update_alias` action shape is not
accepted for O2. The implementation switches/rolls back with one alias
transaction containing delete/create alias actions, keeps Qdrant derived from
objectgraph objects, and keeps deterministic hash embedding test-only. O2
verifier thread `019f0285-e660-7cd1-a468-554e9b175825` returned `accept` for
branch-level continuation. Real local Qdrant verification was later discharged
with Nix Qdrant `1.18.1` on `localhost:6333`; the uncached live
build/switch/rollback test passed. News depends on source/web
objects. Choir-in-Choir should use News as its real
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
Index`). The verifier first returned `blocked` because the worker turn was
still `inProgress` and no final Qdrant report/diff/tests existed yet. After
worker completion, the verifier returned `accept` for branch-level
continuation, with the live local-Qdrant proof still open. That proof passed
after starting Nix Qdrant `1.18.1` with `/tmp/choir-qdrant-o2-proof` storage.
The same verifier returned `accept` on the final O2 completion readback. O2 is
complete at branch level, with no main/staging/platform settlement claim. O3
Phase 1 worker thread `019f02af-74d3-73a0-ae15-cf0809739b3b` completed in
`/Users/wiz/.codex/worktrees/a870/go-choir` and reported clean detached HEAD at
`017b4113`. The worker first created docs checkpoint `7623b5f1` choosing
Texture-store source tables behind an objectgraph-compatible contract, then
implementation commit `017b4113` adding `texture_source_entities`,
`texture_source_refs`, `CreateRevisionWithSourceGraph`, canonical ID/version
helpers, and focused tests. Those commits were incorporated into this
orchestration branch as `7e6874a9` and `3adcd0ae`. The worker resolved the
accepted P2 by using a single URL-safe `objectgraph.StableSuffixFromKey(...)`
suffix with no extra colon-separated components. O3 verifier thread
`019f02b0-47a4-74b2-b78a-44d13bdd958d` returned `accept`: no blocking findings,
checkpoint-before-code satisfied, transaction/head invariant preserved,
focused Phase 1 tests passed, `git diff --check` passed, full
`internal/store` passed, and `internal/objectgraph` passed. Root incorporation
checks also passed on this branch:
`nix develop -c go test ./internal/store -run 'TestTextureSourceGraphCanonicalIDsUseSingleURLSafeSuffix|TestCreateRevisionWithSourceGraphPersistsPinnedSourceRecords|TestCreateRevisionWithSourceGraphFailureDoesNotAdvanceDocumentHead' -count=1`,
`nix develop -c go test ./internal/objectgraph -count=1`, and
`nix develop -c go test ./internal/store -count=1`. Evidence boundary:
branch-level code/test/verifier acceptance only; no API producer path,
frontend/source-open, Qdrant projection, main, staging, product, deployment, or
landing claim. O3 Phase 2 was launched through current Codex thread tools as a
bounded shadow-write producer slice. Worker setup returned pending worktree
handle `local:c6b79ff4-1a9f-491c-81e5-ea1cdc44df60`, which resolved to worker
thread `019f02c4-6b34-70d1-a268-5bd7ccc4d489` (`O3 worker - Source Entity
Phase 2 shadow-write`) in `/Users/wiz/.codex/worktrees/fcf1/go-choir` on
branch `codex/o3-phase2-shadow-write-producer`. The worker created docs
checkpoint `caf5b737 checkpoint O3 phase2 shadow-write producer`, choosing the
`edit_texture` appagent tool path through `commitTextureToolEdit`. The worker
then completed implementation commit `32a5d338 implement O3 phase2 texture
tool source shadow writes`: `patch_texture` / `rewrite_texture` now call
`CreateRevisionWithSourceGraph` in shadow-write mode for `choir.source_entity`
records derived from structured `SourceEntities`, while legacy revision reads
and DTOs still use `texture_revisions.source_entities_json`. Worker-reported
tests passed: focused runtime source-graph/legacy compatibility tests, focused
Phase 1 store boundary tests, `internal/runtime -run TestTextureTool`, full
`internal/store`, and `git diff --check`; worker worktree was clean. Verifier
thread `019f02c4-a8c3-78e2-b3d6-e08e45ba8fda` (`O3 verifier - Source Entity
Phase 2`) returned `accept` with no blocking findings and reran the same
focused/broader checks. Worker commits were incorporated into this
orchestration branch as `fb876caa` and `5d349eaf`; root checks passed:
focused runtime source-graph/legacy compatibility tests, focused Phase 1 store
boundary tests, `internal/runtime -run TestTextureTool`, full
`internal/store`, and `git diff --check`. Mutation class is orange/red-adjacent
with protected surfaces: Texture canonical writes, source identity/ref edges,
legacy DTO compatibility, source-open routing, Qdrant source-of-truth
boundaries, auth/session renewal, gateway/provider calls, and staging/deploy
claims. O3 Phase 3 completed through worker thread
`019f02d4-4877-7f82-89bd-ac87addc7bb3` and verifier thread
`019f02d4-80e7-7c73-8085-bc1c52beebf2`. Worker commits `b0ad6de1` and
`98e77766` were accepted and incorporated into this branch as `22829e24` and
`f8769358`. The selected Texture tool path now resolves body
`source_ref.attrs.source_entity_id` against graph `choir.source_entity` records
derived from the same materialized `SourceEntities` array, writes pinned
`choir.source_ref` records, and fails unresolved graph refs before document head
advancement. Verifier verdict: `accept`, no blocking findings. Residual risk:
the duplicate-normalization repair at `internal/runtime/tools_texture.go` lacks
a dedicated two-legacy-IDs regression test. Root checks passed:
focused Phase 3/Texture runtime tests, focused Phase 1 store boundary tests,
full `internal/store`, and `git diff --check`. Evidence class is branch-level
code/test/verifier acceptance only; no O3-complete, main, staging, product,
deployment, public producer, source-open, Qdrant, graph-first read, auth,
gateway/provider, or deploy claim exists yet.

O3 Phase 4 worker thread `019f02ed-7ce9-7d30-906b-f497a95ecc6d`
(`O3 worker - Source API Phase 4`) completed cleanly in
`/Users/wiz/.codex/worktrees/ba60/go-choir`. Worker docs checkpoint
`cc0de09e` chose the additive Texture API read shape:
keep legacy revision `source_entities` unchanged and add explicit
`source_entity_objects` plus `source_refs` graph wrapper arrays to Texture
revision responses when graph records exist. Worker implementation commit
`9ab4a810` adds revision-scoped graph reads, enriches existing Texture revision
responses with those wrapper arrays, preserves legacy fields, and repairs the
Phase 3 residual duplicate-normalization risk with a focused two-legacy-ID
regression test. Worker evidence commit `b74f5a87` records exact test evidence
and leaves the worktree clean.

Worker-reported checks passed:
`nix develop -c go test ./internal/runtime -run 'TestTextureToolSourceGraphDuplicateLegacyIDsResolveToSharedGraphEntity|TestTextureToolCommitWritesStructuredRevisionAndRejectsStaleBase' -count=1`;
`nix develop -c go test ./internal/store -run 'TestTextureSourceGraphCanonicalIDsUseSingleURLSafeSuffix|TestCreateRevisionWithSourceGraphPersistsPinnedSourceRecords|TestCreateRevisionWithSourceGraphFailureDoesNotAdvanceDocumentHead' -count=1`;
`nix develop -c go test ./internal/runtime -run 'TestTextureTool' -count=1`;
`nix develop -c go test ./internal/store -count=1`; and `git diff --check`.

Verifier thread `019f02ed-d05e-78f1-975c-1de2df51451b`
(`O3 verifier - Source API Phase 4`) returned `revise`. Rerun checks passed:
the focused runtime duplicate/API check, focused store graph checks,
`internal/runtime -run TestTextureTool`, full `internal/store`, and
`git diff --check`. The blocking finding is [P2]: the Phase 4 candidate enriches
revision listing one revision at a time, and each enrichment calls a graph read
path that queries refs and then scans all owner source entities. At
`limit=10000`, an existing revision-list read becomes repeated graph queries
plus repeated owner-wide source scans.

Worker thread `019f02ed-7ce9-7d30-906b-f497a95ecc6d` repaired the revise
finding in code-only commit `f9a23cea batch texture source graph wrappers for
revision lists`. Revision-list responses now batch source graph wrapper reads
once per list via `ListTextureSourceGraphForRevisions`; single-revision reads
keep the existing helper. Legacy `source_entities` remains unchanged, and
`source_entity_objects` plus `source_refs` remain additive. Worker-reported
checks passed: focused store batch/graph tests, focused runtime duplicate/API
tests, `internal/runtime -run TestTextureTool`, full `internal/store`, and
`git diff --check`. Residual risk: the batch helper still scans owner source
entities once per revision-list response to preserve entity-only shadow-write
wrappers; it no longer repeats that scan per listed revision.

Verifier thread `019f02ed-d05e-78f1-975c-1de2df51451b` accepted the repaired
Phase 4 candidate. Accepted worker commits were incorporated into this
orchestration branch as `3eddef63 expose texture source graph wrappers in
revision APIs` and `03346092 batch texture source graph wrappers for revision
lists`. Root checks passed: focused store batch/graph tests, focused runtime
duplicate/API tests, `internal/runtime -run TestTextureTool`, full
`internal/store`, and `git diff --check`. Evidence class remains branch-level
code/test/verifier acceptance only; no O3-complete, main, staging,
source-open/frontend behavior, Qdrant projection, publication/export,
graph-first enforcement, auth/session, gateway/provider, promotion, deploy, or
rollback proof exists.

O3 Phase 5 source-open/frontend worker has materialized. Worker pending
worktree handle `local:e1f57d79-acef-4354-9dcf-5fd39bb28ec0` resolved to
thread `019f031a-6008-7c42-a36a-cc3ffebe707c` (`O3 worker - Source Open
Phase 5`) in `/Users/wiz/.codex/worktrees/1050/go-choir`. Verifier thread
`019f031a-9eb9-7301-9db8-62bbb84e727a` (`O3 verifier - Source Open Phase 5`)
returned `blocked` before the worker materialized; treat that as stale
launch-order evidence, not a Phase 5 rejection. The bounded slice is
`O3-phase5-source-open-frontend-wrappers`: adapt frontend source-open derivation
so Texture revisions can consume graph-backed `source_entity_objects` and
`source_refs` when legacy `source_entities` is absent, while preserving
publication bundle priority, legacy `source_entities` fallback, and the rule
that legacy `metadata.media_source_refs` are not synthesized into source
entities. Excluded surfaces remain O4 News/Universal Wire, Qdrant projection,
publication/export, auth/session renewal, gateway/provider calls, staging/deploy,
graph-first enforcement, promotion, and rollback behavior.

Worker thread `019f031a-6008-7c42-a36a-cc3ffebe707c` finished Phase 5 with
commit `927d58a68bc36ca8a4d2e82066c8961f60b5587d` (`derive texture sources
from graph wrappers`). The chosen mapping keeps `revisionSourceEntities`
priority as publication bundle sources, then legacy revision `source_entities`,
then graph-backed `source_entity_objects`. Wrapper records are converted into
the existing local entity shape consumed by `sourceEntityID`,
`sourceEntityOpenPlan`, and `sourceEntityLaunchPayload`; `source_refs` are used
only to preserve body-level legacy `source_ref` ids when multiple refs point at
the same graph entity version. Legacy `metadata.media_source_refs` still do not
synthesize source entities. Worker checks passed:
`npx playwright test tests/texture-source-entities.spec.js -g "revisions do not synthesize source entities from legacy media refs|revision source entities"`;
`npm run build` with unrelated existing Svelte/a11y/chunk warnings only; and
`git diff --check HEAD~1..HEAD`. Worker tracked hygiene was clean; ignored
artifacts remained `frontend/node_modules/` from `npm ci` and `frontend/dist/`
from `npm run build`.

Verifier thread `019f031a-9eb9-7301-9db8-62bbb84e727a` returned `accept` for
Phase 5. The accepted worker commit was incorporated into this orchestration
branch as `0189d59a derive texture sources from graph wrappers`. Root checks
passed:
`npx playwright test tests/texture-source-entities.spec.js -g "revisions do not synthesize source entities from legacy media refs|revision source entities"`
from `frontend/` (5 tests);
`npm run build` from `frontend/` with unrelated existing Svelte/a11y/chunk
warnings only; and `git diff --check HEAD~1..HEAD`. Root proof artifacts
`frontend/test-results/` and `frontend/dist/` were removed after validation, and
the tracked worktree was clean.

O3 Phase 6 source-open/browser-product proof is accepted and incorporated at
branch level. Pending worktree handle
`local:c0f12b0c-2845-46eb-bb84-8f135082ec9c` resolved to thread
`019f032c-7960-7563-8b75-c8a681a388f8` (`O3 worker - Source Open Phase 6`) in
`/Users/wiz/.codex/worktrees/5e10/go-choir`; the thread is pinned. Worker
branch `codex/o3-phase6-source-open-browser-product-proof` contains commit
`65a08d44 test O3 phase6 graph wrapper source open path`, changing only
`frontend/tests/texture-source-entities.spec.js`. The worker proof constructs a
Texture revision through public Texture APIs, intercepts only
`GET /api/texture/revisions/{id}` into graph-only `source_entity_objects` plus
`source_refs` with no legacy `source_entities`, then verifies native
`source_ref` rendering plus Source Viewer default routing and explicit Web Lens
routing through the UI. Worker-reported commands passed: the single new focused
browser proof with `--timeout=120000`, the adjacent Phase 5 regression filter
plus the new test (6 tests), and `git diff --check`; `npm run build` was not run
because the change is test-only. Tracked worker hygiene is clean; ignored
`frontend/node_modules/` and service logs remain confined to the worker
worktree. Independent verifier thread
`019f0343-df0b-7442-8d2e-7714b3fd3988` (`O3 verifier - Source Open Phase 6`)
returned `accept` with no blocking findings after inspecting the diff, rerunning
the exact focused Phase 6 Playwright proof, rerunning the 6-test regression
filter, and checking tracked hygiene. The accepted worker commit was
incorporated into this orchestration branch as `9eeb5115 test O3 phase6 graph
wrapper source open path`. Root checks passed:
`npx playwright test tests/texture-source-entities.spec.js -g "Texture renders and opens graph-wrapper sources when legacy revision source entities are absent" --timeout=120000`;
`npx playwright test tests/texture-source-entities.spec.js -g "revisions do not synthesize source entities from legacy media refs|revision source entities|Texture renders and opens graph-wrapper sources when legacy revision source entities are absent" --timeout=120000`;
and `git diff --check HEAD~1..HEAD`. Root generated proof outputs
`frontend/test-results/` and `frontend/playwright/` were removed; ignored
`frontend/node_modules/` and `frontend/frontend.log` remain as local
dependency/log artifacts. Evidence class is local branch-level
test/verifier/root-rerun acceptance only. No O3-complete, main, staging,
product acceptance, deploy, backend graph-wrapper production, Qdrant
projection, publication/export, auth/session, gateway/provider, graph-first
enforcement, promotion, or rollback claim exists.

O4 Phase 1 web-capture foundation is accepted and incorporated at branch level.
Pending worktree handle
`local:3a8578f8-9c76-4572-bca1-2c3b2d02b638` resolved to thread
`019f034d-ebc1-75a3-9c4b-269e8b9d6be7`
(`O4 worker - Web Capture Object Foundation`) in
`/Users/wiz/.codex/worktrees/b850/go-choir`; the thread is titled and pinned.
Worker branch `codex/o4-phase1-web-capture-object-foundation` contains
checkpoint commit `ae0fb49f checkpoint O4 web capture foundation gap` and
implementation commit `7e9418af add web capture objectgraph foundation`.
Worker-reported implementation adds a typed `choir.web_capture.v1` metadata
contract, validation, `Service.CreateWebCapture`, extracted-text object body
storage, deterministic identity tests, required-field/URL validation tests,
SQLite durability tests, and `captured_from` edge persistence tests. Worker
checks passed: `nix develop -c go test ./internal/objectgraph`,
`git diff --check`, and clean tracked status. Independent verifier thread
`019f0353-95c0-7020-8047-2e7d6fab7e66`
(`O4 verifier - Web Capture Object Foundation`) has been launched against
worker commits `ae0fb49f` and `7e9418af`, titled, and pinned. The verifier
returned `revise_before_continue`: focused `internal/objectgraph` tests passed,
the implementation commit passed `git show --check`, and no code-level blocker
was found, but `git diff --check 68cfb026..7e9418af` and
`git show --check ae0fb49f` fail because
`docs/o4-web-capture-foundation-checkpoint-2026-06-26.md` has a new blank line
at EOF. Evidence remains worker-local until repair, independent verifier
acceptance, and root incorporation.

Worker thread `019f034d-ebc1-75a3-9c4b-269e8b9d6be7` repaired the verifier
finding with commit `b79251db fix O4 checkpoint trailing blank line`, making
worker branch `codex/o4-phase1-web-capture-object-foundation` HEAD
`b79251db69d22b00d69676187ff6f989ec7fcc1c`. Worker-reported repair checks
passed: `git diff --check 68cfb026..HEAD`, `git show --check HEAD`,
`nix develop -c go test ./internal/objectgraph`, and `git status --short --ignored`
with no output. The same verifier thread
`019f0353-95c0-7020-8047-2e7d6fab7e66` re-reviewed the repaired head and
returned `accept` with no findings. The verifier reran
`nix develop -c go test ./internal/objectgraph`, `git diff --check
68cfb026..HEAD`, `git show --check b79251db`, checked the candidate file list,
and confirmed no unintended runtime, proxy, store, Texture, sourcecycled,
sandbox, or frontend path changes. Accepted worker commits were incorporated
into this orchestration branch as `cc031a79 checkpoint O4 web capture
foundation gap`, `a77fd21d add web capture objectgraph foundation`, and
`99f68b56 fix O4 checkpoint trailing blank line`. Root checks passed:
`git diff --check 68cfb026..HEAD`, `git show --check --oneline HEAD`, and
`nix develop -c go test ./internal/objectgraph`. Tracked root status is clean;
ignored local env/log/dependency artifacts remain unrelated. Evidence class is
branch-level local test/verifier/root-rerun acceptance only. No Universal Wire
feed proof, sourcecycled ingestion, Qdrant, main, staging, product acceptance,
deploy, publication/export, auth/session, gateway/provider, promotion, or
rollback claim exists. O4 Phase 2 worker thread
`019f035c-2a13-7f20-abd9-960b9866189b` (`O4 worker - Universal Wire Web
Capture Read`) resolved from pending worktree handle
`local:6462c8b4-ca0f-4c42-bdc5-ad578dda6f15` in
`/Users/wiz/.codex/worktrees/5f31/go-choir`; it was titled and pinned on
2026-06-26. Early worker trace reports a runtime-owned objectgraph service gap
and is following Problem Documentation First before any route behavior change.
No O4 Phase 2 final report, verifier, root incorporation, or acceptance exists
yet. The worker later completed on branch
`codex/o4-phase2-universal-wire-web-capture-read` at
`77b3f251c8e41b552efa41a577e81fa10baab7d9`, after checkpoint commit
`b264e8e766c1f1accb1578aa76a0dbf92aabf5ea`. The worker-reported diff adds
`docs/o4-universal-wire-web-capture-read-checkpoint-2026-06-26.md`,
`internal/runtime/objectgraph_runtime.go`, and updates
`internal/runtime/runtime.go`, `internal/runtime/test_helpers_test.go`,
`internal/runtime/universal_wire.go`, and
`internal/runtime/universal_wire_test.go`. Worker-reported checks passed:
`nix develop -c go test ./internal/runtime -run 'TestHandleUniversalWireStories'`,
`nix develop -c go test ./internal/objectgraph`,
`git diff --check f3272233..HEAD`, and `git show --check --oneline HEAD` plus
`HEAD~1`. Worker tracked/ignored status was clean/no output. Evidence boundary
is worker-local branch-level focused tests only. Verifier thread
`019f0364-d34d-7270-bcb9-ebefb5cb2ade` (`O4 verifier - Universal Wire Web
Capture Read`) returned `accept` with no blocking findings. The verifier
confirmed checkpoint-before-code, the narrow runtime-owned objectgraph service
boundary, Texture-edition priority, honest empty-state/fallback behavior, and
focused tests. Accepted worker commits were incorporated into this orchestration
branch as `4d8b0f95 checkpoint O4 web capture read gap` and `b3d4f646 add
Universal Wire web capture read path`. Root checks passed:
`git diff --check d6f0b389..HEAD`; `git show --check --oneline 4d8b0f95`;
`git show --check --oneline b3d4f646`;
`nix develop -c go test ./internal/runtime -run 'TestHandleUniversalWireStories'`;
and `nix develop -c go test ./internal/objectgraph`. The root runtime test
emitted a non-fatal Nix eval-cache SQLite busy warning while Go returned `ok`.
Tracked root status is clean; ignored local env/log/dependency artifacts remain
unrelated.

next move: choose the next O4 source/citation move from the now-observed
native Texture browser harness blocker. O4 Phase 10b replacement worker thread
`019f0405-4fea-70f1-b248-5b6ebce70775` (`O4 worker - Native Texture Citation
Proof Replacement`) in `/Users/wiz/.codex/worktrees/013f/go-choir` returned no
candidate and left a clean worktree at
`24382118 record O4 native citation worker no-candidate`. It identified a
plausible test-only tightening for native body `source_ref` reader-artifact
proofs, but reverted it because the focused Playwright harness could not be run
locally: full `start-services.sh` failed at platformd/Dolt state, the
`CHOIR_ENABLE_PLATFORMD=0` harness failed frontend startup under pnpm/esbuild
build-script policy, and preview-only/auth setup failed without the full
auth/proxy stack. The next bounded route should either document this browser
harness blocker first or launch a harness-focused worker to make the existing
Texture source-ref Playwright proof runnable without changing product behavior.
O4 Phase 10 worker thread
`019f03ff-f119-75d3-8bf2-ae3f50af3ab4` (`O4 worker - Native Texture Citation
Source Open`) resolved in `/Users/wiz/.codex/worktrees/d3ed/go-choir` on branch
`codex/o4-phase10-native-texture-source-ref-open-proof`, but returned no
candidate and left the worktree clean at its starting head
`7b94d220 record O4 source artifact acceptance`. O4 Phase 9 worker thread
`019f03e9-8fe1-7503-a9a2-f55ee5430c54` completed in
`/Users/wiz/.codex/worktrees/199d/go-choir`; verifier thread
`019f03f2-bb27-7d80-90a1-e172558b9c61` accepted the repaired head
`f7e8fced`. Root incorporated the checkpoint as `afe8e70d` and implementation
as `9ac7d6c2`; the repair commit was empty in root after conflict resolution
because root documentation already carried corrected provenance. Root checks
passed: `git show --check --oneline afe8e70d`;
`git show --check --oneline 9ac7d6c2`; `git diff --check 617a0a45..HEAD`;
`nix develop -c go test ./internal/runtime -run
'^TestHandleUniversalWireStories' -count=1 -timeout=120s`; `npm run build`;
and focused `PLAYWRIGHT_BASE_URL=http://127.0.0.1:4173 npx playwright test
tests/universal-wire-app.spec.js -g 'Universal Wire opens graph capture sources
through Source Viewer by default and Web Lens explicitly' --timeout=120000`.
Actual Delta V: 0; current V remains 31. O4 Phase 8 worker thread
`019f03d8-2a15-7a61-ab7f-82ea0213cce2` and verifier thread
`019f03e1-5342-7b61-a557-917c1ef1c407` accepted worker commits `4975163f` and
`cbf04485`, which root incorporated as `db46f8fe checkpoint O4 empty feed
diagnostics gap` and `f510386b add Universal Wire empty feed diagnostics`.
Root checks passed: `git show --check --oneline db46f8fe`;
`git show --check --oneline f510386b`; `git diff --check 49b363cc..HEAD`;
`nix develop -c go test ./internal/runtime -run
'^TestHandleUniversalWireStories' -count=1 -timeout=120s`; `npm run build`;
and focused `npx playwright test tests/universal-wire-app.spec.js -g
'Universal Wire renders empty feed diagnostics without synthetic stories'
--timeout=120000`. Evidence remains branch-local/root-rerun only, not staging
or full News benchmark proof. O4 Phase 7
worker thread
`019f03c9-2c8f-73b1-bfca-ed7badd4383f` (`O4 worker - Graph Source-Ref Feed`)
and verifier thread `019f03d1-0071-7371-bdd6-a3bd840c9e76` (`O4 verifier -
Graph Source-Ref Feed`) accepted worker commits `35420443` and `8a0a69d1`,
which root incorporated as `24f48768 checkpoint O4 graph source-ref feed gap`
and `62503e67 carry Wire graph source entity provenance`. Root checks passed:
`git show --check --oneline 24f48768`; `git show --check --oneline 62503e67`;
`git diff --check 4f67aaf9..HEAD`; `nix develop -c go test ./internal/runtime
-run '^TestHandleUniversalWireStories' -count=1 -timeout=120s`; `nix develop -c
go test ./internal/cycle -run
'^TestWriteWebCaptureGraphObjectsProjectsSourceItems$' -count=1 -timeout=60s`;
and `nix develop -c go test ./cmd/sourcecycled -run
'^TestRunCycleWritesSourceItemsToObjectGraphWebCaptures$' -count=1
-timeout=60s`. Expected Delta V: 1; Actual Delta V: 1. Current V is 32. Do not
claim full News benchmark, staging, native Texture `source_ref`
publication/export, Qdrant, promotion, rollback, or run acceptance from this
branch-level proof. O4 Phase 6 worker thread
`019f03b9-7d73-7d13-9d58-4bec2361f5c8` (`O4 worker - Authenticated Wire API
Proof`) in `/Users/wiz/.codex/worktrees/f0b3/go-choir` completed. Orchestration
observed the worker branch dirty with one focused test change in
`internal/runtime/universal_wire_test.go` and a long-running
`go test ./cmd/sourcecycled -run Test.*ObjectGraph|Test.*RuntimeStore|Test.*WebCapture -count=1`
process, then sent a bounded follow-up asking the worker to classify progress
or hang and finalize honestly. The worker replied that the long-running command
completed and that the runtime-package test placement is invalid because it
creates a Go import-cycle boundary. A later read-only check found no active test
process but still only the invalid runtime test dirty, so orchestration sent a
second steering prompt requiring removal of that failed edit before finalizing,
with either a valid relocated `cmd/sourcecycled` proof commit or a precise
blocker/no-candidate report. The worker then removed the invalid runtime edit,
relocated the proof into `cmd/sourcecycled/main_test.go`, and produced candidate
commit `e406ca23 test O4 sourcecycled Wire API graph path`; external checks
show a single test-file change, `git show --check --oneline e406ca23` passes,
and the worker worktree is clean. A final-report prompt was sent after the
commit. The worker final report confirms branch
`codex/o4-phase6-authenticated-universal-wire-product-api-proof` at
`e406ca23`, changed file `cmd/sourcecycled/main_test.go`, clean worker
worktree status, passed focused sourcecycled/runtime/cycle tests, and
non-claims for staging/deploy/auth renewal/vmctl/provider/Qdrant/Texture
canonical writes/publication/export/promotion/rollback/run acceptance.
Independent verifier worktree handle
`local:fda573a5-c918-4c70-9b9e-4f4e6b843960` resolved to verifier thread
`019f03c2-88b6-7481-b570-79190baeeb0b` (`O4 verifier - Authenticated Wire API
Proof`) in `/Users/wiz/.codex/worktrees/d9c6/go-choir`; the verifier returned
`accept` with no findings and confirmed the candidate is a one-file test-only
proof using registered public runtime routes and authenticated
`X-Authenticated-User`, with no internal/test-only route seeding. Accepted
worker commit `e406ca23` was incorporated into root as `6dec06b4 test O4
sourcecycled Wire API graph path`. Root checks passed:
`git diff --check 413d97c3..HEAD`; `git show --check --oneline 6dec06b4`;
`nix develop -c go test ./cmd/sourcecycled -run '^TestRunCycleWritesSourceItemsToObjectGraphWebCaptures$' -count=1 -timeout=60s`;
`nix develop -c go test ./internal/runtime -run 'TestHandleUniversalWireStories(FallsBackToGraphBackedWebCaptures|RequiresAuth)$' -count=1 -timeout=60s`;
and `nix develop -c go test ./internal/cycle -run '^TestWriteWebCaptureGraphObjectsProjectsSourceItems$' -count=1 -timeout=60s`.
The worker replaces pending handle
`local:b9a89dc6-e09f-4eec-8617-7706221de218` for orchestration purposes. The
assignment is an authenticated Universal Wire product-API proof slice: show,
through product-visible evidence if feasible, that configured sourcecycled
objectgraph storage and `/api/universal-wire/stories` work together, or document
the precise config/deploy blocker and the narrowest durable improvement.
Expected Delta V: 1 for authenticated API acceptance after verifier acceptance
and root incorporation; actual Delta V: 1. Do not claim News benchmark,
staging, native Texture `source_ref` carry-forward, publication/export, Qdrant,
promotion, rollback, or run acceptance from this branch-level proof. O4 Phase 5 verifier thread
`019f03b0-6a16-79b0-888d-b8a48e6a378f` (`O4 verifier - Web Capture
Ingestion`) returned `accept` with no blocking findings. The verifier confirmed
checkpoint-before-code, narrow sourcecycled/objectgraph boundaries, provenance
coherence through source entities and `captured_from` edges, opt-in
`cmd/sourcecycled` objectgraph DB wiring, preserved Universal Wire fallback
semantics, and clean candidate worktree status. Accepted worker commits were
incorporated into this orchestration branch as `ca639a9e checkpoint O4
sourcecycled web capture ingestion` and `632919ab write sourcecycled web
captures to objectgraph`. Root checks passed: `git diff --check 76d21413..HEAD`;
`git show --check --oneline ca639a9e`; `git show --check --oneline 632919ab`;
`nix develop -c go test ./internal/objectgraph -count=1`;
`nix develop -c go test ./cmd/sourcecycled -count=1`;
`nix develop -c go test ./internal/cycle -count=1`; and
`nix develop -c go test ./internal/runtime -run 'TestHandleUniversalWireStories' -count=1`.
Root tracked status is clean; ignored local env/log/dependency artifacts remain
unrelated. Earlier pending verifier handle
`local:2f4d614e-19ab-4a0a-9b88-b1a688bda10c` did not resolve in
`list_threads` and is superseded for orchestration by the readable replacement
verifier. O4 Phase 5 worker thread
`019f039f-9dd6-7881-a4ec-8607c9a4bb34` (`O4 worker - Web Capture Ingestion`)
completed on branch
`codex/o4-phase5-sourcecycled-web-capture-ingestion-replacement` in
`/Users/wiz/.codex/worktrees/o4-phase5-sourcecycled-web-capture-ingestion-replacement`
with checkpoint commit `4395c251 checkpoint O4 sourcecycled web capture
ingestion` and implementation commit `543c6742 write sourcecycled web captures
to objectgraph`. The worker reports a narrow orange slice: `internal/cycle`
projects eligible sourcecycled `sources.Item` rows into durable
`choir.web_capture` objects via `objectgraph.Service.CreateWebCapture`, creates
`choir.source_entity` endpoints and `captured_from` edges for provenance, and
`cmd/sourcecycled` writes graph captures only when
`SOURCE_SERVICE_OBJECTGRAPH_DB_PATH`, `SOURCECYCLED_OBJECTGRAPH_DB_PATH`, or a
derived `RUNTIME_STORE_PATH` objectgraph DB path is configured. Worker-reported
checks passed: `nix develop -c go test ./cmd/sourcecycled -count=1`;
`nix develop -c go test ./internal/cycle -count=1`;
`nix develop -c go test ./internal/objectgraph -count=1`;
`nix develop -c go test ./internal/runtime -run 'TestHandleUniversalWireStories' -count=1`;
`git diff --check HEAD~2..HEAD`; and `git show --check --oneline HEAD`.
Worker worktree status is clean. Evidence boundary is worker-local branch-level
code/test proof only; no push, CI, deploy, staging, Texture native
`source_ref`, publication/export, Qdrant, auth/session renewal,
provider/gateway, promotion/rollback, or run-acceptance claim. Earlier pending
worktree handle
`local:2848c27e-c530-4401-87fb-709786e6e4b2` did not resolve in `list_threads`
and is superseded for orchestration by the readable replacement worker. The
worker is assigned the next smallest graph-backed News/Wire realism axis:
sourcecycled/web/source ingestion into durable `choir.web_capture` graph
objects, while keeping native Texture `source_ref` carry-forward,
publication/export, staging/deploy, Qdrant, provider/gateway, auth/session,
promotion, rollback, and run-acceptance claims out of scope unless explicitly
assigned. O4 Phase 4 verifier thread
`019f0395-93f6-7ad3-b89f-63aa07d9d5b0` (`O4 verifier - Source Open Browser
Proof`) returned `accept` with no findings. Earlier pending verifier worktree handles
`local:5cdd17ec-f3ed-489f-8339-37caa04201c4` and
`local:05c26241-c132-4699-a101-faa5183bdf45` did not resolve in `list_threads`
and are superseded for orchestration by the readable local verifier thread,
which was instructed to inspect the candidate in a detached temporary worktree
without mutating the shared orchestration checkout.
O4 Phase 4 worker thread `019f037f-41d4-7fa2-8ff7-d4a01ff78a64` (`O4 worker -
Universal Wire Source Open Browser`) produced branch
`codex/o4-phase4-universal-wire-source-open-browser-proof-replacement` at
`d49a19bd7ee2624e47b4bcd2f47e11e75f9195a4` (`prove Wire graph capture source
opening`). The worker reports a narrow orange frontend proof in
`frontend/src/lib/UniversalWireApp.svelte` and
`frontend/tests/universal-wire-app.spec.js`: Universal Wire graph-backed capture
cards map accepted source/open identity fields into existing Source
Viewer/Web Lens launch policy, keep Source Viewer as the default durable source
open action, expose Web Lens only as an explicit live/original action, and avoid
native Texture `source_ref`, publication/export, sourcecycled ingestion,
staging, deploy, Qdrant, provider/gateway, auth/session renewal, promotion,
rollback, and run-acceptance claims. Worker-reported focused browser proof and
`npm run build` passed; tracked status is clean on the worker branch, with only
ignored local logs/dependencies remaining. Verifier checks passed:
`git status --short --ignored`, `git diff --check 407bddce..HEAD`,
`git show --check --oneline d49a19bd7ee2624e47b4bcd2f47e11e75f9195a4`,
focused `npx playwright test tests/universal-wire-app.spec.js -g "graph capture sources" --timeout=120000`,
and `npm run build`; generated verifier artifacts were removed and the detached
verifier worktree was clean. Accepted worker commit was incorporated into this
orchestration branch as `2ad415b4 prove Wire graph capture source opening`.
Root checks passed: `git diff --check 8d52cf14..HEAD`;
`git show --check --oneline 2ad415b4`; `npm run build`; and focused
`npx playwright test tests/universal-wire-app.spec.js -g "graph capture sources" --timeout=120000`.
Root build emitted existing Svelte/a11y/chunk warnings; Vite emitted expected
proxy connection refusals for unmocked preference/websocket requests while no
backend proxy was running, but the focused proof passed. The earlier pending worker handle
`local:a5a3855d-0a7e-4bea-9bda-a4b2ba0fe840` did not resolve in `list_threads`
and is superseded for orchestration by the readable replacement worker.
Independent verifier thread
`019f0376-a32c-74b3-b1bc-35b9823e648f` (`O4 verifier - Universal Wire Source
Identity`) returned `accept` with no blocking findings. Earlier verifier
creation returned unresolved pending worktree handle
`local:ebca0ae2-f086-4b63-801b-70f26306a7eb`; that handle is superseded for
orchestration by the readable replacement verifier. O4 Phase 3 worker thread
`019f036b-3492-7213-b261-00daeee6445e` (`O4 worker - Universal Wire Source Ref
Citations`) completed on branch
`codex/o4-phase3-universal-wire-source-ref-citations` in
`/Users/wiz/.codex/worktrees/4aec/go-choir` at
`5b6086e1d42a990dc9baf1aad71cebdd6fcb5797`. The worker reports a checkpoint
commit first, then an additive Universal Wire DTO slice carrying graph/source
identity for `choir.web_capture` fallback cards without minting native Texture
`source_ref` citations. Accepted commits were incorporated into root as
`07dcb8e4 checkpoint O4 wire source identity gap` and `f7d4a852 carry Wire web
capture source identity`. Worker/verifier/root checks passed:
`nix develop -c go test ./internal/runtime -run 'TestHandleUniversalWireStories' -count=1`;
`nix develop -c go test ./internal/objectgraph -count=1`;
worker/verifier diff hygiene; root `git diff --check e18e92c8..HEAD`; and
root `git show --check --oneline` for both incorporated commits. Evidence is
branch-level local test/verifier/root-rerun proof only.

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
