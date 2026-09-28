# Replay eligibility stale manifest — `texture_decisions` deleted from DDL, still declared present

Date: 2026-09-28
Status: **repaired** (commit `b85af274`; CI green; staging deploy verified).
Mutation class: **red** (replay-eligibility manifest gates checkpoint/route
projection and restore admission — a run-acceptance surface).

## The defect

`dc108d7e` ("consume-at-commit delivery") deleted the `texture_decisions`
SQL DDL, its scan helpers, and every writer — decisions moved to canonical
events/object graph — but left `"texture_decisions":
ReplayEmptyUntilSupported` in `replayAirworthinessEntries`
(`internal/agentcore/replay_eligibility.go`).

Replay eligibility is default-deny on declared-but-absent objects: every
fresh workspace and the replay probe's scratch projection lack
`texture_decisions`, so `replayEligibility` classified it as
`MissingTables` and refused every self-development checkpoint with
`"declared schema objects are missing from the observed workspace"`.

## Blast radius (CI failure cluster)

Nine tests failed on `dc108d7e`'s CI run (36463956126), all with the same
reason string:

- `TestPlatformUpdatePushAppliesAndRestores`
- `TestPlatformUpdateBootstrapsAbsentRoute`
- `TestPlatformUpdateResumesPendingTransition`
- `TestPlatformUpdateBootSweepResumesPending`
- `TestPlatformUpdateStrandedTailResumes`
- `TestCheckpointBindAcceptsEligibleRestoreSet`
- `TestReplayCompletenessReconstructsNonNilEventChain`
- `TestSelfDevReconcileMaterializesDerivably` (93.99s — refusal-retried)
- `TestSelfDevReconcileRecoversMaterializingOperation` (108.85s — same)

The slow SelfDev tests were the tell: the materialize leg was spinning on
checkpoint refusals. Once the manifest entry was reclassified, the same
tests ran ~5s.

## Root cause and repair

The table retirement was half-done: schema deleted, declaration kept.
The designed retirement path already existed (`ReplayRetiredAbsent`, used
by `app_adoptions`, `app_change_packages`, …) — this defect was the missed
manifest edit.

`b85af274` completes the retirement:

1. `texture_decisions` → `ReplayRetiredAbsent` in the manifest.
2. `DROP TABLE IF EXISTS texture_decisions` in `bootstrapTexture`, so
   computers that minted the table pre-retirement converge to the declared
   contract at next store open.

## Standing-question note

This is the schema-boundary version of the registry-hygiene failure the
standing questions warn about: a deletion that updated the code but not the
declaration the auditor reads. The manifest exists precisely to catch
silent drift — here it caught it correctly (ineligible was the right
verdict; the declaration was stale). Repairing the declaration, not
weakening the gate, is the conforming fix.

Refs: dc108d7e (defect), b85af274 (repair), CI runs 36463956126 (failure),
36476906223 (green). Mission:
docs/definitions/choir-sub-rlm-document-channel-2026-09-22.md (M2).
