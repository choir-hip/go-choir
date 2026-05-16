# MissionGradient: Worker Vsuper Substrate Recovery v0

Status: ready for execution
Date: 2026-05-16
Operator: outer Codex supervising Choir through staging, git, CI, deploy, and product-path proof

## One-Line Goal String

```text
/goal Run docs/mission-worker-vsuper-substrate-recovery-v0.md as a Codex-operated MissionGradient mission: investigate and repair or precisely isolate the worker VM runtime restart/timeout path that blocks super -> worker VM -> vsuper delegation; use staging product-path prompt-bar and Trace evidence as the acceptance path, use vmctl worker health, worker sandbox runtime logs, and delegate_worker_vm submit/status behavior as diagnostics, land required platform fixes through git/CI/deploy, then rerun the same Choir-in-Choir sweep-shaped workload until Trace shows vsuper coordinating worker and verifier co-super agents over channels with export/promotion or reviewable candidate evidence, or records a lower-level worker-runtime blocker with durable VText, Trace, run-acceptance, rollback refs, residual risks, and the next safe probe.
```

## Real Artifact

The artifact is the deployed Choir-in-Choir delegation substrate:

```text
visible staging prompt bar
-> conductor
-> VText mission report
-> super
-> request_worker_vm
-> delegate_worker_vm(profile=vsuper)
-> worker VM runtime
-> vsuper
-> worker co-super + verifier co-super over channels
-> candidate/export/promotion or precise blocker evidence
-> run acceptance record
```

The immediate product problem is not Trace polish or onboarding copy. Those are
the pressure workload after substrate proof. The substrate is uphill when a real
staging prompt can cause durable/risky Choir work to leave the foreground and
enter a worker-VM `vsuper` world where worker and verifier agents can coordinate
with evidence.

## Starting State

Previous v2 run evidence:

- deployed prompt-boundary fix and acceptance-support commits reached staging;
- latest deployed commit after the fix-forward loop:
  `cd126205351e37995cda5ca9a0b5ce671e111bb4`;
- GitHub Actions run: `25957115265`, all jobs passed;
- final staging `/health` reported proxy and sandbox deployed commit
  `cd126205351e37995cda5ca9a0b5ce671e111bb4`;
- main v2 evidence directory:
  `.gstack/evidence/overnight-v2b-2026-05-16T07-19-38-121Z`;
- v2 trajectory: `8745307c-c524-4d2b-b6f4-8a0e5ce1ac68`;
- v2 VText doc: `58cf2353-031a-4d8f-bdcf-62be0ae59130`;
- v2 VText head revision:
  `e8d2ac61-6fdb-4754-bf2a-3e435be4bd75`;
- worker lease succeeded:
  `worker-aff792db60131e0b` / `vm-36d7af1312ba3159f8c704eda9b6f655`;
- `delegate_worker_vm` was attempted 6 times and failed with worker runtime
  restarts and submit/status timeouts before `vsuper` could establish
  worker/verifier co-super topology;
- v2 run acceptance: `runacc-2097e341b76d6c62cbba`, `staging-smoke-level`,
  `blocked`;
- final post-deploy smoke after `cd12620`:
  `.gstack/evidence/postdeploy-acceptance-smoke-2026-05-16T08-26-03-920Z`;
- post-deploy run acceptance:
  `runacc-80784b35e21616f40e77`, `staging-smoke-level`, `blocked`.

Learning from v2:

- The prompt boundary is now good enough for `super` to request a worker VM.
- The blocker moved down one layer: worker VM delegation cannot reliably submit
  or observe a `vsuper` run.
- Run-acceptance synthesis now has a path to record blocked
  `delegate_worker_vm` results, but the v2 record could not be re-synthesized
  after session refresh rotation. New proof should preserve live auth state or
  immediately synthesize acceptance before the session rotates away.
- The prompt-bar submission response exposes VText `decision.initial_loop_id`
  as a loop id, not a trajectory id. Harness code must distinguish those.

## Value Criterion

Minimize divergence between the intended delegation event graph and deployed
Trace evidence while preserving product-path proof, authority boundaries,
foreground/candidate separation, rollback, and run-acceptance causality.

Penalize:

- bypassing the visible prompt-bar/product route;
- treating Node B logs, direct worker URLs, or internal endpoints as acceptance
  proof rather than diagnostics;
- claiming co-super, export, promotion, or rollback evidence before Trace and
  durable records show it;
- patching symptoms without explaining the worker restart/timeout state;
- leaving acceptance synthesis, screenshots, logs, or VText unable to explain
  the next blocker.

## Quality Gradient

Expected quality: `solid`.

Solid means:

- one clear root cause or one sharply isolated lower-level blocker;
- narrow code changes with targeted tests;
- platform changes committed, pushed, CI-verified, deployed, and staging-health
  verified;
- deployed product-path proof rerun after changes;
- final VText, Trace, acceptance, rollback refs, and residual risks named.

Substandard work:

- adding more prompt pressure when the worker runtime is the failing layer;
- increasing retry counts without proving the worker can accept and complete a
  run;
- local-only proof for worker/vmctl/gateway/delegate behavior;
- UI or docs changes that make the mission look productive while the substrate
  still cannot create `vsuper`.

## Hard Invariants

- Staging `https://draft.choir-ip.com` is the acceptance environment.
- Behavior-changing fixes use:

```text
commit -> push origin main -> monitor CI -> monitor staging deploy
-> verify staging commit identity -> run deployed acceptance proof
```

- Do not edit tracked files directly on Node B as a source/config shortcut.
- Product-path browser proof may use `/api/prompt-bar`,
  `/api/prompt-bar/submissions/*`, `/api/vtext/*`, `/api/trace/*`,
  `/api/promotions/*`, `/api/continuations/*`, and
  `/api/run-acceptances/*`.
- Browser/public proof must not use `/api/agent/*`, `/api/prompts`,
  `/api/test/*`, `/internal/*`, raw event mutation endpoints, direct service
  ports, or manual success seeding.
- Diagnostic ops may inspect staging health, CI/deploy logs, vmctl state,
  worker sandbox logs, and service logs when available. These observations can
  guide fixes but cannot substitute for product-path acceptance.
- Super may do bounded scratch/API/script work directly when it is read-only,
  ephemeral, or low-risk. Choir/app/harness/repo/runtime/candidate/export/
  promotion work must cross the worker-VM/vsuper boundary.
- Foreground/canonical state remains stable until explicit verified promotion.
- Worker co-super does not verify its own work. Verifier co-super must inspect
  independently and message concrete pass/fail evidence over channels.
- Failed worker/candidate attempts must leave diagnostics, rollback/discard
  refs where applicable, and the next safe probe.

## Anti-Fake-Island Invariant

Allowed low-resolution proof is a projection of the real system:

- one deployed prompt-bar run that reaches `vsuper`;
- one `vsuper` run that creates worker/verifier co-super channels;
- one candidate/export blocker with exact worker-runtime evidence;
- one acceptance record that truthfully marks the boundary reached.

Forbidden shortcuts:

- fake transclusion panels, decorative citations, or placeholder Trace cards;
- manually inserted events, success records, run acceptance checkpoints, or
  promotion candidates;
- local worktree worker simulations presented as staging vmctl proof;
- direct worker `http://172.*` calls from the browser/public proof path;
- claiming `promotion-level` or `continuation-level` without verifier,
  owner-review, promotion/rollback, and continuation evidence.

## Homotopy Parameters

Increase realism only along topology-preserving axes:

- single product prompt -> repeated prompt after fix;
- one worker VM -> fresh worker VM after health classification;
- submit timeout evidence -> status polling evidence -> worker run events;
- `vsuper` starts -> `vsuper` spawns worker/verifier co-supers;
- no export -> export blocker -> concrete patchset export -> promotion queue;
- desktop Trace readability -> mobile Trace readability after substrate proof.

Do not jump from a failed worker submit to UX fixes. The next realism axis is
worker delegation health, not surface polish.

## Belief State

Starting belief:

- Prompt-boundary fixes are deployed and effective enough for super to lease a
  worker VM.
- The worker VM lease object exists and returns a sandbox URL, but the worker
  runtime either restarts during delegated runs or fails to respond to submit
  and status requests.
- The precise failing layer is unknown: possible causes include worker boot
  readiness, runtime process restart, gateway/token/service routing, request
  timeout, run persistence during restart, or delegate polling semantics.

Highest-impact uncertainty:

```text
Does a freshly leased staging worker VM expose a stable runtime that can accept,
persist, and complete a vsuper run long enough for delegate_worker_vm to observe
it?
```

Next observation that reduces uncertainty:

- correlate one fresh product-path `delegate_worker_vm` attempt with vmctl
  worker ownership state, worker sandbox health/logs, runtime run records,
  submit/status HTTP error class, and Trace tool events.

Update this belief state after every surprising observation before mutating.

## Receding-Horizon Control

Use short control intervals:

1. Choose one uncertainty: worker boot readiness, submit path, status path,
   runtime restart cause, timeout behavior, or evidence synthesis.
2. Predict the evidence if that uncertainty is the blocker.
3. Probe with the smallest noncanonical diagnostic or product-path run.
4. Patch only the implicated layer.
5. Run focused tests.
6. Commit, push, CI, deploy, staging health.
7. Rerun product-path prompt-bar proof.
8. Update VText/evidence/run acceptance and continue or stop.

Mutation radius:

- small, targeted platform fixes are allowed through git;
- no broad refactors unless the root cause proves the existing structure is
  wrong;
- no UX/onboarding patch until `vsuper` and co-super topology is proven or a
  lower-level worker-runtime blocker is durable.

## Control Priorities

### P0: Verify Baseline And Preserve Evidence

- Confirm current staging health reports `cd126205351e37995cda5ca9a0b5ce671e111bb4`.
- Snapshot prior evidence dirs and run ids in the new final report.
- Keep browser auth state fresh during new proof runs; synthesize run
  acceptance immediately after evidence appears.

### P1: Reproduce Or Classify Worker Delegation Failure

Use the visible staging prompt bar with the same candidate-world objective. The
goal is not a new UX fix. The goal is to reproduce the delegation edge with
fresh evidence.

Required observations:

- `request_worker_vm` result with worker id, vm id, sandbox URL, machine class;
- `delegate_worker_vm` invocation and result;
- submit/status phase, timeout class, run id if one was created;
- worker runtime health and logs around the same timestamps;
- whether the worker runtime records `vsuper` run creation before restart;
- whether failure is stale-worker-specific or fresh-worker-general.

### P2: Fix The Narrowest Proven Layer

Possible fix classes:

- worker readiness gating before returning a lease;
- vmctl worker health classification and stale-worker discard;
- gateway credential/token propagation;
- worker runtime startup/restart persistence;
- `delegate_worker_vm` submit/status timeout handling;
- trace/run-acceptance evidence preservation for failed delegates.

Each fix needs a targeted regression test matching the observed failure.

### P3: Rerun Product-Path Choir-In-Choir Proof

After deploy, submit the inner prompt below through the visible staging prompt
bar. Success requires Trace and durable evidence, not just a clean API call.

### P4: Only Then Use UX/Onboarding As Workload

If `vsuper` and co-super topology works, allow the worker/verifier pair to take
the smallest Trace readability or prompt-bar/window ergonomics improvement.

If topology still blocks before `vsuper`, do not patch UX. Record the lower
substrate blocker and next safe probe.

## Inner Choir Prompt

Submit through the visible staging prompt bar after any required platform fix is
deployed:

```text
Use Choir to safely improve one small part of Choir itself without changing the active computer directly. Create a VText mission report. This is app/harness/Choir-in-Choir development, so ask super for a background candidate computer instead of doing durable repo/runtime/app mutation in the foreground. In that candidate world, have vsuper coordinate separate worker and verifier co-super agents over agent-to-agent channels. The worker should make or precisely block the smallest safe improvement related to Trace readability or prompt-bar/window ergonomics. The verifier should independently inspect evidence and either pass it or message the worker with the concrete failing condition. Record exact VText revision ids, Trace trajectory/run ids, worker VM or candidate ids, worker/verifier messages, tests, export/promotion/rollback refs where available, blockers, residual risks, and the next objective. Super may do bounded scratch/API/script work directly if it is only ephemeral evidence gathering, but Choir/app/harness/repo/runtime/candidate/export/promotion work must go through the worker VM and vsuper boundary. If delegate_worker_vm fails, preserve the exact submit/status phase, worker run id if any, worker VM health, runtime restart evidence, and the next safe probe instead of falling back to foreground mutation. Do not use slash commands, forbidden internal/test routes, manual success seeding, direct service ports, local-only proof, fake transclusion panels, decorative citations, or canonical mutation without verification/promotion. Before stopping on a blocker, apply route-changing cognitive transforms and record the next safe probe. Before stopping successfully, do a quality pass.
```

## Dense Feedback Channels

- Git status, git log, CI run/job logs, deploy logs.
- Staging `/health` proxy/sandbox commit identity and vmctl health.
- Product prompt-bar submission status.
- Trace moments for conductor, VText, super, `request_worker_vm`,
  `delegate_worker_vm`, `vsuper`, `spawn_agent`, channel messages,
  `export_patchset`, promotion candidates, and VText revisions.
- Worker VM and runtime logs around delegate submit/status timestamps.
- Tests for vmctl concurrency/readiness, delegate worker submit/status,
  prompt-boundary routing, worker channel events, export, and run acceptance.
- Browser screenshots for logged-out desktop, auth-on-mutation, VText, Trace
  desktop, and Trace mobile when relevant.
- Browser request audit for forbidden public routes.

## Evidence Ledger

For each nontrivial claim, record:

```text
claim:
evidence source:
command or observation:
artifact path:
result:
uncertainty/caveat:
promotion relevance:
```

Minimum final evidence:

- pushed commit SHA(s);
- CI run and deploy job URLs;
- staging health/build identity;
- prompt-bar submission id;
- VText doc and revision ids;
- Trace trajectory/run ids;
- worker id, vm id, sandbox URL, machine class;
- `delegate_worker_vm` submit/status result;
- `vsuper`/co-super/channel/export/promotion evidence or exact blocker;
- run acceptance id and level;
- rollback refs;
- residual risks;
- next objective.

## Rollback Policy

- Platform code rolls back by git revert or fix-forward on `main`, CI, and
  staging deploy.
- Failed worker/candidate worlds are discarded/archived, not promoted.
- If a worker fix worsens vmctl routing, revert or fix-forward the specific
  vmctl/runtime change and verify staging health before further prompt-bar
  testing.
- No candidate output promotes without verifier contract evidence, owner review,
  and rollback/discard refs.

## Learning Side-Channel

Record tactical learnings in the mission final report and, when they change
future proof behavior, in focused tests or docs.

Classify surprises:

- Tactical: retry semantics, timeout classes, harness field names, auth-session
  renewal details. Apply directly.
- Target-level: worker readiness or acceptance-synthesis shape needs
  reparameterization. Patch mission or docs before continuing.
- Invariant-level: authority boundary, candidate promotion semantics, public API
  proof rules, or foreground/canonical mutation model appears wrong. Stop and
  escalate before changing it.

## Stopping Conditions

Stop successfully when:

- staging is on the latest required commit and health proves proxy/sandbox
  identity;
- product prompt-bar proof reaches either:
  - `super -> request_worker_vm -> delegate_worker_vm -> vsuper -> worker
    co-super + verifier co-super` with channel evidence; or
  - a lower-level worker-runtime blocker with submit/status/log/Trace evidence
    precise enough for the next fix;
- VText report names doc/revision ids, worker/candidate ids, channel/export or
  blocker evidence, tests, rollback refs, residual risks, and next objective;
- run acceptance is synthesized and truthfully states the reached level;
- browser request audit shows forbidden-route count `0`;
- code-review quality pass finds no unaddressed P0/P1 issues in touched code.

Stop negatively only when:

- the failing layer is named more precisely than "worker delegation failed";
- no smaller safe product-path or diagnostic probe remains;
- rollback/discard refs are recorded;
- the next safe probe is explicit.

