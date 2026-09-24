# Mission Report: CI Flake Investigation and Guards

Date: 2026-09-14 UTC
Status: complete — verified on hosted runs
Mutation class: yellow (tests + detector gate; no product behavior)
Problem doc: docs/problems/ci-test-flake-unbounded-provider-waits-2026-09-14.md

## Mission goal and artifact

Investigate recent CI unreliability (random failures that pass on retry),
root-cause it, and create guarantees plus regression guards.

## Value criterion

Eliminate the observed flake mechanism and make its reintroduction a CI
failure, while preserving the product contracts the tests assert.

## Belief state (conjecture ledger)

| # | Claim | Test | Status |
|---|-------|------|--------|
| C1 | The observed "random" failures are mostly deterministic stale-test/build failures, not flakes | Failure-log audit of all ci.yml/race.yml failures in 30d | supported: 4 of 6 ci.yml failures were deterministic (stale catalog pins, build errors, vocab gate, classifier test) |
| C2 | The true flake family is unbounded test receives on channels only Execute closes | Goroutine dump: test blocked 9m54s at lifecycle_authority_test.go:144 | supported; mechanism verified against persistActivationState |
| C3 | A grep-level guard can prevent reintroduction | Synthetic violation fails the guard; clean tree passes | supported |
| C4 | Other bare `<-start`-style receives are the same hazard | Audit: they are test-controlled barriers closed by the test itself | falsified — not this family |

## Failure audit (30 days, all workflows)

| Run | Commit | Failed job(s) | Verdict |
|---|---|---|---|
| 34731934316 | 47e8ff81 | modelpolicy tests | deterministic: stale catalog pins after provider deletion (f8db2ed1) |
| 34728405572 | f8db2ed1 | agentcore + non-runtime shards | deterministic: same stale pins |
| 34699924720 | 190c7f7d | Vocabulary Gates | deterministic: writer-purity violation |
| 34664754132 | 9f255cd3 | scale + race shard 1 | deterministic: test build errors |
| 34652842420 | 93bb82c9 | Plan CI Lanes | deterministic: classifier contract test |
| 34641698611 | 0a80213b | race shard 2 | **flake: 10m hang, unbounded `<-provider.started`** |
| 33412710079 (race.yml) | 2ce20d61 | race shard 2 | flake: 200-iter poll exhausted; test deleted in bbf9edd6 |
| 30810894373 (race.yml) | 794b99c9 | non-runtime shard 0 | flake: 10m store stall; repaired by TMPDIR=/dev/shm (e22b99d4) |

## What shipped

- `8dfad03f` green(docs): problem doc (documentation-first).
- `6285a775` yellow(tests): bounded all provider-channel waits in
  `lifecycle_authority_test.go` via `waitProviderChan`; deadline test branches
  on whether Execute ran; `waitForTerminalRun` 2s→10s; bounded proxy `done`
  drains; `hang-guard:` markers on intentional fixture release waits; new
  `scripts/check-test-hang-guards.sh` wired into the Vocabulary Gates job.
- `d541d39f` yellow(tests): widened the guard from line-initial to any
  non-select-case receive after a synthetic-violation probe exposed the gap.

## Evidence

- Reproduced mechanism by code path: `persistActivationState` returns
  `persisted=false` on stored-terminal → `Execute` skipped → `started` never
  closes → unbounded receive hangs to the 10m panic.
- Fixed tests pass `-count=3` standard and `-race -count=20` locally.
- CI run 34832567532 (6285a775): all race shards green; the fixed test passed
  under `-race` in CI.
- CI run 34833924053 (d541d39f): green.
- Guard verified adversarially: synthetic `<-provider.started` fails it.

## Residual risks

- The guard covers the `started|finished|release|done|completed` fixture-channel
  family; a differently-named unbounded channel would evade it. The 10m
  go-test panic remains the backstop.
- Production note (out of scope, recorded): `activate` logs but does not retry
  a failed `initial_dispatch`; non-Super runs have no lost-dispatch watchdog.
  If the actor runtime can drop dispatches, ordinary runs could stall pending.
  Worth a separate mission if dispatch loss is ever observed in production.

## Rollback refs

- Revert 6285a775 and d541d39f via PR; guard script is additive.
