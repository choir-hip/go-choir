# Effects owner-scoped guest-boot refresh

**Boundary:** execute (route map 9 red prep / route map 10). Not live proof. Not a live send. Effects remain OFF.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Parent HEAD:** `0ee3a61e` (`code: wire guest self-development mode authority without enabling effects`)
**Mutation class:** red (owner-scoped lifecycle refresh product path)

## Live observation after host `0ee3a61e`

`https://choir.news/health` is `0ee3a61e` (`deployed_at` 2026-08-16T02:56:05Z, `built_at` 20260816023148). CI https://github.com/choir-hip/go-choir/actions/runs/31922108205 succeeded, including Node B deploy job `95105083015`.

The retained computer did **not** pick up guest mode authority:

| Call | Result |
|---|---|
| `POST .../self-development/genesis` | 409 `self-development effects are disabled` |
| `POST .../self-development/operations` (idempotency `effects-red-mode-off-2026-08-16T03:50Z`) | **503** `self-development mode authority unavailable` |
| `GET .../kernel-capabilities` | 503 `kernel capability authority unavailable` |
| `GET .../self-development/mode` | 200 `mode=off` generation 0 |
| `choir computer status` | active, epoch **268** |
| `GET /api/compute/status?computer_id=computer-03335285269bdba4f94377e56879f9e6` | 200 at 2026-08-16T03:53:44Z |
| `GET /api/acceptance/execution-identity` | 503 `execution identity authority unavailable` |

No mail was sent. Mode was not set. Restore was not invoked. Rematerialize was not invoked. Outbox `Armed` remains false. Owner gates were not deleted.

### Retained identity (compute status)

- ComputerID `computer-03335285269bdba4f94377e56879f9e6`
- kind `interactive`, state `active`, warmness `premium_always_on`, epoch **268**
- runtime reachable (`service=autoputer`, `runtime_health=ready`)
- `immutable_identity.joined=true`
- frozen `code_commit` **`7122f2799be4458f4b925be11990321c7e70ffc4`** (2026-07-16 `ops(vmctl): pin staging promotion authority`)
- `code_ref` `code:sha256:499bee7bf2a486941c5a717a8b25b4030bc869929f96a0ac625f08e9eac9f380`
- `artifact_program_ref` `artifact-program:sha256:9d90c8666a1d9a69f46daca644bb9470505831bb9926e21d2a577d0bd9aa5a6f`

That CodeRef/ArtifactProgramRef pair matches the constructed fleet row preserved by the `0ee3a61e` deploy (`candidate-fleet-e15cb89f25d963c220319b7b`). Global active-VM refresh correctly left constructed realizations in place (G4). The 503 is therefore a **constructed freeze**, not a missing host deploy.

`choir computer restart` remains stop+`Resolve`/`RecoverVM` on the existing boot artifacts. It would not pick up `0ee3a61e`. Compute `wake_current_computer` also does not refresh a reachable active computer.

The 503 string is post-July self-development guest code (`selfdevControl` unmounted). Frozen ComputerVersion identity is still `7122f279`. Those two facts can both be true: the route is constructed, and the running autoputer may be a later persistent updater/image that still lacks `WithSelfDevelopmentControl`.

## What landed

Owner-scoped guest-boot refresh is now a product verb, not an internal-only deploy loop:

- `POST /api/computers/{id}/lifecycle/refresh` calls `RefreshDesktopContext` → `POST /internal/vmctl/refresh` → `RefreshVMForDesktop`
- Force-reboots onto **current deploy boot artifacts**, preserves persistent data, advances realization epoch
- `choir computer refresh --computer … --idempotency-key …`
- `choir computer restart` is unchanged (stop+resolve)

This is **not** rematerialize-from-tape, **not** `choir computer create`, and **not** global deploy rewrite of constructed computers. G4 still excludes `snapshot_kind=constructed-computer-version` from deploy-time refresh. Owner-scoped refresh of a named computer is the authority named by this Definition's `now.next_action`.

Guest init still prefers `/mnt/persistent/choir-updater/current/bin/autoputer` over the image binary. A refresh that preserves persistent data may therefore still 503 if that updater current is the unmounted-control binary. Re-probe after the deployed actuation; do not treat this commit as live 409.

## Tests

- `TestParseComputerLifecyclePathAcceptsRefresh` — refresh parses as VM lifecycle; rematerialize does not
- `TestComputerRefreshUsesVmctlRefreshNotResolve` — refresh hits `/internal/vmctl/refresh` only; no stop/resolve/rematerialize; epoch 268→269
- `TestComputerRestartPreservesOrdinaryUserStopResolveSemantics` — restart still stop+resolve
- `TestLifecycleControlPersistsRefreshIntentBeforeIdempotentCompletion` — refresh requires epoch advance
- `TestComputerLifecycleCommandsUseTargetedProductAPI` — CLI posts `/lifecycle/refresh`

## What this is not

This is not live 409. This is not red promote+restore. Do not call refresh until **this** commit deploys to staging. Do not set mode. Do not send live mail. Do not rematerialize. Do not independently green restore. ComputerVersion route remains `7122f279` until a later self-development promote; refresh does not rewrite that join.

## Ceremony

- **Conjecture delta:** Host `0ee3a61e` is not guest mode-authority proof. Live 409 needs owner-scoped guest-boot refresh of the retained computer, then a start probe.
- **Protected surfaces:** genesis stays proxy-disabled; mode stays `off`; outbox `Armed=false`; G4 global-deploy non-interference with constructed computers; tape-recovery restore substrate unused here.
- **Admissible evidence:** compute-status identity above; the tests above. Not a live refresh actuation.
- **Rollback:** revert this commit. Refresh path disappears; restart remains stop+resolve; constructed freeze unchanged.
- **Heresy delta:** `named` the G4 tension (owner-scoped refresh of a constructed realization can desync running binary from frozen CodeRef). `preserved` global-deploy exclusion. `repaired` the missing owner-scoped product verb that `now.next_action` already named.

## Next

Wait for staging deploy of this commit. Then `choir computer refresh` on `computer-03335285269bdba4f94377e56879f9e6` (not rematerialize, not restart). Re-probe start: want 409 `current signed mode does not authorize proposal` without setting mode. If still 503, the persistent updater current is the remaining guest binary. Then red promote+restore. Do not send live mail.
