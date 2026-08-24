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
