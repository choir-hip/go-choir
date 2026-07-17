# G4 Root-Cause Cluster — Staged Candidate Lifecycle Is Split Across Routed and Unrouted Authorities

**Observed:** 2026-07-17 on deployed staging Node B at `e6fa53f10db3ba9499175d7a1d7912a0cbe2f876` while proving recovered-owner zero-realization reconstruction.

## Mutation ceremony

- Mutation class: **red**.
- Substrate: vmctl ownership lifecycle and D-ROUTE staged-candidate state machine.
- Protected surfaces: constructed candidate ownership, Firecracker stop/destroy, route authority, legacy detach/restore, rollback.
- Conjecture delta: a candidate can be safely cleaned up before its first route CAS only if the exact unrouted disposal authority owns the necessary stop transition; generic lifecycle actions intentionally require a route and cannot complete that transition.
- Admissible evidence: deployed HTTP refusal plus exact ownership/route state; focused lifecycle tests; matching CI/deploy; deployed active-candidate disposal/reconstruction proof.
- Rollback: revert the disposal-boundary source change before any new disposal. The current owner proof candidate remains immutable-input reconstructable and may safely idle-hibernate under existing policy before old disposal.
- Heresy delta: `discovered: 1`; `introduced: 0`; `repaired: 0` at this checkpoint.

## Three related G4 failures

Within one fleet-cutover preparation cycle:

1. A legacy ownership could not leave its owner/desktop key without deletion, so matching pre-route construction was impossible. The repaired exact signed detach/restore boundary moved legacy state aside without destroying it.
2. A first bootstrap could not roll back to route absence, so exact legacy restore was impossible after a post-bootstrap failure. The repaired signed generation-1 `bootstrap_rollback` transition now returns the route to absence.
3. An **active but unrouted** constructed candidate cannot be stopped or disposed after pre-CAS verification failure. Generic `/internal/vmctl/stop` and `/hibernate` correctly require an immutable route. Exact unrouted disposal correctly requires route absence, but currently refuses an active candidate. There is no authorized transition between those states.

Deployed receipt for case 3:

- candidate `candidate-owner-recovery-g4-20260717` was constructed and independently verified from immutable owner recovery inputs;
- its route `computer:5bd6de97-3b58-408c-bf89-c42c81b083de:owner-recovery-g4-20260717` remained absent, as required before G4;
- `POST /internal/vmctl/stop` returned HTTP 409: immutable ComputerVersion route absent;
- exact `POST /internal/vmctl/computer-version-realizations/dispose-unrouted` returned HTTP 409: `candidate must be non-active`;
- the candidate remains healthy and reconstructable; no route or legacy owner state changed.

## Root cause: substrate, not three endpoint symptoms

The staged candidate state machine crosses an authority seam:

```text
construct candidate (unrouted, active)
  -> independently verify
  -> either route it
     or stop + destroy it before route
```

Generic user lifecycle endpoints are route-governed and should remain so. The dedicated unrouted disposal endpoint is the existing replacement authority for pre-route cleanup, but its contract assumes another endpoint has already stopped the VM. No conforming endpoint can satisfy that assumption. This is one incomplete staged-candidate lifecycle boundary, not a reason to weaken route enforcement on generic stop/hibernate or add another route writer.

The common cause across all three G4 failures is that pre-route, route-CAS, and rollback states were designed as individually safe operations without one fate-sharing transition model for a legacy-to-constructed cutover. The detach and bootstrap-rollback repairs completed the legacy and route halves. Active unrouted disposal is the remaining pre-route hole.

## Required substrate repair

Extend the existing **exact unrouted constructed-candidate disposal** authority; do not add a generic unrouted stop endpoint.

After it has validated route absence, realization identity, owner/desktop binding, committed construction version, disk receipt, and non-published candidate status, the endpoint must:

1. stop/hibernate the exact active Firecracker VM through `VMManager` when necessary;
2. observe the ownership become a permitted non-active state;
3. destroy the exact realization and reclaim its construction disk through the existing disposal path;
4. atomically remove only the matching constructed ownership;
5. emit the existing immutable disposal receipt;
6. remain idempotent for exact replay and refuse stale, routed, wrong-version, wrong-disk, published, legacy, or foreign candidates without stopping them.

Validation must precede the stop side effect. A forged request must not become a capability to stop another VM. Stop failure must preserve the ownership and disk for retry; destroy/reclaim failure must leave diagnosable state and cannot be reported as disposal success.

## G4 consequence

No fleet detach or CAS is authorized until this source repair passes focused tests, CI and matching Node B deploy, and the deployed owner proof candidate is disposed while active through the exact unrouted endpoint, reconstructed from the same immutable refs, independently verified with a new realization/disk receipt, and then safely disposed or retained for frozen G4 review.
