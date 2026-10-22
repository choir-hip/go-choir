# Remove zot (decision and deletion record, 2026-10-22)

Station: FP10, lane `lane/fp10-remove-zot`, first-pass metamission
(`docs/definitions/choir-launch-first-pass-metamission-2026-10-21.md`).

## Decision

Owner, 2026-10-22 02:24 UTC (ledger entry 02:24): "For the zot question, we
should actually just remove zot." This settles N4 of
`docs/design/vsock-control-plane-and-nic-removal-2026-10-22.md` (inventory
A9 in `docs/reports/guest-network-inventory-2026-10-21.md`): the Console's
zot needs no egress row because it no longer exists.

## What zot is today

zot is the Management Console's out-of-process repair session:

- the browser window `frontend/src/lib/SuperConsoleApp.svelte`, registered as
  app `super-console` ("Engineering Console", desk and desktop launcher);
- the proxy route `GET /api/super-console/ws`
  (`internal/proxy/handlers.go` `HandleManagementConsoleWS`), which relays
  WebSocket frames to the guest;
- the guest route of the same path in the autoputer
  (`internal/autoputer/terminal.go`), a singleton PTY that spawns
  `CHOIR_ZOT_PATH` (the third-party `patriceckhart/zot` 0.2.6 TUI agent built
  by the flake, pointed at the gateway's OpenAI-compatible route) or, without
  it, the autoputer's own `zot-session` mode;
- the `zot-session` mode (`cmd/autoputer/main.go`,
  `internal/autoputer/run.go` `RunZotSession`) and its implementation
  `internal/zot` (a line REPL that writes `.choir/zot/sessions/*` logs and
  diagnosis reports), plus the standalone `cmd/zot`.

Production consumers (standing question 4): only the Console window. The
plain-shell `NewTerminalHandler` in `terminal.go` has no production caller
(the raw Terminal app was retired, heresy H028); it exists only for tests.

## What the Console loses

Everything. The Console window is the zot session and nothing else: there is
no shell, no log view and no other tab. After the deletion the
`super-console` app, its desk and desktop launcher entries and its route are
gone. Repair inside a computer goes through the desks (engineering capsules)
and the owner's supervised paths, not an in-guest TUI agent with the
autoputer's uid. A desktop that had the Console window open restores it as
a closable "Unknown app: super-console" placeholder (`AppHost.svelte`'s
existing unknown-app branch); no crash.

Security side effect: the in-guest child process class "terminal and zot
children" (s0/s1 problem records, auth-boundary-gaps F2 residual) has no
production member left; the guest image and Node B no longer ship the
third-party zot binary.

## Citers (`git grep -n -i zot` outside `docs/archive`) and their fate

Code and build:

| Citer | Fate |
| --- | --- |
| `frontend/src/lib/SuperConsoleApp.svelte` | deleted |
| `frontend/src/lib/apps/registry.ts` `super-console` entry | deleted (launcher entry goes with it); the `terminal` surface kind stays only if another app uses it |
| `frontend/src/app.css` `[data-super-console-preview]`, `FloatingWindow.svelte` `super-console` rule | deleted |
| `frontend/tests/*.spec.js` Console cases (`desktop-shell-core`, `prompt-surface-registry`, `responsive-layout`, `trace-settings-registry`, `desktop-state-persistence`, `terminal-app.spec.js`) | Console cases deleted or the app id swapped for another heavy app; `terminal-app.spec.js` deleted |
| `internal/proxy/handlers.go` route and `HandleManagementConsoleWS`, `managementConsoleWSURLForTarget` | handler deleted; the path keeps an explicit 404 case so it never reaches the generic `/api/*` forward to guests still on an old image (deferred refresh) |
| `internal/proxy/api_key_computer_authority_test.go` super-console cases | deleted |
| `internal/autoputer/terminal.go`, `terminal_test.go` | deleted (no production caller remains) |
| `internal/autoputer/run.go` `RunZotSession` and Console registration; `run_test.go` | deleted |
| `cmd/autoputer/main.go` `zot-session` mode | deleted |
| `internal/zot`, `cmd/zot` | deleted |
| `flake.nix` `zotPkg` and `packages.zot` | deleted |
| `nix/autoputer-vm.nix` `goChoirPackages.zot`, `CHOIR_ZOT_PATH` | deleted |
| `nix/node-b.nix` `goChoirPackages.zot` | deleted |
| `.github/scripts/build-sboms-differential(-test)`, `verify-sbom-candidate(-test)`, `sbom-semantic-fingerprint-test` | `zot` dropped from the package lists and the external-package rule; the fingerprint test's external example becomes `obscura` |
| `.gitignore` `/zot` | deleted |
| `internal/gateway/openai_compat.go` comments, `gateway_test.go`, `prompt_cache_key_test.go` strings | comments and fixtures de-named; the route stays (see residuals) |
| `internal/apihandler/in_guest_caller_test.go`, `internal/autoputer/boot_timeline_diag_http_test.go`, `internal/server/guest_owner.go`, `internal/server/host_caller.go` comments | "terminal, zot" examples become "capsule workloads"/in-guest processes |
| `internal/appdesk/appdesk.go` reserved name `super-console` | kept: retired app ids stay reserved (like `terminal`) |
| `go.mod` `github.com/creack/pty/v2` | left: it becomes unused, but editing `go.mod`/`go.sum` changes the flake's `vendorHash`, which this lane cannot recompute without a Nix build; a tidy is a follow-up |

Docs (current-state docs are fixed; dated records stay as receipts):

| Citer | Fate |
| --- | --- |
| `docs/choir-doctrine.md` (two factual mentions) | factual fix only: Console/zot is removed; doctrine otherwise untouched |
| `docs/current-architecture.md` | fixed |
| `docs/learn/03-gateway-and-providers.md` | fixed |
| `docs/platform-os-app-state.md` Super Console row | fixed (removed 2026-10-22) |
| `docs/heresy-detectors.md` H028 | fixed (the replacement surface is gone too) |
| `cmd/desktop/README.md` `/api/super-console/ws` | fixed |
| `docs/problems/auth-boundary-gaps-from-surface-audit-2026-10-14.md`, `s1-non-root-runtime-child-uid-boundary-2026-10-04.md` (open residuals naming zot children) | dated note appended |
| `docs/definitions/*`, `docs/design/*`, `docs/reports/*`, `docs/reviews/*`, `docs/evidence/*`, `docs/model-pools-design-2026-10-12.md`, other `docs/problems/*` | dated records: left as written (they record what was true then) |
| `docs/deck/shot-worldwire.png` | binary false positive |

## Residuals

- The gateway's OpenAI-compatible routes (`/provider/openai/v1/*`) lose
  their only in-repo client. Deleting them is a gateway (red) change
  outside this station; candidate for a later deletion slice.
- `github.com/creack/pty/v2` stays in `go.mod` until a tidy with a
  recomputed `vendorHash`.
- Old guests keep their `/api/super-console/ws` route until refreshed; the
  proxy's explicit 404 makes it unreachable from the browser in the
  meantime.

## Now card

(filled in at the end of the lane)
