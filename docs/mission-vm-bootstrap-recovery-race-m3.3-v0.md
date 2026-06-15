# Mission M3.3 - VM Bootstrap Recovery Race v0

## Summary

Owner-facing boot sometimes shows `BOOTSTRAP FAILED (502)` or pending recovery,
then a manual browser reload succeeds. The old recovery logic was introduced to
repair stale or unhealthy VM routes during the 2026-06-09 disk-full incident.
That incident's terminal cause was a full owner `data.img`, but the recovery
patches left a new behavior: foreground bootstrap and recovery requests can time
out or cancel while vmctl is still doing useful recovery work. The user sees
failure even when the VM becomes ready seconds later.

This paramission makes that behavior debuggable and then fixes it narrowly
before M3 uses vmctl refresh/restart as lifecycle-settlement evidence.

## Current Evidence

- Owner screenshot on 2026-06-15 at about 06:17 America/New_York showed
  `BOOTSTRAP FAILED (502)`, repeated `VM route returned 502; retrying`, then
  `Requesting computer recovery`.
- Node B logs at `2026-06-15T10:17:12Z` and `2026-06-15T10:17:21Z` showed proxy
  bootstrap/recovery calls for owner `5bd6de97-3b58-408c-bf89-c42c81b083de`
  canceling while vmctl was resolving or refreshing.
- vmctl then completed useful work at `2026-06-15T10:17:24Z`: owner VM
  `vm-5b0c1bef1e2b6d7f8dad7d0e8473ed19` refreshed active at
  `http://10.200.44.2:8085`.
- A later check found the same owner VM active at `http://10.200.52.2:8085`,
  `status=ready`, deployed commit `0a5fb602151c8373086c4a2774e1236faa53831b`,
  and persistent disk use about `8.9%`.
- Public health still reported aggregate bootstrap instability:
  `bootstrap.total` 52 requests, 40 HTTP 200, 3 HTTP 502, 9 resolve errors,
  max bootstrap duration about 15 seconds.
- A separate public platform/source route is also stale:
  `vm-universal-wire-platform` is persisted as `booting` at
  `http://10.200.17.2:8085`, and `sourcecycled` retries through vmctl's
  Unix-socket sandbox proxy about every 32 seconds with 502s. Node B vmctl logs
  after the owner-path deploy showed reverse-proxy dial timeouts to
  `10.200.17.2:8085`; a direct operator probe to that route timed out. This may
  pollute aggregate health but should not be conflated with the owner boot path.

## Not This Mission

- Do not re-open the old disk-full recovery as the primary theory unless fresh
  evidence shows guest persistent disk pressure again.
- Do not absorb the full System Monitor mission. This mission may add or repair
  narrow status/recovery evidence needed to fix first-load boot behavior, but it
  should not build a broad dashboard.
- Do not weaken M3 by treating "reload succeeds eventually" as lifecycle proof.
- Do not delete VM state, prune storage, restart live services, or clean up
  platform-source state as part of this mission without explicit evidence and
  rollback refs.

## Parallax State

status: open_handoff

mission conjecture: if bootstrap/recovery becomes a durable, owner-scoped
computer lifecycle transition that survives browser request cancellation and
reports honest progress, then M3 can safely use vmctl refresh/restart as a
staging proof substrate without first-load/reload-success false failures.

deeper goal (G): users should experience Choir as a persistent computer that can
wake, recover, and explain its state without operator intervention or reload
rituals. M3 lifecycle proof also needs the same substrate to distinguish real
actor-rewarm failures from boot-control noise.

witness/spec (A/S): a narrow runtime/proxy/vmctl/frontend repair plus deployed
proof. The witness must either eliminate the first-load 502/reload-success
failure or turn it into a product-visible durable recovery state that continues
after the browser request ends and opens the desktop automatically when ready.

invariants / qualities / domain ramp (I/Q/D):
- I: active computer data is preserved; recovery may refresh/reboot the current
  owner computer but must not delete or replace `data.img`.
- I: recovery is owner-scoped and browser-public responses stay redacted: no
  emails, user identifiers, VM identifiers, sandbox URLs, host paths, or
  credentials.
- I: foreground browser cancellation must not cancel the underlying recovery job
  once an authenticated owner has requested it.
- I: M3 must not settle on a proof polluted by bootstrap/recovery control-plane
  races.
- Q: status should distinguish `resolving`, `waking`, `refreshing`, `ready`,
  `failed`, and `source/platform route unhealthy` well enough for the boot UI
  and operator health to avoid conflating separate routes.
- D ramp: local unit tests for proxy/vmctl cancellation and status semantics;
  staging owner-path proof on `https://choir.news`; optional separate staging
  proof or blocker for the Universal Wire platform route.

variant (ranking function) V: 1.
1. reproduce or synthesize a deterministic first-load cancel/reload-success
   failure shape; (implemented in
   `TestComputeRecoveryContinuesAfterClientCancelAndStatusBootstrapObserveReady`)
2. identify whether the fix belongs in proxy recovery context, vmctl recovery
   job semantics, frontend boot polling, or a combination; (current evidence:
   proxy-owned durable recovery/status is the narrow v0 locus; vmctl job records
   are not yet required)
3. implement the narrow durable recovery/status repair with tests; (local proxy
   repair implemented; focused checks, race check, and broad local gate passed)
4. deploy and prove the owner-path first-load recovery behavior on staging;
   (supported for fresh owner boot/recovery status on deployed `a1c0ad0d`;
   cancellation-specific proof remains the deterministic local regression)
5. separate or repair the stale Universal Wire/sourcecycled 502 route so health
   counters no longer hide owner bootstrap regressions. (separated and still
   failing in `go-choir-sourcecycled.service`; successor blocker remains)

budget: one focused paramission before M3. If the fix expands into broad System
Monitor or storage-retention work, stop with an open handoff and keep M3 blocked
on the narrow substrate.

authority / bounds: mutation class red for runtime behavior. Protected surfaces:
proxy `/api/shell/bootstrap`, `/api/compute/recovery`, vmctl resolve/refresh,
active user computers, source/platform computer routing, frontend boot/recovery
state, health lifecycle counters, staging deploy. Problem Documentation First
applies before code changes.

evidence packet: before/after lifecycle counters, focused proxy/vmctl/frontend
tests, `nix develop -c scripts/go-test-runtime-shards` if runtime paths change,
frontend test/build if Desktop changes, pushed commit, CI, Node B deploy,
staging health identity, deployed product-path proof showing first page load
recovers without manual reload after a cold/hibernated owner computer, and a
separate note for the Universal Wire/sourcecycled route.

heresy delta: discovered: recovery logic built for the old disk-full/stale-route
incident can now present failure while recovery succeeds later. introduced: none
accepted. repaired: only after deployed proof shows browser request cancellation
no longer strands or misreports recovery.

position / live conjectures / open edges:
- C1 active: the owner-visible bug is a cancellation/control-plane mismatch:
  browser probe timeout and request context cancel proxy recovery before vmctl's
  useful refresh completes.
- C2 active: sourcecycled's stale Universal Wire platform route is a separate
  health-noise source and may need its own recovery path, but should not drive
  the owner bootstrap fix.
- C3 active: current `HandleComputeRecovery` still uses `r.Context()` for
  lookup, resolve, and refresh, and it returns only the completed synchronous
  response. If the browser cancels or a foreground deadline trips, the proxy
  cancels the vmctl request and has no product-visible recovery operation for
  `/api/compute/status` or the next bootstrap probe to observe.
- C4 testing: v0 can be proxy-owned durable status over existing vmctl
  operations: detach the owner-authorized refresh/wake operation from the
  browser request context, coalesce it per owner/desktop, expose redacted
  `recovery_status`/state through compute status, and let bootstrap polling
  observe the refreshed route when vmctl completes.
- C5 supported locally: proxy-owned status is enough for the deterministic unit
  shape. The implementation starts a detached, coalesced recovery operation
  after authentication, waits for fast completion to preserve existing
  synchronous success behavior, returns/ends safely when the browser request is
  canceled or slow, and exposes only redacted `recovery` status through compute
  status. Status does not overlay terminal recovery current/runtime snapshots on
  fresh lookup results, avoiding stale route reports after later lifecycle
  changes. Explicit vmctl job records remain a successor only if staging shows
  proxy-owned status is insufficient.
- C6 supported on staging for owner-path recovery: commit
  `a1c0ad0d5ba6f7923c19f0346da979a7ea51a818` deployed to proxy and sandbox at
  `2026-06-15T11:04:42Z`; CI run `27541798919` and Node B deploy passed. A
  deployed lifecycle proof passed, and a fresh owner product-path probe
  registered `m33-recovery-1781521988690-t9qqci@example.com`, reached
  authenticated desktop ready in about 8s, got `/api/compute/recovery`
  `200` with redacted `recovery.status=ready`, then got
  `/api/compute/status` `200` and `/api/shell/bootstrap` `200` for sandbox
  `vm-711255255b16ffdd090879de629fd32d` without manual reload. Staging recovery
  completed too quickly to produce an aborted browser request, so the
  cancellation predicate is proven by the deterministic local regression rather
  than by a slow staging recovery.
- C7 active: the Universal Wire platform/source route is not just a
  sourcecycled configuration problem. Sourcecycled now targets vmctl's
  Unix-socket sandbox proxy for `universal-wire-platform`, but `HandleSandboxProxy`
  calls `LiveSandboxURL(owner, "platform")`, which can return the cached route
  for a persisted `booting` ownership without first ensuring or recovering the
  platform computer. On Node B, that ownership is
  `vm-universal-wire-platform`, `state=booting`, `sandbox_url=http://10.200.17.2:8085`,
  `epoch=58`; sourcecycled POSTs through the proxy still produce 502s while vmctl
  logs dial timeouts to `10.200.17.2:8085`.

next move: implement the narrow platform route repair in vmctl, not in
sourcecycled: before proxying the Universal Wire platform owner, ensure/recover
the `universal-wire-platform` / `platform` computer or return an explicit
service-unavailable error instead of blindly reverse-proxying a stale booting
route. Add deterministic vmctl coverage for a persisted booting ownership with
no in-memory boot waiter.

ledger file: `docs/mission-vm-bootstrap-recovery-race-m3.3-v0.ledger.md`

version / lineage: created on 2026-06-15 as M3.3 after settled M3.1/M3.2 and
before M3 lifecycle cutover settlement. Related docs:
`docs/incident-vm-bootstrap-stale-route-2026-06-09.md`,
`docs/mission-lifecycle-cutover-v0.md`,
`docs/mission-computer-recovery-system-monitor-v0.md`,
`docs/mission-node-b-storage-retention-v0.md`.

learning state: the old visible symptom (`BOOTSTRAP FAILED (502)`) no longer
means one root cause. It can be stale route, guest disk full, vmctl cleanup leak,
source/platform stale route, or foreground cancellation during a useful refresh.
The product must expose enough state to distinguish those classes.

settlement: settled only when staging proves owner bootstrap/recovery no longer
requires manual reload after a cold/hibernated/recovered current computer,
health separates owner bootstrap from source/platform route failures, and the
final report records commit, CI, deploy identity, staging proof, rollback refs,
heresy delta, and residual risk. If source/platform recovery remains unsolved,
record it as a separate successor blocker rather than hiding it in aggregate
502s.

## Suggested Goal String

```text
Use Parallax on docs/mission-vm-bootstrap-recovery-race-m3.3-v0.md. Treat it
as the pre-M3.3 bootstrap/recovery substrate repair before M3 lifecycle
settlement. Current status is open_handoff with V=5. Preserve Choir Doctrine,
AGENTS.md, docs/computer-ontology.md, and the M3.1/M3.2 VText/control-plane
invariants. The bug is not simply the old disk-full incident: owner bootstrap
can show BOOTSTRAP FAILED (502) or pending recovery, then manual reload works,
because browser request cancellation and foreground bootstrap deadlines can
misreport or cancel recovery while vmctl completes useful refresh seconds later.

Mutation class is red. Protected surfaces include proxy /api/shell/bootstrap,
/api/compute/recovery, vmctl resolve/refresh, active user computers, source or
platform computer routing, frontend boot/recovery state, health lifecycle
counters, and staging deploy. Do not delete VM state, prune storage, restart live
services, or broaden into the full System Monitor mission without explicit
evidence and rollback refs. Problem Documentation First applies before code
changes.

First move: read the Parallax State,
docs/incident-vm-bootstrap-stale-route-2026-06-09.md,
frontend/src/lib/Desktop.svelte, internal/proxy/compute_status.go,
internal/proxy/handlers.go, internal/vmctl/client.go, and
internal/vmctl/ownership.go. Build a deterministic regression for the
first-load cancel/reload-success shape: authenticated owner requests recovery,
the browser request cancels or times out, vmctl still completes refresh, and the
next bootstrap/status observes ready without manual reload. Then implement the
narrow durable recovery/status repair. Required verification: focused tests for
touched paths, runtime shards if runtime/proxy/vmctl changes, frontend tests or
build if Desktop changes, push to origin/main, CI, Node B deploy, staging health
identity, and deployed product-path proof that first page load recovers without
manual reload after a cold/hibernated owner computer. Also separate or explicitly
record the stale Universal Wire/sourcecycled platform-route 502 so aggregate
health does not mask owner bootstrap behavior. Settlement requires commit, CI,
deploy identity, staging proof, rollback refs, heresy delta, and residual risk.
```
