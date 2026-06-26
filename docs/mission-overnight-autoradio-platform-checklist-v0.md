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
- [ ] Attach native `source_ref` citations and durable Source Viewer/reader
  artifacts to synthesized Universal Wire Texture articles.
- [ ] Update existing Universal Wire Texture articles and the live world model
  when new relevant information arrives instead of duplicating stale cards.
- [ ] Make the Universal Wire app render the current Texture article/world-model
  publication surface; raw `choir.web_capture` projections may remain diagnostic
  only and must not be labeled as the fulfilled product.

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

witness/spec (A/S): This paradoc, its ledger, thread callbacks, worker branches,
verifier verdicts, landed commits, CI/deploy/staging receipts, accepted blockers,
and successor paradocs.

invariants / qualities / domain ramp (I/Q/D): Follow Choir Doctrine and
`AGENTS.md`; use Problem Documentation First for new platform behavior problems;
preserve authority boundaries; use Codex thread tools for independent
worker/verifier evidence when available; never treat same-context review as an
independent prover; Source Viewer/reader artifacts are the default durable source
surface and Web Lens is explicit live/original inspection. Domain ramp:
docs/checkpoint -> branch tests -> focused local proof -> CI/deploy -> staging
product acceptance.

variant (ranking function) V: 73 total obligations = 9 WIP-preservation + 8
object graph + 7 Qdrant + 8 source-entity + 13 News/Universal Wire + 7
self-development + 7 Nucleus + 6 Choir Base + 8 Autoradio/Pipecat obligations.
Current value: 31. Last variant change: +5 open Universal Wire product
obligations after owner clarification that graph-backed capture projections are
not the product benchmark. Latest Delta V: 0 for the deployed Universal Wire
diagnostic-boundary repair: commits `73f0a888385a15a01a84eb726255b39662627b4d`
and `0975eea990a0de44f99a55ae0e5fb5aee2416bbd` stop publishing raw
`choir.web_capture` graph captures as public Wire articles and update the stale
sourcecycled proof, but they do not yet create English synthesis Texture
articles, update a live world model, or complete authenticated browser
acceptance because the accessible Chrome session is not authenticated to Choir.
Prior Delta V: 0 for the deployed O5 handoff repair
attempt: commit `c6bddff039636971727e9d05d5c267caa82e6d4b` repaired the
apparent channel-local mailbox cursor bug and deployed successfully, but the
post-deploy authenticated Chrome acceptance probe is blocked at passkey sign-in,
so no new O5 checklist obligation is closed yet. Earlier Delta V: 2 for starting
O5 through the authenticated staging product path: prompt bar created Texture doc
`d4b61d05-0e1c-44a9-a7b3-5e4b1048d812`, trajectory
`2f331a8d-3228-42da-9afb-238b33e2a7b9`, and Texture run
`0fe24855-e1f6-4f01-9230-51b5983cbb18`; Texture wrote v1 mission narrative
`b4afd7f0-1be6-43ab-87fd-38dc50cbd721` and requested Super execution, but no
Super run, worker/candidate evidence, AppChangePackage, or final
Texture-visible blocker has appeared yet.

budget: Overnight budget is already partially spent; solvency is feasible only
for bounded O4 follow-through plus explicit open handoff of the broader O5-O8
mission unless the owner grants a new long run.

authority / bounds: Orchestration may create worker/verifier Codex threads,
inspect worktrees, make docs/checkpoint commits, and land reviewed code through
the repo's normal loop. Behavior-changing work must declare mutation class,
protected surfaces, admissible evidence, rollback path, conjecture delta, and
heresy delta before editing. User granted staging deploy authority by pushing to
`origin/main` and authenticated Chrome QA on `choir.news`.

mutation class / protected surfaces: Current repo move is green evidence
documentation. The deployed Universal Wire diagnostic-boundary repair was an
orange runtime/API behavior change touching `/api/universal-wire/stories` and
sourcecycled/runtime tests; it deliberately did not touch Texture canonical
writes, provider/gateway calls, publication/export, Qdrant, promotion/rollback,
run acceptance, vmctl, or deployment routing beyond the normal staging deploy.
The landed O5 repair was orange/red-adjacent runtime/store behavior touching
coagent mailbox backlog selection and Texture-to-Super wake routing evidence,
with protected surfaces around coagent wake, Texture/Super routing, Trace
evidence, worker/candidate delegation, run acceptance, and auth/session because
staging proof depends on an authenticated browser. The earlier staging product
probe was a red-adjacent product-state mutation through authenticated prompt
bar, Texture canonical revision creation, Trace/channel message evidence, and
Super-request routing.

evidence packet: Behavior-changing landings require pushed commit SHA, CI run,
deploy status, staging health/build identity, deployed acceptance proof, verifier
thread id/verdict, rollback refs, mutation class, protected surfaces, heresy
delta, conjecture delta, residual risks, and next realism axis. Docs-only moves
require diff hygiene, docs truth workflow after push when pushed, and clear
non-claims.

heresy delta: `repaired` for the discovered stale-client asset problem at the
route/product-evidence level. Remaining evidence cap: the QA setup navigated the
Chrome tab after deploy, so this is not a pure preserved-tab-across-deploy proof;
the direct old-chunk 200 plus product source-open proof is the accepted evidence
for this pass.

position / live conjectures / open edges: O0 preservation, O1 objectgraph, O2
Qdrant derived index, O3 source entity phases, and the O4 capture-projection
substrate are recorded in the ledger with worker/verifier thread ids, accepted
commits, CI receipts, deploy identity, and staging/product evidence. Staging O4 runtime
repair `5b61fdc4fda5376d1fc39b119f12687944d41427` made sourcecycled project
3,833 web captures into the public platform VM graph, and the active VM returned
12 Universal Wire graph-backed stories. Follow-up deploy/static-asset repair
`b0f646d41301a57daae334264ca67e20d4aa2218` passed CI run `28260381195`, Node B
deploy job `83734770344`, and health identity `b0f646d4` deployed at
`2026-06-26T19:29:51Z`. The deploy smoke verified the public frontend asset
route, and staging serves both the current asset and the exact old failing chunk
`/assets/BrowserApp-BACPaCdk.js` with immutable asset headers. Authenticated
Chrome as `yusefnathanson@me.com` showed 12 Universal Wire articles;
`OPEN SOURCE` opened a Source Viewer reader artifact for `Our 36 favorite gaming
deals on Prime Day for Switch, PS5, and Xbox`, and `WEB LENS` opened
`https://www.theverge.com/gadgets/951901/prime-day-video-games-switch-playstation-xbox-pc-deal-sale`
with a source reader snapshot. Final O4 verifier thread
`019f0570-cab8-78e1-8dca-2f058ecf7e13` accepted closure for this
graph-backed capture-projection News benchmark scope. Non-O4 realism axes remain
open: native Texture body `source_ref` citation carry-forward,
publication/export, Qdrant projection, provider/search freshness, run
acceptance, promotion/rollback, and preserved-tab-across-deploy proof beyond one
previous asset URL resolving. Owner clarification on `2026-06-26` reopens the
actual Universal Wire product benchmark: the deployed screenshot showed
Portuguese individual-source cards titled `Telegram Post from Metropoles
Telegram`, repeated per capture, with card text explicitly saying the cards are
capture projections and not Texture article publications or native
`source_ref` citations. The intended product is multilingual ingestion ->
object-graph/Texture processing -> English synthesis Texture articles -> live
world model -> article updates on new relevant information. A bounded
diagnostic-boundary repair landed after that clarification: commit
`73f0a888385a15a01a84eb726255b39662627b4d` changes Universal Wire so graph
captures remain diagnostic substrate and the public story list stays empty until
a Texture synthesis edition exists; commit
`0975eea990a0de44f99a55ae0e5fb5aee2416bbd` repairs the stale sourcecycled proof.
Push CI run `28263422604` passed but skipped deploy because the head diff was
test-only, so a manual `workflow_dispatch` with `force_staging_deploy=true`
ran CI/deploy as run `28263687466`. That run passed and Node B deploy job
`83745803112` succeeded. Staging `/health` reports proxy and sandbox deployed
commit `0975eea990a0de44f99a55ae0e5fb5aee2416bbd`, deployed at
`2026-06-26T20:37:16Z`. Unauthenticated `/api/universal-wire/stories` returns
401 as expected, and adding `X-Authenticated-User` from curl still returns 401,
so public auth cannot be bypassed with that header. The accessible Chrome window
also returns `{"error":"authentication required"}` for the API, so deployed
authenticated acceptance is blocked until the owner reauthenticates the usable
Chrome profile/session. O5 has now started through the product path:
authenticated Chrome as `yusefnathanson@me.com` submitted
`O5_PRODUCT_PATH_PROBE_20260626` through prompt bar at `2026-06-26T19:45:58Z`.
The product created Texture doc `d4b61d05-0e1c-44a9-a7b3-5e4b1048d812`,
trajectory `2f331a8d-3228-42da-9afb-238b33e2a7b9`, and Texture run
`0fe24855-e1f6-4f01-9230-51b5983cbb18`. Texture wrote v1
`b4afd7f0-1be6-43ab-87fd-38dc50cbd721` as a mission narrative, invoked
`request_super_execution` with update
`33cb84e5-d9a3-4144-8bce-eda258125b07`, and emitted a channel message to
`super:5bd6de97-3b58-408c-bf89-c42c81b083de`. A second Super request was
deduped with `dedupe_reason=texture_run_already_requested_super`. By
`2026-06-26T19:50:23Z`, the trajectory still listed only conductor and Texture
agents, no Super loop id/state, no worker/candidate evidence, no
AppChangePackage, no new revision after v1, and no final Texture-visible
blocker; Texture passivated at `2026-06-26T19:48:40Z` with
`reason=idle_deadline`. O5 repair commit
`c6bddff039636971727e9d05d5c267caa82e6d4b` changes the coagent mailbox backlog
query so undelivered channel messages are visible even when a persistent actor's
prior processed cursor is higher than the new channel-local sequence. CI run
`28262237653`, FlakeHub run `28262237683`, and Node B deploy job `83740978086`
all succeeded. Staging health reports proxy and sandbox deployed commit
`c6bddff039636971727e9d05d5c267caa82e6d4b`, deployed at
`2026-06-26T20:07:06Z`. The post-deploy Chrome proof attempted to submit
`O5_HANDOFF_REPAIR_PROOF_20260626`, but the product required passkey sign-in
before accepting the durable prompt. Chrome then reported that the passkey UI
was blocking automation. This is an acceptance blocker, not proof that the
handoff repair works on staging.

next move: Problem Documentation First is satisfied for the clarified Universal
Wire product gap by this paradoc/ledger update, and the immediate bad fallback
has been repaired so raw graph captures are diagnostic-only rather than public
articles. Next, design a bounded Universal Wire synthesis slice: choose a
multilingual source cluster, create or update a Texture article in English with
native `source_ref` citations and Source Viewer artifacts, expose that article
through Universal Wire, and record the remaining world-model/update gap. The O5
post-deploy handoff proof is still useful, but it should not distract from the
fact that the product target is synthesis through Texture rather than capture
projection.

ledger file: `docs/mission-overnight-autoradio-platform-checklist-v0.ledger.md`

version / lineage: v0 created after email-freeze landing. It supersedes loose
queue ordering from `docs/worktree-review-2026-06-23.md` for overnight execution
order while preserving that report as evidence.

learning state: Thread-native orchestration is now proved usable in this Codex
environment through `codex_app` thread tools. The current shared learning is that
local/branch graph proofs did not settle O4 until staging auth, deploy identity,
sourcecycled readback, active platform VM API, and authenticated browser product
QA all agreed, but even that did not settle the actual Universal Wire product.
The graph-backed capture projection is useful substrate evidence and bad product
evidence. The diagnostic-boundary repair improves the product by removing that
bad evidence from the public article list, but it is not the synthesis pipeline.
O5's first product-path probe improved the belief state: prompt bar
and Texture materialization work on staging for a self-development mission, and
Texture can issue a Super execution request, but the observed request did not
wake a visible Super loop or produce downstream worker/package evidence. The
local root cause is plausibly repaired and deployed, but product acceptance is
blocked until Chrome is re-authenticated with passkey.

settlement: not settled. O4 News/Universal Wire is accepted only at the
graph-backed capture-projection substrate scope plus the deployed
diagnostic-boundary repair that prevents raw captures from being published as
articles; owner clarification shows that the actual product benchmark remains
open. O5 has started through product prompt-bar/Texture/Super-request evidence.
The first O5 handoff repair is landed and deployed but not product-accepted
because Chrome is waiting on passkey sign-in or another usable Choir auth
session. This mission remains `working` because O4 synthesis/world-model
obligations, O5 package/blocker/verifier obligations, and O6-O8 remain open.
Exit requires `settled`, `open_handoff`, `blocked`, or `superseded` with
remaining V and next assignment explicit.

## Suggested Goal String

```text
Use Parallax on docs/mission-overnight-autoradio-platform-checklist-v0.md. Treat it as the source program for an overnight, thread-native mission. One orchestration thread owns the checklist, ledger, worker/verifier thread creation, dependency order, worktree hygiene, and evidence synthesis. Current Codex thread tools are available through codex_app after tool discovery: list_projects/create_thread to start bounded project-scoped implementation and verifier threads, send_message_to_thread for follow-ups and explicit callbacks, read_thread/list_threads to reconnect verdicts, handoff_thread/get_handoff_status for ownership-transfer cases, and set_thread_title/set_thread_pinned/set_thread_archived for operator hygiene. If thread tools are not exposed in a later execution environment, record that as a blocker to thread-native settlement and do not treat same-context review as independent proof. Execute in order: O0 preserve WIP, O1 object graph, O2 Qdrant derived index, O3 source entities, O4 News/Universal Wire, O5 Choir-in-Choir self-development, O6 Nucleus capsules, O7 Choir Base, O8 Autoradio/Pipecat vertical slice. Each worker assignment must name mutation class, protected surfaces, admissible evidence, rollback path, heresy delta, callback target, and stop condition. Follow AGENTS.md: Problem Documentation First for new behavior problems; behavior-changing landings require commit, push, CI, deploy identity, staging acceptance, verifier evidence, rollback refs, and residual risks. Update Parallax State in place and append to docs/mission-overnight-autoradio-platform-checklist-v0.ledger.md after each material pass. Exit only as settled, open_handoff, blocked, or superseded, with remaining V and next thread assignment explicit.
```
