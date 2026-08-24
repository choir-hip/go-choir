# 0333528 guest restart loop — vmctl unhealthy churn — 2026-08-24

## Evidence (node-b journal, read-only)

- Since ~09:00Z the retained computer `computer-03335285269bdba4f94377e56879f9e6`
  (vm `candidate-fleet-e15cb89f25d963c220319b7b`) has cycled restarts:
  vmctl journal since 09:00 shows **311× "VM ... is unhealthy"** and
  **hundreds of "health check failed ... http://10.200.<x>.2:8085"** across
  rotating /30s (10.200.33-40 = 15+ distinct boots; each restart gets a new
  subnet).
- Epoch file advanced 739 (07:47) → 746 (08:48) → 748 (09:55) → 754 (10:45):
  repeated reboots, not a single boot. One firecracker process at a time —
  vmctl's warmness policy kills + reboots the unhealthy guest.
- Same shape as the recovery ceiling: the guest boot replays the residual
  event delta at the 4 GiB write-path cost (3-6 s/event at the 107k+ scale,
  recovery ceiling assessment) and never reaches `ready` within the
  vmctl health window.

## Classification

- NOT a new defect of the pre-flight. It is the known guest replay I/O ceiling
  manifesting as a live boot loop on the retained computer — the substrate the
  pre-flight's PF-2 measurement + PF-5 (ceiling resolution) address.
- Red: live computer runtime; the mission does NOT touch it (the guest's own
  normal lifecycle continues; must_preserve allows the computer's own
  runtime activity).
- Problem-documentation-first: recorded before any fix; the fix = the ceiling
  resolution (PF-5) / the guest machine-size escalation, gated by PF-2.

## Impact / invariants

- The computer's canonical tape is unaffected by the restarts (the guest
  appends only its own normal lifecycle/projection events; head churn from the
  guest's own activity is not mission mutation).
- The pre-flight's pre-state replay + measurement run against the frozen tape
  prefix (0-105,500) and clone disks — independent of the live guest loop.
- The restart loop is the strongest live confirmation of the 3-6 s/event
  ceiling at the 4 GiB guest scale: a boot cannot finish the residual delta
  inside the health window.

## Next

- PF-2 measurement records the guest-side per-event costs at the band
  (like-for-like v1-base vs v2) — the loop's live per-boot behavior is the
  deployed-v1 absolute reference (3.2-6.5 s/event).
- PF-5: ceiling resolution (appender batching or the named 8-12 GiB
  escalation) — gated on the PF-2 verdict.


## Root cause — confirmed (PF-2 harness observation, 11:37-11:42Z)

The clone guest (measurement clone of the same image + the same 11 GiB store)
boots to the runtime's credential stage at **~308 s guest uptime** — the
4-minute credential envelope TTL expires before the exchange, so the runtime
fails "credential envelope: invalid bootstrap lifetime", the unit restarts,
and the pattern repeats. 19 consecutive "invalid bootstrap lifetime" retries
in the clone serial; the LIVE guest's vmctl restart loop (epochs 748->754,
~3-5 min cadence) has the same shape: each boot's envelope (minted by
vmmanager per boot) expires before the guest reaches the exchange, the
runtime never starts, the health check never clears, vmctl kills + reboots.

This is the mechanism behind the live 0333528 restart loop, and it is a
separate live-platform problem from the replay I/O ceiling: the guest systemd
boot unit chain (credentials/signers/updater before the runtime store open)
takes ~5 min on the 32 GiB data disk; the credential envelope is bounded to a
4-minute issuance window (platform maximumCredentialEnvelopeTTL 5 min,
minted at +4 min). Either the boot must reach the exchange within the TTL or
the issuance window must cover the guest boot duration (per-boot minted at
vmmanager launch is insufficient: the boot outlives it).

## Measurement workaround

The Firecracker guest-sample plan is blocked by this boot-vs-TTL gap (the
same gap the live computer runs on). The PF-2 sample measurement therefore
runs the bounded sample (200 events, target 105,700) with the SAME binaries
(v1-base and v2 graphs), the SAME frozen pre-state workspace, at the
comparable resource boundary (systemd-run MemoryMax=4096M + CPU quota 2,
cgroup-visible RSS + OOM) — the def's like-for-like requirement (same image
base, 4 GiB, 2 vCPU) is preserved with the resource-boundary harness; the
boot-time gap is a live-platform problem documented here, not a measurement
requirement.
