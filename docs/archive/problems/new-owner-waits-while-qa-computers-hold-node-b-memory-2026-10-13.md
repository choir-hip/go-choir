# A new owner's computer waits while finished QA computers hold Node B memory

Found 2026-10-13 by the Cut 0 tracer agent (station C0 of
`docs/archive/definitions/choir-reeng-metamission-2026-10-12.md`). Status:
documented; mitigated for the tracer, not fixed for owners.

## What happens

While several finished test computers were still running, a freshly
registered owner's computer stayed `not_started` for 20 minutes
(`GET /api/runtime/observability` answered 503 throughout; tracer label
`c0-tracer-dev-4`). As soon as the agent stopped its finished test
computers, new mints succeeded within about 12 seconds again
(`c0-tracer-dev-8`: mint 12.4 s, prompt bar ready at 11.4 s).

## Likely mechanism (from code; not yet confirmed in vmctl logs)

vmctl refuses a start that would add a VM to a host below its memory
floor (`RecoveryRefusalHostMemoryPressure`, internal/vmctl/host_capacity.go),
the guard added after the 2026-10-10 out-of-memory incident
(docs/archive/problems/node-b-oom-from-retained-qa-computers-2026-10-10.md). Idle
QA computers are not reclaimed fast enough to make room, so the new
owner's boot waits on retry. The confirming observation is vmctl's
refusal reason for that owner's computer in the health or ownership
projection, read without SSH.

## Why it matters

- A real new owner signing up while QA or idle computers hold memory
  would see no computer for many minutes.
- Every cut's acceptance mints fresh test computers (three tracer runs per
  cut); leftover computers stall later mints and skew the
  push-to-healthy-test-computer measurement.
- Minting during a deploy also stalled (`c0-tracer-dev-1`, `-dev-3`).

## Mitigation in place

The tracer stops the computer it minted at the end of each attempt
(scripts/reeng_tracer.mjs). Five early dev owners were registered without
keys and cannot be stopped by the tracer; they are disposable and mostly
never started.

## Open

Whether idle-computer reclaim should make room for a waiting first boot
(reclaim policy, red: VM lifecycle) is a later station's decision; Cut 0
only records the observation and keeps its own footprint small.
