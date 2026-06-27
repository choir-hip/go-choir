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

Current staging evidence: deployed `4f8cae7a1f9b5217533c1196fecc69e6bd68257c`
incorporates the accepted deterministic split/update slice. CI run
`28278039956`, Docs Truth Check run `28278039951`, FlakeHub run `28278039978`,
deploy job `83788697055`, and public health identity all passed for that SHA;
health reports proxy and sandbox `commit`/`deployed_commit` equal to
`4f8cae7a1f9b5217533c1196fecc69e6bd68257c`, deployed at
`2026-06-27T04:07:47Z`. Authenticated Computer Use replay in the owner's
signed-in Chrome tab showed ordinary Texture still loading with
`Document loaded`, Universal Wire rendering `5 articles`, and multiple headline
opens producing nonblank Texture article windows with version controls, source
counts, native source-ref/source buttons, rendered article text, and
`Document loaded`. Public unauthenticated `/api/universal-wire/stories` still
returns 401 as expected.

Current repair boundary: the deployed product now supports the narrower
conjecture that deterministic graph-backed source groups can publish multiple
readable Texture-backed Universal Wire articles after CI/deploy. It does not
claim the full Universal Wire product: staging still shows helper-like prose,
some visibly incoherent deterministic clusters, no production-quality
provider/model synthesis, no Qdrant/world-model projection, and no deployed
evidence that later relevant sources update existing articles.

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
statement. Current value: 4. Last decided conjectures: CI/deploy accepted
`4f8cae7a1f9b5217533c1196fecc69e6bd68257c`; public health shows proxy and
sandbox serving that deployed commit; authenticated Computer Use replay in the
owner's signed-in Chrome tab proves ordinary Texture still loads, Universal Wire
renders `5 articles`, and headline opens produce nonblank Texture article
windows with source counts, native source buttons, rendered article text, and
`Document loaded`; the semantic News conjecture is weakened because staged
articles still contain helper prose and visibly incoherent deterministic
clusters. Prior decided conjecture: verifier thread
`019f0733-283d-7be3-abc4-61e1f33fbdf9` supports worker commit `44893c3e` for the
branch-local deterministic split/update slice, incorporated in root as
`5efdcd45`. Current live conjectures are: C6 later relevant sources update
existing articles/world-model identities at deployed product scope; C7 synthesis
quality becomes article-like English, not helper prose; C8 semantic/world-model
clustering replaces bounded deterministic grouping; C9 O5-O8 still embed the
Autoradio benchmark into durable Choir product paths.

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

mutation class / protected surfaces: Current move is green documentation-first
checkpoint for the next O4 realism gap. The next O4 implementation move is
orange/red if it changes Universal Wire runtime story selection, objectgraph
cluster state, Texture document/revision writes, public story route semantics,
or synthesis policy. Authorized protected surfaces are Universal Wire story
DTOs, runtime synthesis/article materialization, existing article revision/upsert
semantics, Texture revisions through existing helpers, source entity/source_ref
projection, Wire edition linkage, and read-only Texture publication surfaces for
platform-owned Wire articles. It must not touch auth/session renewal, vmctl,
deployment routing, provider/gateway credentials, Qdrant, promotion/rollback,
run acceptance, or publication/export outside existing Wire edition helpers.

evidence packet: Behavior-changing settlement needs pushed commit SHA, CI run,
deploy status, staging health/build identity, deployed acceptance, verifier
thread id/verdict, rollback refs, mutation class, protected surfaces, heresy
delta, conjecture delta, residual risks, and next realism axis. Branch/local
worker proof may stop at focused tests, `git diff --check`, commit SHA, dirty
classification, residual risks, and non-claims. Docs-only moves need diff
hygiene and Docs Truth Check if pushed.

heresy delta: `repaired` at deployed product scope for the Universal Wire
zero-article regression caused by proxy/platformd sync dropping the supplied
runtime-owned current revision after full-history sandbox fetch missed.
`repaired` also remains for prior read-owner, platformd current-head, revision
envelope, and source-copy regressions recorded in the ledger. `discovered`
remains for semantic production cluster selection, provider/model synthesis
quality, source freshness, live world-model maintenance, update-existing-article
semantics, and the fact that the deployed product currently exposes multiple
deterministic helper-style synthesis articles rather than the intended semantic
multi-article world model.

position / live conjectures / open edges: O0-O3 are accepted from prior ledger
evidence. O4 is no longer blocked on the immediate deployed article-readability
failure: root documented the proxy/platformd supplied-revision sync miss in
`7c9db378`, repaired it in `7f3b42b6`, recorded independent verifier acceptance
from thread `019f070c-85d3-7b51-ba0f-0c22a66e542a` in `cb79fa39`, pushed that
SHA to `origin/main`, monitored CI/deploy/health to success, and ran
authenticated Chrome/Computer Use product replay. The replay proved ordinary
Texture loading, one Universal Wire article, headline-to-Texture rendering,
source affordances, and platformd rows for document
`d3661377-4731-4617-a351-63236b08597d`. Root then documented the next O4
realism gap at the ledger tail: current code still routes eligible captures
through the single stable `sourcecycled-live` cluster, so deployed proof of one
readable article is not proof of multiple semantically clustered English
synthesis articles or same-article updates for later relevant sources. Root
pushed docs-first checkpoint `2b324eb6`, Docs Truth Check `28277468193` passed,
and root requested worker `O4-deterministic-story-clustering-slice-worker`.
The pending worktree handle
`local:0d6a1c85-5367-481b-953c-0b7070774214` materialized as Codex thread
`019f0728-81cb-7193-9f0d-e65f3263768f` in worktree
`/Users/wiz/.codex/worktrees/1fe8/go-choir`. The worker returned
`ready_for_verifier` for commit
`44893c3eab7cedd8d3e41c6c953fd51d32b68ff5`, claiming deterministic
pre-synthesis story grouping over graph-backed captures. Independent verifier
thread `019f0733-283d-7be3-abc4-61e1f33fbdf9` accepted that branch-local claim:
real grouping before synthesis, two unrelated groups -> two clusters/docs/Wire
transclusions, later related source -> same article revised, raw captures remain
diagnostic-only, and source invariants preserved. Root incorporated the worker
commit as `5efdcd45` and reran the focused and broader runtime selectors
successfully. Root then pushed `4f8cae7a` to `origin/main`; CI run
`28278039956`, deploy job `83788697055`, Docs Truth Check run `28278039951`,
and FlakeHub run `28278039978` passed; public health reports proxy and sandbox
serving deployed commit `4f8cae7a1f9b5217533c1196fecc69e6bd68257c`; authenticated
Computer Use replay in the owner's signed-in Chrome tab showed Universal Wire
rendering `5 articles` and open Texture article windows with source counts,
source buttons, rendered body text, and `Document loaded`. This supports the
deployed deterministic multi-article readability conjecture and weakens the
stronger semantic News conjecture because the visible product still uses helper
prose and deterministic groupings. Root then documented the next O4 realism
gap: the deployed surface has readable Texture-backed cards, but the core News
conjecture remains false until live ingested sources are clustered by shared
story/world-model signals and synthesized into article-quality English that
does not narrate Universal Wire's helper mechanics. This problem is documented
before any code change that touches clustering, provider/model synthesis, or
Texture update semantics.

Root created bounded Codex worker request
`O4-semantic-source-clustering-article-quality-slice-worker`; pending worktree
handle `local:429785dd-7621-45a1-91de-6ae793a91bac` materialized as Codex
thread `019f074f-13a2-7793-a02e-16b6bf0a45fc`, titled
`Improve source clustering quality`, in worktree
`/Users/wiz/.codex/worktrees/29be/go-choir`. Read-thread status showed it was
active, had edited `internal/runtime/sourcecycled_web_captures.go`,
`internal/runtime/wire_synthesis.go`, and
`internal/runtime/universal_wire_test.go`, had focused cluster/materialization
tests passing, and was running the broader Universal Wire runtime selector.
The worker then returned `ready_for_verifier` for commit
`880c3ac5021e86395a98551123e0f503f9c1a70e` (`Refine Universal Wire
source-aware synthesis slice`), with focused and broader runtime selectors,
`git diff --check`, and clean worktree evidence.

Root requested independent verifier
`O4-semantic-source-clustering-article-quality-slice-verifier`; pending worktree
handle is `local:dd0dbdad-5135-493c-bf12-794f8aefa21a`. At the time of this
state update, `list_threads` had not yet returned a materialized verifier
thread id.

next move: reconnect to pending verifier handle
`local:dd0dbdad-5135-493c-bf12-794f8aefa21a`; if it accepts worker commit
`880c3ac5021e86395a98551123e0f503f9c1a70e`, decide root incorporation. If it
returns `revise_before_continue`, `blocked`, or `supersede`, record the verdict
and choose the next discriminator from the evidence.

ledger file: `docs/mission-overnight-autoradio-platform-checklist-v0.ledger.md`

version / lineage: v0 created after email-freeze landing. It supersedes loose
queue ordering from `docs/worktree-review-2026-06-23.md` for overnight execution
order while preserving that report as evidence.

learning state: Thread tools are exposed and usable in this environment. The
Parallax skill now measures conjecture descent, not obligation count; each pass
must produce a strong, clear, definitive statement or buy observer evidence. The
main O4 learning is that local route tests repeatedly missed staging product
edges: publication eligibility, platformd synchronization, current-head reads,
revision-list envelope shape, and supplied-revision fallback all had to be
proven through deployed product replay. The latest staging learning is that a
bounded deterministic concept-grouping layer can survive CI/deploy and produce
multiple readable Texture-backed Wire articles, but this still does not satisfy
the Universal Wire product claim: article prose remains helper-like, clusters can
mix unrelated sources, and no deployed proof shows later relevant source
arrivals revising an existing world-model article.

settlement: not settled. O4 is accepted for deployed deterministic multi-article
readability at `4f8cae7a`: CI/deploy, health identity, and authenticated product
replay support that narrow claim. The actual News benchmark remains open until
authenticated staging evidence shows multilingual live ingestion producing
coherent English synthesis Texture articles over durable semantic
source/world-model objects, with later relevant sources updating existing
articles. O5 has started through prompt-bar/Texture/Super-request evidence;
O5-O8 remain open. Exit requires `settled`, `open_handoff`, `blocked`, or
`superseded` with remaining V and next assignment explicit.

## Suggested Goal String

```text
Use Parallax on docs/mission-overnight-autoradio-platform-checklist-v0.md. Treat it as the source program for the thread-native mission. Current status is working with V=4 conjectures, not obligation count. Each pass must decide a conjecture with a strong definitive statement or buy observer evidence. Root deployed `4f8cae7a1f9b5217533c1196fecc69e6bd68257c`; CI run `28278039956`, Docs Truth Check run `28278039951`, FlakeHub run `28278039978`, deploy job `83788697055`, and public health identity passed, with proxy and sandbox reporting that exact deployed commit. Authenticated Computer Use replay in the owner's signed-in Chrome tab proved ordinary Texture still loads, Universal Wire renders `5 articles`, and headline opens produce nonblank Texture article windows with version controls, source counts, native source buttons, rendered article text, and `Document loaded`. This supports the scoped deployed conjecture that deterministic graph-backed source groups can publish multiple readable Texture-backed Universal Wire articles. It weakens the stronger News conjecture: staged articles still use helper-style prose, deterministic clusters can be visibly incoherent, and there is no deployed proof of semantic/world-model clustering, provider-quality synthesis, or later relevant source arrivals revising an existing article. Independent verifier thread `019f0733-283d-7be3-abc4-61e1f33fbdf9` accepted worker commit `44893c3eab7cedd8d3e41c6c953fd51d32b68ff5` for the branch-local deterministic split/update slice; root incorporated it as `5efdcd45` and root focused/broader runtime selectors passed before the push. The next O4 realism gap is documented before code: deployed Wire needs source-aware story/world-model clustering and article-quality English synthesis, not deterministic helper prose. Worker thread `019f074f-13a2-7793-a02e-16b6bf0a45fc` returned `ready_for_verifier` for commit `880c3ac5021e86395a98551123e0f503f9c1a70e` in `/Users/wiz/.codex/worktrees/29be/go-choir`. Root requested independent verifier `O4-semantic-source-clustering-article-quality-slice-verifier`; pending worktree handle is `local:dd0dbdad-5135-493c-bf12-794f8aefa21a`. Next move: reconnect to the pending verifier and record its verdict before root incorporation. Use Codex app thread tools when exposed: list_projects/create_thread for bounded workers/verifiers, read_thread/list_threads to reconnect verdicts, send_message_to_thread for follow-ups/callbacks, handoff_thread/get_handoff_status only for ownership transfer, and set_thread_title/set_thread_pinned/set_thread_archived for hygiene. Each worker/verifier assignment must name the conjecture it will decide, mutation class, protected surfaces, admissible evidence, rollback path, heresy delta, callback target, and stop condition. Follow AGENTS.md and Problem Documentation First. Behavior-changing landings require commit, push, CI, deploy identity, staging acceptance, verifier evidence, rollback refs, and residual risks. Update Parallax State in place and append to docs/mission-overnight-autoradio-platform-checklist-v0.ledger.md after each material pass. Exit only as settled, open_handoff, blocked, or superseded with remaining V and next assignment explicit.
```
