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
