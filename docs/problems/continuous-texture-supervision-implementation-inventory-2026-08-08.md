# Continuous Texture Supervision — Implementation Inventory

**Status:** open implementation problem receipt  
**Observed:** 2026-08-08 on clean `main` / `origin/main`
`d40a711f67144b4e1c41e3155e4d79975682dec1`; staging `/health` reported
`6965f7f71f764f91737b21804bc376281cbdbe8f`.  
**Authority:**
[`choir-continuous-texture-supervision-2026-08-07.md`](../definitions/choir-continuous-texture-supervision-2026-08-07.md).
This receipt records facts and freezes the next schema boundary. It is not
runtime acceptance or authority to weaken that Definition.

## Why this receipt exists

Read-only pre-mutation inventory found additional production gaps after the
Definition was promoted. Under the problem-documentation-first invariant, this
problem receipt must land before repair code. The local Definition dashboard was
running and serving a current non-authoritative projection at
`http://127.0.0.1:8787/` while the inventory was performed.

Mutation class for this receipt is **green**. The later implementation is
**red** because it touches lifecycle reducers, Texture canonical writes,
actor restart delivery, role authority, capsule isolation, evidence, and
acceptance. Effects remain OFF.

## Observed authorities and defects

### Addressing and lifecycle delivery

- `internal/agentprofile.Policy.AllowedDelegateTargets` is spawn policy. It is
  consumed by `CanDelegate` and `coagentowner`'s `spawn_agent`; there is no
  independent message-address policy. Texture is correctly spawn-Researcher
  only and must stay that way.
- `RegisterCoagentUpdateTools` installs the immediate generic `update_coagent`
  for Super, CoSuper, Researcher, Processor, and Reconciler, but not Texture.
  Its only target resolver is in `internal/agentcore/tools_researcher.go`.
- The generic resolver accepts channel/request metadata as target authority,
  considers a Texture channel before an explicit `agent_id`, and contains
  `ErrNotFound` fallbacks. The final `GetAgentByScope` check in
  `tools_worker_update.go` runs authorization only on successful lookup and
  ignores every lookup error. Missing or arbitrary targets can therefore pass
  the tool boundary.
- Researcher target resolution proves only an owner/computer-scoped
  `texture:<doc_id>` shape. It does not prove requester, document, trajectory,
  or work binding. The store-side `QueueLifecycleUpdate` similarly requires a
  lifecycle Texture target but does not prove that target is the Texture bound
  to the named trajectory.
- `QueueLifecycleUpdate` is a sound producer-report reducer: its optional work
  consequence belongs to the reporting producer. It must not be overloaded for
  downward control. The packet type has one ambiguous `WorkItemID` and no
  direction or `target_work_item_id`.
- Lifecycle packets already share the object-graph worker-update kind, events,
  snapshots, receipts, and `ListPendingLifecycleUpdates`. Legacy readers filter
  them by `LifecycleVersion`; no new store or mailbox is needed.
- The persistent Super reconciler reads only the pre-cutover mailbox. Generic
  warm injection can read lifecycle packets only after a lifecycle run exists,
  while the one persistent Super is intentionally non-lifecycle. No cold
  lifecycle opener/reconstructor exists for it.
- New lifecycle traffic can still be made legacy-shaped by omitting computer or
  trajectory identity. Natural-language owner revise currently does exactly
  that through `DispatchWorkerUpdate`.

### Texture turn and canonical document

- `prepareTextureRevisionV2` is the structured-document normalization authority:
  it validates `BodyDoc`, derives readable `Content`, normalizes source entities,
  and rejects a conflicting supplied projection.
- Unbound `CreateRevision` and `CommitLifecycleArtifactHead` call that
  authority. `StartLifecycle` and `ApplyLifecycleUpdateWithSourceGraph` do not.
  Lifecycle v0 and agent-written lifecycle revisions can therefore persist an
  empty or disagreeing readable projection.
- `commitTextureToolEdit` is still a two-transaction turn: it first queues a
  synthetic Texture-self packet and then applies it with the revision and source
  graph. Audit, mutation/checkpoint projection, delivery marking, events, and
  publication follow in additional failure domains.
- The existing `applyLifecycleUpdate` conditional object-graph batch is the
  replacement to connect. It already fate-shares head CAS, source objects and
  refs, primary/related inbound dispositions, producer work consequences,
  events, and a command receipt. The dead standalone `writeTextureSourceGraph`
  path has no production caller and is not a repair target.
- There is no durable `revision | no_change | wait | block` Texture-turn outcome,
  no ordered outbound-control set in the replay digest, and no atomic owner
  correction cursor/obligation.
- Direct owner revisions are canonical but do not atomically record correction
  causality. `/revise` can write a pre-cutover packet and identical later
  corrections can dedupe forever because the old stable ID omits an occurrence
  identity.
- Public structured revisions and initial lifecycle sources do not always
  derive/persist source graph objects and refs. Exact source open identity is
  not universally available.
- Go and browser readable projections disagree for expanded source refs. The
  stream is an in-memory event-bus SSE projection with a snapshot/subscribe race,
  no durable cursor, and no restart replay. The existing lifecycle event pager
  is the replacement to project through the Texture API and CLI.

### Persistent Super, CoSuper, and capsules

- The current Super-to-CoSuper design is a generic two-slot SQL side authority
  (`implementation`, `verifier`) with a hard cap of two. It is not the
  Definition's many assignment-specific, lifecycle-bound model.
- Arbitrary Super-profile runs can reach generic CoSuper spawn; the path is not
  restricted to exact `super:<owner>`.
- The Linux capsule substrate has strong per-run opaque capabilities, overlay
  isolation, broker verb checks, network namespace, seccomp, Landlock, receipts,
  and freeze behavior. It also allows multiple grants into one capsule and has
  no assignment-specific cancellation/restart authority.
- Production `internal/sandbox/run.go` never constructs a capsule executor or
  calls `WithCapsuleExecutor`. Deployed role registries therefore omit capsule
  tools entirely. This is a connection problem, not proof that capsules work in
  the product path.
- The current verifier path is the opposite of the owner-ratified boundary: it
  has no writable capsule, while its shared CoSuper registry exposes
  `record_self_development_verification`, computer-event append, updater-root
  mutation, and effect-proposal/finalization authority. Those powers must be
  removed before any production capsule wiring.
- Cancellation persists run/trajectory truth and lifecycle late packets safely,
  but it does not revoke a capsule capability, freeze/destroy the capsule, or
  record assignment-attempt-specific late disposition.

### Registry and deletion hygiene

The promoted Definition's owner-ratification receipt cited the deleted path
`docs/problems/texture-lifecycle-dual-tape-authority-2026-08-03.md`. The
Definition itself and this receipt are the live problem authorities. The stale
citer must be repaired in the next Definition update; the deleted document must
not be restored.

## Existing replacements to connect

1. `applyLifecycleUpdate`, `commitLifecycleTransition`,
   `lifecycleSourceGraphBatch`, and `PutBatchConditional` for one lifecycle and
   canonical-document transition.
2. `ListPendingLifecycleUpdates` plus lifecycle snapshot/event replay for all
   new direction-specific packets.
3. Texture-owned cold/warm reconstruction for lifecycle Texture and generic
   work-item reconstruction for Researcher; extend these rather than adding a
   poller.
4. Exact `persistentSuperAgentID(owner)` and the one persistent agent record;
   add a lifecycle control reader without promoting Super to a lifecycle class.
5. `prepareTextureRevisionV2` for every canonical structured revision.
6. `ListLifecycleEventPage` for durable, resumable public Texture observation.
7. The capsule broker/executor isolation substrate, after self-development
   effect tools are separated from capsule-local work and production lifecycle
   fate-sharing is explicit.

## Frozen Implement A–C schema boundary

The first runtime candidate joins addressing, direction-specific lifecycle
control, and the atomic Texture turn. It must not expose a temporarily unsafe
Texture tool or an independently committing control call.

### Static policy

Rename spawn policy explicitly (`AllowedSpawnTargets`, `CanSpawn`) and add a
coarse `AllowedMessageTargets` / `CanMessage` capability matrix. Static policy
is only a registry gate; it never substitutes for durable binding checks.

| Caller profile | Spawn targets | Message target profiles |
|---|---|---|
| Texture | Researcher | Researcher, exact persistent Super |
| Researcher | none | requesting Texture |
| Super | Researcher, CoSuper | requesting Texture and its own bound children |
| CoSuper | none | owning Super/Texture result path |
| Processor/Reconciler | Texture | requesting Texture |
| Conductor/Email | existing spawn behavior only | none |

### Packet/work direction

Extend lifecycle packet projection additively with:

- `direction = producer_report | target_control` for lifecycle packets;
- `producer_work_item_id` for the reporting caller's obligation;
- `target_work_item_id` for the addressed actor's obligation; and
- a runtime-derived control/update identity plus ordered payload digest.

Legacy packets may decode an empty direction. Every new lifecycle enqueue must
set one direction. `QueueLifecycleUpdate` remains producer-report-only, accepts
only `producer_work_item_id`, and preserves its replay fixtures. A target control
accepts only `target_work_item_id`; its sender cannot complete or refuse that
work.

### One committing Texture-turn command

Add one `ApplyTextureTurn` request/receipt to the existing lifecycle reducer. It
carries runtime-derived owner, computer, caller Texture agent/run, document and
trajectory; expected lifecycle version and document head; ordered inbound
packet dispositions; exactly one outcome (`revision`, `no_semantic_change`,
`wait`, or `block`); optional canonical structured revision plus source graph;
and zero or more ordered target controls.

Each control carries exact target agent, exact target work, control identity,
normalized packet, content/payload digest, and either:

- continuation of an existing open Researcher or Super target work; or
- an exact persistent-Super opener whose new work record and first
  `execution_request` are part of the same batch.

The expected head, outcome, inbound dispositions, target-work open/reuse,
control objects, source objects/refs, events, and command receipt commit in one
conditional batch. Ordered controls and target-work bindings participate in the
command digest. Equal retry replays. Changed order, payload, target, work, head,
or outcome conflicts. Wake occurs only after commit. There is no separately
registered generic Texture `update_coagent` and no independent public
`QueueLifecycleControl` mutation.

### Store-side target validator

The reducer, not only the tool, must prove:

1. caller run is a lifecycle Texture activation for exact
   `texture:<trajectory doc_id>`;
2. owner, computer, trajectory, document, caller agent, and expected head all
   match canonical objects;
3. an existing Researcher target is a successfully loaded same-scope
   Researcher agent with an open same-trajectory work item assigned to it and
   provenance binding it to the caller Texture;
4. a Super target is exactly `super:<owner>`, successfully loaded in the same
   computer, has profile/role Super, remains non-lifecycle, and owns the named
   open target work; the opener may atomically create only that exact work;
5. the static message matrix permits the role pair; and
6. every lookup error refuses. Channel shape, caller fields, prompt text,
   metadata fallback, or an inferred latest run never grants authority.

### Negative matrix

Tests must refuse, before durable mutation or wake:

- missing lookup and injected lookup errors;
- cross-owner, cross-computer, cross-document, and cross-trajectory targets;
- non-lifecycle Texture caller or caller run/agent mismatch;
- stale head or lifecycle version;
- foreign/unassigned/terminal producer or target work;
- arbitrary Super ID, Super with wrong owner/computer/profile/role, or generic
  lifecycle-Super promotion;
- every direct CoSuper target and every skip-level Texture-to-CoSuper call;
- Researcher not requested by this Texture, or only channel-shaped binding;
- direction/work-field ambiguity, sender attempt to settle target work,
  duplicate identity with changed payload, and reordered control replay;
- new control appearance in legacy mailbox/cursor readers; and
- wake before a successful conditional commit.

Positive tests must prove exact bound Researcher continuation, exact
persistent-Super open/reuse, more than one ordered control, equal replay,
restart reconstruction, and no legacy mailbox write.

## Later frozen findings

Implement D must first split all self-development/event/updater/effect tools
from capsule-local CoSuper work, then add assignment-specific durable authority,
production executor wiring, writable networkless verification capsules,
immutable-subject result identities, and cancellation/restart fate-sharing.
Implement E must project canonical version/source identity and the lifecycle
event cursor through the existing Texture API and `choir texture` CLI. Neither
slice may create a supervision service, second tape, or product-internal test
route.

## Baseline evidence

Before mutation, focused `internal/agentprofile`, lifecycle store, and relevant
agentcore update/restart tests passed in the repository's native environment.
The runtime shard wrapper selected its normal shard set rather than honoring the
attempted narrow filter, so this is a broad local baseline signal, not product
acceptance. No code, runtime configuration, external effect, or staging state
was changed by this inventory.

## Registry pre-flight failure discovered after the inventory

The first live `scripts/doccheck` probe after writing this receipt failed before
considering the new file: `docs/mission-graph.yaml` has one working product
entrypoint, while the authority manifest reports zero current product Definition
roots. The active Definition's manifest entry has `is_root: []` even though it is
the one owner-ratified executable product mission. The three navigation surfaces
therefore disagree about executable topology. This is a registry-hygiene defect,
not an implementation blocker to guess around. This dated problem update lands
before the manifest repair.

## Next safe action

Update the active Definition's sole `now` card to reference this landed receipt,
repair its deleted problem citer and the manifest root mismatch, reconcile exact
source/deploy/worktree identity, rerun live doccheck, then build and test one
frozen A–C candidate. Do not register the Texture affordance until store-side
validation, atomic turn, readers, replay, and the negative matrix are all present
in that same candidate.


## Joined repair integration findings — 2026-08-08

The first joined capsule-security, upward-report, and Texture-durability candidate
exposed three integration defects before publication:

1. a late CoSuper report on a still-live trajectory added the same trajectory
   object condition through both the live reducer transition and the historical
   authority join, so Dolt rejected the conditional batch as a duplicate rather
   than persisting evidence;
2. historical late-evidence authority still required the persistent Super's
   mutable `ActiveRunID` to name the original parent run, which would reject an
   authenticated delayed result after the persistent Super had legitimately
   activated a later control; and
3. four broad agentcore fixtures still exercised the superseded generic,
   unassigned CoSuper `update_coagent`/rewarm path. The exact closed registry now
   refuses that path, as required; those fixtures must either seed legacy rows
   directly when testing compatibility or use an ordinary Researcher for generic
   restart coverage.

Evidence was the joined focused Store failure
`TestPartialAssignmentReportPreservesActiveCapsuleThenTerminalFreezes`
(`objectgraph dolt: duplicate condition ... choir.trajectory`) and runtime-shard
failures naming `tool "update_coagent" not found` or `unassigned CoSuper cannot
execute`. These are not reasons to restore generic CoSuper authority.

The repair direction is substrate-level: CAS a live trajectory exactly once;
for post-terminal evidence condition the immutable historical trajectory while
allowing the static persistent-Super agent to point at a newer active run; keep
the original parent run/control/work identity as the authentication join; and
remove stale generic-CoSuper test setup. Late reports remain report/evidence
only—no candidate, packet, wake, projection, reopen, or semantic revision.


## Independent joined lifecycle review blockers — 2026-08-08

A fresh read-only review of joined candidate `59d4fc6e6eb6a1404aa232dbf3ea75defc1f3bf7`
returned **REPAIR** despite the green joined suites. It found:

- persistent-Super reports are authenticated against an open Texture target work
  and a nonterminal Super run before `QueueLifecycleUpdate` reaches its
  terminal-trajectory late branch; after trajectory cancellation cancels those
  works, a real delayed Super result is refused instead of durably recorded as
  evidence-only;
- a direct owner edit advances the head and queues its new-head correction but
  neither consumes nor explicitly disposes already-pending owner occurrences on
  the old head, while runtime injection and the reducer read only current-head
  occurrences; the old instruction can remain pending forever;
- delivered persistent-Super controls/reports are read oldest-first with
  `limit=100` and no cursor, so occurrence 101 can remain permanently invisible,
  and `report_to_texture` can select a stale control; and
- runtime injection marks occurrence IDs in mutable run state before the
  returned packet is durably appended to run memory. An append failure after
  delivery binding can lose the only executable projection of that control.

The review also found that one revision incorporating multiple inbound packets
can project several public `version` events for the same immutable revision,
because both per-update apply events and the turn-commit event carry the new
revision refs and timestamp. One canonical revision must have one version
projection; the other events remain typed cursor transitions.

These are protected lifecycle/delivery/observation defects, not test gaps to
waive. The candidate remains unpublished and effects remain OFF. Repairs must
preserve exact historical authority for late evidence, atomically disposition
old-head owner occurrences during direct edit, page or unboundedly read the
complete exact-run delivery set, derive injection dedupe only from durable run
memory, and designate exactly one event kind as the version projection.


The parallel capsule-security review also returned **REPAIR** with two high
source defects:

- generic run or trajectory cancellation terminalizes lifecycle run/work truth
  but does not durably transition the bound CoSuper assignment through revoke
  intent, executor destruction/inspection, acknowledgement, and assignment
  cancellation. An active capsule/handle is then treated as healthy by restart
  reconciliation, and the overlay does not rejoin live run/work/trajectory
  state; writable authority can outlive the cancelled product obligation; and
- a real terminal assignment result racing cancellation and citing actual prior
  execution receipts reaches the evidence-only branch, but terminal command
  validation still requires granted receipts minted from a frozen live capsule.
  Because cancellation has revoked/destroyed it, the late result is rejected;
  only a command-free narration can currently persist.

Generic cancellation must delegate exact assignment fate before terminal
projection (or atomically record the assignment revoke intent that reconciliation
must finish), and overlay execution must revalidate live obligation state. Late
reports must authenticate persisted raw executor receipts and retain them as
non-granting evidence without pretending they certify a frozen final subject.
No late result may wake, reopen, create a candidate, or gain a verification Pass.
Real Linux namespace/seccomp/Landlock behavior remains an acceptance evidence
gap rather than a waiver for these source defects.


## Persistent-Super delivery settlement gap — 2026-08-08

Follow-up source inspection of the rejected candidate found one additional
single-authority defect. `BindLifecycleControlDelivery` replaces the persistent
run's `lifecycle_control_bindings` metadata with only the current bind batch,
which invalidates older exact assignment/control joins when the same resident
Super receives a later continuation. More fundamentally, delivered controls and
CoSuper return packets remain `UpdatePending`: `report_to_texture` queues the
Super's upward report but atomically dispositions none of the exact packets that
the Super actually consumed. Those target-Super packets can therefore block
trajectory settlement forever even though durable run memory proves they entered
the actor context.

The repair must append/deduplicate immutable run control bindings rather than
replace them, and the canonical Super-to-Texture report command must atomically
disposition the complete exact-run packet occurrence set proven present in
authenticated durable run memory. It may not trust model-authored IDs, dispose a
packet absent from memory, or create another cursor/tape. Replay, continuation,
partial progress, cancellation, and more than 100 packets must retain exact
control/work authority.


## Delivered-run restart recovery gap — 2026-08-08

Repair inspection found that deriving injection deduplication from durable memory
is necessary but not sufficient. A runtime-memory append error still flows into
generic execution failure/terminalization, and the added unit probe only calls a
fresh injector directly; it does not prove the actor/runtime transition. On
process restart, generic boot recovery passivates a running Researcher, but the
assigned-work reconciler creates a new run and cannot rebind controls already
durably delivered to the old exact run. Only persistent Super currently has an
exact delivered-run reactivation path. Thus an append failure or restart can
still strand a Researcher control even when mutable seen state is correct.

The repair must keep an append failure nonterminal and retryable, and must
reactivate the same exact passivated Researcher run when its authenticated
pending delivered-control set and open work/trajectory authority remain valid.
It must not rebind the occurrence to a new run, dispatch before a durable state
transition, or generalize restart authority to unrelated roles. Acceptance must
exercise the actor/runtime path, not only invoke an injector closure.


## Cancellation-intent/report race — 2026-08-08

Joined source review of the durable cancellation repair found that the intent
currently gates ordinary lifecycle transitions but explicitly permits every
`record_co_super_assignment` command. A terminal assignment report racing after
the durable trajectory cancellation intent but before the first capsule revoke
transition can therefore still freeze executor state, derive a candidate, mark
Pass, queue an upward packet, and wake the parent. The cancellation intent has
already made cancellation authoritative; report completion must not win this
window.

Both runtime and Store must treat the existence of the exact trajectory
cancellation intent as late/evidence-only authority. The runtime must skip
freeze/candidate/granted-receipt effects and retain only authenticated raw
receipts; the Store must independently demote Pass, strip candidate/mutation
authority, and emit no packet/wake/projection/reopen. Per-tool capsule
revalidation must also reject further read/write/exec effects once the intent is
durable, while the report tool remains available solely to record delayed
evidence. A deterministic report-versus-intent race test is required.


## Joined capsule restart and drain blockers — 2026-08-08

Fresh capsule/security review of the joined cancellation candidate found three
additional protected-surface gaps:

1. A restarted Linux capsule executor reconstructs empty in-memory maps.
   `HasCapsule` and the structured revocation receipt can consequently declare
   `CapsuleAbsent` without inspecting or cleaning a surviving state directory,
   overlay mount, or `/sys/fs/cgroup/capsule/<id>` membership. `Pdeathsig` is not
   proof of host resource cleanup. Restart reconciliation must kill/delete the
   exact orphan cgroup, detach the exact overlay, remove the private capsule
   state tree, verify all three are absent, and only then sign the acknowledgement.
   Receipt creation itself must independently refuse visible residue.
2. Compatibility permits an old assignment binding with an empty
   `ExecutionHandleDigest`. That may permit closure/replay, but a delayed report
   containing command receipts has no exact handle identity against which to
   authenticate raw executor receipts. Receipt-bearing late evidence must refuse
   without the stored domain-separated execution-handle digest; command-free
   narration may remain evidence.
3. The lifecycle reducer terminalizes run records before runtime drain queries.
   The drain currently queries active records only and excludes a cancelled
   snapshot activation, so an actually resident provider context can remain in
   `Runtime.running` without receiving actor cancellation. Draining must join
   all exact lifecycle trajectory run records with the process-local resident
   activation set, signal only that intersection (never start a historical
   terminal actor), and remove/cancel each resident context after durable
   cancellation.

These are implementation blockers. Real Linux restart cleanup and a real
resident-provider cancellation/late-return sequence remain required acceptance
evidence; source or Darwin simulation cannot promote effects.


## Capsule identity path safety — 2026-08-08

Review of restart cleanup found that the existing basename-only capsule identity
check admits `.` and `/`-shaped identities. A cleanup API must never permit an
identity to collapse to the executor state root or cgroup parent. Assignment
binding, spawn, cleanup, and receipt authorities must require the same canonical
single path component: nonempty, trimmed, not `.`/`..`, and containing no slash
or backslash. Negative tests must prove refusal before filesystem or cgroup
mutation.


## Cancelled provider late-tool closure — 2026-08-08

The repaired resident drain now correctly cancels the provider context, but a
provider that returns an already in-flight terminal `record_assignment_result`
after observing cancellation still passes through tool-loop message persistence
and execution using that cancelled context. It can therefore fail before the
late-evidence reducer despite durable raw executor receipts.

Only the runtime-authenticated, sole terminal assignment-report call may receive
a bounded `context.WithoutCancel` closure after the provider response has fixed
the run and tool-call identity. Assistant/tool-result memory and the exact report
commit may use that closure; capsule read/write/exec, partial reports, mixed
batches, injected turns, and further provider calls must remain cancelled. A
successful closure stops the tool loop without completing or reopening the
cancelled run. Deterministic proof must block a provider, cancel it, return the
terminal tool call, and retain evidence while ordinary mutation remains denied.


## First pushed candidate CI failures — 2026-08-08

Pushed candidate `76e9cd7716aa069b303b4a524784261bb7cabd86`
entered race-selected CI run
[`31257297770`](https://github.com/choir-hip/go-choir/actions/runs/31257297770)
and failed before deploy. This is a new landing blocker and is recorded before
repair:

- `go vet` found a possible detached terminal-closure context leak in
  `internal/toolregistry/toolloop.go`; the cancel function is not mechanically
  discharged on every path.
- The real Linux capsule revocation test failed. Its exact assertion/log must be
  inspected and the source or test repaired from Linux evidence, not Darwin
  assumptions.
- Two `internal/actorruntime` race fixtures now observe lifecycle producer runs
  and canonical Texture reactivation in their active-run queries, contradicting
  their older fixture expectations. The tests must distinguish the intended
  exact activation rather than treating every trajectory run as the Texture
  activation; any underlying product misprojection must be repaired instead.
- The isolated Store race package hit Go's default ten-minute package timeout at
  `TestApplyTextureTurnConsumesComplete101OwnerOccurrenceSet`. Full Store already
  takes roughly 6.5 minutes without Race on this candidate. The CI shard gives
  the Store its own job but does not raise the default package timeout, making
  the selected red-candidate race lane structurally incapable of completing.
  Preserve Store isolation and set a bounded timeout below the 30-minute job,
  rather than skipping Race or deleting coverage.

Deploy and acceptance remain blocked; effects remain OFF.


## Staging immutable-route acceptance blocker — 2026-08-08

Candidate `99fc3e6b7bf151ddad1f0927ca18a24ba5275d10` passed race-selected
CI run [`31257971088`](https://github.com/choir-hip/go-choir/actions/runs/31257971088),
and the deployment receipt reports the exact sandbox and active-computer build
at that SHA. The authenticated product path nevertheless refuses the new
lifecycle create route with HTTP 404 `texture endpoint not found`.

The authenticated current computer is `computer-03335285269bdba4f94377e56879f9e6`
at realization epoch 130. `/api/compute/status` joins that computer to immutable
code commit `7122f2799be4458f4b925be11990321c7e70ffc4`, not the reviewed candidate.
The deployment log explains the discrepancy: it preserved the constructed
`candidate-fleet-e15cb89f25d963c220319b7b` computer and refreshed a different
active user's computer to `99fc3e6b`. Thus host health and the published
activation receipt are not sufficient evidence that the owner-scoped stable
computer serving this Definition runs the accepted source.

This blocks all product-path continuous-supervision evidence, including real
Linux capsule cleanup. It must not be bypassed with SSH, a test route, or a new
candidate/worker computer, and it does not authorize an unreviewed promotion or
route mutation. The next safe action must identify the existing constructed
computer's authority and either perform the standard reviewed route/promotion
transition with rollback and exact identity receipts, or obtain a separately
authorized product acceptance environment whose immutable computer identity is
joined to `99fc3e6b`. Effects remain OFF and the registries remain open.


## Lifecycle Texture create does not activate initial work — 2026-08-08

An already-authorized staging acceptance account was recovered through its
existing authenticated browser session. A nonce-bound execution-identity join
proved host, guest sandbox, deployment receipt, VM epoch 8245, and platform
attestation all bind exact candidate
`99fc3e6b7bf151ddad1f0927ca18a24ba5275d10`. This resolves the earlier lack of
an exact-candidate product environment without mutating the preserved
constructed computer.

The first authenticated product request exposed a new source defect before any
repair. `POST /api/texture/lifecycle-documents` returned HTTP 201 and durably
created document `11902866-d32e-55c4-9483-d9bd47c91a6c`, revision
`d1a831ba-6af5-5206-aa03-49caf4b047dc`, trajectory
`8f3b6ac6-dbdf-5bfe-99f0-661961c64f3d`, and initial Texture-assigned work
`74fa5e0f-92ee-5e3a-ac8f-0c4b8f044e4c`. For more than sixty seconds the
snapshot remained reducer sequence 1 with exactly the start event, the work
open, no updates, no later revision, and the Texture activation passivated. No
provider/run/actor activity followed.

The create handler calls `ReconcileAgentWake`, but that reconciler starts a new
Texture run only when a pending lifecycle update or owner-instruction occurrence
exists. `StartLifecycle` durably creates the initial assigned work and v0
revision without either occurrence, so the successful new start commit has no
executable projection. This violates the Definition's prompt-bar/create entry
point and wake-after-commit contract. The repair must treat exact open initial
work assigned to the lifecycle Texture as a durable wake reason, preserve
concurrent replay/idempotency, bind the new run to that exact work and current
head, and recover it after a crash between start commit and dispatch. It must not
invent a second queue, enqueue a synthetic legacy packet, dispatch before the
start commit, or wake an unrelated/terminal/non-lifecycle document.

Effects remain OFF. The created trajectory is retained as failure evidence and
must be cancelled through the public lifecycle authority after repair or final
acceptance; it must not be silently deleted.


## Concurrent initial-work reconciliation stales the winner — 2026-08-08

Independent review rejected local repair candidate
`572b49fa793ab8128254142a9f0b1172654d86f0` with a reproducible high race.
Two `ReconcileAgentWake` callers may both pass the initial active-run check.
Caller A can create the sole valid Texture activation and its pending mutation;
caller B then enters the generic pending-mutation cleanup and marks A's now-bound
mutation stale before caller B's own activation projection loses. Store CAS still
prevents a second durable activation, but the sole dispatched winner subsequently
fails `ValidateActivationAuthority`, leaving the initial work open and again
without an executable projection.

The reviewer reproduced the race with a 40-way barrier in all ten trials: one
stored run/dispatch, many losing reconcile errors, and invalid authority for the
winner every time. Repair must never stale a mutation merely because this caller
has not yet observed an active run. Cleanup may stale only an exact mutation
proven unbound after re-reading the agent/run authority, or the reconciler must
reuse the concurrently installed valid activation. A concurrent regression must
prove one authoritative winner, no winner-staling, no duplicate dispatch, and
safe loser/replay behavior.


## Global Texture wake serialization blocks unrelated documents — 2026-08-08

Independent review rejected repair `015a11048d0ed01ff2376177a1dcc865328e961f`
with a second high liveness finding. Its single Handler-wide wake mutex is held
through the entire reconcile and run-construction path. Run construction imports
media sources sequentially and may wait up to thirty seconds per URL, without a
bounded URL count. One slow document can therefore block committed initial work,
owner instructions, coagent updates, and boot reconciliation for every other
owner and document on the computer.

Same-document initial-wake serialization is required, but cross-document
serialization is not. The lock must be keyed by the canonical
`{owner_id, computer_id, doc_id}` scope (or expensive work must occur outside a
narrow lock followed by exact authority revalidation). Lock lifecycle must not
race waiters or leak unboundedly. Regression proof must retain the 40-way
same-document winner-authority assertion and additionally prove a blocked wake
for one document does not delay an unrelated document's reconcile/dispatch.


## Exact-candidate provider credential failure — 2026-08-08

Candidate `ac6dd16b1980a1a3faedd7d1d83fefa79395a1ee` passed selected CI run
[`31261269488`](https://github.com/choir-hip/go-choir/actions/runs/31261269488)
and deployed successfully. A fresh nonce-bound execution identity joined the
same stable acceptance computer, VM epoch 8247, guest sandbox, host build, and
deployment receipt to exact `ac6dd16b`. Boot recovery then correctly created
Texture run `5ee276b3-d25c-41ac-afaa-5879a6ea5ecf` for the previously stranded
initial work, proving the repaired committed-start projection executed after a
no-SSH deployment restart.

The provider call failed before iteration zero with ChatGPT HTTP 401
`refresh_token_reused` ("Your refresh token has already been used to generate a
new access token. Please try signing in again."). The run became failed, the
initial work remained open, and no revision or downward control was produced.
The acceptance computer's durable model policy pins Texture and Super to
ChatGPT `gpt-5.5` and Researcher to ChatGPT `gpt-5.4-mini`; CoSuper is pinned to
DeepSeek. This is real gateway-credential/product evidence, not a local failure
or permission to forge auth.

Provider routing and credentials are protected. Any remediation must use an
owner-visible product configuration or the canonical gateway credential-renewal
authority, record before/after policy and key identity without exposing secrets,
and preserve rollback. It must not edit Node B through SSH, inject a token into
the guest, weaken auth, silently fall back across policy, or claim the failed
run as supervision evidence. Effects remain OFF.


### Authorized alternate-route probes and public cancellation

The owner-visible product configuration path was then exercised without changing
source, route, computer realization, or gateway credentials. Before mutation,
`System/model-policy.toml` had SHA-256
`7192b8b1600561a331fda32f27628296c3f5b9bd1ba30dd5fb82681985c45e2a`.
Temporary exact-role selections were written through authenticated
`PUT /api/files/System/model-policy.toml`, checked through
`GET /api/model-policy/resolve`, and rolled back through that same public file
surface. The exact original bytes and SHA-256 were restored after the probes.
No secret entered the guest policy file.

The available alternate routes did not provide an accepting multi-turn tool
loop:

- DeepSeek-selected Texture run `1666bedd-bbab-435b-be56-e6abe761f6a1`
  retained DeepSeek metadata but exhausted its configured provider-precondition
  fallbacks and terminated at iteration 2 on the same ChatGPT 401.
- Fireworks legacy-model run `0ef1b196-87b3-43b0-89dd-3d35f230e094`
  terminated at iteration 3 on the same terminal ChatGPT fallback; the deployed
  Kimi selection was separately tried with and without explicit reasoning as
  runs `d6df68a5-c72c-4901-ad83-af5624844459` and
  `58e9c17d-cdf9-42c3-bb4d-738186ec1bef`, each terminating at iteration 1 on
  the same ChatGPT credential failure.
- Z.AI-selected run `db8164af-2709-44a3-b5a9-7a37ad6df9ea` was durably blocked
  by an upstream HTTP 429 before iteration zero.
- Bedrock-selected run `0e2b8082-fc68-49dd-abc5-64dc6628db08` failed closed as
  an unsupported deployed provider.

None of these runs created a Researcher, lifecycle update, Texture revision,
source, Super direction, capsule, or effect. They are availability evidence,
not successful supervision evidence. Silent policy fallback is therefore not a
valid closure, and further owner-policy permutations cannot repair a stale
server-owned ChatGPT refresh token or exhausted/unavailable provider capacity.
The remaining remediation authority is the canonical host-side gateway
credential/provider operator path; this Definition still forbids SSH, guest
credential injection, auth weakening, or an unreviewed source fallback.

The retained failed trajectory was then closed through the authenticated public
lifecycle command, not deleted. Command
`public-cancel:cts-failed-acceptance-cancel-ac6dd16b-v7` conditionally matched
lifecycle version 7 and head
`d1a831ba-6af5-5206-aa03-49caf4b047dc`; the returned snapshot records trajectory
`8f3b6ac6-dbdf-5bfe-99f0-661961c64f3d` and its initial work as `cancelled`,
terminal head unchanged, lifecycle version/reducer sequence 9, and activation
`58e9c17d-cdf9-42c3-bb4d-738186ec1bef` cancelled. The temporary acceptance API
key was revoked through `DELETE /auth/api-keys/{id}` (HTTP 204), and a subsequent
authenticated probe returned HTTP 401. Effects remain OFF. Exact-candidate
identity, create-route availability, initial-work restart projection, public
owner instructions, model-policy rollback, and public cancellation are now
proved; the repeated supervision/capsule/no-effect acceptance remains blocked
on protected server-side provider availability.


### Post-escalation recurrence and structural assessment

A later continuation did not assume that the external credential/capacity state
was unchanged. Exact product identity was reverified at deployed
`ac6dd16b1980a1a3faedd7d1d83fefa79395a1ee`, VM epoch 8247, and the original
model-policy SHA-256 was still
`7192b8b1600561a331fda32f27628296c3f5b9bd1ba30dd5fb82681985c45e2a`.
A fresh ChatGPT-policy trajectory
`41cec88f-510f-53cc-a5e7-84c372b5421b` created document
`7e5357f6-2a3f-535e-a948-89c8f4cf20fd`, then Texture run
`dcf2088c-836e-47db-8173-80f0adb7dcf3` failed at iteration zero with the same
HTTP 401 `refresh_token_reused`. It was conditionally cancelled through the
public lifecycle endpoint; the returned trajectory lifecycle version is 3.

A cooldown-aware Z.AI retry then used the authenticated owner-visible policy
surface and fresh trajectory `aca3504c-2ae0-5a4e-bab5-b22541e90585` with
document `a6455ce7-cda1-5bdb-8d9b-d52af06bd519`. Texture run
`f0d0e6ea-f98b-484c-9630-b6c849279118` failed at iteration zero because the
Z.AI gateway circuit was open as upstream unhealthy. This is stronger
availability evidence than the earlier single HTTP 429: waiting did not restore
the provider path. The trajectory was publicly cancelled at lifecycle version
3, the exact original policy bytes were restored again, and both temporary API
keys were revoked and rejected on subsequent use.

The dependency graph now converges rather than suggesting another runtime
patch: lifecycle create and committed-start wake → exact Texture actor → guest
gateway client → host gateway provider registry → server-owned provider
credential/capacity. Every component through the host gateway boundary is
proved on the exact product path; failure occurs at protected upstream provider
auth/health. Computer-owned model policy controls selection but cannot author or
renew server-owned credentials. Repository inspection found no scoped product
API or `choir` CLI authority for provider credential renewal. The tracked
`nix/deploy-provider-creds.sh` and the one-time recovery workflow are SSH-shaped
operator paths, which standing question 9 classifies as break-glass rather than
product operability and which this Definition explicitly forbids for acceptance.

After more than three provider permutations and two fresh post-deployment
trajectories, another incremental runtime/policy probe is not convergence. No such scoped
non-SSH replacement was found in the inspected provider, gateway, auth, CLI,
script, and workflow surfaces. The next transition must
be an explicit human/operator restoration of the canonical host gateway
credential or provider account capacity, or a separately ratified red mission
to create an auditable scoped no-SSH renewal authority. This active Definition
cannot silently expand into auth/gateway administration. Effects remain OFF;
all fresh failure artifacts are retained cancelled rather than deleted.


### Sanitized provider-account diagnostics

After staging had already identified the failed gateway boundary, a local
operator-side reproduction used the repository's gitignored provider credentials
to distinguish routing defects from provider-account availability. No credential,
response body, account identity, or token was printed or committed, and these
local calls are diagnostic only—not staging acceptance evidence. Minimal
no-tool requests to the same configured provider/model selections returned:

- DeepSeek `deepseek-v4-flash`: HTTP 402 with an error message classified by the
  adapter probe as balance-related;
- Xiaomi `mimo-v2.5`: HTTP 402, type `insufficient_balance`, code `402`;
- Fireworks `accounts/fireworks/models/kimi-k2p6`: HTTP 412, code
  `PRECONDITION_FAILED`;
- Z.AI `glm-5.2`: HTTP 429, type `rate_limit_error`, code `1113`, with a
  balance-related message classification; and
- AWS Bedrock `us.anthropic.claude-haiku-4-5-20251001-v1:0` in `us-east-1`: a
  direct local bearer invoke returned HTTP 403. No response body was retained or exposed.

This independently reproduces why model-policy permutation cannot restore the
acceptance loop: every configured provider/model route failed before tool
semantics. Account/balance attribution applies to DeepSeek/Xiaomi and the
qualified Z.AI classification; Fireworks proves only a precondition failure,
and Bedrock proves only that this local bearer/model/region combination was
forbidden. Neither local result establishes the host credential state.
Local Codex token metadata reports an unexpired expiry and `codex login status`
reports a ChatGPT login, but neither proves usable auth or supplies a proven or
admissible transfer/renewal authority. Provider-account restoration and safe,
auditable host credential renewal remain equal possible remedies; neither is a
lifecycle patch.

The operator-authority inventory also converged. Repository Actions secrets name
only the Node B SSH host/key; there are no repository environment secrets or
variables and no provider credential secret consumed by deployment. The only
tracked provider credential deployment script copies credentials over SSH, and
the one-time recovery workflow is likewise SSH-based. No credential value was
uploaded or changed during this inspection. A separately ratified authority
must either create a scoped no-SSH renewal path or perform external provider
account restoration before this Definition can resume. Effects remain OFF.


## No bounded current-main-conformant deployed rollback rehearsal — 2026-08-08

The local source-only rollback is reconstructable: reversing the 99 changed
non-document paths from exact candidate `ac6dd16b` returns them byte-for-byte to
rollback ref `cdaa787b`, and the sequential runtime shards, focused packages,
vet, and diff checks pass. That does not make an old GitHub Actions rerun a safe
deployed rollback path.

An independent red-team inspected historical successful deployment run
`31030833230` at `460c142394e12b6e307949d0180da08d1b058745`. Its deployed
runtime tree is the one underlying `cdaa787b` (the only non-document difference
through `cdaa787b` is the non-deployed receipt linter). Re-running the whole
historical workflow would, however:

- publish the historical rolling Flake through a mutable external action as a
  sibling of deployment;
- write a receipt whose preserved event says `refs/heads/main@460c…` while
  current `origin/main` is `e50b5644…`, violating current-main source truth;
- rebuild/switch the complete Node B closure, restart gateway, sandbox, proxy,
  vmctl, and sourcecycled, then refresh every mutable active interactive
  computer rather than only the acceptance scope;
- require another normal CI/build interval before exact-`ac6dd16b` recovery if
  the older host switch or guest refresh fails; and
- start pre-cancellation-intent, pre-owner-instruction, pre-Texture-turn/control-
  delivery, and pre-assignment code against the current persisted graph without
  a backward-read/restart compatibility receipt.

Fresh authenticated preflight reduced but did not eliminate those hazards. The
compute inventory contained one mutable active interactive computer (`primary`,
epoch 8247); all twelve durable runtime runs were terminal (four cancelled,
five failed, three completed); the retained acceptance trajectory was cancelled
at lifecycle/reducer/watermark 9 with exact v0 terminal head; self-development
was OFF generation 0 and model-policy bytes were exact. A scoped preflight key
was revoked and rejected after the decision. Zero active runs does not prove the
older runtime can safely open every newer object kind or bound the global host
and route effects.

A job-scoped deploy rerun avoids the independent rolling-Flake sibling, but
GitHub explicitly reruns the selected job including its dependencies. Recovery
therefore still has approximately normal CI-and-deploy latency, and dependency
outputs/artifacts plus current persisted-graph compatibility would require
attempt-specific proof. No deploy was triggered.

The safe error field is now explicit. A deployed rehearsal needs either (a) a
canonical revert through current `origin/main` followed by a separately reviewed
forward candidate, or (b) a purpose-built, authorized deploy-only rollback path
that cannot publish rolling routes, has an exclusive CI/change freeze, inventories
and verifies every affected computer, captures complete protected state, and can
restore an exact reviewed candidate within a bounded window. Before either path,
old-code startup/recovery must be proved against a reconstructable current graph
snapshot. Until that authority and evidence exist, the local source rehearsal is
the highest admissible rollback result. Effects remain OFF.


### Representative old-runtime/current-graph compatibility receipt

A disposable local cross-version probe has now reduced one part of the deployed
rollback error field without touching staging or production data. Exact candidate
`ac6dd16b` created and committed a representative terminal Dolt/object-graph
fixture through Store authority: a cancelled trajectory, two cancelled work
items and run projections, a consumed owner instruction, a Texture turn with two
revisions, one exact-run-bound control later cancelled with the trajectory, a
durable cancellation intent, eight lifecycle events, and seven command receipts.
The new-only object kinds remained present. Fixture Dolt HEAD was
`lvtb74ss94q6u8jpmtd32707oefj2pu5` with empty `dolt_status`.

A detached exact-`460c1423` runtime—the deployed runtime tree underlying
`cdaa787b`—then opened that same closed marker/workspace. Old lifecycle/scoped
document, revision, snapshot, work, update, and cancelled-run reads passed.
`Runtime.Start` executed with a counting actor-dispatch hook and emitted zero
dispatches. Dolt HEAD and clean status were unchanged before and after. Reopening
with `ac6dd16b` verified the cancellation intent, owner-instruction state, exact
control delivery binding/disposition, revisions, and all new-only object kinds
were intact. The old runtime's normal best-effort localhost Qdrant ensure failed
closed and did not alter Store state. Temporary probe sources and the detached
worktree were removed; session evidence remains under
`/tmp/choir-rollback-proof/`.

This is meaningful structural backward-startup evidence for the terminal object
classes produced or contemplated by the blocked acceptance, including a representative graph
with Texture-turn/control object kinds that staging never reached. It is not a byte-exact
production database copy, full server/actor-adapter proof, VM filesystem proof,
or deployed rollback. No sanctioned Store export/import path exists; the closed
marker plus derived `.texture` directory is the local transferable unit. The
result satisfies the specific representative compatibility mitigation only when
combined with a fresh terminal-only product preflight and independently bounded,
current-main-conformant route recovery. The global routing, rolling-Flake, and
recovery-window hazards identified above remain open.

## Canonical current-main rollback rehearsal and old-response identity ambiguity — 2026-08-08

The independently approved canonical two-leg rehearsal has now executed under one
exclusive owner. Rollback commit `10d4865958b7d8deaab5665f74b37dd1b5005070`
changed exactly the reviewed 99 non-document paths to byte-equivalence with
`cdaa787b`; current mission docs remained. GitHub Actions run `31267448310`
passed the selected Race/build/SBOM gates, published the canonical rolling Flake,
and deployed the new R identity. Nonce-bound product identity joined host, guest,
route, deployment receipt, platform attestation, stable VM
`vm-bbdbbd01c4390b7036067aaa12afeb68`, and guest
`computer-42850e9734d9442386c5dd8bf3afbf19` to exact R at epoch 8248.

The read-only midpoint preserved the exact durable-run digest (twelve terminal,
zero active), lifecycle version/head/work summaries, self-development OFF
generation 0, model-policy SHA-256
`7192b8b1600561a331fda32f27628296c3f5b9bd1ba30dd5fb82681985c45e2a`,
route digest, and the single active interactive-computer inventory. It also
revealed one response-identity ambiguity: R's trajectory representation omitted
the stored `request_id` field on each of six owner-instruction events. No event,
version, head, work, run, policy, or computer value changed, but the full response
digest therefore differed from the frozen baseline. The rehearsal treated that
omission as the predeclared identity-ambiguity abort condition and initiated F
immediately without another old-runtime probe. **Problem-documentation-first
safety exception:** the binding recovery gate required F to begin immediately;
waiting on old code to commit this receipt would have prolonged protected
exposure. F therefore necessarily preceded this first subsequent problem/evidence
commit. This receipt still precedes any compatibility fix; no fix is part of
this mission.

Forward commit `67a61358ceda55c30e9853907f85648bb8531bb8` applied the same
1,289,831-byte frozen patch forward. Its whole Git tree is exactly the pre-R
`2f8d912ed110680f5217dd36c006a96196334f7f` tree
(`d1a03e3e03f25d0ff201fd8d424b38549ccdb552`), so the 99 runtime paths are
byte-equivalent to `ac6dd16b` while current docs remain. GitHub Actions run
`31268477380` passed, published the canonical rolling Flake, and deployed exact F
at epoch 8249. R deploy completion was `17:02:43Z`; F deploy completion was
`17:34:32Z`, an old-runtime exposure of 1,909 seconds (31m49s), within the
45-minute bound.

Final authenticated product comparison restored exact full response digests for
all three retained cancelled trajectories and exactly matched the pre-R run,
policy, self-development, lifecycle, route, VM/guest, and computer-inventory
state; only the allowed deployment/build receipts and monotonic epochs changed.
The scoped key was revoked and returned HTTP 401. This closes the missing
current-main-conformant deployed rollback/forward receipt and proves the
fail-closed response-identity abort behavior. It does not repair the old response
schema or replace the still-blocked provider-dependent repeated supervision,
real capsule, late-result, positive correction/source-open, checkpoint, and run-
acceptance evidence.
