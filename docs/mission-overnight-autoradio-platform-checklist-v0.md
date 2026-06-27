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
  acceptance for `4f8cae7a` proved multiple runtime/platform-owned synthesis
  Texture articles with native source refs, source buttons, source counts, and
  loaded Texture article windows on staging. This does not yet prove semantic
  clustering, world-model reconciliation, or later article updates.
- [ ] Update existing Universal Wire Texture articles and the live world model
  when new relevant information arrives instead of duplicating stale cards.
- [x] Make the Universal Wire app render the current Texture article publication
  surface for the immediate deployed synthesis slice. Deployed `4f8cae7a`
  renders a five-article Universal Wire surface, opens nonblank Texture content
  from headlines, and keeps raw `choir.web_capture` projections as diagnostic
  substrate. Boundary: this proves deployed deterministic multi-article
  readability, not the intended semantic/world-model News surface.

Current staging evidence: deployed `ad4d739e5f89e3574d4923b1bc580c50db53785d`
incorporates the direct stale-edition article repair. CI run `28279556809`,
Docs Truth Check run `28279556816`, FlakeHub run `28279556808`, deploy job
`83792786877`, and public health identity all passed for that SHA; health
reports proxy and sandbox `commit`/`deployed_commit` equal to
`ad4d739e5f89e3574d4923b1bc580c50db53785d`, deployed at
`2026-06-27T05:17:49Z`. Public unauthenticated
`/api/universal-wire/stories` still returns 401 as expected. Authenticated
Computer Use replay in the owner's signed-in Chrome tab showed Universal Wire
rendering `11 articles`; the visible cards no longer showed
`Multiple reports converge on ...`, `incoming reports point to the same
developing story`, `A second source in the cluster...`, or `reports read as one
developing article`. Opening a headline loaded a repaired Texture article window
at v66 with `Sources 24`, native source buttons, rendered article text, and
`Document loaded`.

Current repair boundary: the deployed product now supports the narrow
read-repair conjecture that existing scaffold-framed Universal Wire Texture
articles can revise on read into article-facing copy without loosening raw
`choir.web_capture` diagnostic-only boundaries. This still does not claim the
full Universal Wire product: staging still shows deterministic/formulaic prose,
some visibly incoherent deterministic clusters, no production-quality
provider/model synthesis, no Qdrant/world-model projection, and no deployed
evidence that later relevant sources update existing semantic world-model
articles.

Current failed repair evidence: root documented the read-repair gap in
`2c94a9ed971ca435d5331a8668b900e64f6857aa`, repaired the runtime predicate in
`32ee51f11e976a7b41c7dd554966d332da824759`, recorded local proof in
`9e4b3baa7cc394ec8a59138a40a7598177ac1c2d`, then pushed and deployed that
stack. CI run `28279219223`, Docs Truth Check run `28279219233`, FlakeHub run
`28279219236`, deploy job `83791870176`, public health identity, and
unauthenticated 401 proof all passed for `9e4b3baa`. Authenticated Computer Use
replay in the owner's signed-in Chrome tab still showed `11 articles` with the
same scaffold copy: `Multiple reports converge on ...`, `incoming reports point
to the same developing story`, `A second source in the cluster...`, and
`reports read as one developing article`. A hard reload did not repair them.
The likely cause is that the read-repair path only asks the live graph
synthesizer to run; existing stale edition Texture documents remain stale when
that graph pass does not directly revise the already-transcluded documents. The
next repair must operate on stale edition Texture articles themselves, deriving
repair sources from their current revision/source entities or metadata instead
of depending only on the live graph materializer.

Direct-repair settlement evidence: root documented the deployed miss in
`d6ab80f9b0d8a0898491517498f51792837d89fb`, then repaired stale edition article
handling in `da4bcb7f133569b6847c5a14f95fba9b40898897`. The new path detects
scaffolded Universal Wire edition stories, loads their platform-owned Texture
documents/current revisions, reconstructs synthesis sources from structured
source entities/reader snapshots, creates a new article-facing revision for the
same cluster/doc through the existing synthesis helper, and normalizes the
document title away from stale `Multiple reports converge...` framing. Focused
repair/materialization tests and the broader Universal Wire runtime selector
passed locally. Root deployed it as part of
`ad4d739e5f89e3574d4923b1bc580c50db53785d`, and authenticated staging replay
accepted the read-repair transition for visible Universal Wire cards plus a
headline-opened Texture article.

Current semantic/world-model update gap: O4 still does not satisfy the owner's
Universal Wire target. The runtime grouping path uses bounded token concepts
(`topic:*` plus `signal:*`) over recent graph-backed captures, creates or
revises Texture articles by stable cluster slug, and records
`choir.universal_wire_story_cluster` objects whose body/metadata summarize the
article and source IDs. That is useful substrate, but it is not a live world
model: there is no durable semantic event/entity identity separate from the
heuristic cluster slug, no typed state delta saying what changed when a later
source arrives, no decision rule for whether new information updates an
existing world-model article versus opens a sibling event, and no deployed proof
that the article revision is driven by a semantic object update rather than
source-card regrouping. The next C6/C8 move must document and then build the
smallest real world-model update slice, not another copy/prose cleanup.

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

variant (conjecture descent) V: count driving conjectures still undecided,
under-evidenced for their settlement tier, or lacking a strong definitive
statement. Current value: 3. Last delta: expected Delta V 0 for the deployed
source-arrival discriminator; actual Delta V 0 because staging evidence found
live source arrival but also exposed a blocker: the public Wire read still
collapses into one broad article rather than a coherent multi-story/update
surface. Last decided
conjecture: deployed commit
`a10254d2072c8cc63c910551f3d1fb588fe87605` supports the narrow claim that
current/stale synthesized Universal Wire Texture articles expose product-visible
`semantic_story` DTO evidence through the authenticated product API without
leaking internal semantic ids into article copy. CI run `28281421752`, Docs
Truth Check run `28281421753`, FlakeHub run `28281421747`, deploy job
`83798061615`, health identity, and authenticated Computer Use replay all
passed. The replay showed 12 stories, `semanticStoryCount: 12`, literal
`semantic_story` present, the first story using
`choir.universal_wire_story_cluster.semantic.legacy.v1` with
`legacy_revision_projection`, `source_count: 24`, and `copyLeak: false`. UI
smoke still showed Universal Wire rendering 12 articles and a headline-opened
Texture article loading at v66 with `Sources 24`, native source buttons, and
`Document loaded`. Current live conjectures remain: C6 later relevant sources
update existing articles/world-model identities at deployed source-arrival scope;
C8 semantic/world-model clustering beyond bounded deterministic signals replaces
heuristic grouping; C9 O5-O8 embed the Autoradio benchmark into durable Choir
product paths.

budget: Solvency is tight. Use bounded O4 follow-through plus explicit handoff
for broader O5-O8 unless the owner grants a new long run.

authority / bounds: Orchestration may create Codex worker/verifier threads,
inspect worktrees, make docs/checkpoint commits, and land reviewed code through
the repo loop. User granted staging deploy by pushing to `origin/main` and
authenticated Chrome QA on `choir.news`. The visible Chrome app shell and a
direct Chrome tab to `/api/universal-wire/stories` are authenticated enough for
product UI/API proof in the owner's session; unauthenticated shell/API calls
remain outside that product path. Behavior-changing work must name mutation
class, protected surfaces, admissible evidence, rollback path, conjecture delta,
and heresy delta before editing.

mutation class / protected surfaces: Current move is a green Problem
Documentation First checkpoint for the deployed source-arrival clustering/update
blocker. The next worker is an orange/red branch-local behavior slice over
Universal Wire sourcecycled ingestion/materialization,
`/api/universal-wire/stories`, semantic story cluster state, Texture revision
creation/revision, Wire edition linkage, source entity/source_ref carry-forward,
platform Texture sync/read paths, and product acceptance probes. Rollback path
is revert the next implementation commit(s) back to
`292de87f9fd8d84f15ea2d69315ae574f9953135` plus dependent evidence commits.
The next move must not touch auth/session renewal, vmctl, deployment routing,
provider/gateway credentials, Qdrant, promotion/rollback, run acceptance, or
publication/export outside existing Wire edition helpers unless a separate
checkpoint names that broader conjecture.

evidence packet: Behavior-changing settlement needs pushed commit SHA, CI run,
deploy status, staging health/build identity, deployed acceptance, verifier
thread id/verdict, rollback refs, mutation class, protected surfaces, heresy
delta, conjecture delta, residual risks, and next realism axis. Branch/local
worker proof may stop at focused tests, `git diff --check`, commit SHA, dirty
classification, residual risks, and non-claims. Docs-only moves need diff
hygiene and Docs Truth Check if pushed.

heresy delta: `repaired` at deployed product API tier for the narrow C6/C8
semantic story DTO observability gap: current/stale synthesized Wire Texture
articles now return `semantic_story` evidence through authenticated
`/api/universal-wire/stories` without exposing internal semantic ids in
reader-facing copy. `repaired` remains at branch-local tier for the semantic
story-state slice that records semantic story identity/change in graph/revision
metadata and revises the linked Texture article. `repaired` remains at deployed
product scope for C11, the stale scaffold-framed Universal Wire Texture article
read-repair gap. `repaired` also remains for prior read-owner, platformd
current-head, revision envelope, source-copy, supplied-revision sync,
zero-article, deterministic multi-article readability, and source-opening
regressions recorded in the ledger. `discovered` remains for semantic production
cluster selection, provider/model synthesis quality, source freshness, deployed
later-source arrival update proof, Qdrant/world-model projection, and the fact
that the product is still below the intended semantic multi-article world model.

position / live conjectures / open edges: O0-O3 are accepted from prior ledger
evidence. O4 is accepted for deployed deterministic multi-article readability,
direct stale-edition read repair, and product-visible semantic DTO
observability. The supported deployed head is
`a10254d2072c8cc63c910551f3d1fb588fe87605`; root evidence shows CI/deploy,
health identity, authenticated API proof with 12/12 `semantic_story` stories,
and UI proof of a loaded Texture article at v66 with `Sources 24`. The stronger
News conjecture remains open: current deterministic topic/signal grouping and
formulaic prose are not provider/model-quality synthesis, not Qdrant or
world-model projection, and not deployed proof that later relevant source
arrivals update existing semantic articles.

Current C6 source-arrival update conjecture: if a later sourcecycled source
arrives for an already-materialized Universal Wire semantic story, the product
path should revise the same semantic story object and linked Texture article
instead of creating a duplicate card, losing prior citations, or requiring
manual reseeding. Branch-local root evidence proves the test predicate that the
later/second Texture revision preserves prior/new `source_item_ids`, source
entities, and native `source_ref` citations while the surrounding test holds
same story/article/edition identity. Staging evidence now gives the next
blocker: sourcecycled is live and did deliver new source arrivals, but the Wire
surface is still not the intended multi-story/update product.

Deployed discriminator evidence: Node B sourcecycled is active and its latest
cycle `cycle_aab51c4b894bba17afea9fb2` ran from `2026-06-27T07:10:21Z` to
`2026-06-27T07:11:38Z`, fetched 562 new items from 211 configured sources, and
recorded `web_captures_graph_written` with `capture_count: 561`,
`source_entity_count: 561`, `captured_from_edges: 561`,
`objectgraph_mode: runtime_api`, and target
`http://unix/internal/vmctl/sandbox-proxy/universal-wire-platform/internal/runtime/objectgraph/web-captures`.
The unauthenticated public API remains 401 as expected, and Chrome automation
could not complete an authenticated replay because another extension UI blocked
the Chrome extension session. A lower-tier sandbox route diagnostic with
`X-Authenticated-User` showed `/api/universal-wire/stories` returning
`source: universal-wire-edition-texture`, `story_count: 1`, edition
`95afb28c-1095-4b96-bdf8-c1b89b13bc56`, included doc
`d3661377-4731-4617-a351-63236b08597d`, headline `Cory Doctorow on the Right -
and Wrong - Way to Criticize AI`, `semantic_story.change_type: source_added`,
`previous_source_count: 24`, `current_source_count: 24`, topic concepts
`energy`, `harbor`, `health`, and 273 signal concepts. This is evidence of a
live ingestion/update blocker, not product acceptance.

next move: monitor independent verifier pending handle
`local:60ba30c4-6e1c-4c6f-8b7e-8c09f760bf33` for
`O4-deployed-source-arrival-clustering-update-verifier`. Worker thread
`019f07f3-7ddc-7e72-8b9f-e26bc4af1833` returned `ready_for_verifier` on branch
`codex/o4-deployed-source-arrival-clustering-update` with docs-first checkpoint
`cad191e3` and implementation commit `f888487b`. Claimed branch-local proof:
the catch-all `sourcecycled-live` mega-article fallback is removed; existing
cluster IDs are reused only when grouped sources overlap prior source IDs;
unchanged `state_refreshed` groups do not resynthesize unrelated existing
articles; semantic signatures are capped/cleaned; deployed-shaped regression
`TestHandleInternalSourcecycledWebCapturesKeepsDeployedShapedArrivalsSeparated`
passed with focused and broader Universal Wire runtime selectors. Do not
incorporate until the independent verifier returns `accept`.

ledger file: `docs/mission-overnight-autoradio-platform-checklist-v0.ledger.md`

version / lineage: v0 created after email-freeze landing. It supersedes loose
queue ordering from `docs/worktree-review-2026-06-23.md` for overnight execution
order while preserving that report as evidence.

learning state: Thread tools are exposed and usable in this environment. The
Parallax skill now measures conjecture descent, not obligation count; each pass
must produce a strong, clear, definitive statement or buy observer evidence. The
main O4 learning is that local route tests repeatedly missed staging product
edges: publication eligibility, platformd synchronization, current-head reads,
revision-list envelope shape, supplied-revision fallback, stale edition state,
Texture open behavior, and legacy/current semantic DTO projection all required
deployed product replay. The current product is better but still below the
owner-stated Universal Wire target: deterministic grouping and formulaic prose
are only a substrate for a future semantic live world model. Product proof for
semantic observability must include legacy/current article fallback behavior,
not only fresh branch-local synthesis fixtures. The next hard proof is
source-arrival update behavior, not DTO existence.

settlement: not settled. O4 is accepted for deployed deterministic
multi-article readability, direct stale-edition read repair, and current/stale
semantic DTO observability through `a10254d2072c8cc63c910551f3d1fb588fe87605`:
CI/deploy, health identity, and authenticated product replay support those
narrow claims. The actual News benchmark remains open until authenticated
staging evidence shows multilingual live ingestion producing coherent English
synthesis Texture articles over durable semantic source/world-model objects,
with later relevant sources updating existing articles. O5 has started through
prompt-bar/Texture/Super request evidence; O5-O8 remain open. Exit requires
`settled`, `open_handoff`, `blocked`, or `superseded` with remaining V and next
assignment explicit.

## Suggested Goal String

```text
Use Parallax on docs/mission-overnight-autoradio-platform-checklist-v0.md. Treat it as the source program for the thread-native mission. Current status is working with V=3 conjectures, not obligation count. Each pass must decide a conjecture with a strong definitive statement or buy observer evidence. Root deployed `a10254d2072c8cc63c910551f3d1fb588fe87605`; CI run `28281421752`, Docs Truth Check run `28281421753`, FlakeHub run `28281421747`, deploy job `83798061615`, and health identity passed, with proxy and sandbox reporting that exact deployed commit. Authenticated Chrome/Computer Use proof loaded `https://choir.news/api/universal-wire/stories` in the owner's signed-in Chrome session and showed 12 stories, `semanticStoryCount: 12`, literal `semantic_story`, first-story schema `choir.universal_wire_story_cluster.semantic.legacy.v1`, change type `legacy_revision_projection`, `source_count: 24`, and `copyLeak: false`; UI smoke showed 12 readable articles, Texture v66, `Sources 24`, native source buttons, and `Document loaded`. This settles only semantic DTO observability for current/stale synthesized Wire articles. Current C6 source-arrival update conjecture: if a later sourcecycled source arrives for an already-materialized Universal Wire semantic story, the product path should revise the same semantic story object and linked Texture article instead of creating a duplicate card, losing prior citations, or requiring manual reseeding. Sourcecycled staging evidence now shows the real blocker: latest cycle `cycle_aab51c4b894bba17afea9fb2` fetched 562 new items and wrote 561 runtime graph captures, but a sandbox route diagnostic of `/api/universal-wire/stories` still returns one broad article with `change_type: source_added`, unchanged 24-source count, broad topics `energy/harbor/health`, and 273 signal concepts. Chrome authenticated replay is still needed but was blocked by an extension UI in this pass; do not count the sandbox-header diagnostic as product acceptance. Docs-first blocker checkpoint `98f38396fa08c0f94a26d55e362433b160d06864` is pushed and Docs Truth Check run `28282334682` passed. Worker thread `019f07f3-7ddc-7e72-8b9f-e26bc4af1833` returned `ready_for_verifier` on branch `codex/o4-deployed-source-arrival-clustering-update` with docs-first checkpoint `cad191e3` and implementation commit `f888487b`; verifier pending handle `local:60ba30c4-6e1c-4c6f-8b7e-8c09f760bf33` owns `O4-deployed-source-arrival-clustering-update-verifier`. Next move is to read verifier verdict, then incorporate only if accepted. Do not claim full Universal Wire: provider/model-quality synthesis, broad semantic clustering, Qdrant, production update semantics, and full News benchmark settlement remain open. Use Codex app thread tools when exposed: list_projects/create_thread for bounded workers/verifiers, read_thread/list_threads to reconnect verdicts, send_message_to_thread for follow-ups/callbacks, handoff_thread/get_handoff_status only for ownership transfer, and set_thread_title/set_thread_pinned/set_thread_archived for hygiene. Each worker/verifier assignment must name the conjecture it will decide, mutation class, protected surfaces, admissible evidence, rollback path, heresy delta, callback target, and stop condition. Follow AGENTS.md and Problem Documentation First. Behavior-changing landings require commit, push, CI, deploy identity, staging acceptance, verifier evidence, rollback refs, and residual risks. Update Parallax State in place and append to docs/mission-overnight-autoradio-platform-checklist-v0.ledger.md after each material pass. Exit only as settled, open_handoff, blocked, or superseded with remaining V and next assignment explicit.
```
