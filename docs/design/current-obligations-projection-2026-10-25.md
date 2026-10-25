# Current obligations: one ledger-derived view every desk renders (2026-10-25)

Problem record: docs/evidence/selfdev-trace-reviews-2026-10-25.md, the
"Clustering assessment after rounds 1 and 2" (Cluster A) and the Round 3 and
Round 4 sections. Kind: an architectural design (mutation class and review tier per step are in
the section 6 table). This note is green. Read against origin/main e0c67bfb1.
**Finding** means code or a trace shows it; **hypothesis** means inference.

## 0. The problem in one paragraph

Each desk decides "what is required now" from its own hand-built context, and
each one drops a different field. Seven instances, all from 2026-10-25:
(a) Texture lost the owner's 02:59 direction one revision after it was given
(Round 2 §3); (b) the guard reminder did not show that the committed Complete
was `partial` (Round 2 §1); (c) management cast a follow-up before it read the
engineer's answer (Round 2 §3); (d) management did not forward the 05:27 owner
correction to the bound engineer (Round 4 P1); (e) Texture's first cast was a
summary, so the engineer never saw the quality bar (P2); (f) Texture was not
woken by management's final report (P7); (g) the engineer did not see its
activation budget (Round 1 #4). There is an eighth in Round 4 §1: management's
Verify judged against the stale spec. The doctrine already names the substrate.
C7 says obligations, questions and verdicts "become durable workstream state",
I7 says they must be durable obligation state, and I26 says the commitment
ledger is the supervision surface that Texture renders. What is missing is the
read side: one derivation that every desk renders. The writes already exist.

**Core decision.** Add one pure fold, `obligations.Build`, over records the
ledger already holds. It produces one trajectory-scoped value,
`choir.trajectory_obligations.v1`. Each desk sees a role-filtered rendering of
that value, as a runtime-injected user turn. The runtime injects the turn again
whenever the desk's view digest changes, using the Texture frame pattern. The
same value is readable in-cell (`choir.Obligations()`) and from the CLI
(`choir lifecycle obligations <trajectory>`). Nothing is stored. The projection
writes nothing and wakes no one.

## 1. What exists: where each desk derives "what is required now"

| # | Desk | Where (origin/main) | Derives from | Drops |
|---|---|---|---|---|
| 1 | Texture | `buildAgentRevisionRequest`, internal/textureowner/texture_agent_revision.go:673; seed at :680, head `owner_prompt` at :691 | Head revision metadata only: `seed_prompt` carried by `durableMetadataKeys` (texture_task_types.go:41). `owner_prompt` is written per revision at :154 and is not durable | Every owner direction except the head's; open assignments; candidate verdicts; open questions. Patched by #2 |
| 2 | Texture | `collectTextureOwnerDirections`, texture_owner_directions.go:46 (walk 64, keep 8, 4000 chars at :25-29), called at texture_agent_revision.go:432 behind `texture.owner_directions` | Parent-chain walk of user revisions | Directions past 64 revisions or another document's chain; still no assignments, verdicts or questions |
| 3 | Texture | Frame, internal/agentcore/texture_frame.go:125 (`currentTextureFrame`), injected at :165 | Head blocks plus **unread** inbox records past the cursor (`loadCellRecords`, inbox_records.go:144) | Any record once the cursor passes it. A final Report read last turn is gone this turn |
| 4 | Texture | Supervision line, run_context.go:86 | Operation id only. Text tells Texture to read `choir.Updates()` | Operation state, verdict, reject reason |
| 5 | Texture | Mid-run owner packet, `lifecycleOwnerRevisionTurnForRun`, management_controller.go:2301 | Head only, and only while the head is a pending owner revision (`PendingTextureOwnerRevision`, store/texture_owner_revision.go:39) | Every earlier direction |
| 6 | Texture | Wake predicate, texture_controller.go:563-656 | Four separate tests: armed updates, owner head, `InboxUnread` (:612), open work item | Nothing is wrong with the data; P7 shows a final Report sat unread 12 min with no wake recorded (finding). The cause is not yet traced (hypothesis: no trigger called the controller) |
| 7 | Management | Overlay, runtimeprompts/overlays/rlm_management_runtime.yaml:19, :44-46; run context gets identity only (run_context.go:68) | Its own reads of `choir.Records()` (cursor-based) and `choir.Pack()` | Whether a Note or Ask it sent is answered; whether an owner direction is newer than an assignment's Cast. Both are prompt rules (:46, commit 8ca63b699) with no data behind them |
| 8 | Management (verify) | `choir.Verify`, tools_capsule.go:440-460 | The candidate's evidence | The newest owner direction (Round 4 §1). The fail reason is stored as a fixed string (:457), so later readers lose it |
| 9 | Engineering | Run context, run_context.go:92-106 | Assignment id, kind and subject digest; the carried-patch notice (carried_patch.go:793) | Owner directions, Notes, the budget and its own last fate. Computed once per run (runtime.go:3720), so it goes stale mid-run |
| 10 | Engineering | Cast `request` field, `engineeringRequestView`, engineering_cast_request.go:101-139 (16 KiB caps at :35-36) | Texture's Cast record, bound by digest | The owner's own words. When Texture summarizes (P2) or re-casts the original (Round 3 §3), that is all the engineer gets |
| 11 | Engineering | Budget line, `withActivationBudget`/`activationBudgetLine`, capsule_jobs.go:523-545 | The process context deadline (`ctx.Deadline()`) | The line is shown only on job-end resumes and guard reminders, never on turn 1 or ordinary turns. A durable deadline exists (runtime.go:550-556) but is not read |
| 12 | Engineering | Completion guard, runtime.go:4346; reminder :4426; fate scan `engineeringOverlayTerminalFateCommitted` :4505 | Parsing tool_result JSON in the **transcript** | The assignment's ledger fate. The reminder names `partial` only after a pattern match (:4433) |
| 13 | Engineering | Carried notice, carried_patch.go:550; history `CarriedCandidates` :674 | Candidates, joined to reports, ops and owner rejection (`carriedRejectionReason` :755) | Management's verify reason (it is read from owner-reject decision events only) |
| 14 | Operator | `/api/runtime/obligations`, obligations_surface.go:14 | Computer-wide counts: wakes, runs, open ops | Not trajectory-scoped; nothing about content |

Pattern (finding): rows 1, 3, 5, 9 and 12 read one record, or one turn's
worth, and treat it as the whole state. Rows 7 and 8 have the rule but not the
data. No row reads the conversation history of an assignment as a whole.

## 2. Check for an existing replacement

None of these is a single source today, but most of the parts exist. Connect
them; do not build a new store.

- **Lifecycle snapshot** (`Store.GetLifecycleSnapshot`, store/lifecycle.go:2299;
  type at types/lifecycle.go:712; CLI `lifecycle snapshot`). It gives the
  trajectory, work items, agents, engineering assignments (disposition, reason,
  report refs, capsule fate), the head revision and 25 events. Read-only on
  ac0cb480 (62 KB): three Texture work items. The first objective holds the
  whole 8,484-character request. The two owner-revise items carry only "Revise
  the document for the owner's new request." (48 chars). One assignment,
  `completed`. `updates` is null. The head metadata has **no `seed_prompt`**
  (finding). It does not hold: earlier revisions, commitment records,
  self-development operations, verdicts, budgets or settings. **This is the
  spine.** The fold calls it once and joins the rest.
- **Conversation fold** (`ConversationRecords`/`ConversationState`/`OpenRounds`,
  store/inbox.go:954-1027; `foldConversation` :877). It is ledger-derived and
  indexed by conversation_id. It folds rounds and hypotheses but not
  Ask/Note/Reply pairing. **Extend it** with a `Questions` field. Do not write
  a second fold.
- **Assignments by round** (`ListEngineeringAssignmentsByParentRound`,
  store/engineering_assignments.go:825) joins a Texture request to its
  assignments. Reuse it.
- **Candidate history** (`storeCarriedHistory.CarriedCandidates`,
  carried_patch.go:674) already joins candidate, assignment, report, operation
  and owner rejection for one trajectory. **Lift it** into the fold, and add
  the verification event payload (`VerifierRefs`) as the verify reason.
- **User revisions** (`ListTextureRevisionsByScopeOG`, store/graph_store.go:2602)
  list a document's revisions by scope. Filtering `AuthorUser` with
  `owner_prompt` gives every owner direction without the bounded parent walk
  in row 2.
- **Delivery precedent**: the frame injector (texture_frame.go:165) re-injects
  when its mark (`doc rev through`) changes. The mark is recovered from
  run-memory runtime injections only, so a model cannot forge it (:31-51). The
  owner-packet injector derives "seen" from run memory the same way
  (management_controller.go:2496-2504). The new turn copies this pattern.
- **Budget**: the durable `activation_budget_deadline` continuation
  (runtime.go:550-556; `HandleActivationBudgetDeadline`,
  continuation_schedule.go:375) is the authority. The fold reads its
  not-before. It does not read the context.

Rejected: (1) extending `LifecycleSnapshot` itself. It is the wire shape of a
CLI/API reply and is read on hot paths (controller boot). Adding joins there
would slow every caller. (2) Extending `/api/runtime/obligations` into a
content view. That surface is computer-wide liveness and should stay counts. It
gains a detector section only (section 5).

## 3. The projection's content

`internal/obligations` (new package, no store import). Its readers are
interfaces bound in agentcore, as `carriedPatchDeps` already is. `Build(ctx,
readers, owner, computer, trajectory) (Obligations, error)` is a pure fold.
`View(o, role, subject) View` filters by role. `Render(v) string` writes the
turn. Fields:

1. **Owner intent.** `Request`: the original request with its authority ref
   (section 4). `Directions[]`: every owner direction in order of the
   revision's `version_number`, each with `revision_id`, `at` and full text.
   `Governing` is the index of the newest. Rendered rule (unconditional
   invariant text, I16): "The newest owner direction governs where it differs
   from the request or an earlier direction."
2. **Rounds and assignments.** For each Texture-to-management round on this
   trajectory: open or closed, and whether its final Report has been read by
   Texture (inbox cursor ≥ seq). Under each round, each assignment: id, kind,
   bound agent/run, disposition with reason, network mode, the Cast's
   objective (first 400 chars plus ref), and the **last committed fate**: the
   newest report's `result` (`partial` marked non-terminal), summary (≤ 600
   chars), `fate_terminal`, and the frozen candidate id if any. Also
   `DirectionAfterCast`: true when the governing direction is newer than the
   assignment's Cast record (the P1 fact).
3. **Candidates.** From the lifted `CarriedCandidates`: candidate id,
   assignment, operation state, management's verdict with its reason (from the
   verification event payload), the owner's decision and reject reason, and
   whether it was carried into a later round.
4. **Open questions.** From the extended conversation fold: each Ask or Note
   addressed across a desk boundary on this trajectory's conversations, whether
   a Reply or later record from the addressee answers it, and the answer text
   (≤ 600 chars). An unanswered Ask stays listed until a Reply arrives or the
   conversation closes.
5. **Budgets and deadlines.** The activation deadline as **absolute UTC**, from
   the durable continuation, so the digest does not change every minute (the
   run-context turn already gives the time). Also job wall-clock limits of a
   running job, and the management task deadline where one applies.
6. **Switches.** The computer settings that change what this trajectory's acts
   do (`engineering.carry_unfinished`, `engineering.progress_to_texture`,
   visual capture grant, capsule network mode), given as facts. The overlays
   keep their templating (tool availability). The projection only names state.
   Scores, calibration and Disagreement records are **excluded** from every
   view (I26: "scores stay off the acting desk's context").

**Role views** (one value, field selection only; there is no second fold):

| Field | Texture | Management | Engineering (impl.) | Verification (Verify step, verification assignment) |
|---|---|---|---|---|
| Request + all directions | full | full | full | full |
| Rounds / assignments | all, one line each | all, with `DirectionAfterCast` | own assignment in full; siblings one line | the candidate's assignment |
| Last committed fate | per assignment | per assignment | **own**, with `partial` named | n/a |
| Candidates + verdicts | all | all | this trajectory's (what was rejected, why) | prior verdicts on the same request |
| Open questions | to/from Texture | sent by and to management | to/from this assignment | none |
| Budget | none | task deadline | activation deadline, job limits | none |
| Switches | evidence embeds | carry/progress | carry, network, capture | capture |

Management runs serve many trajectories. Its view renders one block per
trajectory with an open round addressed to it (`OpenRoundsAddressedTo`,
store/inbox.go:1027), newest activity first, at most 3. Beyond 3 it gives
counts only.

**Size bounds and truncation** (the rendered turn; `choir.Obligations()`
returns the whole value uncut):

- Budget: 10 KiB for engineering and verification, 8 KiB for Texture, 12 KiB
  for management (3 trajectories). Tonight's requests are 7-8.5 KB, so the
  request is the dominant item.
- Never cut, in this order: the **governing direction** (full up to 16 KiB,
  the existing request cap; past that, head and tail with "cut at N of M
  bytes; choir.Obligations().Directions[k] has all of it"), the desk's own open
  assignment and its last fate, and unanswered questions addressed to the desk.
- Cut next, in this order: earlier directions become one line each (time, id,
  first 200 chars), newest first, and then a count. The original request
  becomes head and tail with a pointer (it is never dropped, only shortened).
  Closed assignments and candidates beyond the 3 newest become counts.
- A render that cannot fit even the never-cut set prints the governing
  direction and a pointer, and drops everything else. A test pins this.

## 4. Single state authority (standing question 5)

The projection is derived and never written. Each fact has one writer:

| Fact | Record (authority) | Writer |
|---|---|---|
| Original request | V0, the trajectory's first `AuthorUser` revision (I2b) | prompt bar / trajectory start |
| Owner directions | `AuthorUser` revisions with `owner_prompt` | owner route only, `handleLifecycleOwnerRevision` (texture_agent_revision.go:97-154), I1 |
| Rounds, Casts, Notes, Asks, Replies, Reports | commitment records (inbox_seq, conversation_id, round) | the reducer, from staged intents (`CommitLifecycleAct`, store/inbox.go:447 `planInboxRecords`) |
| Assignment disposition, fate, capsule | `EngineeringAssignment` objects | reducer commands in store/engineering_assignments.go (Open :1147, Bind :1478, Report :1840, Cancel :2106) |
| Candidates, verdicts, owner decision | selfdev operations; verification and decision events | `openFreezeCandidate`, `choir.Verify` (tools_capsule.go), owner act |
| Activation deadline | `activation_budget_deadline` scheduled continuation | runtime at activation start (runtime.go:550) |
| Switches | computer settings (store/computer_setting.go:159-177) | owner acts |

Open authority question (finding, section 2): ac0cb480's head has no
`seed_prompt`, and its request is the first work item's objective. The fold
reads V0's `owner_prompt` or content first and falls back to the first Texture
work item objective. A detector logs any case where the two differ. Choosing
one home is a red follow-up and is listed in section 7.

**Restart.** There is nothing to rebuild. The fold runs at render time on the
tape, after boot like any other time. A crash boot closes rounds with
system-emitted closure records ("System records never wake anyone",
types/commitment.go:457) and cancels assignments `interrupted_by_restart`
(engineering_assignment_fate.go:743-754). The next render shows those fates.
It never re-dispatches them, and the projection schedules nothing, so it
cannot become a boot-time resumer. On a planned restart with a marker, the
resumed run replays its memory, including its last obligations turn. The
injector compares digests and injects the current view once if anything moved
across the restart. The one-resume-per-marker rule is unchanged.

**Fate-sharing (question 7).** A render reads the object graph (snapshot,
records, revisions), the selfdev operation store and the event payload reader.
If any of them fails, the turn is skipped (no injection) and
`obligations_render_failed` is counted on `/api/runtime/obligations`. The run
continues on its prompt, as the frame does at texture_frame.go:172-176. Desk
progress never waits on the projection.

## 5. Delivery

1. **In-run.** `obligationsInjector(rec)` is composed with the frame, the
   coagent and the calibration injectors (runtime.go:3744-3755). Before each
   model call it builds the run's role view. It hashes the canonical JSON of
   the view, excluding time-of-render. If that digest differs from the
   `Choir obligations. digest=<d>` header of the last runtime injection in run
   memory, it appends a full self-contained turn. Every injection is
   append-only, so the cached prefix holds (prompt-cache problem
   2026-10-14). A job-ended record does not change the view. Expected changes
   per run are a handful: owner direction, a fate, a verdict, a question and
   its answer.
2. **Owner correction while an engineer is bound (P1).** The engineer's view
   holds the directions, so the engineer sees the correction at its next model
   call without any relay. Its rendered invariant: "Your grant is
   management's Cast. When the governing owner direction conflicts with it,
   choir.Ask management before acting on the difference." Management still
   decides cancel, narrow or continue (I26 admission stays with management).
   The overlay rule at rlm_management_runtime.yaml:44 ("Speak into an open
   assignment with choir.Note") stays as the means to narrow, not as the
   carrier of the owner's words. A park on a running job (`engineeringJobPark`,
   capsule_jobs.go:396) ends only at job end, which can be up to 45 min. Step 3b
   below proposes that a change to the governing direction signals
   `signalJobWake` (capsule_jobs.go:202), so the park resumes with the new turn.
   It is a separate switch and an owner question.
3. **Idle desks.** The projection adds no wake path. Wakes stay where they
   are: a record wakes its inbox, an owner revision wakes Texture. The
   projection supplies the **detector** for wakes that should have happened.
   A new section on `/api/runtime/obligations`, `unseen`, lists each desk whose
   view has a change it has not rendered, older than 120 s, with the change
   kind. P7 (a final Report unread for 12 min) would have shown as `texture:<doc>
   final_report_unread 720s`. The fix for the missing wake itself goes to the
   wake path once traced. It does not get a second wake bus.
4. **Ad-hoc relays this replaces.** (d): management forwarding owner text. (e):
   Texture's cast as the only carrier of the request (the engineer reads the
   owner's words itself; the Cast stays the grant). (b): the guard's
   transcript-parsed reminder (step 5 reads the ledger fate). (g): the budget
   line computed from `ctx` (it reads the durable deadline).

## 6. Migration

One setting per desk, all off by default (the house pattern, computer_setting.go:159).
While a desk's setting is on, its old symptom block is suppressed in the same
render. That avoids a dual path that could contradict itself (I5). The old
code stays until step 7.

| Step | Change | Setting | Class / tier |
|---|---|---|---|
| 0 | This note | none | green, reviewed as an architectural design (tier 1, Devin) |
| 1 | `internal/obligations` fold + `choir lifecycle obligations <traj>` (read route) + conversation-fold `Questions` + verify reason read; no desk renders | none (read only) | orange, tier 1 |
| 2 | `/api/runtime/obligations` `unseen` and `obligations_render_failed` | none (report only) | orange, tier 1 |
| 3 | Engineering and verification-assignment injector; budget from the durable deadline. Keeps the carried notice, `withActivationBudget` (now from the deadline) and the guard reminder. 3b: governing-direction change wakes a job park | `obligations.engineering`; 3b `obligations.interrupt_park` | orange, tier 1; 3b red (run lifecycle), tier 1 |
| 4 | Management injector (with `DirectionAfterCast`, open questions); Verify sees the governing direction | `obligations.management` | red (run acceptance input), tier 1: additive, switch, one-act rollback |
| 5 | Texture injector; the `owner_directions` block in `buildAgentRevisionRequest` is not printed while on | `obligations.texture` | red (input to canonical writes), tier 1 |
| 6 | Completion guard reads the assignment's committed fate from the ledger instead of parsing the transcript (runtime.go:4505) | `obligations.engineering` | red (run acceptance), **tier 2** (silent if wrong) |
| 7 | Deletions on a dated clock, once each desk has passed its staging scenarios in two rounds: `collectTextureOwnerDirections` + `texture.owner_directions`, transcript fate scan, budget-from-ctx, `actor_rewarm_source_loop_id` write | settings retired | red deletes, **tier 2** |

Rollback for every step: set that desk's setting off (an owner act, no
deploy). The old block returns on the next render. Code rollback: revert the
step's commit. The projection writes nothing, so there is no data rollback.

**Tests: failure modes, written before code** (CLAUDE.md Testing):
1. The newest direction is cut or dropped at any size. A golden test at 2x, 4x
   and 20x the budget checks that it is present in full or with the cut marker.
2. Directions are ordered wrong. Version numbers out of created_at order: the
   fold must follow version_number.
3. The digest changes with time, so the turn is injected on every call. Two
   renders a minute apart with no tape change must give the same digest.
4. A run is injected twice for one change, or a forged header suppresses it.
   Only runtime-injection entries count (copy the texture_frame.go:31 test).
5. Role leak. Golden views per role: no scores or Disagreement in any view,
   and the engineer sees no sibling assignment bodies.
6. A boot changes the view and something wakes. A crash-boot fixture must
   leave zero scheduled continuations from the projection.
7. A render failure blocks a run. With a failing reader, the run starts with
   no injection and the counter goes up.
8. Old and new blocks both print. With the setting on, the
   `owner_directions` text is absent; with it off, the bytes are identical
   to today.
9. `partial` reads as terminal in the engineer's own-fate line (from the
   Round 2 fixture).
10. Replay fixtures. The fold over exported e3e37430 (cancelled) and ac0cb480
    (live) state reproduces each Cluster A instance as a present field.

**Staging proof** (b@c, choir.news, no SSH, artifacts are `trace export` run
memory plus `choir lifecycle obligations` output):
- S1 (Round 2 §3, Round 3 §3): request, then owner revise, then a progress
  relay wake. Texture's next turn holds the governing direction, and its cast
  follows it.
- S2 (P1): owner correction while the engineer is bound. The engineer's next
  obligations turn holds it, and the frozen candidate honours it.
- S3 (P2): Texture casts a summary. The engineer's view still holds the
  request's quality bar and prohibitions.
- S4 (Round 2 §1): Complete(partial). The own-fate line says non-terminal.
- S5 (Round 2 §3 item 3): management sends a Note. Its view lists it
  unanswered until the Reply.
- S6 (Round 4 §1): Verify's evidence cites the governing direction's revision
  id.
- S7 (Round 1 #4): turn 1 of the engineer shows the absolute deadline.
- S8 (P7): a final Report unread for more than 120 s appears in `unseen`.

## 7. Failure modes and heresy delta

Residual failure modes beyond the tests:
- The model reads the projection and still disobeys. The projection fixes the
  input, not the judgment. Staging answers that question. Per CLAUDE.md, a
  disobedience claim needs the run's obligations turn in evidence.
- Prompt growth. About 8-10 KiB per change, a handful of times per run.
  Measure input tokens on S2 against Round 4's 48.3M baseline.
- The engineer over-reads the owner's words as authority beyond its Cast.
  The invariant in section 5 item 2 guards this, and S2 checks it. If it fails,
  narrow the engineer's view to the governing direction only.
- Management's three-trajectory cap hides a fourth live round. That round is
  still counted, and its records still wake management.

Heresy delta:
- **discovered**:
  - "What governs now" has no single derivation. Fourteen hand-built context
    sites (section 1), each dropping a different field.
  - The completion guard derives the assignment fate from the transcript
    rather than the ledger (runtime.go:4505).
  - The original request has two homes with no named authority (`seed_prompt`
    vs the first work item objective). ac0cb480's head has no `seed_prompt`.
  - Open questions (an Ask or Note without a Reply) have no fold: ConversationHead
    tracks rounds and hypotheses only (types/inbox.go:133).
  - Management's verify reason is not on the operation (a fixed
    `TerminalError`, tools_capsule.go:457), and the carried history reads only
    owner rejections.
  - The budget line reads the process deadline although a durable one exists.
- **introduced**:
  - One more runtime-injected turn type (prompt growth, measured).
  - A render path that fate-shares with three readers (degrades to no turn).
  - A temporary dual path per desk until step 7, gated by switches and on a
    deletion clock.
- **repaired** (when steps 3-5 are on and proved): Cluster A instances
  (a)-(g) and Round 4 §1, as data every desk renders. This is not repaired
  until S1-S8 pass on staging.

## 8. Open questions for the owner

1. Should engineering read the owner's own words (this design), or only
   Texture's rendering of them? Reading them directly is what fixes P2. The
   cost is that Texture is no longer the only interpreter of owner intent for
   engineering, though the Cast stays the grant.
2. Should a change to the governing owner direction end an engineer's job park
   (step 3b), or wait for the job to end (up to 45 min)? The harness thesis
   says interruption is a first-class transition. This is a run-lifecycle
   change, so it is red.
3. Which record is the original request's home: V0's content or the first
   work item's objective? The other then becomes a view (I27).
4. Should management's Verify fail reason be written onto the operation
   (`TerminalError`, or a ref to the verification payload) so every reader
   gets it? Today only the event payload holds it.
5. Are three per-desk settings right, or should there be one
   `obligations.render` setting with per-desk rollout by deploy order?
