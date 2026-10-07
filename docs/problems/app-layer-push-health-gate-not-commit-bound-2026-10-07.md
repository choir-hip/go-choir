# App-layer release push health gate is not commit-bound — a rolled-back apply reports "healthy"

**Found:** 2026-10-07, during deployed acceptance for `ca8c8c18`
(delegated-report delivered-page fix). Owner computer
`candidate-fleet-e15cb89f25d963c220319b7b` (canary, user
`5bd6de97-…`) had its apply **fail and roll back**, yet the deploy
reported `app-layer push: 2/2 healthy`.

## Symptom

CI deploy for `ca8c8c18` (run 37551113838) pushed the app-layer release
to the two eligible computers (owner canary + `vm-48bc0981` tracking)
and logged `2/2 healthy, time-to-healthy=96s`. Post-deploy probe:

- `vm-48bc0981` → `build.commit = ca8c8c18` (applied)
- `candidate-fleet-e15cb89f` (owner) → `build.commit = 6d55a585`,
  `deployed_at = 00:27:42Z` — restarted but on the **prior** release.

Owner console:

```
platform update resume: re-driving update
  app-layer-ca8c8c180173-1791332519-candidate-fl after guest restart
platform update resume: platform update: apply failed and prior release
  was restored: updater refused apply: materialization failed and prior
  release was restored: updater: health probe failed: health status=503
```

## Root cause

The deploy loop's healthy gate (ci.yml `app-layer push` phase) polls
`autoputer-proxy/health` for HTTP 200. A **rolled-back** computer's
prior-release autoputer answers 200 — the gate counts it healthy.
`app_layer_healthy` is incremented on status alone, never on
`build.commit == DEPLOY_COMMIT`.

## Fix shape (for the SA station)

- The healthy gate must poll `/api/runtime/observability` (or an
  equivalent identity surface) and count a computer healthy only when
  `build.commit == DEPLOY_COMMIT` — matching the precedent the same
  workflow already sets with `deploy_active_vm_refresh` identity checks
  ("post-refresh identities; anything else stays fail-closed").
- Rollback is a legitimate outcome; the gate should report
  `applied`/`rolled-back`/`failed` distinctly, and a rolled-back push
  on the only canary should probably block or escalate (today it is
  silently green).
- Orthogonal: the 503 health-probe failure that caused this rollback
  (release autoputer not ready within the update fence) is a second
  defect — the apply fence is tighter than the boot time of a release
  whose store carries more replay work.

**Mutation class of this record:** green. The fix is orange (CI gate)
plus the underlying fence question is red (platform update apply).

## Update 2026-10-07 — deterministic apply failure on the owner computer

A manual re-push of the same staged release (fresh offer, minted against
the live canonical head + epoch) reproduced the failure bit-for-bit:

```
platform update resume: re-driving update
  app-layer-ca8c8c18-retry-1791333327-1 after guest restart
platform update resume: platform update: apply failed and prior release
  was restored: updater refused apply: materialization failed and prior
  release was restored: updater: health probe failed: health status=503
```

Correction on timing (2026-10-07): the resume phase's own `dur=` is
near-zero, but the probe then loops 30×~1s — the apply actually burned
~35s of 503s (00:36:05 boot → 00:36:42 refusal) while the owner's
546k-event replay still gated /health. The fence defect is real but the
"~1s" read was the resume-phase duration, not the probe window.
`vm-48bc0981` (much smaller state) applied the same release cleanly. So
the apply health fence is shorter than the boot-to-healthy time of a
large-history computer, and the deploy gate then mislabels the rollback
healthy. Two distinct defects, one green deploy receipt.

**Status (2026-10-07):**
- Gate defect: **fixed `68397ae7`** — the healthy poll now counts
  `build.commit == DEPLOY_COMMIT` via `/api/runtime/observability` and
  fails the deploy when any pushed computer did not land the commit.
  Live on the next app-layer deploy.
- Apply-fence defect: **fixed `19d7913e`** — `HTTPHealthProber` treats a
  503 `{"status":"replaying"}` body whose `committed_sequence`/`progress`
  advances since the last poll as liveness (resets the stall budget);
  stagnant replays still exhaust it; `MaxDuration=15m` is the absolute
  bound. `choir-updater` ships in the guest base image, so the fix landed
  via the `19d7913e` guest-image deploy + active-VM refresh; the owner
  computer now runs `19d7913e`. Regression tests fail-before/pass-after
  (`TestHTTPHealthProber{ToleratesAdvancingReplay,StallFails,
  MaxDurationCaps}`). Deployed acceptance pending the next real app-layer
  apply on the owner (the `19d7913e` commit itself is base-image-only, so
  its layered-release delta was empty).

## Rollback

None — record only.
