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
web-capture objects. The product target is not a raw capture grid: Universal
Wire should ingest many multilingual news stories, process them into the object
graph through Texture-owned agent workflows, publish English synthesis Texture
articles, maintain a live updating world model, and update existing articles
when new relevant information arrives.

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
- [x] Verify source/citation links open to real source artifacts or Source
  Viewer/reader artifacts. O4 Phase 4 proves frontend routing for graph-backed
  fallback cards. O4 Phase 9 adds accepted and incorporated branch-level API/UI
  proof that graph-backed capture source handles carry durable reader snapshots
  and open Source Viewer/reader artifact text by default while Web Lens remains
  an explicit live/original action. O4 Phase 10c repairs the local browser
  harness needed for native Texture source-ref proof. O4 Phase 10d adds accepted
  and incorporated branch-level browser proof that native Texture graph-wrapper
  `source_ref` opening distinguishes inline citation note text from graph object
  reader body text and opens Source Viewer reader artifact content by default.
  O4 final staging proof closes this for the graph-backed Universal Wire capture
  projection scope: authenticated Chrome QA opened a deployed Universal Wire
  card through Source Viewer/reader artifact and Web Lens reader surfaces.
- [x] Open independent verifier thread before claiming News benchmark. Final O4
  settlement verifier thread `019f0570-cab8-78e1-8dca-2f058ecf7e13` returned
  `accept` for closing the graph-backed capture-projection News benchmark scope,
  with the boundary that it audited recorded authenticated Chrome QA plus
  independently reproduced public health/asset checks rather than replaying a
  fresh authenticated browser session.
- [ ] Cluster many multilingual ingested stories into cross-source story/world
  model objects instead of rendering one card per captured source item.
- [ ] Route source clusters through Texture/processor/reconciler workflows to
  create English synthesis Texture articles, not translated or copied individual
  article projections.
- [x] Attach native `source_ref` citations and durable Source Viewer/reader
  artifacts to synthesized Universal Wire Texture articles. O4 deployed
  acceptance for `a2a5a749` proved one runtime-owned synthesis Texture article
  with `[1]`/`[2]` source refs and Source Viewer/reader opening on staging. This
  does not yet prove semantic clustering, world-model reconciliation, or later
  article updates.
- [ ] Update existing Universal Wire Texture articles and the live world model
  when new relevant information arrives instead of duplicating stale cards.
- [ ] Make the Universal Wire app render the current Texture article/world-model
  publication surface; raw `choir.web_capture` projections may remain diagnostic
  only and must not be labeled as the fulfilled product.

Current staging evidence: authenticated Chrome QA after deploy
`a2a5a74910be1c189cd9d9f090695169bf729561` on 2026-06-26 shows Universal Wire
rendering `1 article`, `Universal Wire live synthesis: Telegram Post from
Metropoles Telegram`, as a `universal-wire-edition-texture` story with `[1]` and
`[2]` source refs. `OPEN SOURCE` opens the Source Viewer/reader artifact
`Telegram Post from Metropoles Telegram`, marked `Available source` and `Reader
snapshot ready`, with original link `t.me/Metropoles/407020`. Direct raw
unauthenticated `/api/universal-wire/stories` still returns HTTP 401 as
expected. Raw `choir.web_capture` projections remain diagnostic substrate.

Current failure evidence: owner screenshots on 2026-06-26 at about 18:30 ET
show that the deployed Universal Wire product still renders only one article,
with headline `Universal Wire live synthesis: Telegram Post from Metropoles
Telegram` and third-person/meta body copy beginning `Universal Wire selected 24
graph-backed source captures...`. Clicking the headline opens a Texture window
for that title, but the editor is blank and reports `Get document failed
(404)`. Code inspection confirms the odd headline/body copy is generated by the
runtime sourcecycled synthesis trigger in
`internal/runtime/sourcecycled_web_captures.go`, not by the UI. The 404 is
narrowed to the Universal Wire -> Texture read boundary: frontend opens the
story's `story_texture_doc_id` with `platformRead`, while
`resolveUniversalWireTextureReadOwner` only permits reading the platform-owned
document if the current platform `universal-wire/Wire.texture` edition still
transcludes that doc id. Existing frontend tests mock the Texture document API
and therefore do not prove the deployed cross-owner document read path.
Authenticated API JSON was not captured in this pass because Chrome automation
was blocked by another extension UI; this checkpoint should not claim the exact
stored DTO or edition-head state beyond the screenshot and code evidence.

Acceptance: on `https://choir.news`, authenticated Universal Wire returns and
renders non-empty English synthesis Texture articles from multilingual ingested
source clusters, with native source citations, durable source artifacts, and
evidence that newly relevant source information updates an existing article or
world-model entry. Evidence includes deployed commit identity. The prior O4
verifier result remains valid only for the graph-backed capture-projection
substrate and source-opening path.

### O5 - Choir-in-Choir Self-Development

Goal: use the News/Universal Wire work as the first real self-development
payload rather than a toy task.

Checklist:

- [x] Start from product path, not Codex-only edits. Evidence: authenticated
  Chrome prompt-bar submission on staging at `2026-06-26T19:45:58Z` created
  Texture document `d4b61d05-0e1c-44a9-a7b3-5e4b1048d812` and trajectory
  `2f331a8d-3228-42da-9afb-238b33e2a7b9`.
- [x] Use prompt bar / Texture / super path to create or continue a mission.
  Boundary: Texture authored v1 mission narrative
  `b4afd7f0-1be6-43ab-87fd-38dc50cbd721`, invoked
  `request_super_execution`, and sent a channel message to
  `super:5bd6de97-3b58-408c-bf89-c42c81b083de`; Super did not yet appear as a
  running trajectory agent or produce worker/package evidence.
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

mission conjecture: If this orchestration thread preserves evidence, uses
thread-native workers/verifiers, and advances O4-O8 in dependency order without
weakening staging/product proof, then Choir materially advances toward the
Autoradio benchmark.

deeper goal (G): Autoradio should become the concrete benchmark for Choir's
self-improving mainframe: source-grounded information, durable artifacts,
self-development, sandboxed work, persistent file-like state, and realtime audio
converge into one usable product.

witness/spec (A/S): this paradoc and ledger, thread callbacks, worker branches,
verifier verdicts, landed commits, CI/deploy/staging receipts, accepted blockers,
and successor paradocs.

invariants / qualities / domain ramp (I/Q/D): Follow Choir Doctrine and
`AGENTS.md`; document behavior problems before fixes; use Codex thread tools
for independent worker/verifier evidence when exposed; never treat same-context
review as independent proof; Source Viewer/reader artifacts are the durable
source surface and Web Lens is explicit live/original inspection. Domain ramp:
checkpoint -> branch tests -> focused local proof -> CI/deploy -> staging
product acceptance.

variant (ranking function) V: 73 total obligations. Current value: 28 after
incorporating accepted branch-local O4 world-model/same-article-update proof in
`8121b4d4ca835d1c334e18144296683098506f59`. O4 was reopened by +5 after owner
clarification that graph-backed capture projections are substrate, not the
Universal Wire product. The latest accepted Delta V is 1: verifier thread
`019f0617-f88e-72d3-a71a-c59b8a40e7a7` accepted worker commit
`1e3e72bed659c7992aa09d4bfd6fcd3a84176d39`, now incorporated as `8121b4d4`, for
durable branch-local `choir.universal_wire_story_cluster` identity and
same-article revision when a later relevant source arrives. The latest deployed
failure does not change V by itself because it sharpens an already-open O4
obligation: the headline/article publication surface opens a blank Texture
window with a 404, and the synthesis prose is deterministic platform meta-copy
rather than news article copy. Remaining O4 variant is cross-source/world-model
clustering, deployed publication-surface document readability, article-quality
synthesis, and authenticated product evidence that existing synthesis articles
update when later relevant information arrives.

budget: Solvency is tight. Use bounded O4 follow-through plus explicit handoff
for broader O5-O8 unless the owner grants a new long run.

authority / bounds: Orchestration may create Codex worker/verifier threads,
inspect worktrees, make docs/checkpoint commits, and land reviewed code through
the repo loop. User granted staging deploy by pushing to `origin/main` and
authenticated Chrome QA on `choir.news`. The visible Chrome app shell is now
authenticated enough for product UI proof, while direct raw navigation to
`/api/universal-wire/stories` still returns HTTP 401 and should not be mistaken
for a non-empty product result. Behavior-changing work must name mutation class,
protected surfaces, admissible evidence, rollback path, conjecture delta, and
heresy delta before editing.

mutation class / protected surfaces: Current move is green documentation after
orange runtime incorporation. The next O4 repair is orange/yellow if it changes
runtime, frontend, API behavior, or tests. Authorized protected surfaces are
Universal Wire story DTOs, runtime synthesis/article materialization, existing
article revision/upsert semantics, Texture revisions through existing helpers,
source entity/source_ref projection, Wire edition linkage, and the read-only
Texture publication surface for platform-owned Wire articles. It must not touch
auth/session renewal, vmctl, deployment routing, provider/gateway credentials,
Qdrant, promotion/rollback, run acceptance, or publication/export outside
existing Wire edition helpers.

evidence packet: Behavior-changing settlement needs pushed commit SHA, CI run,
deploy status, staging health/build identity, deployed acceptance, verifier
thread id/verdict, rollback refs, mutation class, protected surfaces, heresy
delta, conjecture delta, residual risks, and next realism axis. Branch/local
worker proof may stop at focused tests, `git diff --check`, commit SHA, dirty
classification, residual risks, and non-claims. Docs-only moves need diff
hygiene and Docs Truth Check if pushed.

heresy delta: `repaired` for raw capture projection publication as public Wire
articles, read-time materialization/backfill for existing sourcecycled captures,
the staging platformd filter that hid runtime-owned synthesis stories, and
branch-local same-article/world-model identity over time for the deterministic
`sourcecycled-live` cluster.
`discovered` now includes the headline-to-Texture 404 and the fact that deployed
article copy is runtime meta-prose rather than a reader-facing news synthesis.
`discovered` remains for semantic production cluster selection, provider
freshness, live world-model maintenance, and update-existing-article semantics.

position / live conjectures / open edges: O0-O3 are accepted from prior ledger
evidence. O4 now has a deployed, authenticated, non-empty slice:
`a2a5a74910be1c189cd9d9f090695169bf729561` passed CI/deploy, health identity,
and Chrome QA with one English synthesis Texture article and Source
Viewer/reader source opening. Verifier threads accepted the source-cluster slice
(`019f05ba-e585-7573-a752-851a43364c9e`), live sourcecycled trigger
(`019f05db-9738-7c82-ad22-06f6763f25c3`), materialization repair
(`019f05f0-81de-76a2-bb57-c2c66db82272`), and platform verification filter
repair (`019f05fc-425f-7790-9b73-5527fffa7fc3`), and world-model/same-article
update slice (`019f0617-f88e-72d3-a71a-c59b8a40e7a7`). Commit `8121b4d4`
incorporates the accepted branch-local proof that the deterministic
`sourcecycled-live` cluster has durable objectgraph identity, source-capture
edges, one article document, one Wire edition transclusion, and later-source
revision of the same article. O4 remains open because the owner-observed
deployed article headline opens a blank Texture editor with `Get document failed
(404)`, the live card copy is deterministic platform meta-prose rather than
news article copy, and semantic multi-story clustering/provider-quality
synthesis are still future realism axes. O5 product path has started through
prompt bar/Texture/Super request, but O5 acceptance has not been replayed after
the latest O4 work.

next move: create a bounded O4 article-surface repair worker. It should start
from current root commit `8121b4d4ca835d1c334e18144296683098506f59`, read the
deployed failure checkpoint, and implement the smallest fix that makes the
Universal Wire headline open the actual platform-owned Texture article without
404 while replacing deterministic Universal Wire meta-copy/headline framing with
reader-facing article synthesis copy for the deterministic slice. It must add a
test that exercises the real Universal Wire story -> Texture document read path,
not only mocked `/api/texture/*`, and must preserve source_ref/Source Viewer
behavior. Stop at `ready_for_verifier`; do not push or deploy. Expected Delta V:
1 if branch-local tests and an independent verifier accept the headline-open
repair. Deployed product acceptance still requires commit, push, CI/deploy
identity, authenticated Chrome replay, and verifier evidence.

ledger file: `docs/mission-overnight-autoradio-platform-checklist-v0.ledger.md`

version / lineage: v0 created after email-freeze landing. It supersedes loose
queue ordering from `docs/worktree-review-2026-06-23.md` for overnight execution
order while preserving that report as evidence.

learning state: Thread tools are exposed and usable in this environment. The
key O4 learning is that staging product evidence changed the route twice:
missing edition materialization and platformd filtering were real product gaps
that local branch tests did not expose. Universal Wire now has a visible
synthesis article, but the next realism axis is identity over time: a story or
world-model object must survive later source arrivals and cause article
revision, not card duplication; branch-local proof for that axis is now
accepted and incorporated, but not deployed/product-accepted. The new owner
screenshots add a second realism axis: the rendered card must open the actual
platform-owned Texture article document, and the content must read as a news
synthesis rather than status text about Universal Wire. The key O5 learning is
that prompt bar and Texture
materialization work, but Texture-to-Super acceptance still needs authenticated
staging replay.

settlement: not settled. O4 News/Universal Wire is accepted at graph-backed
capture-projection substrate scope, deployed diagnostic-boundary repair scope,
verified/deployed first synthesis-route-slice scope, and branch-local
world-model/same-article identity scope. The actual product benchmark remains
open until the deployed headline opens the readable Texture article, article
copy is not platform meta-copy, multilingual live ingestion produces/upserts
English synthesis Texture articles, and authenticated product evidence shows the
world model/existing articles update. O5 has started through product
prompt-bar/Texture/Super-request evidence. The first O5 handoff repair is landed
and deployed but not product-accepted in this pass. This mission remains
`working` because O4 deployed article-surface repair, broader O4 realism axes,
O5 package/blocker/verifier obligations, and O6-O8 remain open. Exit requires
`settled`, `open_handoff`, `blocked`, or `superseded` with remaining V and next
assignment explicit.

## Suggested Goal String

```text
Use Parallax on docs/mission-overnight-autoradio-platform-checklist-v0.md. Treat it as the source program for the thread-native mission. Current status is working with V=28. O4 News/Universal Wire has a deployed, authenticated narrow slice at commit a2a5a74910be1c189cd9d9f090695169bf729561: one English synthesis Texture article renders in Universal Wire with native source_ref citations and Source Viewer/reader opening. Branch-local commit 8121b4d4ca835d1c334e18144296683098506f59 incorporates verifier-accepted worker commit 1e3e72bed659c7992aa09d4bfd6fcd3a84176d39, proving durable `sourcecycled-live` story-cluster identity and same-article revision when a later relevant source arrives. Owner screenshots on 2026-06-26 reveal the current deployed product is still not Universal Wire: it shows one deterministic meta-copy article, and clicking the headline opens a blank Texture window with `Get document failed (404)`. O4 remains open because the real product target is multilingual ingestion -> graph/Texture processing -> English synthesis Texture articles -> live world model -> updates to existing articles; the current route does not yet prove semantic clustering, article-quality synthesis, readable deployed Texture article publication surface, or same-article update semantics with product evidence. Next move: create a bounded O4 article-surface repair worker from 8121b4d4 to fix the headline-to-Texture 404 and replace Universal Wire meta-copy with reader-facing synthesis copy, stopping at ready_for_verifier with branch-local tests that exercise the real story -> Texture document read path rather than only mocked `/api/texture/*`. Use Codex app thread tools when exposed: list_projects/create_thread for bounded workers/verifiers, read_thread/list_threads to reconnect verdicts, send_message_to_thread for follow-ups/callbacks, handoff_thread/get_handoff_status only for ownership transfer, and set_thread_title/set_thread_pinned/set_thread_archived for hygiene. Each worker/verifier assignment must name mutation class, protected surfaces, admissible evidence, rollback path, heresy delta, callback target, and stop condition. Follow AGENTS.md and Problem Documentation First. Behavior-changing landings require commit, push, CI, deploy identity, staging acceptance, verifier evidence, rollback refs, and residual risks. Update Parallax State in place and append to docs/mission-overnight-autoradio-platform-checklist-v0.ledger.md after each material pass. Exit only as settled, open_handoff, blocked, or superseded with remaining V and next assignment explicit.
```
