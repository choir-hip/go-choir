# MissionGradient: Runtime Human-Proof Experiment Rerun v1

**Status:** checkpoint_incomplete — local verifier sequencing and worker proof-tool PATH patch passes runtime tests; staging deploy/proof pending
**Date:** 2026-05-23
**Supersedes immediate continuation of:** [mission-supervision-continuation-experiment-rerun-v0.md](mission-supervision-continuation-experiment-rerun-v0.md)
**Returns to:** [mission-human-proof-experiment-rerun-v0.md](mission-human-proof-experiment-rerun-v0.md)
**State ledger:** [platform-os-app-state.md](platform-os-app-state.md)

## One-Line Goal String

```text
/goal Run docs/mission-runtime-human-proof-experiment-rerun-v1.md as a Codex-operated MissionGradient mission: repair the remaining Choir-in-Choir runtime gates, then restart the four human-proof experiments sequentially through Choir itself. Start from deployed checkpoint e53cf19, where active worker continuation, VText dashboard updates, and recipient-build ref normalization are working, but the Chiron rerun produced a useful blocker instead of a reviewable AppChangePackage because verifier co-super ran before implementation evidence and browser-proof evidence failed with missing Obscura/worker-playwright capability. First root-cause and fix verifier sequencing, stale-verifier replacement, browser-proof worker routing, worker image/PATH freshness, candidate route health, and human-proof publication gates. Codex may patch runtime, harness, prompts, Trace, VText, Apps & Changes gates, worker images, tests, and diagnostics, but must not hand-code Chiron, Motion, Liquid, or Python-mode experiment features. Land fixes through git/CI/deploy, verify staging identity, and prove through visible product-path VText/Trace/run-acceptance/screenshots/video where available that a Choir worker can reach terminal reviewable package evidence or a precise blocker without false success. Then rerun Chiron Shelf observability, process/window/agent animation language, Choir Liquid Material Engine, and Python code mode one at a time through Choir-developed candidate computers, using VText as the live owner-readable dashboard and media/benchmark evidence as the review layer. If incomplete, report checkpoint_incomplete or blocked_incomplete, update this mission doc with a resumable checkpoint, and continue/redirect/delegate any safe executable next probe inside current authority before stopping.
```

## Mission Frame

The previous runtime work made real progress. Staging at `e53cf19` proves that:

- active worker state can reach VText instead of disappearing;
- VText can produce owner-readable dashboard revisions during and after a worker run;
- `finish_worker_delegation` can preserve worker, vsuper, co-super, and child-run evidence;
- AppChangePackage recipient builds no longer pass product source-ledger tokens such as `git:go-choir-candidate@<sha>` directly to Git checkout.

The Chiron sequential proof at `e53cf19` also produced the right kind of failure: a blocker certificate rather than a fake success. That is useful, but it is not enough to restart the experiment portfolio.

Known current blocker field:

```text
implementation commit exists
-> verifier ran too early against base
-> browser proof failed because Obscura was missing or unreachable in worker context
-> no screenshot/video evidence
-> no AppChangePackage published
-> no recipient adoption
```

This mission fixes that narrower runtime/evidence gate. Then it reruns the four experiments through Choir-in-Choir. Codex is allowed to repair the substrate, but Codex is not allowed to hand-code the experiment features.

## Current Evidence

Latest deployed platform checkpoint:

- Commit: `e53cf1910a20780cf726f32c35cc8121bc2acfd2`
- GitHub Actions run: `26331553832`
- Deploy job: `77518474310`
- Staging health showed proxy and sandbox deployed commit `e53cf1910a20780cf726f32c35cc8121bc2acfd2`.

Latest Chiron product-path proof:

- Evidence directory: `test-results/chiron-sequential-e53cf19-20260523T114634Z`
- Outcome: `no_matching_package`
- Submission/trajectory id: `3cab7310-9bcb-4a95-a0d8-2050f8ee3b31`
- VText doc id: `a50d74a1-a9b3-4596-9349-42c58ac95c4b`
- Source run acceptance: `runacc-11208196d272779fbb69`, `staging-smoke-level`, accepted for runtime/proof-harness submission but not for experiment success
- Worker/vsuper run id: `401742d5-aa62-4e75-9feb-0a3c00fd681a`
- Worker id: `worker-e2f3d2a9f9bf5ed8`
- VM id: `vm-6c6011e05d9b7af8c6f5a93d413d89bb`
- Candidate implementation commit reported by worker: `ef5051b10f9e07e10fa0da418690c6ffc649b8db`
- Build evidence id reported by worker: `26ebb0fe-9fcb-4bdb-8a05-770305c50b9e`
- AppChangePackages: none
- Screenshot refs: none
- Video refs: none
- Fresh post-implementation verifier: missing

The latest VText dashboard correctly recommended: do not promote, do not treat as package-complete, and do not call the run a feature success.

## Real Artifact

The artifact is a reliable human-proof self-development loop:

```text
visible prompt path
  -> VText mission dashboard
  -> super starts and supervises async worker/vsuper
  -> implementation co-super changes candidate source
  -> verifier co-super starts only after implementation evidence exists
  -> browser-proof worker captures screenshots/video or precise blocker
  -> VText narrates whole run in plain English
  -> Trace exposes LLM content, tool calls, and agent messages
  -> run acceptance records the right evidence level
  -> AppChangePackage publishes only if human proof and verifier evidence exist
  -> recipient build/adoption/rollback proof
```

The experiment output is not reviewable until the owner can inspect a narrative VText, screenshots/video or benchmark artifacts, and a plain-English recommendation. Package ids, commits, digests, and worker ids are supporting evidence, not the owner-facing proof.

## Hard Invariants

- Codex may repair runtime, harness, prompts, evidence gates, worker images, and diagnostics directly.
- Codex must not hand-code Chiron, Motion, Liquid, or Python-mode experiment features.
- Experiments run sequentially until the sequential loop is stable. No concurrency in this mission.
- Super remains the foreground worker-control authority.
- VText may ask super for clarification and narrate uncertainty. VText does not control workers directly.
- VSuper owns candidate-world orchestration.
- Co-supers report primarily to vsuper.
- Verifier must not inspect a mutable candidate before implementation evidence exists.
- If an early verifier has already completed against base, its result is stale. Spawn exactly one replacement verifier after implementation evidence rather than treating stale failure as terminal.
- Browser-proof evidence must be real screenshots/video or a precise blocker. Static posters, generated reports, fixture-only markup, package/build receipts, and `502` candidate previews are not feature proof.
- AppChangePackage publication requires owner-readable human proof, valid verifier evidence, rollback refs, and no known missing required evidence.
- Active computers are not mutated directly. Candidate/background computers mutate; active state changes only through verified adoption/promotion with rollback.
- Staging product proof is required for platform behavior claims.

## Runtime Repair Targets

### 1. Verifier Sequencing

Patch prompts and, where necessary, runtime guardrails so `vsuper` does not spawn a verifier into a race.

Required behavior:

- implementation co-super starts first;
- vsuper waits for commit, package, blocker, or other explicit implementation evidence;
- verifier starts only after that evidence exists;
- verifier objective includes the exact commit/package/evidence to inspect;
- stale verifier output is labeled stale if it predates implementation evidence;
- one replacement verifier may run after implementation evidence;
- no duplicate verifier storms.

### 2. Browser-Proof Worker Path

The experiment loop needs screenshots and video. Do not put Playwright into every VM, but make a bounded proof path reliable.

Required behavior:

- ordinary worker VMs expose the browser/extraction tools they claim to expose, including Obscura if prompts tell agents to use it;
- stale or warm worker VMs cannot silently keep old images after a platform deploy when the mission requires current browser tools;
- `worker-playwright` is available as the special class for authenticated screenshots/video and Playwright traces;
- if a non-Playwright worker needs browser proof, it can either invoke a proven Obscura path or request/hand off to a browser-proof worker without losing candidate evidence;
- missing browser capability is reported as a named substrate blocker, not as a failed experiment feature.

### 3. Worker Image And PATH Freshness

Root-cause why the Chiron worker reported `obscura: command not found` even though the deployed guest image is expected to include Obscura.

Probe and fix:

- worker VM image generation and deployed-image identity;
- hibernated/warm worker reuse across deploys;
- service environment propagation into bash/tool subprocesses;
- `CHOIR_OBSCURA_BIN`, `OBSCURA_BIN`, and `PATH` inside worker tool calls;
- worker image status diagnostics surfaced to VText/Trace/run acceptance.

### 4. Human-Proof Publication Gates

Keep the successful blocker behavior, but make the positive path reachable only when it is real.

Required gates:

- no `reviewable` status without narrative VText plus media/benchmark evidence;
- no package publication without verifier evidence after implementation commit;
- no install/adoption readiness without recipient build plus behavior proof;
- no `Try Live` success when candidate route is `502`, boot-pending, auth-failed, or stale;
- technical refs stay available under details, not as the primary owner UI.

### 5. Trace And VText Signal

VText is the primary supervision surface for long runs. Trace is the forensic surface.

Required behavior:

- VText receives substantive updates after implementation start, implementation commit/blocker, verifier start/result, evidence capture, publication, adoption, rollback, and terminal blocker;
- each VText revision summarizes the whole run so far, not only the latest delta;
- VText body stays owner-readable and puts raw ids in an appendix;
- Trace default views prioritize LLM-generated content, tool calls, agent-to-agent messages, worker updates, redirects, cancellations, browser-proof artifacts, and terminal evidence;
- run acceptance links the latest VText doc/revision and media/benchmark artifacts.

## Runtime Acceptance Gate

Do not restart the four experiments until staging proves:

- visible prompt path starts exactly one worker/vsuper run for a test objective;
- implementation co-super produces a commit/package/blocker before verifier inspection;
- verifier evidence is post-implementation or is explicitly marked stale and replaced once;
- worker tool environment can find Obscura when ordinary worker prompts claim it, or the system routes browser proof to `worker-playwright`;
- screenshot/video proof is available for a browser-proof task, or a precise substrate blocker is recorded before package publication;
- VText dashboard updates at every substantive milestone;
- Trace exposes the signal events without requiring raw-log archaeology;
- run acceptance records runtime-supervision proof separately from feature/package/adoption proof;
- staging `/health` reports the pushed commit.

## Experiment Rerun

After the runtime gate passes, rerun the experiments one at a time through visible product-path Choir-in-Choir.

### 1. Chiron Shelf Observability

The Shelf should stream real tool calls, interim model messages, and agent-to-agent/channel messages while work is happening. It must not block the Desk menu, app buttons, prompt input, or window controls. Focusing the prompt should hide, pause, or de-emphasize the Chiron stream.

Proof: VText narrative, screenshots, video of a real run, Trace signal view, run acceptance, package/adoption/rollback refs or precise blocker.

### 2. Process, Window, And Agent Animation Language

Motion should make boot, wake, app launch, window raise/minimize/restore, worker activity, and agent/process transitions legible. It should communicate causality rather than decoration.

Proof: video of real transitions, reduced-motion behavior, performance notes, VText report, rollback path.

### 3. Choir Liquid Material Engine

Liquid should redesign real Choir desktop surfaces while preserving functionality. It must not be a poster route, private DOM capture, or persisted screenshot trick.

Proof: before/after screenshots of actual desktop/app surfaces, video if animated, desktop/mobile resource benchmark, Safari/WebGL/WebGPU/fallback note, security/privacy note.

### 4. Python Code Mode

Python mode should be evaluated as a real tool-loop/profile family alternative to bash for arbitrary headless computer use.

Proof: benchmark comparing time, token payload, loop iterations, error recovery, trace readability, and failure modes; VText narrative; recommendation to replace, supplement, or reject.

## Dense Feedback

Maintain:

- one live mission dashboard VText;
- one owner-readable VText report per experiment;
- screenshots and video links in VText when available;
- benchmark links for Liquid and Python mode;
- Trace links for each substantive run;
- run-acceptance records for runtime gate and each experiment;
- a concise learning log about MissionGradient behavior, especially where VText supervision helped or failed.

## Forbidden Shortcuts

- Do not hand-code experiment features with Codex.
- Do not run experiments concurrently in this mission.
- Do not spawn verifier before implementation evidence exists.
- Do not treat stale verifier output as valid verification.
- Do not treat missing Obscura, missing Playwright, `502`, boot pending, or auth failures as feature failures.
- Do not publish packages without human proof and post-implementation verifier evidence.
- Do not treat package/build/run-acceptance receipts as working-feature proof.
- Do not accept static screenshots, posters, fixture-only markup, generated reports, or fake Chiron text.
- Do not ask owners to paste package ids in ordinary UI.
- Do not use `export_patchset` or `/api/promotions`.
- Do not use local-only proof for platform behavior.

## Rollback Policy

Every platform patch must be rollbackable by Git:

```text
git revert <sha>
```

Every candidate/adoption proof must record:

- source computer/ref;
- candidate commit or blocker;
- verifier result;
- human-proof media/benchmark refs;
- package id if published;
- recipient build/adoption refs if adopted;
- rollback ref/profile;
- route/default-base impact if platform computer promotion occurs.

If runtime fixes worsen worker supervision, rollback the platform patch before running more experiments.

## Run Checkpoint And Resumption State

```text
status: checkpoint_incomplete
last checkpoint:
  e53cf19 deployed. Recipient-build ref normalization landed and staging
  health verified. Chiron rerun produced an owner-readable blocker instead of
  false success. A local continuation patch now removes the prompt/runtime
  ambiguity that let verifier co-super inspect before implementation evidence.
current artifact state:
  runtime can preserve active worker evidence and VText can narrate the run.
  Deployed runtime now blocks vsuper verifier spawning until an implementation
  co-super has reached terminal commit/package/blocker evidence. Worker shell
  tool environments append configured Obscura/Playwright binary dirs to PATH,
  and prompts require PATH/CHOIR_OBSCURA_BIN/OBSCURA_BIN diagnostics before
  treating browser proof as unavailable. A follow-on local patch has now
  isolated a second runtime blocker: super's worker-VM lease dedupe was scoped
  only to owner/desktop/super-run, so a later request for worker-playwright in
  the same run could silently reuse the existing worker-small lease.
what shipped:
  e53cf1910a20780cf726f32c35cc8121bc2acfd2 normalizes source-ledger git tokens
  before recipient build checkout. 915bb74eb3b0f71ebc71f73560060d9173ebb3a1
  shipped the verifier sequencing guard plus worker browser-tool PATH
  hardening; GitHub Actions run 26333349912 passed and deployed to staging, and
  /health reported proxy and sandbox deployed_commit 915bb74eb3b0f71ebc71f73560060d9173ebb3a1.
  The worker-lease machine-class dedupe patch is local and not yet committed,
  pushed, deployed, or proven on staging.
what was proven:
  staging Chiron proof reached VText dashboard and worker/vsuper evidence, and
  correctly withheld AppChangePackage publication when fresh verifier and
  screenshot/video evidence were missing.
  The 915bb74 staging rerun reached a better VText dashboard with marker
  chiron-seq-1779541877681 and leased worker-small
  worker-6984208e295e75bd / vm-fe8392c7000649bb0b987a70d837859f, but no
  worker-playwright lease was created. Independent vmctl ownership inspection
  showed only the worker-small lease for that trajectory, proving the browser
  evidence handoff remained blocked before package review.

  Local tests for the shipped 915bb74 patch:
  - nix develop -c go test ./internal/runtime -run 'TestVSuperSpawnAgentEnforcesActiveChildBudget|TestVSuperVerifierSpawnRequiresCompletedImplementation|TestWorkerVSuperDelegateContractPreventsCheckoutRaces|TestToolCommandEnvUsesPersistentScratchRoot|TestToolCommandEnvAddsConfiguredBrowserToolDirsToPath' -count=1
  - nix develop -c go test ./internal/runtime -count=1
  Local tests for the worker-lease dedupe patch:
  - nix develop -c go test ./internal/runtime -run 'TestSuperRequestWorkerVMDedupesSameRunByMachineClass|TestSuperRequestWorkerVMDedupesSameRunSamePurpose|TestSuperRequestWorkerVMReplacesUnreachableLeaseAfterDelegateFailure|TestToolCommandEnvAddsConfiguredBrowserToolDirsToPath|TestVSuperVerifierSpawnRequiresCompletedImplementation' -count=1
  - nix develop -c go test ./internal/runtime -count=1
unproven or partial claims:
  staging deploy identity for the worker-lease dedupe patch; product-path proof
  that super can request a distinct worker-playwright after worker-small
  implementation evidence; reviewable Chiron; Motion, Liquid, and Python
  experiments; recipient adoption; rollback.
belief-state changes:
  the runtime is now failing at a narrower evidence gate rather than losing
  worker state entirely. The positive path needs deployed sequencing and a
  super-managed browser-proof worker handoff, not another broad async rewrite.
  The prior `obscura: command not found` was narrowed by PATH hardening; the
  current proof failure is the lease-dedupe/product-supervision path that kept
  browser evidence on worker-small instead of creating worker-playwright.
remaining error field:
  staging verification of machine-class-specific worker leases; proof prompt
  may still need to make the second super-managed browser evidence delegation
  explicit enough; no experiment media proof.
highest-impact remaining uncertainty:
  can a product-path Choir worker produce implementation evidence, then a fresh
  verifier and real browser-proof media, then publish exactly one reviewable
  AppChangePackage without Codex hand-coding the feature?
next executable probe:
  commit the worker-lease machine-class dedupe patch, push main, monitor
  CI/deploy, verify staging identity, then rerun Chiron through visible product
  path with an explicit two-stage implementation/proof-worker objective until
  it publishes one reviewable AppChangePackage or records a precise blocker
  with media-capability evidence.
suggested resume goal string:
  use the One-Line Goal String in this file.
evidence artifact refs:
  test-results/chiron-sequential-e53cf19-20260523T114634Z
  test-results/chiron-sequential-915bb74-20260523T131116Z
rollback refs:
  git revert e53cf1910a20780cf726f32c35cc8121bc2acfd2 for the latest recipient
  build normalization patch if it causes regression.
  git revert 915bb74eb3b0f71ebc71f73560060d9173ebb3a1 for the verifier/PATH
  runtime patch if it causes regression.
```
