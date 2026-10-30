# P6 merge (ff01e0a2f) turns main's CI red: data race between recordStartupManifest and the async effective-release report (2026-10-15)

Date: 2026-10-15 13:26-13:36 UTC. Push `ff01e0a2f` (merge of
`feat/p6-p8-release-adopt` onto `964c598ca`, owner option (a)), CI run
38742617961. Status: documented; nothing fixed; main is red and does not deploy.
Staging stays at `964c598ca`.

## Observed

Only `Go Test (race, agentcore/textureowner shard 6)` failed (and with it the
`Go Vet + Test + Build` aggregation); `govulncheck` is report-only. Test:
`TestEnsureServingBaselineImportsImmutableBaselineWhenCurrentMissing`
(`internal/agentcore/selfdev_surface_boot_test.go:125/143`), `WARNING: DATA
RACE`:

- write: `(*Runtime).recordStartupManifest` at `rematerialize.go:635`
  (`rt.selfdevStartupReleaseDigest = manifest.ContentDigest`), called by the
  test's second (idempotent) `ensureServingBaseline`;
- previous read: `(*Runtime).EffectiveRelease` at `effective_release.go:120`
  (`out.ServingReleaseDigest = rt.selfdevStartupReleaseDigest`), from the
  goroutine `reportEffectiveReleaseAsync` (`effective_release.go:329-334`)
  that the FIRST `recordStartupManifest` started (`rematerialize.go:639`).

## Cause

P6's report goroutine reads `rt.selfdevStartupReleaseDigest` without
synchronisation while a later `recordStartupManifest` can write it again. In
production `recordStartupManifest` runs once per boot, but the test (and any
second call, e.g. a re-bootstrap) races; the race detector lane is not
selected on every CI run, which is why the P6 branch's own tests passed. The
local focused tests I ran (`-run 'Restore|EffectiveRelease|Release|Adopt'`)
do not use `-race` and did not include this test.

## Not done

The merge was not undone and no fix was added (stop rule: CI failed for a code
reason). Candidate repairs, to be chosen by the owner of P6: pass the digest
to `reportEffectiveReleaseAsync` by value instead of reading the field, or
guard the field with the runtime's existing mutex, or call
`reportEffectiveReleaseAsync` only on the first record.

Heresy delta: discovered one (an unsynchronised read of a runtime field in a
background goroutine); introduced one (by the P6 merge); repaired none.
