# Staging Evidence: Substrate Boot Probes and Storm-Stop Re-Confirmation (Partial)

- Date: 2026-09-03
- Mutation class: red observations, no code mutation
- Computer: `computer-03335285269bdba4f94377e56879f9e6`
- Base restore fence: `99949fe2` (intact and immutable)
- Guest commit during probes: `7bd488cda31304f1c89feb97f8c54748ef4b8369` (docs-only commit; behavior-bearing deploy remains `42d47604`, landed 2026-08-28)
- Realization epochs exercised: 837 -> 838 -> 839 -> 840 -> 841 -> 842 -> 843 -> 844

## Scope Correction

An earlier draft of this document claimed all four acceptance criteria of
Definition 1 were proven. Owner review (2026-09-03) rejected that claim:

- **Not proven**: FIFO selection under competing requests — no Super run was
  created during the window; both queued requests stayed `pending` with
  `delivered_to_run_id: null`. Zero supersession was vacuous (nothing ran).
- **Not executed**: restart-durability contract requires an in-flight request;
  neither request was ever selected or in-flight.
- **Not proven**: stale-duplicate settlement — only a code citation
  (`internal/store/lifecycle.go:1357-1375`, verified accurate); no settlement
  event was observed, and the 08-19 producer-report settlement deliverable was
  not addressed.
- **Landing receipts incoherent**: `ci_ref` was recorded `not_applicable`
  against a `landing.required: true` red-class definition, and the recorded
  `deploy_ref` was a docs-only commit that cannot have run the deploy pipeline.

This document now records only what was actually observed. It is partial
evidence toward Definition 1, not a completion proof. The completion status
flip in the definition and registries was reverted.

## Observation 1: 5x Consecutive Product-Path Normal Boot Probe (proven)

Each boot was executed through the general product path (`choir computer refresh`)
advancing the realization epoch, querying `vmctl` on Node B for the new IP, and
polling guest `/health` with a hard timeout.

```json
[
  {"boot_probe": 1, "idempotency_key": "effects-boot-probe-def1-1788395533-run1", "refresh_elapsed_sec": 12.91, "guest_health_poll_sec": 0.39, "prior_epoch": 838, "resulting_epoch": 839, "hold_status": null, "guest_status": "ready", "guest_commit": "7bd488cda31304f1c89feb97f8c54748ef4b8369"},
  {"boot_probe": 2, "idempotency_key": "effects-boot-probe-def1-1788395549-run2", "refresh_elapsed_sec": 12.17, "guest_health_poll_sec": 0.39, "prior_epoch": 839, "resulting_epoch": 840, "hold_status": null, "guest_status": "ready", "guest_commit": "7bd488cda31304f1c89feb97f8c54748ef4b8369"},
  {"boot_probe": 3, "idempotency_key": "effects-boot-probe-def1-1788395564-run3", "refresh_elapsed_sec": 12.67, "guest_health_poll_sec": 0.38, "prior_epoch": 840, "resulting_epoch": 841, "hold_status": null, "guest_status": "ready", "guest_commit": "7bd488cda31304f1c89feb97f8c54748ef4b8369"},
  {"boot_probe": 4, "idempotency_key": "effects-boot-probe-def1-1788395580-run4", "refresh_elapsed_sec": 12.25, "guest_health_poll_sec": 0.35, "prior_epoch": 841, "resulting_epoch": 842, "hold_status": null, "guest_status": "ready", "guest_commit": "7bd488cda31304f1c89feb97f8c54748ef4b8369"},
  {"boot_probe": 5, "idempotency_key": "effects-boot-probe-def1-1788395595-run5", "refresh_elapsed_sec": 12.4, "guest_health_poll_sec": 0.56, "prior_epoch": 842, "resulting_epoch": 843, "hold_status": null, "guest_status": "ready", "guest_commit": "7bd488cda31304f1c89feb97f8c54748ef4b8369"}
]
```

**Verdict**: 5/5 normal boots succeeded without `RUNTIME_MAINTENANCE_HOLD`.
Residual database scans did not hang normal boot. This criterion stands.

## Observation 2: Storm-Stop Held Across 5 Boots + 1 Restart (bonus, re-confirms `3654d925`/`5e01ac3a`)

Across all five boots and the epoch 843 -> 844 restart, **zero new Super runs
were created** (live `/api/runs` query, 2026-09-03 ~01:00Z: newest Super-adjacent
runs remain the 2026-08-28 replacement-continuation series, all `passivated`).
The storm-stop claim machinery from `3654d925` (producer-report claims) and the
replacement-continuation prompt from `5e01ac3a` did not restorm during repeated
boots. This re-confirms the 2026-08-19/20 storm-stop evidence on the current
guest; it does not by itself certify the FIFO scheduling contract.

## Observation 3: Arrival Ordinals Allocated and Durable (partial)

Two competing Texture execution requests were created via the product API:

- Doc A (`e4d4ab5c-bf89-571c-a329-c7448c12962e`, trajectory `bb5b3544`):
  update `78d6510b`, `arrival_ordinal: 2`, work item `425aecc3`
- Doc B (`3b12d89a-125f-5067-b882-7cecce50bb81`, trajectory `4f0311fd`):
  update `1aa8a7b4`, `arrival_ordinal: 3`, work item `c762c228`

Both updates persisted across the epoch 844 restart with ordinals intact and
`disposition: pending`. This proves ordinal allocation and durability only.
Selection, execution, and FIFO ordering were never observed because no Super
ever woke. Note: after the probe window, the two documents' own Texture agents
ran revise-event passes (00:38Z, 00:44Z) that were still active when the
completion claim was made at 00:55Z; their downstream effects were not settled
inside the observation window.

## Deterministic Test Receipts (re-verified 2026-09-03)

- `internal/store`: `TestListPendingLifecycleUpdatesArrivalOrdinalSort`,
  `TestApplyTextureTurnAssignsComputerScopedArrivalOrdinalsToSuperExecutionRequests`,
  `TestArrivalOrdinalIsComputerScoped`,
  `TestArrivalOrdinalAllocationConflictsInsteadOfReusing` — PASS (`ok internal/store 7.4s`)
- `go run ./cmd/doccheck -mode live` — PASS

## Residual Work Toward Definition 1

Per the revised Definition 1 (2026-09-03, boot-is-recovery ontology):

1. Live-trigger FIFO selection with exactly one executing assignment.
2. Boot-does-not-schedule assertion (no Super minted from backlog at boot).
3. In-flight resume across restart preserving selection.
4. Producer-report settlement: tombstone the nine 08-19 cancel reports as late
   evidence with explicit settlement receipts.
