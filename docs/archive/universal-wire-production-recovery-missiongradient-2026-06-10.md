# Universal Wire Production Recovery MissionGradient — 2026-06-10

## Requirements Contract

Primary incident/root-cause contract: [`docs/universal-wire-empty-front-page-root-cause-2026-06-10.md`](universal-wire-empty-front-page-root-cause-2026-06-10.md).

Existing mission/state context:

- [`docs/mission-report-wire-community-news-2026-06-09.md`](mission-report-wire-community-news-2026-06-09.md)
- [`docs/mission-wire-community-news-v1.md`](mission-wire-community-news-v1.md)
- [`docs/universal-wire-activation-topology-2026-06-10.md`](universal-wire-activation-topology-2026-06-10.md)

This document controls the recovery trajectory, belief state, evidence, rollback, and stopping conditions for the next coding/deployment run. The incident document controls the bug facts observed before this mission starts.

## Short `/goal` String

```text
Use MissionGradient. Complete docs/universal-wire-production-recovery-missiongradient-2026-06-10.md by fixing Universal Wire production end-to-end: sourcecycled must not overload the platform computer, processor handoffs must complete into VText article revisions, platform publication must sync full VText revision history into corpusd, the durable Wire edition must expose non-empty stories, and the authenticated Universal Wire app must render article cards on staging. Preserve platform/user computer authority boundaries, avoid fake/manual seeded articles, maintain an evidence ledger, update the incident and mission docs, commit/push/deploy behavior-changing fixes, verify CI/deploy/staging identity, and stop only on complete proof or a hard blocker after root-cause probes plus cognitive reframing. If the stopping condition is not reached, do not call the mission complete; land and report only checkpoint_incomplete or blocked_incomplete with a resumable next executable probe.
```

## Real Artifact

The production artifact is the **Universal Wire production loop**:

```text
source fetch -> ingestion handoff -> bounded platform processor execution -> VText article revision -> autonomous platform publish -> corpusd full-revision sync -> durable Wire edition/story index -> authenticated Universal Wire app cards
```

The mission is not merely to restart a VM, make `/health` green, or submit runtime runs. The artifact works only when authenticated staging users see real Wire articles backed by VText revision history.

## Invariants

1. **Published means corpusd durable state.** Platform-published VText documents and revisions must be synced into corpusd DoltDB. Do not define publication as transient sandbox state.
2. **VText owns articles.** Processors may request/open/revise articles through VText/channel semantics; they must not manually create front-page rows or fake edition cards.
3. **Platform computer owns Universal Wire production.** User computers read the published platform corpus; they do not own global Wire production state.
4. **No product-path bypass.** Do not manually seed success rows, use browser-public internal/test routes as proof, or patch tracked files directly on Node B as source of truth.
5. **Backpressure is mandatory.** Downstream platform runtime capacity must control upstream processor admission.
6. **Completion evidence beats admission evidence.** `processor_submitted` is not production proof. Proof requires article revision, corpusd sync, edition visibility, and rendered story cards.
7. **Rollbackable platform changes.** Behavior-changing changes require commit, push, CI/deploy monitoring, deployed commit identity, staging acceptance proof, and rollback refs.
8. **No fake islands.** Any simplified proof must preserve the same production interfaces, authority boundaries, persistence path, and verifier semantics.

## Value Criterion

Minimize divergence between the intended Universal Wire causal graph and the deployed staging graph while preserving authority and persistence invariants.

Penalize:

- overload that admits more processor work than the platform computer can complete;
- hidden queues or states not reflected in sourcecycled/runtime/corpusd evidence;
- frontend success without corpusd-backed VText revisions;
- corpusd rows without edition/front-page visibility;
- local-only or manually seeded proof;
- green health checks that do not imply article production;
- fixes that couple the public read path to a wedged write-side platform VM.

## Quality Gradient

Expected quality: **solid**.

Substandard/rushed work includes:

- changing a default or timeout without proving the causal chain;
- suppressing errors or retries to make logs quieter;
- adding a second parallel “wire articles” store;
- creating one demo article by hand;
- treating source fetch count, run submission, or VM health as article-production proof;
- leaving sourcecycled able to re-wedge the platform VM under the next burst;
- failing to update docs/evidence after discovering a new root cause.

## Cognitive Transform Review

### Current uncertainty or obstacle

Universal Wire shows zero articles even though sourcecycled fetches thousands of source items and submitted processors. The system accepted work but did not produce durable, visible published articles. The main uncertainty is where the accepted runs die: platform runtime overload, VText creation, publish eligibility, corpusd sync, edition mutation, or public read path.

### Selected transforms

1. **State machine transform** — The system must be represented as states and transitions, not logs. This exposes impossible/stuck states such as `ownership_active` + `guest_unhealthy`, `processor_submitted` + no runtime completion, and `corpusd_empty` + frontend honest-empty.
2. **Backpressure transform** — The decisive variable is downstream capacity. A per-drain batch limit is not control; sourcecycled must sense live active processor count and runtime health before admitting more work.
3. **Commutative diagram transform** — Two paths should agree: sandbox VText article state -> corpusd sync -> public stories, and runtime/publish events -> sourcecycled completion ledger -> public stories. Today they do not commute because corpusd is empty and sourcecycled only knows submission.
4. **Contrapositive transform** — If corpusd has zero VText documents/revisions, then no completed autonomous publication reached the durable published corpus, regardless of source counts or submitted processor runs.
5. **Prototype honesty / homotopy transform** — The smallest useful fix must still use the real sourcecycled/runtime/VText/corpusd/frontend path. A fake seeded article or alternate endpoint would solve a different object.

### Route-changing insights

- Implementation must start by preventing overload and exposing completion state; otherwise restarts only recreate the wedge.
- Verifier must wait for corpusd VText rows and story cards, not merely accepted runtime runs.
- Scope must include read-side durability: the public Universal Wire stories endpoint should not depend on a wedged write-side sandbox once corpusd is the publication store.
- Stopping condition must be “visible corpusd-backed article cards on staging,” not “processors submitted” or “health ready.”

### Changed plan

- **Implementation:** add live active-run/backpressure guard; add runtime overload rejection; add completion/publish/edition evidence; repair publish/edition chain only after the VM is no longer self-wedging.
- **Verifier/evidence:** record sourcecycled queue state, runtime active/completed runs, corpusd doc/revision counts, edition doc/transclusion state, `/api/universal-wire/stories` response, and browser-rendered cards.
- **Scope:** fix production loop end-to-end; avoid UI-only empty-state changes unless they expose diagnostics without hiding the bug.
- **Stopping condition:** require deployed staging article cards backed by corpusd full VText revision history.

### Next high-information action

Recover or safely inspect the platform VM enough to classify the 64 submitted processor runs: running forever, failed, completed without VText, VText-created but publish-ineligible, publish-failed, or edition-failed.

## Homotopy Parameters

Increase realism continuously along these axes:

1. **Processor concurrency:** 1 active processor -> bounded small N -> production target capacity.
2. **Source volume:** one eligible source item -> one processor route -> all active source classes.
3. **Persistence proof:** sandbox VText revision -> corpusd full revision sync -> durable edition/story index.
4. **Read proof:** internal API -> authenticated `/api/universal-wire/stories` -> Universal Wire UI cards.
5. **Failure pressure:** healthy VM -> overload rejection -> retry/drain recovery -> no wedge under burst.
6. **Deployment realism:** local/focused tests -> staging deploy -> authenticated product-path proof.

Each low-resolution step must use the same production interface family. No mocks or alternate stores.

## Belief State

### Current artifact state

- sourcecycled source fetch is healthy and produces thousands of candidate items.
- platform VM ownership reports active, but direct sandbox requests hang and vmctl health marks it unhealthy.
- platform Firecracker process is CPU-saturated.
- corpusd has zero synced VText documents/revisions.
- Universal Wire UI honestly renders zero articles.

### Evidence for state

- source service latest-cycle API: `success_fetch_count=198`, `item_count=4241`.
- sourcecycled logs: `processor_submitted=32` twice, then UDS proxy timeout errors.
- vmctl logs: repeated health check failures for `vm-universal-wire-platform`.
- direct curl to sandbox health/story endpoint: timeout/no headers.
- corpusd Dolt SQL: `platform_vtext_documents=0`, `platform_vtext_revisions=0`.

### Main uncertainties

- Whether submitted processor run records exist in the platform sandbox and what states they hold.
- Whether any VText article docs were created before the VM wedged.
- Whether autonomous publish eligibility rejected generated revisions.
- Whether edition mutation/transclusion is also broken after publication is repaired.
- Whether read path already supports corpusd-backed stories or still depends on sandbox state.

### Highest-impact uncertainty

The first failure transition after `runtime accepted processor run`.

### Next observation that reduces uncertainty

Inspect platform runtime/VText state after recovering the VM or mounting its data disk safely while stopped, and classify submitted run IDs against VText/publish/edition records.


## Conjecture Ledger

This mission is now best understood as conjecture control rather than a flat plan. Each conjecture records: current status, strongest evidence, and the next falsifier.

### C1 — Split-brain list/open state
- **Status:** confirmed, mitigated
- **Claim:** Universal Wire list was surfacing guest-local platform-VM article revisions before durable corpusd publication existed, while headline-open expected the durable published doc path.
- **Evidence:** users observed 0 ↔ 12 oscillation and `Get document failed (404)` on click; corpusd remained at `0 docs / 0 revs`; runtime list path read guest-local store.
- **Action taken:** list indexing now requires durable publication markers.
- **Next falsifier:** if list still shows stories while corpusd is zero, the new list guard is incomplete.

### C2 — Transport / host publish path was the blocker
- **Status:** ruled out as current blocker
- **Claim:** guest could not reach or use the host publish path.
- **Evidence:** guest-local curl to host TAP `:8082/health` now succeeds; direct host replay of a real full article payload returned `201 Created`.
- **Action taken:** fixed publish URL config, proxy desktop resolution, and tap firewall `8082`.
- **Next falsifier:** none for current frontier; keep as historical ledger entry.

### C3 — Queue accounting alone caused the stall
- **Status:** ruled out as sole blocker
- **Claim:** sourcecycled stale `submitted` rows were the main reason progress stopped.
- **Evidence:** queue accounting fixes improved progression, but processors still completed without corpusd rows.
- **Action taken:** submitted-row reconciliation, missing-run requeue, stale-row resets.
- **Next falsifier:** if queue freezes again with no guest work, inspect sourcecycled reconciliation first.

### C4 — Processor coverage decisions were poisoned by unpublished guest-local docs
- **Status:** confirmed, fixed in substrate
- **Claim:** `search_wire_corpus` was searching the whole guest-local corpus, causing false “already covered” conclusions from unpublished local articles.
- **Evidence:** live processor explicitly reported “existing article coverage”; earlier ghost stories existed only in guest-local state; search tool originally queried guest-local corpus wholesale.
- **Action taken:** `search_wire_corpus` now searches published-only docs.
- **Next falsifier:** if a clean processor still claims “already covered,” then a different coverage path or stale continuity is responsible.

### C5 — Stale processor continuity / agent identity is steering live decisions
- **Status:** active leading conjecture
- **Claim:** the processor still carries old checkpoint beliefs such as “corpus search restoration blocked” or legacy coverage assumptions, and that stale continuity is suppressing VText spawn even after substrate fixes.
- **Evidence:** completed processor outputs include “VText spawning deferred — blocked on wire corpus search restoration for dedup verification”; old processor agent identities were stable; channel-only versioning was insufficient, so agent identity was versioned to `processor-v2:*`.
- **Action taken:** processor `channel_id` and `agent_id` now versioned to `processor-v2:*`.
- **Next falsifier:** inspect the first clean processor run on the new identity. If it still reports stale restoration/coverage beliefs, the poison is not coming from the old processor identity alone.

### C6 — Single admitted processor still destabilizes the guest before durable publish
- **Status:** active
- **Claim:** even one admitted processor can push the platform VM into an unhealthy state before corpusd sync occurs.
- **Evidence:** one-at-a-time admission works; guest often becomes unhealthy with `running_processor_runs` around 1 while corpusd remains zero.
- **Action taken:** lowered concurrency to 1 and added `running_processor_runs` health instrumentation.
- **Next falsifier:** capture the exact child-run topology and terminal state of one clean admitted processor. If it completes without unhealthy periods, then the main issue is semantic, not capacity.

### C7 — Current semantic frontier
- **Status:** active primary conjecture
- **Claim:** on a fresh platform guest and fresh processor continuity channel, one admitted processor should either (a) produce one durable corpusd publication, or (b) expose the exact child-run / coverage-decision reason it does not.
- **Strongest evidence:** latest clean processor completed and explicitly decided not to spawn VText because it believed no new VText work was required. Corpusd stayed zero.
- **Next falsifier:** run one clean processor on the fresh `processor-v2:*` identity and inspect its full terminal reasoning, child runs, and any publish attempt.

### C8 — Ingestion and publication are over-coupled inside a single processor run
- **Status:** active architectural conjecture
- **Claim:** the current processor is doing too much in one long-lived run: ingesting source items, deduping against existing coverage, deciding publication priority, optionally fetching full text, optionally spawning VText, and effectively acting as an orchestration gate. This coupling raises the chance that a semantic deferral ("already covered", "need dedup verification", "need publication direction") prevents durable publication entirely.
- **Evidence:** clean processors complete without crashing yet still return outcomes like "No VText spawns required" or "VText spawning deferred"; queue/transport fixes no longer change that decision frontier.
- **Action taken:** none yet on architecture; only substrate and observability fixes so far.
- **Next falsifier:** if a processor on fresh continuity still suppresses VText spawn after published-only corpus search is fixed, decoupling becomes the leading architectural remedy rather than a fallback idea.

## Realest Decoupled Pipeline Preserving Topology

This is not the "smallest" pipeline. It is the **realest** one that preserves the current system's topology, authority boundaries, and product ontology.

```text
source fetch
-> normalized source facts / source items
-> processor evidence pass (read-only semantic extraction, no publication authority)
-> durable candidate story ledger on platform VM
-> coverage / dedup pass against published corpus only
-> publication-candidate selection / ranking
-> VText article spawn or revision
-> autonomous publish to corpusd
-> durable Wire edition update
-> public stories list / headline open
```

### Why this is the realest cut

- **Preserves platform VM ownership** of live article creation and editorial state.
- **Preserves corpusd** as the durable public publication surface.
- **Preserves VText ownership** of article bodies and revision history.
- **Preserves sourcecycled** as ingestion/orchestration substrate, but removes the demand that one processor run must perform ingestion, dedup, editorial triage, and publication control all at once.
- **Preserves current topology**: no fake side stores, no manual seed path, no alternate article substrate.

### Boundary split

1. **Processor evidence pass**
   - input: source item handles + prior candidate context
   - output: structured findings / candidate story records / watch items
   - not allowed to decide "do nothing forever" merely because a guest-local unpublished doc exists

2. **Coverage / dedup pass**
   - input: candidate story records
   - corpus: published-only durable corpus
   - output: `new`, `update_existing`, `defer`, or `duplicate`

3. **Publication candidate selection**
   - explicit prioritization step
   - determines which `new` / `update_existing` candidates actually become VText work
   - removes hidden editorial policy from the processor's final paragraph

4. **VText article spawn / revision**
   - VText owns article body creation and revision history
   - output must carry durable lineage required for autonomous publish

5. **Autonomous publish / sync**
   - platform VM -> corpusd
   - only after this step is a story eligible for the canonical Wire list

### Why this helps the current failure mode

It turns the processor from a monolithic control object into a narrower semantic extractor. The current system lets one processor completion short-circuit the whole pipeline with a conclusion like "already covered" or "need dedup verification". The decoupled version moves those decisions into explicit durable states and separate passes.

### Expected benefits

- fewer long-lived processor runs
- lower guest pressure per run
- explicit durable candidate state instead of hidden continuity beliefs
- clearer evidence for why a story did or did not become a VText article
- easier replay / verification of the pipeline from candidate -> publish

### Risks

- more durable intermediate state means more schema / ledger complexity
- if the candidate ledger becomes the new canonical article substrate, that would violate topology; it must remain a **pre-VText candidate ledger**, not a parallel article store

### Architectural invariant

The decoupled pipeline must not create a second article truth.

- candidate ledger = pre-article planning state
- VText = article truth
- corpusd = public durable publication truth

Any design that lets candidate records or sourcecycled rows stand in for VText articles is a regression.

### C9 — Publication chain escapes the root processor run
- **Status:** confirmed, active
- **Claim:** a processor may spawn a VText child that completes, yet the effective publication work continues under a super-owned branch on the same document channel. If sourcecycled only tracks the root processor run (or even its direct children), it can admit the next processor while the prior publication chain is still active.
- **Evidence:** clean lifecycle trace on 2026-06-11: processor `9a2606ee...` spawned VText `ffaa48da...` and completed; then a super-owned parent `f653e2b6...` spawned VText `42eda5a8...` on the same channel; sourcecycled admitted the next processor `dbc6478a...` before that super/vtext continuation finished; corpusd remained zero.
- **Action taken:** child-run-aware sourcecycled reconciliation was added for direct descendants, but this result shows that root-child tracking alone is insufficient when the chain continues through another supervising run on the same document channel.
- **Next falsifier:** account for active publication chains by shared channel / trajectory / candidate-ledger state rather than only by direct root-child relationships, then re-run one-at-a-time admission and see whether a second processor is still admitted before the first document channel settles.

- **Current strongest evidence:** clean lifecycle trace showed `processor 9a2606ee -> vtext ffaa48da (completed) -> processor completes -> super f653e2b6 continues on same channel -> vtext 42eda5a8 (completed)` while corpusd stayed zero. This is sufficient to falsify the simpler root-run completion invariant for Universal Wire.

### Outside current experimental envelope
These are not conjectures but known blind spots in the current experimental program:
- exact live child-run topology inside the platform guest after one processor admission;
- exact continuity contents influencing the new `processor-v2:*` identity;
- whether any remaining coverage decisions are bypassing `search_wire_corpus`;
- whether corpusd sync will succeed once a processor actually chooses to spawn VText again.

## Investigation & Cognitive Reframing Loop

Before declaring blocked:

1. Observe the live state at all boundaries: sourcecycled queue, vmctl health, platform sandbox health, runtime runs, VText docs/revisions, corpusd sync tables, edition/story API, UI.
2. Identify the earliest transition whose precondition is true and postcondition is false.
3. Patch or instrument only that transition.
4. Verify the next transition before widening scope.
5. Re-run 2-5 cognitive transforms if observations contradict the belief state.

Tactical blockers that should trigger another autonomous probe:

- sandbox HTTP timeout;
- missing runtime run records;
- publish eligibility failure;
- sourcecycled queue status ambiguity;
- corpusd sync failure;
- edition alias/transclusion mismatch.

Invariant-level or external blockers requiring escalation:

- missing provider credentials or exhausted paid provider quota with no authorized alternate;
- destructive disk/database repair risk without safe snapshot/rollback;
- need to change platform/user authority boundary;
- need to expose internal/test APIs publicly.

## Receding-Horizon Control

Control interval: one causal boundary at a time.

Mutation radius:

- one subsystem change before focused verification;
- no simultaneous frontend/read-path/write-path rewrites unless the boundary evidence requires it;
- docs-first checkpoint before any new behavior-changing platform fix discovered from staging evidence.

Loop:

1. choose the earliest failing transition;
2. predict what evidence should change;
3. patch or instrument minimally;
4. run focused local test(s);
5. commit/push/deploy if behavior-changing;
6. verify staging identity and product-path behavior;
7. update belief state, incident doc, and mission report;
8. continue, narrow, branch, rollback, or stop.

## Dense Feedback Channels

- sourcecycled internal API: latest ingestion handoff, queue/request status, source fetch health.
- sourcecycled logs: dispatch attempts, submit failures, drain activity.
- vmctl ownership and logs: active vs unhealthy state, live sandbox URL, health failures.
- platform sandbox health: `running_runs`, disk usage, commit identity, runtime status.
- runtime run records/traces: processor states and tool calls.
- VText store: article docs/revisions and metadata required for autonomous publish.
- corpusd DoltDB: `platform_vtext_documents`, `platform_vtext_revisions`, publication/sync evidence.
- public API: authenticated `/api/universal-wire/stories` returns non-empty stories from expected source.
- browser proof: Universal Wire app renders story cards and headline click opens full VText revision history.
- CI/deploy: GitHub Actions, deployed commit identity, rollback refs.

## Evidence Ledger Format

For each nontrivial claim, record:

```text
claim:
evidence source:
command or observation:
artifact path or log ref:
result:
uncertainty/caveat:
promotion relevance:
```

Do not claim article production from run admission. Do not claim publication without corpusd rows and story API visibility. Do not claim UI success without browser/product-path proof.

## Mission Report

Markdown report path: `docs/mission-report-universal-wire-production-recovery-2026-06-10.md`

PDF report path: `~/Library/Mobile Documents/com~apple~CloudDocs/mission reports/mission-report-universal-wire-production-recovery-2026-06-10.pdf`

Update cadence:

- after initial substrate inspection;
- after docs-first bug checkpoint;
- after each behavior-changing commit;
- after CI/deploy/product-path proof;
- after any target-level route change;
- at incomplete checkpoint/blocker/completion.

Audience: owner/operator who needs to understand what happened, what changed, what was proven, what remains unsafe, and how to resume.

Evidence linking policy: keep logs/large command output in artifacts or named commands; report should summarize and cite, not dump.

## Run Checkpoint & Resumption State

status: `checkpoint_incomplete`
last checkpoint: Transport, backpressure, queue accounting, and guest-local ghost-story listing were repaired enough to isolate the remaining semantic frontier.

current artifact state: Universal Wire should no longer surface unpublished guest-local stories as canonical list items; corpusd still has zero docs/revisions; one-at-a-time processor admission works; the strongest current evidence is that admitted processors often decide not to spawn VText because of stale/incorrect coverage beliefs, or they spawn child work that does not converge into durable publication.

what shipped:

- guest→host publish transport repairs (publish URL, tap 8082, desktop resolution)
- host publish path repairs (inline payload, raw internal reads, run-metadata preservation)
- sourcecycled queue/accounting repairs (submitted reconciliation, missing-run requeue, stale-row resets)
- Wire list honesty guard (durable-published-only indexing instead of guest-local ghost stories)
- published-only corpus search for `search_wire_corpus`
- one-at-a-time processor admission experiment

what was proven:

- the ghost 12-card list was guest-local platform VM state, not manual mocks;
- corpusd remains the durable source of truth and is still empty;
- host publish endpoint can succeed on full replay payloads;
- guest can reach host proxy on TAP 8082;
- sourcecycled can now admit exactly one processor at a time and self-heal submitted rows much better than before;
- at least one admitted processor completed and explicitly chose not to spawn VText because it believed existing coverage already existed.

unproven or partial claims:

- whether the published-only corpus-search fix has been observed through a truly clean processor run free of stale continuity;
- whether live processor child-run topology converges to VText spawn and publish;
- whether any current live processor-generated revision can produce the first non-zero corpusd document/revision row without manual replay.

belief-state changes:

- the dominant failure is no longer transport or UI; it is a semantic/agentic decision frontier inside the processor run;
- continuity/channel memory is now a leading hypothesis, because processors still emit stale beliefs like “wire corpus search restoration” after the substrate changed.

remaining error field:

- processor continuity poisoning or stale coverage beliefs;
- possible child-run non-convergence after a single admitted processor;
- no durable corpusd publication yet.

highest-impact remaining uncertainty: when a single clean processor run is admitted on a fresh guest and fresh processor channel, does it spawn VText and publish durably, or does it still suppress itself because of stale continuity / false coverage?

next executable probe: run one fresh processor on the new `processor-v2:*` channel, inspect its channel memory and terminal result, and if needed clear/bypass continuity for one trial to test whether VText spawn reappears.

suggested resume goal string: Use MissionGradient. Complete docs/universal-wire-production-recovery-missiongradient-2026-06-10.md by proving or falsifying the current semantic frontier: on a fresh platform guest and fresh processor continuity channel, one admitted processor must either produce one durable corpusd publication or expose the exact child-run / coverage-decision reason it does not. Keep Universal Wire list honesty (published-only), preserve platform/user authority boundaries, maintain evidence-ledger updates in the mission report, and stop only on durable corpusd proof or a hard blocker after root-cause probes.

evidence artifact refs:

- docs/universal-wire-empty-front-page-root-cause-2026-06-10.md
- docs/mission-report-universal-wire-production-recovery-2026-06-10.md
- direct host publish replay returned 201 while live processors still withheld VText spawn
- guest-local curl to host 8082 succeeded after tap firewall repair

rollback refs: latest host/runtime deploys are on `main`; platform guest can be stop/resume refreshed cleanly; no manual state seeding or fake article insertion was used.


## Post-Mission Refactor Notes

These notes are intentionally outside the current proof branch but should guide the next design/doc revision cycle:

- remove parent/child as the primary causality/control abstraction; use coagent trajectory + artifact/channel scoped liveness instead;
- VText should not route to co-super or vsuper;
- VText should only route to super for real coding/execution/privileged work, not vague continuation/orchestration;
- supers will use nucleus sandboxes for ephemeral execution;
- rename sandbox -> autoputer for persistent computers;
- rename corpusd -> corpusd for durable publication service;
- evolve MissionGradient so conjecture ledgers are first-class rather than manually appended.

## Forbidden Shortcuts

- Do not seed corpusd docs/revisions manually and call that production.
- Do not create a frontend-only sample card or alternate endpoint.
- Do not treat `processor_submitted`, `running_runs`, or source fetch counts as article-production proof.
- Do not bypass VText ownership or edition transclusion.
- Do not route public proof through `/api/agent/*`, `/api/test/*`, `/internal/*`, raw mutation endpoints, or other browser-public internal/test-only routes.
- Do not leave sourcecycled able to submit unbounded bursts after a restart.
- Do not patch Node B tracked files directly as source of truth.
- Do not silence health/proxy errors without fixing causal failure.

## Rollback Policy

- Git rollback: every behavior-changing fix lands as a commit on main; rollback target is prior deployed commit SHA.
- Deploy rollback: record GitHub Actions run, deployed commit identity, and Node B health for each deploy.
- VM rollback: before destructive data-image or database repair, stop the VM and preserve/snapshot rollback refs where available.
- Database rollback: corpusd/sandbox Dolt commits or data-image backups must be named before mutation.
- Queue rollback: avoid deleting sourcecycled processor requests unless their state and replay consequences are documented.
- Runtime rollback: overload/backpressure changes must fail closed by leaving work queued, not by dropping handoffs.

## Learning Side-Channel

Tactical learning goes into the mission report and tests.

Target-level learning updates this MissionGradient doc and the incident doc.

Invariant-level learning must stop and escalate before changing publication ownership, product authority boundaries, or public/internal route contracts.

Durable architecture lessons should be backported to canonical docs only after the behavior-changing fix is proven on staging.

## Stopping Condition

Report `complete` only when all are true:

1. sourcecycled cannot overload the platform computer under normal drain cadence;
2. platform runtime rejects/defers overload instead of wedging;
3. at least one processor completes through VText article revision creation;
4. corpusd contains the article document plus full revision history;
5. durable Wire edition/story API returns non-empty stories;
6. authenticated Universal Wire app renders article cards on staging;
7. clicking a headline opens the VText article with revision history;
8. CI passed and staging deployed commit identity is verified;
9. evidence ledger and mission report are updated;
10. rollback refs and residual risks are named.

If useful changes land but these are not all proven, report `checkpoint_incomplete`, not success.

If a blocker remains after root-cause probes and cognitive transforms, report `blocked_incomplete` with exact evidence and the smallest safe next probe.
