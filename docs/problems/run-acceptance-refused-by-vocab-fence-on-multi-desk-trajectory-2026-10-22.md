# Run acceptance cannot be written for a trajectory that used two desks

Date: 2026-10-22. Status: **fixed in lane `lane/fp0-acceptance-vocab-path` (commit
`fe4c58050`), not yet proved on staging**. Found by FP0's phase 1
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
[s2a-management-tasks-refused-by-vocab-fence-2026-10-14.md](s2a-management-tasks-refused-by-vocab-fence-2026-10-14.md).

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

## Residual

A computer still before the cutover may hold acceptances whose path names
V1 roles (`texture > researcher`). Migration rewrites only exact V1 tokens,
so such a path is not migrated, and the serving fence would refuse it at
cutover, as it did before this fix (then as one token, now by part).
