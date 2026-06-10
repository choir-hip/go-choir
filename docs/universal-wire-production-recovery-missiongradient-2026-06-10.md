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
Use MissionGradient. Complete docs/universal-wire-production-recovery-missiongradient-2026-06-10.md by fixing Universal Wire production end-to-end: sourcecycled must not overload the platform computer, processor handoffs must complete into VText article revisions, platform publication must sync full VText revision history into platformd, the durable Wire edition must expose non-empty stories, and the authenticated Universal Wire app must render article cards on staging. Preserve platform/user computer authority boundaries, avoid fake/manual seeded articles, maintain an evidence ledger, update the incident and mission docs, commit/push/deploy behavior-changing fixes, verify CI/deploy/staging identity, and stop only on complete proof or a hard blocker after root-cause probes plus cognitive reframing. If the stopping condition is not reached, do not call the mission complete; land and report only checkpoint_incomplete or blocked_incomplete with a resumable next executable probe.
```

## Real Artifact

The production artifact is the **Universal Wire production loop**:

```text
source fetch -> ingestion handoff -> bounded platform processor execution -> VText article revision -> autonomous platform publish -> platformd full-revision sync -> durable Wire edition/story index -> authenticated Universal Wire app cards
```

The mission is not merely to restart a VM, make `/health` green, or submit runtime runs. The artifact works only when authenticated staging users see real Wire articles backed by VText revision history.

## Invariants

1. **Published means platformd durable state.** Platform-published VText documents and revisions must be synced into platformd DoltDB. Do not define publication as transient sandbox state.
2. **VText owns articles.** Processors may request/open/revise articles through VText/channel semantics; they must not manually create front-page rows or fake edition cards.
3. **Platform computer owns Universal Wire production.** User computers read the published platform corpus; they do not own global Wire production state.
4. **No product-path bypass.** Do not manually seed success rows, use browser-public internal/test routes as proof, or patch tracked files directly on Node B as source of truth.
5. **Backpressure is mandatory.** Downstream platform runtime capacity must control upstream processor admission.
6. **Completion evidence beats admission evidence.** `processor_submitted` is not production proof. Proof requires article revision, platformd sync, edition visibility, and rendered story cards.
7. **Rollbackable platform changes.** Behavior-changing changes require commit, push, CI/deploy monitoring, deployed commit identity, staging acceptance proof, and rollback refs.
8. **No fake islands.** Any simplified proof must preserve the same production interfaces, authority boundaries, persistence path, and verifier semantics.

## Value Criterion

Minimize divergence between the intended Universal Wire causal graph and the deployed staging graph while preserving authority and persistence invariants.

Penalize:

- overload that admits more processor work than the platform computer can complete;
- hidden queues or states not reflected in sourcecycled/runtime/platformd evidence;
- frontend success without platformd-backed VText revisions;
- platformd rows without edition/front-page visibility;
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

Universal Wire shows zero articles even though sourcecycled fetches thousands of source items and submitted processors. The system accepted work but did not produce durable, visible published articles. The main uncertainty is where the accepted runs die: platform runtime overload, VText creation, publish eligibility, platformd sync, edition mutation, or public read path.

### Selected transforms

1. **State machine transform** — The system must be represented as states and transitions, not logs. This exposes impossible/stuck states such as `ownership_active` + `guest_unhealthy`, `processor_submitted` + no runtime completion, and `platformd_empty` + frontend honest-empty.
2. **Backpressure transform** — The decisive variable is downstream capacity. A per-drain batch limit is not control; sourcecycled must sense live active processor count and runtime health before admitting more work.
3. **Commutative diagram transform** — Two paths should agree: sandbox VText article state -> platformd sync -> public stories, and runtime/publish events -> sourcecycled completion ledger -> public stories. Today they do not commute because platformd is empty and sourcecycled only knows submission.
4. **Contrapositive transform** — If platformd has zero VText documents/revisions, then no completed autonomous publication reached the durable published corpus, regardless of source counts or submitted processor runs.
5. **Prototype honesty / homotopy transform** — The smallest useful fix must still use the real sourcecycled/runtime/VText/platformd/frontend path. A fake seeded article or alternate endpoint would solve a different object.

### Route-changing insights

- Implementation must start by preventing overload and exposing completion state; otherwise restarts only recreate the wedge.
- Verifier must wait for platformd VText rows and story cards, not merely accepted runtime runs.
- Scope must include read-side durability: the public Universal Wire stories endpoint should not depend on a wedged write-side sandbox once platformd is the publication store.
- Stopping condition must be “visible platformd-backed article cards on staging,” not “processors submitted” or “health ready.”

### Changed plan

- **Implementation:** add live active-run/backpressure guard; add runtime overload rejection; add completion/publish/edition evidence; repair publish/edition chain only after the VM is no longer self-wedging.
- **Verifier/evidence:** record sourcecycled queue state, runtime active/completed runs, platformd doc/revision counts, edition doc/transclusion state, `/api/universal-wire/stories` response, and browser-rendered cards.
- **Scope:** fix production loop end-to-end; avoid UI-only empty-state changes unless they expose diagnostics without hiding the bug.
- **Stopping condition:** require deployed staging article cards backed by platformd full VText revision history.

### Next high-information action

Recover or safely inspect the platform VM enough to classify the 64 submitted processor runs: running forever, failed, completed without VText, VText-created but publish-ineligible, publish-failed, or edition-failed.

## Homotopy Parameters

Increase realism continuously along these axes:

1. **Processor concurrency:** 1 active processor -> bounded small N -> production target capacity.
2. **Source volume:** one eligible source item -> one processor route -> all active source classes.
3. **Persistence proof:** sandbox VText revision -> platformd full revision sync -> durable edition/story index.
4. **Read proof:** internal API -> authenticated `/api/universal-wire/stories` -> Universal Wire UI cards.
5. **Failure pressure:** healthy VM -> overload rejection -> retry/drain recovery -> no wedge under burst.
6. **Deployment realism:** local/focused tests -> staging deploy -> authenticated product-path proof.

Each low-resolution step must use the same production interface family. No mocks or alternate stores.

## Belief State

### Current artifact state

- sourcecycled source fetch is healthy and produces thousands of candidate items.
- platform VM ownership reports active, but direct sandbox requests hang and vmctl health marks it unhealthy.
- platform Firecracker process is CPU-saturated.
- platformd has zero synced VText documents/revisions.
- Universal Wire UI honestly renders zero articles.

### Evidence for state

- source service latest-cycle API: `success_fetch_count=198`, `item_count=4241`.
- sourcecycled logs: `processor_submitted=32` twice, then UDS proxy timeout errors.
- vmctl logs: repeated health check failures for `vm-universal-wire-platform`.
- direct curl to sandbox health/story endpoint: timeout/no headers.
- platformd Dolt SQL: `platform_vtext_documents=0`, `platform_vtext_revisions=0`.

### Main uncertainties

- Whether submitted processor run records exist in the platform sandbox and what states they hold.
- Whether any VText article docs were created before the VM wedged.
- Whether autonomous publish eligibility rejected generated revisions.
- Whether edition mutation/transclusion is also broken after publication is repaired.
- Whether read path already supports platformd-backed stories or still depends on sandbox state.

### Highest-impact uncertainty

The first failure transition after `runtime accepted processor run`.

### Next observation that reduces uncertainty

Inspect platform runtime/VText state after recovering the VM or mounting its data disk safely while stopped, and classify submitted run IDs against VText/publish/edition records.

## Investigation & Cognitive Reframing Loop

Before declaring blocked:

1. Observe the live state at all boundaries: sourcecycled queue, vmctl health, platform sandbox health, runtime runs, VText docs/revisions, platformd sync tables, edition/story API, UI.
2. Identify the earliest transition whose precondition is true and postcondition is false.
3. Patch or instrument only that transition.
4. Verify the next transition before widening scope.
5. Re-run 2-5 cognitive transforms if observations contradict the belief state.

Tactical blockers that should trigger another autonomous probe:

- sandbox HTTP timeout;
- missing runtime run records;
- publish eligibility failure;
- sourcecycled queue status ambiguity;
- platformd sync failure;
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
- platformd DoltDB: `platform_vtext_documents`, `platform_vtext_revisions`, publication/sync evidence.
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

Do not claim article production from run admission. Do not claim publication without platformd rows and story API visibility. Do not claim UI success without browser/product-path proof.

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

last checkpoint: Incident root cause documented in `docs/universal-wire-empty-front-page-root-cause-2026-06-10.md`; MissionGradient control doc created here.

current artifact state: Universal Wire still shows zero articles on staging; source fetch works; processor admission overloaded/wedged the platform VM; platformd has zero VText rows.

what shipped: Documentation only in this checkpoint.

what was proven:

- source ingestion is not the immediate blocker;
- platform VM is unhealthy/wedged despite active ownership;
- sourcecycled submitted processor runs without end-to-end completion evidence;
- platformd contains no published VText docs/revisions.

unproven or partial claims:

- exact state of the 64 submitted processor runs;
- whether VText documents were created inside the platform sandbox;
- whether publish eligibility or edition mutation would fail after overload is fixed;
- whether platformd-backed public story reads are fully wired.

belief-state changes:

- replaced prior false belief “32 submitted means producing news” with “submission overloaded the platform computer and no publication reached platformd.”

remaining error field:

- missing live concurrency/backpressure;
- missing completion/publication ledger;
- platform VM health/status mismatch;
- platformd empty;
- public app empty.

highest-impact remaining uncertainty: earliest failed transition after accepted processor run.

next executable probe: recover or safely inspect platform VM runtime state and classify submitted processor runs by completion/VText/publish/edition outcome.

suggested resume goal string: use the short `/goal` string above.

evidence artifact refs:

- `docs/universal-wire-empty-front-page-root-cause-2026-06-10.md`
- live logs and command output from 2026-06-10 investigation summarized in that doc.

rollback refs: no behavior-changing rollback needed for this docs checkpoint. Future behavior-changing commits must name git SHA and deployment rollback target.

## Forbidden Shortcuts

- Do not seed platformd docs/revisions manually and call that production.
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
- Database rollback: platformd/sandbox Dolt commits or data-image backups must be named before mutation.
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
4. platformd contains the article document plus full revision history;
5. durable Wire edition/story API returns non-empty stories;
6. authenticated Universal Wire app renders article cards on staging;
7. clicking a headline opens the VText article with revision history;
8. CI passed and staging deployed commit identity is verified;
9. evidence ledger and mission report are updated;
10. rollback refs and residual risks are named.

If useful changes land but these are not all proven, report `checkpoint_incomplete`, not success.

If a blocker remains after root-cause probes and cognitive transforms, report `blocked_incomplete` with exact evidence and the smallest safe next probe.
