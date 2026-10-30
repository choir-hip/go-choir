# Delegated-cast producer report poisons persistent-Management delivered-page listing — every subsequent activation dies at ~30s

**Status:** confirmed on staging, reproducible, deterministic. Four failures on disposable computer-0ca7656f: runs 2d296b8f (+34s), 802bf244 (+34s), 117d1a5a (+28s), e9adeb39 (+35s) — all failed with the identical `list pending update_coagent turns: lifecycle invalid transition` signature, each re-minting within the same second. **The burn cycle then self-terminated at 23:10:10Z** — no new binds for >14 min (final count 23:24Z) while a separate texture deferral storm persisted (66 `dispatcher: deferred` events this boot, texture:3be842e0/0cdfb20e deferrals=24, discarding update 70edbe83 each round — a distinct defect class: texture activations returning without disposing the exact trigger). Termination is consistent with the delivered report discharging the work item's pending obligations — the poisoned listing remains, but the re-mint trigger stopped. The defect's blast radius is therefore "every *subsequent* activation on a computer that carries a delegated producer report dies at ~30s", not an unbounded CPU burn.

**Found:** 2026-10-06, during SMG close probe legs on a fresh disposable (computer-0ca7656fc6bd61241bef25d27c7332cd, marker smg-dispo4/dispo5, build 475902d7 — post AppendEvent-scan fix, post mint-slot fix).

## Symptom

Every persistent-Management activation on the computer fails at `tool loop inject turns after tools: list pending update_coagent turns: lifecycle invalid transition`, roughly 28–34s after `persistent Management live occurrence bound run=<id>`. Recovery (`reconcilePersistentManagementActorLocked`) re-mints a run immediately (`rewarm delivered-pending-runs=N`), the new run binds, executes ~2 tool-loop iterations, and fails identically — an indefinite burn loop with no convergence. No `slot_occupied`, panic, or OOM. Console evidence: `docs/evidence/smg-rlm-acceptance-disposable-2026-10-06.json` companion console log (vm-5aa981f84ca6fd9ef3461e186477ffd2, runtime PID 3228).

## Root cause (source-traced, hypothesis pending ledger confirmation)

`ListLifecycleControlsDeliveredToRunPage` (`internal/store/lifecycle_control_delivery.go:738-805`) validates the complete exact-run delivered set before returning a page — it throws on the first packet that fails its predicates rather than skipping it. The ProducerReport arm at :779-796 requires, among others:

- `producerRun.TrajectoryID != trajectoryID` fails when the producer is a **delegated-cast** engineering run, and
- `producerWork.TrajectoryID != trajectoryID` at :794 likewise — delegated assignments mint their work item (`work:<assignmentID>`) and run under the parent texture trajectory? — the trace shows `from=engineering:delegated-sha256:<digest>` agents reporting while the listing requires trajectory equality.

Any of the ProducerReport checks at :780-794 failing makes the whole page listing return `ErrLifecycleInvalidTransition`. `pendingCoagentUpdatesForRun` calls this listing inside the tool loop's inject-turns path; one poisoned packet therefore kills every subsequent activation, since the packet is durable (`update_delivered` is durable evidence; a packet that fails validation remains delivered and continues to poison the listing).

**Mutation class of this record:** green. The fix is red (persistent-Management lifecycle binding path — a `protected_surfaces` member on the SMG station).
## Confirmed mechanism (fix landed this session)

Root cause confirmed by a failing-then-passing store regression test
(`TestDelegatedCastReportDoesNotPoisonConsumerDeliveredListing`): the
delivered-page ProducerReport arm calls
`persistentManagementControlBinding(run.Metadata, trajectoryID,
targetWorkID, update.ControlBindingID)` on the **consuming** run's
`lifecycle_control_bindings`, whose `update_id` entries are lifecycle
control update ids (`mgmt-control-…`). A delegated cast's report instead
carries `ControlBindingID = assignment.Binding.ParentControlID` = the
cast's **commitment record** id — a different ID space that can never
match. The listing threw on the whole page, killing every activation.

Fix in `internal/store/lifecycle_control_delivery.go`: the ProducerReport
arm fetches the producer run + work item first (predicates unchanged),
then accepts either the consumer-side control binding (owner casts) OR
the delegated work-item lineage join — `producerWork.Details
parent_control_id == update.ControlBindingID && parent_work_item_id ==
targetWorkID && parent_loop_id == run.RunID`. The join mirrors
`requireEngineeringDelegatedParentAuthority`'s own authority checks.

Residual: the listing still throws whole-page on a genuinely corrupt or
unauthenticated delivered packet (a packet that is durable-but-invalid
stays poisonous). Quarantine-vs-throw is a separate slice-1 decision;
this fix removes the authentic-but-misparsed class.
## What is proven vs hypothesized

Proven:
- Three deterministic failures, same signature, same ~30s post-bind timing.
- A `co_super` producer report from a `delegated-sha256` engineering cast was the last durable packet before each failure.
- The producer-report arm's predicates can return `ErrLifecycleInvalidTransition`; the failing runs all died in the listing call, not in bind.
- SMG legs 1-3 still landed durably on dispo4: `co_super_assignment_opened`, `co_super_capsule_disposition_set`, `co_super_assignment_cancelled`, `update_delivered result:sha256:b137…` bound to `mgmt-work-smg-rlm-open-smg-dispo4-1791327783` — evidence that the product path works between run deaths.

Hypothesized:
- Which exact predicate fails (producerRun.TrajectoryID vs producerWork.TrajectoryID vs producerWork.AssignedAgentID) needs the packet/run records from the disposable's store; the console does not print the failing predicate. Repro in a focused store test is the cheapest confirmation.

## Fix shape (for the receiving station — SA slice 1+)

The ProducerReport validation arm must accommodate delegated-cast producers: either accept the delegated trajectory/work lineage explicitly (validate against `requireEngineeringDelegatedParentAuthority`-equivalent joins) or scope trajectory-equality to the producer's own declared trajectory rather than the consuming run's. The listing must not kill the consuming run over a packet it cannot consume — consider quarantining invalid delivered packets into evidence instead of returning `ErrLifecycleInvalidTransition` (a delivered-poison packet currently bricks the management desk forever).

## Rollback

None — record only. The disposable burn loop is bounded by VM reclaim; the owner computer is unaffected (its storm predates this defect).

## Regression gate

A disposable computer that receives one delegated-cast producer report must not fail subsequent persistent-Management activations. This is a candidate regression assertion for SA's storm-convergence suite alongside `sa-management-mint-no-start-slot-deadlock`.
