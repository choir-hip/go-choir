# FP10 · three small P2 items (RES-15, ARCH-15, ARCH-14)

Station of the launch first pass
([metamission](choir-launch-first-pass-metamission-2026-10-21.md)); items from
[the security-density checklist](../security-density-architecture-checklist-2026-10-21.md).
Lane branch `lane/fp10-p2-small`.

## now

| Item | Class | Status | What landed on the branch |
| --- | --- | --- | --- |
| RES-15 | green | done | comment in `internal/vmctl/host_admission.go`: admission bounds starts, not a running VM's later growth; backstops are pressure reclaim and leases |
| ARCH-15 | green | done | `launcher.desk` is `launcher.menu`, `DESK_APPS` is `LAUNCHER_APPS`, DeskSheet prop `launcherApps`; DOM `data-desk-*` hooks left; frontend typecheck not run (no node_modules in this worktree) |
| ARCH-14 | orange | done, unwired | `internal/appverbs`: registry, per-caller allow, deny by default, errors as results, off unless `CHOIR_APP_VERBS`; no callers |

Rebased 2026-10-22 onto main 51cb3a8cd (clean, no conflicts); re-checked:
`go test -race ./internal/appverbs` passes, `go vet ./internal/appverbs ./internal/vmctl`
clean, no `DESK_APPS`/`launcher.desk` reference left in the repo.

Rollback: revert the three commits; none touches state or a protected surface.
Heresy delta: discovered none, introduced none, repaired none.

## FP0 to land

1. Rebase on main; `go test ./internal/appverbs`; `go build ./internal/vmctl`.
2. Frontend: `npm run check` and the build in `frontend/` (not run here).
3. ARCH-14 needs no deployed proof (unwired). Wiring it to a desk is a later
   station and would be red where it reaches Texture or vmctl.

## Open owner questions

None. Residual: ticking the three checklist rows is FP0's.
