# Candidate Realizations Never Reach Readiness and Are Killed Every 180 Seconds

**Date:** 2026-08-13  
**Status:** open; investigation in progress  
**Classification:** VM lifecycle, guest readiness, and staging operability  
**Mutation class of a repair:** red

## Observation

Staging deployed `633131aa0521bd1a427f335e147610a314829886` at
`2026-08-13T02:00:57Z`. After that deployment, both candidate-fleet
realizations repeatedly boot, start the guest autoputer runtime, fail to become
reachable, and are killed approximately every 180 seconds.

At `2026-08-13T02:57:38Z`, Node B reported new candidate firecracker processes
with 31 seconds uptime while the two primary firecracker processes retained
approximately 27 days of uptime. The preceding candidate exits were:

```text
2026/08/13 02:48:02 candidate-fleet-e15cb89f... exited with error: signal: killed
2026/08/13 02:48:03 candidate-fleet-d03dacaa... exited with error: signal: killed
2026/08/13 02:51:04 candidate-fleet-e15cb89f... exited with error: signal: killed
2026/08/13 02:51:04 candidate-fleet-d03dacaa... exited with error: signal: killed
2026/08/13 02:54:04 candidate-fleet-e15cb89f... exited with error: signal: killed
2026/08/13 02:54:05 candidate-fleet-d03dacaa... exited with error: signal: killed
2026/08/13 02:57:05 candidate-fleet-d03dacaa... exited with error: signal: killed
2026/08/13 02:57:06 candidate-fleet-e15cb89f... exited with error: signal: killed
```

The retained computer's guest API at `10.200.20.2:8085` timed out. Guest console
output shows the VM reaching systemd and starting `go-choir-run-autoputer-runtime`
(`store: open`, autoputer initialization, and wire publication) before the
firecracker process is killed. No guest panic or self-requested shutdown has
been established.

At the same time, `https://choir.news/health` reported `status=ok` and
`vmctl_status=ok`. That surface proves the vmctl HTTP service responds; it does
not prove that candidate realizations are ready or that the retained computer
is reachable.

## Belief State

Established:

- the kill is external to the guest process (`signal: killed`), not a recorded
  guest kernel panic;
- the cadence is approximately 180 seconds for both candidate realizations;
- the guest boots far enough to start its runtime but does not expose the
  retained computer API before the kill;
- primaries were not upgraded or restarted by this deployment.

Unresolved:

- which host component sends SIGKILL;
- which readiness predicate remains false;
- whether the missing predicate is guest runtime readiness, execution-identity
  renewal, network attachment, route registration, or another lifecycle join;
- whether `633131aa` caused the failure or merely forced reconstruction through
  a previously broken candidate startup path.

## Protected Surfaces

- vmctl lifecycle and realization reconstruction;
- guest readiness and capability/session renewal;
- candidate and primary route projection;
- staging deployment and acceptance identity.

No primary upgrade is authorized by this record. A candidate must survive beyond
the observed kill window and pass product-path acceptance before any primary is
reconstructed onto the new release.

## Admissible Evidence

The repair requires staging evidence, not a local VM approximation:

1. identify the SIGKILL caller and its configured deadline;
2. identify the exact readiness state that failed to advance;
3. reproduce the failure on a candidate realization;
4. deploy the repair;
5. observe a candidate surviving beyond two prior kill windows;
6. prove retained-computer guest API and product-path behavior;
7. only then assess a primary upgrade.

## Rollback

Before a source repair, operational rollback is reconstruction onto the last
known-good deployed release only if the owner chooses restoration over continued
diagnosis. After a source repair, revert its commit and redeploy. Do not mutate
primary data images, route slots, canonical event heads, or guest-local Dolt as
a diagnostic shortcut.

## Heresy Delta

- `discovered`: candidate readiness is not represented by the current proxy
  health result, allowing `status=ok` while both candidate realizations loop;
- `introduced`: none;
- `repaired`: none.

## Next Probe

Trace the host process that enforces the approximately 180-second deadline, then
follow its readiness input back to the guest/runtime transition. Fix the missing
state transition or invalid deadline contract at the substrate; do not lengthen
the timeout without proving boot progress is otherwise correct.
