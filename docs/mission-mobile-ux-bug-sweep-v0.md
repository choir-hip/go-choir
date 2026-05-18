# Mission: Mobile UX Bug Sweep v0

Goal string:

```text
Run docs/mission-mobile-ux-bug-sweep-v0.md as a Codex-operated MissionGradient mission:
repair the mobile web desktop substrate and two representative apps without boiling the ocean.
Use staging Playwright and product-path evidence to make Podcast usable as an ordinary app
and Trace usable as an evidence app: logged-out read/explore, floating-window mobile
desktop controls, reliable raise/focus/fit/snap/restoration, Podcast library/search/feed/player/progress,
Trace mobile drill-in/run-acceptance/inspector readability, and VText flicker regression
coverage. Land through git/CI/deploy for platform UX fixes, verify staging identity, and
finish with screenshots, DOM metrics, acceptance results, rollback refs, residual risks,
and the next realism axis. Do not hide product failures behind debug controls, manual IDs,
local-only proof, or platform deploy claims that are not verified on staging.
```

## Execution Model

This is a Choir-in-Choir multiagent sweep, not a local Codex patch sprint.

Codex is the mission operator, product-path driver, and evidence reviewer. Codex should not directly edit Choir product code for the UX fixes. It may edit this mission document, collect evidence, run Playwright, review exported patches, land approved work through git/CI/deploy, and perform owner/platform review actions. Product-code changes should be proposed by Choir agents through the visible staging prompt bar, worker VM/candidate computer path, vsuper orchestration, co-super worker/verifier channels, export/candidate evidence, and promotion/landing records.

If the Choir-in-Choir substrate itself blocks all delegation or evidence capture, Codex should investigate that as a substrate blocker, record precise evidence, and only make direct platform repairs after the blocker is named and the repair is clearly necessary to restore the product-path development loop.

## Real Artifact

The durable artifact is Choir's mobile web desktop experience on staging and the Choir-in-Choir development capacity that improves it: the shared Svelte shell, Podcast app surface, Trace evidence surface, VText coexistence behavior, worker/candidate/export evidence, and owner-review path that users actually touch through `https://draft.choir-ip.com`.

This mission is not a native mobile rewrite and not a reduced single-app phone mode. Mobile should remain a powerful web desktop. The work should make floating windows, app switching, playback, inspection, and logged-out exploration usable on a touch viewport.

## Invariants

- Mobile remains a desktop: floating windows, overlapping windows, app chrome, and task/dock affordances stay available.
- Logged-out read/explore must work where the action is public or ephemeral; mutation and persistence still require auth.
- Product-path evidence matters: use public/authenticated product APIs and Playwright, not internal/test success seeding.
- Product changes are developed in candidate/worker contexts, not by foreground Codex file edits.
- Compose many small UX improvements by dispatching sequential product-path goals to super/vsuper, with independent worker and verifier co-super evidence per improvement.
- Debug/provenance controls stay available but must not displace primary app controls.
- Platform behavior changes land through git, CI, deploy, staging identity verification, and deployed acceptance proof.
- Do not mutate active user computers directly for risky candidate/promotion work.

## Value Criterion

Minimize mobile task friction while preserving desktop power and evidence trust:

```text
L = ordinary app friction + evidence inspection friction + shell focus uncertainty
    + auth overblocking + missing playback state + unverifiable UX claims
```

Move uphill by reducing that loss with deployed evidence.

## Belief State

Starting belief:

- Podcast is implemented inside `ContentViewer.svelte`, not as a true app.
- Trace has useful data but mobile evidence is buried in long scroll surfaces.
- Window overlap is desirable, but current touch raise/focus/fit/snap/restore affordances are insufficient.
- Logged-out app launch is overblocked by `Desktop.svelte`.
- VText flicker is plausible from reactive contenteditable synchronization but still needs focused evidence.

Primary uncertainty:

- Which local fixes are sufficient to make staging mobile feel materially better without overbuilding shell architecture.
- Whether the current super -> worker VM -> vsuper -> co-super path can reliably produce small sequential UX candidates with reviewable evidence.

Next observations:

- Visible staging prompt-bar runs for each bounded UX goal.
- Trace/VText/export evidence showing super delegates mutable app/shell work to worker VM/vsuper instead of doing it in foreground.
- Codex review of each exported candidate patch before landing.
- Staging Playwright proof after deploy.
- Screenshots and DOM metrics for Podcast, Trace, VText coexistence, and shell controls.

## Homotopy Axes

- Shell controls: restore/focus bug fix -> fit/snap controls -> overview/dock improvements.
- Podcast: generic content view -> app-grade navigation/player -> persistent playback/subscription state.
- Trace: long responsive page -> compact panes/drill-in controls -> richer evidence browsing.
- Auth: app-launch wall -> action-level auth boundary.
- VText: suspected flicker -> reproducible regression -> targeted fix.
- Choir-in-Choir capacity: one large manual patch -> sequential small delegated candidates -> verified export/promotion loop.

Each step must remain a projection of the real product path.

## Dense Feedback

- Frontend build.
- Focused Playwright tests on mobile viewport (`390x844`) and desktop sanity viewport.
- Trace evidence for each delegated goal:
  - super request;
  - worker VM/vsuper lease;
  - worker and verifier co-super channel messages;
  - candidate/export refs or precise blocker;
  - verifier result and rollback refs.
- Staging screenshots and DOM metrics for:
  - logged-out Podcast launch/search;
  - Podcast library/feed/player/progress;
  - Trace summary/acceptance/inspector readability;
  - Trace and VText open together with reliable raise/focus/fit/snap/restore;
  - prompt bar growth/shrink;
  - VText typing while Trace is open.

## Forbidden Shortcuts

- Do not convert mobile to single-app mode.
- Do not hide window overlap by closing/minimizing other apps automatically.
- Do not use local-only proof for deployed behavior claims.
- Do not let Codex directly implement the UX product patches unless the mission records a substrate blocker that prevents Choir-in-Choir execution.
- Do not collapse multiple small UX goals into one giant unverifiable patch.
- Do not accept a worker self-report without independent verifier evidence.
- Do not claim Podcast is usable with native audio controls only.
- Do not use source/provenance/VText buttons as substitutes for playback controls.
- Do not require manual IDs for normal candidate/promotion evidence inspection.
- Do not weaken auth for mutations or persisted private state.

## Rollback

Rollback reference starts at pre-mission HEAD:

```text
cad9eaacabe927d19d8be6c0b064a6732c29c70c
```

Any landed commit must include a normal git rollback path and staging identity evidence.

## Stopping Condition

Stop successfully only when staging evidence shows:

- the sweep itself ran through Choir-in-Choir product-path delegation for the UX patches, with Trace-visible worker/verifier evidence;
- logged-out users can open and explore Podcast without auth;
- Podcast has a usable mobile library/search/feed/player/progress path;
- mobile desktop windows remain floating/overlapping but can be reliably raised, fit/snapped, minimized/restored;
- Trace mobile evidence can be inspected without hunting through an unreadable page;
- VText and Trace can coexist without obvious flicker or focus regression;
- CI/deploy/staging identity and rollback refs are recorded.

Stop with a blocker only after root-cause probes and at least one cognitive search-space transform if an invariant-level issue prevents the next safe probe.

## Sequential Dispatch Queue

Dispatch one bounded goal at a time through the visible staging prompt bar. Do not start the next mutation goal until the previous candidate has export/promotion evidence or a precise blocker.

1. Mobile desktop window controls:
   - Preserve floating windows on mobile.
   - Improve raise/focus, minimized restore, show-desktop interaction, fit-to-screen, snap/restore affordances, and touch targets.
   - Verify with Trace + VText open together on a 390x844 viewport.

2. Logged-out read/explore boundary:
   - Allow Podcast public/explore launch and search while logged out.
   - Keep import/subscription/playback persistence behind auth.
   - Verify logged-out and logged-in paths separately.

3. Podcast app usability:
   - Add app navigation/back, library/search/feed hierarchy, scrollable episodes, sticky now-playing, play/pause, seek, speed, scrubber, and basic progress state.
   - Move RSS import, source, VText, and provenance behind secondary/inspect affordances.
   - Verify on mobile and desktop.

4. Trace mobile evidence readability:
   - Keep Trace a desktop app window, but make trajectory summary, run acceptance, evidence refs, timeline, and inspector reachable/readable in compact panes.
   - Verify long payload wrapping and inspector reachability.

5. VText coexistence/flicker:
   - Reproduce and fix any focused-edit flicker or selection loss when Trace is open and stream/head events arrive.
   - Verify VText + Trace together on mobile desktop.

6. Candidate/promotion UX follow-up if time remains:
   - Replace normal-use manual candidate ID flow with contextual candidate cards from current Trace/promotion/run evidence.
   - Keep manual ID entry in advanced/developer mode.

Each goal should produce:

- exact prompt submitted;
- trajectory/run ids;
- worker/vsuper/cosuper evidence;
- patch/export/promotion candidate refs or blocker;
- verifier result;
- screenshots/DOM metrics;
- rollback ref;
- residual risk and whether the next queue item should proceed.

## Operator Log

### 2026-05-18: Queue Item 1 Product-Path Dispatch

Outer Codex dispatched mobile desktop window controls through the visible staging prompt bar instead of editing product code locally.

- Trajectory: `c450d6cf-91e4-49d5-a02b-de7cfbf644b3`
- VText doc: `d6edfaf3-536e-4f84-b3ca-0367c1c27eeb`
- Worker VM: `vm-43a120ad5f50c578f7fb524645f5fcb6`
- Worker id: `worker-da6eaac92d4e6812`
- Worker loop: `5fbdcfc3-d1a5-4ba5-8d47-854e382cbd03`
- Vsuper agent: `beff512e-0ea7-459d-a533-09afeb83ff47`
- Implementation co-super: `640dfe01-bf3c-460a-8da7-b43616f5e09b`
- Verifier co-super: `f3ce79a5-e2d0-4f55-9292-95d614bd8644`
- Queued promotion candidate: `2e137069-c346-458b-ac4b-fb9bc74a067d`

Result: delegation substrate is alive, but the candidate is not safe to promote.

The initial export at worker head `9093a7e8694f5d132aa9815a8ca2f36d6998b106` failed verifier review because Trace and VText can overlap on a `390x844` viewport with no visible raise/focus affordance for a covered non-minimized window. Vsuper then made a repair at worker checkout head `3ff345b44c1e1fb5d84ab1824965618b412f8d98`, adding all-open-window task indicators and passing static/build checks, but `export_patchset`/promotion queue still returned the stale verifier-failed `9093a7e...` artifact and patch SHA `dbe3a2a5c3fe98eaf6ec7d753135f630fff41714c5a7b2a68d1fc72acd057f00`.

Disposition: do not promote candidate `2e137069-c346-458b-ac4b-fb9bc74a067d`. The next sequential dispatch must repair or precisely isolate the worker export reuse/current-HEAD mismatch before continuing to Podcast or Trace app UX work.

### 2026-05-18: Export-Identity Repair Dispatch Exposed Worker Lease Liveness Blocker

Outer Codex dispatched a second product-path goal through the visible staging prompt bar to repair or isolate the stale export/current-HEAD mismatch from trajectory `c450d6cf-91e4-49d5-a02b-de7cfbf644b3`.

- Trajectory: `d26b0f17-10e8-475f-a714-1c60e72fc6fd`
- VText doc: `7f9180c9-31de-4e6a-838a-4ea39d27be49`
- Worker VM: `vm-0eb8b202f09fd19d28e662bd682f15be`
- Worker id: `worker-d86592159ae19647`
- Worker loop: `6f82e8eb-b9b6-4937-bcd7-3968c07c0fa7`
- Worker sandbox URL: `http://172.196.0.2:8085`

Result: this run did not reach export repair. It exposed a lower substrate bug in worker lease reuse.

The first `delegate_worker_vm` attempt returned structured failure evidence after polling the worker loop, with state still reported as `running` but runtime status unreachable:

```text
delegate_worker_vm status: Get "http://172.196.0.2:8085/internal/runtime/runs/6f82e8eb-b9b6-4937-bcd7-3968c07c0fa7?owner_id=08a6c1a3-9e6c-4fed-9f72-33360efb09c0": dial tcp 172.196.0.2:8085: connect: no route to host
```

The super retry called `request_worker_vm` again, but the runtime returned the same unreachable worker handle with `dedupe_reason=super_run_already_requested_worker_vm`; the next `delegate_worker_vm` submit failed with the same `no route to host` sandbox URL.

Disposition: this is a legitimate direct Codex platform repair under the Execution Model because the Choir-in-Choir substrate prevented the next sequential UX candidate. The runtime dedupe path must treat unreachable worker delegate results as a poisoned lease: clear the in-process request cache, ignore stale `request_worker_vm` run-log results for that worker, ask vmctl for a fresh worker, and preserve explicit replacement evidence.

Local substrate patch evidence:

- Added regression coverage for a closed worker sandbox followed by a repeat `request_worker_vm` in the same super run.
- `delegate_worker_vm` now records `worker_request_cache_invalidated` for unreachable worker-runtime submit/status failures.
- `request_worker_vm` now records `replaced_unreachable_worker_request` when it bypasses an invalidated lease and requests a fresh worker.
- Focused tests passed with documented ICU flags:

```text
go test -count=1 ./internal/runtime -run 'TestSuperRequestWorkerVMReplacesUnreachableLeaseAfterDelegateFailure|TestSuperRequestWorkerVMDedupesSameRunDifferentPurposes|TestSuperRequestWorkerVMReusesActiveLeaseUnlessParallelAllowed|TestDelegateWorkerVMReturnsSubmitFailureEvidence'
go test -count=1 ./internal/runtime ./internal/vmctl -run 'TestSuperRequestWorkerVM|TestDelegateWorkerVMReturnsSubmitFailureEvidence|TestOwnershipRegistry_RequestWorkerReusesActiveLeaseUnlessParallelAllowed|TestHandler_RequestWorker|TestClient_RequestWorker'
```

Next step after deploy: rerun the export-identity repair dispatch on staging. If a fresh reachable worker is allocated, continue to repair the stale export/current-HEAD mismatch before proceeding to Podcast/Trace UX queue items.

### 2026-05-18: Podcast UX Dispatch Exposed Host Pressure Reclaim Blocker

Outer Codex dispatched a fresh product-path goal for Podcast logged-out explore plus mobile usability through the staging prompt bar.

- Trajectory: `c5907854-221d-4462-8199-d849d3bc74fd`
- VText doc: `e3fb7fdd-c013-42b1-964d-1a753ce4aa3a`
- Worker VM: `vm-033c5402ce46776bb601624a36ee740b`
- Worker id: `worker-db1ac9cc8eef386a`

Result: the run reached worker delegation, then staging ingress became intermittently unavailable. Public `/health`, `/api/trace`, and `/api/prompt-bar/submissions` timed out during TLS/HTTP access, while direct Node B checks showed local services healthy. Caddy then core-dumped and restarted. Node B health showed sustained memory pressure with 41 active Firecracker VMs and zero reclaim-eligible VMs because stale worker VMs whose old purposes mentioned `verifier`, `promotion`, or `rollback` were classified as `critical_protected` forever.

Disposition: this is a substrate blocker for completing any multiagent UX sweep reliably. The repair changes critical worker protection from permanent to time-bounded: critical-purpose workers remain protected during recent activity, but stale critical workers become pressure-reclaim eligible after the protection window. Focused vmctl tests cover both recent critical-worker protection and stale critical-worker reclaim.
