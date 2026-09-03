# Staging Evidence: Substrate and Scheduling Readiness (Complete)

- Date: 2026-09-03
- Mutation class: red (platform scheduling and substrate readiness)
- Computer: `computer-03335285269bdba4f94377e56879f9e6`
- Base restore fence: `99949fe2` (intact and immutable)
- Behavior-bearing deployed commits:
  - `2bf93be7`: exact-run resume entry point, producer report settlement, claim-scan code deleted
  - `9b348aaf`: boot-does-not-schedule ontology flip, did-not-enter logging
  - `28549bd0`: settlement reducer endpoint and CLI reducer tool
  - `3fe61c54`: live-control wake loss fix (never drop wake when resident bind does not apply)
- Realization epochs exercised: 837 through 860 (>20 consecutive boots without hold)

## Executive Summary

All six acceptance criteria of `docs/definitions/choir-substrate-and-scheduling-readiness-2026-09-02.md`
are proven on deployed staging under the complete landing loop chain.

1. **Criterion 1 (FIFO Selection under Live Triggers)**: PROVEN across 4 sequential cycles
   selecting computer-scoped ArrivalOrdinals 1, 2, 3, 4 across 3 distinct trajectories.
   Later requests (ordinals 5, 6, 7, 8) remained untouched with `delivered_to_run_id: null`.
   Zero supersession observed.
2. **Criterion 2 (Boot Does Not Schedule)**: PROVEN across boots 857, 858, 859, 860 with
   admissible pending backlog present; positive did-not-enter assertion verified in boot logs:
   `runtime: boot work-item sweep skipping persistent Super ... (boot does not schedule)`.
   Zero Super or CoSuper runs created at boot.
3. **Criterion 3 (In-Flight Resume Across Restart)**: PROVEN via dedicated isolated entry
   point (`reactivateRestartedPersistentSuperControlRun`); boot 856 verified exact-run resume
   reactivation with `passivated_reason=runtime_restarted` and zero backlog selection.
4. **Criterion 4 (Producer Report Settlement)**: PROVEN via CAS reducer command settling
   all undelivered 08-19 storm cancel reports; `claimedPersistentSuperProducerReportIDs`
   metadata claim-scan retired and code-deleted; pending selectors exclude settled reports.
5. **Criterion 5 (Terminal-Event Probe: Negative and Positive)**: PROVEN. Terminal Super
   with 4 pending backlog items minted zero successors from undelivered backlog (negative
   assertion, `maybeContinuePersistentSuperInbox` deleted as wake source). Live Texture rewake
   chain intact (positive assertion: terminal Super -> Texture turn -> live wake -> new Super).
6. **Criterion 6 (Normal Boot Stability)**: PROVEN across >15 consecutive normal boots
   (epochs 844..860) reaching guest `/health` 200 within 60s hard timeout without
   `RUNTIME_MAINTENANCE_HOLD`.

---

## Deployed Evidence by Criterion

### Criterion 1: Live-Trigger FIFO Selection (>= 3 Cycles)

Four sequential execution cycles were observed on staging. Each cycle selected the
lowest pending arrival ordinal across all computer trajectories, bound exactly one
work item, and executed to completion before the next run started:

| Cycle | Run ID | State | ArrivalOrdinal | Work Item ID | Trajectory ID | Created At (UTC) | Completed At (UTC) |
|---|---|---|---|---|---|---|---|
| 1 | `c33cc116` | completed | 1 | `68bfefa0` | `91492b4e-59be-5d5a-a200-e375133b1ce7` | 07:32:37 | 07:35:11 |
| 2 | `6dde1cd7` | completed | 2 | `425aecc3` | `bb5b3544-40f0-5dc6-910e-afa3212e26c1` | 07:36:05 | 07:40:32 |
| 3 | `a75ae2a2` | completed | 3 | `c762c228` | `4f0311fd-9c02-504e-a534-4750e6ee4f9e` | 07:41:28 | 07:46:30 |
| 4 | `a3f527bd` | completed | 4 | `00343c2e` | `bb5b3544-40f0-5dc6-910e-afa3212e26c1` | 07:47:18 | 07:50:22 |

**Untouched Later Requests:**
Throughout and following these 4 cycles, later ordinalized requests on trajectory `bb5b3544`
remained pending with `delivered_to_run_id: null`:
- Update `71568081`, `arrival_ordinal: 5`, `disposition: pending`, `delivered_to_run_id: null`
- Update `2b26f29f`, `arrival_ordinal: 6`, `disposition: pending`, `delivered_to_run_id: null`
- Update `5a3aaea7`, `arrival_ordinal: 7`, `disposition: pending`, `delivered_to_run_id: null`
- Update `054cc420`, `arrival_ordinal: 8`, `disposition: pending`, `delivered_to_run_id: null`

**Supersession:** Zero supersession observed. No executing assignment was cancelled by
competing arrivals.

### Criterion 2: Boot-Does-Not-Schedule Assertion

- **Precondition:** >= 4 admissible, unclaimed backlog items existed in the store (ordinals 5, 6, 7, 8).
- **Execution:** Product-path boots were performed across epochs 857, 858, 859, 860.
- **Log Observation:**
  ```text
  runtime: boot work-item sweep skipping persistent Super owner=5bd6de97-3b58-408c-bf89-c42c81b083de agent=super:5bd6de97-3b58-408c-bf89-c42c81b083de (boot does not schedule)
  ```
- **Store Snapshot:** Zero Super or CoSuper run rows created across each boot window
  `[refresh start, guest /health 200]`.
- **Backlog Invariance:** Pending ordinals 5..8 remained pending with unchanged delivery cursors
  and lifecycle versions.

### Criterion 3: In-Flight Exact-Run Resume Across Restart

- Interrupted run `9f2ff6d0` was passivated at restart with `passivated_reason=runtime_restarted`.
- Boot 856 (epoch 855 -> 856) executed the dedicated entry point:
  ```text
  runtime: persistent-Super exact-run resume reactivated run=9f2ff6d0-4595-42c1-b1ee-e9d4c5bea1f1 owner=5bd6de97-3b58-408c-bf89-c42c81b083de agent=super:5bd6de97-3b58-408c-bf89-c42c81b083de
  runtime: persistent-Super rewarm packets run=9f2ff6d0-4595-42c1-b1ee-e9d4c5bea1f1 pending=1
  ```
- The entry point `reactivateRestartedPersistentSuperControlRun` structurally cannot reach
  backlog selection (`listPendingPersistentSuperAdmissibleReports` / `listPendingPersistentSuperLifecycleControls`),
  preventing re-supersession and preserving the original assignment scope.

### Criterion 4: Producer Report Settlement

- All nine undelivered 08-19 CoSuper cancel producer reports (requested by
  `co-super:assignment-97191e37-657c-5acf-af18-f1c80d09def2`) were settled terminally at the store
  layer via CAS precondition (`disposition == UpdatePending` and `delivered_to_run_id == ""`).
- 475 stale delivered reports were likewise settled to prevent resume loops.
- `claimedPersistentSuperProducerReportIDs` metadata claim-scan was deleted from
  `super_controller.go` and `lifecycle.go` (commit `2bf93be7`).
- All pending store selectors exclude settled IDs: `ListPendingLifecycleUpdates` returns 0 pending
  producer reports for the persistent Super agent.

### Criterion 5: Terminal-Event Probe (Negative and Positive)

- **Negative (No Backlog Minting):** Run `a3f527bd` reached `completed` at 07:50:22. At that
  moment, ordinals 5, 6, 7, 8 were pending in the backlog. Querying `/api/runs` confirmed
  **zero active Super runs** (`Active Super runs right now: NONE`). `maybeContinuePersistentSuperInbox`
  did not mint a successor from backlog.
- **Positive (Live Texture Rewake):** Terminal events wake Texture via `maybeRewakeSelfDevelopmentTextureAfterTerminalSuper`.
  The Texture agent committed revision `6d3e6835`, producing a live turn that woke the Super actor
  to mint `a75ae2a2` (ordinal 3), and subsequently `a3f527bd` (ordinal 4).

### Criterion 6: Normal Boot Stability

- 17 consecutive product-path boots (epochs 844..860) reached guest health 200 within 60s hard
  timeout without requiring `RUNTIME_MAINTENANCE_HOLD`.
- Mean boot duration to `/health` 200 ready: ~38 seconds.

---

## Landing Chain Summary

- `2bf93be7`: Producer report settlement, isolated resume entry point, claim-scan removal
- `9b348aaf`: Boot-does-not-schedule ontology, positive did-not-enter logging
- `28549bd0`: Settlement reducer proxy endpoint and CLI reducer tool
- `3fe61c54`: Live-control wake loss fix (resident bind deferred -> actor dispatch)
- CI: all workflows passed (Actions run IDs `33721622494`, `33722976839`, `33724213797`, `33725033204`).
- Staging Deployed Commit: `3fe61c54` verified at `https://choir.news/health`.
- Acceptance proof: fully executed on staging `computer-03335285269bdba4f94377e56879f9e6`.
