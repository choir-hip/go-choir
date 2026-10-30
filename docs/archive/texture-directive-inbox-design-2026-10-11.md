# How a Texture takes work from outside the owner's request (P2d2 open question)

> **Revised 2026-10-12** ([re-engineering decisions](../re-engineering-decisions-2026-10-12.md)): Shape D is not built. Texture's obligation becomes its inbox fold over records addressed to `texture:<doc>`, as a stage of the records-as-work migration; the per-addressee sequencing fix survives.

Design doc for the owner's review, 2026-10-11. Resolves the structural question
that held protocol station P2d2
([definitions/choir-protocol-p2d-texture-addressing-2026-10-11.md](definitions/choir-protocol-p2d-texture-addressing-2026-10-11.md),
receipt P2d-review3, `now.decision.open_question`). Read-only research against
`main` 3e277431. No code changed. Mutation class of the eventual change: red
(Texture root, owner-reserved).

## Summary

**What I found.** A Texture already has a way to take input that names no work
item: owner revisions and unbound producer reports carry no target work, and
the resolver binds them to whatever run is active, or to the one open Texture
work item (`ResolveTextureActorOccurrence`,
`internal/textureowner/texture_controller.go:1404-1432`). The wake only
triggers; the turn consumes every pending packet up to its scheduled sequence,
whatever woke it. P2d's second revision broke this by giving each directive its
own work item, which then has to match the run's one work item. That mismatch
is the whole defect P2d-review3 found.

**Gemini's P2d-review3 claim is confirmed, and it is stronger than stated.** The
store turn commit accepts no directive at all. `ApplyTextureTurn` lets the
`directive` direction past one check and then calls
`ResolveLifecyclePacketWorkBindings`, which has no `directive` case and returns
an error. The whole turn then fails as `ErrLifecycleCommandConflict` ("lifecycle
command digest conflict"). I ran a probe test through `go test -overlay` (no
repo files touched), and it reproduces this failure.

**Recommendation.** Use shape D, "join or reopen". A directive carries no work
item. The act's commit does one of two things:

- if the Texture has an open request work item, the directive joins it;
- if the Texture's latest work item is completed, the commit reopens request
  work exactly as an owner revise does (`reopenTextureWorkForOwner`, the same
  constant-objective fingerprint).

Two small invariants make this whole:

- **Completion guard.** A turn cannot complete the request work while a
  directive addressed to it is still pending and unconsumed by that turn. This
  replaces "the work belongs to the directive".
- **Dead-work candidates.** A passivated run bound to a closed work item is not
  reactivation authority. It can never pass `ValidateActivationAuthority`
  anyway.

I prefer D to the standing inbox (A). A needs a new standing work-item prefix and
about nine exclusion sites. It also keeps the trajectory from ever settling, and
its rollback has to drain work items as well as packets. D reuses the owner's
reopen path, adds no prefix, and its rollback drains packets only. It also leaves
nothing behind if the supervision subject is deleted later. B (a run with a set
of work items) is the largest change, and it still needs A's exclusions.

**How sure I am.** The code findings are high confidence (file:line below; one
is reproduced by a probe test). The behavior claims about staging are
hypotheses, labelled as such: guest logs are not in Node B's journal, so I have
no trace for them.

## 1. What binds a Texture run to work today

All lines are at `main` 3e277431.

- **A run binds exactly one work item.**
  - `submitTextureAgentRevisionRun` sets `lifecycle_work_item_id` to the first
    open work item assigned to `texture:<doc>`
    (`internal/textureowner/texture_agent_revision.go:500-509`).
  - "First" is lexicographic by work item id (`internal/store/lifecycle.go:2482`).
  - Unlike its neighbours, this loop does not exclude the supervision prefix.
- **A turn settles only the caller's work.**
  - `ApplyTextureTurn` pins the caller work as open
    (`internal/store/texture_turn.go:357-372`).
  - It settles it only to `completed`, and only with a revision outcome
    (`:149-154`, `:464-479`).
  - An inbound packet marked `delivered` touches no work (`:537-541`).
- **The occurrence's work must equal the run's work.**
  - `validateOccurrenceReconciliationCandidates` refuses when a passivated
    candidate's work (`texture_controller.go:310`) or the active run's work
    (`:323`) differs from the resolved work.
  - `ValidateOccurrenceActivationAuthority` refuses when the run's work differs
    from `ResolvedTargetWorkItemID` (`:422-425`).
  - `ValidateActivationAuthority` also requires that work to be open (`:389-397`).
- **How the resolved work is chosen.** In order, `ResolveTextureActorOccurrence`
  uses:
  1. the occurrence's `TargetWorkItemID`;
  2. else the agent's active run's `lifecycle_work_item_id`;
  3. else the one open Texture work item, refusing if there are two
     (`:1404-1435`).

  This is the existing coalescing path. Owner revisions take it, and so do
  producer reports without a control binding.
- **The wake is a trigger; consumption is a scan.**
  - `reconcileAgentWakeLocked` arms on every pending packet addressed to the
    Texture. `ListPendingLifecycleUpdates` does not filter direction
    (`internal/store/lifecycle.go:2101-2138`, `texture_controller.go:584-621`).
  - It binds them all to the run it starts or reactivates (`:755`).
  - At commit, `textureTurnPendingInbound` consumes every covered packet. It
    keys on the run's `scheduled_message_seq`, not on the occurrence that woke
    it (`texture_turn_runtime.go:85-160`).
- **Open Texture work has wider effects.** Besides the run binding:
  - **Initial-work wake.** Any open Texture work except the supervision prefix
    arms an initial-work wake when nothing else is armed and no run is active
    (`texture_controller.go:638-666`). It also keeps sleeping runs
    reactivatable (`:694`, `:990-1007`).
  - **Owner revise target.** `ownerRevisionTarget` counts every open Texture
    work item except supervision. Two of them refuse the owner's revise with
    409 "multiple open desk target work items"
    (`texture_agent_revision.go:134-137`, `:232-261`).
  - **Owner reopen.** `textureWorkSettledByDesk` decides whether an owner
    revise may reopen work. It looks at the Texture work item with the latest
    `UpdatedAt` (`:263-295`).
  - **Settlement.** Texture trajectories settle only with no open work
    (`RequireNoOpenWorkItems`, `texture_handoff.go:254`;
    `lifecycleSettlementReady`, `lifecycle.go:2746-2757`). A Texture run that
    goes terminal requests settlement (`lifecycle.go:3188-3195`).
- **A crash closes wakes, not packets or work.**
  - A markerless boot closes pre-boot `coagent_result` wakes as
    `interrupted_by_restart` (`internal/actorruntime/handler.go:108-126`,
    `:167-171`).
  - Texture's `Start` does nothing at boot (`texture_controller.go:128-133`).
  - Pending packets and open work survive until the next input.

## 2. Gemini's claim (P2d-review3): confirmed, and broader

**The claim.** The store turn commit's acceptance of directives is narrower than
round two stated.

**What the code does.**

- `texture_turn.go:503-513` admits `Direction == directive` in the condition.
  But it first calls `ResolveLifecyclePacketWorkBindings(update)`, and any
  `bindingErr` returns `ErrLifecycleCommandConflict`.
- That function's switch handles `""`, `producer_report` and `control`. Every
  other direction falls to `default` and returns "unknown lifecycle packet
  direction" (`internal/store/lifecycle_texture_target.go:254-275`).
- The only other caller guards this. It skips the call for directives before
  invoking it (`internal/store/lifecycle_control_delivery.go:757-763`).
- The comment at `texture_turn.go:504-506` ("the targetWorkID guard already
  skips empty bindings") is wrong: the binding error is checked first.

**Probe.** I wrote a scratchpad test, mounted into `internal/store` with
`go test -overlay` so the repo is untouched. It writes a pending directive
packet on the document's trajectory, addressed to the fixture's Texture, then
commits a turn that consumes it. Output:

```
ResolveLifecyclePacketWorkBindings(directive) err = unknown lifecycle packet direction "directive": lifecycle invalid transition
ApplyTextureTurn(delivered directive) err = lifecycle command digest conflict
ApplyTextureTurn(incorporated directive) err = lifecycle command digest conflict
```

**Further narrowing, by code reading:**

- **Only `delivered` could ever work for a directive.** `incorporated` and
  `rejected` need a non-empty `ProducerWorkItemID` (`:199-201`), and that value
  must equal the packet's producer work (`:514-516`). A directive has none.
- **Latent duplicate refusal.** Two `delivered` inbound entries with an empty
  `WorkDisposition` would be refused as "duplicate terminal producer work
  consequence" (`:206-211`): `""` is not `open`, so both land on work id `""`.
  The runtime always sends `open` (`texture_turn_runtime.go:154`), so this is
  latent.

**Consequence for P2d2 (a failure mode).** Suppose `textureTurnPendingInbound`
is widened to admit directives before the store binding is fixed. Then every
Texture turn fails while a directive is pending, the run never commits, and the
activation breaker trips after three dead activations. The binding fix and the
consume predicate must land in one commit.

**Related (hypothesis, no trace).** Research's Note to its own Texture already
mints a directive packet today. Its wake content is the `sha256:` form
(`management_controller.go:3366`). The resolver's legacy path matches it and
calls `TextureProducerReportOccurrence`, which rejects the directive direction
(`internal/agentcore/texture_lifecycle_api.go:403-405`). The resolver returns
that as a plain error (`texture_controller.go:1222-1225`), not an invalid
occurrence. So the handler defers the wake instead of consuming it
(`handler.go:637-642`). Deferrals back off and are poisoned after a bound
(`internal/actor/dispatcher.go:155-158`, `:389-433`). Two things to check
against a guest console:

- whether these wakes defer up to the deferral bound;
- whether the deferred row holds back that Texture's later wakes. The dispatcher
  `break`s after a deferral.

P2d's pre-landing audit should count these packets, and the deferral lines for
them too.

## 3. The shapes

Every shape shares P2d2's items 1, 2 (packet sequence on the document's
trajectory), 3 (a directive occurrence kind with record authority), 4 to 5
(consumption and rendering), 7 (bound), 8 (retract), 9 (teaching) and 10
(refusal fate). Section 2's store binding fix is also shared. The shapes differ
only in which work item a directive's run binds to. Each one is judged on the
same questions:

- activation authority;
- coalescing with an owner revision;
- settling with no loop;
- crash behavior;
- the bound;
- retract;
- rollback;
- size.

### Baseline: one work item per directive (P2d revision 2)

- **Authority.** The resolver sets `ResolvedTargetWorkItemID` to the directive's
  own work D.
  - If a run bound to the owner's work W is active, the active-candidate check
    (`texture_controller.go:323`) refuses the occurrence. The handler consumes
    the wake ("reconcile refused", `handler.go:667-670`) and the packet sits
    pending until some other input arrives.
  - If no run is active, `submitTextureAgentRevisionRun` binds the first open
    work item by id, which can be W, not D. Then
    `ValidateOccurrenceActivationAuthority` refuses (`:422-425`).
- **Coalescing.** Not possible. The owner's run is bound to W and the
  directive's run to D. Two occurrences cannot share one run.
- **Settle.** D closes only on a revision turn that completes it. While D is
  open:
  - every reconcile treats the document as armed (initial-work wake,
    `docWakeArmed`);
  - `ownerRevisionTarget` counts D, so with W also open the owner's next revise
    gets a 409.
- **Crash.** D survives open, and the next unrelated wake re-arms it.
- **Verdict.** Unworkable, as P2d-review3 found.

### A: one standing inbox per Texture (`work:texture-directives:<doc>`)

- **Authority.** The inbox I is only a binding anchor.
  - If the directive always resolves to I, a run active on W refuses it, the
    same collision as the baseline. So A only works if the directive resolves
    the way unbound reports do: to the active run's work, else to the open
    request work, else to I.
  - That makes A a special case of C.
  - It also needs the dead-work candidate rule (section 4, item g). An idle
    Texture whose sleeping run completed W at the current head has a passivated
    candidate bound to W, which does not equal I (`:291-313`).
- **Coalescing.** Works once resolution follows the active run.
- **Settle.**
  - Turns never settle I. The store must refuse a `completed` disposition on I.
  - Packets are consumed at commit. There is no loop if I is excluded by prefix
    from the initial-work wake (`:642`).
  - I is open forever, so the trajectory never settles. Engineering-bound
    documents already behave this way: their supervision item never settles.
- **Exclusion sites the prefix needs** (each one is a place to forget):
  - initial-work wake (`texture_controller.go:640-643`);
  - `ownerRevisionTarget` (`texture_agent_revision.go:241`), or every owner
    revise next to I gets a 409;
  - `textureWorkSettledByDesk` (`:283`). If I is newer than the completed W,
    "latest" is the open I, the reopen never happens, and the owner gets a 409
    "no open desk target";
  - `lifecycleDocDeskProfile` (`texture_controller.go:71`);
  - the resolver's one-open-work fallback (`:1422-1431`), which must prefer
    request work and otherwise use I;
  - `submitTextureAgentRevisionRun`'s selection (`texture_agent_revision.go:502`);
  - the store's turn guard keeping I open;
  - the rollback drain;
  - settlement, if the owner wants directive-fed trajectories to settle.
- **Crash.** I survives as a standing item, not an obligation. Packets survive
  pending.
- **Rollback.**
  - Drain the packets.
  - Also close every inbox item. The old code would count I as owner target
    work (409s), arm initial-work wakes on it, and could bind a run to it.
- **Size.** Common items plus about nine sites. Red.

### B: a run carries a set of work items

- **Precedent.** Management and research runs already bind `work_item_ids`
  (`lifecycleRunBindsWork`, `internal/store/lifecycle_control_delivery.go:34-54`;
  `lifecycleActivationWorkItemIDs`, `lifecycle.go:2949`).
- **What changes.** The Texture side assumes one work item in many places:
  - `ApplyTextureTurnRequest.CallerWorkItemID`, its digest normalization and
    its object conditions (`texture_turn.go:46`, `:353-381`);
  - `TextureTurnRecord.CallerWorkItemID` (`:635-639`);
  - `GetAppliedTextureTurnByCallerRun`;
  - both authority validators;
  - the resolver;
  - submit;
  - `tools_texture.go:376`;
  - `coagent_injection.go:21`.

  There are 11 Texture-side reads of `lifecycle_work_item_id`.
- **Dynamic membership.** A directive committed while a run is active is not in
  that run's set. So the bind step would have to add work to a running run's
  metadata, which races the run's own `UpdateRun`.
- **Settle.**
  - Per-directive work must close on consumption whatever the outcome. That is
    a new rule beside "completed requires a revision".
  - The directive items still need the prefix exclusions from A, or
    `ownerRevisionTarget` returns "multiple".
  - Its one real advantage is settlement: per-directive items close, so the
    trajectory can settle.
- **Crash.** Per-directive work survives open. A passivated run bound to a set
  needs set-aware candidate rules.
- **Rollback.**
  - Drain packets and close directive work.
  - Runs recorded with sets are unreadable to the old single-work validators
    until they end.
- **Size.** The largest. It changes the Texture run and turn contract, which is
  the part of the Texture root the owner reserved. Red.

### C: join the open request work, else the inbox

C is A with the resolution rule made explicit. It inherits all of A's
exclusions for the idle case, plus its settlement and rollback costs. It works.
D does the same job without the inbox.

### D (recommended): join, or reopen request work like an owner revise

- **At the act's commit** (CommitLifecycleAct, target `texture:<doc>`, one
  command, conditioned on the document trajectory):
  - **Refuse visibly** when:
    - the trajectory is not live;
    - the document is engineering-bound;
    - the Texture agent is not live;
    - the latest Texture work is `refused` or `cancelled`. An owner revise is
      refused in that state too (`texture_agent_revision.go:138-161`), and the
      directive matches it.
  - **Join** if an open request work item exists. The commit mints no work.
  - **Reopen** if the latest Texture work is `completed`. Mint one request work
    item inline, with a not-exists condition, the same objective constant as
    `reopenTextureWorkForOwner` and an id derived from the record. The mint
    follows `textureSupervisionSubject`
    (`internal/store/engineering_assignments.go:1784-1823`). CommitLifecycleAct
    already holds `trajectoryMu`, so it cannot call `OpenLifecycleWork`.
    Because the objective fingerprint is shared, an owner reopen and a directive
    reopen cannot both leave an open item (`lifecycleWorkFingerprintAvailable`,
    `lifecycle.go:2822-2829`).
  - **The packet** carries no `TargetWorkItemID`.
- **Authority.**
  - The directive occurrence resolves its work like an unbound producer report
    (`texture_controller.go:1404-1432`): the active run's work, else the one
    open request work item. No new work rule.
  - Its own checks (record exists and is not retracted, emitter, addressee,
    packet pending on this trajectory) are P2d item 3, unchanged.
  - `ValidateOccurrenceActivationAuthority` gains the same
    `MessageSeq <= ScheduledMessageSeq` check producer reports have (`:440-442`).
- **Coalescing.**
  - An owner revise and a directive pending together resolve to the same work.
    The owner revise targets the joined or reopened item, because
    `ownerRevisionTarget` finds exactly one.
  - Both arm one run, the run binds both packets and the owner head, and its
    commit consumes both. This is how owner input and research reports
    coalesce today.
- **Settle, with no loop.**
  - The directive is consumed as `delivered` at commit. Nothing is left
    pending, so no wake re-arms.
  - The request item stays open after a no-change turn. That is today's normal
    state after an owner request answered without settling. The initial-work
    wake only runs inside a reconcile that some occurrence triggered. No new
    item exists per directive, so nothing accumulates.
  - **The race P2d-review2 found** (a turn completes W while a directive is in
    flight) is closed by the completion guard:
    - The directive's commit bumps the document trajectory.
    - A turn reading the earlier version fails its CAS and retries
      (`tools_texture.go:395-400`).
    - On the retry, the guard sees a pending directive outside this turn's
      inbound. The runtime keeps W open and adds a runtime note to the reason.
    - The store refuses `completed` in that state, so the guard holds under any
      interleaving.
    - The directive's own wake then reactivates the run bound to W, which is
      still open.
  - **Bounded.** The guard cannot hold W open forever:
    - the four-per-request bound stops new directives;
    - the three-attempt delivery budget terminalizes any directive that cannot
      be delivered (`texture_controller.go:762-813`).
- **Crash.**
  - A reopened item survives exactly as an owner-reopened item does today.
  - The directive's wake is a `coagent_result`, closed at a markerless boot.
  - Nothing at boot mints work.
  - For packets, see question 2 at the end. My recommendation is that the
    pre-boot directive wake also terminalizes its packet as
    `interrupted_by_restart`, so a later turn never silently consumes a
    pre-crash directive (CLAUDE.md: crash closes, never resumes).
- **Bound.** Unchanged from P2d item 7.
  - CommitLifecycleAct counts directive packets to `texture:<doc>` on the
    document trajectory since the latest owner input. `latestOwnerInput` moves
    from `texture_turn_runtime.go:171-185` into the store.
  - The fifth commits its record and a terminal packet (`bound_spent`), with no
    reopen and no wake.
- **Retract.**
  - The CancelAct terminalizes the original packet, if pending, in the same
    commit. The existing `ExpireStaleLifecyclePacket` command already sets
    `cancelled` and marks the wake projected (`internal/store/lifecycle.go:1059`,
    `lifecycle_expire.go:37-100`). It needs a reason field, because today it
    writes `stale_obligation_expired`.
  - A reopened item is not closed. It is ordinary request work the owner may
    also be using.
- **Rollback.**
  - `git revert`, plus a drain that terminalizes pending directive packets to
    Textures (reason `rollback`), using the same expire command.
  - Reopened items need no drain: the old code reads them as owner-reopened
    work.
- **Size.**
  - The shared items.
  - The reopen inside CommitLifecycleAct.
  - The completion guard, in the store and the runtime.
  - The dead-work candidate rule, in two places.
  - No new prefix and no exclusion sites.
  - Red.

## 4. Recommended change, by code site (shape D)

Store (red):

- **a. Binding fix.** `internal/store/lifecycle_texture_target.go:254`
  (`ResolveLifecyclePacketWorkBindings`) gets a `directive` case that returns
  empty bindings, and an error if either work field is set. Fix the comment at
  `texture_turn.go:504-506`.
- **b. Turn commit.**
  - `internal/store/texture_turn.go:191-234` accepts directive inbound as
    `delivered` only. It refuses `incorporated` and `rejected` with a named
    error, not a digest conflict.
  - Before the commit, if `CallerWorkDisposition == completed`, refuse with
    `ErrConcurrentStateChange` while any pending directive addressed to the
    caller on this trajectory is missing from `req.Inbound` (the completion
    guard).
- **c. Act commit.** `internal/store/lifecycle_commit_act.go:342-413`, for a
  `texture:<doc>` target:
  - load and condition the document trajectory, number the packet from it, and
    bump it once (P2d item 2);
  - check the bound;
  - refuse or join or reopen, as in section 3;
  - commit a past-bound packet terminal.
- **d. Shared objective constant.** Move `textureOwnerReopenObjective`
  (`texture_agent_revision.go:295`) and the "latest Texture work" rule
  (`textureWorkSettledByDesk`, `:263-295`) into the store. The owner path and
  the act path then use one definition.
- **e. Expire command.** `internal/store/lifecycle_expire.go:37` takes a
  disposition reason. It is used for retract, refusal fate, crash closure and
  the rollback drain.

Texture owner and agentcore (red):

- **f. Directive occurrence.**
  - Add the occurrence kind, its constructor and its encoding to
    `internal/agentcore/texture_lifecycle_api.go:250-405`.
  - Build it from the legacy `sha256:` match
    (`texture_controller.go:1216-1227`), or encode it at
    `management_controller.go:3366` for Texture targets.
  - `ProducerAgentID` is the emitter (the handler's source check,
    `handler.go:648-657`).
- **g. Dead-work candidates.**
  - `texture_controller.go:291-313` (`validateOccurrenceReconciliationCandidates`)
    and `:959-1007` (`reactivatePassivatedTextureRun`) skip passivated
    candidates whose `lifecycle_work_item_id` is not open.
  - This only removes outcomes that already fail `ValidateActivationAuthority`
    (`:389-397`). Today nothing reaches them, because an owner revise always
    moves the head first.
- **h. Directive resolution.**
  - Add a directive case to `ResolveTextureActorOccurrence` (`:1251-1338`):
    record authority, then fall through to the existing work resolution at
    `:1404-1432`.
  - `TextureActorOccurrencePostcondition` (`:1508-1529`) reads the packet's
    disposition.
  - `ValidateOccurrenceActivationAuthority` (`:440-442`) gets the sequence
    check for directives.
- **i. Runtime consumption.**
  - `texture_turn_runtime.go:49-160` gets one consume predicate shared with
    `textureDispositionIneligibleReason`. A directive must be bound to this run,
    and `coagentUpdateDeliverableForRun` (`management_controller.go:1716-1719`)
    already requires that.
  - `applyTextureLifecycleTurn` (`:438-441`) forces `open` and adds a note when
    pending unconsumed directives exist.
- **j. Rendering.** Fill the nil worker-message slot at
  `texture_agent_revision.go:476` (P2d item 5).
- **k. Handler fates.** In `internal/actorruntime/handler.go`:
  - a refused directive occurrence (`:637-641`, `:667-670`) terminalizes its
    packet with the reason (P2d item 10);
  - a pre-boot directive wake (`:167-171`) terminalizes its packet as
    `interrupted_by_restart`, if the owner agrees (question 2).

Unchanged from P2d2 as drafted: validation (`needTo`), the P2d1 gate, the
management overlay, and the four-per-request bound with its Texture-side twin
in `textureTurnControls`.

Edits to P2d2's text if D is adopted:

- **Item 2.** "always mints one open Texture work item for this directive"
  becomes "joins the open request work, or reopens request work when the latest
  is completed".
- **Item 3.** "sets `ResolvedTargetWorkItemID` to the directive's own work item"
  becomes "resolves work as an unbound producer report does".
- **Item 4.** "the same commit completes the directive's own work item" becomes
  the completion guard.
- **Rollback.** Drop "closes the directive work items they own".

## 5. Failure modes (shape D), written before code

- **The binding fix lands without the consume predicate, or the reverse.**
  Every turn fails while a directive is pending, and the breaker trips (section
  2). Land them in one commit, with a test that a pending directive does not
  block an unrelated turn.
- **The guard starves completion.** Directives keep arriving and W never
  completes. This is bounded by four directives per request and the
  three-attempt budget. Test: a fifth directive does not hold W.
- **The guard is enforced only in the runtime.** A directive commits between
  the runtime's snapshot and the store commit. The store must refuse, relying on
  the trajectory CAS. Test the interleaving at the store.
- **The two reopen paths diverge.** An owner reopen and a directive reopen use
  different objectives, both items open, and the owner's revise gets a 409.
  Prevented by one shared constant (item d). Test the concurrent owner-plus-
  directive reopen.
- **The dead-work rule hides real authority.** A passivated run bound to open
  work is skipped by mistake. Test both sides: open work keeps the candidate;
  completed work drops it and a fresh run starts bound to the reopened item.
- **A fresh run loses actor memory.** A fresh run replaces a sleeping one. It
  is rewarmed only through budget spend (`texture_agent_revision.go:516-538`).
  Accept, and name it in the turn note.
- **The activation check uses the wrong sequence space.** `latestEligibleWorkerMessage`
  (`texture_controller.go:433-439`) compares a trajectory sequence with channel
  message sequences. A document that also carries eligible worker channel
  messages refuses the run. This is shared with producer reports (P2d's
  activation test).
- **Every directive costs an in-flight turn one retry.** Each directive commit
  bumps the document trajectory, so a concurrent Texture turn fails its CAS once
  and the model must retry (`tools_texture.go:395-400`). The guard needs that
  bump. Residual: the runtime could resubmit unchanged inputs itself when the
  head did not move.
- **Pre-existing research Notes change meaning.** These directive packets
  become consumable after deploy. Run the pre-landing audit, and record their
  first consumption.
- **A settled document refuses directives.** A directive to a settled
  trajectory is refused at commit (visible). D lets a directive-fed document
  settle when its run ends with all work closed. A cannot settle.
- **The outbox redispatches after a crash (hypothesis).** The actor-wake outbox
  sweep dispatches unprojected wakes after boot (`agentcore/runtime.go:2579-2668`).
  If a directive's wake was minted and not projected before a crash, it may be
  dispatched with a post-boot timestamp and so escape the `interrupted_by_restart`
  check. The crash test must cover this window.

## 6. Tests (shape D)

Store:

- **Binding.** A directive with empty work binds; one with work set is refused.
- **Consumption.**
  - The probe in section 2 inverted: a pending directive is consumed as
    `delivered`.
  - `incorporated` or `rejected` on a directive gets a named refusal.
- **Completion guard.**
  - `completed` is refused while an unconsumed directive is pending.
  - It is accepted when the directive is in this turn's inbound.
  - An interleaving test: the directive commits after the turn's snapshot, the
    store refuses, and the retry keeps W open.
- **Act commit.**
  - The packet is numbered on the document trajectory.
  - The commit joins when W is open (no work object written).
  - It reopens when the latest work is completed.
  - It refuses when the latest is refused or cancelled, the document is
    engineering-bound, or the trajectory is closed.
  - A concurrent owner reopen and directive reopen leave exactly one open item.
  - The fifth directive commits its record and a terminal `bound_spent` packet,
    with no work and no wake.
- **Retract.** The original packet is cancelled in the CancelAct's commit, and
  the stand-down packet is terminal.

Controller and handler:

- **Idle Texture.** The Texture's sleeping run completed W at the current head.
  A directive:
  - reopens W';
  - skips the dead candidate;
  - starts one run bound to W';
  - is consumed;
  - starts no further run without new input. Count runs over a window.
- **Coalescing.** An owner revise and a directive pending together produce one
  run. Its prompt carries both, and its commit consumes both.
- **Forgery.** A forged directive is refused and its packet terminalized:
  - wrong owner;
  - research or engineering emitter;
  - retracted record;
  - missing record.
- **Source check.** A directive wake passes the expected-source check.
- **Crash.**
  - A markerless boot closes a pre-boot directive wake, runs no turn and mints
    no work.
  - If question 2 is adopted, the packet is terminal `interrupted_by_restart`.
  - The outbox window: a wake minted, then a crash before projection.

Deployed (end to end, repeatable artifact):

- On a fresh test computer, an owner-prompted management Cast to
  `texture:<doc>` on a document whose work is completed.
- The audit joins record → packet → reopened work → consuming turn → revision.
- Then an owner revise, which must join the same work item, not get a 409.

## 7. Heresy delta (this review)

- **Discovered.**
  - The store turn commit refuses every directive, and reports it as "digest
    conflict" (confirmed by probe).
  - The comment at `texture_turn.go:504-506` is false.
  - `submitTextureAgentRevisionRun` does not exclude the supervision prefix that
    its neighbours exclude (by code reading).
  - A passivated candidate bound to completed work would refuse any wake that
    does not move the head. This is latent today.
  - Hypothesis: research Notes to their own Texture defer rather than consume.
  - Hypothesis: the outbox can redispatch a pre-crash wake after boot.
- **Introduced.** None. No code changed.
- **Repaired.** None.

## Open questions for the owner

1. **D or A?** D (reopen request work) puts management-fed work into the same
   object as owner requests. The obligations surface would show "Revise the
   document for the owner's new request" for a management-fed reopen, unless the
   shared objective text becomes neutral, for example "Revise the document for
   new input". Changing it changes the fingerprint for future reopens only. Do
   you accept D's conflation, or do you want A's separate inbox and its nine
   exclusion sites?
2. **Crash fate for directive packets.** Today a crash closes wakes, and pending
   producer reports wait for the owner's next action. Should directive packets
   whose wake was interrupted terminalize as `interrupted_by_restart`? I
   recommend yes. It is stricter than producer reports, which is an
   inconsistency we would name.
3. **Settlement.** Under D, a directive-fed document can settle once its run
   ends with all work closed, and later directives are then refused (visible).
   For Autopaper stories, is that right, or should a story's trajectory never
   settle while its source poll is live (P5)?
4. **Explicit dispositions.** Should the Texture desk be able to reject a
   directive with a reason (`rejected`), or only receive it (`delivered`, with
   its answer in the turn's reason)? P2d2 as recommended allows only
   `delivered`.
