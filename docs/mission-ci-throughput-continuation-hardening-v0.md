# MissionGradient: CI Throughput + Continuation Hardening v0

Status: ready for execution
Date: 2026-05-17
Operator: outer Codex improving GitHub Actions/deploy throughput, then using the faster landing loop to continue Choir-in-Choir continuation and termination hardening through staging evidence

## One-Line Goal String

```text
/goal Run docs/mission-ci-throughput-continuation-hardening-v0.md as an 8-hour Codex-operated MissionGradient mission: first improve the GitHub Actions and staging deploy loop without weakening evidence quality, ensuring docs-only commits do not run CI/CD, speeding up Go especially internal/runtime tests by profiling and removing avoidable slow paths while preserving coverage, and speeding up the Node B NixOS deploy switch path by investigating and safely eliminating redundant rebuild/switch work; land required fixes through git, push main, monitor CI/deploy, and verify staging identity. Then spend the recovered loop capacity on the next Choir self-development objective: prevent duplicate vsuper/co-super spawn/cast/export attempts, force worker/vsuper runs to terminate with export/reviewable candidate or precise blocker, and produce continuation/run-memory evidence through the visible staging prompt bar. Stop only with durable CI/deploy timing evidence plus VText/Trace/run-acceptance/screenshots/DOM metrics showing export/promotion or continuation-level progress, or a precise blocker after root-cause probes, cognitive search-space transforms, rollback refs, residual risks, and the next executable probe.
```

## Mission Shift

The previous mission reached deployed `export-level` proof and exposed a
remaining continuation/termination frontier. It also made the cost of every
platform loop obvious: CI runtime, full Go test time, and Node B deploy/switch
latency now tax every receding-horizon probe.

This mission groups the work because the CI/deploy loop is part of the agentic
substrate. Faster, quieter landing is not separate polish; it increases the
number of real staging proof cycles available during the overnight window.

Do not let the throughput objective replace product proof. The optimized
pipeline must still preserve the same authority, CI, deploy, and staging
identity guarantees for behavior-changing commits.

## Current Belief State

Known deployed baseline from the prior mission:

- latest pushed platform commit: `26c7015172fffb229b934eaef0aab4f978f51f99`
- CI run: `25979999444`, successful
- deploy job: `76367109520`, successful
- staging health: proxy and sandbox both reported
  `26c7015172fffb229b934eaef0aab4f978f51f99`
- product proof: visible prompt-bar Playwright proof passed in 6.4 minutes
- run acceptance: `runacc-66e29607976f42ae1edc`, `export-level`, accepted
- worker VM: `vm-86c7339b3640f5c834d37e85633ad086`
- vsuper worker loop: `8d261912-b18d-442e-b4e0-9b55bd6f2c57`
- co-super child runs: `d433dbf8-8e72-443b-9910-75bea28d6b25` and
  `65cff4f7-8ea8-4aa1-8107-92dfe6f39087`, both completed
- queued candidate: `6c4d265c-b896-467a-8746-08a3f555e89f`
- continuation selected: `0164cf7b-100f-45c9-afcb-5552a90278d5`
- continuation compaction status: `skipped` because no compactable entries

Pipeline observations:

- `.github/workflows/ci.yml` already has `paths-ignore` for `docs/**` and
  top-level `*.md` on push and pull request. The mission must verify whether
  this fully satisfies "docs-only commits => no CI/CD" in the desired sense
  before changing it.
- Current CI runs `go vet ./...`, `go test ./... -count=1`, `go build ./cmd/...`,
  plus a frontend build. Recent Go jobs were about 3m40s.
- Deploy currently SSHes to Node B, resets `/opt/go-choir`, writes deploy
  identity, pre-builds the NixOS toplevel with `nix build`, then runs
  `nixos-rebuild switch --flake .#go-choir-b`, then builds guest images,
  restarts vmctl, and health-checks services.
- The deploy script may be doing redundant evaluation/build/switch work, but
  this must be measured on Node B before changing activation semantics.
- Local full-stack proof may be blocked by local service or sandbox-store state;
  staging remains the acceptance environment.

Highest-impact uncertainties:

- Does "docs-only" mean only `docs/**` plus root Markdown, or every Markdown and
  documentation-only path? Changing path filters without clarity can suppress CI
  for files that actually affect runtime.
- Which `internal/runtime` tests dominate CI time, and are they slow because of
  sleeps/timeouts, repeated heavyweight fixtures, serial state, live-ish
  integration behavior, or unavoidable coverage?
- Is Node B deploy latency dominated by Nix toplevel rebuild/switch, guest image
  build/copy, service restart/health waits, or remote cache/store behavior?
- Can duplicate co-super spawn/cast/export attempts be prevented at the tool
  idempotency layer, prompt contract layer, or run-loop termination layer without
  hiding real retries?
- What minimal real workload creates compactable run-memory evidence for a
  continuation-level proof without fabricating compaction?

## Real Artifact

The artifact is a faster and more reliable deployed Choir self-development loop:

```text
git commit
-> GitHub Actions CI with correct path filters and faster Go/runtime tests
-> Node B staging deploy with less redundant NixOS switch work
-> staging health identity
-> visible prompt-bar Choir-in-Choir workload
-> super -> worker VM vsuper -> implementation/verifier co-super channels
-> export/promotion candidate or precise blocker
-> continuation/run-memory evidence
-> Trace/VText/run acceptance proof
```

The CI/deploy pipeline and the worker/vsuper continuation substrate are one
control loop. The mission should optimize both without crossing authority
boundaries or accepting fake proof.

## Invariants

- Staging `https://draft.choir-ip.com` is the acceptance environment for
  platform behavior, worker VM, auth, gateway/model calls, Trace, VText,
  promotion, rollback, and run acceptance claims.
- Behavior-changing platform changes complete:

```text
commit -> push origin main -> monitor CI -> monitor staging deploy
-> verify staging commit identity -> run deployed acceptance proof
```

- Docs-only commits are different: CI intentionally ignores `docs/**` and
  top-level `*.md`. Do not weaken path filters to force docs-only CI. Do not
  over-broaden ignores so that runtime-affecting files skip CI.
- Browser/public acceptance uses visible product surfaces and public
  authenticated product APIs only: `/api/prompt-bar`,
  `/api/prompt-bar/submissions/*`, `/api/vtext/*`, `/api/trace/*`,
  `/api/promotions/*`, `/api/continuations/*`, and
  `/api/run-acceptances/*`.
- Browser-public proof must not bypass through `/api/agent/*`, `/api/prompts`,
  `/api/test/*`, `/internal/*`, direct service ports, raw event mutation
  endpoints, or manually seeded success records.
- Node B tracked files are not edited directly as a source/config shortcut.
- Faster CI must preserve meaningful coverage. Do not skip slow tests by
  renaming, tagging, or filtering them out unless an equivalent scheduled/manual
  coverage path is added and the tradeoff is explicitly recorded.
- Faster NixOS switch must preserve deploy identity, service health, vmctl guest
  image correctness, rollback semantics, and failure visibility.
- Foreground/canonical state remains stable. Background/candidate computers
  mutate. Canonical state changes only by promotion.
- `super` may do stateless or minor operational work, but Choir app/harness/repo
  candidate/promotion/platform mutations belong in worker VM `vsuper`.
- No fake-island placeholders, fake transclusion panels, fake candidate refs,
  fake verifier transcripts, or summaries that launder missing evidence into
  success.

## Value Criterion

Minimize:

```text
CI/CD waste + slow runtime test feedback + redundant Nix deploy work
+ duplicate worker/co-super churn + continuation evidence gaps
+ hidden state + verifier Goodharting
```

while preserving coverage, deploy identity, rollback, authority boundaries, and
product-path proof.

The mission moves uphill when:

- docs-only commits demonstrably do not run CI/CD, with a precise definition of
  "docs-only" that does not hide runtime-affecting changes;
- CI timing evidence shows faster Go/runtime feedback or a precise slow-test
  profile with actionable next fixes;
- Node B deploy logs show less redundant NixOS switch work or a named measured
  blocker;
- duplicate co-super spawn/cast/export attempts are reduced by idempotent
  runtime/tool behavior or prompt contracts, not normalized as healthy;
- continuation/run-memory evidence is real and durable, including compaction
  when the run actually has compactable history;
- every platform change is landed, deployed, and proved on staging.

The mission moves downhill when:

- docs-only behavior is "fixed" by disabling CI for files that affect code,
  workflows, Nix, package locks, generated assets, or deployment;
- slow tests are skipped without an alternate coverage plan;
- deploy speed is improved by bypassing NixOS activation, health checks, deploy
  identity, or rollback evidence;
- local-only proof is used for staging or worker VM claims;
- duplicate tool attempts disappear only from summaries while still happening in
  Trace;
- continuation-level is claimed when compaction/run-memory says `skipped`.

## Homotopy Axes

- CI filters: existing `paths-ignore` audit -> precise docs-only coverage ->
  optional path-filter workflow split if needed
- Go test speed: raw `go test ./... -count=1` -> package/test timing profile ->
  focused runtime fixture/sleep fixes -> safe parallelism/cache improvements
- Runtime tests: slow opaque tests -> named slow tests -> bounded deterministic
  tests with equivalent assertions
- Deploy speed: opaque Node B deploy duration -> phase timings -> remove
  redundant NixOS build/switch work -> preserve rollback/health semantics
- Worker termination: duplicate spawn/cast/export attempts -> idempotent child
  slots and casts -> single export attempt or explicit retry evidence -> clean
  vsuper terminal report
- Continuation: selected-only -> selected with run-memory refs -> compaction
  produced from compactable history -> continuation-level acceptance
- Acceptance: CI timing evidence -> staging identity -> product-path export
  proof -> continuation-level proof only when true

## Receding-Horizon Control

1. Measure before mutating. Capture recent CI timings, deploy job phase timings,
   current workflow filters, and slow Go/internal/runtime tests.
2. Patch the smallest pipeline layer that produces a measurable improvement
   without weakening coverage or deploy guarantees.
3. Commit, push, monitor CI/deploy, verify staging identity, and record before
   vs after timings.
4. If pipeline fixes expose new blockers, investigate and transform the search
   space before stopping.
5. After the pipeline loop is healthier, run the continuation/termination
   hardening workload through the visible staging prompt bar.
6. Patch duplicate co-super spawn/cast/export or continuation/run-memory gaps
   only from trace/event evidence, not speculation.
7. Land those fixes through the same improved CI/deploy loop.
8. Rerun product proof and synthesize run acceptance honestly.

## Dense Feedback

- GitHub Actions run list and job logs for before/after durations.
- Workflow path-filter inspection and, if needed, a docs-only commit or
  `workflow_run`/GitHub event evidence proving no CI/CD trigger.
- `go test` timing data, preferably package-level plus slow-test names for
  `internal/runtime`.
- Local focused Go tests for any runtime test changes.
- Deploy job logs with phase timing for git reset, NixOS toplevel build,
  switch/activation, guest image build/copy, vmctl restart, and health checks.
- `/health` staging build identity after each behavior-changing deploy.
- Visible staging prompt-bar Playwright proof.
- Trace snapshots showing worker/co-super topology and duplicate-attempt status.
- VText document/report with worker VM, child run, export, continuation, and
  rollback refs.
- RunAcceptanceRecord synthesized from existing evidence.
- Desktop/mobile screenshots and DOM metrics for Trace readability when UI is
  touched.

## Expected Work, Subject To Investigation

### GitHub Actions docs-only behavior

- Confirm whether `.github/workflows/ci.yml` path filters already satisfy:
  docs-only commits do not trigger CI/CD.
- If not, adjust filters precisely. Prefer explicit safe docs paths over broad
  ignores.
- Do not treat changes to `.github/**`, `nix/**`, `frontend/**`, `go.mod`,
  `go.sum`, generated assets, or lockfiles as docs-only.
- If proving docs-only requires a commit, use a docs-only proof commit only if
  it is valuable and clearly named; otherwise use GitHub event/run evidence.

### Go/internal/runtime test speed

- Profile current Go CI/test duration before edits.
- Identify slow `internal/runtime` tests by name and wall-clock duration.
- Look for removable sleeps, oversized timeouts, repeated heavyweight stores,
  serial tests that can safely use `t.Parallel`, live-ish test setup that should
  be fake-clocked, and duplicate coverage.
- Preserve failure evidence tests for worker/vsuper/delegate/run-acceptance.
- Avoid deleting or skipping tests unless replaced by equivalent assertions or
  moved to an explicit slow/manual lane with rationale.

### Node B NixOS rebuild switch speed

- Add or use phase timing in deploy logs before changing deployment.
- Investigate whether pre-building
  `.#nixosConfigurations.go-choir-b.config.system.build.toplevel` followed by
  `nixos-rebuild switch --flake .#go-choir-b` repeats avoidable work.
- Evaluate safe alternatives such as `nixos-rebuild switch --no-build` after
  prebuild, direct profile update plus `switch-to-configuration`, or keeping the
  current path if it is already store-cache dominated.
- Preserve deploy identity in `/var/lib/go-choir/deploy.env`, service health
  checks, guest image deployment, vmctl restart, and rollback semantics.
- Do not edit tracked files on Node B as the source of truth.

### Continuation and duplicate worker behavior

- Use the last proof trajectory and a fresh prompt-bar run to inspect duplicate
  spawn/cast/export patterns.
- Decide whether the fix belongs in:
  - child slot idempotency;
  - cast/update deduplication;
  - `export_patchset` idempotency;
  - worker/vsuper prompt contracts;
  - tool-loop termination behavior;
  - run acceptance synthesis.
- Require worker/vsuper to end with export/reviewable candidate or precise
  blocker, not sleep/poll churn.
- Design a workload that naturally creates compactable run memory before
  continuation selection. If compaction remains skipped, record why and the next
  minimal compactable probe instead of claiming continuation-level.

## Forbidden Shortcuts

- Do not broaden CI path ignores so runtime-affecting files skip CI.
- Do not speed up CI by removing coverage without an explicit replacement path.
- Do not bypass NixOS activation, staging health, deploy identity, or rollback
  semantics to make deploy logs shorter.
- Do not report a local Nix or Go timing as deployed proof.
- Do not use browser-public internal/test-only routes for acceptance.
- Do not use fake artifacts, fake co-super transcripts, fake compaction, or
  manual success seeding.
- Do not claim continuation-level when compaction is skipped or only selected.
- Do not spend the whole overnight loop on speculative UX polish while CI/deploy
  or continuation substrate remains opaque.

## Acceptance Targets

Pipeline target:

- docs-only behavior is verified or precisely fixed;
- Go/internal/runtime test speed is improved with before/after timing evidence,
  or a root-caused slow-test blocker is recorded with next patch;
- Node B deploy/switch latency is improved with before/after timing evidence,
  or a measured Nix/deploy blocker is recorded with next safe probe;
- pushed SHA is green in CI and deployed to staging;
- `/health` reports the deployed SHA on proxy and sandbox.

Product target:

- visible staging prompt-bar run creates VText-backed Choir self-development
  task;
- Trace shows super delegating to worker VM vsuper;
- worker/vsuper coordinates implementation and verifier co-super agents over
  real channels;
- duplicate spawn/cast/export behavior is reduced or precisely evidenced;
- worker produces export/reviewable candidate or precise blocker;
- continuation/run-memory evidence is durable and honestly represented;
- run acceptance reaches the correct level:
  - `export-level` for export evidence;
  - `promotion-level` only with verifier contract plus owner review and
    promotion/rollback evidence;
  - `continuation-level` only with run-memory/compaction and continuation
    evidence.

Clean blocker target:

- blocker includes timing evidence, CI/deploy/job refs if pipeline-related;
- blocker includes trajectory/run/worker ids, worker events, Trace/VText refs,
  and run-acceptance status if product-related;
- at least one root-cause probe and one changed search strategy were attempted;
- rollback refs and next executable probe are named.

## Rollback Policy

- Platform rollback: revert the pushed commit(s) and redeploy through normal
  CI/CD.
- Workflow rollback: revert `.github/workflows/ci.yml` changes if CI coverage,
  deploy, or docs-only behavior regresses.
- Test-speed rollback: revert runtime test changes if coverage becomes flaky or
  weaker.
- Deploy-speed rollback: revert deploy script changes if staging identity,
  rollback, guest images, vmctl, or service health becomes less reliable.
- Candidate rollback: discard/archive queued candidates that are not promoted.
- Evidence rollback: never delete bad evidence; append corrected evidence and
  mark superseded/blocked state honestly.

## Stopping Condition

Stop when one of these is true:

- `throughput + export-level`: CI/deploy throughput improvements are landed and
  measured, staging is healthy at the pushed SHA, and a visible prompt-bar
  product proof reaches accepted `export-level` with worker/co-super evidence
  and duplicate-attempt status recorded.
- `throughput + continuation-level`: the above plus durable compaction/run-memory
  and continuation evidence supports honest `continuation-level` acceptance.
- `hard blocker`: after root-cause probes and cognitive transforms, a pipeline,
  Nix, runtime, or continuation blocker remains with exact evidence, rollback
  refs, residual risks, and next executable probe.

Do not stop only because the first implementation route failed if another safe
probe inside the mission authority boundary remains.
