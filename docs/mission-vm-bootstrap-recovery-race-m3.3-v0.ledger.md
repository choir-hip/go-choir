# Mission M3.3 - VM Bootstrap Recovery Race Ledger

## 2026-06-15T10:34:48Z - Create Pre-M3 Bootstrap Recovery Paramission

Claim/scope: the owner-visible `BOOTSTRAP FAILED (502)` / reload-success
behavior is now its own pre-M3 substrate bug. It should not be hidden inside M3
proper, because M3 needs vmctl refresh/restart as trustworthy lifecycle proof.

Move: created `docs/mission-vm-bootstrap-recovery-race-m3.3-v0.md` as a narrow
Parallax paramission. The paradoc records the current evidence: owner bootstrap
requests canceled around `2026-06-15T10:17Z`, vmctl refreshed the owner VM active
seconds later, the owner VM later reported `ready` with low persistent-disk use,
and a separate stale Universal Wire/sourcecycled route at `10.200.17.2:8085`
continues to produce 502 health noise.

Expected Delta V: 0 for repair, -1 for mission ambiguity. Actual Delta V: -1
against mission ambiguity. The implementation pass now has a focused source
program instead of resuming M3 while the boot/recovery substrate is noisy.

Receipts:

- Owner VM current health after investigation: `status=ready`,
  `sandbox_id=vm-5b0c1bef1e2b6d7f8dad7d0e8473ed19`, deployed commit
  `0a5fb602151c8373086c4a2774e1236faa53831b`, persistent disk about `8.9%`
  used.
- Public health after investigation: `bootstrap.total` 52 requests, 40
  `http_200`, 3 `http_502`, 9 `resolve_error`, max duration about 15 seconds.
- Stale platform/source route found:
  `vm-universal-wire-platform`, state `booting`, route
  `http://10.200.17.2:8085`; `sourcecycled` retries against it about every 32
  seconds with 502.

Open edge: implement the durable recovery/status repair and deployed first-load
proof before M3 lifecycle settlement.

## 2026-06-15T10:44:15Z - Problem-First Checkpoint For Canceling Recovery Request

Claim/scope: current source inspection supports the narrower v0 failure shape:
the recovery endpoint starts from authenticated owner intent, but lookup,
resolve, refresh, and the HTTP response remain tied to the browser request
context. When the foreground recovery request is canceled or outlives a browser
deadline, the proxy can cancel vmctl work and has no redacted durable operation
state for `/api/compute/status` or a later bootstrap probe to observe.

Move: read the current incident record and the named code paths:
`docs/incident-vm-bootstrap-stale-route-2026-06-09.md`,
`frontend/src/lib/Desktop.svelte`, `internal/proxy/compute_status.go`,
`internal/proxy/handlers.go`, `internal/vmctl/client.go`, and
`internal/vmctl/ownership.go`. Rewrote Parallax State to name the current
problem before implementation.

Expected Delta V: -2 by resolving the reproduction shape and fix locus. Actual
Delta V: -2. The deterministic regression still needs to be written, but it now
has a precise predicate: cancel the browser recovery request, let vmctl refresh
complete anyway, then prove status/bootstrap observe ready without manual reload.

Receipts:

- `HandleComputeRecovery` calls `LookupDesktopContext`,
  `ResolveDesktopContext`, and `RefreshDesktopContext` with `r.Context()`.
- `computeRecoveryResponse` carries only a synchronous result and no operation
  identity or pending/refreshing status.
- `HandleComputeStatus` reports lookup/runtime state, but no in-flight recovery
  state.
- `Desktop.svelte` fires `requestBootstrapRecovery(...)` asynchronously while
  bootstrap polling continues; it does not have a durable recovery handle to
  poll if the request fails or is canceled.

Open edge: implement proxy-owned durable recovery/status or prove that vmctl job
records are required. No runtime code has been changed in this checkpoint.

## 2026-06-15T10:57:20Z - Local Durable Recovery Repair

Claim/scope: proxy-owned durable recovery/status is sufficient for the local
first-load cancel/reload-success shape. Once an authenticated owner asks for
recovery, browser request cancellation must not cancel the underlying vmctl
refresh; later compute status/bootstrap should observe the completed route.

Move: implemented a proxy recovery tracker and refactored
`HandleComputeRecovery` so `wake_current_computer`/`resume_current_computer`
starts or joins a detached owner/desktop recovery operation. Fast vmctl
completion still returns the existing synchronous `200` response. Slow recovery
can return `202`, and canceled browser requests return without canceling the
detached vmctl work. `/api/compute/status` now includes a redacted `recovery`
object but keeps current computer/runtime facts from a fresh lookup/probe rather
than overlaying terminal recovery snapshots.

Expected Delta V: -2 for deterministic regression plus local repair. Actual
Delta V: -2 locally. Staging deploy/proof and Universal Wire route separation
remain open, so the mission is not settled.

Receipts:

- Regression:
  `TestComputeRecoveryContinuesAfterClientCancelAndStatusBootstrapObserveReady`
  cancels the browser recovery request while vmctl refresh is blocked, verifies
  the vmctl refresh context remains live, observes `recovery.status=refreshing`
  through compute status, releases refresh, then verifies status `ready` and
  bootstrap `200` with the recovered sandbox id.
- Compatibility regression:
  `TestComputeRecoveryWakeKeepsObservationWhenUnreachableRefreshFails` preserves
  the previous behavior where a refresh failure after successful lookup returns
  the current unreachable runtime observation instead of converting the whole
  recovery response to `502`.
- Focused checks passed:
  `nix develop -c go test ./internal/proxy -run 'TestComputeRecovery' -count=1`;
  `nix develop -c go test -race ./internal/proxy -run 'TestComputeRecovery' -count=1`;
  `nix develop -c go test ./internal/proxy -count=1`.
- Broad check on the final staged state passed:
  `nix develop -c scripts/go-test-local`.
- Independent prover found two blocking issues before this ledger entry:
  terminal recovery snapshots could stale-overlay fresh compute status, and
  refresh failure on an existing unhealthy current computer had become a `502`.
  Both were repaired before commit.

Open edge: commit, push, monitor CI/deploy, prove staging owner-path recovery,
and report or repair the stale Universal Wire/sourcecycled route separately.
