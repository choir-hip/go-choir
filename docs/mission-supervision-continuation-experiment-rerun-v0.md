# MissionGradient: Supervision Continuation And Experiment Rerun v0

**Status:** superseded by [mission-runtime-human-proof-experiment-rerun-v1.md](mission-runtime-human-proof-experiment-rerun-v1.md)
**Date:** 2026-05-23
**Starts from deployed runtime checkpoint:** `0d321ac34e52da47c6e5af4aae506765a19fdc4a`
**Supersedes immediate continuation of:** [mission-supervision-runtime-repair-experiment-rerun-v0.md](mission-supervision-runtime-repair-experiment-rerun-v0.md)
**Returns to:** [mission-human-proof-experiment-rerun-v0.md](mission-human-proof-experiment-rerun-v0.md)
**State ledger:** [platform-os-app-state.md](platform-os-app-state.md)

## Supersession Note

This mission records the continuation-runtime checkpoint that led to the
`e53cf19` staging proof. Its immediate continuation has moved to
[mission-runtime-human-proof-experiment-rerun-v1.md](mission-runtime-human-proof-experiment-rerun-v1.md),
which starts from the newer evidence: active worker/VText continuation and
recipient-build ref normalization are working, but the Chiron rerun still
blocked before AppChangePackage publication because verifier sequencing and
browser-proof worker capability are not reliable enough for human-reviewable
experiment output.

## One-Line Goal String

```text
/goal Run docs/mission-supervision-continuation-experiment-rerun-v0.md as a Codex-operated MissionGradient mission: fix Choir's supervision continuation runtime, then restart the four human-proof experiments through Choir-in-Choir. Start from deployed checkpoint 0d321ac, where active worker finish evidence is preserved but the visible Chiron rerun still ended with no AppChangePackage because super stopped after worker_run_active while vsuper/co-super loops remained active. Root-cause and repair the continuation gap so active worker checkpoints cause owner-readable VText updates and safe super continuation/redirect until each worker reaches a real package, reviewable blocker, cancellation, or timeout certificate. Preserve super as the sole worker-control authority while allowing VText to ask super for clarification/continuation; make vsuper/co-super substantive updates fan out to VText, super, and Trace without duplicate controllers; prevent same-turn duplicate starts/spawns/tool calls from consuming the run; keep all long operations nonblocking and resumable; and make Trace/run-acceptance surface LLM content, tool calls, agent messages, worker events, active obligations, cancellations, and terminal evidence. Land runtime/harness/prompt/diagnostic fixes through git/CI/deploy, verify staging identity, and prove on staging through the visible prompt path with VText, Trace, run acceptance, screenshots, and, where available, video that async supervision continues to terminal evidence rather than stopping at worker_run_active. Only then rerun Chiron Shelf observability, process/window/agent animation language, Choir Liquid Material Engine, and Python code mode sequentially through Choir-developed candidate computers. Codex may fix runtime/harness/orchestration/prompting/evidence substrate directly but must not hand-code experiment features. If Choir-in-Choir fails, Codex investigates the substrate failure, patches/deploys it, and reruns the same experiment through Choir. Finish with owner-readable VText dashboards/reports, real screenshots/video/benchmarks, Trace/run-acceptance evidence, package/adoption/rollback refs or precise blockers, residual risks, and the next realism axis. If incomplete, report checkpoint_incomplete or blocked_incomplete, update this mission doc with a resumable checkpoint, and continue/redirect/delegate any safe executable next probe inside current authority before stopping.
```

## Mission Frame

The last runtime repair landed a useful checkpoint. Staging at `0d321ac`
proves that `finish_worker_delegation` no longer hides the active worker state:
when the worker is still running, the returned evidence includes the worker run,
worker VM, vsuper/co-super state, worker events, and a checkpoint source.

That is still not enough for the experiment rerun. The visible Chiron probe
created a VText dashboard and started a worker/vsuper/co-super run, but after
`finish_worker_delegation` returned `worker_run_active`, persistent super ended
without driving the active worker obligation to terminal evidence. The worker
had no AppChangePackage, the implementation and verifier co-super loops were
still running, and the product proof eventually reported
`no_matching_package`.

The new root problem is supervision continuation:

```text
active worker evidence exists
-> VText narrates partial state
-> super does not reliably continue observing/redirecting
-> worker/vsuper/cosuper obligations remain nonterminal
-> no human-proof experiment output exists
```

This mission fixes that runtime control loop, then restarts the four
experiments as the test of the loop. Codex may directly repair the platform
substrate. Codex must not hand-code the experiment features. The experiments
are the payload that proves Choir can improve itself.

## Current Evidence

Deployed checkpoint `0d321ac`:

- GitHub Actions run `26329301806` passed and deployed to staging.
- `/health` on `https://draft.choir-ip.com` reported proxy and sandbox build
  commit `0d321ac34e52da47c6e5af4aae506765a19fdc4a`.
- Staging Chiron product-path proof ran for 47 minutes and passed as a test
  harness, but its product outcome was `no_matching_package`.
- Evidence directory:
  `test-results/chiron-sequential-0d321ac-20260523T094559Z`.
- Trace/submission id:
  `cfb4cf60-1b1a-4b83-be74-2e70c8fdce5e`.
- Worker run id:
  `fa3dfff9-bb94-402e-aae8-92948e4ff532`.
- Worker id:
  `worker-4316a294bf32eae8`.
- Worker VM id:
  `vm-b4f7d3f19ba830115aad8a0704bdc4b2`.
- VSuper agent id:
  `d4d245dc-97ec-42ff-ab84-1cbb88455aa2`.
- Implementation co-super:
  `a7c789d5-8293-4a2e-84e9-3a17dc65e5b3`.
- Verifier co-super:
  `7bea350e-3d33-4a33-84e0-dade9c181b2f`.
- Source run acceptance:
  `runacc-f2b4bad9d660f7f1c17c`, `staging-smoke-level`, `blocked`.

What improved:

- Active `finish_worker_delegation` now preserves worker and child-agent
  evidence instead of returning an empty active state.
- VText produced a human-readable dashboard revision with the active worker
  state.
- Trace recorded worker ids, vm ids, child agent ids, delegation moments, and
  duplicate same-turn tool errors.

What failed:

- Super stopped after seeing `worker_run_active` instead of continuing the
  supervision loop.
- Both co-super loops were still active when the foreground run ended.
- No AppChangePackage, behavior proof, screenshot/video proof, or precise
  terminal blocker was produced by the Chiron experiment.
- Duplicate same-turn `start_worker_delegation`, `spawn_agent`, and `bash`
  attempts were rejected, but still consumed attention and trace noise.
- The proof produced screenshots, but no retained video artifact.

## Real Artifact

The artifact is a continuation-safe self-development loop:

```text
visible product prompt
  -> VText mission dashboard
  -> persistent super
  -> async worker VM / vsuper
  -> implementation and verifier co-supers
  -> substantive updates copied to VText + super + Trace
  -> super observes, redirects, cancels, or continues
  -> terminal package, reviewable blocker, cancellation, or timeout certificate
  -> owner-readable evidence packet
```

The runtime is not fixed until active worker obligations are durable,
narratable, and driven to terminal evidence without blocking a model tool loop.

The experiment rerun is not fixed until each experiment is produced by
Choir-in-Choir and has owner-readable human proof: VText narrative,
screenshots/video or benchmark, Trace/run-acceptance refs, and a clear
promotion recommendation or blocker.

## Hard Invariants

- Super is the only foreground worker-control authority.
- VText may ask super for clarification, continuation, or uncertainty
  resolution. VText does not redirect/cancel workers directly.
- VSuper owns candidate/background computer orchestration.
- Co-supers report primarily to their supervising vsuper.
- Skip-level super-to-co-super directives are valid only when the supervising
  vsuper receives the same directive atomically.
- A worker run cannot be called complete while `worker_run_active`,
  `finish_ready=false`, child loops are active, or required terminal evidence
  is missing.
- Long-running operations must be async, bounded, cancellable, and resumable.
- No hidden synchronous `request_worker_vm -> delegate_worker_vm` success path.
- No duplicate same-turn starts/spawns/tool calls should create additional
  successful work or drown out the useful evidence signal.
- VText dashboard freshness is part of the supervision contract.
- Human proof gates cannot treat package/build/run-acceptance receipts as a
  working feature demo.
- Candidate/background computers mutate. Active computers change only through
  verified adoption/promotion with rollback.
- Staging proof is required for platform behavior claims.

## Runtime Repair Targets

### Active Worker Continuation

Make active worker checkpoints actionable. When a tool result, worker update,
or synthesized checkpoint says:

- `worker_run_active`;
- `finish_ready=false`;
- child/co-super loops are active;
- no AppChangePackage exists;
- no terminal blocker exists;
- behavior proof is missing;

then the system must preserve that as an active obligation and cause a safe
continuation path. Acceptable v0 shapes include:

- VText writes a narrative revision and sends a clarification/continuation
  request to super with exact worker refs;
- persistent super's completion guard enqueues a follow-up inbox item for the
  same worker obligation after a bounded delay/backoff;
- `finish_worker_delegation` returns structured continuation obligations that
  the runtime records independently of model phrasing.

The preferred shape is VText-visible and super-controlled:

```text
active worker update
  -> VText revision: "still active, terminal evidence missing"
  -> VText -> super: "continue observing worker_run_id X"
  -> super observes or redirects
  -> repeat until package/blocker/cancel/timeout certificate
```

Avoid creating a second controller. VText requests; super decides.

### Worker Update Fanout

Substantive vsuper/co-super updates should be one durable update with typed
recipients, not disconnected copies:

- VText for owner-readable narrative;
- super for control/supervision;
- Trace for causal inspection;
- run acceptance for evidence synthesis.

The update should carry:

- worker run id and worker id;
- vm id and route health where relevant;
- source VText doc/revision target;
- coordination channel id;
- child/co-super state summary;
- latest tool calls and blockers;
- terminal evidence or active obligation;
- next requested supervision action, if any.

### Prompt Contracts

Patch prompts only where runtime structure supports them. The prompts should
say plainly:

- super must not end after `worker_run_active`;
- vsuper must submit substantive updates at real milestones and blockers;
- co-super must report to vsuper, not directly compete for super authority;
- VText should summarize the whole run so far, include uncertainty, and ask
  super for clarification/continuation when it cannot produce a faithful
  dashboard;
- verifier may run code and write scratch tests/evidence, but may not author
  candidate source, promote, or mutate active state.

### Trace And Run Acceptance Signal

Trace should still retain complete causal detail, but the default review path
must emphasize:

- LLM-generated content;
- tool calls and important tool results;
- agent-to-agent messages;
- worker updates;
- active obligations;
- redirects and cancellations;
- package/adoption/rollback evidence;
- terminal blockers.

Run acceptance must distinguish:

- runtime-supervision proof;
- human-proof experiment evidence;
- package/adoption evidence;
- continuation evidence.

Do not mark a UX experiment successful from runtime receipts alone.

### Evidence Workers

The experiment loop needs screenshots, video, and benchmarks. Do not require
Playwright in every user/candidate VM. Establish a bounded browser-proof worker
path:

- special Playwright-capable worker VM class for authenticated product proof,
  screenshots, and video;
- Obscura remains useful for extraction/light browser work only where it has
  proven equivalent capability;
- if video capture is unavailable, record that as a substrate blocker before
  claiming experiment reviewability.

## Runtime Acceptance Gate

Do not resume the four experiments until staging proves:

- visible prompt-bar path starts a worker delegation;
- `request`, `start`, `observe`, and `finish` reference one worker run id;
- duplicate start/spawn/tool attempts are blocked and do not create false work;
- a real vsuper/co-super substantive update reaches VText and super;
- VText creates a new owner-readable revision from that update;
- VText can ask super for continuation/clarification without controlling the
  worker directly;
- super continues or redirects after active worker evidence;
- the run reaches one terminal outcome: AppChangePackage, precise blocker,
  cancellation certificate, or bounded timeout certificate;
- Trace exposes the signal events without forcing owner review through raw
  noise;
- run acceptance records the right evidence level and does not conflate runtime
  proof with app-feature proof;
- staging `/health` reports the pushed commit.

## Experiment Rerun

Run experiments sequentially until the sequential loop is stable. Do not return
to concurrent experiments in this mission.

### 1. Chiron Shelf Observability

Desired experiment:

- Shelf streams live tool calls, interim model messages, and agent-to-agent
  channel messages while work is happening.
- Desk menu, app buttons, and prompt input remain usable.
- Clicking into the prompt hides or de-emphasizes the stream.
- Trace or VText keeps a readable log of streamed items.

Proof:

- video of a real run;
- screenshots showing non-blocking controls;
- VText narrative explaining what works and what does not;
- Trace/run-acceptance refs;
- package/adoption/rollback refs or a precise blocker.

### 2. Process, Window, And Agent Animation Language

Desired experiment:

- boot, wake, app launch, window raise/minimize/restore, background worker, and
  agent/process transitions feel legible and alive;
- motion does not block controls or hide important state;
- animation communicates causality rather than decoration.

Proof:

- video of several transitions;
- reduced-motion behavior;
- owner-readable VText report;
- performance notes.

### 3. Choir Liquid Material Engine

Desired experiment:

- real Choir desktop UI receives a material treatment while preserving existing
  functionality;
- implementation avoids private DOM capture and does not become a poster route;
- performance is benchmarked on desktop and mobile-class viewports.

Proof:

- before/after screenshots of real desktop/apps;
- video if animated;
- resource benchmark;
- privacy/security note.

### 4. Python Code Mode

Desired experiment:

- Python mode is evaluated as a real tool-loop/profile family alternative to
  bash for arbitrary headless computer use;
- benchmark compares time, token payload, loop iterations, error recovery, and
  trace readability;
- recommendation says whether Python should replace, supplement, or stay out
  of the profile family.

Proof:

- benchmark artifacts;
- VText narrative;
- Trace/run-acceptance refs;
- failure modes and security constraints.

## Dense Feedback

During the mission, maintain:

- one live mission dashboard VText;
- one VText report per experiment;
- screenshots and video links in those VTexts where available;
- Trace links for each substantive run;
- run-acceptance records for runtime gate and each experiment;
- a concise learning log about supervision/runtime behavior.

VText style should be owner-readable: plain English, narrative, current state,
what changed, what is blocked, and what happens next. Technical ids belong in
appendices or collapsible details.

## Forbidden Shortcuts

- Do not hand-code Chiron, Motion, Liquid, or Python mode experiment features
  with Codex.
- Do not call `worker_run_active` or `finish_ready=false` a completed run.
- Do not let VText control workers directly.
- Do not issue private skip-level commands to co-supers without copying vsuper.
- Do not treat screenshots of static posters as experiment proof.
- Do not treat package/build/run-acceptance receipts as working-feature proof.
- Do not keep stale static seed UI that labels changes reviewable without
  human proof.
- Do not accept 502 candidate previews as Try Live success.
- Do not require package ids, hashes, or internal refs as ordinary owner UI.
- Do not use local-only proof for platform behavior.
- Do not hide gateway/auth/timeout/video-capture failures inside generic
  experiment failure summaries.

## Rollback Policy

Every platform patch must have a normal git rollback ref:

```text
git revert <sha>
```

Every candidate/adoption proof must record:

- source computer/ref;
- recipient candidate/ref;
- recipient build artifacts;
- adopted active ref or blocker;
- rollback ref/profile;
- route/default-base impact if platform computer promotion occurs.

If the runtime patch worsens worker supervision, rollback the platform patch
before running more experiments.

## Run Checkpoint And Resumption State

```text
status: checkpoint_incomplete
last checkpoint:
  local continuation patch after deployed 0d321ac adds active worker
  obligations, VText-visible continuation guidance, and a persistent-super
  continuation message when worker evidence is still active/nonterminal.
current artifact state:
  active worker checkpoints are no longer only narrative evidence. They now
  record active_worker_obligation=true, carry continuation worker refs, and
  enqueue a runtime-supervision message to persistent super telling it to
  continue the existing worker instead of starting a duplicate.
what shipped:
  deployed 0d321ac preserves active finish evidence. The continuation patch is
  locally implemented and ready for git/CI/deploy.
what was proven locally:
  nix develop -c go test ./internal/runtime -run
  'TestFinishWorkerDelegationActiveIncludesWorkerEvidence|TestBuildAgentRevisionRequestRequiresSuperContinuationForActiveWorker|TestPersistentSuperProcessesConcurrentInboxDeliveriesInFollowupRun|TestFinishWorkerDelegationMirrorsWorkerSubmitUpdateToActiveVText|TestSubmitWorkerUpdateUsesVTextRequesterMetadataWhenAgentMissing|TestDelegateWorkerVMReturnsTimeoutRunEvidence'
  -count=1

  nix develop -c go test ./internal/runtime -run
  'Test.*Worker.*Delegation|TestDelegateWorkerVM|TestFinishWorkerDelegation|TestObserveWorkerDelegation|TestRedirectWorkerDelegation|TestCancelWorkerDelegation|Test.*WorkerUpdate|Test.*VTextWorker|Test.*PersistentSuper'
  -count=1

  nix develop -c go test ./internal/runtime -count=1
unproven or partial claims:
  staging deploy identity for the continuation patch; visible prompt proof that
  super follows the continuation message after worker_run_active; terminal
  worker evidence; experiment feature output; video evidence; package/adoption
  proof.
belief-state changes:
  the main blocker is now product-path proof of continuation, not local runtime
  mechanics: the local runtime can record an active obligation and request
  persistent-super continuation without creating a second controller.
remaining error field:
  persistent super still processes new inbox deliveries as follow-up runs when
  it is already active; duplicate same-turn attempts add noise; video/evidence
  worker path is not reliable.
highest-impact remaining uncertainty:
  will the deployed product path drive the same worker to terminal package,
  blocker, cancel, or timeout evidence after receiving the continuation request?
next executable probe:
  commit the continuation patch, push main, monitor CI/deploy, verify staging
  identity, then rerun the visible Chiron proof and inspect VText/Trace/run
  acceptance for post-worker_run_active super continuation.
suggested resume goal string:
  use the One-Line Goal String in this file.
evidence artifact refs:
  test-results/chiron-sequential-0d321ac-20260523T094559Z
  frontend/test-results/chiron-sequential-product--3fb34-evidence-or-precise-blocker-chromium/trace.zip
rollback refs:
  git revert 0d321ac34e52da47c6e5af4aae506765a19fdc4a if active evidence
  preservation causes regression.
```
