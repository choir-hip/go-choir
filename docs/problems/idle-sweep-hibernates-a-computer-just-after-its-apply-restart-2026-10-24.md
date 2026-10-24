# The idle sweep hibernates a computer 35 s after its self-development apply restart

Status: open. Recorded 2026-10-24 by FP0. Mutation class of this record: green.

## What happened

The D34 proof drill (label fp0-jobpark-1024b) ran on a fresh class:test computer,
computer-734d0344e9d5d7e51eef11df61f58a5b (VM vm-ee2fb953783eef4a7f8debba046f9494),
minted at 05:56:59 UTC on platform 911be16b1.

- 06:12:41 the owner approval was accepted.
- 06:12:41 to 06:26:59 the host build (about 14 min).
- 06:27:01 the planned restart booted the guest (restart_kind planned,
  reason self_development_apply, no crash lines).
- 06:27:08 the operation read `applied`.
- 06:27:36 vmctl hibernated the VM by snapshot, `reason=idle`
  (`hibernateVMForDesktopWithReason`, epoch 13276).
- 06:27:28 to 06:51:43 every API-key request from the drill was refused by the
  proxy with 502 "computer authority unavailable"
  (`state="booting"` then `state="hibernated"`,
  `cause=api key computer realization unavailable`).
- The drill could not read its post-apply witness, served check, probe or
  restore. It exited 6. Its trace export ran against a hibernated computer
  and every layer read `unserved`.

An operator resolve at 06:5x woke the computer. A second export read every
layer, which carried the D34 evidence (see the ledger).

## Why, from the code

- `VMCTL_IDLE_TIMEOUT=30m` (nix/node-b.nix). The hibernate fell exactly 30 min
  after the mint (05:57:36 + 30 min = 06:27:36).
- `idleOwnershipCandidates` (internal/vmctl/warmness_policy.go) measures idle
  from `LastActiveAt`, which tracks ownership-resolution traffic only. The
  drill's traffic after the mint was API-key traffic, and the API-key path
  (`resolveComputerURLForComputerTarget`,
  internal/proxy/api_key_computer_authority.go) reads the target's state
  without a resolve, so it never refreshed `LastActiveAt`. The owner approval
  did not refresh it either. Neither did the planned apply restart.
- Until the restart, the guest's own busy signal kept the computer out of the
  sweep (the engineering run, then the operation in flight). The freshly
  booted guest answered /health with no work, so the first sweep after the
  boot found a computer idle for 30 min and hibernated it.
- The API-key path never wakes a computer: a non-active realization is a 502,
  not a resolve. So a client that uses only an API key cannot reach a
  computer that hibernated under it.

## Who it reaches

- Any self-development cycle driven by API key (the drill, the CLI, the
  overnight app-rewrite conductor on b@c) whose post-apply checks start more
  than the idle timeout after the last ownership resolution. A browser user
  is less exposed, because the desktop resolves and wakes.
- A computer that just took a planned restart reads idle to the sweep even
  when an owner act or an operator check is waiting on it.

## Not the cause

- Not D34 (the job park lives inside the engineering run, before the freeze).
- Not the vmctl list failure of 04:28 (fixed by D33): at 06:5x the list
  returned 200 with 156 ownerships, and the paper and b@c were active.

## Open decisions (not taken here)

1. Should a planned restart, an owner act or an authenticated API-key request
   refresh `LastActiveAt`? The narrowest is the planned restart: a boot is
   activity.
2. Should the API-key path resolve (and so wake) a hibernated computer, as the
   desktop path does? That widens what an API key can cause on the host.
3. Should the drill keep its computer warm across the host build (a held
   lease), so the proof does not depend on the sweep?

## Evidence

- Drill log: /Users/wiz/Dev/lanes/_runs/drill-d34.log (local).
- Receipt and bundles: ~/.config/choir-qa/traces/fp0-jobpark-1024b/ (local):
  receipt.json, bundle-a1.json (every layer unserved), bundle-a1-woken.json
  (after the operator resolve).
- Node B journal, go-choir-vmctl and go-choir-proxy, 06:26:30 to 06:52 UTC.

## Design for open decision 1 (FP0 loop-gaps lane, 2026-10-24; no code yet)

Gap K of `docs/design/app-packages-replan-2026-10-24.md` section 5. The fix is
in vmctl and in the proxy's API-key authority, so it is red (vmctl, auth). Per
the lane brief the lane stops at this design. Mutation class of this section:
green.

### Where activity is recorded today

`LastActiveAt` is written only inside vmctl's registry, and only on a resolve
or a lifecycle transition: `resolveDesktopContext` (`internal/vmctl/ownership.go`
lines 2137, 2151, 2235), the platform-computer paths
(`internal/vmctl/platform_computer.go` lines 146 to 520) and creation. The
cookie path reaches those through `resolveComputerURL`, so every browser
request that resolves refreshes it. The API-key path joins the key to an
ownership with `LookupComputerContext` (a `GET /internal/vmctl/lookup`,
`HandleLookup`, `internal/vmctl/handlers.go:456`), which reads and writes
nothing, then uses the ownership's `ComputerURL` directly
(`resolveComputerURLForComputerTarget`). That is deliberate: the key path must
never wake a computer. The missing piece is a write that refreshes activity
without resolving.

### Proposed change

1. vmctl: a new internal-only route `POST /internal/vmctl/touch` with body
   `{user_id, computer_id}` (or a `Registry.TouchComputerActivity(userID,
   computerID)` method behind it). It joins on the same `(UserID, ComputerID)`
   pair the lookup does, and:
   - sets `LastActiveAt = now` only when `State == VMStateActive`; for any
     other state it changes nothing and returns the state. It never resolves,
     wakes, starts or boots (this preserves "an API key cannot wake a
     computer", open decision 2 stays open and unchanged);
   - skips the write when the stored value is under 60 s old, so a polling
     client costs one registry write a minute, not one per request;
   - calls `saveLocked` on a real update only.
2. proxy: `requireAPIKeyComputerTarget` is the single choke point for every
   key route (`internal/proxy/api_key_computer_authority.go`). After both of its
   success returns it calls `h.touchComputer(target)`: fire-and-forget on a
   goroutine with a 2 s timeout, throttled by a per-ComputerID `sync.Map` of the
   last send (30 s), errors logged and dropped. A failed touch never fails or
   delays the request.
3. Nothing changes in the cookie path, the held or protected warmness classes,
   or the guest busy probe.

The planned apply restart (the second half of decision 1) is a different
writer: vmctl itself performs the refresh, so it can set `LastActiveAt` where it
records the restart marker. That is a separate small change in
`HandleApplySelfDevelopmentRouteProjection`'s path and is left out of this
design until the touch lands, because the touch alone keeps an API-key-driven
computer warm through the restart (the drill polled every few seconds).

### Failure modes, written before code

1. Key-driven traffic only, a request every 5 min for 40 min on an
   `idle_timeout=30m` host: the computer must not be swept. This is the bug.
2. A touch for a hibernated, stopped, stopping or booting ownership changes
   nothing: state, `LastActiveAt`, epoch and `ComputerURL` stay as they were,
   and no VM is started.
3. Owner A's key naming owner B's computer: the proxy's existing ownership join
   refuses it before any touch; vmctl's touch also requires the `(user,
   computer)` pair to match, so a bug in the proxy still cannot touch a
   foreign computer.
4. vmctl unreachable or slow: the request still succeeds and returns in the
   same time (no touch on the request path).
5. A client polling at 1 Hz: at most one registry write a minute per computer,
   and one proxy call every 30 s.
6. A held computer, a protected class and a computer with in-flight runs
   behave as before; the touch only moves the idle clock.
7. Persistence: after a vmctl restart `LastActiveAt` is the last saved value, no
   older than the last real touch minus 60 s.

### Open choices this design makes, with defaults

- Every authenticated key request counts as activity, including read-only
  polls. A monitor that polls a key-bound computer will keep it warm. Default:
  accept that, since the owner's own CLI is the common client and it is also
  the client that was hibernated mid-flow. If a monitor must not hold a
  computer, it can send a request class the proxy answers itself.
- `POST /internal/vmctl/touch` rather than a flag on `GET /internal/vmctl/lookup`,
  so a lookup stays free of side effects.

### Tests for the fix commit

A vmctl registry test (touch refreshes an active ownership, leaves a hibernated
one untouched, honors the 60 s floor, refuses a wrong user), a proxy test (an
API-key request on an active target sends exactly one touch inside the throttle
window and a vmctl failure does not change the response), and the existing
warmness sweep test extended with the key-only scenario of failure mode 1.
Tier 1 review (Devin). Rollback: revert; the route is additive and nothing
depends on it.
