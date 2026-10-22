# FP10 · three small P2 items (RES-15, ARCH-15, ARCH-14)

Station of the launch first pass
([metamission](choir-launch-first-pass-metamission-2026-10-21.md)); items from
[the security-density checklist](../security-density-architecture-checklist-2026-10-21.md).
Lane branch `lane/fp10-p2-small`.

## now

| Item | Class | Status | What landed on the branch |
| --- | --- | --- | --- |
| RES-15 | green | done | comment in `internal/vmctl/host_admission.go`: admission bounds starts, not a running VM's later growth; backstops are pressure reclaim and leases |
| ARCH-15 | green | done | `launcher.desk` is `launcher.menu`, `DESK_APPS` is `LAUNCHER_APPS`, DeskSheet prop `launcherApps`; DOM `data-desk-*` hooks left. Review bunch 11 found the rename also wrote `launcher.menutopIcon` (no desktop icons); fixed in commit "DESKTOP_ICON_APPS filters on launcher.desktopIcon again" |
| ARCH-14 | orange | done, unwired | `internal/appverbs`: registry, per-caller allow, deny by default, errors as results, off unless `CHOIR_APP_VERBS`; no callers |

Rebased 2026-10-22 onto main 51cb3a8cd (clean, no conflicts); re-checked:
`go test -race ./internal/appverbs` passes, `go vet ./internal/appverbs ./internal/vmctl`
clean, no `DESK_APPS`/`launcher.desk` reference left in code (the task text in
the security-density checklist, ARCH-15, still names both, by design).

After review bunch 11 (blocking: `menutopIcon`), checks run in `frontend/`
(node 22.23.1, pnpm 11.9.0): `pnpm install --frozen-lockfile` and `pnpm build`
pass. `pnpm dlx svelte-check@4 --threshold error` reports 754 errors in 17
files, the identical set on origin/main's versions of the four touched files,
so this lane adds none; it also did not flag `menutopIcon` (no tsconfig, so it
does not type `.ts` modules this way). `tsc --noEmit` on `registry.ts` alone
fails with TS2339 on the broken revision and passes on the fixed one. The repo
has no frontend typecheck in CI; a `tsc`/svelte-check job with a tsconfig is a
residual (it would first meet the 754 existing errors).

Rollback: revert the four code commits; none touches state or a protected surface.
Heresy delta: discovered none, introduced none, repaired none.

## FP0 to land

1. Rebase on main; `go test ./internal/appverbs`; `go build ./internal/vmctl`.
2. Frontend: `pnpm install --frozen-lockfile && pnpm build` (run here, pass);
   optionally eyeball the desktop icons on staging after deploy.
3. ARCH-14 needs no deployed proof (unwired). Wiring it to a desk is a later
   station and would be red where it reaches Texture or vmctl.

## Open owner questions

None. Residual: ticking the three checklist rows is FP0's.
