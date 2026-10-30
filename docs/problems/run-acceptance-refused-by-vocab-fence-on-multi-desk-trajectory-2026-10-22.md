# Run acceptance cannot be written for a trajectory that used two desks

Date: 2026-10-22. Status: **fixed in lane `lane/fp0-acceptance-vocab-path` (commit
`fe4c58050`, on main as `1ddcfbd00`, deploy H `43e318712`), proved on staging
2026-10-22 07:47 UTC (see "Staging proof" under Proof plan)**. Found by FP0's phase 1
staging gate of run-telemetry batching (lane `lane/fp0-batching-e2e`), while
running design section 9 step 6. Not caused by batching: the refusal is a
vocabulary check on a metadata string, and batching does not change it.
Evidence class: staging (deployed build `d991907e6`, test computer
`computer-71afbf1461dffabcce8eeebe558d03bd`), product path.

## Evidence

`POST /api/run-acceptances/synthesize` with `target_mission_id` and
`trajectory_id`, owner-authenticated through the proxy:

- Trajectory `87837770-…` (Texture document, first draft only, one desk):
  202, record `runacc-ab663015caa2cba0087b`, state `blocked`, docs-level.
- The same trajectory after Texture opened research, three calls between
  05:21 and 05:23 UTC, and trajectory `a3015be4-…` (the kill-drill document,
  restart-interrupted): 400 every time, body

      upsert run acceptance: store: create object: vocab write guard:
      vocabfence: og:choir.run_acceptance:metadata.authority_profile="texture > research"
      not in v2 vocabulary

Every Texture trajectory on this computer that ran long enough opened a
research run (the first draft of a seeded document already does), so no
restart-interrupted trajectory could be synthesized.

## Cause

`acceptanceAuthorityProfile` (`internal/agentcore/run_acceptance.go:216`)
joins the distinct roles of the trajectory's runs with `" > "`. The object
graph write guard (`internal/store/vocab_migrate_og.go`, `ogRoleLeafKeys`
includes `authority_profile`) treats that field as one vocabulary token and
checks it against the V2 set (`internal/vocabmigrate/fence.go`, `v2LiveSet`).
A single role passes; any joined path fails. Same class as
[s2a-management-tasks-refused-by-vocab-fence-2026-10-14.md](../archive/problems/s2a-management-tasks-refused-by-vocab-fence-2026-10-14.md).

## Effect

Run acceptance synthesis fails for every multi-desk trajectory on a computer
past the vocabulary cutover, which is most real work (Texture with research).
For the batching gate it means step 6 ("acceptance on a trajectory with
pending telemetry returns the same verdict as flag off", and "on a
restart-interrupted trajectory") could only be run on a one-desk trajectory.

## Fix options

- Do not classify `authority_profile` as a role leaf, or split it on `" > "`
  and check each part.
- Or store the path as a list field the guard checks element by element.

## Fix (applied)

Split in the guard (tests `67c3e9b3b`, fix `fe4c58050`). The stored shape
stays a string: no frontend, script, or Go reader parses a run acceptance's
`authority_profile`, and a list would change the record's JSON shape for
existing rows and API callers. Dropping the field from the role leaves was
rejected: it would let a retired role through.

- `vocabmigrate.AuthorityPathSeparator` (`" > "`) is the one separator;
  `acceptanceAuthorityProfile` joins with it.
- `vocabmigrate.AuthorityPathFields` gives the fence one field per part.
  A path with an empty part is given whole and refused, because the fence
  skips empty values.
- The split applies only to `choir.run_acceptance` `authority_profile`; on
  work items and other kinds the field stays one role.

Tests (cutover store): `texture > research` and a three-desk path write; a
single role writes; `texture > researcher`, `super > research`, `" >  > "`,
`texture > `, `texture>research`, and a path on `choir.work_item` are
refused (`internal/store/vocab_write_guard_role_path_test.go`); a two-desk
acceptance built by `acceptanceAuthorityProfile` upserts and reads back
after `MigrateVocabularyToV2`
(`internal/agentcore/run_acceptance_vocab_path_test.go`).

## Proof plan (FP0, staging)

After the fix lands on `main` and deploys, on a test computer past the
cutover (not the paper):

1. Check the computer's build identity reports the fix commit or later.
2. Open a Texture document whose first draft opens research; wait for the
   research run to appear in the trajectory.
3. `POST /api/run-acceptances/synthesize` with `target_mission_id` and the
   trajectory id, owner-authenticated through the proxy. Expect 202 and a
   record whose `authority_profile` is `texture > research`; record the id.
4. Repeat on a restart-interrupted Texture trajectory that opened research
   (the batching gate's step 6 case).

### Staging proof (FP0, 2026-10-22 07:47 UTC)

Run by the self-development monitor for FP0 on the test computer
`computer-71afbf1461dffabcce8eeebe558d03bd` (vm-c5c801f8…, warmness
`test`). The calls went owner-authenticated through the proxy
(`https://choir.news`) with the test computer's key.

- Build identity:
  - proxy `x-choir-build-commit: 43e318712553e2a69d5da87d37d41963223d0424`;
  - guest /health at 07:47:19: status ready, running_runs 0, build and
    deployed commit `43e318712…`, deployed_at 07:34:14Z.
  - vmctl's `deploy refresh caller=deploy` round for 43e318712 ran at
    07:31:36. It deferred this computer, which was busy with the
    freeze-close drill. The retry sweep refreshed it at 07:34:17
    (`outcome=refreshed planned_restart=true`).
- Plan step 2 was not re-run. Both trajectories are the ones that
  answered 400 under `d991907e6` (Evidence above), and each already had
  a research run.
- Before: the computer held one acceptance, `runacc-ab663015caa2cba0087b`
  (trajectory 87837770), with `authority_profile` `texture`, state
  `blocked`, docs-level, and health commit `d991907e6`.

| call (body: `target_mission_id` `fp0-batching-phase1-staging-2026-10-22`) | time (UTC) | code | record | `authority_profile` | state, level |
|---|---|---|---|---|---|
| `POST /api/run-acceptances/synthesize`, `trajectory_id` `87837770-4f79-5414-9c4b-2d5352658a1f` | 07:47:35 | **202** | `runacc-ab663015caa2cba0087b` (the same record, upserted) | **`texture > research`** | blocked, staging-smoke-level |
| `POST /api/run-acceptances/synthesize`, `trajectory_id` `a3015be4-976a-55d5-b46b-1130f2ef5f13` (restart-interrupted) | 07:47:46 | **202** | `runacc-c6d55bcaf418abe61210` (new) | **`texture > research`** | blocked, staging-smoke-level |

- Neither call returned a vocabfence error.
- Both records name health and deployment commit `43e318712…`.
- In both records:
  - checkpoints `submitted` and `texture_opened` passed;
  - the invariants `trajectory_path_observed`, `capsule_effect_bounded`,
    `core_acceptance_not_claimed` and `checkpoint_causal_order` passed.
- `blocked` is the record's own verdict, not a refusal: "RunAcceptance is
  trajectory diagnostics only; self-development acceptance requires
  canonical event, checkpoi…".
- `GET /api/run-acceptances/<id>` at 07:47 read both records back
  (200, `texture > research`).

Result: plan steps 3 and 4 pass on staging. The multi-desk path writes
through the cutover write guard, and it reads back.

## Residual

A computer still before the cutover may hold acceptances whose path names
V1 roles (`texture > researcher`). Migration rewrites only exact V1 tokens,
so such a path is not migrated, and the serving fence would refuse it at
cutover, as it did before this fix (then as one token, now by part).
