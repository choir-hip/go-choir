# MissionGradient: Supervision Runtime Repair And Experiment Rerun v0

**Status:** superseded by [mission-supervision-continuation-experiment-rerun-v0.md](mission-supervision-continuation-experiment-rerun-v0.md)
**Date:** 2026-05-23
**Supersedes immediate continuation of:** [mission-async-supervision-runtime-hardening-v0.md](mission-async-supervision-runtime-hardening-v0.md)
**Resumes:** [mission-human-proof-experiment-rerun-v0.md](mission-human-proof-experiment-rerun-v0.md)
**State ledger:** [platform-os-app-state.md](platform-os-app-state.md)

## Supersession Note

This mission records the earlier runtime repair trajectory. Its immediate
continuation has moved to
[mission-supervision-continuation-experiment-rerun-v0.md](mission-supervision-continuation-experiment-rerun-v0.md),
which starts from deployed checkpoint `0d321ac` and the Chiron proof showing
that active finish evidence is now preserved but supervision continuation still
stops too early at `worker_run_active`.

## One-Line Goal String

```text
/goal Run docs/mission-supervision-runtime-repair-experiment-rerun-v0.md as a Codex-operated MissionGradient mission: first repair and prove Choir's async supervision runtime, then restart the four human-proof experiments through Choir-in-Choir. Start from deployed runtime checkpoint 732eb4a and the async proof showing request/start/observe/finish use one worker_run_id, but worker submit_worker_update still fails with delivery target requester lookup, VText only receives fallback/stale supervision evidence, and run acceptance can confuse a no-package runtime proof with app-package failure. Root-cause and fix worker-update delivery/fanout so vsuper/co-super substantive updates reach both the live VText dashboard and supervising super without giving VText worker-control authority; keep super as sole redirect/cancel authority; preserve nonblocking start/observe/redirect/finish semantics; prevent duplicate starts; audit gateway/auth/502/timeouts as named runtime failures; improve Trace/run-acceptance signal for LLM content, tool calls, agent messages, worker events, cancellation, and terminal evidence; and establish a bounded screenshot/video/benchmark evidence-worker path rather than requiring Playwright in every VM or claiming Obscura replacement before it proves equivalent staging capability. Land runtime/harness/prompt/diagnostic fixes through git/CI/deploy, verify staging identity, and prove on staging through the visible product path with VText, Trace, run-acceptance, and screenshots that async worker supervision is live, narratable, and resumable. Only then rerun Chiron Shelf observability, process/window/agent animation language, Choir Liquid Material Engine, and Python code mode sequentially through Choir-developed candidate computers. Codex must not hand-code experiment features; if Choir-in-Choir fails, Codex fixes the runtime/harness/orchestration/prompting/evidence substrate and reruns the same experiment through Choir. Finish with owner-readable VText dashboards/reports, real screenshots/video/benchmarks, Trace/run-acceptance evidence, package/adoption/rollback refs or precise blockers, residual risks, and the next realism axis. If incomplete, report checkpoint_incomplete or blocked_incomplete, update this mission doc with a resumable checkpoint, and continue/redirect/delegate any safe executable next probe inside current authority before stopping.
```

## Mission Frame

The previous async-supervision preflight made real progress. Deployed staging
commit `732eb4a` proves that the product path can:

- request a worker VM;
- start one async worker run;
- observe the same `worker_run_id`;
- finish that same run without a duplicate successful start;
- block duplicate same-turn `start_worker_delegation` execution.

That is not enough to restart the four experiments. The proof also showed the
next runtime fault clearly:

```text
submit_worker_update failed:
resolve delivery target requester lookup: record not found
```

As a result, worker/vsuper updates are not reliably delivered to the live VText
dashboard or supervising super. VText can show stale or fallback narration even
when Trace has terminal worker evidence. That is exactly the observability gap
that caused the previous experiment portfolio to drift into machine receipts,
static reports, and weak human proof.

This mission repairs the supervision loop first. Then it restarts the four
experiments as a meta-experiment: Choir must develop the features inside
candidate/background computers while Codex supervises from VText, screenshots,
video, Trace, and run-acceptance evidence.

Codex may directly edit runtime, harness, orchestration, prompting, diagnostics,
Trace, VText, Apps & Changes review gates, and acceptance synthesis. Codex must
not hand-code Chiron, Motion, Liquid, or Python mode experiment features.

## Real Artifact

The artifact is a self-development loop that is observable by humans while it
runs:

```text
visible product prompt
  -> VText mission dashboard
  -> persistent super
  -> async worker VM / vsuper
  -> implementation and verifier co-supers
  -> substantive worker updates fan out to super + VText + Trace
  -> screenshots/video/benchmarks captured by bounded evidence workers
  -> candidate package/adoption/rollback evidence
  -> owner-readable report
```

The runtime part is not complete until a worker's substantive update is
delivered as a real causal update, not only as a fallback after terminal
collection.

The experiment part is not complete until each experiment has human proof:
narrative VText, media/benchmark evidence, and a plain-English recommendation.
Package refs and build receipts are supporting details, not the review object.

## Hard Invariants

- Super is the foreground control authority. Only super may redirect, cancel,
  or reassign worker/vsuper work from the foreground.
- VText may narrate uncertainty and ask super clarifying questions. VText does
  not issue worker-control commands.
- VSuper curates substantive owner-level checkpoints from co-super work.
- Co-supers report primarily to their supervising vsuper.
- Skip-level super-to-co-super directives are valid only when the supervising
  vsuper receives the same directive atomically.
- `request_worker_vm` leases; `start_worker_delegation` starts; `observe`,
  `redirect`, `cancel`, and `finish` are explicit bounded controls. No hidden
  synchronous delegate path is a success route.
- A worker run must not be counted as successfully started more than once for
  the same worker handle and objective.
- VText dashboard freshness is a required part of long-run supervision proof.
- Human proof gates must not treat package publication, recipient build, or
  run-acceptance receipts as working-feature evidence.
- Active computers are not mutated directly. Candidate/background computers
  mutate; active state changes only through verified adoption/promotion with
  rollback.
- Staging product proof is required for platform behavior claims.

## Starting Belief State

Known from the deployed proof at `732eb4a`:

- Async request/start/observe/finish can preserve a single worker run id.
- Same-turn duplicate `start_worker_delegation` is guarded.
- The old hidden blocking path is no longer the accepted proof route.
- `submit_worker_update` can still fail inside worker/vsuper runs because the
  delivery target resolver cannot find the requester record in the worker
  context.
- VText may receive synthesized or stale checkpoint content rather than the
  real worker update.
- Run acceptance still has app-package assumptions that can mark a pure runtime
  proof as blocked for the wrong reason.
- Gateway auth failures, route `502`s, worker timeouts, and browser-evidence
  gaps can still masquerade as experiment failures unless they are separated in
  Trace, VText, and run acceptance.
- Obscura can remain useful for extraction and lightweight browsing, but the
  current mission still needs a special browser-proof worker class for
  authenticated product actions, screenshots, and video unless Obscura proves
  the same capability on staging.
- The four experiments should remain paused until this supervision loop is
  repaired and proven on staging.

Highest-impact uncertainty:

```text
Can a real worker/vsuper/co-super update be delivered live to both VText and
super, incorporated into owner-readable VText, and observed in Trace/run
acceptance without blocking super or creating multiple controllers?
```

## Required Runtime Repair

Investigate from product evidence before patching. Start with the failing
`submit_worker_update` path:

- inspect worker-run metadata propagation from super to delegated worker/vsuper;
- inspect delivery target resolution for VText requester, super requester,
  channel id, trajectory id, and worker context;
- determine why requester lookup fails when the worker run is remote or
  isolated;
- decide whether the durable target should be requester metadata, a stable
  coordination channel, a VText dashboard id, or a typed copied update event;
- patch the smallest clean product path that preserves the authority model.

The expected shape is:

- worker/vsuper sends one substantive update;
- the update is durable and addressable;
- VText can consume it into a narrative revision;
- super can observe or receive the same update for supervision;
- Trace shows the update with useful signal, not only raw noisy events;
- no duplicate control signal is created;
- no worker-control authority is granted to VText.

Also repair or precisely scope:

- run-acceptance contracts so runtime-supervision proofs are not blocked by
  app-package requirements when no AppChangePackage was requested;
- VText terminal evidence synthesis so `finish_worker_delegation` completion is
  not contradicted by stale VText prose;
- worker event summaries so failed and pending worker-run evidence remains
  durable and owner-reviewable;
- prompts for super, vsuper, co-super, verifier, and VText so significant
  updates are sent at a useful cadence and style;
- Trace UI/API projections so LLM content, tool calls, agent-to-agent messages,
  worker updates, redirects, cancellation, and terminal evidence are easy to
  find.
- blocking/timeouts across gateway, worker VM route resolution, provider calls,
  delegate tools, and evidence capture so no long operation freezes super or
  gets flattened into a generic failed experiment;
- evidence-worker capability for screenshots/video/benchmarks: prefer a
  special Playwright-capable worker class when full browser proof is needed,
  not Playwright in every user/candidate VM; use Obscura only for the parts it
  demonstrably supports.

## Runtime Verification Gate

Do not resume experiments until staging proves:

- visible prompt-bar product path starts a worker delegation;
- `request_worker_vm`, `start_worker_delegation`, `observe_worker_delegation`,
  and `finish_worker_delegation` refer to one worker run id;
- duplicate start attempts are blocked before a second successful worker run is
  created;
- at least one worker/vsuper `submit_worker_update` succeeds;
- the live VText dashboard incorporates that update into a new revision;
- super receives or can observe the same update without losing control;
- Trace exposes the update, tool calls, and terminal worker evidence;
- run acceptance records the correct contract for runtime supervision;
- gateway/auth/502/timeout failures are either absent or separately visible as
  named blockers with next probes;
- screenshot/video capture for experiment evidence is available through a
  bounded evidence worker, or the mission records the missing capability as a
  blocker before restarting UI experiments;
- cancellation or redirect evidence is either proven or left as an explicit
  residual risk with the next probe;
- staging `/health` reports the pushed commit.

## Experiment Rerun

After the runtime gate passes, restart experiments one at a time. No
concurrency until sequential Choir-in-Choir is reliable.

### 1. Chiron Shelf Observability

Desired experiment:

- Shelf streams live tool calls, interim model messages, and agent-to-agent
  channel messages while work is happening.
- Desk menu, app buttons, and prompt input remain usable.
- Clicking into the prompt suppresses or de-emphasizes the Chiron stream.
- Trace or VText keeps a readable log of streamed items.

Proof:

- Playwright or worker-playwright video of a real run;
- screenshots showing non-blocking controls;
- VText narrative that explains what worked and what did not;
- Trace/run-acceptance refs;
- package/adoption/rollback refs or a precise blocker.

### 2. Process, Window, And Agent Animation Language

Desired experiment:

- boot, wake, app launch, window raise/minimize/restore, worker activity, and
  agent progress become more legible through motion;
- motion communicates state instead of adding decoration.

Proof:

- video of real process/window/agent transitions;
- reduced confusion in the narrative review;
- performance or jank notes;
- rollback path.

### 3. Choir Liquid Material Engine

Desired experiment:

- redesign real Choir desktop surfaces with a liquid/glass material treatment
  while preserving functionality;
- use WebGL/WebGPU-like acceleration only where it earns its cost;
- no private DOM capture or persisted preview screenshots.

Proof:

- screenshots/video of the actual desktop, not a poster;
- desktop and mobile resource benchmarks;
- fallback behavior on mobile Safari or unsupported GPU paths;
- security/privacy review;
- recommendation to promote, iterate, or abandon.

### 4. Python Code Mode A/B

Desired experiment:

- compare Python-mode arbitrary headless computer use against the existing bash
  tool in real agent/tool-loop conditions;
- benchmark time, token usage, tool-loop iterations, trace readability, error
  handling, and developer ergonomics.

Proof:

- benchmark artifacts;
- narrative VText explanation of tradeoffs;
- recommendation on whether Python replaces, complements, or is deferred.

## Dense Feedback

Required evidence sources:

- VText mission dashboard with multiple causal revisions;
- per-experiment VText reports written for the owner, not for the database;
- screenshots and videos for UI/motion/liquid/chiron experiments;
- benchmark JSON and plain-English benchmark summary for Liquid and Python;
- Trace trajectories focused on LLM content, tool calls, agent messages, and
  worker updates;
- run-acceptance records with the right acceptance level and caveats;
- candidate preview health records;
- browser-evidence worker records for screenshots/video, including whether the
  capture used Playwright, Obscura, or another bounded evidence path;
- recipient build and package/adoption/rollback records where install is
  attempted;
- staging commit identity after platform changes;
- mission doc checkpoint updates after major runtime proof or each experiment.

## Forbidden Shortcuts

- Do not hand-code experiment features with Codex.
- Do not restart the experiments before the worker-update/VText supervision
  path is fixed or precisely blocked.
- Do not let fallback synthesized updates masquerade as live worker updates.
- Do not call package/build receipts human proof.
- Do not hide `502`, auth, gateway, worker, preview, route, video, benchmark, or
  VText failures behind reviewable labels.
- Do not install Playwright in every VM just to paper over evidence capture.
- Do not claim Obscura replaces Playwright for this mission unless it proves
  authenticated product actions, screenshots, and video on staging.
- Do not use platform deploy as proof of user-computer/candidate experiment
  success.
- Do not reintroduce synchronous delegate waiting as a success path.
- Do not let VText and super both send conflicting worker-control messages.
- Do not run multiple experiments concurrently in this mission.
- Do not produce technical VText reports full of package ids and hashes as the
  owner-facing artifact.
- Do not call `checkpoint_incomplete` complete.

## Rollback Policy

Platform runtime/harness fixes must follow:

```text
commit -> push origin main -> monitor CI -> monitor staging deploy
-> verify staging identity -> run deployed acceptance proof
```

Every behavior-changing commit needs a rollback ref. If the supervision runtime
regresses active computers, worker delegation, VText revision, or Trace
inspection, revert the implicated commit and preserve the failed evidence.

Experiment package/adoption attempts must record:

- source package/change id;
- source and recipient computer refs;
- recipient build artifacts;
- behavior proof;
- active-state rollback ref;
- uninstall/disable limitations;
- residual private-data or capability risks.

## Stopping Condition

`complete` only when:

- runtime-supervision repair is deployed and proven on staging;
- live worker updates reach VText and super with clear authority semantics;
- Trace and run acceptance expose the right signals;
- the four experiments have been rerun sequentially through Choir-in-Choir, or
  each uncompleted experiment has a precise blocker with the next executable
  probe;
- all completed experiments have owner-readable VText reports, media or
  benchmark proof, Trace/run-acceptance refs, and promotion recommendations;
- rollback refs and residual risks are recorded.

Use `checkpoint_incomplete` when useful platform progress lands but the
stopping condition is not satisfied. The checkpoint must name the next safe
probe.

Use `blocked_incomplete` only after root-cause probes and cognitive transforms
leave an invariant-level or external blocker.

## Chiron Rerun Checkpoint: 2026-05-23

The first post-runtime-gate Chiron probe restarted the sequential experiment
through the visible product path. It did not prove Chiron. It did prove that the
next blocker is still in the supervision runtime, not in Chiron design.

Evidence:

- proof harness:
  `frontend/tests/chiron-sequential-product-proof.tmp.spec.js`
- evidence directory:
  `test-results/chiron-sequential-20260523T081544Z`
- Playwright trace:
  `frontend/test-results/chiron-sequential-product--3fb34-evidence-or-precise-blocker-chromium/trace.zip`
- source trajectory/submission:
  `d850d92a-b90d-48f3-842a-f9fa5d5d3a37`
- VText dashboard:
  `bcb8329e-ce45-426c-9bc5-5552fca3208f`
- VText head revision:
  `7454b343-205f-405f-8578-76ceca8f87a2`
- run acceptance:
  `runacc-86cb5ab95084483a9084`, accepted,
  `staging-smoke-level`
- worker VM:
  `vm-75580c3b67b14b95d055556e085fc2b4`
- worker:
  `worker-10476a1dd63bbe16`
- vsuper loop:
  `e9062cb7-fcff-4c9e-965d-a4cb4330cc95`
- implementation co-super loop:
  `7ee4ff0b-3588-4968-abca-f8cc0d827189`
- verifier co-super loop:
  `21ccc7e6-6bf0-487e-8f32-2fa4c38d16f7`

Outcome:

```text
outcome: no_matching_package
selected_package: null
source acceptance: accepted / staging-smoke-level
trace: 3 agents, 147 moments, 2 delegations, 0 evidence refs, 0 rollback refs
```

The VText dashboard was materially useful: it reported the candidate-world
topology, worker sandbox URL, active vsuper/co-super loops, base SHA, and the
worker's partial source-edit state. It also showed that implementation modified
`frontend/src/lib/BottomBar.svelte`, then hit `old_string not found` while no
package, screenshot, video, or precise blocker was returned.

Root cause found after the probe:

- `finish_worker_delegation` returned `worker_run_active` without fetching or
  preserving worker/child-run evidence.
- `observe_worker_delegation` already had the right evidence shape, but super's
  finish path could still lose the child tool failure context needed to
  redirect vsuper.
- Duplicate same-turn tool-call guards worked, but their errors added noise to
  the run and remain a prompt/runtime-tuning target.

Local runtime patch:

- `finish_worker_delegation` now fetches active worker evidence before returning
  `worker_run_active`.
- Active finish checkpoints to VText only when the active evidence is
  actionable: package evidence, child-run evidence, tool errors, or evidence
  fetch errors. Ordinary startup/root events and direct worker channel traffic
  are not synthesized into duplicate checkpoints.
- Regression test:
  `TestFinishWorkerDelegationActiveIncludesWorkerEvidence`.

Focused local proof in `nix develop`:

```text
go test ./internal/runtime -run 'TestDelegateWorkerVMToolRunsWorkerRuntimeAndCollectsExport|TestFinishWorkerDelegationMirrorsWorkerSubmitUpdateToActiveVText|TestFinishWorkerDelegationActiveIncludesWorkerEvidence|TestDelegateWorkerVMReturnsTimeoutRunEvidence' -count=1
go test ./internal/runtime -run 'Test.*Worker.*Delegation|TestDelegateWorkerVM|TestFinishWorkerDelegation|TestObserveWorkerDelegation|TestRedirectWorkerDelegation|TestCancelWorkerDelegation' -count=1
```

Both passed. A full `internal/runtime` package run remains noisy/expensive and
previously surfaced this same active-checkpoint interaction; use the focused
worker-delegation slice as the relevant pre-deploy guard unless the full suite
is being separately stabilized.

Next safe probe after deploy:

1. Push/deploy the active-finish evidence patch.
2. Verify staging identity.
3. Rerun only Chiron through the visible product path.
4. Require VText to show the worker child tool failure, package, or precise
   blocker while the worker is still active.
5. Do not proceed to Motion/Liquid/Python until Chiron either produces a real
   human-proof package/evidence path or a lower-level blocker with the worker
   events visible in VText/Trace/run acceptance.

## Run Checkpoint & Resumption State

```text
status: checkpoint_incomplete — runtime gate passed, first Chiron rerun isolated
  an active-finish evidence gap
last checkpoint:
  2026-05-23 staging proof at commit 846cfbb closed the worker-update and
  runtime-acceptance gap from the earlier 732eb4a checkpoint. The deployed
  product path now reaches request/start/observe/finish with one worker_run_id,
  mirrors a worker submit_worker_update into the active VText channel, wakes
  VText, and synthesizes a runtime-supervision run acceptance without requiring
  an AppChangePackage.
current artifact state:
  Async worker delegation is proven enough to resume the sequential
  Choir-in-Choir experiment rerun. The four experiment features themselves have
  not restarted and remain unproven.
what shipped:
  - 9bad6fd, 497f691, 72beeaf, and 732eb4a shipped the initial async
    supervision surface.
  - 490f70a repaired worker-update target resolution/fanout and runtime
    supervision acceptance.
  - 846cfbb repaired runtime-supervision acceptance ordering.
ci/deploy:
  - CI/deploy run for 846cfbb:
    https://github.com/yusefmosiah/go-choir/actions/runs/26327200933
  - Deploy job:
    https://github.com/yusefmosiah/go-choir/actions/runs/26327200933/job/77507019637
  - staging /health reported proxy and upstream commit
    846cfbbf2eb47206c6262d0ab032845c013ff8eb, built at 20260523074113,
    deployed at 2026-05-23T07:43:07Z.
what was proven:
  Local focused proof:
  - internal/runtime tests for submit_worker_update requester metadata,
    finish_worker_delegation worker-update mirroring, and runtime-supervision
    run acceptance passed in nix develop.
  Deployed product proof:
  - first deployed proof:
    test-results/async-supervision-runtime-proof-846cfbb-20260523T075607Z
  - VText-wait deployed proof:
    test-results/async-supervision-runtime-proof-846cfbb-vtextwait-20260523T080251Z
  - Playwright trace/video for the VText-wait proof:
    frontend/test-results/async-supervision-runtime--aad53-evidence-or-precise-blocker-chromium/trace.zip
    frontend/test-results/async-supervision-runtime--aad53-evidence-or-precise-blocker-chromium/video.webm
  - trajectory/submission id:
    2d45d210-cce7-4276-9ec8-b68d62cafb68
  - VText dashboard doc:
    b7663242-616b-4a23-a80f-bc7065f059fb
  - accepted run acceptance:
    runacc-0addeeafd0abe7c9154d
  - worker run:
    6e9eaaf3-5119-4318-8dde-a74e91a65a7b
  - worker VM:
    vm-2e6c63b2b834b6441c324cb32f82d24f
  - worker:
    worker-c38f1d6d33760bd2
  - final VText head revision:
    192cfee2-2601-4664-b945-db4eeb94e95f
  - run acceptance state:
    accepted / staging-smoke-level
  - worker_update_checkpoint:
    worker_submit_update_mirrored
  - mirrored_worker_update_count:
    1
  - VText head incorporated the marker, worker refs, command evidence, and a
    request/start/observe/finish status dashboard after waiting for the
    worker-update synthesis run.
unproven or partial claims:
  - The four experiments have not been rerun.
  - Browser evidence capture was proven through the outer Codex Playwright
    proof harness, not yet by a product-requested `worker-playwright` evidence
    worker. The code and Nix profile for `worker-playwright` exist, but the next
    UI experiment should treat screenshot/video production as a product-path
    evidence requirement and record any worker-playwright failure explicitly.
  - Redirect/cancel/resume semantics are implemented in the broader async
    runtime work but were not part of this terminal worker-update proof.
  - Trace is still noisy as a human review surface, though the needed tool
    calls, channel messages, and terminal worker evidence are durable.
belief-state changes:
  The central blocker was not VText wake itself. The stale-dashboard observation
  came from reading VText immediately after run acceptance. When the deployed
  proof waited for the worker-update synthesis run, VText produced the expected
  owner-readable dashboard revision.
remaining error field:
  Product-path screenshot/video evidence capture inside Choir workers; noisy
  Trace review; experiment-specific prompts and proof gates; possible
  duplicate tool-call attempts inside worker turns, which are currently guarded
  and visible but should be reduced by prompt/runtime tuning.
highest-impact remaining uncertainty:
  Whether Chiron can now be built by Choir-in-Choir, with the feature work done
  inside a candidate/background computer and supervised primarily through live
  VText narrative plus media evidence rather than Codex hand-coding.
next executable probe:
  Restart only the Chiron Shelf observability experiment through the visible
  product path. Require the live VText dashboard to update after substantive
  worker steps, require screenshots/video or a named worker-playwright blocker,
  require Trace/run-acceptance refs, and do not proceed to Motion/Liquid/Python
  until Chiron proves the sequential loop or records a precise blocker.
suggested resume goal string:
  Continue this mission from the runtime-gate-passed checkpoint and run the
  Chiron experiment first; keep the one-line goal string in this file as the
  full mission envelope.
evidence artifact refs:
  - test-results/async-supervision-runtime-proof-732eb4a-20260523T025003Z
  - test-results/async-supervision-runtime-proof-846cfbb-20260523T075607Z
  - test-results/async-supervision-runtime-proof-846cfbb-vtextwait-20260523T080251Z
  - frontend/test-results/async-supervision-runtime--aad53-evidence-or-precise-blocker-chromium/trace.zip
  - frontend/test-results/async-supervision-runtime--aad53-evidence-or-precise-blocker-chromium/video.webm
rollback refs:
  - Revert 846cfbb if runtime-supervision run-acceptance ordering regresses.
  - Revert 490f70a if worker-update target resolution, mirroring, or VText
    dashboard wake regresses.
  - Revert 732eb4a and earlier async-supervision commits only if the async
    worker tool surface itself must be withdrawn.
```
