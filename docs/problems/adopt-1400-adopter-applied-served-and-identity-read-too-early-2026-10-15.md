# Adopt drill adopt-1400: adopter applied, but "serves the change" and "identity names adoption" read false (2026-10-15)

Date: 2026-10-15 14:00-14:16 UTC. Drill: `node scripts/reeng_tracer.mjs --label adopt-1400 --scenario adopt --expect-sha e485b1966… --timeout-min 30 --apply-min 10`
on staging `e485b1966` (P6-P8 merge ff01e0a2f plus the startup-manifest mutex
fix; CI green, deployed 13:59Z). Computers `computer-ec4fc89b…` (publisher) and
`computer-5b7a9b70…` (adopter), both stopped by the tracer. Evidence:
`~/.config/choir-qa/traces/adopt-1400/receipt.json`. Status: documented,
nothing patched.

## Outcome

Publisher engineering drill: reached `applied` (restore skipped by design;
`restore_to_pre_apply_head` false is the skip, not a failure of the adopt claim).
Adopt acceptance: `reached=adopter_applied`, failed `[adopter_serves_the_change,
identity_names_adoption]`; passed: publisher_applied,
release_published_and_reads_back, tampered_record_refused_unchanged,
unpublished_record_refused_unchanged, adoption_names_release_and_trajectory,
repeat_names_same_adoption, frozen_patch_is_the_release, adopter_applied,
planned_restart.

## Evidence on the two failures

- Adopter operation `selfdev-6b3ddc11…`: executing 14:15:11 -> frozen 14:15:22
  (`patch_matches_release: true`) -> accepted 14:15:42 -> materializing 14:15:45
  -> applied 14:15:54.88. The tracer read `adopter_served` at 14:15:55.03
  (status 200, `served: false`) and `adopter_identity` at 14:15:55.08, 0.15 s
  and 0.2 s after `applied`.
- Identity block at that moment: `release_digest == serving_release_digest`
  (`1ca2a8ff…`), `marker selfdev-fe1c9e49…`, `layered: false`, `build_commit
  e485b1966…`, event head sequence 625, and `tree_error: "self-development
  release whose applied operation is not recorded yet"` with no
  `adopted_release_id`. P6 serves a non-final tree entry and retries it after
  30 s (`effectiveTreeRetry`), so the adoption id is expected to appear later.

## Classification (hypothesis; not re-read after the 30 s retry)

The adopter's apply succeeded and the planned restart happened; both failed
checks were read within a fraction of a second of `applied`, before the
restart had finished serving the new release and before P6's 30 s retry could
record the adoption on the identity block. Most likely a tracer timing gap
(the checks should poll for a bounded window), not an adoption failure. Not
confirmed: the tracer stopped both computers right after the read, so the
settled state was not observed.

Owed: rerun the adopt drill with `adopter_served` and `adopter_identity` polled
(for example 90 s), or read them again before stopping the adopter.

Heresy delta: discovered 0; introduced 0; repaired 0.

## Plan (tracer only, no product change)

In `adoptScenario`, after the adopter's operation reaches `applied`, poll
`adopter_serves_the_change` (marker surface returns this run's token) and
`identity_names_adoption` (`effective_release.adopted_release_id == release_id`,
an `operation_id`, and the tree hash matching on the same base) every 5 s for
up to about 120 s, which covers the planned restart and P6's 30 s identity
retry (`effectiveTreeRetry`). On timeout the last observed served status and
identity block are recorded as they were (`claim.served`, `claim.identity`) plus
`claim.adopter_poll` with the attempt count and when each condition first held.
Then rerun the adopt drill once under a new label with the adopt-1400 command.
If both checks pass it is the first passing P6-P8 adopt loop; if they fail on
the settled state, write up the trace and stop (no patch).
