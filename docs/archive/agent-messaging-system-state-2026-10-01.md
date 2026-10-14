# Agent-to-Agent Messaging: Current System-State Map (2026-10-01)

Survey of every inter-agent messaging mechanism — raw channel mail, lifecycle
worker-update packets, semantic commitment acts, deferred/deadline wakes, and
immediate emissions — as implemented today. Basis for the convergent consensus
pass on design normalization. All citations verified against the worktree.

## 1. The four durable delivery layers

1. **Channel log** (`choir.channel_message` object rows + `ChannelRead`).
   Audit/mail envelope; addressed casts emit `EventChannelMessage` and request a
   `channel_message` actor wake. NOT a cold-start mechanism and not a lifecycle
   packet (`internal/agentcore/channel_store.go:29-110`,
   `internal/actorruntime/handler.go:297-351`).
2. **Lifecycle worker-update packets** (`choir.worker_update`, direction
   `control` | `producer_report`, `choir.lifecycle_event`,
   `choir.lifecycle_command`). Canonical delivery into Texture/Research/
   Management. `QueueLifecycleUpdate` validates scope/digests/authority and
   commits the packet + outbox atomically (`internal/store/lifecycle.go:3224-
   3680`, `:2248-2335`).
3. **Actor-wake outbox** (`choir.actor_wake_outbox`, projected to the SQLite
   `actor_updates` tape by the outbox projector). Crash-gap repair for every
   committed lifecycle transition (`internal/store/lifecycle.go:485-863`;
   `internal/agentcore/runtime.go:2514-2568`).
4. **SQLite actor tape** (`actor_updates` + dispatcher →
   `actorruntime/handler.go` update-kind dispatch → run execution).
   (`internal/actor/log_sqlite.go`, `internal/actor/dispatcher.go`,
   `internal/actorruntime/handler.go:82-116`.)

Adjacent but distinct: **commitment records** (`choir.commitment_record`,
append-only; observed via `choir.Pack` / Texture evidence materialization, no
wake — `internal/store/commitment.go`, `internal/types/commitment_views.go:449-495`)
and **emissions** (`choir.Emit`, immediate envelope on sender's channel +
wake; recipient drains via `choir.Emits()` — `internal/agentcore/tools_desk.go:
479-513`).

## 2. The verb surface (what desks can stage)

`ChoirExports` module sets (`internal/yaegikernel/choir.go:176-198`). Staged
verbs land in the cell tray; the reducer commits them post-cell.
`Assign`/`Emit` are immediate, not tray intents.

| Verb → intent kind | Staged fields | Desks exporting |
|---|---|---|
| `Message` → `message` | ToDesk, MsgKind, Body | mgmt, eng, research, texture |
| `Outcome` → `outcome` | ToDesk (= activation ID), Body | mgmt, eng, research, texture |
| `Spawn` → `spawn` | Role, Objective | mgmt, eng |
| `Complete` → `complete` | Result, Verdict, Summary, refs | eng |
| `Freeze` → `freeze` | BuildRecipeRef, TestReceipts, toolchain | eng (selfdev op) |
| `Verify` → `verify` | Decision, VerifierRefs, BundleDigest | eng verifier slot only |
| `ApplyTexture` → `texture_apply` | Body (edit/decide/email JSON) | texture |
| `Cast` → `cast` | ToDesk, Objective, Statement | mgmt, eng, research |
| `Ask` → `ask` | ToDesk, Question | mgmt, eng, research, texture |
| `Note` → `note` | ToDesk, Body | mgmt, eng, research, texture |
| `Reply` → `reply` | ToDesk, TargetRef, Answer | mgmt, eng, research, texture |
| `CancelAct` → `cancel` | TargetRef | mgmt, eng, research, texture |
| `Escalate`/`EscalateActions` → `escalate` | ToDesk, Body, Actions? | mgmt, eng, research, texture |
| `Precommit` → `precommit` | Precommit JSON | mgmt, eng, research, texture |
| `Report`/`ReportPacket` → `report` | ToDesk, Claim|Packet, refs, ResolverID | mgmt, eng, research, texture |
| `Resolve` → `resolve` | TargetRef, Resolve JSON | mgmt, eng, research, texture |
| `Disagreement` → `disagreement` | Disagreement JSON | mgmt, eng, research, texture |

Processor/reconciler/conductor/email export **no staged semantic verbs**;
they use the legacy JSON-tool surface (`spawn_agent`, `update_coagent`,
`report_to_texture` for persistent management only —
`internal/agentcore/tool_profiles.go:349-380`).

## 3. Reduce-path matrix

`commitTray` dispatch (`internal/agentcore/rlm_reduce.go:902-969`):

| Intent | Condition | Path | Durable write | Wake |
|---|---|---|---|---|
| `message` | `isAssignedDesk()` (= run metadata `assignment_id` non-empty) | `commitMessageIntent` → authority resolve → lifecycle `QueueLifecycleUpdate` **or** pre-cutover `DispatchWorkerUpdate` | worker_update packet + event + receipt, or channel+worker-update rows | `wakeUpdatedCoagent` (new pending only) |
| `message` | not assigned | `castStagedIntent` | channel envelope row + event | `channel_message` wake |
| `outcome` | always | `castStagedIntent` (no authority check) | channel envelope | `channel_message` wake |
| `spawn` | `spawnRoleAllowed` at validation | `castStagedIntent` → `spawn_request` to ReturnTo | channel envelope | `channel_message` wake |
| `complete` | eng assignment bound | `commitCompleteIntent` (fate saga) then envelope | assignment report/fate rows | `wakeUpdatedCoagent` + channel wake |
| `freeze` / `verify` | eng/verifier | `commitFreezeIntent` / `commitVerifyIntent` | bundle/verification event rows | none |
| `texture_apply` | texture | `commitTextureAuthorIntent` → `ApplyTextureTurn` | revision/work/control/event rows | `wakeTextureControl` per new control |
| `cast` | semantic | commitment record → `openDelegatedCastAssignment` → envelope | commitment + assignment open + envelope | spawn-deadline outbox + channel wake |
| `ask`/`note`/`reply`/`escalate`/`report` | semantic, addressed | commitment record + `castStagedIntent` | commitment + envelope | `channel_message` wake |
| `precommit`/`resolve`/`disagreement`/`cancel` | semantic, ledger-only | commitment record only | commitment | none |

Critical predicate gap: `isAssignedDesk` tests only
`run.Metadata.assignment_id` (`rlm_reduce.go:1239-1243`). Lifecycle research
runs carry `work_item_ids` + `lifecycle_control_bindings` but no
`assignment_id` (observed on staging run 20cd8749, `/api/runs`), so every
verb from a lifecycle research run — including `ReportPacket` to texture —
falls to `castStagedIntent`: commitment record + channel row whose
`channel_message` wake no-ops on texture targets (`handler.go:306-308`).

## 4. Wake/consumer behavior per transport

- `channel_message`: reactivates an existing non-texture parked/active run via
  `ReactivateRunCanonical`; **never mints a run**; **returns unchanged for
  `texture:*` targets** (intentional — texture consumes canonical occurrences
  only). Research/processor/reconciler targets DO reactivate.
- `coagent_result`: canonical lifecycle occurrence. For `texture:*`: strict
  occurrence/authority validation → `ReconcileActorOccurrenceWake` → exact run.
  For lifecycle research: recovery admission or parked reactivation. For
  persistent management: binds resident or holds pending (never mints a
  report-only run). Generic: `ReconcileCoagentWake` may mint.
- `lifecycle_work_assigned` / `lifecycle_cancellation` / `owner_revision` /
  deadline family (`delegated_assignment_spawn_deadline`,
  `assigned_engineering_fate_deadline`, `engineering_progress_overdue_deadline`,
  `activation_budget_deadline`, `cell_terminal_deadline`,
  `fresh_mint_management_resume_deadline`, `reactivated_management_resume_deadline`,
  `wire_reconciler_publish_deadline`, `selfdev_materialization_retry`):
  each a dedicated handler with exact-content validation.
- Unknown kinds: logged + marked processed — a silent durable sink
  (`handler.go:112-116`).
- Binding ≠ consumption: `DeliveredToRunID` claims a packet; only a canonical
  texture turn or management report commit disposes it.
  `ListAllPendingLifecycleUpdates` returns only **unbound** pending rows;
  bound-but-unconsumed packets need `pendingCoagentUpdatesForRun` /
  `ListActionablePendingLifecycleUpdates`.

## 5. Authority gates

- `spawnRoleAllowed` (validation only): mgmt→any, eng→eng/research, research→
  research. (`rlm_reduce.go:96-142`)
- `resolveCoagentUpdateAuthorityWithStore`: assigned Message only; exact
  caller/target profiles, `CanMessage`, co-super slots; selects lifecycle vs
  pre-cutover (`tools_worker_update.go:327-409`).
- `validateLifecycleCoagentUpdateAuthority`: producer ∈ {research, processor,
  reconciler}; target texture|management; exact trajectory/document/requester/
  work binding; requester read from **caller run metadata** `requested_by_*`
  (`tools_worker_update.go:484-578`).
- `QueueLifecycleUpdate` store-level exact target checks
  (`store/lifecycle_texture_target.go`).
- Delegated cast: `openDelegatedCastAssignment` authority (`rlm_reduce.go:
  1000-1039`). Cast staging itself applies **no** role matrix — research can
  stage Cast to engineering; a normalization question.
- Complete/Freeze/Verify: dedicated assignment/operation/verifier-slot
  authorities, unrelated to messaging authority.

## 6. Prompt contract vs implementation (mismatch list)

1. **Texture→research follow-up**: `texture.yaml:75-77` and
   `run_system.yaml:19` teach direct `Message`/`Note`; `revision_policy.yaml:59`
   says they're "rejected". Reality: channel mail to a research desk DOES
   reactivate a parked run — it works as an unauthenticated wake but is not the
   lifecycle control path and cannot cold-start. Both prompt statements are
   wrong in opposite directions; ApplyTexture `target_work_item_id` controls
   are the authenticated path.
2. **Research→texture report**: `rlm_research_runtime.yaml` instructs
   Report/ReportPacket; those take the ledger+channel path — durable row, no
   texture wake. The lifecycle queue path is reachable only via assigned-Message
   (`assignment_id`), which lifecycle research runs lack. User-observed bug:
   research findings never reach texture.
3. **Management return**: `report_to_texture` tool is the real lifecycle
   return; semantic `Report` to texture same dead channel. Overlay names this
   correctly but also says "Engineering's Report resolves the cast" — actual
   assignment terminality is `Complete`, not semantic `Report`.
4. **`EscalateActions` payload loss**: `Actions` validated but carried in
   neither `rlmEnvelope` nor the commitment record — action details evaporate
   unless duplicated into `Body` (`rlm_reduce.go:1041-1057`, `:357-380`).
5. **Ledger-only verbs are not delivery**: `Resolve`/`Disagreement`/`Cancel`/
   `Precommit` never wake; any overlay implying notification is wrong.
6. **`Outcome`**: never authority-checked, dead for texture targets, does not
   settle work; exported to all four desks but unspecified for texture.
7. **Legacy `ReduceCellIntents` path**: casts envelopes for everything; ledger-
   only kinds return `(0,nil)` — silently no commitment record; freeze/verify/
   texture_apply error. Dead code or landmine (`rlm_reduce.go:343-370`).
8. **Cast role gate absent**: `spawnRoleAllowed` covers `Spawn` only; Research
   exports Cast and its overlay names it (`rlm_research_runtime.yaml:18-20`).

## 7. Known-good paths (for reference)

- Texture→research open/continue: `ApplyTexture` controls
  (`target_work_item_id` / `open_researcher` / `open_persistent_super`) →
  `ApplyTextureTurn` → control packet + `wakeTextureControl` + outbox.
- Eng→mgmt assigned report: `Message` with packet body → `commitMessageIntent`
  → lifecycle or survivor `DispatchWorkerUpdate`.
- Mgmt→texture: `report_to_texture` tool (typed lifecycle producer report).
- Processor/reconciler→texture: legacy `update_coagent` (same authority +
  queue machinery).
- Mgmt→eng admission: `Cast` → delegated assignment + spawn-deadline saga.

## 8. Normalization questions for the panel

Q1. **Producer-report carrier**: should lifecycle desk cells (research,
    processor, reconciler profiles — runs with `work_item_ids` or lifecycle
    authority but no `assignment_id`) route `Message`/`ReportPacket` addressed
    to `texture:*`/`management:*` through `QueueLifecycleUpdate`, using
    `deriveLifecycleProducerUpdateID` + `requested_by_*` provenance? Or should
    the predicate stay `assignment_id`-only and research runs get stamped
    `assignment_id`/`requested_by_*` at bind (from `work.Details`, where it
    already lives)?
Q2. **Channel mail to `texture:*`**: keep the intentional no-op (audit-only)
    or hard-reject at reduce time so authors get feedback instead of a durable
    dead letter?
Q3. **Texture→research direct mail**: is unauthenticated channel mail
    reactivation of parked research runs an intended affordance (then prompts
    should describe it honestly) or should it be rejected in favor of
    ApplyTexture controls (then the no-op should extend to producer targets)?
Q4. **Ledger-only acts**: is "no wake" right for Resolve/Disagreement/
    CancelAct/Precommit (observation via next Pack), or should addressee-resolver
    get a wake?
Q5. **EscalateActions**: carry `Actions` in envelope+record, or drop the verb
    variant?
Q6. **`Outcome`**: restrict to activation-target notices only, or remove?
Q7. **Legacy `ReduceCellIntents` envelope-only path**: delete, or fix to mint
    commitment records for semantic kinds?
Q8. **Cast role gating**: apply a cast-role matrix (who may open delegated
    engineering assignments) or document research Cast as intended?
Q9. **`Spawn`→channel-envelope**: is `spawn_request` mail to ReturnTo still
    load-bearing, or should Spawn stage a delegated cast-style admission?
Q10. **Unknown update kinds**: keep log-and-consume, or defer for
    operator-visible failure?

## 9. Adjudicated verdicts (convergent panel 2026-10-01)

Panel: `messaging-normalize-20261001`, quorum 5/13 (codex, claude, gpt6-sol,
glm53-flash, gemini38 ok; rest killed by job deadline or missing CLIs).

| Q | Verdict | Split |
|---|---|---|
| Q1 carrier | **(a) widen dispatch**: lifecycle producers (work-item-bound, no assignment_id) route Message/ReportPacket to texture/management through QueueLifecycleUpdate; cover `IntentReport` not just `IntentMessage`; stamp `requested_by_*` at bind via existing `inheritRequesterMetadataFromWorkItem` (runtime.go:1382 — used by trajectory-sweep spawn at :2827, just not the control-activation path). (b) rejected: `assignment_id` arms eng-fate machinery. (c) rejected: unauthenticated mail must not mint canonical occurrences. **Caveat (codex+gemini, verified)**: lifecycle validator requires texture target + texture requester (tools_worker_update.go:493-495, 529-534) — management-addressed reports need an explicit contract or stay rejected. | 5/5 |
| Q2 mail→texture | **hard-reject at reduce time** — durable dead letter hides the bug class | 5/5 |
| Q3 texture→research | **reject texture→desk channel mail**; ApplyTexture controls only; fix both prompt files (contradictory today) | 4/5 |
| Q4 ledger-only | **correct as-is** — no wake; observation via Pack is the contract | 5/5 |
| Q5 EscalateActions | **drop the variant** — payload already evaporates; its consumer (`update_coagent` execution_request) is retired; carrying Actions into envelopes implies privileged execution without authority. Minority (gemini38, codex) wanted Actions carried — overruled: dead letter either way. | 3/5 |
| Q6 Outcome | **restrict to activation-target notices only** (its designed use: `s.activationID`); narrow module sets | 5/5 |
| Q7 legacy reducer | **delete `ReduceCellIntents`** — zero non-test callers (glm verified); port cursor/fate tests to `commitTray` | 5/5 |
| Q8 Cast gating | **add cast-role matrix** (reuse `spawnRoleAllowed` shape); research→eng Cast currently passes staging then fails admission after the record mints | 5/5 |
| Q9 Spawn | **replace with durable admission** — `spawn_request` envelope has no consumer (gemini38 verified: zero consumers); align with delegated-cast saga | 4/5 |
| Q10 unknown kinds | **defer/poison** — quarantined poison row, not silent consume, not error-loop wedge | 5/5 |

**META consensus** — one delivery authority classification per run, computed
at bind (`assigned_engineering` | `lifecycle_producer` | `texture_author` |
`unbound`), replacing `isAssignedDesk` scattering; lifecycle packets are the
only delivery transport, channel log demoted to audit+human observability;
do NOT collapse packets into envelopes (envelopes lack work/requester/target
checks). Migration cost medium-high; glm53's phasing: delete dead reducer +
Spawn→admission first, dispatch widening second, retire `channel_message`
wake as a red-class cutover last.

**Panel-flagged risks beyond the questions** (verified plausible):
1. Commitment record mints *before* delivery; a failed cast admission leaves
   a record claiming a cast that never opened (rlm_reduce.go:983-1036).
2. Bound-but-unconsumed packets vanish from `ListAllPendingLifecycleUpdates`
   (unbound-only selector) — widened Q1 traffic grows this leak; recovery
   must use actionable views.
3. `choir.Emit` is an immediate side-effect during cell eval — escapes the
   tray commit; failed cells can emit, replays double-emit (gemini38).
4. `assignment_id` is overloaded: messaging authority + assignment-fate
   arming share one key (why stamping it wider is unsafe).

## 10. Owner-directed normalization (2026-10-01)

Owner rulings that supersede/refine the panel map:

1. **Record-native messaging.** Raw messaging as an authored verb class goes
   away. Every inter-agent act is a commitment-ledger record first; delivery
   is a *derived projection* — the reducer mints the lifecycle packet from the
   record. Agents never author envelopes or packets directly.
   - Texture follow-up question to research = `precommit` record
     ("this question will yield material evidence for the revision"), work
     item as resolver → derives a `control` packet.
   - Research answer = `report` record → resolves the ask's precommit
     (arrival = mechanical resolution; materiality = scorer verdict lane,
     `disagreement`/scored review; management remains the scorer of record).
   - "Evidence never arrived" = the precommit stays open, ages on the
     materiality projection, texture escalates — replaces silent hollow-
     revision churn with a visible unresolved commitment.
   - Instructions (cast/cancel/controls) are operational records, not
     predictions — the protocol is commitment-records, not literally all
     precommits.
2. **`ApplyTexture` edits the document only.** The `controls` array inside
   the edit JSON — which makes a revision turn also the inter-agent control
   channel — is removed. Control issuance becomes a sibling lifecycle
   command (working name `IssueLifecycleControl`) carrying the same control
   packet schema + authority checks (`store/lifecycle_texture_target.go`),
   invoked by the reducer when a texture-authored record has a delivery arm.
   `QueueLifecycleUpdate` today accepts `producer_report` direction only;
   the control direction currently exists solely inside `ApplyTextureTurn`
   — extracting it is mechanical, not architectural.
   - Atomicity tradeoff to carry into the goal file: today revision +
     control commit in one transaction; split means a control can reference
     a not-yet-landed revision or vice versa. Mitigation: reducer orders
     record+packet commit inside one `commitLifecycleTransition`-class
     store batch; a "this turn must ship a revision" stays an *authoring*
     gate (the texture run decides), not a transport coupling.
   - Prompt contracts follow: texture verbs become `ApplyTexture` (revise),
     `Ask`/`Note`/`Escalate`/`Precommit`/`Report`/`Resolve` (records —
     delivery derived), `Decide`. `Message`/`Note`-as-raw-envelope verbs and
     the controls-in-edit surface both retire.
3. **Sequencing.** Narrow fix first (Q1 dispatch widening +
   `requested_by_*` bind stamping via `inheritRequesterMetadataFromWorkItem`)
   to un-break research→texture on current plumbing; record-native surface
   is a separate orange→red station touching all four desk contracts.

## 11. Record-native messaging (owner-directed, 2026-10-01)

**Axiom:** the commitment record IS the act. `Addressee` is the delivery
instruction. The reducer mints the record AND its derived lifecycle packet
in one commit; delivery divergence is impossible by construction. Addressee
empty → ledger-only observation, no wake — correct, not a gap.

The issuer is "taking a consequential action whose consequences include
catalyzing downstream work": an addressed `precommit` carries stakes ("this
ask will yield material evidence") AND wakes the addressed work item's desk
via its derived control packet.

**Consequence: waking is the record's liability.** A precommit that wakes a
worker spends compute on a promise. Resolved `contradicted`/low-materiality
asks charge the ISSUER's ledger; unanswered material asks charge the TARGET.
Symmetrical scoring makes follow-up quality an improvable behavior, not
free chatter. Precommit scores feed the issuer's own cell frame and RLM
variable state (`choir.Pack` / prompt overlays carry asks-made vs
asks-material vs asks-contradicted) — the iterative-improvement loop for
texture follow-up behavior, per owner.

### Verb → record mapping (candidate, before panel)

| Legacy verb | Record-native form |
|---|---|
| texture→research follow-up (was ApplyTexture control) | `precommit` + derived `control` packet |
| research→texture report (was ReportPacket envelope) | `report` + derived `producer_report` packet → mechanical `resolve` of the ask on arrival |
| peer FYI | `note` record + derived packet — OR folds into generic operational record |
| answer to an ask | candidate collapse: `reply` ≡ `resolve` on the ask's precommit |
| escalation to management | `escalate` operational record + derived packet — OR folds into operational kind with severity |
| self-directed prediction | `precommit`, Addressee empty → ledger-only |
| scorer contradicts resolver | `disagreement` — candidate collapse: `resolve` authored by scorer? |
| cast/cancel | operational records (not predictions) |
| `Message`/`Outcome`/`Spawn` | retire — replaced by record+derived-packet and durable admission |
| `Complete`/`Freeze`/`Verify` | stay as specialized assignment/selfdev handlers — they are fate/state transitions, not desk conversation |

### Minimal-kind question handed to the panel

Candidate reductions: `ask` ⊂ `precommit` (an ask IS a staked request);
`reply` ⊂ `resolve` (the answer IS the ask's resolution); `note`/`escalate`
⊂ one `operational` kind differentiated by severity; `disagreement` ⊂
`resolve`-by-scorer. Minimal kernel candidates: `{precommit, report,
resolve, directive}` or `{precommit, report, resolve, disagreement,
directive}` — the disagreement kind exists to keep scorer≠resolver verdicts
first-class for materiality queries.

## 12. Adjudicated ontology + mechanism (convergent panel 2026-10-01, second pass)

Panel `record-native-20261001`: 7/13 quorum (codex, claude, gpt6-sol, luna,
glm53-flash, gemini38, devin).

### Minimal primitive set — unanimous

**`{precommit, report, resolve, disagreement, directive{note, escalate,
cast, retract}}`** — five record kinds; `directive` carries a typed subtype
(closed enum the reducer routes on), never free text.

| Collapse | Verdict | Ground |
|---|---|---|
| `ask` ⊂ `precommit` | **ACCEPT 7/7** | An ask IS a staked request; unstaked questions are the free chatter the model exists to kill. Repairing too: today's ask record is a shell (question text lived only in the dying envelope). Conditions: question + resolver/work binding + evidence criterion carried on the record. |
| `reply` ⊂ `resolve` | **REFINED → reply is a `report`; arrival mechanically resolves the ask** | claude/luna/devin conflict-of-interest: the answering desk must not close the issuer's stake. `report` w/ ParentID=ask; reducer derives the mechanical resolve ("answered" ≠ "material"). `resolve` stays authored for genuine verdicts. Also verified: reply records today never close the ask (no Resolve body → isResolutionRecord rejects) — asks age open forever. |
| `note` ⊂ `directive{note}` | **ACCEPT 7/7** | Unscored FYI; must be excluded from accrual/projection (today it isn't — see below). |
| `escalate` ⊂ `directive{escalate}` | **ACCEPT 6/7 conditional** | Authority target lives in `Addressee` (management/owner), not severity; requires `ParentID` link to the overdue records so escalation evidence is a join. |
| `disagreement` ⊂ scorer-resolve | **REJECT 7/7** | `ResolveCommitments` is latest-governs: a scorer-authored `resolve` would silently overturn the resolver. Disagreement is deliberately non-governing (`commitment_views.go:139-141, 233-264`) and must stay first-class. |
| `cancel` ⊂ `directive{retract}` | **ACCEPT with close semantics** | Retraction is terminal-close that scores NEITHER party (new `retracted` outcome class); a retracted woken ask still derives a stand-down packet. Verified gap: cancel today never closes the target — records stay open/aging. |
| `cast` ⊂ `precommit` | **REJECT 7/7** | Admission ≠ prediction; cast-as-precommit conflates admission failure with poor outcome (perverse trivial-cast incentive). Assignment fate already scores it; optional derived `resolve` from the terminal Complete. |

### Mechanism (B) — unanimous

Record owns the obligation (immutable, append-only); packet owns delivery
state (bind/consume/retry); target-desk controller owns rebind. Single-commit
mint = generalization of a proven pattern (owner_revision already derives
outbox wakes in one transition, lifecycle.go:733-768). Packet ID derived
deterministically from RecordID — re-mint collapses idempotently.
**Sharpest pre-existing hole (devin, verified):** stranded-bound repair
(`textureStrandedDeliveries`/`ReconcileUpdateDelivery`, max 3 attempts →
exhaust) exists only for producer_report→texture; record-native traffic into
research needs the equivalent bound-scan via `ListBoundPendingUpdatesForTarget`.
Packet exhaustion leaves the record OPEN (charges target) — never fabricate
a resolve. The open precommit aging on materiality is the semantic backstop:
delivery failure degrades to scored commitment failure, never silent loss.

### Scoring loop (C) — guardrails

1. **Tallies, not scores** in `choir.Pack` (asks_made/material/contradicted/
   unanswered) — preserves the score-free ActingPack boundary
   (`commitment_views.go:84-96`) while giving the issuer steering signal;
   owner explicitly relaxes that boundary for tallies only.
2. **Trivial asks score zero by construction**: materiality is scored against
   canonical-revision delta, so resolved-with-trivia earns nothing and still
   costs the wake tax → net negative (gemini38's "payoff vs wake tax").
3. **Symmetry gate**: charge the target for silence only after the packet
   was delivered/consumed (transport fault ≠ target fault); malformed/
   unresolvable asks resolve `unresolved`/`malformed` → charge issuer.
4. **claude's silence charge**: penalize material open claims never routed —
   else optimal play is never asking.
5. Resolver never resolves its own stake; management stays scorer of record.

### Coverage (D)

All desk-conversation verbs fit; `Complete`/`Freeze`/`Verify` stay as fate
machinery (they may mint derived `resolve` records); deadline family
unchanged; **`Emit` is the last uncommitted envelope-shaped path — retire or
reify explicitly**; engineering→management reports need a `management:*`
target contract in the validator (today texture-only — load-bearing gap now,
not optional); owner_revision stays a lifecycle command (owner is not a desk).

### Migration (E) — refined ordering

0. Narrow Q1 fix + `requested_by_*` bind stamping (restore research→texture).
1. Prune dead machinery (ReduceCellIntents, Spawn envelope, EscalateActions,
   Outcome narrowing) — shrinks the cutover surface.
2. Extract `IssueLifecycleControl`; derive packet IDs from RecordID; add
   `management:*` target contract; land stranded-bound repair for
   control-direction packets.
3. Per-KIND cutover (not per-desk): cheapest proof first (`note` →
   directive), then `report` (fixes the live bug), then resolve/disagreement,
   then ask→precommit+control arm, then escalate/cast/retract. Each commit:
   derive packet + stop envelope + update that kind's prompts. Exclusive
   per-kind transport — never dual-delivery for one act; dual-world across
   kinds is tolerable.
4. Retire `channel_message` wake last (red); channel rows remain audit.
5. Scores-in-Pack last, after supervision-only observation.

### New defects discovered by the panel (record before repair)

- **Vacuous operational records**: `commitmentRecordForIntent` mints ask/
  note/reply/escalate/cancel records WITHOUT the body/question/answer text —
  it exists only in the envelope being retired (claude/devin verified,
  rlm_reduce.go:1116-1198 has no case for them). Record-native makes this
  repair mandatory, not optional.
- **Projection pollution today**: note/ask/reply/escalate/cancel records all
  accrue as Open claims in `AccrualByAgent`/`ProjectMateriality` — the
  operational exclusion is a live defect now, before scoring is added.
- **Addressee overload**: `rec.Addressee = precommit.Resolver` — wake-target
  and resolver are one field; asks need them split (research is woken, a
  scorer resolves).
- **No record `kind` field**: kind exists only inside the RecordID string;
  projection classifies by which typed sub-object is non-nil. The directive
  subtype design needs an explicit field.
