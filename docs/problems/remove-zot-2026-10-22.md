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
| `frontend/src/lib/SuperConsoleApp.svelte`, `frontend/src/lib/stores/terminal.js` (its only user) | deleted |
| `frontend/src/lib/apps/registry.ts` `super-console` entry | deleted (desk and desktop launcher entries go with it); the `terminal` surface kind had no other app and is deleted with its `AppSurface.svelte` rule |
| `frontend/src/app.css` `[data-super-console-preview]` and `.terminal-preview`, `FloatingWindow.svelte` `super-console` rule | deleted |
| `frontend/tests/*.spec.js` Console cases | `terminal-app.spec.js` deleted; `desktop-shell-core`, `prompt-surface-registry`, `trace-settings-registry` now assert the Console is absent (icon count 8 -> 7, desk minimum 15 -> 14); `responsive-layout` opens Settings and `desktop-state-persistence` uses `slides` (another heavy app) in its place |
| `internal/proxy/handlers.go` route and `HandleManagementConsoleWS`, `managementConsoleWSURLForTarget` | handler deleted; the path keeps an explicit 404 case so it never reaches the generic `/api/*` forward to guests still on an old image (deferred refresh); test `internal/proxy/super_console_removed_test.go` |
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
  recomputed `vendorHash`; likewise the frontend's `ghostty-web`
  dependency (the Console's terminal renderer) stays in `package.json`
  until a lockfile change with a recomputed `npmDepsHash`. Neither is
  imported any more, so neither ships in the built bundle or binary's
  code paths.
- Dated records (`docs/definitions/*`, `docs/design/*`, `docs/reports/*`,
  `docs/archive/desk-rlm-rectification-plan-2026-09-23.md`,
  `docs/model-pools-design-2026-10-12.md`, older `docs/problems/*`) still
  mention zot and `super-console` as they were; they are receipts.
- Old guests keep their `/api/super-console/ws` route until refreshed; the
  proxy's explicit 404 makes it unreachable from the browser in the
  meantime.

## Now card

```yaml
now:
  station: FP10 remove zot (lane/fp10-remove-zot)
  status: candidate pushed; waiting for FP0 review, landing and deployed proof
  base: 975f1c039
  commits:
    - ce29f97be docs: remove zot: decision and deletion record (green)
    - 61cf540e0 remove zot: the Console window, /api/super-console/ws on both sides, internal/zot, cmd/zot, zot-session (red)
    - ab61a8bca remove zot: citers in the flake, nix, SBOM scripts, comments and docs (red)
    - this now card (green)
  local_evidence:
    - go build ./... (GOMAXPROCS=2 -p=2): ok
    - go vet: proxy, autoputer, cmd/autoputer, gateway, server, apihandler: ok
    - go test: proxy (TestSuperConsoleRouteRemoved, failing before the
      change, plus the API-key authority matrices), autoputer (run_test,
      boot timeline), gateway (OpenAI, prompt cache key), server, apihandler: ok
    - .github/scripts build-sboms-differential-test, verify-sbom-candidate-test,
      sbom-semantic-fingerprint-test: all pass
    - frontend npm run build: ok
    - scripts/check-heresies.sh: exit 0
  not_run_locally:
    - Nix evaluation and the guest/Node B closures (no Nix on the lane VM): CI's
    - the edited Playwright specs (need a running stack): FP0 against staging
  mutation_class: red (proxy route, guest binary and image, Node B package set)
  rollback: git revert ab61a8bca 61cf540e0 (the record commits may stay)
  heresy_delta: {discovered: [], introduced: [], repaired: ["terminal/zot guest children (S1) have no production member"]}
  owner_questions: none
  residuals: see Residuals above (gateway OpenAI-compat route, go.mod and package.json tidy, old-guest route until refresh)
```

## Deployed proof (FP0, after landing)

1. **The route answers 404, anonymous and signed in.**
   `curl -sS -o /dev/null -w '%{http_code}\n' https://choir.news/api/super-console/ws`
   prints `404` (before: `401`). With WebSocket upgrade headers:
   `curl -sS -o /dev/null -w '%{http_code}\n' -H 'Connection: Upgrade' -H 'Upgrade: websocket' -H 'Sec-WebSocket-Version: 13' -H 'Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==' https://choir.news/api/super-console/ws`
   prints `404`. Signed in (owner browser console on choir.news):
   `new WebSocket('wss://choir.news/api/super-console/ws')` fires `error`
   then `close`, and `(await fetch('/api/super-console/ws')).status` is `404`.
2. **The Console window is gone.** Logged out, from `frontend/`:
   `PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/prompt-surface-registry.spec.js -g "carry neither Trace nor"`
   passes (no `super-console` in the desk sheet). By hand: the desk sheet
   and the desktop have no "Engineering Console" entry; the served bundle has
   no Console chunk:
   `curl -s https://choir.news/ | grep -o 'assets/index-[^"]*\.js' | head -1 | xargs -I{} curl -s https://choir.news/{} | grep -c 'super-console'`
   prints `0`.
3. **The guest image carries no zot binary.** In CI's or Node B's Nix (FP0):
   `nix eval .#packages.x86_64-linux.zot` fails with "does not provide
   attribute"; `nix path-info -r .#nixosConfigurations.go-choir-autoputer-vm.config.system.build.toplevel | grep -c -- '-zot-'`
   prints `0`; the same on Node B's system closure prints `0`. On a
   refreshed computer, the autoputer unit's environment carries no
   `CHOIR_ZOT_PATH` (`systemctl show autoputer -p Environment`, via the
   supported diag path, not SSH into a user computer).
4. **The paper is healthy** after the deploy (the metamission's standing
   acceptance): schedules and polls read as usual.
