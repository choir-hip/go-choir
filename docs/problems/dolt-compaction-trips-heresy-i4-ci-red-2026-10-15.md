# Landing fix/bound-guest-dolt-history turns main's CI red: heresy detector I4 (2026-10-15)

Date: 2026-10-15 12:28-12:38 UTC. Push `c92746ed0` (merge of
`fix/bound-guest-dolt-history`), CI run 38734858854. Status: documented,
nothing fixed; main's deploy is blocked until a decision.

## Observed

- `Heresy Detector` failed (`scripts/check-heresies.sh --fail-on-regression`):
  family **I4 destructive embedded rollback guard**, 3 hits for
  `CALL DOLT_RESET`; enforced at zero. `Go Vet + Test + Build` failed only on
  its final aggregation step ("Confirm selected CI gates passed"); no Go test
  failed. `govulncheck (report only)` failed as it has on prior runs.
- The hits: `internal/store/dolt_compaction.go` lines 859, 864, 876
  (`CALL DOLT_RESET('--soft', ...)` to squash the guest store's history, and to
  restore the old head on failure). Local focused tests pass
  (`go test ./internal/store -run 'Compaction|Maintenance'`).
- `docs/heresy-detectors.md` I4 allows one typed exception,
  `internal/projectionbase/compact.go` (history-only soft reset to the root
  commit, after a backup, refusing unless the content witness is identical
  before and after). `dolt_compaction.go` does the same kind of reset for the
  guest store, paper computer only, with a witness check, but it is not in the
  detector's exclude list.
- The merge was verified locally only for build and store tests; the detector
  was not run before the push (my mistake: the lander ran focused Go tests, not
  `scripts/check-heresies.sh`).

## Decision needed (not taken)

Either add `internal/store/dolt_compaction.go` as a second typed exception to
I4 (docs/heresy-detectors.md and the detector exclude list; it is a doctrine
manifest change, "yellow"), or revert the merge. Everything after
`13380efc7` (account recovery, deployed and verified) is therefore not
deployed: the Dolt compaction code is on main but CI is red, so no deploy
runs until this is resolved.

Heresy delta: discovered 0 new; introduced: a second reset site needing a typed
exception (I4); repaired: none.
