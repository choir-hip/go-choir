# MissionGradient: Worker VM Liveness And Delegate Evidence v0

Status: ready for execution
Date: 2026-05-16
Operator: outer Codex supervising Choir through staging, git, CI, deploy, and product-path proof

## One-Line Goal String

```text
/goal Run docs/mission-worker-vm-liveness-and-delegate-evidence-v0.md as a Codex-operated MissionGradient mission: use investigate and cognitive-transform discipline to root-cause and repair the worker sandbox runtime liveness regression that prevents super -> worker VM -> vsuper delegation from reaching co-super coordination; do not stop merely because a lower-level blocker is observed. Start from the deployed structured-evidence fixes at 73150df, correlate fresh staging prompt-bar delegate failures with vmctl worker state, worker sandbox health/logs, runtime run records, provider/tool-loop activity, deploy/code identity, and timeout/restart paths; patch the implicated platform layer, tests, prompt contracts, or diagnostics, then commit, push main, monitor CI/deploy, verify staging identity, and rerun the same visible prompt-bar Choir-in-Choir workload. Continue receding-horizon investigate -> fix -> deploy -> product-proof loops until Trace, VText, and run acceptance show vsuper coordinating worker and verifier co-super agents over real channels with export/reviewable candidate evidence, or until a hard external/invariant blocker remains after named root-cause probes with durable worker events/log refs, compaction/continuation status, rollback refs, residual risks, and the next executable probe. Preserve logged-out read/explore usability, avoid forbidden internal/test acceptance routes, forbid fake islands, carry timeout_seconds through worker delegation, and keep the path from 200 tool loops toward 1000 or budget-governed no-fixed-cap execution explicit.
```

## Mission Shift

The previous mission did useful work: it made blocked `delegate_worker_vm`
results visible and acceptance-safe. That is no longer enough.

This mission treats a sandbox/runtime blocker as an invitation to investigate
and repair. A blocked run may be an intermediate evidence checkpoint, but it is
not a terminal outcome unless:

- an external authority boundary prevents the next diagnostic or fix;
- an invariant would need to change;
- the system is unsafe to mutate further without owner review;
- or the mission has executed the named root-cause probes and records a precise
  lower-level platform blocker with enough evidence for the next operator to act.

If the next objective can be defined, execute it. Do not stop only because a
clean next objective can be written.

## Research Basis

Latest deployed commits:

- `462cc14af229ea4b1bd5c57a1e8e38a5afebc9d5` raised the tool-loop cap and
  preserved failed/pending worker delegate evidence.
- `73150dfc3d28829e332c7390b93bed0bb79ef682` made worker submit/status timeout
  paths return structured non-error blocker results.

Fresh staging evidence:

- evidence directory:
  `.gstack/evidence/worker-vsuper-delegate-termination-fresh-2026-05-16T17-06-03-324Z`
- staging commit: `73150dfc3d28829e332c7390b93bed0bb79ef682`
- trajectory: `9295a55c-ac0c-4a78-9adc-b3883a2c05bf`
- VText doc: `e8a71e6b-ae80-4772-9ab7-ced7de6f7f10`
- run acceptance: `runacc-a650c805c4eb4bc9bfa1`, `staging-smoke-level`,
  `blocked`
- worker: `worker-c3635c319b83564d`
- worker VM: `vm-5ef2faa870b9d275ef8a71984041c8d3`
- worker sandbox URL: `http://172.92.0.2:8085`
- first delegate worker run:
  `fe3e94e9-c801-4637-9d95-c0a83b8a712b`
- first delegate result:
  `worker_run_status_failed`, state `running`, status request timed out
- worker event fetch also timed out
- subsequent delegates to the same worker returned `worker_run_submit_failed`
  because the worker sandbox did not answer submit requests
- no `vsuper`, worker co-super, verifier co-super, export, compaction, or
  continuation evidence was observed

Important nuance:

- The structured parent evidence fix worked. The remaining problem is not that
  the parent forgot the failure.
- The worker accepted the first run, then its runtime became unavailable to
  status, events, and later submit calls.
- A warm authenticated run had similar failures, but a fresh-account run proved
  the new code was active in the worker path and still hit sandbox liveness.
- The auto-chained `delegate_worker_vm` created after `request_worker_vm` still
  omitted `timeout_seconds`, even though later manual delegate calls included it.
- The system previously had successful worker/delegate proofs in nearby docs,
  but those proofs must be compared by substrate: local host-process worker,
  staging Firecracker worker, same-runtime worktree fallback, and deployed
  worker VM are not interchangeable proof levels.

## Real Artifact

The artifact is the deployed worker-delegation substrate:

```text
visible staging prompt bar
-> conductor
-> VText mission/task document
-> foreground super
-> request_worker_vm
-> delegate_worker_vm(profile=vsuper, timeout_seconds carried)
-> worker VM sandbox runtime
-> vsuper
-> worker co-super plus verifier co-super over channels
-> export/reviewable candidate or precise blocker
-> parent Trace, VText, and RunAcceptanceRecord evidence
```

The immediate artifact is not a polished UX issue. Trace readability and
prompt-bar ergonomics remain the test workload after substrate liveness is
recovered.

## Invariants

- Staging `https://draft.choir-ip.com` is the acceptance environment.
- Product-path acceptance uses visible prompt-bar flow and public authenticated
  product APIs such as `/api/prompt-bar`, `/api/vtext/*`, `/api/trace/*`,
  `/api/promotions/*`, `/api/continuations/*`, and
  `/api/run-acceptances/*`.
- Browser-public acceptance must not use `/api/agent/*`, `/api/prompts`,
  `/api/test/*`, `/internal/*`, raw event mutation endpoints, direct service
  ports, or manual success seeding.
- Diagnostic work may inspect vmctl state, worker sandbox health, worker logs,
  systemd/deploy logs, runtime stores, and direct worker endpoints when needed.
  Diagnostic evidence guides fixes but does not substitute for product-path
  acceptance.
- Foreground super may do read-only, ephemeral, or low-risk diagnostics.
  Choir app/harness/repo/runtime/candidate/export/promotion mutation must go
  through the intended platform path or be done by outer Codex as a platform
  fix with git/CI/deploy proof.
- Canonical user state changes only by verified promotion. Failed workers are
  discarded or left as evidence, not laundered into success.
- Worker/co-super/verifier messages must be real channel/runtime evidence, not
  transcript placeholders.
- Fake islands are forbidden: no fake transclusion panels, fake candidate
  exports, fake promotion records, decorative Trace cards, or summaries that
  hide the missing substrate.
- Behavior-changing fixes must complete:

```text
commit -> push origin main -> monitor CI -> monitor staging deploy
-> verify staging commit identity -> run deployed product-path acceptance
```

## Value Criterion

Minimize the distance between intended delegation topology and durable staging
evidence while preserving authority boundaries, rollback, and honest failure
semantics.

Better states:

- fresh worker VMs expose stable runtime health before delegation;
- the first delegated vsuper run can be submitted, observed, and either
  completed or precisely blocked;
- worker status and event endpoints remain responsive during long tool loops;
- worker runtime code/deploy identity is visible per worker, not inferred only
  from proxy `/health`;
- delegate retry policy does not hammer a known-unhealthy worker;
- `request_worker_vm` and auto-chained delegation carry `timeout_seconds`;
- run acceptance captures pending, failed, blocked, and successful delegate
  transitions causally;
- vsuper either starts worker/verifier co-supers and exports evidence, or
  reports a specific capability/runtime blocker before budget exhaustion;
- compaction and continuation status are explicit for long-running runs;
- the path from 200 loops to 1000 or budget-governed no-fixed-cap execution is
  tied to cancellation, lease lifetime, cost, compaction, and evidence
  backpressure.

Worse states:

- more retry/sleep padding without identifying why the worker runtime stopped
  responding;
- accepting proxy `/health` as proof that every persistent worker computer is
  on the deployed code;
- claiming export/promotion readiness without channel and candidate evidence;
- stopping with only "sandbox timed out" when logs, runtime records, or code
  instrumentation could be collected next.

## Investigate Kernel

Use the investigate skill as the operating loop:

1. Investigate: collect facts before changing code. Correlate one failing run
   across product Trace, worker id, VM id, sandbox URL, vmctl ownership, worker
   process health, runtime logs, run store, provider/tool-loop state, and
   deploy identity.
2. Analyze: separate symptoms from causes. Distinguish submit path, status path,
   event path, provider hang, tool-loop hang, runtime HTTP starvation, VM
   networking, process restart, old-code worker, and resource pressure.
3. Hypothesize: name competing explanations and the observation that would
   falsify each one.
4. Implement: patch the smallest implicated layer, add tests or instrumentation,
   deploy it, then rerun product-path proof.

Iron law: no speculative fix without a root-cause hypothesis and a falsifiable
probe. A small diagnostic patch is acceptable if it directly increases evidence
resolution.

## Cognitive Transforms

Current obstacle:

The system can now preserve parent-side evidence for worker delegate failure,
but the worker sandbox becomes unresponsive after or during the first delegated
run, preventing vsuper/co-super topology from forming.

Selected transforms:

1. Depth extraction: the blocker is not "timeout." The hidden mechanism is loss
   of observability/liveness at the worker runtime boundary.
2. Substrate split: compare local host-process, same-runtime worktree, staging
   Firecracker worker, and warm persistent user computer as different
   substrates. Do not merge their evidence.
3. Causal graph inversion: start from the exact failing HTTP transition and walk
   backward to worker process, run loop, provider call, VM networking, and
   deployment identity.
4. Anti-Goodhart: do not make the acceptance record cleaner unless the runtime
   is actually more controllable.
5. Receding-horizon autonomy: a blocker that defines the next safe probe should
   immediately spawn the next probe inside the same mission.

Changed plan:

- implementation: prioritize worker runtime liveness, per-worker identity,
  timeout propagation, unhealthy-worker quarantine/restart, and event/log
  capture before UX changes.
- verifier/evidence: require correlated Trace plus vmctl/worker/log evidence
  for the same worker run id.
- scope: only move to Trace/onboarding candidate work after a delegated vsuper
  can remain observable.
- stopping condition: do not stop at "blocked" unless the next probe crosses an
  external/invariant boundary or has been executed and still leaves a precise
  hard blocker.

Next high-information action:

Create a fresh product-path worker delegate run, then immediately correlate the
same worker id and run id with worker `/health`, runtime logs, vmctl ownership,
process restart evidence, provider request state, and runtime store entries.
If any of these are not externally accessible, add the smallest platform
diagnostic endpoint or log surface needed, behind the existing internal/server
trust boundary, then deploy and rerun.

## Starting Belief State

Believed current state:

- Foreground prompt-bar, VText, super, worker lease, and delegate invocation
  work on staging.
- Parent `delegate_worker_vm` now preserves submit/status failures as structured
  evidence.
- Worker sandbox liveness is the next blocker. It can accept the first run but
  becomes unable to answer status, event, and later submit requests.

Main uncertainties:

- Is the worker sandbox process alive but blocked, crashed and restarted,
  network-isolated, provider-blocked, store-locked, or resource-starved?
- Does the worker HTTP server share blocking state with the long-running tool
  loop in a way that starves status/events?
- Are worker VMs booting the deployed code, or can warm/persistent workers run
  older images after deploy?
- Does the first delegated run create a provider/tool call that wedges the
  runtime before it can persist events?
- Does `request_worker_vm` dedupe hand back a stale or already-unhealthy worker?
- Does missing `timeout_seconds` on the auto-chained delegate shorten the first
  observation window and trigger repeated retries into a bad worker?

Highest-impact uncertainty:

```text
Why does the worker sandbox stop answering status/events after accepting the
first delegated vsuper run?
```

Next observation:

For one fresh run id, prove whether the sandbox process is alive, what it is
doing, whether the run is persisted, whether the provider/tool loop is active,
and whether the worker code identity matches the deployed commit.

## Receding-Horizon Control

Operate in short loops:

1. Pick one liveness hypothesis.
2. Predict a concrete observation.
3. Run the smallest diagnostic.
4. Patch or instrument only the implicated layer.
5. Run focused tests.
6. Commit and push behavior changes.
7. Monitor CI and staging deploy.
8. Verify staging commit identity.
9. Rerun the same visible prompt-bar workload.
10. Update belief state and either continue or stop under the hard stopping
    rules.

If a run returns a structured blocker and the next probe is obvious, execute the
next probe. Do not turn the probe into only a final-report recommendation.

## Dense Feedback Channels

- local unit tests for `request_worker_vm`, `delegate_worker_vm`, worker
  status/event polling, unhealthy-worker handling, timeout propagation, and run
  acceptance synthesis;
- staging `/health` for proxy, upstream, vmctl, pressure, warmness, and commit
  identity;
- per-worker code identity from the worker runtime itself;
- vmctl ownership, worker state, VM id, sandbox URL, warmness class, and restart
  generation;
- worker sandbox `/health` and runtime run-status/event endpoints as
  diagnostics;
- worker process logs around submit/status/event timestamps;
- provider/gateway request attempt evidence for the delegated run;
- runtime store records for the worker run id;
- Trace moments for request/delegate/result/pending events;
- VText revisions and worker updates;
- run acceptance record synthesized from existing evidence;
- screenshots only for Trace/readability once substrate proof is unblocked.

## Control Priorities

1. Reproduce the liveness failure on staging from the visible prompt bar with a
   fresh user/session when needed.
2. Correlate the failure with worker process, vmctl, logs, run store, provider,
   and deployed-code identity.
3. Add per-worker identity and liveness evidence if it is missing.
4. Fix or quarantine unhealthy worker reuse. A worker that times out on
   status/events should not receive repeated delegate submits without restart,
   recovery, or explicit diagnostic classification.
5. Carry `timeout_seconds` through `request_worker_vm` `next_required_args` and
   through required auto-chained delegation.
6. Ensure `delegate_worker_vm` differentiates:
   - submit unreachable;
   - submit accepted but status unreachable;
   - status alive but run still active;
   - event endpoint unreachable;
   - worker run failed/blocked;
   - parent timeout;
   - worker runtime restart.
7. If the worker HTTP server can be starved by a long provider/tool loop, repair
   runtime concurrency or move status/event observation to a control path that
   remains alive.
8. If the worker is killed/restarted, repair restart detection and run
   resumption or produce a structured `worker_runtime_restarted` blocker.
9. If provider/gateway calls hang the worker, add bounded provider timeout and
   visible gateway attempt evidence.
10. Once worker liveness is recovered, rerun the Choir-in-Choir workload until
    vsuper coordinates worker and verifier co-super agents over real channels.
11. Only then spend mutation budget on Trace readability or prompt-bar/window
    ergonomics as the self-development workload.
12. Synthesize run acceptance and VText report from durable evidence.

## Expected Platform Work

Likely changes, subject to investigation:

- add per-worker `/health` fields for deployed commit, boot generation, process
  start time, active run count, current run ids, and provider/tool-loop state;
- make vmctl expose or log worker restart generation and last health transition;
- add worker runtime log capture keyed by worker id, VM id, and run id;
- carry `timeout_seconds` from worker request to chained delegation;
- add unhealthy-worker quarantine/restart semantics after status/event timeout;
- make `delegate_worker_vm` return `worker_runtime_unreachable`,
  `worker_runtime_restarted`, or `worker_status_unavailable` with precise
  phase and refs;
- prevent repeated identical delegate submits to an already-unhealthy worker
  unless the retry path first restarts or revalidates the worker;
- add tests for status endpoint timeout after accepted submit, event endpoint
  timeout, restart classification, timeout propagation, and run acceptance
  preservation;
- audit whether 200 tool-loop iterations is still only a safety ceiling and
  define the telemetry needed to move to 1000 or budget-governed no-fixed-cap;
- improve compaction/continuation evidence only after the liveness path is
  observable enough to run long enough to need it.

## Acceptance Targets

Success target:

- pushed commit reaches green CI and staging deploy;
- staging `/health` reports the pushed commit;
- visible staging prompt-bar run creates VText task evidence;
- Trace shows super requesting a worker VM and delegating to vsuper;
- worker runtime remains observable through submit, status, and event polling;
- Trace or linked worker evidence shows vsuper coordinating worker and verifier
  co-super agents over channels;
- worker produces export/reviewable candidate evidence or a precise blocker
  from inside the worker runtime;
- run acceptance reaches `export-level` when export evidence exists, or a
  truthful blocked level if the remaining problem is lower-level;
- VText final report names trajectory/run/acceptance ids, worker/VM ids,
  verifier contracts, rollback refs, residual risks, and next objective.

Clean hard-blocker target:

- all named root-cause probes that remain inside authority boundaries have been
  executed;
- blocker identifies the failing layer, not just the symptom;
- evidence includes Trace event refs, worker id, VM id, worker run id when any,
  worker code identity or reason it could not be read, vmctl state, worker
  health/log refs or reason absent, delegate phase, terminal error, and run
  acceptance id;
- the next probe is executable by a future agent without rediscovering context;
- no local-only or internal-route diagnostic is presented as acceptance.

## Rollback

Code rollback:

- revert the mission commits on `origin/main`;
- monitor CI and staging deploy;
- verify staging health identity after rollback;
- rerun a small prompt-bar smoke if behavior changed.

State rollback:

- discard failed worker/candidate computers rather than promoting them;
- do not promote any candidate without verifier evidence and owner review;
- keep VText and run acceptance blockers as evidence, not canonical success.

Known rollback references at mission start:

- pre-termination-fix base:
  `a4aec9899131c73f663bad72255072a442a2483e`
- structured evidence commits:
  `462cc14af229ea4b1bd5c57a1e8e38a5afebc9d5`
  and `73150dfc3d28829e332c7390b93bed0bb79ef682`

## Stopping Condition

Stop only when one of these is durable:

1. Deployed staging proof shows delegated vsuper coordination with worker and
   verifier co-super channel evidence plus export/reviewable candidate evidence,
   VText report, run acceptance, CI/deploy identity, and rollback refs.
2. A hard lower-level platform blocker remains after investigate/cognitive
   transform loops and named root-cause probes. The blocker must include worker
   liveness evidence, vmctl/runtime/log/run-store refs or exact access limits,
   run acceptance, rollback refs, residual risks, and a next probe that is
   smaller than "debug sandbox."

Do not stop simply because the run can write a good next mission. If the next
mission is executable under the current authority boundary, execute it as the
next receding-horizon loop.
