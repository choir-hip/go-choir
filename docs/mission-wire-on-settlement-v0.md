# Mission M5 — Wire on Settlement (cutover step 6; the route-switch evidence gate) — v0

Source: `docs/mission-portfolio-2026-06-11.md` §M5. Program:
`docs/choir-rearchitecture-durable-actors-2026-06-11.md` (cutover step 6,
§2.3). Spec: `specs/wire_pipeline.tla` (sits above actor_protocol.tla;
invariants SuppressedImpliesPublished, EditionHonest, SettledSound; liveness
EveryItemSettles, EditionConverges — model-checked in CI). Discipline:
`skills/parallax/SKILL.md`. Predecessor: M1
(`docs/mission-trajectory-model-v0.md`, settled 2026-06-12) — publication
trajectories, work items, the obligations query, and `runs.trajectory_id`
all exist. M2–M4 are explicitly NOT prerequisites: settlement accounting
does not require the messaging cutover.

## Source form (from the portfolio, verbatim intent)

**Real artifact:** `sourcecycled` reconciles on trajectory settlement instead
of `isTerminalRuntimeState && ActiveChildRuns == 0` (main.go:590); processor
opens publication trajectories carrying coverage/publish decisions; `maxProc`
raised above 1.

**Bridge conjecture:** the wire_pipeline.tla result transfers — with durable
decisions and settlement accounting, parallel processors publish with zero
accounting leaks, retiring the serialization stopgap on evidence rather than
hope. *Falsifier:* a multi-story cycle at maxProc > 1 with a publication
accounting leak, or a trajectory that settles while coagent work is still
mutating its artifact (the settlement-rule edge from N2′).

**Settlement:** one real multi-story production cycle, parallel processors,
front page honest and full, settlement queryable. **This run is the evidence
gate for calling the rearchitecture's core claim supported.**

**Dependencies:** M1. **Size:** 1 overnight mission + 1 observed production
cycle.

## Parallax State

status: blocked (2026-06-12; route-switch substrate and the
independent-review metadata-merge fix are committed, pushed, CI-green, and
deployed to staging at `4b4562a2`; product-path wire-cycle proof is narrowed
but blocked on an authenticated owner session; public publication/retrieval
surfaces are live but do not expose the cycle predicate; the production
maxProc>1 evidence gate remains open. Blocking authority: owner. Smallest
discharge: open or provide an authenticated owner session on
`https://choir.news`, then run one real Universal Wire product-path cycle.)

**mission conjecture:** if publication trajectories carry coverage/publish
decisions as durable work items and subject refs, settlement is evaluated
transactionally from its rule-as-data, and sourcecycled reconciles on that
settlement instead of run-tree liveness, then parallel processors
(maxProc > 1) publish with zero accounting leaks — and the rearchitecture's
core claim (durable causality replaces parent/child control, on evidence)
is supported by a production run, not a document.

**deeper goal (G):** durable actors, evidence-bearing promotion, and
self-development operational instead of documentary (portfolio G). M5 is
the program's first production falsifier — the portfolio names it the
highest-information pairing with M1.

**witness/spec (A/S):** the reconcile flip in sourcecycled; work items
opened/completed at publication decision points; `publish_ref` recorded
into trajectory subject refs at publish commit; settlement status flipped
inside the transactions that could change the verdict; `maxProc > 1` in
production. Spec: wire_pipeline.tla invariants, transferring via runtime
conformance only.

**invariants / qualities / domain ramp (I/Q/D):**
- I: settlement is earned, never polled into existence — evaluation happens
  in the same transactional step as the state change that could flip the
  verdict (glossary: settlement). No publication is suppressed except
  against the published corpus (SuppressedImpliesPublished). The front
  page never lists an unpublished story (EditionHonest).
- Q: "front page honest and full" is the product-visible quality bar — no
  missing stories, no duplicates, after a parallel cycle.
- D ramp, explicitly staged: (1) unit/example tests on settlement
  transitions; (2) a local multi-story cycle with parallel processors
  against a real store; (3) staging; (4) **one observed production cycle —
  the evidence gate**. The claim is asserted only at the domain reached;
  a green local cycle is not the gate.

**variant (ranking function) V:** count remaining settlement blockers:
missing product-path staging wire-cycle proof; missing honest-and-full
instrument; missing observed production multi-story cycle at maxProc>1;
undecided processor-phase admission scope outside the story-route branch;
unspecified non-fetch deferred wake policy; missing final verdict on the
rearchitecture core claim. Current V=6. Last observed ΔV: +1 from fixing the
independent review falsifier: `PatchRevisionMetadata` now uses
`Store.jsonPatchMu`, and
`TestVTextRevisionMetadataConcurrentMergePatchesPreserveKeys` verifies
concurrent revision metadata patches preserve all keys. Last observed ΔV: 0
from Chrome owner-session probe: the visible `https://choir.news/` page still
renders the signed-out preview (`Local preview - sign in to save`), so the
product-proof observer is still missing. The pushed deployed stack now ends at
`4b4562a2`; CI run `27449221402` passed and `/health` reports proxy+sandbox
build/deployed commit `4b4562a2e01549291a3ff2080ec2a187ef5f365f`.

**budget:** next run should assume one owner-authenticated product-proof
session plus one production-cycle observation window. Solvency verdict:
landing is done; the next uncertainty is authenticated product proof and
whether source traffic/cycle timing can produce a real maxProc>1 cycle.

**authority / bounds:** repo changes on a branch; production observation
requires deploy — note the deploy gate is currently red on two known
pre-existing CI failures (M1 settlement, residual 1; accepted by founder
2026-06-12 as known-heresy fallout). The production cycle is an owner-
attention gate per the portfolio (resource edge: owner attention, not
agent capacity). Landing proof (commit, push, CI, deploy identity, the
observed cycle's receipts) required in this document before settlement.

**position / live conjectures / open edges:** M5a's substrate work has
locally produced durable publication obligations, per-source-item processor
decisions, successful `settled` publication trajectories, `cancelled`
terminal non-publication branches, deferred request projection, zero
`ActiveChildRuns` control reads in sourcecycled, and aligned story-route
processor admission between sourcecycled and runtime. Local checks are green:
`nix develop -c go test ./internal/runtime ./internal/cycle ./internal/store ./cmd/sourcecycled`;
`nix develop -c go test -tags comprehensive -count=1 ./internal/runtime`;
`nix develop -c go vet ./...`; `nix develop -c go vet -tags comprehensive ./...`;
`git diff --check`. Current metadata-merge fix receipts:
`nix develop -c go test ./internal/store -run TestVTextRevisionMetadataConcurrentMergePatchesPreserveKeys -count=1`
passed; `nix develop -c go test ./internal/store -count=1` passed in
`56.994s`; `nix develop -c go test ./internal/runtime -run 'TestWire|TestVText|TestTrajectory|TestProcessor' -count=1`
passed; and `nix develop -c go test ./internal/runtime ./internal/cycle ./internal/store ./cmd/sourcecycled -count=1`
passed (`internal/runtime` 22.374s, `internal/cycle` 2.825s,
`internal/store` 60.112s, `cmd/sourcecycled` 5.561s); after that, the wider
local gates `nix develop -c go test -tags comprehensive -count=1 ./internal/runtime`
passed in `102.177s`, `nix develop -c go vet ./...` passed, and
`nix develop -c go vet -tags comprehensive ./...` passed. Review fixes now
serialize Store JSON merge patches within one Store instance and preserve
projected request verdicts during stale runtime-submission recovery. Remote
CI/deploy evidence now exists: push CI `27449221402` for `4b4562a2` passed
all gates, deploy job `81140982346` succeeded in 21s, FlakeHub publish run
`27449221388` succeeded, and `/health` reports proxy+sandbox deployed commit
`4b4562a2e01549291a3ff2080ec2a187ef5f365f` at
`2026-06-12T23:37:50Z`. Public staging probe on that deployed commit showed
`/api/universal-wire/stories` and `POST /api/prompt-bar` both return 401
without auth, while `/api/platform/retrieval/search?q=wire` returns 15 public
publication results and public export for
`/pub/vtext/climate-change-raises-bilateral-trade-costs-through-maritime-shipping-disruption-boe-research-fi-pub09e4bf037`
returns publication `pub-09e4bf03-7cf8-43ea-88f1-191c6f68bc1b`, version
`pubver-1b8910c7-ab8e-43e5-9570-346ea94e35ca`, Markdown content length
`4390`, `private_material_omitted=true`, and source revision hash
`9a1f53d16ada1e0bd3f1683b11ba16a04995695325c00bbf90d120aadbcb1fa1`.
The same public search also shows duplicate-looking titles with distinct
publication ids and source revision hashes; that is a next honest-and-full
discriminator for the authenticated edition/front-page proof, not yet an M5
accounting verdict. Chrome owner-session probe after landing still rendered
the signed-out preview (`Local preview - sign in to save`). Open edge:
product-path cycle proof is blocked because authenticated owner APIs are
unavailable; public platformd corpus health cannot settle the production
route-switch conjecture.

**next move:** owner opens or provides an authenticated session on
`https://choir.news`; then resume from this paradoc and run the smallest
product-path wire-cycle observation that can link a real Universal Wire cycle
to trace/vtext/publication/front-page receipts and show source traffic,
sourcecycled cycle timing, honest-and-full front-page instrumentation,
duplicate/stale publication handling, and production maxProc policy. Do not
use internal/test routes to compensate for missing auth.

**ledger file:** `docs/mission-wire-on-settlement-v0.ledger.md` for future
append-only Parallax pass entries. Historical passes before this checkpoint
remain embedded below under `ledger / move log` and should not be
transcribed unless auditing requires it.

**suggested resume goal string:** `Use Parallax on docs/mission-wire-on-settlement-v0.md. Treat it as the M5 paradoc and source program. Resume from the Parallax State and append to docs/mission-wire-on-settlement-v0.ledger.md. Current status is blocked, V=6: the revision-metadata JSON merge serialization gap found by independent review is fixed, pushed, CI-green, and deployed at 4b4562a2e01549291a3ff2080ec2a187ef5f365f; staging /health identity is proven; public platformd publication search/export still works; /api/universal-wire/stories and POST /api/prompt-bar are auth-gated without an owner session; Chrome still shows the signed-out preview. First obtain an authenticated owner session on https://choir.news. Then run only browser-public product paths to observe the smallest Universal Wire cycle evidence available: session/provenance proof, trace/vtext/publication/front-page receipts, sourcecycled cycle timing, duplicate/stale-publication interpretation, and whether a real multi-story cycle at maxProc>1 can be observed. Do not use /api/agent, /internal, /api/test, raw event mutation, or manual success seeding. If auth, source traffic, or instrumentation blocks the proof, update the paradoc/ledger with exact blocker receipts and next discriminator. Do not call M5 settled without production multi-story maxProc>1 evidence, honest-and-full front-page proof, rollback refs, and a verdict on the durable-actors core claim.`

### Position — code inventory (compiled 2026-06-12, post-M1)

1. **The condition to retire:**
   `reconcileSubmittedProcessorRequests` (cmd/sourcecycled/main.go:567 ff.)
   polls `getRunStatus` per submitted request and can now move
   `runtime_status` off `submitted` from one durable processor-phase
   certificate directly: completed
   `processor_resolution.resolution_state=all_source_items_decided_with_story_route`
   no longer waits on terminal `run.State`. The remaining run-state fallback
   is narrower still: terminal `run.State` now chiefly covers unresolved /
   deferred bookkeeping plus genuine failed/cancelled runtime exits
   (main.go:590). Direct request verdict branches no longer read
   `ActiveChildRuns`, and story-route processor-capacity release no longer
   does either. The flip remains: reconcile on the trajectory's settlement
   (status / obligations from durable state), not the run tree.
2. **Two serialization knobs, only one is "maxProc":**
   - runtime overload guard: `maxProc := 1` default,
     `RUNTIME_MAX_PROCESSOR_RUNS` env override (api.go:934–943) — **this is
     the stopgap that retires on evidence**;
   - sourcecycled dispatch backpressure: `maxProcessorRequests`
     (main.go:201, applied ~:615–640) — flow control, stays.
3. **What M1 already provides:** processor spawns mint
   kind=`publication` trajectories with settlement rule
   `{RequireNoOpenWorkItems: true, RequiredSubjectRefs: ["publish_ref",
   "edition_ref"]}`
   (internal/runtime/trajectory.go); `TrajectoryObligations` +
   `EvaluateTrajectorySettlement` (pure); `GET /api/trajectories/{id}`;
   work-item CRUD with fingerprint dedup. Successor M5a has now added
   trajectory-level `publish_ref` / `edition_ref` patching, a
   processor-opened `wire_story_resolution` work item, the narrower
   publish/edition work item, and per-source-item processor decision work
   items. Successful publication now does reach `status=settled`, the
   published-corpus branch now reaches `status=cancelled`, and explicit
   terminal no-story outcomes now have a parallel `cancelled` path while
   `deferred` remains open. The remaining witness is the honest general
   reconcile rule over those terminal and nonterminal branches.
4. **Publication mutation path actually observed (Q1 probe, 2026-06-12):**
   `edit_vtext` writes the story revision
   (internal/runtime/tools_vtext.go:444–483), then immediately calls
   `maybeAutonomousPublishWireArticle`. That path does:
   `publishWireArticleToPlatform` → `persistWirePlatformPublicationRef`
   (patches revision metadata only; no trajectory write) →
   `autonomousPublishWireArticleToEdition` (writes a new edition revision
   and advances the edition doc head). This is the concrete publish/edition
   path that must grow publication work items, trajectory subject refs, and
   settlement evaluation. The remaining control surface to map is the
   coverage/suppression decision before story publication.
5. **The spec's shape constrains the witness:** wire_pipeline.tla §
   `Settle(s)` — "Settlement is earned: published, listed, nothing left to
   do" (spec line ~154); SettledSound — a settled trajectory is published
   AND in the edition. The runtime's settlement transition must be the
   conformance image of `Settle`, with the work-item ledger as "nothing
   left to do".
6. **What sourcecycled can actually observe today (Q3 probe, updated by
   successor pass 10, 2026-06-12):** `GET /internal/runtime/runs/{id}` now
   returns run state, `ActiveChildRuns`, metadata, and a trajectory-obligation
   surface (`trajectory_id`, trajectory status, settlement-ready bit,
   waiting-on reasons, open-work-item count)
   (`internal/runtime/api.go`, `cmd/sourcecycled/main.go`). So the remaining
   route-switch question is no longer "is there an admissible observer
   surface?" but "which predicate on that surface is honest to consume?".
7. **The transactional substrate is split across stores (SHIFT receipt,
   2026-06-12):** trajectories/work items live in the runtime store
   (`internal/store/trajectory.go`, backed by `s.db`), while story revisions
   and edition head changes live in the VText store
   (`internal/store/vtext.go`, `CreateRevision`/`UpdateDocument` over
   `s.vtextHandle()`). There is no cross-store transaction primitive, and no
   helper that can atomically span `{publish_ref / edition_ref / work-item
   state / trajectory status}` and
   `{story revision metadata / edition revision + head}`. A literal
   one-transaction witness is therefore not constructible on the current
   substrate.
8. **Coverage/suppression decisions are not durable objects yet (SHIFT
   receipt, 2026-06-12):** `BuildIngestionHandoff`
   (`internal/cycle/ingestion_handoff.go`) only batches source items into
   processor requests with source item ids, ingestion event ids, a processor
   key, and a prompt. No durable record carries "opened", "attached", or
   "suppressed as already covered" before processor/VText behavior runs.
   So the mission's source form is still ahead of the repo not just in
   settlement wiring, but in the existence of explicit coverage decisions.
9. **Successor M5a ruled out a false shortcut (SHIFT receipt, 2026-06-12):**
   the repo does have durable trajectory-scoped `worker_updates`
   (`internal/runtime/tools_worker_update.go`, `internal/store/store.go`),
   and processor/reconciler prompts ask agents to use them for checkpoints.
   But those updates are optional, owner-addressed, and prose-shaped. Their
   presence can show a checkpoint happened; their absence cannot honestly mean
   "already covered" or "suppressed". So the missing decision ledger is still
   missing; it cannot be faked by reinterpreting worker-update absence as a
   verdict.
10. **Successor M5a also narrowed the ledger root (PROBE receipt, 2026-06-12):**
   the publication trajectory already spans processor, processor-spawned VText
   revision runs, and downstream worker updates
   (`internal/runtime/runtime.go`, `internal/runtime/tools_coagent.go`,
   `internal/runtime/trajectory_test.go`, `internal/runtime/agent_tools_test.go`,
   `internal/runtime/vtext_test.go`). So the remaining decision ledger does
   not need a second causality root; the honest question is now the typed
   obligation/decision shape on that existing publication trajectory.
11. **Successor M5a has now landed a first typed downstream obligation
    (CONSTRUCT receipt, 2026-06-12):** processor-opened VText handoff now
    creates a durable `wire_story_resolution` work item on the inherited
    publication trajectory, and successful autonomous publish completes it
    together with the narrower publish/edition work item
    (`internal/runtime/tools_coagent.go`,
    `internal/runtime/wire_publication.go`). Focused local tests passed, but
    this is still substrate-level proof only: suppression-as-already-covered
    remains untyped and the route-switch / production evidence gate stays here.
12. **Successor M5a has now also covered the no-VText branch
    (CONSTRUCT receipt, 2026-06-12):** processor run start now opens a durable
    `wire_processor_request_resolution` work item on the publication
    trajectory. That item stays open even if the processor never opens VText,
    so missing suppression/non-publication verdicts now remain visible as open
    obligations instead of disappearing behind terminal run state
    (`internal/runtime/runtime.go`,
    `internal/runtime/wire_publication.go`). Focused local tests passed; this
    is still substrate-only evidence.
13. **Successor M5a has now added a first explicit typed processor verdict
    surface (CONSTRUCT receipt, 2026-06-12):** processor runs can now record
    request-scoped non-publication outcomes such as `already_covered` with
    `record_wire_processor_decision`, and processor → VText handoff now
    records `opened_vtext` while completing the generic request item in favor
    of the more specific `wire_story_resolution` item
    (`internal/runtime/tools_wire_processor.go`,
    `internal/runtime/tools_coagent.go`,
    `internal/runtime/wire_publication.go`,
    `internal/store/trajectory.go`). Focused local tests passed, but this is
    still substrate-only evidence: the new verdict is request-scoped, not yet
    the per-item suppression settlement M5's gate eventually needs.
14. **Successor M5a has now pushed that decision surface down to explicit
    source-item scope (CONSTRUCT receipt, 2026-06-12):** processor run start
    now opens one `wire_source_item_resolution` work item per source item;
    non-publication processor verdicts complete those per-item items; and
    multi-item processor → VText handoffs must now name exact
    `source_item_ids`, so `opened_vtext` is recorded only on the covered
    source items rather than silently over the whole batch
    (`internal/runtime/runtime.go`,
    `internal/runtime/tools_coagent.go`,
    `internal/runtime/tools_wire_processor.go`,
    `internal/runtime/wire_publication.go`). Focused local tests passed, but
    this is still substrate-only evidence: the remaining gap is now the
    trajectory-level terminal path for all-suppressed requests, not item
    identity or verdict shape.
15. **Successor M5a has now added a first honest terminal certificate for
    one suppression branch (CONSTRUCT receipt, 2026-06-12):**
    `already_covered` now requires `covered_by_doc_id` pointing to a
    published VText, records that publication evidence (`covered_by_doc_id`
    + route path) on the per-source-item/request details, and when every
    source item in the batch resolves that way the generic request item
    completes and the publication trajectory moves to `cancelled`
    (`internal/runtime/tools_wire_processor.go`,
    `internal/runtime/wire_publication.go`,
    `internal/runtime/wire_processor_decision_test.go`). Focused local tests
    passed, but this is still substrate-only evidence: `cancelled` is a
    non-publication terminal certificate, not publication settlement, and the
    route-switch gate still lacks terminal semantics for the other
    non-publication verdicts plus its eventual consumer rule.
16. **Successor M5a has now also landed the narrower observer surface on the
    payload sourcecycled already polls (CONSTRUCT receipt, 2026-06-12):**
    `GET /internal/runtime/runs/{id}` now includes trajectory obligations
    (`trajectory_id`, trajectory status, settlement-ready bit, waiting-on
    reasons, open-work-item count), and `sourcecycled`'s local decode shape
    understands that payload even though it does not consume the new fields
    yet (`internal/runtime/api.go`, `internal/runtime/api_test.go`,
    `cmd/sourcecycled/main.go`, `cmd/sourcecycled/main_test.go`). Focused
    local tests passed. This is still substrate-only evidence: the observer
    blind spot is narrower, but the route-switch gate still has to decide
    which predicate on that surface is honest to consume.
17. **Successor M5a has now pushed that observer surface one step closer to
    the actual consumer rule (CONSTRUCT receipt, 2026-06-12):**
    the same internal run-status payload now also exposes the durable
    processor request-resolution certificate (`work_item_id`, request-item
    status, `resolution_state`, source-item counts, `last_decision`,
    `story_doc_id`, `covered_by_doc_id`) from request start through the
    published-corpus-coverage terminal branch
    (`internal/runtime/api.go`,
    `internal/runtime/api_test.go`,
    `internal/runtime/wire_publication.go`,
    `cmd/sourcecycled/main.go`). Focused local tests passed. This is still
    substrate-only evidence: the later reconcile flip still has not chosen
    whether that request-resolution certificate, trajectory status, or a
    composite of both is the honest predicate.
18. **That same observer surface now reveals a projection mismatch on the
    consumer's own ledger (SHIFT receipt, 2026-06-12):** `sourcecycled`
    still maps any terminal runtime run with `state=cancelled` to
    processor-request status `dispatch_failed`
    (`cmd/sourcecycled/main.go`), while the runtime now uses publication-
    trajectory `status=cancelled` plus processor
    `resolution_state=all_source_items_suppressed_against_published_corpus`
    as an honest already-covered terminal certificate
    (`internal/runtime/wire_publication.go`,
    `internal/runtime/api_test.go`,
    `internal/runtime/wire_processor_decision_test.go`). The cycle-side
    `processor_requests` ledger currently only carries
    `queued/submitted/completed/deferred/dispatch_failed/superseded`
    (`internal/cycle/storage.go`). So the remaining route-switch problem is
    narrower and sharper: it is not only which runtime predicate to wait
    for, but how that predicate projects into sourcecycled's own request
    vocabulary without collapsing honest suppression into failure.
19. **The repo now also shows that this projection issue is latent in the
    future route-switch, not yet active on today's run-state poll path
    (PROBE receipt, 2026-06-12):** publication `TrajectoryCancelled` is set
    from the processor decision ledger only
    (`internal/runtime/wire_publication.go`), and the current runtime run
    lifecycle has no corresponding transition keyed off trajectory
    cancellation (`internal/runtime/runtime.go`; `internal/runtime/api.go`).
    `sourcecycled` still reconciles submitted requests only from terminal
    `run.State` + `ActiveChildRuns`
    (`cmd/sourcecycled/main.go`). So the all-covered branch is not currently
    being misclassified because a cancelled *run* is observed; the sharper
    route-switch obligation is that once sourcecycled starts consuming
    trajectory/request-resolution state, it must introduce an explicit
    projection from that trajectory-side terminal certificate into the
    cycle-side request ledger rather than assuming the run lifecycle already
    encodes it.
20. **That projection path is further constrained because
    `processor_requests.status` currently serves two jobs at once (SHIFT
    receipt, 2026-06-12):** the same `submitted` status is both
    `sourcecycled`'s request-lifecycle handle for later reconciliation
    (`ListSubmittedProcessorRequests`) and its active backpressure estimate
    (`CountRecentlySubmittedProcessorRequests`) that limits new processor
    submissions (`cmd/sourcecycled/main.go`,
    `internal/cycle/storage.go`). So a naive settle flip that projects
    published-corpus coverage to a terminal request state as soon as the
    trajectory cancels would also stop counting the still-running processor
    loop against the in-flight cap. The remaining consumer-side problem is
    therefore not just "which terminal verdict maps to which request
    status?" but "how are request verdict accounting and live resource
    accounting separated or jointly represented without lying on either
    axis?".
21. **That dual-use accounting split is now a real substrate instead of a
    named obstacle (CONSTRUCT receipt, 2026-06-12):**
    `processor_requests` now carries a separate `runtime_status` alongside
    request `status`; sourcecycled's submitted-request reconciliation and
    in-flight backpressure now read `runtime_status='submitted'` rather than
    overloading the request verdict field; and the latest source-service
    handoff surface now exposes both fields
    (`internal/cycle/storage.go`,
    `internal/cycle/storage_test.go`,
    `cmd/sourcecycled/main.go`,
    `cmd/sourcecycled/main_test.go`,
    `internal/sourceapi/types.go`). Focused local tests passed. This is still
    route-side substrate only: sourcecycled has not yet switched to
    trajectory/request-resolution terminal predicates, and publication
    `settled` remains unwired.
22. **One narrow trajectory/request-resolution consumer branch is now real in
    sourcecycled itself (CONSTRUCT receipt, 2026-06-12):** while runtime
    capacity remains keyed by `runtime_status`, sourcecycled now projects the
    composite branch
    `{trajectory.status=cancelled, processor_resolution.status=completed,
    resolution_state=all_source_items_suppressed_against_published_corpus,
    covered_by_doc_id!=empty}` to request verdict `completed` even before the
    underlying runtime run is terminal
    (`cmd/sourcecycled/main.go`,
    `cmd/sourcecycled/main_test.go`). Focused local tests passed. This is
    still not the general settle flip: sibling non-publication branches still
    lack explicit consumer semantics, and at this point publication
    `status=settled` was still unwired.
23. **A first publication-side consumer branch now survives runtime completion
    without falling back to run-tree liveness (CONSTRUCT receipt,
    2026-06-12):** sourcecycled now keeps polling requests whose verdict is
    still `submitted` even after `runtime_status` leaves `submitted`, and it
    projects the publication branch
    `{run.State=completed, ActiveChildRuns=0, trajectory.settlement_ready=true}`
    to request verdict `completed`
    (`internal/cycle/storage.go`,
    `cmd/sourcecycled/main.go`,
    `cmd/sourcecycled/main_test.go`). Focused local tests passed, including
    the two-step case where a runtime run completes before settlement-ready
    becomes true and sourcecycled repolls it later. This is still not full
    settlement-status consumption: the route uses the trajectory
    `settlement_ready` observer because runtime still does not stamp
    publication trajectories to `status=settled`.
24. **The publication-side route now has a real settled-status branch instead
    of only a readiness observer (CONSTRUCT receipt, 2026-06-12):**
    successor M5a now stamps successful publication trajectories to
    `status=settled`, and sourcecycled's publication-side completion branch
    now accepts either `{trajectory.status=settled}` or the earlier
    `{trajectory.settlement_ready=true}` observer under terminal
    `run.State=completed`
    (`internal/runtime/wire_publication.go`,
    `internal/runtime/wire_publication_test.go`,
    `cmd/sourcecycled/main.go`,
    `cmd/sourcecycled/main_test.go`). Focused local tests passed. This is
    still a transition-safe partial flip rather than the full route switch:
    the compatibility branch remains, and sibling non-publication branches
    still need explicit terminal/consumer semantics.
25. **One sibling non-publication branch now also has an explicit request
    projection instead of hanging forever (CONSTRUCT receipt, 2026-06-12):**
    successor M5a now cancels publication trajectories whose source items all
    resolve to explicit terminal no-story decisions, while keeping
    `deferred` under a distinct open resolution state; sourcecycled now
    projects the composite branch
    `{trajectory.status=cancelled, processor_resolution.status=completed,
    resolution_state=all_source_items_decided_without_story_route}` to
    request verdict `completed` without releasing the runtime slot early
    (`internal/runtime/wire_publication.go`,
    `internal/runtime/wire_processor_decision_test.go`,
    `internal/runtime/api_test.go`,
    `cmd/sourcecycled/main.go`,
    `cmd/sourcecycled/main_test.go`). Focused local tests passed. This is
    still not the full consumer rule: `deferred` remains intentionally open,
    and the publication-side compatibility branch still accepts
    `settlement_ready`.
26. **The still-open `deferred` branch now also has a bounded request-ledger
    image instead of being repolled forever (CONSTRUCT receipt,
    2026-06-12):** once a processor run is terminal and its request-
    resolution certificate says
    `all_source_items_deferred_without_story_route`, sourcecycled now marks
    the processor request verdict `deferred` while preserving
    `runtime_status=completed`, so the branch stops re-polling the same
    finished run but still remains visibly nonterminal
    (`cmd/sourcecycled/main.go`,
    `cmd/sourcecycled/main_test.go`). Focused local tests passed. This is
    still not a full resumption protocol: the request now has an honest
    longer-lived image, but the future wake/requeue rule for `deferred`
    remains open.
27. **The first concrete wake path for `deferred` is now continuity refresh,
    not self-repolling the finished run (CONSTRUCT receipt, 2026-06-12):**
    when a later cycle emits a new queued processor request on the same
    `continuity_ref`, sourcecycled's continuity supersession now marks older
    `deferred` requests `superseded` alongside older queued requests instead
    of leaving the deferred request as the live open branch forever
    (`internal/cycle/storage.go`,
    `internal/cycle/storage_test.go`). Focused local tests passed. This is
    still not a complete wake policy: it covers source-fetch-driven
    successor requests, not owner- or reconciler-driven resumption.
28. **The settled publication-success branch now consumes the trajectory
    certificate directly instead of waiting for run-tree liveness
    (CONSTRUCT receipt, 2026-06-12):** sourcecycled now projects
    `trajectory.status=settled` to request verdict `completed` even while the
    underlying runtime run is still `running` with active child runs, while
    still preserving `runtime_status='submitted'` so the capacity slot is not
    released early
    (`cmd/sourcecycled/main.go`,
    `cmd/sourcecycled/main_test.go`). Focused local tests passed. This is
    still not the full settle flip: the fallback
    `trajectory.settlement_ready` compatibility branch still waits on
    terminal `run.State=completed` + `ActiveChildRuns==0`.
29. **The remaining publication compatibility tail is now at least
    observable instead of silent (PROBE receipt, 2026-06-12):**
    sourcecycled now distinguishes direct `trajectory.status=settled`
    completion from the older
    `{run.State=completed, ActiveChildRuns=0, trajectory.settlement_ready}`
    fallback and emits an explicit log receipt if that legacy fallback is
    used
    (`cmd/sourcecycled/main.go`). Focused local tests for both the direct
    settled branch and the compatibility branch still passed. This does not
    delete the fallback; it turns an invisible assumption into a measurable
    branch for later landing/staging evidence.
30. **The publication-success compatibility tail is now deleted, not merely
    observable (CONSTRUCT receipt, 2026-06-12):** sourcecycled's
    publication-side request completion branch now accepts only
    `trajectory.status=settled` and no longer carries the older
    `{run.State=completed, ActiveChildRuns=0, trajectory.settlement_ready}`
    fallback
    (`cmd/sourcecycled/main.go`, `cmd/sourcecycled/main_test.go`). Focused
    local tests still passed for the direct settled branch, the terminal
    non-publication branches, the deferred branch, and the unresolved-after-
    runtime-completion path. This is still not the full settle flip: the
    reconciler continues to read `run.State` / `ActiveChildRuns` for
    `runtime_status` updates and failure accounting, and non-fetch deferred
    resumption remains open.
31. **Sourcecycled no longer uses `ActiveChildRuns` to hold processor
    capacity after the processor run itself is terminal (CONSTRUCT receipt,
    2026-06-12):** `reconcileSubmittedProcessorRequests` now moves
    `runtime_status` off `submitted` as soon as `run.State` is terminal,
    even if child runs are still active, while leaving the request verdict
    `submitted` until a later durable trajectory certificate settles it
    (`cmd/sourcecycled/main.go`, `cmd/sourcecycled/main_test.go`). Focused
    local tests still passed for the direct settled branch, the terminal
    non-publication branches, the deferred branch, the unresolved-after-
    runtime-completion path with active children, and the new capacity proof
    that a completed processor run frees sourcecycled backpressure despite
    live child runs. This is still not full settlement-only reconcile: the
    route now has zero `ActiveChildRuns` control reads in sourcecycled, but
    it still reads terminal `run.State` for runtime bookkeeping and failure
    accounting.
32. **The story-route processor phase can now release sourcecycled capacity
    before terminal `run.State` (CONSTRUCT receipt, 2026-06-12):**
    when the durable processor request-resolution certificate is
    `{status=completed, resolution_state=all_source_items_decided_with_story_route}`,
    `reconcileSubmittedProcessorRequests` now moves `runtime_status` to
    `completed` even if the processor run itself is still `running` with
    active child runs, while leaving the request verdict `submitted` until a
    later publication-settlement certificate arrives
    (`cmd/sourcecycled/main.go`, `cmd/sourcecycled/main_test.go`). Focused
    local tests still passed for the direct settled branch, the terminal
    non-publication branches, the deferred branch, the unresolved-after-
    runtime-completion path, and a new proof that a completed story-route
    processor-resolution frees sourcecycled backpressure despite live child
    runs. This is still not full settlement-only reconcile: run-state
    bookkeeping remains for deferred/unresolved branches and genuine failed /
    cancelled exits.
33. **That early story-route release is not yet the same thing as real
    runtime admission capacity (PROBE receipt, 2026-06-12):** when
    sourcecycled frees its local processor slot from the completed
    story-route request-resolution certificate while the processor run itself
    still remains `running`, the next queued submission can still hit the
    runtime overload guard's `429 Too Many Requests` response because the
    runtime continues to count the still-running processor run by profile
    (`internal/runtime/api.go`, `cmd/sourcecycled/main_test.go`). The
    sourcecycled client then burns through its transient retry loop before
    giving up, while leaving the queued request queued and the live request at
    `{status=submitted, runtime_status=completed}`. This does not refute pass
    29's local bookkeeping change; it sharpens the remaining realism gap:
    sourcecycled's local processor-capacity predicate and the runtime's
    overload predicate are now provably different.
34. **The same mismatch is now proven against the real runtime handler, not
    just a sourcecycled mock (PROBE receipt, 2026-06-12):** a real processor
    run can have a completed durable request-resolution item with
    `resolution_state=all_source_items_decided_with_story_route` while its run
    record still remains `running`, and `HandleInternalRunSubmission` will
    still return `429 Too Many Requests` for the next processor submission
    because the runtime overload guard counts that running processor by
    profile (`internal/runtime/api.go`, `internal/runtime/api_test.go`).
    This upgrades the evidence class on `cross_cap_mismatch`: it is no longer
    merely a sourcecycled retry artifact, but a verified disagreement between
    the route-side capacity predicate and the runtime's own admission
    predicate.
35. **That story-route admission mismatch is now closed at the runtime
    boundary too (CONSTRUCT receipt, 2026-06-12):** the runtime overload
    guard now stops counting a `running` processor against
    `RUNTIME_MAX_PROCESSOR_RUNS` once its durable processor request-resolution
    item is completed with
    `resolution_state=all_source_items_decided_with_story_route`
    (`internal/runtime/runtime.go`, `internal/runtime/api_test.go`). The
    sourcecycled story-route branch now succeeds against the same runtime
    predicate instead of burning through transient `429` retries
    (`cmd/sourcecycled/main_test.go`). This does not generalize every branch:
    it aligns the proven story-route handoff case while the remaining
    deferred/unresolved and failure surfaces stay on their narrower existing
    predicates.

Blind spots from this position (edge classes named):
- **substrate_split:** the current witness asks for a transaction the repo
  cannot literally perform today because the decisive state is split across
  the runtime DB and the VText DB. If this mission constructs against the
  current topology without naming the split, it will fake atomicity and
  claim stronger settlement than the substrate can bear. The next route
  choice is architectural, not just wiring: unify the transaction domain, or
  weaken the witness to an explicitly two-phase durable protocol with a
  named residual inconsistency window.
- **frame_lock (the N2′ edge, now load-bearing):** v1's settlement rule
  can be *vacuously satisfied* — a publication trajectory with zero work
  items and a recorded publish_ref settles even if coagent work is still
  mutating the artifact. The portfolio's falsifier names exactly this. The
  discharge: work items must be opened at the moment work begins (coverage,
  publish, edition, coagent revision), not retrofitted at completion — if
  any mutation path lacks an obligation record, the settlement claim is
  dishonest. First control interval: enumerate every mutation path on a
  publication artifact and show its obligation record.
- **missing_oracle:** "front page honest and full" needs an instrument —
  a deterministic post-cycle check (story count vs fetched items, dedup,
  published-vs-listed diff), not eyeballing. Build it before the cycle.
- **terminal_semantics:** the successor now proves that one all-suppressed
  branch can end honestly as `cancelled`, but M5 still has to decide what the
  later route-switch consumes: settlement only, or terminal trajectory status
  with an explicit distinction between `settled` publication and `cancelled`
  non-publication. Successful publication trajectories now do stamp
  `status=settled`, the published-corpus branch now cancels, and explicit
  terminal no-story outcomes now cancel too. The remaining gap is narrower:
  `deferred` still intentionally stays open, and M5 still has to state one
  honest general terminal rule over `{settled, cancelled, deferred}` rather
  than treating publication success as a special compatibility case.
- **consumer_rule:** the earlier "what can sourcecycled see?" blind spot is
  now narrower because the internal run-status payload already carries both
  trajectory obligations and the processor request-resolution certificate.
  What remains undecided is the actual consumer rule: whether reconcile
  should wait for `trajectory.status == settled`, accept `cancelled` for
  published-corpus coverage branches, or require a more specific
  request-resolution predicate or composite. One such composite is now real
  for the published-corpus `already_covered` branch, the publication-success
  branch now consumes only `trajectory.status=settled`, and a second such
  composite is now real for explicit terminal no-story outcomes. The
  remaining question is how to state the final general predicate once for all
  terminal branches, and what explicit wake/resumption rule the now-bounded
  `deferred` branch should participate under.
- **status_projection:** even with the right runtime-side predicate,
  sourcecycled's current processor-request vocabulary still has no honest
  image for "terminal because the published corpus already covers this
  batch." Today that mismatch is latent rather than active: the current
  reconcile loop still watches only run-state terminality, not trajectory
  terminality. But once the route switches to durable settlement state, it
  cannot simply reuse `completed/dispatch_failed` without an explicit
  projection rule. The new `runtime_status` split removes the earlier field
  overload, and the coverage-backed `cancelled` branch now projects to
  `completed` with focused proof. Publication `status=settled` now also
  projects to `completed` with focused proof. Explicit terminal no-story
  `cancelled` now also projects to `completed` with focused proof, and the
  intentionally open `deferred` branch now projects to request `deferred`
  once its runtime run is terminal. The remaining projection work is
  narrower: `deferred` still needs a future wake/resumption rule, not a first
  request-ledger image.
- **remaining_run_state_reads:** sourcecycled now has zero
  `ActiveChildRuns` control reads, and one positive processor-phase branch
  (`all_source_items_decided_with_story_route`) can release
  `runtime_status` from durable request-resolution state alone. What still
  depends on terminal `run.State` is narrower: deferred/unresolved
  bookkeeping plus genuine failed/cancelled runtime exits. Exit for M5 still
  asks the same sharper question at smaller scope: is that residual
  run-state bookkeeping an honest processor-capacity/failure surface, or
  should the final route-switch witness replace it with a trajectory-side
  capacity/failure protocol too?
- **processor_phase_scope:** story-route processor-phase completion now frees
  capacity in both sourcecycled and the runtime admission guard from the same
  durable request-resolution certificate. What remains undecided is the scope
  of that alignment: should other completed processor-resolution terminal
  branches such as published-corpus coverage or explicit no-story also release
  processor admission before terminal `run.State`, or is the earlier release
  only honest for the story-route handoff case?
- **deferred_resumption:** the branch is now honest and has one concrete wake
  path: a later source cycle on the same `continuity_ref` supersedes the old
  deferred request with a fresh queued successor. What remains missing is the
  rest of the authority story: owner- or reconciler-driven wake without a
  new source-fetch successor is still unspecified.
- **independence:** the TLA+ result is universal over the model only;
  production transfer is exactly what this mission tests. Never report
  the spec's ∀ as the system's.
- **resource:** maxProc > 1 raises real concurrency on the platform
  computer; the overload guard exists because it wedged before. Ramp:
  2 processors first, bounded cycle, watch; then portfolio-normal.

### Initial conjectures

- **C1 (bridge):** with durable decisions + settlement accounting,
  parallel processors publish with zero accounting leaks. *Test:* the
  staged D ramp ending in one production multi-story cycle at maxProc > 1
  with the honest-and-full instrument green. *Falsifier:* any accounting
  leak (lost story, duplicate, suppressed-without-published-cover) in
  that cycle.
- **C2 (no vacuous settlement):** no trajectory settles while any path is
  still mutating its artifact. *Test:* mutation-path enumeration with an
  obligation record per path (the frame_lock discharge above), plus an
  adversarial test that starts a coagent revision and asserts the
  trajectory shows nonzero obligations until it completes. *This inherits
  M2's C4 oracle shape — build it here, M2 reuses it.*
- **C3 (reconcile equivalence):** flipping sourcecycled to
  settlement-based reconcile loses no dispatch accounting — every
  processor request reaches one honest terminal request state exactly once,
  and no earlier than its trajectory's terminal predicate (`settled`
  publication or an explicitly justified non-publication terminal
  certificate such as published-corpus `cancelled`), while the in-flight
  resource budget remains conservative until the underlying runtime work is
  actually no longer consuming processor capacity. *Test:* dispatcher tests
  over publication-settled, coverage-backed-cancelled, and genuine failure
  branches during the transition window; one probe that shows early request
  completion does not prematurely release the backpressure slot; and one
  two-step proof that a request can remain unresolved after runtime
  completion, then complete later from the trajectory observer without
  reintroducing `ActiveChildRuns`; grep-level zero reads of `ActiveChildRuns`
  in sourcecycled at exit.
- **C4 (stopgap retires on evidence):** RUNTIME_MAX_PROCESSOR_RUNS
  default flips above 1 only after C1's production cycle — the evidence
  order is the point; raising it first would be hope, not evidence.

### Open questions (after pass 3 split)

- **Q1:** where should durable coverage/suppression decisions live? The
  current ingestion handoff layer does not materialize them; if publication
  trajectories are meant to "carry coverage/publish decisions", the route
  now moves to successor
  `docs/mission-wire-settlement-substrate-v0.md` before any settle flip is
  honest here. Successor pass 3 further narrows this: existing
  `worker_updates` are not a valid answer because they cannot encode
  suppression by typed verdict. Successor pass 4 narrows it again: whatever
  the answer is, it lives on the existing publication trajectory rather than a
  second ledger root. Successor pass 5 narrows it again: processor-opened
  article work now does open one typed obligation at handoff time, so the
  remaining missing piece is the explicit non-publication/suppression verdict.
  Successor pass 6 narrows this once more: even the no-VText processor branch
  is now durably blocked by an open request-resolution obligation rather than
  disappearing, so the missing piece is the typed verdict itself, not merely
  branch visibility. Successor pass 7 narrows it again: a request-scoped
  typed verdict now exists, so the remaining missing piece is the
  per-source-item suppression settlement shape rather than total absence of a
  typed processor outcome. Successor pass 8 narrows it again: per-source-item
  decision work items now exist and multi-item VText handoffs bind exact
  source-item ids, so the remaining missing piece is the trajectory-level
  terminal protocol for all-suppressed requests rather than item identity or
  verdict shape. Successor pass 9 narrows it again: the
  `already_covered` branch now has a published-evidence-backed terminal
  `cancelled` path, so the remaining missing piece is terminal semantics for
  the other non-publication verdicts and the later route-switch consumer.
- **Q2:** choose the settlement substrate honestly: unify the runtime and
  VText transaction domains, or explicitly weaken the witness to a two-phase
  protocol (`story/edition durable first`, then `trajectory settlement
  catch-up`) with a named residual risk. M1 shipped the evaluator pure and
  unwired on purpose; pass 2 showed this is an architectural choice, not just
  a missing function call. This also moves to successor
  `docs/mission-wire-settlement-substrate-v0.md`.
- **Q3:** what sourcecycled consumes — trajectory status via
  `GET /api/trajectories/{id}` piggybacked on its existing poll loop
  (external observation of durable state is fine; the never-polled rule
  binds *evaluation*, not observers), or settlement surfaced on the run
  status payload it already fetches. Smallest honest change wins, but only
  after the successor mission settles the substrate it will read. Successor
  M5a has now landed trajectory-level `publish_ref`/`edition_ref` plus an
  in-flight publish obligation, and pass 10 further narrows the route by
  landing trajectory obligations on the internal run-status payload itself.
  Pass 11 narrows it again by landing the processor request-resolution
  certificate on that same payload. So the remaining question is no longer
  "which surface can carry the read?" but "which predicate or composite on
  that now-live surface is honest to consume, and how does its terminal
  verdict project into sourcecycled's own `processor_requests.status`
  vocabulary once the route *stops* relying on run-state terminality?" Pass
  20 makes that narrower again: one publication-side branch now works by
  retaining an unresolved request verdict after runtime completion and later
  consuming `trajectory.settlement_ready`, so the remaining question is how
  much farther that observer-level route should extend before M5 forces
  runtime-side `status=settled`. Pass 21 narrows it again: successful
  publication trajectories now really do reach `status=settled`, and
  sourcecycled can consume that terminal predicate while remaining compatible
  with the earlier readiness observer during the transition. Pass 22 narrows
  it again: explicit terminal no-story outcomes now have their own cancelled
  projection while `deferred` remains open. The remaining question is when
  the compatibility branch can be deleted and how the still-open `deferred`
  path participates in the final general rule. Pass 23 narrows it again:
  `deferred` now has an honest request-ledger image once runtime is terminal,
  so the remaining question is no longer "how does the route stop repolling
  this branch?" but "what future event wakes it?". Pass 24 narrows it again:
  a later source cycle on the same `continuity_ref` now supersedes the old
  deferred request, so the remaining question is no longer the
  source-fetch-driven wake path but whether non-fetch authorities need their
  own wake protocol. Pass 25 narrows it again: the direct
  `trajectory.status=settled` branch no longer waits on `run.State` or
  `ActiveChildRuns`, so the remaining question is no longer whether settled
  is strong enough to replace run-tree liveness, but what residual run-tree
  reads remain after the publication compatibility tail disappears. Pass 28
  narrows it again: sourcecycled no longer reads `ActiveChildRuns` for
  control at all, so the remaining question is whether the final
  `run.State` bookkeeping should remain as honest processor-capacity /
  failure accounting or be replaced by a trajectory-side protocol too. Pass
  29 narrows it again: one positive processor-phase branch now frees
  `runtime_status` from completed request-resolution state before terminal
  `run.State`, so the residual run-state question now excludes the
  story-route handoff case and focuses on deferred/unresolved bookkeeping
  plus genuine failed/cancelled exits. Pass 30 sharpens that route again:
  freeing sourcecycled's local processor slot earlier does not yet buy real
  runtime admission, because the runtime overload guard still counts the
  still-running processor run by profile and can return `429` until that run
  leaves `running`. Pass 31 upgrades that route from mocked-route evidence to
  real runtime-handler evidence. Pass 32 narrows it again: that mismatch is
  now closed for the story-route branch, so the remaining capacity-surface
  question is about scope rather than the existence of a disagreement.
- **Q4:** the deploy path for the observed cycle — staging deploy is
  currently gated by the two known pre-existing CI reds (accepted
  baseline). Decide with the founder: step over the gate for this deploy,
  or clear those two tests first. **Escalation point, not silent choice.**

**ledger / move log:**

- 2026-06-12 PROBE (Q1 + Q3; doc/repo read-only scope).
  Claim: the deciding unknown is still C2/C3, not processor parallelism by
  itself — if settlement is vacuous or sourcecycled cannot observe it
  honestly, raising `maxProc` proves nothing.
  Position: publication mutation is currently
  `edit_vtext` → `maybeAutonomousPublishWireArticle` →
  `publishWireArticleToPlatform` → `persistWirePlatformPublicationRef`
  (revision metadata only) → `autonomousPublishWireArticleToEdition`
  (edition revision + doc head). Sourcecycled still reconciles on terminal
  run state + `ActiveChildRuns`.
  Blind spot: there is still no transactional substrate for
  `{open/complete work item + patch trajectory subject refs +
  evaluate settlement + UpdateTrajectoryStatus(settled)}` as one step, and
  the coverage/suppression obligation point remains unmapped.
  Move: probe.
  Bound: no code mutation; inspect only the mission refs, spec, and
  runtime/sourcecycled publication surfaces.
  Update: Q1 is now bounded enough to start the first construct on the real
  publish/edition path; Q3 narrowed to two honest options, and the current
  code implements neither.
  Exit: open_handoff.

- 2026-06-12 SHIFT (vantage + domain; doc/repo read-only scope).
  Claim under test: the "transactional settlement substrate" named in the
  witness might be a wiring gap, not an architectural gap.
  Position: trajectory/work-item state is persisted on the runtime DB while
  story revisions and edition head changes are persisted on the separate
  VText DB; processor handoffs batch source items but do not materialize
  durable coverage/suppression decisions.
  Blind spot reduced: the problem is no longer "find the right helper to
  call." The repo currently lacks both a unified transaction domain and the
  decision objects the mission says trajectories should carry.
  Move: shift.
  Bound: no code mutation; inspect store topology and the ingestion-handoff
  decision surface only.
  Update: the next honest move is not a superficial reconcile flip. It is
  either an architectural construct that unifies or bridges the store split,
  or a narrowed witness that explicitly downgrades the settlement claim to a
  two-phase protocol at a smaller evidence class.
  Exit: open_handoff.

- 2026-06-12 SHIFT (identity + lineage split; docs-only scope).
  Claim under test: this document can still serve as the next executable
  mission despite the repeated `substrate_split` obstacle.
  Position: the source form still correctly names the eventual evidence gate
  (`sourcecycled` on settlement, multi-story cycle, `maxProc > 1`), but the
  repo findings now show two hidden prerequisites: an honest settlement
  substrate across the runtime/VText store split, and durable
  coverage/suppression decision objects.
  Blind spot reduced: continuing this document as if it were the next coding
  target would optimize the wrong object — it would force either fake
  atomicity or a silent broadening of M5 into a substrate mission.
  Move: shift.
  Bound: docs only; preserve the evidence-gate identity here and split the
  substrate obligations into a successor mission.
  Update: successor created at
  `docs/mission-wire-settlement-substrate-v0.md`. This document remains the
  route-switch evidence gate, but it is now an `open_handoff` waiting on
  that successor rather than the next direct construct target.
  Exit: open_handoff.

- 2026-06-12 CONSUME SUCCESSOR (handoff receipt; code landed under M5a).
  Claim under test: the successor split should materially reduce M5's blind
  spot rather than becoming a fake island.
  Position: M5a landed a bounded construct — publication trajectories now
  record `publish_ref` and `edition_ref`, and the publication settlement rule
  requires both refs before readiness.
  Blind spot reduced: M5 no longer waits on a missing trajectory-ref
  substrate. What remains is the harder substrate problem the split was
  meant to isolate: explicit coverage/suppression decision objects,
  settlement protocol choice across the runtime/VText store split, and the
  later sourcecycled read flip.
  Move: construct consumed via open handoff.
  Bound: no new code in this document's scope; record only the successor
  outcome that changes M5's next route.
  Update: successor M5a moved from `proposed` to `open_handoff` with passing
  focused tests. M5 remains deferred until that mission settles the decision
  ledger and protocol choice.
  Exit: open_handoff.

- 2026-06-12 CONSUME SUCCESSOR (handoff receipt; second code construct under M5a).
  Claim under test: successor substrate work should also make failed or
  in-flight publishes legible, not only successful ones.
  Position: M5a has now added a durable publish-path work item that opens
  before external publish begins and completes only after edition linkage
  lands.
  Blind spot reduced: the publish/edition path is no longer invisible while
  in flight. The remaining gap is broader than publish completion: explicit
  coverage/suppression decisions and the cross-store settlement protocol are
  still unresolved.
  Move: construct consumed via open handoff.
  Bound: no new code in M5 scope; record only the successor outcome that
  changes what M5 can now honestly assume.
  Update: successor M5a remains `open_handoff` with another passing focused
  construct. M5 still waits on the broader decision ledger and later read
  flip before it can resume as the evidence gate.
  Exit: open_handoff.

- 2026-06-12 CONSUME SUCCESSOR (handoff receipt; worker-update shortcut ruled out).
  Claim under test: successor substrate work might be able to reuse an
  existing durable surface rather than adding a typed publication decision
  ledger.
  Position: M5a probed the processor/reconciler decision boundary and found
  `submit_coagent_update` plus trajectory-scoped `worker_updates` already
  persisted in the runtime store.
  Blind spot reduced: that surface is durable but not verdict-bearing.
  Because updates are optional and free-form, absence still means "no durable
  checkpoint arrived", not "already covered" or "suppressed". So the route
  cannot honestly skip a typed coverage/suppression ledger.
  Move: construct consumed via open handoff.
  Bound: no new code in M5 scope; record only the successor finding that
  changes what shortcuts are still on the table.
  Update: this mission remains deferred on the same successor, now with one
  false shortcut explicitly ruled out.
  Exit: open_handoff.

- 2026-06-12 CONSUME SUCCESSOR (handoff receipt; publication trajectory root confirmed).
  Claim under test: successor substrate work might still need a second
  causality root for VText/coagent obligations even if the worker-update
  shortcut is false.
  Position: M5a probed trajectory inheritance and the processor → VText →
  worker path. The runtime inherits `trajectory_id` across child runs; the
  processor VText route starts a VText revision run under the processor
  parent; tests already show the VText run and later worker updates stay on
  the same trajectory.
  Blind spot reduced: the remaining ledger is missing, but its root is no
  longer ambiguous. The publication trajectory itself is the honest ledger
  root for downstream article work.
  Move: probe consumed via open handoff.
  Bound: no new code in M5 scope; record only the successor finding that
  changes the shape of the next substrate construct.
  Update: M5 still waits on successor M5a, but the successor's next construct
  is now narrowed to a typed obligation/decision surface on the existing
  publication trajectory.
  Exit: open_handoff.

- 2026-06-12 CONSUME SUCCESSOR (handoff receipt; processor-born story-resolution obligation landed).
  Claim under test: successor substrate work should make processor-opened
  article work visible before external publish starts, not only after the
  narrower publish helper activates.
  Position: M5a used the confirmed publication trajectory root to add a
  higher-level `wire_story_resolution` work item at processor → VText
  handoff, then completes it on successful autonomous publish. Focused local
  tests now show successful publish closes both that item and the nested
  publish/edition item, while failed platform publish leaves both open.
  Blind spot reduced: for processor-born stories that do route into VText,
  the frame-lock edge is narrower because the trajectory now shows in-flight
  article work from handoff time, not only from external publish start.
  Move: construct consumed via open handoff.
  Bound: no reconcile flip, no deployment claim, no explicit suppression
  verdict yet. This is still substrate-only evidence.
  Update: M5 remains deferred on successor M5a, but the missing ledger is now
  smaller and more concrete: explicit non-publication/suppression decisions
  and the later read/settle flip.
  Exit: open_handoff.

- 2026-06-12 CONSUME SUCCESSOR (handoff receipt; no-VText processor branch now visible).
  Claim under test: successor substrate work should also cover the processor
  branch that terminates without opening VText, not only the opened-story path.
  Position: M5a added a request-scoped
  `wire_processor_request_resolution` work item at processor run start.
  That item remains open whether or not the processor later opens VText, so a
  terminal processor run without an explicit verdict now leaves a durable open
  obligation on the publication trajectory.
  Blind spot reduced: the missing suppression ledger is now narrower. The
  branch is no longer invisible; what remains absent is the typed
  non-publication/suppression verdict that would let this request-scoped item
  resolve honestly.
  Move: construct consumed via open handoff.
  Bound: no deployment claim, no reconcile flip, no explicit suppression
  outcome yet. This is still substrate-only evidence.
  Update: M5 remains deferred on successor M5a, but the successor has now
  made both the opened-story path and the no-VText path visible as durable
  obligations on the publication trajectory.
  Exit: open_handoff.

- 2026-06-12 CONSUME SUCCESSOR (handoff receipt; request-scoped typed processor verdicts landed).
  Claim under test: successor substrate work should not stop at branch
  visibility; it should add an explicit typed processor verdict without
  over-claiming suppression settlement.
  Position: M5a extended the generic processor request item with a typed
  verdict surface. Processor → VText handoff now records `opened_vtext` and
  completes the generic request item in favor of the downstream
  story-resolution item, while explicit non-publication outcomes such as
  `already_covered` can be recorded durably with
  `record_wire_processor_decision`.
  Blind spot reduced: this mission no longer waits on total absence of an
  explicit processor verdict. What remains missing is narrower and more
  honest: the verdict is still request-scoped rather than per-source-item,
  and it still does not supply an admissible suppression settlement path for
  the production evidence gate.
  Move: construct consumed via open handoff.
  Bound: no deployment claim, no reconcile flip, no settlement claim. This
  is still substrate-only evidence.
  Update: M5 remains deferred on successor M5a, but the successor has now
  made "explicit typed processor verdict exists" true at request scope while
  keeping non-publication outcomes visibly open.
  Exit: open_handoff.

- 2026-06-12 CONSUME SUCCESSOR (handoff receipt; per-source-item processor decisions landed).
  Claim under test: successor substrate work should not stop at a
  request-scoped verdict; it should bind decisions and story handoffs to
  exact source-item ids on the publication trajectory.
  Position: M5a now opens one source-item decision work item per
  `source_item_id`, requires explicit `source_item_ids` on multi-item
  processor → VText handoffs, and records `opened_vtext` or explicit
  non-publication verdicts on those per-item items.
  Blind spot reduced: this mission no longer waits on item identity or a
  typed per-item decision shape. What remains missing is narrower and more
  honest: an all-suppressed request still has no trajectory-level settle or
  cancel path, so the route-switch evidence gate remains deferred on that
  terminal protocol rather than on source-item attribution.
  Move: construct consumed via open handoff.
  Bound: no deployment claim, no reconcile flip, no settlement claim. This
  is still substrate-only evidence.
  Update: M5 remains deferred on successor M5a, but the successor has now
  made "exact source items resolved by this story or verdict are named
  durably" true.
  Exit: open_handoff.

- 2026-06-12 CONSUME SUCCESSOR (handoff receipt; published-coverage-backed cancellation path landed).
  Claim under test: successor substrate work should not leave the
  all-suppressed branch as a permanently open fake obligation once the route
  can prove every source item is already covered by the published corpus.
  Position: M5a now requires `covered_by_doc_id` for `already_covered`,
  validates that the referenced VText is actually published, persists the
  resulting publication evidence on the per-item/request decision ledger, and
  completes the generic processor request while cancelling the publication
  trajectory when every source item resolves this way.
  Blind spot reduced: this mission no longer waits on total absence of a
  terminal protocol for all-suppressed requests. What remains missing is
  narrower and more honest: `cancelled` is only a non-publication terminal
  certificate for the `already_covered` branch, not publication settlement,
  and the eventual route-switch still needs semantics for the other
  non-publication verdicts plus its consumer rule.
  Move: construct consumed via open handoff.
  Bound: no deployment claim, no reconcile flip, no settlement claim. This
  is still substrate-only evidence.
  Update: M5 remains deferred on successor M5a, but the successor now makes
  "all source items already covered by published corpus" a durable,
  evidence-backed terminal branch rather than an indefinitely open request.
  Exit: open_handoff.

- 2026-06-12 CONSUME SUCCESSOR (handoff receipt; internal run-status trajectory observer landed).
  Claim under test: successor substrate work should reduce M5's observer
  blind spot before the route-switch flips, by exposing trajectory obligations
  on the internal status payload sourcecycled already polls.
  Position: M5a now adds trajectory status, settlement-ready, waiting-on, and
  open-work-item count to `GET /internal/runtime/runs/{id}`, and sourcecycled
  can decode that richer payload without changing behavior yet.
  Blind spot reduced: this mission no longer waits on total absence of an
  admissible observer surface. What remains missing is narrower and more
  honest: the later reconcile flip still needs an explicit consumer rule for
  which predicate on that surface counts as completion vs dispatch failure.
  Move: construct consumed via open handoff.
  Bound: no deployment claim, no reconcile flip, no settlement claim. This
  is still substrate-only evidence.
  Update: M5 remains deferred on successor M5a, but the successor now makes
  "sourcecycled can observe trajectory obligations on the payload it already
  fetches" true.
  Exit: open_handoff.

- 2026-06-12 CONSUME SUCCESSOR (handoff receipt; processor request-resolution observer landed).
  Claim under test: successor substrate work should not stop at abstract
  trajectory obligations; it should expose the concrete request-resolution
  certificate that a later reconcile predicate could actually consume.
  Position: M5a now includes processor request-resolution state on
  `GET /internal/runtime/runs/{id}` and initializes that durable request
  item with `awaiting_source_item_decisions` at processor run start, so the
  payload can surface the certificate immediately rather than only after a
  later reconcile.
  Blind spot reduced: this mission no longer waits on absence of a
  request-resolution observer. What remains missing is narrower and more
  honest: the route-switch still has to decide whether the consumer rule
  keys off request resolution alone, trajectory status alone, or a composite.
  Move: construct consumed via open handoff.
  Bound: no deployment claim, no reconcile flip, no settlement claim. This
  is still substrate-only evidence.
  Update: M5 remains deferred on successor M5a, but the successor now makes
  "sourcecycled can observe both trajectory obligations and the processor
  request-resolution certificate on the payload it already fetches" true.
  Exit: open_handoff.

- 2026-06-12 SHIFT (vantage + vocabulary; sourcecycled request-status projection).
  Claim under test: now that the runtime observer surface exposes both
  trajectory obligations and request-resolution, this mission may be ready
  to choose the reconcile predicate directly.
  Position: the runtime now has an honest `cancelled` terminal certificate
  for fully `already_covered` requests, but sourcecycled still maps any
  terminal runtime `cancelled` state to processor-request `dispatch_failed`,
  and the cycle-side request ledger has no distinct terminal suppression
  state.
  Blind spot reduced: the remaining gap is no longer only "which runtime
  predicate does sourcecycled consume?" It is also "what request-ledger
  state may that predicate project to?" Without that projection rule, even a
  correct runtime predicate still misclassifies published-corpus coverage as
  failure.
  Move: shift.
  Bound: no code mutation; inspect only the runtime/status and
  cycle/request-status surfaces already in repo.
  Update: M5 remains deferred on successor M5a, but the route-switch
  question is now narrower and more precise: choose an honest composite
  predicate and its cycle-ledger projection together.
  Exit: open_handoff.

- 2026-06-12 PROBE (run lifecycle vs trajectory lifecycle; repo read-only scope).
  Claim under test: the newly named projection mismatch may already be
  observable as a bug on today's `sourcecycled` reconcile path.
  Position: `already_covered` now cancels the publication trajectory, but
  that cancellation is recorded only on the trajectory/dependency ledger.
  The current reconcile loop still waits on terminal `run.State` and
  `ActiveChildRuns`, and there is no code path that mirrors
  `TrajectoryCancelled` into a cancelled runtime run.
  Blind spot reduced: the next construct is not "fix current cancelled-run
  misclassification." It is "make the future route-switch introduce an
  intentional projection from trajectory/request-resolution terminal states
  into the cycle request ledger."
  Move: probe.
  Bound: no code mutation; inspect only the current runtime lifecycle and
  sourcecycled reconcile surfaces.
  Update: M5 remains deferred on successor M5a, but the consumer-rule route
  is sharper: keep run terminality and trajectory terminality as separate
  axes until the settle flip explicitly joins them.
  Exit: open_handoff.

- 2026-06-12 SHIFT (vocabulary + domain; request verdict vs resource budget).
  Claim under test: once sourcecycled can observe a trajectory-side terminal
  certificate, it may be able to project that directly into the existing
  `processor_requests.status` field.
  Position: the same `submitted` field is currently used for two different
  control loops: later request reconciliation and current active-concurrency
  backpressure. A terminal projection based on trajectory cancellation can be
  honest for request accounting while still being dishonest for resource
  accounting if the processor run remains alive.
  Blind spot reduced: the next construct is no longer "teach sourcecycled one
  more terminal status." It is "separate or explicitly couple request verdict
  accounting and resource-budget accounting so the settle flip cannot free
  capacity early."
  Move: shift.
  Bound: no code mutation; inspect only the cycle-storage/query and
  sourcecycled dispatch/reconcile surfaces already present in repo.
  Update: M5 remains deferred on successor M5a, but the route-switch
  obligation is now a three-part join: consumer predicate, terminal request
  projection, and conservative resource-budget release.
  Exit: open_handoff.

- 2026-06-12 CONSTRUCT (request verdict vs runtime-capacity split substrate; code + focused tests).
  Claim under test: the route-switch cannot be honest while one
  `processor_requests.status` field simultaneously means both "terminal
  request verdict" and "still occupies an active processor slot".
  Position: pass 17 showed that a future trajectory-driven terminal
  projection could free the backpressure slot early if request verdict and
  runtime-capacity accounting remain fused.
  Blind spot reduced: `processor_requests` now carries a separate
  `runtime_status`, and submitted-request reconciliation plus in-flight
  counting now key off that field while the request `status` remains
  independently available for future terminal projection.
  Move: construct.
  Bound: no settle flip, no trajectory-predicate consumer change, no deploy
  claim. This construct splits the accounting substrate only.
  Update: landed the request/runtime accounting split in
  `internal/cycle/storage.go`, projected it through the latest
  source-service handoff response, and added focused proof that request
  verdict can diverge from runtime-capacity accounting while sourcecycled
  still holds the in-flight slot:
  `nix develop -c go test ./internal/cycle -run 'TestStoragePersistsIngestionHandoffsAndLatestCycleSummary|TestProcessorRequestRuntimeStatusCanDivergeFromVerdictStatus|TestStorageSupersedesQueuedProcessorContinuityAndDependentReconcilers'`
  and
  `nix develop -c go test ./cmd/sourcecycled -run 'TestSourceServiceAPIIngestionHandoffLatestReportsAgentHandoffs|TestIngestionRuntimeDispatcherSubmitsProcessorProfilesOnly|TestIngestionRuntimeDispatcherKeepsRuntimeSubmittedRequestsInFlightAfterVerdictCompletion|TestIngestionRuntimeDispatcherKeepsQueuedRequestOnTransientRuntimeFailure'`.
  Exit: open_handoff.

- 2026-06-12 CONSTRUCT (already-covered consumer projection; code + focused tests).
  Claim under test: once request verdict and runtime-capacity accounting are
  split, sourcecycled may be able to consume at least one honest
  trajectory-side terminal certificate without waiting for the whole settle
  flip.
  Position: pass 18 made it safe to let request verdict diverge from
  runtime-capacity state, but no route-side consumer branch actually used the
  richer trajectory/request-resolution payload yet.
  Blind spot reduced: sourcecycled now projects the published-corpus
  coverage branch to request verdict `completed` when the runtime status
  payload shows trajectory `cancelled` together with the completed
  `all_source_items_suppressed_against_published_corpus` certificate and
  `covered_by_doc_id`, while `runtime_status` remains `submitted` until the
  run truly exits.
  Move: construct.
  Bound: no general settle flip, no publication-settled consumer, no deploy
  claim. This construct covers only one explicit non-publication terminal
  branch.
  Update: landed the route-side published-corpus coverage projection in
  `cmd/sourcecycled/main.go` with focused proof in
  `cmd/sourcecycled/main_test.go`:
  `nix develop -c go test ./cmd/sourcecycled -run 'TestSourceServiceAPIIngestionHandoffLatestReportsAgentHandoffs|TestIngestionRuntimeDispatcherSubmitsProcessorProfilesOnly|TestIngestionRuntimeDispatcherKeepsRuntimeSubmittedRequestsInFlightAfterVerdictCompletion|TestIngestionRuntimeDispatcherProjectsPublishedCorpusCoverageWithoutReleasingBudget|TestIngestionRuntimeDispatcherKeepsQueuedRequestOnTransientRuntimeFailure'`.
  Exit: open_handoff.

- 2026-06-12 CONSTRUCT (publication settlement-ready consumer branch; code + focused tests).
  Claim under test: after the request/runtime accounting split, sourcecycled
  may be able to consume a publication completion branch without requiring
  runtime to stamp trajectory `status=settled` yet.
  Position: pass 19 proved one non-publication terminal branch, but a
  completed processor run whose publication trajectory was still not
  settlement-ready would still be misclassified if sourcecycled collapsed all
  terminal `run.State=completed` cases to request `completed`.
  Blind spot reduced: sourcecycled now continues polling requests whose
  verdict remains `submitted` even after `runtime_status` becomes completed,
  and it projects request verdict `completed` only once the later poll sees
  `trajectory.settlement_ready=true` under terminal run completion.
  Move: construct.
  Bound: no runtime-side `status=settled` stamping, no deploy claim, no
  general consumer flip for every branch. This construct covers one
  publication-side observer route only.
  Update: landed the unresolved-after-runtime-completion publication branch
  in `cmd/sourcecycled/main.go` and added focused proof in
  `cmd/sourcecycled/main_test.go`:
  `nix develop -c go test ./cmd/sourcecycled -run 'TestSourceServiceAPIIngestionHandoffLatestReportsAgentHandoffs|TestIngestionRuntimeDispatcherSubmitsProcessorProfilesOnly|TestIngestionRuntimeDispatcherKeepsRuntimeSubmittedRequestsInFlightAfterVerdictCompletion|TestIngestionRuntimeDispatcherProjectsPublishedCorpusCoverageWithoutReleasingBudget|TestIngestionRuntimeDispatcherKeepsPollingSubmittedVerdictAfterRuntimeCompletes|TestIngestionRuntimeDispatcherKeepsQueuedRequestOnTransientRuntimeFailure'`.
  Exit: open_handoff.

- 2026-06-12 CONSTRUCT (publication `status=settled` branch with transition compatibility; code + focused tests).
  Claim under test: the publication-side consumer should not ossify around
  `settlement_ready` once the substrate can honestly stamp successful
  publication trajectories to `status=settled`.
  Position: pass 20 proved that a request could remain unresolved after
  runtime completion and later complete from the trajectory observer, but the
  observer branch still depended on `settlement_ready` because runtime had no
  successful-publication lifecycle transition.
  Blind spot reduced: successful publication trajectories now do reach
  `status=settled`, and sourcecycled can consume that terminal predicate
  without regressing the transition window because the route still accepts
  the earlier readiness observer during the cutover.
  Move: construct.
  Bound: no deployment claim, no deletion of the compatibility observer yet,
  no general settle flip for every branch. This construct covers only the
  successful publication branch plus the route-side compatibility it needs.
  Update: successor M5a stamped successful publication trajectories to
  `status=settled` in `internal/runtime/wire_publication.go`, and M5 widened
  the publication-side request-completion branch in
  `cmd/sourcecycled/main.go` with focused proof in
  `internal/runtime/wire_publication_test.go` and
  `cmd/sourcecycled/main_test.go`:
  `nix develop -c go test ./internal/runtime -run 'TestWireAutonomousPublishTranscludesEditionAndDebounces|TestWirePlatformPublishFailsClosedWithoutEditionWhenPlatformdFails'`
  and
  `nix develop -c go test ./cmd/sourcecycled -run 'TestIngestionRuntimeDispatcherKeepsPollingSubmittedVerdictAfterRuntimeCompletes|TestIngestionRuntimeDispatcherProjectsPublishedCorpusCoverageWithoutReleasingBudget|TestIngestionRuntimeDispatcherKeepsRuntimeSubmittedRequestsInFlightAfterVerdictCompletion'`.
  Exit: open_handoff.

- 2026-06-12 CONSTRUCT (explicit no-story cancelled projection; code + focused tests).
  Claim under test: once the substrate distinguishes explicit terminal
  no-story outcomes from `deferred`, the route-side consumer should stop
  leaving the terminal no-story branch unresolved forever.
  Position: pass 21 gave the route a settled publication-success branch and a
  published-corpus cancelled branch, but explicit `not_newsworthy` /
  `insufficient_evidence` outcomes still had no request projection while
  `deferred` was not yet visibly distinct.
  Blind spot reduced: explicit terminal no-story outcomes now cancel the
  trajectory and project to request `completed` without releasing the
  runtime-capacity slot early, while `deferred` remains a named open branch
  instead of being silently collapsed into the same terminal image.
  Move: construct.
  Bound: no deployment claim, no deletion of the publication compatibility
  observer, no full consumer rule for all remaining branches. This construct
  covers only the explicit terminal no-story path plus its route-side
  projection.
  Update: successor M5a split terminal no-story from deferred in
  `internal/runtime/wire_publication.go`, and M5 added the matching
  request-projection branch in `cmd/sourcecycled/main.go` with focused proof
  in `internal/runtime/wire_processor_decision_test.go`,
  `internal/runtime/api_test.go`, and `cmd/sourcecycled/main_test.go`:
  `nix develop -c go test ./internal/runtime -run 'TestRecordWireProcessorDecisionToolRecordsPerSourceItemNonPublicationVerdict|TestRecordWireProcessorDecisionToolCancelsExplicitNoStoryTerminalBranch|TestRecordWireProcessorDecisionToolKeepsDeferredBranchOpen|TestHandleInternalRunStatusIncludesProcessorResolutionTerminalBranch|TestHandleInternalRunStatusIncludesExplicitNoStoryTerminalBranch|TestWireAutonomousPublishTranscludesEditionAndDebounces|TestWirePlatformPublishFailsClosedWithoutEditionWhenPlatformdFails'`
  and
  `nix develop -c go test ./cmd/sourcecycled -run 'TestIngestionRuntimeDispatcherProjectsPublishedCorpusCoverageWithoutReleasingBudget|TestIngestionRuntimeDispatcherProjectsExplicitNoStoryTerminalWithoutReleasingBudget|TestIngestionRuntimeDispatcherKeepsPollingSubmittedVerdictAfterRuntimeCompletes|TestIngestionRuntimeDispatcherKeepsRuntimeSubmittedRequestsInFlightAfterVerdictCompletion'`.
  Exit: open_handoff.

- 2026-06-12 CONSTRUCT (deferred request-state projection; code + focused tests).
  Claim under test: once `deferred` is distinguished from terminal no-story
  outcomes, the route-side consumer should stop repolling the same completed
  runtime run forever while still keeping the request visibly nonterminal.
  Position: pass 22 gave explicit terminal no-story outcomes a cancelled →
  completed request projection, but a terminal runtime run paired with
  `all_source_items_deferred_without_story_route` still left the processor
  request at verdict `submitted`, so sourcecycled would keep polling the same
  finished run indefinitely.
  Blind spot reduced: sourcecycled now gives that branch an explicit verdict
  state of `deferred` once runtime is terminal. The request stops being
  reconciled as if it were still active, runtime capacity is honestly free,
  and the branch remains visibly open rather than being collapsed into
  `completed` or `dispatch_failed`.
  Move: construct.
  Bound: no deployment claim, no wake/requeue protocol yet, no deletion of
  the publication compatibility observer, no full consumer rule for all
  branches. This construct covers only the route-side request image for the
  deferred branch.
  Update: landed the deferred request-state projection in
  `cmd/sourcecycled/main.go` with focused proof in
  `cmd/sourcecycled/main_test.go`:
  `nix develop -c go test ./cmd/sourcecycled -run 'TestIngestionRuntimeDispatcherProjectsPublishedCorpusCoverageWithoutReleasingBudget|TestIngestionRuntimeDispatcherProjectsExplicitNoStoryTerminalWithoutReleasingBudget|TestIngestionRuntimeDispatcherMarksDeferredBranchWithoutRepollingForever|TestIngestionRuntimeDispatcherKeepsPollingSubmittedVerdictAfterRuntimeCompletes|TestIngestionRuntimeDispatcherKeepsRuntimeSubmittedRequestsInFlightAfterVerdictCompletion'`.
  Exit: open_handoff.

- 2026-06-12 CONSTRUCT (deferred wake via continuity refresh; code + focused tests).
  Claim under test: once `deferred` has a stable request-ledger image, it
  should not remain the live open branch forever when later source evidence
  produces a successor processor request on the same continuity line.
  Position: pass 23 stopped infinite repolling by marking terminal deferred
  runs as request `deferred`, but continuity refresh still superseded only
  older queued requests. A later cycle on the same `continuity_ref` would
  therefore leave the old deferred request alongside the fresh queued
  successor with no explicit handoff between them.
  Blind spot reduced: continuity refresh now treats older `deferred`
  requests like older queued requests and marks them `superseded` when a
  later cycle emits a fresh queued successor on the same
  `continuity_ref`. The wake path is now explicit for source-fetch-driven
  refresh instead of being left implicit in historical state.
  Move: construct.
  Bound: no deployment claim, no owner/reconciler wake protocol, no full
  general settle flip. This construct covers only the source-fetch-driven
  deferred resumption path.
  Update: landed deferred continuity-refresh supersession in
  `internal/cycle/storage.go` with focused proof in
  `internal/cycle/storage_test.go`, plus route compile proof in
  `cmd/sourcecycled`:
  `nix develop -c go test ./internal/cycle -run 'TestStorageSupersedesQueuedProcessorContinuityAndDependentReconcilers|TestProcessorRequestRuntimeStatusCanDivergeFromVerdictStatus'`
  and
  `nix develop -c go test ./cmd/sourcecycled -run 'TestSourceServiceAPIIngestionHandoffLatestReportsAgentHandoffs|TestIngestionRuntimeDispatcherMarksDeferredBranchWithoutRepollingForever'`.
  Exit: open_handoff.

- 2026-06-12 CONSTRUCT (direct settled-status consumption; code + focused tests).
  Claim under test: once runtime can honestly stamp `trajectory.status=settled`,
  sourcecycled should not keep that branch hostage to `run.State` /
  `ActiveChildRuns` before request verdict accounting can complete.
  Position: pass 24 gave the route a fetch-driven wake path for `deferred`,
  but the publication-success path still only consumed settled data after the
  runtime run itself was terminal, even though `status=settled` is already a
  stronger certificate than run-tree liveness.
  Blind spot reduced: sourcecycled now consumes `trajectory.status=settled`
  immediately for request verdict accounting while leaving
  `runtime_status='submitted'` untouched until the runtime run really exits,
  so accounting no longer waits on `ActiveChildRuns` where the trajectory
  certificate is already decisive.
  Move: construct.
  Bound: no deployment claim, no deletion of the fallback
  `settlement_ready` compatibility branch yet, no full general settle flip.
  This construct covers only the direct settled-status publication-success
  path.
  Update: landed the direct settled-status consumer branch in
  `cmd/sourcecycled/main.go` with focused proof in
  `cmd/sourcecycled/main_test.go`:
  `nix develop -c go test ./cmd/sourcecycled -run 'TestIngestionRuntimeDispatcherProjectsPublishedCorpusCoverageWithoutReleasingBudget|TestIngestionRuntimeDispatcherProjectsExplicitNoStoryTerminalWithoutReleasingBudget|TestIngestionRuntimeDispatcherProjectsSettledTrajectoryWithoutWaitingForRunTree|TestIngestionRuntimeDispatcherMarksDeferredBranchWithoutRepollingForever|TestIngestionRuntimeDispatcherKeepsPollingSubmittedVerdictAfterRuntimeCompletes|TestIngestionRuntimeDispatcherKeepsRuntimeSubmittedRequestsInFlightAfterVerdictCompletion'`.
  Exit: open_handoff.

- 2026-06-12 PROBE (instrument the compatibility tail; code + focused tests).
  Claim under test: after pass 25, the remaining `settlement_ready`
  compatibility tail may already be dead code under current runtime behavior,
  but the route has no receipt if that assumption is false.
  Position: direct `trajectory.status=settled` now completes verdict
  accounting without run-tree liveness, and the only remaining publication
  liveness tail is the older
  `{run.State=completed, ActiveChildRuns=0, trajectory.settlement_ready}`
  fallback.
  Blind spot reduced: sourcecycled now distinguishes the direct settled
  branch from the legacy fallback and logs a receipt when the fallback is
  actually used. The branch is still present, but it is no longer an
  invisible assumption.
  Move: probe.
  Bound: no deploy claim, no deletion of the compatibility fallback, no
  broader settle flip. This pass changes observability only.
  Update: landed explicit compatibility-tail logging in
  `cmd/sourcecycled/main.go`; focused branch proofs still passed:
  `nix develop -c go test ./cmd/sourcecycled -run 'TestIngestionRuntimeDispatcherProjectsSettledTrajectoryWithoutWaitingForRunTree|TestIngestionRuntimeDispatcherKeepsPollingSubmittedVerdictAfterRuntimeCompletes'`.
  Exit: open_handoff.

- 2026-06-12 CONSTRUCT (delete the publication compatibility tail; code + focused tests).
  Claim under test: after pass 26 made the fallback observable and current
  runtime already stamped successful publication trajectories to
  `trajectory.status=settled`, sourcecycled no longer needed the legacy
  `{run.State=completed, ActiveChildRuns=0, trajectory.settlement_ready}`
  publication branch.
  Position: pass 25 already let direct `trajectory.status=settled` complete
  verdict accounting without waiting on run-tree liveness, and the focused
  route proofs no longer needed `settlement_ready` to exercise the
  unresolved-after-runtime-completion path.
  Blind spot reduced: publication-success request completion now consumes only
  the durable trajectory certificate, so the remaining route-switch gap is no
  longer whether the compatibility branch survives but the broader general
  terminal rule and the residual run-tree reads still used for
  runtime-status/failure accounting.
  Move: construct.
  Bound: no deploy claim, no full grep-zero `ActiveChildRuns` cleanup, no
  change to deferred wake policy. This pass deletes only the legacy
  publication fallback.
  Update: deleted the compatibility branch from
  `cmd/sourcecycled/main.go` and kept focused route proofs green in
  `cmd/sourcecycled/main_test.go`:
  `nix develop -c go test ./cmd/sourcecycled -run 'TestIngestionRuntimeDispatcherProjectsPublishedCorpusCoverageWithoutReleasingBudget|TestIngestionRuntimeDispatcherProjectsExplicitNoStoryTerminalWithoutReleasingBudget|TestIngestionRuntimeDispatcherProjectsSettledTrajectoryWithoutWaitingForRunTree|TestIngestionRuntimeDispatcherMarksDeferredBranchWithoutRepollingForever|TestIngestionRuntimeDispatcherKeepsPollingSubmittedVerdictAfterRuntimeCompletes|TestIngestionRuntimeDispatcherKeepsRuntimeSubmittedRequestsInFlightAfterVerdictCompletion'`.
  Exit: open_handoff.

- 2026-06-12 CONSTRUCT (delete the final `ActiveChildRuns` control read; code + focused tests).
  Claim under test: once publication-success verdicts no longer depend on
  run-tree liveness, sourcecycled should stop holding processor backpressure
  on `ActiveChildRuns` too; a completed processor run should free sourcecycled
  capacity even while child runs remain alive.
  Position: after pass 27, the remaining `ActiveChildRuns` dependency sat
  only in terminal run bookkeeping. That meant a processor request could have
  an honest unsettled verdict plus a completed processor run, yet still keep
  sourcecycled's processor slot occupied solely because child runs were
  active.
  Blind spot reduced: sourcecycled now has zero `ActiveChildRuns` control
  reads. The remaining route-switch gap is narrower still: terminal
  `run.State` is the only remaining run-lifecycle input, and it now stands
  alone as either honest processor-capacity/failure bookkeeping or the next
  route-side debt to replace.
  Move: construct.
  Bound: no deploy claim, no delete-all-`run.State` move, no deferred
  authority wake policy change. This pass removes only the `ActiveChildRuns`
  dependency from reconcile bookkeeping.
  Update: deleted the `ActiveChildRuns` gate in
  `cmd/sourcecycled/main.go` and added focused proof in
  `cmd/sourcecycled/main_test.go` that a completed processor run frees
  sourcecycled capacity despite active child runs, while the unresolved
  request still repolls until a later trajectory settlement certificate:
  `nix develop -c go test ./cmd/sourcecycled -run 'TestIngestionRuntimeDispatcherProjectsPublishedCorpusCoverageWithoutReleasingBudget|TestIngestionRuntimeDispatcherProjectsExplicitNoStoryTerminalWithoutReleasingBudget|TestIngestionRuntimeDispatcherProjectsSettledTrajectoryWithoutWaitingForRunTree|TestIngestionRuntimeDispatcherMarksDeferredBranchWithoutRepollingForever|TestIngestionRuntimeDispatcherKeepsPollingSubmittedVerdictAfterRuntimeCompletes|TestIngestionRuntimeDispatcherStopsReadingActiveChildRunsForCompletedProcessorCapacity|TestIngestionRuntimeDispatcherKeepsRuntimeSubmittedRequestsInFlightAfterVerdictCompletion'`.
  Exit: open_handoff.

- 2026-06-12 CONSTRUCT (story-route request-resolution releases processor capacity; code + focused tests).
  Claim under test: if the durable processor request-resolution item says
  `{status=completed, resolution_state=all_source_items_decided_with_story_route}`,
  sourcecycled should not keep the processor slot occupied merely because the
  processor run record is still `running` while downstream VText/publication
  work continues.
  Position: after pass 28, `run.State` was the only remaining sourcecycled
  runtime-capacity control read. That still forced the story-route handoff
  case to wait for run-lifecycle bookkeeping even though the durable
  processor-phase certificate already said the processor had finished its own
  work.
  Blind spot reduced: sourcecycled can now free processor capacity from one
  durable request-resolution certificate directly, without waiting for
  terminal `run.State`. The remaining run-state fallback is narrower:
  deferred/unresolved branches plus genuine failed/cancelled exits.
  Move: construct.
  Bound: no deploy claim, no general delete-all-`run.State` move, no change
  to deferred wake policy, no new terminal request projections. This pass
  covers only the completed story-route processor-phase branch.
  Update: added the story-route runtime-capacity release branch in
  `cmd/sourcecycled/main.go` and focused proof in
  `cmd/sourcecycled/main_test.go` that a completed
  `all_source_items_decided_with_story_route` processor-resolution frees
  sourcecycled backpressure even while the processor run remains `running`
  with active child runs and the request verdict stays unresolved until
  later settlement:
  `nix develop -c go test ./cmd/sourcecycled -run 'TestIngestionRuntimeDispatcherProjectsPublishedCorpusCoverageWithoutReleasingBudget|TestIngestionRuntimeDispatcherProjectsExplicitNoStoryTerminalWithoutReleasingBudget|TestIngestionRuntimeDispatcherProjectsSettledTrajectoryWithoutWaitingForRunTree|TestIngestionRuntimeDispatcherMarksDeferredBranchWithoutRepollingForever|TestIngestionRuntimeDispatcherKeepsPollingSubmittedVerdictAfterRuntimeCompletes|TestIngestionRuntimeDispatcherStopsReadingActiveChildRunsForCompletedProcessorCapacity|TestIngestionRuntimeDispatcherKeepsRuntimeSubmittedRequestsInFlightAfterVerdictCompletion'`.
  Exit: open_handoff.

- 2026-06-12 PROBE (early local capacity release vs runtime processor `429`; focused test proof).
  Claim under test: after pass 29, the story-route branch may already have
  aligned sourcecycled's processor-capacity predicate with real runtime
  admission capacity.
  Position: sourcecycled can now move a story-route request to
  `runtime_status=completed` from the durable processor-resolution
  certificate even while the processor run remains `running`. The remaining
  realism question is whether the runtime overload guard follows that same
  predicate or still keys off the run's `running` state.
  Blind spot reduced: the repo now proves the predicates diverge. A queued
  successor request can be admitted by sourcecycled's local backpressure
  check, then hit repeated runtime `429 Too Many Requests` responses because
  the runtime still counts the earlier processor run by profile while it
  remains `running`.
  Move: probe.
  Bound: no runtime or sourcecycled semantic change, no deploy claim, no new
  projection rule. This pass adds realism evidence only.
  Update: added focused proof in `cmd/sourcecycled/main_test.go` that the
  story-route release branch can still collide with the runtime processor
  guard and consume the transient retry loop before leaving the queued
  request queued:
  `nix develop -c go test ./cmd/sourcecycled -run 'TestIngestionRuntimeDispatcherKeepsQueuedRequestOnTransientRuntimeFailure|TestIngestionRuntimeDispatcherStopsReadingActiveChildRunsForCompletedProcessorCapacity|TestIngestionRuntimeDispatcherStoryRouteCapacityReleaseCanStillHitRuntimeProcessor429|TestIngestionRuntimeDispatcherKeepsPollingSubmittedVerdictAfterRuntimeCompletes|TestIngestionRuntimeDispatcherKeepsRuntimeSubmittedRequestsInFlightAfterVerdictCompletion'`.
  Exit: open_handoff.

- 2026-06-12 PROBE (verify `cross_cap_mismatch` against the real runtime handler; focused tests).
  Claim under test: pass 30's `429` mismatch might still be only a
  sourcecycled/mock-server artifact rather than the runtime's own admission
  predicate.
  Position: the route-side probe already showed repeated transient `429`s once
  sourcecycled freed its local slot from completed story-route request
  resolution while the earlier processor run still remained `running`. What
  was still missing was direct proof that `HandleInternalRunSubmission`
  itself behaves that way against the same durable processor-phase state.
  Blind spot reduced: the mismatch is now proven at the runtime boundary too.
  A real processor run can have completed story-route request resolution while
  still being `running`, and the runtime overload guard will still reject the
  next processor submission with `429`.
  Move: probe.
  Bound: no semantic change to runtime or sourcecycled, no deploy claim, no
  admission-policy rewrite. This pass strengthens evidence only.
  Update: added focused proof in `internal/runtime/api_test.go` and reran the
  route-side probe in `cmd/sourcecycled/main_test.go`:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestHandleInternalRunSubmissionCountsRunningProcessorEvenAfterStoryRouteRequestResolutionCompletes|TestHandleInternalRunStatusIncludesTrajectoryObligations|TestHandleInternalRunStatusIncludesProcessorResolutionTerminalBranch|TestHandleInternalRunStatusIncludesExplicitNoStoryTerminalBranch'`
  and
  `nix develop -c go test ./cmd/sourcecycled -run 'TestIngestionRuntimeDispatcherKeepsQueuedRequestOnTransientRuntimeFailure|TestIngestionRuntimeDispatcherStopsReadingActiveChildRunsForCompletedProcessorCapacity|TestIngestionRuntimeDispatcherStoryRouteCapacityReleaseCanStillHitRuntimeProcessor429|TestIngestionRuntimeDispatcherKeepsPollingSubmittedVerdictAfterRuntimeCompletes|TestIngestionRuntimeDispatcherKeepsRuntimeSubmittedRequestsInFlightAfterVerdictCompletion'`.
  Exit: open_handoff.

- 2026-06-12 CONSTRUCT (align runtime admission with the story-route processor-phase certificate; focused tests).
  Claim under test: once sourcecycled frees local processor capacity from the
  completed story-route request-resolution certificate, the runtime overload
  guard should use that same durable processor-phase predicate rather than
  continuing to count the still-running processor run by profile.
  Position: passes 30–31 proved a real mismatch: sourcecycled could admit the
  next queued processor locally while the runtime still rejected it with `429`
  because the earlier processor run remained `running` even after its durable
  request-resolution item completed on the story-route branch.
  Blind spot reduced: the story-route branch is now aligned end-to-end. The
  runtime admission guard no longer counts a running processor once its
  request-resolution item is completed with
  `all_source_items_decided_with_story_route`, and the route-side proof now
  submits successfully against the same runtime predicate.
  Move: construct.
  Bound: no deploy claim, no general processor-admission rewrite for every
  completed terminal branch, no deferred wake-policy change. This pass aligns
  only the proven story-route mismatch.
  Update: updated `internal/runtime/runtime.go` so the processor overload
  guard stops counting the completed story-route processor-phase branch,
  replaced the real-handler `429` proof in `internal/runtime/api_test.go`
  with an admission proof, and updated the route-side test in
  `cmd/sourcecycled/main_test.go` to assert successful submission under the
  aligned predicate:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestHandleInternalRunSubmissionAdmitsProcessorAfterStoryRouteRequestResolutionCompletes|TestHandleInternalRunStatusIncludesTrajectoryObligations|TestHandleInternalRunStatusIncludesProcessorResolutionTerminalBranch|TestHandleInternalRunStatusIncludesExplicitNoStoryTerminalBranch'`
  and
  `nix develop -c go test ./cmd/sourcecycled -run 'TestIngestionRuntimeDispatcherStopsReadingActiveChildRunsForCompletedProcessorCapacity|TestIngestionRuntimeDispatcherStoryRouteCapacityReleaseAlignsWithRuntimeAdmission|TestIngestionRuntimeDispatcherKeepsPollingSubmittedVerdictAfterRuntimeCompletes|TestIngestionRuntimeDispatcherKeepsRuntimeSubmittedRequestsInFlightAfterVerdictCompletion'`.
  Exit: open_handoff.

**version / lineage:** v0, compiled 2026-06-12 immediately after M1
settlement. Predecessor: M1 (settled; this mission consumes its
publication-kind trajectories, work items, obligations query, and is the
named review point for M1's frame_lock settlement-rule edge). Parallel:
M2 doc exists (`docs/mission-messaging-cutover-v0.md`); the original source
form did not depend on it, and still does not. Successor split:
`docs/mission-wire-settlement-substrate-v0.md` now carries the substrate and
decision-ledger obligations discovered in passes 2–3. This document remains
the route-switch decision / evidence gate after that successor settles.

**learning state:** retained here. Inherited: spawned_by vocabulary
doctrine (no parent/child prose); the vacuous-settlement trap was named at
M1 (N2′ edge) and is this mission's central discharge; the two known CI
reds are accepted baseline, but the deploy gate interaction is Q4's
escalation. New in pass 1: the concrete publish path currently writes only
revision/edition state, not trajectory settlement state, so the first
construct must start by adding an honest transactional settlement substrate
rather than flipping sourcecycled first. New in pass 2: the requested
transactional substrate does not exist on the current store topology, and
coverage/suppression decisions are not yet durable records. The route must
therefore change before code can honestly claim settlement soundness. New in
pass 3: this route change is now preserved as a mission split rather than an
ever-expanding M5; the substrate work lives in the successor mission, while
this document keeps the later evidence-gate claim intact. New in pass 4:
the successor has now landed trajectory-level `publish_ref`/`edition_ref`
substrate with focused tests, reducing M5's blind spot without yet proving
the full settlement protocol. New in pass 5: the successor has made the
publish/edition path visible while in flight via a durable work item, but
that still does not amount to a full publication decision ledger. New in
pass 6: the successor ruled out trajectory-scoped `worker_updates` as that
ledger, because they are optional checkpoint packets rather than typed
coverage/suppression verdicts. New in pass 7: the successor also proved that
processor-spawned VText runs and downstream worker updates already remain on
the publication trajectory, so the missing decision ledger is a shape problem,
not a missing root-object problem. New in pass 8: the successor has now
landed a first typed downstream obligation at processor → VText handoff, so
processor-born article work no longer stays invisible until external publish;
the remaining missing ledger is the explicit non-publication/suppression
verdict. New in pass 9: even a processor request that never opens VText now
leaves a durable open request-resolution obligation, so the remaining gap is
the typed verdict itself rather than branch visibility. New in pass 10: that
typed verdict now exists at request scope, and processor → VText handoff
auto-resolves the generic request item into the downstream story-resolution
item. The remaining gap is narrower but still real: per-item suppression
settlement is still not present, so the route-switch evidence gate remains
deferred. New in pass 11: the successor has now pushed that typed decision
surface down to explicit source-item scope and requires multi-item VText
handoffs to bind exact `source_item_ids`. The remaining gap is narrower
again: not decision shape, but the trajectory-level terminal path for
all-suppressed requests. New in pass 12: the successor now gives the
`already_covered` branch an honest terminal certificate by requiring
published-doc evidence and cancelling the trajectory when every source item
resolves that way. The remaining gap is no longer "can any all-suppressed
branch end honestly?" but "what terminal semantics do the other
non-publication branches use, and what exact terminal predicate will
sourcecycled later consume?". New in pass 13: the successor has now landed
trajectory obligations on the internal run-status payload sourcecycled
already polls, so the observer-surface question is narrowed sharply. The
remaining gap is the consumer rule itself, not the absence of a readable
surface. New in pass 14: that same payload now exposes the processor
request-resolution certificate from request start through the
published-corpus-coverage branch. The remaining gap is narrower again: not
missing observer data, but the honest composition rule over the data that is
now visible. New in pass 15: the remaining gap is narrower still than
"choose a consumer rule." The repo now shows a concrete projection
mismatch: runtime `cancelled` can honestly mean published-corpus coverage,
while sourcecycled still projects any cancelled run to `dispatch_failed`
and its request ledger has no distinct suppression terminal state. New in
pass 16: that mismatch is not yet an active bug on today's reconcile path,
because run terminality and trajectory terminality are still separate axes.
The next construct has to join them intentionally rather than assuming the
run lifecycle already carries the trajectory's coverage-backed terminal
certificate. New in pass 17: even that intentional join cannot reuse the
current `processor_requests.status` field blindly, because the field is
pulling double duty as both request verdict and active backpressure budget.
New in pass 18: that double duty is now split in code. The route-switch no
longer needs to solve verdict projection and resource-budget release with one
field, but it still has to choose the honest trajectory/request-resolution
predicate and the terminal request projection that sits on top of the new
substrate. New in pass 19: one such projection is now real for the
published-corpus `already_covered` branch, and it keeps the runtime-capacity
slot occupied while the underlying processor run is still alive. The
remaining gap is narrower again: publication settlement itself and the other
non-publication branches still lack route-side consumer semantics. New in
pass 20: one publication-side branch now also works from the trajectory
observer without waiting on run-tree liveness, but it still relies on
`settlement_ready` because runtime does not stamp publication
`status=settled`. The remaining question is whether M5 should keep extending
that observer-level route or force the substrate successor to finish the
runtime-side settlement stamp before the general flip. New in pass 21: the
successful publication branch now does reach `status=settled`, and the
route-side consumer accepts that terminal predicate while keeping the earlier
`settlement_ready` observer only as transition compatibility. The remaining
gap is no longer "publication success never reaches settled" but when the
compatibility branch can be deleted and the broader consumer rule can be
stated once for all branches. New in pass 22: explicit terminal no-story
outcomes no longer hang forever; they now cancel the trajectory and project
to request `completed`, while `deferred` remains a separately visible open
branch. The remaining gap is no longer "do sibling non-publication verdicts
have any terminal/request image at all?" but how the still-open `deferred`
branch should be carried and when the remaining compatibility rules can be
collapsed into one general reconcile predicate. New in pass 23: `deferred`
now has an explicit request-ledger image once runtime is terminal, so the
remaining gap is no longer the infinite-repoll leak itself but the missing
wake/resumption rule for that open branch. New in pass 24: source-fetch
continuity refresh now gives `deferred` one explicit wake path by
superseding it with a later queued successor on the same `continuity_ref`.
The remaining gap is no longer the fetch-driven wake path but whether
non-fetch authorities need their own resumption protocol. New in pass 25:
the direct `trajectory.status=settled` branch now no longer waits on
`run.State` or `ActiveChildRuns`, so the remaining gap is not whether the
settled certificate is strong enough, but how much longer the
`settlement_ready` compatibility tail has to survive. New in pass 26: that
tail is now explicitly observable if it fires, so the remaining gap is no
longer "can we tell whether the fallback still matters?" but whether later
realism evidence shows it can be deleted. New in pass 27: the direct
publication-success branch now consumes only `trajectory.status=settled`; the
legacy publication fallback is gone. The remaining gap is no longer whether
publication success needs compatibility, but the broader general reconcile
rule over terminal branches plus the residual `run.State` /
`ActiveChildRuns` reads still used for runtime-status and failure accounting.
New in pass 28: sourcecycled now has zero `ActiveChildRuns` control reads at
all. The remaining gap is narrower again: only terminal `run.State` remains
as run-lifecycle bookkeeping, so the next honest question is whether that is
the final acceptable processor-capacity/failure surface or another route-side
control read M5 should eventually replace.
New in pass 29: the completed story-route processor-resolution certificate
now also releases sourcecycled processor capacity before terminal
`run.State`. The remaining run-state question is therefore narrower than
"all positive branches": it is now concentrated on deferred/unresolved
bookkeeping plus genuine failed/cancelled runtime exits. New in pass 30:
that earlier local release is not yet the same thing as real runtime
admission capacity. The runtime overload guard still counts a still-running
processor run by profile and can return `429` until the run leaves
`running`, so the next realism question is whether M5 should align those
surfaces around one durable processor-phase predicate or accept the split as
an explicit two-layer boundary. New in pass 31: that split is now proven
against the real runtime handler, not only the route-side mock. The remaining
question is no longer "is the mismatch real?" but whether M5 should align the
surfaces or explicitly keep them as separate layers with retried `429`
rejection as the boundary. New in pass 32: the story-route mismatch is now
closed. The runtime overload guard and sourcecycled both free capacity from
the same durable `all_source_items_decided_with_story_route` processor-phase
certificate. The remaining capacity question is narrower again: whether that
alignment should generalize to other completed processor-resolution terminal
branches or stay scoped to the story-route handoff case.

**settlement:** not started. This mission is no longer the next construct
target. It resumes only after successor
`docs/mission-wire-settlement-substrate-v0.md` settles enough substrate to
make the evidence gate honest. Exit here still requires: sourcecycled
reconciling on settlement with zero `ActiveChildRuns` reads; the
honest-and-full instrument; the staged domain ramp climbed to **one observed
production multi-story cycle at maxProc > 1** with its receipts (cycle id,
edition diff, instrument output) recorded here; landing proof (commit, push,
CI, deploy identity); and an explicit verdict on the rearchitecture's core
claim — supported / weakened / falsified — at the scope the cycle actually
covered.
