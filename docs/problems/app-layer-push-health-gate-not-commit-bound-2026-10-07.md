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

## Rollback

None — record only.
