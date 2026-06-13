# Mission M2 — Messaging Cutover (cutover step 3) — v0

Source: `docs/mission-portfolio-2026-06-11.md` §M2. Program:
`docs/choir-rearchitecture-durable-actors-2026-06-11.md` (cutover step 3,
§2.2/§2.4, conjecture R4). Spec: `specs/actor_protocol.tla` (send / activate
/ deliver / steer / passivate; conformance target). Discipline:
`skills/parallax/SKILL.md`. Predecessor: M1
(`docs/mission-trajectory-model-v0.md`) — trajectory records and work items
must exist before slots re-key and results become durable updates.

## Source form (from the portfolio, verbatim intent)

**Real artifact:** `update_coagent` (renamed, promoted `submit_coagent_update`)
as the sole agent-to-agent primitive over the `internal/actor` send path;
deletion of `cast_agent`, `cast_agent_update`, `wait_agent`, `notifyParent`,
per-turn inbox polling; co-super slot registry keyed (trajectory, slot) with
atomic claim.

**Bridge conjecture (R4):** one structured message primitive doubling as the
wake primitive makes results single-sourced and control flow legible.
*Falsifier:* a real coordination need that typed kinds + notes cannot express
(watch the kind distribution for prose-stuffing). *Edge (missing_oracle):*
silent stall — liveness now rests on the open-obligations query; a trajectory
that stops progressing while showing zero open obligations kills the design.

**Settlement:** grep-level zero callers of the deleted mechanisms; a vsuper
coordinating two co-supers sees every result exactly once across a process
restart; prompts updated (co-super.md, vsuper.md, vtext.md).

**Dependencies:** M1. **Size:** 1–2 overnight missions; the slot registry is
the riskiest single migration — its own control interval and test.

## Parallax State

status: open_handoff (post-settlement review on 2026-06-13 falsified the
M2 settlement claim; M1 settled 2026-06-12; portfolio sequencing corrected
to architecture-first)

**kind:** spine.

**mission conjecture:** if `update_coagent` becomes the sole agent-to-agent
message and wake primitive over the `internal/actor` send path, and the four
overlapping mechanisms are deleted with the slot registry re-keyed to
(trajectory, slot), then the deeper rearchitecture goal advances: results
become single-sourced, control flow becomes legible (who updates whom with
what kind), and the lost-wake / completed-but-unknown bug classes become
structurally unrepresentable rather than merely less likely.

**deeper goal (G):** durable actors, evidence-bearing promotion, and
self-development operational instead of documentary (portfolio G).

**witness/spec (A/S):** one runtime-store-backed send/wake path for agent
updates: `update_coagent` (rename + promotion of `submit_coagent_update`,
kind enum grown with `directive` and `assignment`) appends an idempotent
`worker_updates` row plus addressed channel audit message, then wakes the
target actor from that durable backlog. Deletions per the source form; slot
registry keyed (trajectory, slot) with atomic claim and matching-claim release
on post-claim persistence failure. Spec conformance:
`specs/actor_protocol.tla` invariants (no lost wake, idempotent delivery,
atomic passivation). Successor edge: kind-specific ledger writers for
`assignment` -> work item and `verification` -> run-acceptance evidence must
use the same runtime store transaction, per Q1, but were not part of this M2
deletion batch.

**invariants / qualities / domain ramp (I/Q/D):**
- I: exactly-once ledger effects committed transactionally at send;
  at-least-once visibility is accepted slack, never "fixed" with
  coordination machinery. No new message primitives (pub/sub and
  multi-recipient stay deferred; multi-recipient = loop over idempotent
  sends). VText tool-scope enforcement untouched.
- Q: every prompted coordination use expressible as a typed kind + prose
  body; the kind distribution is watched, not assumed (R4 prose-pressure
  edge).
- D: starts in-process (one computer's runtime); cross-VM send (super ↔
  vsuper outbox, actor_protocol_xvm.tla) is explicitly out of scope —
  deferred until the first real cross-VM pair is live. The restart
  falsifier must run on a real process, not only unit tests.

**variant (ranking function) V:** count remaining M2 blockers:
`update_coagent` rename/promotion not done; old coordination tools still
registered/called; per-turn inbox pollers still active; `notifyParent` still
active; slot registry still keyed by parent run; restart exactly-once
falsifier missing; silent-stall oracle missing; prompts/tests still name old
tools; post-review falsifiers reopened after a false settlement claim.
Current V=3:
1. run completion and `worker_updates` delivery marking are separate writes,
   so a terminal run can be observed while its waking update remains pending;
2. the comprehensive persistent-super blocking provider still keys on the
   old "pending inbox deliveries" prompt and the follow-up test times out
   under the new `update_coagent` prompt;
3. `/internal/runtime/channel-casts` still directly writes
   `worker_updates`, bypassing `update_coagent` authority and shape checks,
   so `update_coagent` is not yet the sole wake writer.
Last ΔV=+3: post-settlement review reopened the mission with three concrete
blockers. Q1 remains decided: durable update append and kind-specific ledger
effects belong in the runtime store transaction; the later kind-specific
ledger writers remain a successor edge, not a blocker for this deletion
cutover unless the repair touches their kind semantics.

**budget:** 1-2 overnight missions. Solvency rule: batch unambiguous deletion
work, but stop for a full Parallax pass if the old and new messaging models
would coexist beyond a temporary shim inside the same landing batch.

**authority / bounds:** repo changes on a branch; behavior changes gated on
the full suite + the named falsifiers; prompts (co-super.md, vsuper.md,
vtext.md) updated in the same cut — stale prompts naming deleted tools are
heresy vectors. Landing proof (commit, push, CI, staging acceptance)
required in this document before settlement; this is a
platform-behavior-changing mission, so local proof is not settlement.

### Position — code inventory (re-verified at M2 start)

What this observer can already see cheaply (receipts from grep; re-verify at
mission start, M1 may shift lines):

1. **The four mechanisms to delete:**
   - `cast_agent` (tools_coagent.go:721), `cast_agent_update`
     (tools_coagent.go:792), `wait_agent` (tools_coagent.go:944) — tool
     definitions; registry entries at tools.go:380–382; dispatch at
     tools.go:718 ff.
   - `notifyParent` — 5 non-test sites (runtime.go:1197, 1331, 2342, def
     2454 ff.).
   - per-turn inbox polling — `injectPendingInboxTurns` (runtime.go:1230,
     called at runtime.go:1141) and a second poller in
     super_controller.go:37 (`ListPendingInboxDeliveries`) — **note: the
     super controller is a polling consumer the portfolio entry does not
     name; sweep it in the same cut or the deletion is incomplete.**
2. **The survivor:** `submit_coagent_update` (tools_worker_update.go;
   registry tools.go:395) is already the structured, idempotent append
   path. Q1 names its target form `update_coagent` and keeps it in the
   runtime store transaction. Consumers that key off its old tool name:
   researcher/tool cadence checks, delegate-worker fallback checks,
   vmctl evidence checks, live workflow tests, and prompt defaults — the
   rename must sweep literals and any event payloads carrying the tool
   name.
3. **The slot registry:** today keyed (spawning run, slot) via
   `activeChildRunForCoSuperSlot(parentRec.RunID, slot)` under
   `childSpawnMu` (runtime.go:529–556 region; sequencing logic through
   ~:703). Re-key to (trajectory_id, slot): M1's `runs.trajectory_id`
   column + index make the claim query one indexed lookup. The riskiest
   migration (R5 names co-super slot sequencing the riskiest single
   semantic) — own control interval, own test, atomic claim semantics
   stated explicitly (the check-then-act race at runtime.go:534–545 is one
   of the bug classes this mission exists to delete; do not reproduce it
   keyed differently).
4. **Q1 decision:** the durable update log for M2 lands in the runtime store
   transaction, not a separate actor SQLite file. Receipt: `worker_updates`,
   `channel_messages`, current `inbox_deliveries`, `work_items`, and
   `run_acceptances` are runtime-store tables. `DispatchWorkerUpdate` already
   gives idempotent append plus channel/inbox rows in one transaction. The
   M2 implementation must extend that same transaction for `assignment` ->
   work item and `verification` -> acceptance evidence. The `internal/actor`
   SQLite log remains protocol-core/test scaffolding unless it is backed by
   the same runtime DB; using a separate file would split append from ledger
   effects and falsify exactly-once ledger semantics.
5. **Prompts naming deleted tools:** vsuper.md, vtext.md, co-super.md
   (prompt_defaults). Settlement includes their update.
6. **M1 hooks available:** work items (`assignment` kind writes one),
   `TrajectoryObligations` (the open-obligations query the silent-stall
   edge rests on), `runs.trajectory_id` for the slot re-key.

Blind spots from this position (edge classes named):
- **missing_oracle (R4's named edge):** silent stall. After wait_agent and
  polling die, liveness rests entirely on open obligations being modeled
  honestly. The discriminator to build *early in this mission*: a stall
  probe — a trajectory with in-flight coordination must never show zero
  open obligations; if it can, the obligation model is too loose and the
  design dies here, cheaply.
- **independence:** actor_protocol.tla covers the protocol model;
  `internal/actor` tests cover the package. Neither covers the runtime
  integration (LLM loop steering at step boundaries, warm injection). The
  model's ∀ transfers only via conformance — say so in every claim.
- **resource:** the deletion touches every coordination path at once. If
  the cut cannot stay green mid-mission, shrink D: land send-path
  unification first (all four mechanisms become shims over send()), then
  delete shims one per control interval.

### Initial conjectures

- **C1 (= R4 bridge):** every real coordination use is a typed message.
  *Test:* audit of prompted uses (verifier failure reports, vsuper
  correctives, VText worker instructions — rearchitecture §2.2 already
  audited these as typed-with-prose-bodies); post-cut, the kind
  distribution shows no prose-stuffing into `notes`. *Falsifier:* a
  coordination need that cannot be a kind.
- **C2 (single source):** a vsuper coordinating two co-supers sees every
  result exactly once, in its mailbox, across a process restart (kill -9
  between append and delivery; the sweep delivers on reboot). *This is the
  mission's behavioral falsifier and its settlement gate.*
- **C3 (slot atomicity):** two concurrent spawns claiming
  (trajectory, slot) — exactly one wins, durably, under crash injection
  between claim and spawn. Own control interval.
- **C4 (no silent stall):** with wait_agent deleted, a trajectory with
  undelivered results or unanswered blockers always shows nonzero open
  obligations via `TrajectoryObligations`. *Falsifier (kills the design,
  per R4):* a stalled trajectory reporting zero open obligations.

### Open questions (first probes)

- **Q1 (decided):** transactional domain for the durable update log is the
  runtime store. The implementation obligation is to put update append,
  audit message, wake/backlog record, and kind-specific ledger effects in one
  runtime-store transaction.
- **Q2:** wake-into-runtime integration — `actor.Handler.HandleUpdate` vs
  the existing `executeRun` LLM loop: M2 needs delivery/steering into live
  loops (replacing `injectPendingInboxTurns`), but full activation-loop
  replacement is M3. Probe: define the M2/M3 boundary precisely — M2 may
  deliver into the existing run loop at step boundaries as long as the
  send path is the only source.
- **Q3:** does anything outside the runtime read `inbox_deliveries` or
  `channel_messages` as a delivery mechanism (vs audit log)? Grep before
  deleting the pollers; channel_messages survives as replay-only log.

**next move:** repair the M2 settlement blockers only. First make run
completion and delivery marking atomic enough for the restart exactly-once
claim (no terminal run visible with its `worker_update_ids` still pending,
or an equivalent stronger store primitive). Then repair the comprehensive
persistent-super follow-up test so it blocks on the new
`update_coagent` prompt and proves queued updates drain in a follow-up run.
Then remove or re-route `/internal/runtime/channel-casts` so it cannot be a
second agent-to-agent wake writer that bypasses `update_coagent` authority.
Re-run the focused falsifiers and the relevant comprehensive tests. Do not
detour into Universal Wire or review UI product behavior.

**ledger file:** `docs/mission-messaging-cutover-v0.ledger.md`.

**version / lineage:** v0, compiled 2026-06-12 from portfolio M2 + code
inventory, then reopened after M1 settlement and the portfolio
architecture-first correction. Line numbers may drift; re-verify at start.
Predecessor: M1 (`docs/mission-trajectory-model-v0.md`). Successors gated on
this: M3 (lifecycle), M4 (continuation deletion), M5 (Wire falsifier).

**learning state:** retained here. Inherited from M1's circuit: provenance
vocabulary is `spawned_by` only (no parent/child, even in prose — glossary
"provenance (spawned_by)"); the slot re-key consumes M1's
`runs.trajectory_id` column; the silent-stall oracle consumes M1's
`TrajectoryObligations`.

**settlement:** retracted. Commit
`8052d242afc80320b7cd1b34a2f7a4bb306f1f13` did land and deploy, but the
post-settlement review found the local falsifier evidence was incomplete and
partly stale. New receipts:
- Failed focused restart proof:
  `nix develop -c go test ./internal/runtime -run 'Test(UpdateCoagentPendingUpdateSurvivesRestartAndDeliversOnce|TrajectoryObligationsReportPendingUpdateCoagent|VSuperCoSuperSlotReusedByTrajectorySlot)' -count=1`
  failed because `TestUpdateCoagentPendingUpdateSurvivesRestartAndDeliversOnce`
  observed the super run terminal while the update still had empty
  `DeliveredToRunID` / nil `DeliveredAt`.
- Failed comprehensive prompt/follow-up proof:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'Test(PersistentSuperProcessesConcurrentInboxDeliveriesInFollowupRun|PersistentSuperBlockedRunDoesNotStarveFreshInboxDelivery|InstallDefaultAgentToolsProfiles)' -count=1 -v`
  failed because `TestPersistentSuperProcessesConcurrentInboxDeliveriesInFollowupRun`
  timed out waiting for the first super run. The blocking provider still
  searches for the old "pending inbox deliveries" prompt while the runtime
  now emits "pending update_coagent records".
- Review found `/internal/runtime/channel-casts` registered and live, with
  `HandleInternalChannelCast` directly constructing `WorkerUpdateRecord` and
  calling `DispatchWorkerUpdate`; `tools_vmctl.go` posts to that route.
  That is a second wake-writer path, not the `update_coagent` tool.
- Rollback ref for the original cutover remains
  `d188e88bfc33582bb9479d5d9c0511c599f077de`; current repair should be a
  new documentation-first commit followed by code repair, not a history
  rewrite.
