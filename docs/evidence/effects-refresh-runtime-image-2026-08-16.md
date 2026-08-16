# Effects refresh runtime uses the current deploy image — 2026-08-16

**Boundary:** execute. Not live import. Super was not started. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Observation:** `docs/evidence/effects-refresh-stale-updater-current-2026-08-16.md`

## What landed

`RefreshVMWithConfig` sets `RefreshRuntime` and adds kernel param `choir.refresh_runtime=1`. Guest init unlinks `/mnt/persistent/choir-updater/current` when that param is present, then execs the image binary. Ordinary boot/restart/recover omit the param, so a promoted release still wins.

This does not wipe Dolt residue, desktop rows, or updater releases. It only drops the current pointer on refresh so the host-deployed autoputer runs.

## Tests

`go test ./internal/vmmanager -run 'TestBuildFirecrackerConfig_RefreshRuntime|TestRefreshConfigForCurrentDeploy'`

## Unchanged

Do not run live import until staging has this guest and another owner-scoped refresh. EmptyUntilSupported unchanged. Genesis 409. Armed=false. Super not started. Constructed freeze remains 7122f279.
