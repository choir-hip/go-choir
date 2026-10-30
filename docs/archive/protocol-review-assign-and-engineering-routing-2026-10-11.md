# Review: `choir.Assign`, and routing engineering's reports through management

> **Revised 2026-10-12** ([re-engineering decisions](../re-engineering-decisions-2026-10-12.md)): Assign is deleted. Jev is part of management and either passes a report straight through or management reasons and writes it; there is no hold (none exists in code). Path B dissolves into owner → Texture → management → engineering.

2026-10-11. Research and review for the owner; no code changed. Answers
design §9 item 3 and the owner's addressing note in
[precommitment-protocol-design-2026-10-11.md](../precommitment-records.md)
("Owner answers"). Read against `main` 4b114e61. Evidence is code reading
unless marked as a trace or receipt. Items marked **hypothesis** were not
confirmed by a trace or a test.

## Summary

**What I found.**

1. **`choir.Assign` does nothing.** It is a stub from the August broker
   prototype. It writes to a map inside the worker process and returns
   `{status: "dispatched"}`. At cell end the map drains into strings like
   `assign/<id>/<task>` in the cell output. No store write, record, wake or
   assignment comes from it. Only the engineering desk has it. The trace
   audit counted 0 uses. About 55 lines of production code exist only for
   it, plus about 15 lines of tests. **Certainty: high.** I traced every
   reference.
2. **The real "assignment" is something else.** It is the durable
   `EngineeringAssignment` (capsule, attempt, fate, reports). A management
   `choir.Cast("engineering", …)` already opens one through
   `openDelegatedCastAssignment`. So "fold Assign into Cast" is done in
   effect: deleting the stub changes nothing.
3. **The simplification worth having is in the admission paths.** The
   assignment has three admission authorities:
   - management's delegated Cast;
   - an "owner/document cast", where an owner-authored revision on an
     engineering-bound document is the admission. The self-development
     operations API and M11 use it, and no management is involved;
   - a legacy run-parented "co-super" branch. Only the test seed and the
     Node B replay goldens reach it.

   The document cast is also the only place where engineering's reports go
   straight to a Texture, through the minted `texture:<doc>` supervision
   subject.
4. **Jev** is the decision model `typesafe/jev-1.13`. You give it a state
   and typed questions, and it returns probability distributions, through
   the gateway at `/provider/v1/judgments`. Today it has one caller: the
   host-side frozen-candidate judgment. Management has no verb for it.
   Station M5 ("management = scorer + jev.decide + passthrough-or-translate
   routing", owner-ratified 2026-09-29) is `blocked_incomplete`. So Jev
   cannot make the routing decision today. Its interface fits a
   forward/restate/hold choice, and measured latency is about 0.2 s p50.
5. **The owner's routing proposal restates the 2026-09-29 ratified shape.**
   That shape was never built.

**What I recommend.**

- Delete `choir.Assign` and its broker plumbing now (green-to-yellow risk).
  For one release, keep an exported stub that returns a retirement message,
  as P2c did for Message and Emit.
- Keep the word "assignment" as the noun for the durable engineering object.
  Do not rename the store types.
- Agree with the owner that **engineering addresses only management**. Get
  there by retiring the document-cast path, so management's Cast is the one
  admission path and the one report path. That is the substantive
  simplification. It is red-class, and it has three prerequisites:
  - the self-development judgment chain is keyed to the admitting revision.
    I believe, without having tested it, that the chain fails for delegated
    casts today;
  - M11 starts through the API, so its start has to move;
  - run acceptance has a document-cast branch that must change.
- **Partly disagree on the gate criterion.** Use "is this material to the
  owner?" rather than "how technical is it?". Use Jev only to **triage**
  (forward / restate / hold), not to rewrite. Never hold terminal results,
  blockers, owner questions or approval boundaries. Send the engineering
  report to the Texture as a reference, not as inline text. That gives
  protection by default and full detail on demand. Two LLM paraphrases in a
  row (management, then Texture) is the main fidelity risk. The M11 rerun 12
  failure (Texture saying "nothing changed") came from missing state, not
  from technical language.

## Part 1. What `choir.Assign` is

### The full call path

- `internal/yaegikernel/choir.go:425-435`: `ChoirScope.Assign(taskID,
  actorProfile, instruction)` checks `mutateDenied` and then calls
  `s.call(ActionAssign, AssignPayload{…})`. `s.call` (`choir.go:208`) calls
  `s.broker.HandleRequest` directly. This is the broker **inside the worker
  process**, not a host egress round-trip.
- `internal/yaegikernel/broker.go:166-167` dispatches to `handleAssign`.
  `broker.go:356-377` mints a receipt id, stores the payload in
  `b.assignments`, and returns `AssignResult{AssignmentID, Status:
  "dispatched"}`.
- `broker.go:113-131`: `DrainReceipts` turns each stored entry into the
  string `"assign/"+id+"/"+TaskID` and deletes it.
  `session_loop.go:148-150` puts the drained strings into
  `SessionResult.Receipts`. `internal/agentcore/tools_desk.go:375-409`
  copies them into the cell output JSON as `"receipts"`, beside the
  `rlm:<kind>:<seq>` receipts of staged intents. Nothing else reads them.
  The comment at `broker.go:113-116` says so: "host-reconciled, never
  trusted as durable; undrained entries die with the worker". No host-side
  reconciler exists.
- It is not a tray intent. The reducer has no `IntentAssign`
  (`grep IntentAssign` finds nothing). `rlm_reduce.go` handles
  `IntentCancelAssignment`, which is management's capsule revoke and is
  unrelated.
- Surface:
  - `deskModuleSets` (`choir.go:250`) grants `Assign` to **engineering
    only**. Management does not have it; management delegates with `Cast`.
  - The session handle grants `ActionAssign` to every non-research role
    (`choir.go:95`).
  - The Help surface describes it as "Records a worker assignment and
    returns its receipt" (`surface.go:42`).
- Prompt:
  - The engineering overlay lists `choir.Assign` among "Synchronous Syscalls
    (immediate effects)" (`internal/runtimeprompts/overlays/rlm_engineering_runtime.yaml:49`).
  - The same overlay ends with "Never start another agent"
    (`rlm_engineering_runtime.yaml:60`).
  - So the prompt offers a verb that claims an effect, and then forbids the
    effect it claims.
- Use: the problem record's trace audit counts `choir.Assign` at **0 uses**
  (`docs/archive/problems/inter-agent-communication-bypasses-precommitment-records-2026-10-11.md:38`).
- History: the broker action dates from 5e396897 (2026-08-26, "Phase 1 —
  versioned flat DTO broker protocol"). The verb dates from af0cc214
  (2026-09-04). It was a placeholder for subagent spawning, and that
  feature went a different way (delegated Cast, R2).

**Heresy, discovered:** a verb that reports `dispatched` for work that is
never dispatched. It is harmless only because no model has called it.

### What the code that exists only for Assign amounts to

| Where | What | Lines (approx.) |
|---|---|---|
| `yaegikernel/choir.go` | `Assign` method; `deskModuleSets` entry; exports map entry; `ActionAssign` in the session handle; "delegate = Assign+Cast" comment | ~15 |
| `yaegikernel/broker.go` | `assignments` map and init; `case ActionAssign`; `handleAssign`; assignment loop in `DrainReceipts` | ~30 |
| `yaegikernel/broker_protocol.go` | `ActionAssign`, `AssignPayload`, `AssignResult`, validation case | ~13 |
| `yaegikernel/surface.go` | Help entry | 1 |
| overlay | `choir.Assign` in the syscall list | 1 token |
| tests | `choir_parity_test.go:86-96` (assign half of corpus cell 5), `:212`, `:246-247`; `broker_test.go:79` | ~15 |

Deleting Assign also leaves neighbouring code with no caller:

- `choir.Message` now returns a retirement error without calling the broker
  (`choir.go:445-447`). So `handleMessage`, the `messages` map, and all of
  `DrainReceipts` with its `drain` parameter through `session_loop.go` are
  reachable only from tests. About 50 more lines.
- `internal/yaegikernel/actor_state.go` (205 lines, plus 151 lines of test)
  defines `ActorStateManager` and `DurableAssignment`. Nothing outside its
  own test uses it.

### How Assign differs from a Cast to engineering

A Cast to engineering is the real admission path:

- `rlm_reduce.go:974-1011` (legacy branch) and `:1930` (record-native
  branch). The commitment record mints first. Then
  `castOpensEngineeringAssignment(in.ToDesk)` (`rlm_reduce.go:1796-1800`;
  gated to the engineering profile since P2d1, 6990b09d) calls
  `rt.openDelegatedCastAssignment`.
- `engineering_assignment_runtime.go:455-575`. That function:
  - resolves the caster's open work item;
  - derives a deterministic assignment id from the commitment control id;
  - preflights the capsule subject;
  - persists the binding with `CastAuthority: delegated`, and the
    lifecycle outbox mints the spawn wake in the same transaction;
  - `resumeDelegatedCastAssignment` (`:582-600`) later spawns, binds and
    activates.
- The store authenticates the caster through
  `requireEngineeringDelegatedParentAuthority`
  (`internal/store/engineering_assignments.go:884-962`). It checks:
  - a live trajectory;
  - a persistent caster agent (`LifecycleVersion == 0`) whose run is bound
    to the trajectory;
  - that the commitment record the cast minted was authored by the caster;
  - the caster's open work item.

So Cast does everything Assign's name promises: durable, record-first,
authority-checked and capsule-bound. Assign does none of it. **Folding Assign
into Cast is complete in substance. All that remains is deleting the stub.**

One gap to close while folding: the delegated branch never checks that the
caster is management. In practice only the computer-scoped
`management:<owner>` agent has `LifecycleVersion == 0`. Texture, research
and assigned engineering agents are minted at version 1 or higher
(`lifecycle_control_issue.go:142`, `engineering_assignments.go:1800`). The
store comment says "a persistent Management (or desk) run" (`:865`). The
legacy branch does check `Profile == Management` (`:677`). The essence the
owner named is "admitted by management", and it should be an explicit check
rather than a side effect of versioning. That is one line.

### The three admission authorities of the durable assignment

`types.EngineeringCastAuthority` (`internal/types/engineering_assignment.go:30-40`)
names two of them. The store's authority switch
(`engineering_assignments.go:648-700`, historical twin `:711-775`) has three
branches:

1. **Delegated** (`CastAuthority == "delegated"`): a management Cast.
   Reports go to `management:<owner>` and are admitted by
   `persistentManagementAdmissibleReport`
   (`management_controller.go:1642-1655`). This is the path the protocol
   design keeps.
2. **Owner/document cast** (`ParentRunID == ""`, parent
   `engineering:<docID>`).
   - `startAssignedEngineeringForDocument`
     (`engineering_assignment_runtime.go:263-438`) says in its doc comment:
     "It replaces the retired Management-mediated assign_co_super opener:
     the revision event IS the admission."
   - Its entry points:
     - `ReconcileEngineeringDesk` and `reconcileEngineeringCast`
       (`engineering_desk.go:132-315`);
     - the boot scan (`textureowner/texture_controller.go:110-125`);
     - `ensureSelfDevelopmentEngineeringDoc`
       (`selfdev_texture_join.go:41-120`), which
       `POST /api/computers/{id}/self-development/operations` calls
       (`api_self_development.go:1219,1230`).
   - It builds a synthetic document per operation, with an initial revision
     marked `AuthorKind: AuthorUser` (`selfdev_texture_join.go:81`).
   - **This path has no management in it.** It is also the only path whose
     reports go to a Texture (Part 2).
3. **Legacy co-super** (`ParentRunID != ""`, not delegated): a persistent
   management *control run* parent, which used to be opened by the deleted
   `assign_co_super` tool. No production code opens such a binding any
   more. `store.SeedEngineeringAssignmentAuthority`
   (`engineering_assignment_seed.go:24`) seeds it for ten test files, and
   the replay goldens' manifest records exactly that seed
   (`parent_run_id: run-management-assignment`,
   `docs/archive/evidence/rlm-replay/goldens/manifest.json`).

Approximate size of what exists only for branches 2 and 3:

| Area | Lines |
|---|---|
| `engineering_desk.go` cast reconcile and restart recast (the verification chain inside it must be re-homed, not deleted) | ~200 |
| `selfdev_texture_join.go` | ~120 |
| `start/resumeAssignedEngineeringForDocument` | ~200 |
| `requireEngineeringDocumentParentAuthority` | ~80 |
| Legacy branch | ~100 |
| Supervision subject, its three commit sites, and the report retarget | ~90 |
| Texture `supervisionOnly` and the supervision-prefix exclusions | ~30 |
| Run-acceptance document-cast checkpoint (`run_acceptance.go:175-185`) | ~10 |

That is roughly 800 lines, plus their tests. This is where "is assign a
needless subsystem?" really applies. The question is less about the verb
than about a second admission path that bypasses management.

### What must stay (the essence)

- The `EngineeringAssignment` object and its saga: binding, attempt,
  capsule and network mode, spawn/bind/activate, fate, reports, deadline,
  cancel with executor acknowledgement (`CancelAssignment`). Capsule-bound
  mutation by engineering is this object.
- Admission by a management Cast, authenticated against the commitment
  record it minted.
- `choir.Complete` and `choir.Freeze` on the engineering surface, the freeze
  candidate (`freeze_candidate.go`), and the host-side candidate judgment
  (`selfdev_candidate_judgment.go`).
- The noun "assignment". It is the right word for an attempt-numbered,
  capsule-bound unit of work with a fate. Renaming
  `EngineeringAssignment` would churn the vocabmigrate identity seeds and
  replay identities for no gain. The verb a desk uses is already Cast.

### Prerequisites before branch 2 can go (findings from code reading)

- **The self-development judgment chain is keyed to the admitting
  revision.**
  - `reconcileEngineeringVerification` loads
    `GetLifecycleRevision(…, implementation.Binding.ParentControlID)`
    (`engineering_desk.go:371`). For a document cast, `ParentControlID` is
    the revision id (`engineering_assignment_runtime.go`, `parentControlID
    := req.RevisionID`). For a delegated cast it is the commitment record
    id.
  - `reconcileEngineeringCast` also refuses a document with no
    `engineering:<doc>` desk agent (`engineering_desk.go:200-209`).
  - **Hypothesis:** a delegated-cast freeze on a Texture's trajectory
    therefore never reaches `judgeFrozenCandidate`. If the head is
    owner-authored, the reconcile errors with "not bound to the engineering
    desk". If the head is agent-authored, it errors with "load admitting
    revision". I found no test that drives a delegated freeze to a judgment
    (`r2_acceptance_test.go` and the other delegated tests stop earlier).
    This matches the Gate 2 record: "The Texture-to-engineering path for a
    self-development request has not been exercised end to end"
    (`definitions/choir-appdev-gate2-supervised-self-development-2026-10-09.md`).
  - Fix shape: key the chain by the operation and the assignment, and take
    the objective from the operation's prompt artifact or the cast record's
    objective.
- **M11 starts through the API.** `scripts/m11_selfdev_episode_probe.mjs`
  POSTs an operation. It has to start some other way: an owner prompt to a
  Texture, or an owner directive to management that management turns into a
  Cast. The owner's own input is not agent speech, so an owner-originated
  directive to management fits design §6.
- **Management can enter the document-cast path through ProductAPI.**
  `/api/computers/` is on the allowlist (`tools_product_api.go:190`), and
  ProductAPI sets `X-Authenticated-User` to the owner
  (`tools_product_api.go:69`). So management can
  `POST …/self-development/operations` and start a document cast whose
  admitting revision is recorded as owner-authored. That is a hidden second
  admission path, and the fold must close it. The same prefix also admits
  `POST …/self-development/rollbacks` (`api_self_development.go:197-202`).
  Whether a rollback is a protected owner act is an open question below.
- **Possible boot-time resumer (hypothesis).**
  - The boot scan (`texture_controller.go:110-125`) runs
    `ReconcileEngineeringDesk` on every boot, crash or planned.
  - Its restart-recast branch (`engineering_desk.go:213-258`) reopens a
    restart-cancelled implementation at the next attempt (at most 3), and
    nothing checks the planned-restart marker.
  - A crash boot does fail the pre-decision operation
    (`closeSelfDevelopmentOperationsAfterCrash`,
    `self_development_materializer.go:87-112`). I did not verify whether
    the recast still fires before or after that, on a trajectory that is
    still live.
  - If it fires, it conflicts with the restart rule in CLAUDE.md ("Do not
    add boot-time resumers that run without a planned-restart marker").
    Retiring branch 2 removes the question either way.

### Migration

- **Deleting `choir.Assign`.**
  - Nothing durable migrates. The broker map lives in the worker and dies
    with it.
  - The replay goldens (`docs/evidence/rlm-replay/goldens/`:
    `record_assignment_result`, `commit_transaction`, `update_coagent`,
    `inspect_self_development_bundle`, `record_self_development_verification`)
    exercise `choir.Complete`, Freeze, Verify and Report. None of them calls
    `choir.Assign`, so they are unaffected.
  - A cell staged before the deploy that calls `choir.Assign` would fail to
    compile. Following P2c, keep an exported `Assign` that returns
    "choir.Assign is retired: engineering does not start agents; report to
    management with choir.ReportPacket or choir.Escalate" for one release,
    then delete it.
  - Same commit: remove the overlay mention, the parity-test assign half,
    and the dead drain plumbing.
  - Class: yellow (prompt text and tests), with an orange edge (the worker
    protocol's action set).
- **Retiring the legacy co-super branch (3).**
  - In-flight rows: none can be live. No production opener exists, and a
    crash boot closes open work.
  - Historical rows only need to stay readable. Keep
    `requireEngineeringHistoricalParentAuthority`'s branch for one release
    if late reports from pre-50fee6ed rows are a concern, then delete it.
  - Goldens: `TestRLMReplayGoldens` (`rlm_replay_linux_test.go:616`) skips
    unless `RLM_CAPTURE_STATE` is set (`:160-163`). It runs only on Node B,
    against a capture state dir built on 8f987d7f with the seed's
    run-parented binding. Once branch 3 is deleted that replay cannot
    authenticate. The choice is to recapture under a delegated binding, or
    retire the goldens as a finished cutover receipt (the five retired JSON
    tools have been gone since 50fee6ed). I lean towards retiring them: the
    files stay as evidence, and only the Linux driver goes.
- **Retiring the document cast (2).** Red class: run acceptance,
  self-development operations, lifecycle reducer.
  - Order:
    1. key the judgment chain by operation;
    2. move M11's start onto a management Cast;
    3. close the ProductAPI path;
    4. switch the run-acceptance checkpoint to the delegated assignment;
    5. delete the openers, the supervision subject, `supervisionOnly`, and
       the boot reconcile.
  - In-flight document-cast assignments: deploys restart only idle
    computers, so a busy computer keeps the old binary until it finishes.
    For an assignment that a *planned* restart would resume under the new
    code, the update boot should terminalize it with a visible fate
    ("superseded by update"). The alternative is to keep the opener for one
    release. There is no auto-migration into a delegated binding: the
    authority is different, and inventing a management cast after the fact
    would forge provenance.

## Part 2. Engineering → management → Texture, and Jev

### What happens today

- **Delegated casts (management → engineering).** Engineering's reports go
  to management.
  - `buildEngineeringReturnPacket` (`engineering_assignments.go:1719-1762`)
    targets `Binding.ParentAgentID` and `ParentWorkItemID`.
  - Management admits engineering producer reports of kinds
    `evidence_update`, `execution_result`, `blocker`, `question`,
    `proposal` and `decision_request` (`management_controller.go:1642-1655`).
  - Management then reports to the Texture that opened its work
    (`Report`/`ReportPacket`, management overlay
    `rlm_management_runtime.yaml:23-28`).
  - This is already the owner's shape, minus Jev.
- **Document casts (owner revision or self-development API →
  engineering).**
  - The parent is `engineering:<docID>`, a desk agent that "never runs"
    (`engineering_desk.go:17-22`). `buildEngineeringReturnPacket` therefore
    retargets the report to `texture:<docID>` and
    `work:texture-supervision:<docID>` (`engineering_assignments.go:1748-1757`).
  - `textureSupervisionSubject` (`:1778-1820`) mints that agent and work
    item inside the report or cancel commit (sites `:2258`, `:2393`,
    `:2722`).
  - The Texture controller treats it as report-only (`supervisionOnly`,
    `texture_controller.go:594-606`; occurrence guard `:1380-1390`; desk
    profile exclusion `:55-90`).
  - The Texture run gets a "Self-development supervision" overlay
    (`tool_profiles.go:249`).
  - **Here engineering addresses a Texture directly.** Engineering's model
    does not know it. Its overlay says "Report progress to Management …
    addressing only the requesting Management agent_id"
    (`rlm_engineering_runtime.yaml:60`). The retarget happens in the store.
- **Policy already agrees with the owner.** `agentprofile.PolicyFor`
  (`agentprofile.go:76-81`) gives engineering `AllowedMessageTargets:
  [Management]`. P2d refuses an engineering Cast or Note to `texture:<doc>`
  at validation, and P2d lists "engineering or research addressing a
  Texture" as excluded
  (`definitions/choir-protocol-p2d-texture-addressing-2026-10-11.md`,
  artifact item 1 and `excluded`). The supervision subject is the one
  remaining exception.

### Evidence from M11 about what the owner sees

- **Rerun 13** (`docs/evidence/m11-rerun-2026-10-10T14-46-02Z.json`,
  `document_render`). The operation's synthetic document was revised by the
  supervision Texture (`last_author_kind: appagent`). Its content includes a
  table of SHA-256 digests, a 52-file release tree, "70 entries — 52
  artifact files", and "Freeze classifier v1". That is the technical
  leakage the owner is pointing at. It happened with engineering reports
  feeding the Texture directly. (Receipt: the content is quoted in the
  JSON.)
- **Rerun 12**
  (`docs/archive/problems/m11-rerun-12-restore-refused-and-texture-reports-no-change-2026-10-10.md`,
  Failure 3). The Texture told the owner that no change had been applied
  when it had been. The Texture wrote from a cancel report of the producing
  run. Hypothesis H1 in that record: the Texture renders from desk reports
  and never sees the operation's state. That is a **fidelity-of-state**
  failure, not a technical-language one. A management relay fixes it only
  if management attaches the operation state. Management can read that
  state (`GET /api/computers/…/self-development/operations`).
- The supervision Texture narrates a **synthetic per-operation document**.
  The Gate 2 exit test starts from "the owner asks Texture" in their own
  document. On the document-cast path, the owner's request Texture and the
  operation's supervision Texture are two different Textures narrating one
  piece of work. Routing through management to the requesting Texture
  removes that split.

### What Jev is, and whether it can decide this today

- **What it is:**
  - the decision model `typesafe/jev-1.13`, pinned at the gateway
    (`internal/gateway/jev.go:19-23`);
  - called through OpenRouter's Decisions alpha endpoint as
    `POST /provider/v1/judgments`, with a per-VM bearer and a 60/min bucket;
  - input: `state` plus typed `questions` (choice, binary and so on, with
    criteria). Output: a full probability distribution and a confidence
    score per question (`gatewayruntime/jev.go:13-28`).
  - Receipt `docs/archive/evidence/m4-jev-transport-roundtrip-2026-09-30.md`:
    331 input tokens, $0.000014 per call.
  - Routing experiment `docs/archive/problems/prompt-bar-minesweeper-demo-2026-10-10.md`:
    24 of 24 correct, p50 208 ms, p90 269 ms.
- **Where it is used:**
  - one caller, `judgeFrozenCandidate`
    (`agentcore/selfdev_candidate_judgment.go`). It runs host-side, asks
    three yes/no questions, and parses fail-closed (`:93-128`). It replaced
    the engineering verifier run (owner, 2026-10-10).
  - **Desks have no Jev verb.** It is not in any `deskModuleSets`.
- **History:** the routing role was ratified on 2026-09-29: "the routing
  work that management does, as a passthrough (if jev agrees) or translator
  between eng and texture, is really coupled with precommitment scoring"
  (`definitions/choir-signal-management-scorer-jev-2026-09-29.md`,
  `decision`). That station is `blocked_incomplete`. Design §9 item 2 now
  says the scorer is "to be learned from iteration".
- **Answer: no, not today.** The transport works, and its shape (a choice
  question with criteria, a confidence score, low cost, sub-second) fits a
  per-report triage. What is missing is a caller in the report path: either
  a host-side hook before management wakes, or a management verb.

### My view of the owner's proposal

**Agree:** engineering addresses only management. Reasons:

- It matches the desk ontology in CLAUDE.md (management owns computer-wide
  coherence and admits engineering), engineering's own message policy, and
  P2d's validation.
- It removes the supervision-subject machinery and the two-Textures split.
- It gives one place, management, that knows the operation's state, the
  assignment's fate and the owner's request together. Rerun 12 failed for
  lack of exactly that.
- It makes the hand-off a record management authors under P3 (a prediction
  on every Cast and Note), which the protocol can score.

**Disagree, or refine, in four places.**

1. **Use materiality as the gate, not technicality.**
   - "How technical" is a property of wording, and the Texture is already
     the owner-facing writer. Its run overlay is where wording belongs. Some
     technical content should reach the owner on purpose: the owner asked
     on 2026-10-10 that code diffs be transcluded into the document and
     that engineering revisions state hypotheses, what is built, and what
     would falsify it (`prompt-bar-minesweeper-demo-2026-10-10.md`, "Owner
     direction").
   - The question management should answer per report is "does this change
     what the owner knows, must decide, or what the document claims?".
   - My suggested Jev question:
     `{"type":"choice","choices":["forward","restate","hold"]}`, with
     criteria written for materiality. Technical density is one input:
     restate when the report is material but not written for the owner.
2. **Jev triages; it does not translate.**
   - Jev returns distributions, not prose. Restating needs an LLM turn
     (management's RLM), which costs a model turn per report.
   - Engineering reports are technical by nature, so I expect "restate" to
     be the common outcome. That makes Jev's saving the hold and forward
     cases, not the restate case.
   - Cheaper alternative: make engineering write the owner-facing sentence.
     The report packet already has `summary`. Require it to be
     plain-language and have Jev check it ("is this summary sufficient for
     the owner?"). Forward on yes; restate with an RLM call only on no or
     low confidence. This follows the 2026-09-29 rule ("Jev default,
     escalating to an RLM sub-cast on low confidence").
3. **Protect by default, keep detail on demand.**
   - Do not paste the engineering report into the Texture's prompt. Send
     management's owner-language body plus the engineering report's record
     id and evidence refs, so the Texture can transclude the diff or cite
     the receipt when the owner asks.
   - Inline raw text is what the M11 document echoed. LLMs repeat what they
     are given, so removing raw text from the prompt is more reliable than
     telling the writer to ignore it.
   - Without the references, two paraphrases in a row (management, then
     Texture) lose fidelity, and nobody can check either one.
4. **Some report kinds are never held.**
   - Always forward: terminal results (completed, failed, cancelled),
     blockers, questions or decision requests to the owner, the approval
     boundary (frozen candidate awaiting approval), and the
     applied/rejected/rolled-back transitions.
   - Hold only progress chatter (`evidence_update` without a state change).
   - A hold is a terminal disposition on the packet with a reason, visible
     in the trace. It is never a pending obligation that a boot could
     resume.
   - Otherwise the rerun 12 failure ("nothing changed") turns into a quieter
     one ("the owner never heard it changed").

### Consequences

- **Latency.**
  - Today, a document-cast report wakes the supervision Texture directly:
    one Texture turn, about 64 s for a first turn in the 2026-09-29 QA
    timeline (`orientation-jev-supervision-metamission-2026-09-29.md` §A).
  - Through management:
    - with host-side Jev triage first: about 0.2-0.4 s, then a management
      turn only on restate, then the Texture turn;
    - with management's model deciding every report: one extra model turn
      per report, tens of seconds to minutes (hypothesis; I found no
      measured management-turn timing).
  - There is one `management:<owner>` per owner, so self-development
    reports queue behind whatever else management is doing. Autopaper W2
    plans to send poll observations to management (design §7 P5).
    Management must coalesce pending engineering reports into one turn, and
    the triage should run host-side so a hold costs no management wake.
- **What the owner sees in self-development (M11, Choir-in-Choir).**
  - The narrative moves from the synthetic operation document to the
    Texture where the owner asked.
  - Fewer revisions: holds disappear.
  - Plainer language, with digests and diffs as transclusions.
  - Approval evidence must not depend on management's paraphrase. The
    approval surface (`choir self-dev show|approve`, and a GUI control that
    is still missing) reads the operation directly. Keep it that way.
- **What breaks.**
  - The supervision subject, its three commit sites, and the report
    retarget.
  - Texture's `supervisionOnly`, the supervision-prefix exclusions, and the
    "Self-development supervision" overlay.
  - The run-acceptance document-cast checkpoint.
  - M11's start and its `document_render` leg, which reads the synthetic
    document.
  - Tests on these paths: `store/engineering_assignments_test.go` and five
    agentcore test files that use the document-cast openers
    (`api_self_development_test.go`,
    `engineering_assignment_overlay_test.go`,
    `engineering_desk_verification_chain_test.go`,
    `scheduling_readiness_test.go`, `selfdev_texture_join_test.go`).
  - The judgment chain needs re-keying (Part 1 prerequisites).
  - Engineering's overlay does not change: it already reports to
    management.
- **Interaction with P2d.**
  - P2d gives management exactly the delivery this needs: a Cast or Note to
    `texture:<doc>`, minting a record and Texture work, consumed by the turn
    and bounded to four per request.
  - Management's restated report becomes a P2d directive. Under P3 it
    carries a prediction (for example "Will the next revision state the
    candidate awaits approval? p=0.9"), which scores whether management's
    relay was faithful.
  - P2d's `must_preserve` says "producer_report … occurrences resolve
    exactly as today". That stays true for research → Texture. For
    engineering, the producer-report path to a Texture disappears, and P2d's
    note "research and engineering already reach a Texture through producer
    reports" should be corrected.
  - P2d-review3's open question proposes a standing directive inbox per
    Texture (`texture-directives:<doc>`) "following the existing
    supervision surface". With the supervision subject gone, that inbox
    becomes the one standing non-request work item on a Texture. That is a
    simplification, not a conflict, and it is a reason to settle P2d2's
    inbox shape before deleting the supervision subject, so the code pattern
    is not lost.
- **Research is out of scope here.** Research also reports to the Texture
  directly. I would leave it that way: it is the Texture's own per-document
  desk, it is on the latency-critical news loop, and the owner's direction
  names engineering only.

### Proposed order of work

1. **Green/yellow:** delete `choir.Assign` with a retirement stub, plus the
   dead drain plumbing and `actor_state.go`. Add an explicit `Profile ==
   Management` check to delegated admission (orange, one line, with a
   rollback revert).
2. **Red, P2d2 first:** land management → `texture:<doc>` delivery.
3. **Red:** key the self-development judgment chain by operation and
   assignment, then prove that a delegated-cast freeze reaches a judgment on
   staging. This is the Gate 2 exit path anyway.
4. **Red:** host-side triage hook (Jev, three-way, materiality criteria,
   with the never-hold classes enforced in code, not in the prompt) before
   the management wake. Management restates only on restate or low
   confidence, and the restatement is a P2d directive with references.
5. **Red:** move M11's start onto a management Cast. Close the ProductAPI
   self-development start. Retire the document cast and the supervision
   subject. Retire or recapture the replay goldens with the legacy branch.

Each slice keeps its rollback as a git revert. Slice 5 needs the
in-flight terminalization described under Migration.

## Heresy delta (this review)

- **Discovered:**
  - `choir.Assign` reports `dispatched` for nothing;
  - the engineering overlay offers Assign and forbids its effect;
  - delegated admission is management-only by accident of versioning, not
    by a named check;
  - management can start a document cast, recorded as owner-authored,
    through ProductAPI;
  - (hypothesis) the judgment chain cannot serve delegated casts;
  - (hypothesis) the restart-recast branch is a boot-time resumer with no
    marker check.
- **Introduced:** none. No code changed.
- **Repaired:** none.

## Open questions for the owner

1. Do you agree the gate criterion is materiality rather than technicality?
   Do you agree that terminal results, blockers, owner questions and
   approval boundaries must never be held?
2. Should engineering be required to write a plain-language `summary` that
   Jev checks, so most reports forward without a management model turn? Or
   should management restate every material report?
3. Should the triage run host-side before management wakes (fast, and a
   hold costs nothing), or inside management's turn as a verb (slower, but
   management sees everything)?
4. Retire the document-cast path, and so the self-development operations
   API's own engineering document, once the delegated judgment chain is
   proven? M11 would then start from an owner request to management or to a
   Texture.
5. The replay goldens depend on the legacy run-parented authority. Retire
   them as a finished cutover receipt, or recapture them under a delegated
   binding on Node B?
6. Is `POST …/self-development/rollbacks` through management's ProductAPI
   intended? It acts as the owner. If rollback is an owner-only act, it
   belongs on the ProductAPI refusal list with the Texture writes.
