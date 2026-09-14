# CI Test Flake: Unbounded Provider-Channel Waits Race the Activation Deadline

Date: 2026-09-14 UTC
Status: repaired and verified (runs 34832567532, 34833924053; guard probe-tested)
Mutation class: yellow (test-only change + detector gate; no product behavior)
Classification: CI assurance substrate

## Problem

`TestActivationBudgetProgressDeadlineTerminalizesAndReleases` hung for the full
10-minute `go test` panic timeout in run `34728405572` (job
`Go Test (standard, agentcore/textureowner shard 7)` was not the failure; the
timeout was in `Go Test (race, agentcore/textureowner shard 2)` of run
`34641698611`, commit `0a80213b`).

The test arms `rt.cfg.ActivationBudget = 25ms`, starts a run, then does an
**unbounded** `<-provider.started`. The provider's `Execute` closes `started`
only if it runs. But `ExecuteActivationSync` registers a `context.AfterFunc`
progress deadline on the activation budget; if the dispatch goroutine is
scheduled late (loaded `-race` runner), the deadline terminalizes the run
*before* `executeActivation` reaches `persistActivationState`, which then sees
the stored terminal state and returns early — `Execute` never runs, `started`
never closes, and the test hangs until the 10-minute panic.

This is a test bug, not a product bug: the product contract (a run that
outlives its activation budget is cancelled and a late completion cannot
resurrect it) holds in both interleavings. The test over-specified the
interleaving by requiring `Execute` to start.

## Evidence

- Run `34641698611` (commit `0a80213b`, race-selected), job
  `Go Test (race, agentcore/textureowner shard 2)` id `103403100280`:
  `panic: test timed out after 10m0s`, goroutine 3135 blocked in
  `TestActivationBudgetProgressDeadlineTerminalizesAndReleases` at
  `lifecycle_authority_test.go:144` (`<-provider.started`) for 9m54s.
- `persistActivationState` (runtime.go:1518) returns `persisted=false` when the
  stored run is already terminal — the exact path that skips `Execute`.

## Audit of the same family

Bare channel receives on provider-controlled channels in `internal/agentcore`
tests:

- `lifecycle_authority_test.go:75` `<-provider.started` — safe today (no
  deadline armed), but a store hiccup in the dispatch goroutine's
  `GetLifecycleRun`/`GetRunByOwner` silently drops the activation and hangs.
- `lifecycle_authority_test.go:95` `<-provider.finished` — same exposure.
- `lifecycle_authority_test.go:144` `<-provider.started` — the observed hang.
- `lifecycle_authority_test.go:157` `<-provider.finished` — hangs whenever
  `started` never closed OR the deadline won before `Execute` ran.

Other bare receives (`<-start`, `<-startRace` in
`internal_run_idempotency_test.go`, `update_coagent_source_packet_test.go`,
`lifecycle_authority_test.go:332,340`) are test-controlled barriers closed by
the test itself — not this family.

## Related flake history (already repaired)

- `TestPersistentSuperReplacementContinuationAfterUnflaggedClaim` (run
  `33412710079`, 2026-08-31): 200-iteration poll exhausted under `-race`; test
  deleted in `bbf9edd6` substrate rewrite.
- `TestCreateRunRejectsActiveAdmissionToTerminalTrajectory` (run
  `30810894373`, 2026-08-03): 10m store stall from disk contention; repaired by
  `e22b99d4` moving test `TMPDIR` to `/dev/shm`.
- Runs `34728405572`/`34731934316` (2026-09-13) were deterministic stale-test
  failures after the provider-catalog deletion commit `f8db2ed1`, not flakes.

## Required repair invariants

- No test may perform an unbounded receive on a channel whose closure depends
  on `Execute` running. Every such wait must be bounded and must tolerate the
  run terminalizing before `Execute` starts.
- The deadline test must still prove both contracts: (a) a run that outlives
  the activation budget is cancelled with the progress-deadline error, and
  (b) when `Execute` did start, a late completion cannot resurrect it.
- `waitForTerminalRun`'s 2s poll budget gets headroom for loaded `-race`
  runners.
- A regression guard must fail CI if a new unbounded provider-channel receive
  appears in `internal/**` tests.

## Repair plan

1. Rewrite `TestActivationBudgetProgressDeadlineTerminalizesAndReleases` to
   bound the `started` wait and branch on whether `Execute` ran; assert the
   cancelled terminal state and error in both branches; bound the `finished`
   wait the same way.
2. Bound the `started`/`finished` waits in
   `TestCancelResidentRunReleasesImmediatelyAndRejectsLateCompletion` with a
   shared `waitProviderChan` helper.
3. Raise `waitForTerminalRun` deadline 2s -> 10s.
4. Add `scripts/check-test-hang-guards.sh` (run in the Vocabulary Gates job):
   fail on a bare `<-` receive of a provider/test-fixture channel in
   `internal/**` `*_test.go` outside a `select` with a timeout.

## Rollback

Revert the bounded commits via PR. The guard script is additive; removing it
restores discovery-only behavior.
