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

## 2026-06-15T11:15:00Z - Deployed Owner Recovery Proof And Source Route Separation

Claim/scope: the owner bootstrap/recovery substrate repair landed and is
supported on staging for the owner path, but the Universal Wire platform/source
route remains a separate unresolved 502 source. The mission should therefore
handoff with a named remaining edge instead of claiming full settlement.

Move: pushed runtime commit `a1c0ad0d5ba6f7923c19f0346da979a7ea51a818` to
`origin/main`, monitored CI/deploy, verified staging identity, ran deployed
owner lifecycle proof, ran an authenticated fresh-owner recovery/status probe,
and inspected sourcecycled post-deploy logs.

Expected Delta V: -1 for deployed owner-path proof and route separation. Actual
Delta V: -1. V is now 1 for the remaining Universal Wire platform/source route
successor blocker.

Receipts:

- CI run `27541798919` completed successfully for
  `a1c0ad0d5ba6f7923c19f0346da979a7ea51a818`; Node B `Deploy to Staging`
  completed successfully in the same run.
- `curl -fsS https://choir.news/health` reported proxy and upstream sandbox
  deployed commit `a1c0ad0d5ba6f7923c19f0346da979a7ea51a818`, deployed at
  `2026-06-15T11:04:42Z`.
- Deployed lifecycle proof passed:
  `GO_CHOIR_RUN_DEPLOYED_LIFECYCLE=1 CHOIR_DEPLOYED_BASE_URL=https://choir.news npx --prefix frontend playwright test frontend/tests/adaptive-lifecycle-control-deployed.spec.js --reporter=line`.
- Fresh-owner deployed recovery/status probe registered
  `m33-recovery-1781521988690-t9qqci@example.com`, reached authenticated
  desktop ready in about 8s, observed `/api/compute/status` `200` with
  `current_computer.state=active` and `runtime.status=ready`, observed
  `/api/compute/recovery` `200` with redacted `recovery.status=ready`, then
  observed `/api/shell/bootstrap` `200` for
  `vm-711255255b16ffdd090879de629fd32d` without manual reload.
- Staging recovery completed in about 30ms, so it did not produce an aborted
  browser request. The cancellation-specific predicate remains covered by
  `TestComputeRecoveryContinuesAfterClientCancelAndStatusBootstrapObserveReady`.
- Deployed `/health` after the probe showed owner/proxy `bootstrap.total` count
  13 with `http_200` only in the active window, separating the owner bootstrap
  path from sourcecycled failures.
- Read-only Node B diagnostics showed `go-choir-sourcecycled.service` still
  active and still logging repeated `runtime returned 502 Bad Gateway` dispatch
  attempts after the deploy, including `2026-06-15T11:13:48Z`.

Open edge: open or resume a narrow Universal Wire platform-computer recovery
mission for `universal-wire-platform` / `platform` before treating
sourcecycled dispatch health as repaired. Do not hide that 502 class inside
owner bootstrap health.

## 2026-06-15T11:22:30Z - Problem-First Checkpoint For Platform Proxy Booting Route

Claim/scope: the remaining sourcecycled 502 edge is a vmctl platform-computer
route problem, not the old static `SOURCE_SERVICE_RUNTIME_BASE_URL` problem and
not an owner bootstrap regression. Sourcecycled is already using the vmctl
Unix-socket sandbox proxy, but vmctl can route that proxy to a persisted
`booting` Universal Wire platform ownership that has no active in-memory boot
operation after service restart.

Move: inspected Node B service environment, sourcecycled/vmctl logs, vmctl
list state, and the sandbox proxy/ownership code before changing behavior.
Rewrote Parallax State to make the source/platform route repair the next
bounded red-surface move.

Expected Delta V: 0 for repair, +0 for documentation required before a red
behavior change. Actual Delta V: 0; the owner recovery substrate remains
supported, and the remaining V=1 edge is now narrower.

Receipts:

- `go-choir-sourcecycled.service` has `SOURCE_SERVICE_RUNTIME_OWNER_ID=universal-wire-platform`
  and `VMCTL_SANDBOX_PROXY_SOCK=/run/go-choir/vmctl.sock`.
- `cmd/sourcecycled/main.go` builds UDS endpoint
  `/internal/vmctl/sandbox-proxy/{owner}/internal/runtime/runs`.
- `internal/vmctl/handlers.go` sandbox proxy resolves
  `LiveSandboxURL(ownerID, "platform")` before reverse proxying.
- Node B `go-choir-sourcecycled.service` logged repeated
  `runtime returned 502 Bad Gateway` dispatch attempts after deployed commit
  `a1c0ad0d5ba6f7923c19f0346da979a7ea51a818`.
- Node B `go-choir-vmctl.service` logged matching proxy errors:
  `dial tcp 10.200.17.2:8085: i/o timeout`.
- Operator `GET /internal/vmctl/list` over the vmctl Unix socket showed
  `vm-universal-wire-platform`, owner `universal-wire-platform`, desktop
  `platform`, `state=booting`, `sandbox_url=http://10.200.17.2:8085`,
  `epoch=58`.
- Direct operator probe to `http://10.200.17.2:8085/health` timed out.

Open edge: add vmctl regression coverage and repair the sandbox proxy/platform
computer readiness path, then deploy and prove sourcecycled can reach the
platform runtime without hiding owner bootstrap health.

## 2026-06-15T11:46:00Z - Platform Proxy Route Repair Settled

Claim/scope: the remaining Universal Wire/sourcecycled 502 edge was repaired
inside vmctl routing. The sandbox proxy now ensures/recovers the platform
computer before reverse-proxying the Universal Wire platform owner, and a
persisted `booting` platform ownership without an in-memory waiter is recovered
instead of treated as a live route.

Move: added deterministic vmctl tests, committed runtime repair
`04a466f8761da772e9198c46011f2ad39018c4b2`, pushed to `origin/main`, monitored
CI and Node B deploy, verified staging identity, and ran deployed operator
diagnostics against vmctl's Unix-socket proxy and platform runtime.

Expected Delta V: -1 for repairing and proving the final source/platform route
edge. Actual Delta V: -1. V is now 0; the mission is settled with a separate
residual capacity/backpressure risk.

Receipts:

- Problem Documentation First was satisfied by prior docs checkpoint
  `9c25abce`.
- Focused and package checks passed after implementation:
  `nix develop -c go test ./internal/vmctl -run 'TestEnsureUniversalWirePlatformComputer|TestSandboxProxy' -count=1`;
  `nix develop -c go test ./internal/vmctl -count=1`.
- Broad local gate passed:
  `nix develop -c scripts/go-test-local`.
- CI run `27543626790` completed successfully for
  `04a466f8761da772e9198c46011f2ad39018c4b2`, including Go vet/build, runtime
  shards, non-runtime tests, integration smoke, TLA+ model check, docs truth
  check, and Node B `Deploy to Staging`.
- `curl -fsS https://choir.news/health` reported status `ok`, vmctl status
  `ok`, proxy commit and upstream sandbox commit
  `04a466f8761da772e9198c46011f2ad39018c4b2`, deployed at
  `2026-06-15T11:40:31Z`.
- Node B `GET /internal/vmctl/list` over `/run/go-choir/vmctl.sock` showed
  `vm-universal-wire-platform`, owner `universal-wire-platform`, desktop
  `platform`, `state=active`, `sandbox_url=http://10.200.55.2:8085`,
  `epoch=60`, `last_active_at=2026-06-15T11:41:43.827Z`.
- Node B vmctl logs showed recovery/boot of `vm-universal-wire-platform`,
  including `vmmanager: booted VM vm-universal-wire-platform
  (host=http://10.200.55.2:8085 epoch=60)`.
- Operator route diagnostic:
  `POST /internal/vmctl/sandbox-proxy/universal-wire-platform/health` over the
  vmctl Unix socket returned HTTP `405 Method Not Allowed`, proving the proxy
  reached the runtime instead of timing out on stale `10.200.17.2`.
- Direct platform health:
  `GET http://10.200.55.2:8085/health` returned HTTP `200` with
  `status=ready`, `sandbox_id=vm-universal-wire-platform`, deployed commit
  `04a466f8761da772e9198c46011f2ad39018c4b2`, disk use about `21.95%`.
- Sourcecycled logs after the platform route recovery window showed runtime
  `429 Too Many Requests` backpressure and no checked vmctl `502` or dial
  timeout route failures.

Residual risk: sourcecycled still cannot submit additional processor runs while
the platform runtime reports about 45 running processor runs / 50 total running
runs; that is capacity/backpressure behavior, not the stale vmctl route failure
this mission repaired. Cancellation-specific owner staging proof remained too
fast to naturally abort; the exact cancellation predicate is covered by the
deterministic local regression.
