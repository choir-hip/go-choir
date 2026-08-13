# Candidate Realizations Never Reach Readiness and Are Killed Every 180 Seconds

**Date:** 2026-08-13  
**Status:** root causes isolated; repair pending
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

Established root causes:

- vmctl owns the SIGKILL. `VM_BOOT_READY_TIMEOUT=180s` bounds
  `waitForGuestReady`; after the guest fails its health probe for 180 seconds,
  vmctl force-kills the candidate and the warmth reconciler recreates it. The
  timeout behaves as configured and must not be lengthened.
- retained user computer `computer-03335285269bdba4f94377e56879f9e6`
  reaches the guest runtime, but credential bootstrap stops before the API can
  start. corpusd issues a canonical envelope and matching lifecycle receipt;
  exchange then rejects the receipt because `credentialLifecycleReceipt` uses
  `bootstrapControlKeyResolver`. That resolver intentionally refuses its
  in-memory fallback once a computer event head exists, while
  `control_key_history` contains no row for the persistent platform signer.
  A focused verifier against the staging Dolt row reproduced the exact error:
  `control key resolver: bootstrap key absent after genesis`.
- platform candidate `candidate-fleet-d03dacaa7404b1e4412b2e6f` is a separate
  failure. Its legacy actor log contains unscoped mailbox identities absent
  from its current object graph; migration fails closed on the first orphan
  (`00dba6bf-6c03-4fce-853d-087e3f08a72d`). Its prior primary remains running.
  This failure must not be conflated with the retained user's credential
  bootstrap failure.

Conjecture:

- `633131aa` did not create either substrate defect. The deploy restarted vmctl
  and reconstructed candidates through startup paths that had not been tested
  against an established event head or an orphaned legacy actor log.

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

1. preserve the 180-second readiness bound and repair the credential transition;
2. add a regression with an established computer head and absent control-key-history row;
3. keep event-receipt key-history enforcement unchanged;
4. deploy the repair;
5. observe the retained candidate surviving beyond two prior kill windows;
6. prove retained-computer guest API and product-path behavior;
7. separately disposition the failed platform candidate before assessing any primary upgrade.

## Rollback

Before a source repair, operational rollback is reconstruction onto the last
known-good deployed release only if the owner chooses restoration over continued
diagnosis. After a source repair, revert its commit and redeploy. Do not mutate
primary data images, route slots, canonical event heads, or guest-local Dolt as
a diagnostic shortcut.

## Heresy Delta

- `discovered`: candidate readiness is not represented by the current proxy
  health result; established-computer credential receipts are verified through
  a bootstrap-only resolver with no registered current-key path; a separate
  stale platform candidate cannot migrate orphaned legacy actor mailboxes;
- `introduced`: none;
- `repaired`: none.

## Next Probe

Replace only credential-lifecycle receipt verification with an exact current
platform-signer resolver; leave event-receipt control-key-history enforcement
unchanged. Add the established-head regression, deploy, and prove the retained
candidate survives beyond 360 seconds. Treat the failed platform candidate's
orphan mailbox migration as a distinct recovery decision; do not delete or
replay its backlog as part of the retained-user repair.
