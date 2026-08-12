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
- Production `internal/autoputer/run.go` never constructs a capsule executor or
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
and the deployment receipt reports the exact autoputer and active-computer build
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
proved host, guest autoputer, deployment receipt, VM epoch 8245, and platform
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
same stable acceptance computer, VM epoch 8247, guest autoputer, host build, and
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
- AWS Bedrock in `us-east-1`: direct local bearer invokes returned HTTP 403 for
  both gateway seed `us.anthropic.claude-haiku-4-5-20251001-v1:0` and the exact
  owner-policy-selected `us.anthropic.claude-sonnet-4-6`. No response body was
  retained or exposed.

This independently reproduces why model-policy permutation cannot restore the
acceptance loop: every configured provider/model route failed before tool
semantics. Account/balance attribution applies to DeepSeek/Xiaomi and the
qualified Z.AI classification; Fireworks proves only a precondition failure,
and Bedrock proves only that these local bearer/model/region tuples were
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
- rebuild/switch the complete Node B closure, restart gateway, autoputer, proxy,
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

### Exact-F default-provider recurrence

After forward commit `67a61358ceda55c30e9853907f85648bb8531bb8`
was independently joined to the stable product at epoch 8249, the normal owner
web session was restored without bypassing authentication and minted a fresh
computer-scoped API key. That first key omitted `write:texture`; its attempted
generic document create was refused HTTP 403 `missing required scope:
write:texture` before mutation. The key was revoked and rejected post-revocation
with HTTP 401. A replacement key with the intended Texture-write scope was then
used. It first made the mistaken generic create described below, and the correct
lifecycle route subsequently started the unchanged continuous-prose acceptance
objective and produced document
`39eafb8c-11c6-5ecc-a8c9-aec323eaa67d`, revision
`79dc0bed-d71b-5a31-97d5-371c3c06d916`, trajectory
`e5f85464-560b-5383-b199-cf4c62c12145`, work
`8d62ca55-cae2-5f9a-95e5-83c0245b3fb1`, and exact Texture run
`74a5a20f-24a3-4b25-b11c-1072f881f8a9`.

The exact-F run selected the restored default policy `chatgpt/gpt-5.5` and failed
at iteration zero with the same HTTP 401 `refresh_token_reused` response before
any tool semantics. It created no Researcher, update, generated revision, source,
Super control, assignment, capsule, checkpoint, materialization, route, or
acceptance. The trajectory was retained and publicly cancelled—not deleted—at
lifecycle/reducer/watermark 3 with its exact v0 terminal head and work cancelled.
After cancellation the complete run inventory was thirteen terminal runs (five
cancelled, five failed, three completed) and zero active runs; self-development
remained OFF generation 0 and model-policy SHA-256 remained
`7192b8b1600561a331fda32f27628296c3f5b9bd1ba30dd5fb82681985c45e2a`.
Both temporary keys used during setup were revoked and rejected with HTTP 401.

A generic non-lifecycle document create was mistakenly called before the
lifecycle route and produced empty document
`457320df-e047-405c-b2a1-a0263b4cb5dc`. Its product readback reports
`current_version_number: 0` and `revision_count: 0`; it has no trajectory or run
and was retained rather than silently deleted. This operator setup mistake is
not counted as product acceptance or a platform defect.

This exact-F recurrence rules out deployment restart alone as credential
restoration and reconfirms that the current blocker is protected host ChatGPT
credential authority. It does not authorize another model-policy permutation,
credential transfer, SSH recovery, or auth weakening. Effects remain OFF.

### Operator-ratified scoped ChatGPT restoration preflight

At `2026-08-08T23:24Z` the operator explicitly authorized using the existing
local ChatGPT token and SSH access to Node B. This is the previously missing
protected authority; it is scoped to restoring Node B ChatGPT authentication,
not to changing model policy, other provider credentials, route state, guest
credentials, or self-development effects.

The tracked operator helper is `nix/deploy-provider-creds.sh` at SHA-256
`bf2eadd1f446c93e405125612b05757fee51f5d35b90b341f06b301e417e4560`.
Sanitized preflight found local `~/.codex/auth.json` SHA-256
`cc74452413e4622d253524960d160c059860e26d6dcc8310a17b08a5abee7de6`,
mode 0600, with access, refresh, and ID tokens present and access expiry
`2026-08-13T00:05:24Z`. Node B's current
`/var/lib/go-choir/codex-auth.json` is SHA-256
`eb1b7317613de015a3f29948cb738e74e84f60f0d073186760808afadba7aeb5`,
mode 0600, last refreshed `2026-07-24T00:10:18.068693Z`. After canonical newline handling, account-identity digests are equal; the
preflight's earlier apparent difference was a local hashing-method error, not a
credential identity change. The exact file digests still differ and the operator
explicitly selected the newer local record as the replacement.

The full helper would also regenerate `gateway-provider.env`. Per-key digest
comparison proved every planned value already equals Node B except
`AWS_BEARER_TOKEN_BEDROCK`; invoking the whole helper would therefore exceed the
ChatGPT-only authority by changing Bedrock. The restoration will instead use the helper's exact ChatGPT subset under an
exclusive Node B `flock`: upload only into a root-only directory, verify the
expected SHA-256, root ownership, mode 0600, JSON validity, and required token
shape without printing values, then `install` an already-0600 same-filesystem
canonical temp and atomically `mv -T` it over
`/var/lib/go-choir/codex-auth.json`. Traps remove both temps and restore the
backup on install/service failure, preventing any readable-mode window. Only
`go-choir-gateway` restarts. The existing env file remains byte-identical at
SHA-256
`7c5cc6e848471bc0e7afccbcfd3704c61dc185dffce0c535379f7c817bd5b8ef`.

**Red mutation ceremony.** Conjecture delta: replacing only the stale Node B
Codex auth file with the operator-selected local record will restore the exact-F
ChatGPT tool boundary without source, policy, route, computer, or other-provider
change. Protected surfaces: the token in SSH transit, the root-owned auth file,
and the gateway restart. Admissible evidence: sanitized pre/post file digests,
mode/owner, unchanged env digest, active gateway with a new PID, exact staging
build identity, and one bounded sanitized canonical product ChatGPT probe that
proves usability rather than file deployment alone, followed by the fresh
accepted trajectory and before/after self-development/policy/run/lifecycle
receipts. Rollback: first create and hash a root-only mode-0600 timestamped copy
of the old auth file; on any copy, service, health, auth/refresh, or product-probe
failure, atomically reinstall the already-0600 backup, restart, and verify the
old SHA/account digest plus fresh PID and exact-F health. Retain the backup only
through acceptance, then delete it under recorded disposition. Heresy delta at
preflight: discovered none, introduced none, repaired none. Effects remain OFF;
credential restoration alone is not acceptance.

### First scoped restoration attempt safely rolled back before provider proof

Docs ceremony `77be0419ba28ff12c6b5375323be9ead2a38168c` and docs CI
`31284157112` passed before mutation. Under the accepted red mechanics, the old
auth SHA-256 `eb1b7317…` was copied to root-owned mode-0600 rollback ref
`/var/lib/go-choir/provider-auth-backups/codex-auth.cts-chatgpt-20260808T233240Z-d42b86b444.rollback.json`.
The local SHA-256 `cc744524…` was validated in a root-only directory, installed
through an already-0600 same-filesystem temp, and atomically exposed. The gateway
env remained exact at `7c5cc6e…`; only `go-choir-gateway` restarted, PID
`3782119→3806387`, and public host health remained exact F `67a61358`.

The first bounded product probe then revealed an operator-context mismatch
before any provider call. The local `.env` `CHOIR_API_KEY` belongs owner
`5bd6de97-3b58-408c-bf89-c42c81b083de`; its nonce-bound product route was
candidate `candidate-fleet-e15cb89f25d963c220319b7b`, guest
`computer-03335285269bdba4f94377e56879f9e6`, guest commit `d69e1a6f`, epoch 130,
not acceptance owner `c72404bb-3c43-4a53-8671-b5cbc48b24a7` and its retained
exact-F lifecycle environment. The new lifecycle endpoint therefore returned
HTTP 404, and a legacy generic document create returned HTTP 500 with no matching
document retained. No Texture/provider run was created and no ChatGPT request
occurred.

Because usability was unproved, the fail-closed rollback gate fired immediately.
The already-0600 backup was atomically restored, the gateway restarted PID
`3806387→3806702`, canonical auth returned exactly to `eb1b7317…`, env remained
`7c5cc6e…`, and public host health remained exact F. The temporary wrong-owner
probe key was revoked and returned HTTP 401. The root-only backup remains for the
next bounded attempt. A normal browser session for the correct acceptance owner
is now authenticated and can mint the proper computer-scoped key; no second auth
mutation may occur until this problem receipt lands.

Heresy delta: discovered an operator probe-owner mismatch and a sanitized digest
method error; introduced none durably; repaired none yet. The provisional auth
replacement was fully rolled back. Effects remain OFF.

### Second scoped restoration proved usable ChatGPT on exact F

Rollback receipt `8f55bb964652f7263a575a9a05ac655202f39efe` and docs CI
`31284504546` passed before the second mutation. The correct acceptance-owner
browser session minted key `ak_45ce1796-7044-4086-ad48-5f7789f6b4ba`, and a
fresh nonce-bound identity joined owner/computer scope
`vm-bbdbbd01c4390b7036067aaa12afeb68`, guest
`computer-42850e9734d9442386c5dd8bf3afbf19`, route digest
`sha256:648d6071215206b190376ff6c24f3c93c08483b09bfb2ffc4790c00f3dd66489`,
epoch 8249, and host/guest/platform to exact F `67a61358` before host mutation.

The same accepted transaction replaced old auth `eb1b7317…` with local
`cc744524…`, kept root:root mode 0600, left gateway env byte-exact at
`7c5cc6e…`, restarted only `go-choir-gateway` PID `3806702→3807256`, and kept
public health exact F. Rollback ref
`/var/lib/go-choir/provider-auth-backups/codex-auth.cts-chatgpt-second-20260808T234218Z-c7b9f88760.rollback.json`
is root-owned mode 0600 and retained through acceptance.

The bounded canonical product probe succeeded through real
`chatgpt/gpt-5.5` with no auth/refresh error: document
`c0956f24-a1ec-5280-aa53-5776dca6f4b2`, v0
`c1b47b8e-8f7c-57ea-ba73-563d0230ae67`, trajectory
`2cbf6c95-2344-5064-bf24-919bfb9f8cf0`, work
`f2e18cb7-007b-54c2-a26b-e1ea783831fe`, run
`cb19cfff-acb5-4fe4-966c-cd84b29a7630`, and appagent v1
`3de79895-9e1d-5e0d-b357-5eb6a40096e7`. The exact v1 content was the requested
single marker `CTS_CHATGPT_AUTH_RESTORED_85d09891afd34046`; work completed, the
persistent run passivated without error, and the trajectory was then retained
and publicly cancelled at lifecycle 4/reducer/watermark 5 with exact v1 terminal
head.

Final probe readback was fourteen terminal and zero active runs (six cancelled,
five failed, three completed), self-development OFF generation 0, policy
SHA-256 `7192b8b1600561a331fda32f27628296c3f5b9bd1ba30dd5fb82681985c45e2a`,
unchanged gateway env, active PID 3807256, and exact-F health. This proves the
protected ChatGPT availability blocker repaired. It is not the full supervision
acceptance: no Researcher, persistent Super, parallel CoSuper capsule,
source-open/correction, delayed result, restart, checkpoint comparison, or run
acceptance was exercised by the bounded marker probe. Effects remain OFF.

Problem delta: stale ChatGPT credential availability repaired. Heresy delta:
discovered none, introduced none, repaired none; credential restoration is a
protected operator repair, not an architecture heresy repair. The scoped key and
rollback backup remain live only for the immediate full acceptance and must be
revoked/deleted under terminal disposition.

### First full restored-provider acceptance exposed control-schema refusal

After restoration receipt `86559e3d` and docs CI `31284908588` passed, a fresh
nonce-bound preflight again proved owner/computer scope, stable VM
`vm-bbdbbd01c4390b7036067aaa12afeb68`, guest
`computer-42850e9734d9442386c5dd8bf3afbf19`, route digest `648d6071…`, epoch
8249, exact F, self-development OFF generation 0, policy `7192b8b1…`, and zero
active runs. The deployed CLI then created continuous-prose document
`82693dd5-93ad-595e-93a8-a12ea85d7f33`, v0
`dcbbcd84-fb2e-5502-92ca-9fb1211ec336`, trajectory
`14c99be0-dd07-50b8-b074-a92088a0f7b9`, and work
`22cc125b-832e-56c7-8fc5-6b56c175cba9`. Correctly ordered CLI `watch --after
0 --limit 100 --once <doc>` returned durable observation cursor 1 for the exact
start occurrence.

Real `chatgpt/gpt-5.5` run `2ce06146-9aa5-4cda-be17-d3b246479f8e` then
passivated without a canonical revision or child control. Its public run result
states that an attempted atomic Researcher-control packet was rejected by the
runtime schema and that the actor became non-writable after the failure. It
explicitly confirms no Researcher was opened, no execution/capsule evidence
exists, and the v0 owner instruction remains canonical. Public trajectory
readback is still live at lifecycle/reducer/watermark 1, exact v0 head, and one
open work item; revision count remains one. This is not provider failure: the
same restored ChatGPT path completed the preceding bounded marker proof and this
run itself completed a real model turn.

The generated runtime prompt also tells Texture that execution effects are
unavailable while effects are OFF and to preserve execution requirements only as
blockers. That instruction conflicts with the accepted Definition, which keeps
protected host/materialization/checkpoint/route effects OFF while requiring
assignment-runtime-owned disposable capsule execution as no-effect acceptance.
The deployed product therefore currently gives the model contradictory
obligations: use lifecycle-native Researcher/Super/capsule controls to prove the
loop, yet treat capsule execution as unavailable.

This is a newly documented platform-behavior problem, so no retry, owner tell,
code fix, cancellation, restart, or effects change follows in this commit.
Before another product attempt, source-level convergence must identify (1) the
exact rejected Researcher-control schema delta and why one rejected tool made the
activation non-writable, and (2) the stale effects-OFF prompt/authority boundary
that incorrectly conflates disposable capsule execution with forbidden protected
host effects. The preferred repair is substrate-level and minimal: connect the
already implemented lifecycle/capsule authority rather than add a bridge,
keyword workflow, prompt exception, or broad effects enablement. The retained
trajectory remains public evidence and must later be recovered or cancelled
through lifecycle authority.

Problem delta: provider blocker remains repaired; newly discovered deployed
control-schema refusal and effects-authority prompt conflict block full
acceptance. Heresy delta: discovered a likely deployed authority-boundary
contradiction; introduced none; repaired none. Effects remain OFF.

### Structural convergence and repair ceremony for the control refusal

Problem receipt `76eed5bf` and docs CI `31285400390` passed before diagnosis.
Read-only source convergence identifies three joined defects rather than a
provider or typed-store failure.

First, every atomic Texture control advertises `packet` to the model as only an
opaque JSON object, while runtime recursively strict-decodes the complete
`coagent_source_packet.v1` and enforces kind/content/action/safety semantics.
The canonical complete model-facing packet schema already exists inline in
`newUpdateCoagentTool`; the atomic Texture tools do not reuse it. Exact rejected
call bytes are unavailable through the public run projection, so we do not claim
which guessed field failed. The later non-writable state narrows the stage:
outer strict decode succeeded and semantic/pre-CAS apply failed.

Second, that pre-commit error calls `FailAgentMutation`. The existing required
write-tool loop is expressly designed to let a model correct and retry a failed
write, but the next attempt then finds mutation state `failed` and refuses it as
non-writable. The atomic store has not committed at this stage. Terminal
no-write run handling already owns failure if the model exhausts retries, so
poisoning mutation on an individual pre-commit tool error is premature.

Third, all three active Texture prompt overlays state that effects-OFF makes
execution/capsule verification unavailable and forbids Super/CoSuper requests.
The already implemented replacement is narrower: atomic `open_persistent_super`
creates the exact bound persistent-Super work/control; wake-after-commit starts
that non-lifecycle Super; only Super receives assignment authority; the
assignment runtime owns networkless disposable capsule intent/effect/inspection/
acknowledgement; assigned CoSuper receives only its exact capsule-local registry.
Protected host, self-development, event, checkpoint, materialization,
acceptance, route, VM, SSH, and generic spawn effects remain unavailable. The
prompt is still describing the superseded blanket boundary and prevents the
implemented safe path from being exercised.

Mutation class is **red**. Conjecture delta: if the atomic tools reuse one
canonical explicit packet schema, pre-commit tool errors keep the mutation
pending for the existing bounded retry loop, and prompt overlays distinguish
assignment-capsule effects from forbidden host effects, then a real provider can
commit one revision plus runtime-derived Researcher/Super controls atomically and
later Super can own isolated capsules without enabling any protected effect.
Protected surfaces are Texture tool schemas and canonical writes, lifecycle
control/opening and mutation state, persistent-Super wake, capsule assignment
runtime, prompt authority, Trace, cancellation, and no-effect/run acceptance.

Binding repair:

1. Extract the existing packet-payload JSON schema once at the lowest practical
   shared source layer and reuse it for `update_coagent` plus atomic Texture
   controls. Texture sees only payload fields; runtime-owned envelope/target/
   direction/work/identity fields remain absent and strict runtime validation
   remains authoritative. Do not add coercion, unknown-field tolerance, or a
   second drifting schema.
2. Remove only premature mutation failure for a pre-commit atomic apply/tool
   error. Preserve pending state for same-run bounded correction; preserve
   terminal no-write failure, CAS refusal, one-revision-per-turn, exact head,
   replay, and fail-closed authority.
3. Replace the blanket effects-OFF prompt statements with the exact boundary:
   Texture may request its owner's persistent Super only through an atomic
   `open_persistent_super` control carrying a valid execution request; Texture
   never directly spawns CoSuper or uses generic execution tools; Super alone
   owns assignment-runtime capsules; protected effects remain unavailable; no
   execution claim is allowed before durable capsule evidence arrives.

Admissible source evidence is a shared-schema equality/closed-shape contract;
strict invalid-packet negatives; a same-run invalid-semantic-packet then corrected
packet proving zero partial state followed by one atomic revision/Researcher
open; a tool-loop invalid→valid retry integration; prompt golden tests; existing
lifecycle/capsule authority, replay, race, cancellation, and negative matrices;
full CI. Admissible completion evidence is a new exact deployed commit and real
provider product trajectory. Rollback is a normal origin/main revert through CI
and deploy to F/current predecessor, followed by public cancellation or safe
recovery of affected trajectories. Effects stay OFF throughout.

Heresy delta at ceremony: discovered an opaque-schema/early-failure/stale-prompt
join that hid an implemented replacement; introduced none; repaired none until
source and deployed proof both pass. No code edit begins before this ceremony is
independently reviewed, committed, pushed, and green.


### Source-review authority gap before control-repair landing

Independent red source/security review of the uncommitted control repair rejected
landing with two high authority findings and no critical finding. First,
`actions[].inputs` is intentionally an open domain bag, but the recursive
runtime-owned-field rejector did not yet cover persistent-Super assignment and
capsule bindings such as control binding, assignment, capsule, capability,
execution-handle, candidate, and terminal-outcome witness identities. Those
fields are not envelope authority in the shared payload schema and must remain
runtime-derived even when nested in model-authored action input.

Second, Texture's installed role registry still includes the generic
`cancel_agent` tool. Its non-Super path accepts an arbitrary model-authored agent
id and searches active same-owner runs without exact lifecycle target/work
binding. An assigned CoSuper target reaches `CancelRun`, which invokes assignment
and capsule revocation. That is a pre-existing model-callable capsule-lifecycle
path inconsistent with the settled boundary that persistent Super alone owns
assignment capsules and Texture never controls CoSuper directly. This review
discovered the path; the current repair did not introduce it.

Mutation class remains **red**. Protected surfaces added to the immediate repair
are role registry and cancellation entry authority. The smallest repair is to:
(1) extend the existing recursive strict rejector with the runtime-owned
assignment/capsule binding and outcome-witness fields plus focused nested
negatives; and (2) remove `cancel_agent` from Texture's installed model registry
while preserving public lifecycle cancellation, reducer intent/late-evidence
semantics, and persistent Super's exact assigned-slot cancellation authority.
No generic replacement, Texture→CoSuper control, or broad effect enablement is
allowed. Admissible evidence is exact registry contract/negative tests,
recursive nested negatives, existing lifecycle/cancellation/capsule/race
matrices, independent source/security rereview, full CI, and exact deployed
product proof. Rollback remains a normal origin/main revert through CI/deploy;
effects remain OFF.

Problem delta: discovered one missing recursive identity class and one
pre-existing generic Texture cancellation authority; introduced none; repaired
none until source and deployed proof pass. This problem-first receipt precedes
any committed cancellation-registry fix.


### Control-repair CI gate timeout after source acceptance

Runtime candidate `8ac0b27d47c26d83392bc38488d0b62c258c60a7` passed
local sequential runtime shards, the focused cancellation regression, full
Texture-owner tests, focused Race, vet, schema/retry/prompt contracts, and two
independent red rereviews. CI run `31287510605` nevertheless failed only in the
intentional non-Race rerun of
`TestCancelRunTrajectoryDrainsMoreThanOneActivePage`: after its Race shard
passed, the 1,001-run Dolt fixture reached the production cancellation-drain
context deadline while looking up the first run. The same scale test passed
locally in isolation and in sequential shards, but timed out during a prior
parallel local shard and an independent broad review. This is now a three-
occurrence test-load cluster, not a new control-repair semantic failure.

Mutation class for any test-only stabilization would be **yellow**; changing the
production drain deadline or cancellation substrate would be **red** and is not
authorized as a convenience fix. The next safe probe is a failed-job rerun on
the exact commit. If it remains red, stop and assess whether the fixture can
prove multi-page exhaustion without consuming the protected production drain
budget, or whether a separately reviewed cancellation-substrate change is
needed. Do not deploy or claim the repair while CI is red.

Problem delta: discovered CI/load instability in an existing protected
cancellation regression; introduced no known runtime defect; repaired none.


### Exact-deployed Researcher bind failure after atomic control repair

Exact deployment identity for runtime candidate `8ac0b27d47c26d83392bc38488d0b62c258c60a7`
passed at VM `vm-bbdbbd01c4390b7036067aaa12afeb68`, guest
`computer-42850e9734d9442386c5dd8bf3afbf19`, epoch 8250, with every
host/guest/service/deployment commit equal to the candidate and route digest
`sha256:648d6071215206b190376ff6c24f3c93c08483b09bfb2ffc4790c00f3dd66489`.
The repaired runtime recovered retained trajectory
`14c99be0-dd07-50b8-b074-a92088a0f7b9`: Texture run
`02193ba2-20e5-4d79-8731-15696fbba274` committed revision
`d7289837-55f7-51ee-825d-f7e2a12a641c` plus runtime-derived Researcher
`researcher:a1c34c97-5b1c-4177-9b33-d8d331c3df79`, work, and control in one
turn. This is deployed product proof that the opaque-schema/early-mutation
repair reached its first intended atomic transition.

The post-commit Researcher wake then failed before any provider call. At least
three distinct Researcher runs (`876fe019-85dd-4ba7-9257-485e9170a484`,
`c5fa1daf-36b4-4347-879b-25e4f8ad8280`, and
`a54090f8-c945-4feb-b0f8-d15361934c9f`) terminalized immediately with durable
metadata `lifecycle_control_bind_failed=true` and
`lifecycle_control_bind_failure="lifecycle invalid transition"`. The open
control/work caused repeated activation attempts; no Researcher evidence packet
or execution claim exists. This is a new post-commit lifecycle-binding defect,
not a provider recurrence and not a schema refusal.

Mutation class is **red**. Protected surfaces are lifecycle activation/control
binding, actor restart/wake, work/update disposition, cancellation, and
post-commit delivery. The retained trajectory must be publicly cancelled now to
stop repeated failed activations while preserving revision/control/failure
evidence. No code fix is authorized until read-only convergence identifies the
single transition authority and a reviewed repair ceremony is landed. Effects
remain OFF.

Problem delta: the atomic control repair is product-proven through commit and
post-commit wake; a new Researcher bind/retry-loop heresy is discovered;
introduced/repair attribution remains unknown until source convergence.

Public cancellation `cts-repair-researcher-bind-cancel-v3` closed the retained
trajectory at lifecycle version 5/reducer and watermark 7 with exact terminal
head `d7289837-55f7-51ee-825d-f7e2a12a641c`, both work items cancelled, the
latest activation cancelled, and zero active owner runs. The bounded public run
listing immediately after cancellation contained 200 failed records and every
one carried `lifecycle_control_bind_failed`; therefore at least 200 failed
activation attempts were retained before cancellation won. This makes the
restart loop a severe unbounded retry symptom rather than three isolated
failures. No execution/provider call or capsule effect occurred.

### Exact-deployed Researcher progress exposed Texture mailbox starvation

Runtime repair `b5d907a34d0250d966e026d13f64f6d575a4423d` passed every
GitHub Actions job in run `31298004583`, including the fresh attempt-bound
Differential SBOM acceptance and Node B staging deployment. A nonce-bound
product check then joined host, guest, all services, deployment target, and
platform attestation to that exact commit on stable VM
`vm-bbdbbd01c4390b7036067aaa12afeb68`, guest
`computer-42850e9734d9442386c5dd8bf3afbf19`, epoch 8252, and route digest
`sha256:648d6071215206b190376ff6c24f3c93c08483b09bfb2ffc4790c00f3dd66489`.
The previously retained acceptance credentials were explicitly revoked and
returned HTTP 401; a new two-hour scoped key was minted through the already
authenticated acceptance-owner browser session. Effects remained OFF.

Fresh lifecycle document `e3b726e3-2afb-5b9d-b5b7-3a50e127c655`, v0
`1c9315b5-3c11-566f-9dee-ac4ef8c2042f`, trajectory
`8f7a926f-f5de-561d-9879-5f1f51818f3d`, and root work
`6c606dd8-5491-5fe1-b84f-8bde48055b9b` then proved the repaired boundary.
Texture published informative continuous-prose interim v1
`a339da9e-bb46-5262-bd25-50c0485e4265` while work remained open and atomically
opened Researcher `researcher:eebe9a14-89fa-4aaa-8ce5-b027c7fb451c`, work
`fe786687-f92f-4704-ab77-97466122dd08`, and control
`ddc49c43-2bdd-4841-bf63-efb07ae280bd`. Run
`0c7be60d-3871-4857-a7c9-6a829afae2c4` bound and durably delivered that exact
control before provider execution. Thirteen Researcher runs then completed
cleanly through real `chatgpt/gpt-5.5` provider execution and produced exact
source packets with IPFS CID/URI, RFC fragment, W3C selector, and Text Fragment
evidence. None carried `lifecycle_control_bind_failed`. This is deployed product
proof that the prior hydration/bind-loop defect is repaired rather than merely
unit-tested.

A distinct post-delivery problem then prevented convergence. Fifteen
`producer_report` updates accumulated in canonical state with disposition
`pending`; resident Texture run `f6eed29e-79e9-49d0-b943-896a32e6d63c` remained
pending and created no v2, incorporated no source, and opened no persistent
Super. Owner correction
`owner-instruction-99bd6f13a528620a31c0c0eb1a6e63ef8697edc0b64c1a66fdc312649884432e`
was durably queued after the partial evidence, explicitly directing immediate
incorporation, one narrow follow-up, and the two capsule assignments, but it too
was not processed. Meanwhile every completed Researcher activation reported
`work_disposition=open`; canonical open work caused another provider activation
and another report, reaching fourteen Researcher runs (thirteen completed and
one in flight) and fifteen unconsumed reports in about fourteen minutes. This is
not the repaired pre-provider bind loop: provider work and exact evidence
succeeded. It is a newly observed target-mailbox/control-starvation plus
open-worker retry amplification failure.

Public cancellation retained the complete evidence while stopping the loop.
Trajectory `8f7a926f…` is cancelled at lifecycle version 21, reducer/watermark
23, exact terminal head `a339da9e…`; both work items are cancelled, all sixteen
stored updates (one original control and fifteen producer reports) are
cancelled, both agents have empty `active_run_id`, and the last activation is
cancelled. No persistent Super, CoSuper, capsule, host, checkpoint,
materialization, acceptance, route, VM, SSH, or self-development effect occurred.
A non-lifecycle draft document `32474860-b3e5-41e1-9d89-53faf5c86312` was also
created by an operator's first use of the ordinary document endpoint before the
correct lifecycle endpoint; it has no trajectory and is retained rather than
hidden or misrepresented as acceptance evidence.

This is the required problem-first checkpoint. No source repair follows before
this receipt is committed and pushed. Read-only convergence must separately
trace (1) the canonical producer-report/owner-correction wake into the resident
Texture actor and (2) why repeated open Researcher reports have no durable
backpressure or completion bound. The preferred fix must connect an existing
durable mailbox/reconciliation authority or delete a superseded path; it must
not add polling, acknowledge undelivered evidence, auto-complete research,
widen cancellation deadlines, or weaken lifecycle scope.

Mutation class for any repair is **red**. Protected surfaces are producer-report
and owner-instruction delivery, resident Texture actor memory/restart,
lifecycle work settlement, Researcher reactivation, canonical update
acknowledgement, provider execution, cancellation, and no-effect acceptance.
Problem delta: repaired — exact Researcher control hydration/binding and
provider entry; discovered — target Texture mailbox starvation and unbounded
open-work amplification; introduced attribution remains unknown pending source
convergence. Effects remain OFF.

### Structural convergence and repair ceremony for Texture resume starvation

Read-only source convergence after problem receipt `4095b17d` proves two joined
actor-substrate failures and identifies existing replacements.

The target starvation occurs after the first valid report wake. A passivated
Texture run is correctly reactivated in canonical Store state, but
`reactivatePassivatedTextureRun` calls exported `ActivateRun`, which emits the
same deterministic `initial_dispatch` occurrence keyed by the same run ID as the
already processed first activation. The SQLite actor log correctly deduplicates
that one-shot occurrence, leaving the run projected `pending` with no executable
mailbox fact. The Texture-specific `coagent_result` handler then returns nil and
acknowledges each distinct report/correction wake after owner reconciliation;
later reconciliation sees the apparently valid pending mutation and returns
early. The runtime injectors for exact producer packets and owner instructions
therefore never execute. The deployed shape — f6 pending, v1 unchanged, fifteen
pending reports, queued correction — is the exact signature of this seam.

The replacement is already present immediately below the Texture-specific fork:
the role-uniform `coagent_result` resume path executes an exact reconciled run
synchronously in the current actor goroutine and returns durable memory state.
Texture already has stronger canonical document/head/mutation validation than
generic memory. Rewarm must connect that validation to the current occurrence;
it must not redispatch the one-shot initial occurrence. Cold/new Texture run
creation retains its one unique initial dispatch. Boot recovery must dispatch
canonical pending report/instruction occurrences before projecting any
passivated Texture run pending.

A separate old continuation synthesizer amplifies the starvation.
`continueOpenLifecycleWorkAfterTerminal`, added to preserve durable obligations,
interprets one open work item as standing authority to mint an unlimited series
of fresh `terminal_activation_work_recovery` runs after each provider
completion. Pending open producer reports provide no backpressure, and
`StepBudget`/`TokenBudget` are prompt-only. This behavior is explicitly
normalized by an existing test, but it conflicts with the newer exact control,
passivated-memory, and actor-occurrence substrate: durable open work records
responsibility, not continuously runnable demand. A later exact Texture control
is the authority to resume more Researcher work; Texture can atomically
incorporate/reject a report, settle the producer work, or keep it open while
issuing a focused follow-up.

Mutation class is **red**. Conjecture delta: executing a canonically reactivated
Texture run inside the current unprocessed report/correction actor occurrence,
plus deleting no-control Researcher continuation synthesis, should make every
committed wake either produce durable authenticated run memory and an atomic
Texture disposition or remain unprocessed/retryable. One control then authorizes
one bounded Researcher activation chain until a target-visible report, not
unlimited sequential provider calls. Protected surfaces are actor log identity
and acknowledgement, Texture document/head/mutation authority, run memory,
producer-report/owner-instruction injection and disposition, lifecycle work and
control delivery, restart recovery, provider calls, cancellation, and
no-effect/run acceptance.

Authorized repair boundary:

1. Preserve deterministic one-shot `initial_dispatch` identity for cold/new
   runs. For a revision-bound Texture run reactivated from passivated memory,
   remove the duplicate `ActivateRun` bridge. Return an explicit reactivation
   decision to the current Texture `coagent_result` handler, revalidate exact
   owner/computer/document/head/mutation/run authority, execute synchronously in
   that actor handler, and persist/return only the resulting actor memory state.
   Never trust a generic/stale Texture run ID or execute before canonical
   reactivation CAS.
2. Boot recovery enumerates exact pending Texture producer reports and owner
   instructions and dispatches their canonical actor occurrences; duplicates
   deduplicate in the actor log. It does not first turn a passivated Texture run
   into pending through an already-used initial occurrence. Initial owner work
   with no prior run may still create one cold run and one initial dispatch.
3. Delete the `ActivateRun` wrapper if no production caller remains. Delete
   `continueOpenLifecycleWorkAfterTerminal`, both completion call sites, and the
   test contract that demands generic successor runs. In the Researcher branch
   of the broad boot work scan, pending exact controls may use the scoped
   control/fingerprint reconciler; open work with no pending control remains
   durably open and idle. Do not mint a no-control run from recency, generic
   work scan, terminal completion, or provider result.
4. Preserve all reducer authority: producer reports remain pending until a
   Texture turn explicitly incorporates/rejects them; owner instructions remain
   pending until bound to an exact head and applied; open work is neither
   acknowledged nor auto-completed. A later exact control may resume the same
   parked Researcher or create only the canonically authorized activation.
   Cancellation and late evidence remain Store-owned.

Admissible evidence is a full Adapter plus real SQLite-log test in which an
initial Texture run writes v1/passivates and the same run consumes a producer
report into durable runtime memory and v2; the equivalent owner-correction test;
crash/restart cuts before actor append, after append, and after reactivation CAS;
coalesced ordered reports plus correction; negatives for stale generic memory,
wrong head/scope/run/mutation, failed injection append, and no-write turns; and a
bounded Researcher contract proving one open report yields zero generic
successor/provider call across repeated reconcile and restart until a new exact
control arrives. Existing control-bind/restart/cancellation/Race matrices,
independent lifecycle/security review, full CI/SBOM, exact deployment identity,
and fresh end-to-end acceptance remain required.

Rollback is a normal origin/main revert and exact redeployment followed by public
cancellation of any affected trajectory. Pending reports, instructions, work,
and controls remain canonical and inspectable. Never randomize initial IDs, add
polling, mark evidence delivered before run-memory append/Texture turn, restore
generic Researcher continuation, widen cancellation deadlines, or auto-complete
work.

Heresy delta at ceremony: discovered — a duplicate one-shot dispatch was treated
as reactivation authority, distinct committed wakes were acknowledged against a
non-executing pending projection, and open work was treated as unlimited
provider authority. Introduced — none intended. Repaired — none until exact
fault/restart tests, independent review, full landing loop, and authenticated
product proof pass.

### Independent-red-review rejection and tightened authority boundary

The first ceremony review is **REJECT**, so implementation remains unauthorized
until these gaps are incorporated. The following constraints supersede any
looser reading above.

**Occurrence authority and acknowledgement fate-sharing.** A received actor
wake is not authority merely because the target has some pending backlog. Define
two injective, versioned, length-prefixed canonical occurrence identities:

- producer report: exact owner, computer, trajectory, target Texture agent,
  producer agent, update id, producer-update id, producer work id, lifecycle
  version, and reducer/message sequence from the pending Store row;
- owner instruction: exact owner, computer, trajectory, document, target Texture
  agent/work, instruction id, request id, head revision, kind, lifecycle version,
  and reducer sequence from the pending Store row.

Live queue and boot recovery use the same identity and complete actor envelope,
including trajectory and an explicit authenticated owner source for owner
instructions rather than empty/random fields. Before reactivation and again
before provider entry, the handler reloads the exact canonical trigger row and
joins owner, computer, live trajectory/no cancellation intent, document/current
head, Texture agent, open Texture work, run, mutation/revision, occurrence
version, and pending fate. Wrong, stale, foreign, terminal, cancelled, late, or
already disposed occurrences are typed zero-provider outcomes; they never select
global backlog or authorize a write.

A handler may report success to the actor log only after the exact triggering
occurrence is durably appended to authenticated run memory or is atomically
incorporated, rejected, applied, cancelled, or classified late in Store. An
injection, run-state, mutation, provider, no-write, or persistence error while the
trigger remains pending returns an error or durably passivates plus appends a
new explicit retry occurrence before acknowledgement; it may not terminalize the
only resume authority. Expose a checked cold-start dispatch result: projecting a
new run is insufficient until its unique initial occurrence append succeeds, or
the current trigger remains unprocessed. `ExecuteActivationSync` needs an
explicit outcome/postcondition rather than a void success assumption.

Because pre-fix deployments may already contain `processed actor row + canonical
pending row + no authenticated run-memory receipt`, add one deterministic,
versioned recovery occurrence derived from the exact pending Store row and its
canonical target run/memory state. It is not a random retry and cannot collide
with the processed legacy occurrence. Recovery must first prove the absence of
an admissible run-memory/disposition receipt, and repeated boot/reconcile must
deduplicate it. This is a migration for retained stranded facts, not a second
mailbox authority.

**Central Researcher provider admission.** Deleting the immediate completion
hook is necessary but insufficient. Inventory every production caller of
`reconcileAssignedWorkItemActor*`, `sweepPassivatedSpawnedCoagentWork`, broad
open-work sweep, interrupted activation rewarm, start/replay, and provider
entry. Enforce once at the provider boundary: a lifecycle Researcher may execute
only when its exact run is joined to a current pending-or-canonically-delivered
Texture control fingerprint and exact owner/computer/live trajectory/Researcher
agent/open-work versions. Open work alone, generic recency, a terminal provider
result, a passivated spawned-work scan, or a stale pending/running projection is
never provider authority. Preserve separately identified legacy non-lifecycle
behavior. Existing lifecycle Researcher work without a control remains explicitly
idle and inspectable, eligible only for cancellation or a later exact Texture
control; it is not silently settled.

**Expanded cross-ledger fault proof.** Add cuts at canonical trigger commit before
actor append; actor append before handler; reactivation CAS/run update before
execute; authenticated memory append; provider response and any tool result;
atomic Texture-turn commit; handler return; SQLite `MarkProcessed` failure; and
snapshot persistence. At every cut, restart must prove at most one canonical
head/turn/disposition, exact pending or typed terminal trigger fate, no stale
memory authority, and a measured bound on unavoidable provider replay. Counting
provider tests must show one exact Researcher control cannot become two
authorized provider chains across handler retry/restart/cancellation; open work
alone produces zero calls, while a later exact control resumes safely.

The negative matrix includes wrong owner, computer, trajectory, document, head,
Texture/producer agent, target/producer work, run, mutation, update/instruction
id, lifecycle/reducer version, cancellation intent, terminal trajectory, and
late result, each with zero provider, Texture write, control, capsule, or
protected effect. Coalesced reports plus correction must each have one ordered
run-memory/disposition/actor-ack fate. No code edit begins until independent
review accepts this tightened ceremony and the docs-only authority commit is
pushed.

### Final actor-ack and no-write clarification

A second lifecycle review remains **REJECT** until the cross-ledger snapshot gap
is explicit. The repair therefore adopts canonical replay rather than assuming
SQLite processed state and actor snapshot update atomically.

Actor snapshot is never occurrence, run, or lifecycle authority. After handler
return, a crash may persist `MarkProcessed` before `SaveSnapshot`. Boot and live
reconciliation must join the canonical pending trigger/disposition with exact
Store-owned `RunMemory` entries and the canonical Texture run/mutation/head. A
versioned recovery occurrence incorporates the exact trigger id/version, run id,
latest authenticated run-memory tail identity, and current canonical head/turn
identity. If the trigger is still pending, it resumes the exact canonically
validated run even when the old actor row is processed and the actor snapshot is
missing or stale; if the trigger was atomically disposed/applied/cancelled/late,
it produces a typed zero-provider terminal outcome. Repeated recovery of the
same joined state deduplicates, while an advanced run-memory/head/disposition
state yields the next exact identity. The handler must also return an explicit
resume-now decision for a run already projected pending by a crash after
mutation CAS or `UpdateRun`; it may not early-return merely because that pending
projection validates.

Authenticated run memory proves model visibility, but it does not by itself
settle or acknowledge canonical evidence. A successful Texture activation
triggered by producer report or owner instruction must atomically name the exact
trigger in its Texture-turn update/instruction disposition, or commit an explicit
Store-owned waiting/blocked transition that includes a deterministic successor
wake identity. A no-write result, a Texture turn omitting the current trigger,
or a failed injection/persistence cannot return actor success; the triggering
row remains unprocessed/retryable and no lifecycle update/instruction is marked
consumed. Required-write/decision tooling must enforce this postcondition rather
than rely only on prompt compliance.

The required fault matrix now explicitly includes handler return,
`MarkProcessed` success/failure, crash before `SaveSnapshot`, stale/missing
snapshot with present run memory, and crash after mutation CAS/`UpdateRun` with
the run already pending. Each must recover the same exact run/trigger without a
second canonical Texture turn or an unauthorized provider chain.

## Exact `fd83ce64` hibernated-computer wake returns 502 — 2026-08-09

Runtime repair `fd83ce64209beae56f2a515d4408a1d88a2fd6e3` passed GitHub
Actions run [`31306818891`](https://github.com/choir-hip/go-choir/actions/runs/31306818891)
on rerun attempt 2, including the selected Race shards, aggregate Go gate,
Differential SBOM acceptance, rolling publication, and Node B deployment. Public
`/health` reports exact host commit `fd83ce64`.

The successful deployment did not refresh the Definition's stable acceptance
computer. Its log recorded one active computer, the immutable constructed
computer for a different owner, and explicitly reported `No mutable active
interactive computers need refresh`. The Definition's previously exact stable
computer `vm-bbdbbd01c4390b7036067aaa12afeb68` / guest
`computer-42850e9734d9442386c5dd8bf3afbf19` was hibernated and therefore absent
from the active refresh set.

At `2026-08-09T10:28Z`, reopening the retained acceptance-owner browser session
started the normal public bootstrap/wake path. The UI remained in Choir BIOS and
first reported `Bootstrap probe 1 is still waiting; retrying`; after more than
210 seconds it reported repeated `VM route returned 502; retrying` and
`BOOTSTRAP FAILED (502)`. No authenticated product API, nonce-bound guest
execution identity, Texture mutation, provider call, capsule effect, or route
mutation was attempted. A different-owner immutable computer still returned a
valid joined identity whose host build was exact `fd83ce64`, but its preserved
guest remained `d69e1a6f`; that proves host deployment only and is inadmissible
for this Definition's product acceptance.

This is a newly discovered deployment/wake blocker, not evidence that the
Texture mailbox repair regressed and not a repaired heresy. The next action must
remain no-SSH and read-only until public wake either converges or exposes a
durable diagnostic surface. If it persists, diagnose the hibernated stable
computer's ordinary vmctl wake/route transition and its exact guest build using
public product evidence; do not substitute the other owner's immutable computer,
create a candidate/worker computer, weaken exact identity, or mutate Node B
without separate authority. Effects remain OFF.

### Public and CI diagnostics identify terminal Texture boot reconciliation crash

The failed deployment attempt's retained diagnostics make the 502 cause exact.
The refresh returned the stable VM as active at endpoint `10.206.187.2:8085`,
and the guest reached store/schema open, network, receipt signers, capsule
executor, and runtime initialization. Autoputer startup then exited with:

```text
actorruntime: reconcile Texture owner: reconcile subject
c72404bb-3c43-4a53-8671-b5cbc48b24a7/
vm-bbdbbd01c4390b7036067aaa12afeb68/
texture:11902866-d32e-55c4-9483-d9bd47c91a6c:
start reconciled Texture revision: load lifecycle work for Texture revision:
no open assigned work
```

The guest repeatedly restarted, so its health endpoint never became reachable.
The deployment's internal refresh timed out after 300 seconds even though the VM
record said active. Rerun attempt 2 then saw the stable computer as failed rather
than active and skipped it. Two normal public `wake_current_computer` recoveries
subsequently refreshed epoch 8253 and each ended with public
`recovery_timeout`; `/api/compute/status` reports state `failed`,
`recovery_eligible: true`, no runtime, and an absent/unjoined immutable route.

This narrows the blocker to the new fd83 boot reconciliation path over retained
terminal/cancelled Texture state. The deployed trajectory was deliberately
cancelled with no open assigned work, so absence of open work is a normal
terminal fact, not by itself startup-fatal corruption. Any repair must still
fail closed on operational Store failures and on ambiguous live recovery; it
may skip or durably settle only a proved terminal occurrence whose exact
trajectory/work state makes reactivation impossible. It must not invent open
work, reopen the cancelled trajectory, discard mailbox evidence, acknowledge an
operational error, or bypass exact occurrence authority. Problem documentation
precedes any source change.

### RED authorization ceremony — terminal Texture boot classification repair

**Conjecture delta.** `Handler.Start` first reconstructs exact Texture actor
occurrences, then calls generic `ReconcileActorWake` for every durable Texture
subject. `reconcileAgentWakeLocked` currently interprets a retained pending
owner instruction as runnable even when the trajectory snapshot is cancelled,
reaches `submitTextureAgentRevisionRun`, and fails because cancellation correctly
left no open Texture work. Proving the exact snapshot owner/computer/trajectory/document/head binding and
one recognized terminal status (`settled` or `cancelled`) before generic wake
selection, and returning without a run for that exact terminal subject, should
restore boot without weakening live occurrence recovery. A live snapshot must
also prove no cancellation intent. Empty/unknown status, binding mismatch, and
all operational Store errors remain fatal/deferred. Canonical actor rows are
processed only after their exact handler independently returns the existing
typed `TextureActorOccurrenceTerminal` fate; malformed/foreign
`ErrInvalidTextureActorOccurrence` remains a separate quarantine path.

**Protected surfaces.** Autoputer startup, Texture actor boot reconstruction,
lifecycle trajectory/cancellation authority, mailbox acknowledgement, mutation
selection, run admission, and exact staging deployment/acceptance are red. The
repair may touch only the Texture owner reconciliation boundary and focused
tests. It may not change Store cancellation semantics, reopen work, cancel owner
instructions generically, discard actor rows, treat missing/failed Store reads
as terminal, widen startup suppression, change vmctl/route behavior, or alter
provider/capsule/effects policy.

**Admissible evidence.** A real Store plus SQLite actor fixture must retain a cancelled trajectory,
cancelled work/update, pending owner instruction, durable Texture subject, and
restart scan; `Adapter.Start` must return successfully, create/reactivate no
Texture run, preserve the canonical pending instruction/evidence, retain the
actor row as processed only through typed terminal resolution, and perform no
provider work. Separate tests must prove live pending instructions still
reconcile, live cancellation intent blocks admission, and operational lookup,
unknown-status, or exact-scope mismatch errors are not classified terminal. Focused and full package suites, Race, vet, docs/dashboard,
independent lifecycle/security review, complete CI/SBOM, exact Linux guest
identity, and a fresh authenticated mailbox trajectory remain required.

**Rollback.** Revert only the terminal-classification commit to exact
`fd83ce64` source bytes; because deployed fd83 cannot boot the retained stable
computer, rollback is a source fallback for review, not an acceptable staging
state. If the candidate still fails boot or suppresses live work, retain all
artifacts, stop acceptance, and revert before any further platform mutation.

**Heresy delta.** Discovered and introduced by `fd83ce64`: a normal retained
terminal Texture state can crash the persistent computer during boot. Proposed
repair: terminal state becomes non-runnable while evidence remains durable.
Introduced by this repair: none accepted; any broad error suppression or silent
acknowledgement is a new heresy and rejects the candidate. Effects remain OFF.

### Terminal Texture boot repair implementation accepted locally

The bounded repair now classifies exact lifecycle activation eligibility before
any boot recovery candidate, run-memory, passivated-run, or mutation selection.
It exact-joins owner, computer, trajectory, document, and current head; recognizes
only `live`, `settled`, and `cancelled`; treats settled/cancelled and an exact live
cancellation intent as non-runnable; continues live execution only when the
intent lookup returns `ErrNotFound`; and propagates unknown state, binding
mismatch, snapshot failure, and operational intent errors. Generic wake repeats
the same read-only gate under the owner/computer/document lock before mutation.
Only the typed no-open-work sentinel may trigger a final terminal recheck when
cancellation wins during projection; the same condition remains fatal while the
trajectory is live.

A real Store plus SQLite actor restart fixture now retains a pending owner
instruction, a producer update atomically cancelled with exact disposition/ref/
version/sequence, two ambiguous passivated Texture histories and sleeping
mutations, and the exact actor occurrence. Startup and a second restart both
succeed; the occurrence row remains durable and is marked processed only through
existing typed terminal resolution; Store evidence and ambiguous histories are
byte-semantically unchanged; and provider/new-run/new-revision/new-mutation work
remain zero. Focused Race passes. Full `actor`, `actorruntime`, `agentcore`, and
`textureowner` suites pass; `go vet ./...`, docs receipt lint (21/0), doccheck,
dashboard tests/HTTP, gofmt, and diff checks pass. Independent lifecycle and
security rereview both returned `ACCEPT`.

The candidate is not yet deployed or product-proved. Effects remain OFF. The
next gate is commit/push, full CI/Differential SBOM, exact Linux deployment, and
nonce-bound wake/guest identity before any mailbox acceptance mutation.

### Exact candidate deployed; target-owner authentication requires user presence

Commit `7ba05599c15f6d126f86fa6fe1a44bc36a928121` passed CI run
[`31310481745`](https://github.com/choir-hip/go-choir/actions/runs/31310481745),
including all Race shards, aggregate Go, Differential SBOM acceptance, rolling
publication, and Node B deployment. The public proxy health body reports that
exact host commit, and the deploy activation receipt exact-joins all selected
host services. This is host deployment evidence only; it does not substitute
for the retained guest identity gate.

The retained target owner's browser refresh session is no longer valid.
`GET /auth/session` returns an unauthenticated session in each existing local
browser context, and every previously scoped target acceptance key is expired or
revoked as intended. The browser was reconnected to its existing Chrome CDP
profile and an exact passkey login was initiated for the same owner; the platform
is now waiting for Touch ID/security-key user presence. No new account, computer,
key, recovery token, replacement guest, SSH path, internal vmctl path, or weaker
identity probe was used. The separately authenticated different-owner computer
remains inadmissible.

Accordingly the candidate is deployed but the stable computer has not yet been
woken or product-proved. The next admissible transition is owner user presence
at the already-open passkey ceremony. After it succeeds, renew the same-origin
session, inspect the retained computer, issue the normal public recovery if it
is still failed, and require a fresh nonce-bound exact host/guest/service join
before minting a narrowly scoped acceptance key. Effects remain OFF.

### Same-owner authentication recovery audit and challenge expiry

An independent read-only audit found no admissible public same-owner session or
key renewal that can proceed without user presence. `/auth/session` renews only
from a valid stored refresh session; API-key creation requires a valid session
or a current non-revoked/non-expired bearer; desktop exchange requires both
valid access and refresh authority; and the CLI has no login/recovery command.
The public account-recovery flow does not auto-login and still culminates in a
new platform WebAuthn registration ceremony. Moreover, its current request
handler discards the only raw recovery token returned by Store, persists only
the hash, and has no mail-delivery or frontend handoff, so it is not an
end-to-end public recovery path. These findings were source-matched to deployed
auth build `7ba05599`; focused auth tests passed. No SSH, internal API, direct DB,
replacement account/computer, credential extraction, or auth weakening was
used.

The previously displayed Chrome login was misleadingly still labelled
`Waiting for Touch ID / security key...` after its server challenge had exceeded
the five-minute TTL. It was dismissed, and a fresh same-owner login challenge
was started through `/auth/login/begin`. Chrome was foregrounded and a local
notification requested Touch ID/security-key presence. Safe `/auth/session`
polling for the complete five-minute challenge window remained unauthenticated,
so that challenge is now expired too. Login-begin challenge state is the only
new staging auth artifact; no session/key was created and no computer or provider
transition ran.

The next admissible transition is coordination with the retained owner: begin a
new same-owner passkey assertion only when the owner can immediately approve
Touch ID or present/touch the already-registered security key, and complete it
within five minutes. Then verify the exact owner via `/auth/session` before any
key mint or retained-computer wake. Effects remain OFF.

### Authentication browser capability correction

The retained `omp-browser` session is not an admissible human-presence surface.
Live process evidence shows its Chrome is `--headless=new`, uses an OMP daemon
temporary profile, `--password-store=basic`, and `--use-mock-keychain`; its user
agent is `HeadlessChrome/150`. It has no headed browser window an owner can
operate. The page's `Waiting for Touch ID / security key...` text and positive
platform-authenticator capability signal are not evidence that a native Touch ID
sheet is reachable. Activating ordinary Google Chrome foregrounded the separate
normal headed Chrome process, not the OMP WebAuthn request. The two expired OMP
challenges are therefore diagnostic-only headless challenges, not evidence that
an available owner declined or missed a usable native ceremony. A CDP virtual
authenticator would be a replacement/forged credential and remains inadmissible.

The exact normal path is a fresh assertion in an ordinary headed persistent
browser profile that holds or can invoke the retained owner's authenticator.
Coordinate owner presence first, open canonical `https://choir.news/` in that
browser, use its normal Sign in flow, approve the native Touch ID or registered
security-key prompt, and finish within the five-minute server challenge TTL.
Verify `/auth/session` returns exact owner
`c72404bb-3c43-4a53-8671-b5cbc48b24a7` in that same headed same-origin browser;
then mint only a narrow acceptance key through Settings/public
`/auth/api-keys` and transfer it through the existing non-logging mode-0600
secret path. Only then may public retained-computer inspection/wake resume.
LaunchServices accepted a request to open canonical `https://choir.news/` in
the already-running ordinary Google Chrome process, but no authentication claim
or challenge was started. Effects remain OFF.

### Superseded F-era handoff and unusable acceptance-key reconciliation

A later operator handoff named docs ceremony `565abc77`, exact-F runtime
`67a61358`, retained trajectory `14c99be0…`, and acceptance key
`ak_45ce1796-7044-4086-ad48-5f7789f6b4ba` as if they were current. Read-only
reconciliation proves that handoff is historical rather than canonical:
local/origin `main` were exact at docs commit `e92bacd5`; at reconciliation start,
the only dirty paths were three preserved unrelated untracked memos (before this
goal-owned documentation update); public `/health` is HTTP 200 and
reports `7ba05599c15f6d126f86fa6fe1a44bc36a928121`; dashboard `/` is HTTP 200;
and current runtime CI/deploy `31310481745` is completed/success, including Race
shards, Differential SBOM acceptance, and Node B deployment.

The authorized packet/prompt repair did deploy as `8ac0b27d` and recovered
`14c99be0…` through its first atomic Researcher control, after which a distinct
bind-loop defect appeared. That retained trajectory was subsequently publicly
cancelled at lifecycle version 5/reducer-watermark 7 with evidence retained and
zero protected effects. Later bind, mailbox, and terminal-boot repairs culminated
in current `7ba05599`; reverting to F or replaying the old trajectory would
violate current authority.

A read-only public `/api/compute/status` request using the exact retained key ID
`ak_45ce1796-7044-4086-ad48-5f7789f6b4ba` now returns HTTP 401 authentication
required. No secret was printed or transferred, and no credential, provider,
route, environment, policy, guest, effect, or SSH mutation occurred. This 401
proves only that the old key is unusable for current acceptance; it is not
registry revocation proof. The key row and both root-only rollback copies still
require terminal cleanup/disposition after a newly owner-authenticated narrow
key completes acceptance. Effects remain OFF. The admissible next transition
remains a fresh retained-owner assertion in an ordinary headed persistent
browser, followed by a new narrow non-logging key handoff.


### Headed-browser ceremony produced an over-broad unbound key

At `2026-08-09T15:25Z`, a native headed-browser ceremony completed and copied a
key through the non-logging handoff. The agent ingested it directly to a
mode-`0600` file and cleared the clipboard without displaying the secret.
Read-only bearer authentication succeeded, but exact retained-owner identity was
not directly verified. The earlier inference that historical target-bound rows
in this bearer-visible registry proved same ownership was invalid:
`HandleCreateAPIKey` accepts caller-supplied `computer_id` metadata without an
ownership join. The broad key's public compute status instead selected logical
`primary` active at epoch `130`/code `7122f279…`, incompatible with retained
computer `computer-42850…` at logical `primary`, failed epoch `8253`. The admissible conclusion is owner ambiguity, strongly indicating a different
owner but not proving one; its registry rows are not retained-owner authority.

The new row was `ak_e2552ee6-c1bb-40a2-aad6-5b5d0a644112`, label `CLI key`, with
no computer binding or expiry and with `admin`, `manage:keys`, and broad
runtime/Texture/Base authority. It was not the requested narrow acceptance
credential. No lifecycle, provider, route, guest, effect, or acceptance mutation
was attempted with it. The different-primary compute response is refusal
evidence, not target authority.

**RED attenuation ceremony.** Conjecture delta: the native assertion restored
key-registry authority for some authenticated browser account, not proved exact
retained-owner authority, and the selected UI/CLI path minted a perpetual
administrator rather than a target-bound acceptance key.
Protected surface: only the public API-key registry. Admissible evidence: exact
public create/list/revoke status plus post-revocation HTTP 401, never logs or
secret values. The broad bearer is usable only on public key-management
`POST/GET/DELETE` for this single attenuation transaction: one
`POST /auth/api-keys` labelled
`cts-7ba05599-target-attenuation-effda9968935`, with `GET`/`DELETE` limited to
nonce reconciliation and revocation. The child must expire within two hours and
bind exact stable computer `computer-42850e9734d9442386c5dd8bf3afbf19`; joined
realization/VM `vm-bbdbbd01c4390b7036067aaa12afeb68` is not the key binding. Its
exact scopes are `computer:lifecycle`, `computer:self_development:read`,
`acceptance:read`, `read:runtime`, `write:runtime`, `read:texture`,
`write:texture`, and `read:base`. Earlier retained public evidence joins the
retained stable computer, VM, route, and epoch, but does not join this accidental
bearer owner. On an ambiguous POST response, list by the
unique label, revoke every matching child, prove each disposition, self-revoke
the broad key, prove broad post-401, and stop without retry. On a definitive
response, atomically store the once-returned secret mode `0600` and require
exactly one row, exact label and scope set equality, exact stable-computer
binding, and expiry after now but no later than two hours. If secret storage or
any metadata/single-row check fails, revoke every nonce-matching child,
self-revoke the broad key, prove broad post-401 and child post-401 whenever its
secret is available, then stop. On exact success, immediately self-revoke
`ak_e2552ee6…` and prove that same broad secret returns HTTP 401 before any target
probe. Then use the narrow key read-only to prove exact lifecycle status,
self-development read, nonce-bound stable-computer/VM/epoch/route/service
identity, and a wrong-computer denial. A target-probe mismatch occurs after broad
post-401, so it self-revokes the narrow key, proves narrow post-401, and stops. No
write/lifecycle transition is authorized before this gate.
Rollback is explicit revocation plus post-401, never expiry. Heresy delta:
`discovered` — an over-broad perpetual key was minted during the intended narrow
ceremony; `introduced` — none by the agent; `repaired` — none until attenuation
and broad-key post-401 complete. Effects remain OFF.


### First attenuation attempt failed closed before secret persistence

The reviewed one-shot transaction used nonce label
`cts-7ba05599-target-attenuation-effda9968935` and public key-management only.
The server returned one definitive child row,
`ak_834c56be-0d4b-4ae0-8958-56875cecfaa5`. Before any file was created, the
local atomic-storage path failed because the long-lived Python notebook name
`os` had been shadowed by an integer. The agent did not retry storage or key
creation. Following the reviewed failure branch, it listed exactly one live
nonce-matching child, revoked it by public DELETE (HTTP 204), proved its same
secret returned HTTP 401, self-revoked broad administrator key
`ak_e2552ee6-c1bb-40a2-aad6-5b5d0a644112` (HTTP 204), and proved that same broad
secret returned HTTP 401. Revoked credential files were removed and newly
introduced in-kernel secret references were cleared best-effort. No lifecycle,
self-development, runtime, Texture, provider, route, guest, or effect write was
attempted. Effects remain OFF.

This is a safe failed ceremony, not accepted attenuation. No usable new key
remains. The prepared direct ceremony must first require `/auth/session` exact
owner `c72404bb-3c43-4a53-8671-b5cbc48b24a7`; on mismatch it performs no POST and
a fresh retained-owner native assertion is required. Only an exact session may
mint the narrow key: bind stable computer
`computer-42850e9734d9442386c5dd8bf3afbf19`, never VM `vm-bb…`; use the exact
eight-scope set; expire within two hours; and copy the once-returned secret to
the non-logging handoff. The ingestion path must use a fresh isolated shell or
unshadowed module alias and verify mode `0600` before any API use.


**Attenuation outcome delta.** `discovered` — the local notebook name `os` was
shadowed in the atomic-persistence path; `introduced` — one bounded child key
authority existed for the single definitive create and was fully revoked;
`repaired` — the live over-broad administrator exposure and the bounded child
were both retired. Evidence/rollback is the exact pair of public receipts:
child DELETE 204 plus child-secret post-401, and broad self-DELETE 204 plus
broad-secret post-401. Acceptance-key availability remains unrepaired. Effects
remained OFF throughout.


### Exact direct narrow-key ceremony prepared

Source inspection found that the ordinary Settings form cannot mint the required
key: it sends only label/scopes, omits binding and expiry, and does not expose the
lifecycle/self-development scopes. The independently accepted direct same-origin
ceremony is preserved at
`docs/evidence/continuous-texture-supervision-direct-key-ceremony-2026-08-09.md`.
Its exact JavaScript payload SHA-256 is `a66562ec9964ca8d0e8a6932427f97a1a115c49fc3e59751654e12c8e36017b8` and once-ever label is
`cts-7ba05599-direct-narrow-8b7873810a8e`. It exact-checks retained owner `c72404bb…`, performs at most one
POST, binds stable `computer-42850…`, enforces the exact eight scopes/105-minute
expiry, exposes the secret only to an owner-clicked clipboard operation, and
fate-shares every non-success with all-match deletion, zero-live registry proof,
and known-secret post-401. The reviewer returned `ACCEPT`. This is prepared,
not executed; no registry or product mutation occurred. Effects remain OFF.


### Exact-owner inference correction

The earlier independent review accepted target-bound registry rows as transitive
same-owner evidence. Source and live evidence now falsify that premise: key
creation validates syntax/scopes/binding presence but does not authorize the
caller against the supplied stable ComputerID, while the accidental bearer's public compute-status response selected logical
`primary` active epoch `130`/code `7122f279…`, not retained logical `primary`
failed epoch `8253`. This is a
newly discovered audit error, not a regression and not proof of malicious use.
The accidental child and broad key remain fully revoked/post-401, so no live
exposure reopened. The exact owner gate is now the direct ceremony's canonical same-origin
`GET /auth/session` check for `c72404bb-3c43-4a53-8671-b5cbc48b24a7`. That GET
may rotate refresh/access cookies and is explicitly authorized as normal RED
auth/session renewal; mismatch means zero API-key POST and fresh native
assertion. Effects remain OFF.

The source gap is broader than the audit mistake. Cookie-authenticated key
creation does not vmctl-ownership-validate caller-supplied `computer_id`.
Lifecycle STATUS later re-joins exact user/ComputerID ownership, but
`HandleSelfDevelopmentMode` skips vmctl ownership for API-key callers and trusts
binding plus scope, while generic runtime/Texture routes do not universally
enforce `AuthResult.ComputerID`. The prepared ceremony compensates by requiring
exact session owner and exact lifecycle STATUS ownership before self-development
or any generic target call. This does not repair the platform gap; that broader
RED authorization repair remains open before final completion.

Heresy delta: `discovered` — caller-supplied key-binding metadata was mistaken
for ownership proof; `introduced` — none by this correction; `repaired` — the
current authority claim and ceremony gate, not the historical missing exact-owner
proof. Admissible future repair evidence is an exact headed `/auth/session` owner
match before the once-ever POST, followed by the target-bound read-only product
gate.


### API-key stable-computer binding authorization gap — RED repair authorized

The exact-owner correction exposed one substrate defect, not a collection of
route symptoms. Auth persists caller-requested `computer_id` after shape/scope
checks but without ownership authority. Proxy scope checks then select logical
desktops independently and most computer-touching routes never compare the
selected ownership's stable ComputerID to `AuthResult.ComputerID`. Consequently,
a registry row and even a joined execution-identity response prove only the row
metadata or selected computer, not that the bearer is bound to it.

The existing replacement substrate is already live:
`vmctl.Client.LookupComputerContext(ctx, userID, computerID)` performs an exact `(UserID, stable ComputerID)` ownership join and returns the
canonical logical DesktopID without resolving, assigning, waking, refreshing, or
stopping a computer. Existing vmctl lookup may subsequently perform route,
readiness-health, or exact-target gateway-credential maintenance after a valid
ownership record is found; it is not an unqualified pure read. Lifecycle already applies this join before
actuation. The repair must centralize that existing authority rather than couple
the auth service to vmctl or patch route responses individually. Keeping key
creation metadata-only prevents auth/session availability from fate-sharing with
vmctl; the proxy use-time boundary owns the live authorization join.

**Authorized repair boundary.** First, auth creation treats the binding as
requested attenuation metadata and requires non-empty `computer_id` for every
scope capable of selecting/touching a private computer: `admin`, runtime,
Base, Texture, `acceptance:read`, and all `computer:*`; `manage:keys` alone may
remain unbound. Bearer delegation remains subset-only and may not change a bound
parent's ComputerID. Second, every API-key request to a computer-selecting route—including
legacy/bootstrap/unbound `admin`—must first have non-empty
`AuthResult.ComputerID`; empty is generic 403 before any lookup/effect. One shared
proxy guard then joins
`LookupComputerContext(auth.UserID, auth.ComputerID)`, exact UserID/ComputerID,
and any requested path ComputerID or DesktopID from query,
`X-Choir-Desktop`, recovery JSON body, or default `primary`.
Mismatch/not-owned is generic 403; unavailable ownership authority is 503; no
static fallback. It must run after auth/scope checks but before downstream proxy
route lookup or ComputerVersion resolution, resolve/wake/refresh/stop, recovery `startOrJoin`,
autoputer/WS dial, corpusd self-development call, or execution attestation. In
particular, `computeRecoveryRequest.desktop_id` must equal the joined canonical
DesktopID before route or recovery work begins.

Wire the guard through generic protected HTTP, bootstrap, API and Super Console
WebSockets, private publication/proposal autoputer handlers, compute status and
recovery, execution identity, self-development, and lifecycle without weakening
lifecycle's current ordering. Bound-key compute status exposes only the exact
joined computer, not the owner's other computers. Do not indiscriminately gate
mail/notifications, corpusd public reads, public publication/search/pulse, or
server-derived cross-owner delivery routes that select no bearer computer.
Cookie behavior and unbound `manage:keys`-only operation remain unchanged.
Every existing unbound legacy key—including `admin`—is denied all
computer-selecting routes but may still list/self-revoke; `admin` bypasses scopes
only, never binding, ownership, or canonical DesktopID.

**Full RED ceremony.** Mutation class: RED. Conjecture delta: falsified — stored
binding plus scope, or a joined execution identity, is bearer ownership;
proposed — the single use-time `(UserID, stable ComputerID)` vmctl join before
selection/resolve is bearer computer authority. Protected surfaces: API-key
registry/attenuation, auth/key management, proxy routing, vmctl ownership and
lifecycle/recovery, runtime/Base/Texture routes, WebSockets, self-development,
execution identity, and run acceptance. Admissible evidence: auth tables prove target-capable scopes (`admin`, runtime,
Base, Texture, `acceptance:read`, all `computer:*`) without binding return 400,
`manage:keys` alone may remain unbound, bound parents cannot delegate empty or
different binding, and exact same binding persists only metadata. Route-family
negative axes include wrong/not-found owner, conflicting returned UserID or
ComputerID, path ComputerID mismatch, DesktopID mismatch from query, header,
recovery body, and default `primary`, missing vmctl, transport/500/malformed
operational lookup error as 503, unbound legacy/bootstrap-admin, admin
non-bypass, cookie parity, and API-key-only execution identity. Positives cover
every guarded family. Failures make at most one `LookupComputerContext` at the proxy boundary (none
for unbound/missing vmctl; foreign/not-found exits before vmctl's post-record
branch) and, after/beyond that lookup, zero proxy route resolution, desktop
lookup/list, resolve/wake/refresh/stop, recovery `startOrJoin`, runtime probe,
autoputer HTTP, WS upgrade/dial, corpusd self-development/lifecycle intent, guest
identity, or platform attestation. For a valid exact ownership followed by a
caller DesktopID mismatch, existing vmctl lookup may perform internal
route/readiness/credential maintenance on that exact authorized computer; tests
must not mislabel it as wrong-target product effect. Then run recovery/WS Race tests, full CI/SBOM, exact
staging deploy identity, public correct/wrong-target receipts with unchanged wrong-target VM lifecycle
state/epoch, no wrong-target wake/refresh/stop, and no wrong-target product effect, and execution identity whose ComputerID
equals key metadata plus lifecycle-owned target. Registry metadata alone is
never evidence. Rollback: no schema migration; revert the single authorization
commit and redeploy the prior known SHA, explicitly revoke ceremony keys, and
preserve all product evidence—while preferring roll-forward because rollback
reopens the gap.

Heresy delta: `discovered` — creation metadata was treated as ownership and
computer-selecting routes bypassed binding; `introduced` — none by this
read-only assessment; `repaired` — none until the shared guard is deployed and
staging-proved. Effects remain OFF. The prepared direct key payload is PAUSED and
must not execute until the repaired runtime is deployed; its bytes remain exact
and reusable after the new host identity is recorded.


### API-key binding repair local validation and aggregate load failure — 2026-08-09

The authorized RED repair is now locally implemented but not committed or
deployed. Target-capable scopes require non-empty stable-ComputerID attenuation
metadata, while `manage:keys` alone may remain unbound. One shared proxy guard
performs exact use-time `(UserID, stable ComputerID)` ownership lookup, rejects
unbound/foreign/conflicting path and all query/header/body/default DesktopID
selectors, and preserves the exact joined active ComputerURL and VMID rather than
re-resolving a logical desktop for bearer traffic. Recovery joins are
stable-computer-specific and exact identity is rechecked before wake, refresh,
or stop. Bound compute status cannot list sibling computers. Cookie and
non-computer corpusd/maild/public routes retain their previous behavior.

Independent `binding-security-review` and `binding-route-audit` both returned
`ACCEPT`. Focused vmctl/auth/proxy tests, their Race suites, and all
`agentcore`/`textureowner` runtime shards passed. A fresh `go test -count=1
./...` aggregate attempt then failed under concurrent package load in unrelated
`internal/actorruntime`:

```text
TestAdapterSQLiteInjectionAppendRecoveryExecutesWithoutSnapshot:
test unexpectedly relied on actor snapshot memory="" err=database is locked (5) (SQLITE_BUSY)
```

The aggregate subsequently reached the existing ten-minute package timeout
while another activation-budget test remained loaded. The exact failing test
passed alone in 3.144 seconds, and the full `internal/actorruntime` package
passed alone in 51.251 seconds. This is load-sensitive validation evidence, not
a binding-repair failure and not authority to widen production or test
cancellation deadlines. CI's normal sharding remains the admissible aggregate
gate; the implementation cannot deploy unless CI, Race, and differential SBOM
all pass.

Mutation class remains RED. Protected surfaces are API-key attenuation, proxy
computer selection, vmctl ownership classification, lifecycle/recovery,
WebSockets, self-development, execution identity, and run acceptance.
Conjecture delta: supported locally — one exact use-time stable-computer join,
retained through product selection rather than discarded into a desktop
re-resolve, closes the caller-metadata authority gap. Rollback remains a revert
of the single implementation commit followed by normal deployment to the prior
known SHA; roll-forward is preferred because rollback reopens the gap. Heresy
delta: `discovered` — aggregate actorruntime SQLite contention under parallel
package load; `introduced` — none; `repaired` — the API-key gap in reviewed
local source only, not deployed behavior. Effects remain OFF and the direct key
payload remains PAUSED.


### API-key binding repair landed; bearer-path proof awaits exact owner — 2026-08-09

The reviewed repair landed as
`fbc7ff5a048ed58d0f6dd02ae8462ae211eca328` (`security: bind API keys to exact
owned computers`). GitHub Actions run
[31326948312](https://github.com/choir-hip/go-choir/actions/runs/31326948312)
passed every selected Race shard, aggregate Go, differential SBOM acceptance,
rolling publication, and Node B deployment lane. Deployment receipts exact-join
`auth`, `proxy`, `vmctl`, `gateway`, `corpusd`, `maild`, and `sourcecycled` to
`fbc7ff5a`. Public `https://choir.news/health` reports proxy `build.commit` and
`build.deployed_commit` exact `fbc7ff5a`, with `deployed_at`
`2026-08-09T18:09:46Z`.

This closes the source, CI, SBOM, and host activation parts of the RED repair;
it does **not** prove API-key ownership or product-route behavior. Both
accidental keys remain revoked/post-401 and no usable bearer exists. Therefore
correct-target lifecycle selection, wrong-target 403 with unchanged lifecycle
epoch and zero wake/refresh/stop/product effect, pre-recovery execution-identity
refusal, retained-computer recovery, and guest/service identity all remain
open. `/health` is not execution identity.

The next admissible authority is ordinary headed persistent Chrome plus native
owner presence. Canonical `GET /auth/session` must return exact retained owner
`c72404bb-3c43-4a53-8671-b5cbc48b24a7` within the challenge TTL before the
frozen one-POST payload can run. Mismatch, expiry, missing presence, payload hash
mismatch, copy failure, or local persistence failure creates no usable
progress and follows the documented fail-closed cleanup branch. The deployment gate is satisfied, so the payload is authorized for one exact-owner
physical-presence execution; it remains NOT EXECUTED and effects remain OFF.

Mutation class remains RED. Protected surfaces are unchanged. Conjecture delta:
supported for source and host activation, not yet product-proved. Rollback is a
normal revert of `fbc7ff5a` and redeploy to the preceding authorized source;
roll-forward remains preferred because rollback reopens the bearer authority
gap. Heresy delta: `discovered` — none beyond the already recorded gap and
aggregate SQLite contention; `introduced` — none observed by CI/deploy;
`repaired` — source/host enforcement landed, deployed bearer-path repair still
awaits product proof.


### Exact-owner handoff and retained-recovery ceremonies prepared — 2026-08-09

The repaired host is exact and the owner-presence gate is now the only
prerequisite to live bearer proof. Four local helpers were prepared outside the
repository and reviewed without executing them or inspecting any clipboard,
key, or live evidence:

- clipboard ingestion reads only after an explicit owner `copied` signal,
  validates the exact secret shape, persists through same-directory exclusive
  mode-`0600` creation with file and directory fsync, and proves an empty
  clipboard without printing the secret;
- pre-recovery proof opens that credential once with no-follow, uid,
  regular-file, and exact-mode checks, refuses redirects, and requires in order
  exact `fbc7ff5a` public health, retained stable-computer lifecycle `primary` /
  failed / epoch `8253`, self-development `off` generation `0`, wrong stable
  ComputerID path `403`, pre-recovery execution-identity `503`, and an unchanged
  final lifecycle comparator;
- recovery owns one persistent request-bound marker before its sole POST,
  freshly rechecks lifecycle, single-computer/no-concurrent-recovery status, and
  exact host identity, waits only for terminal `ready`, and then requires the
  same stable ComputerID, logical desktop, retained `vm-bb…`, higher epoch, and
  signed nonce-bound `fbc7ff5a` guest/host/deployment/service identity;
- terminal cleanup selects the unique exact label/live row, requires its stable
  binding, eight-scope set, and expiry metadata, obtains exact DELETE `204`,
  proves same-bearer `401`, and only then removes the exact local inode and
  proves path absence. HTTP `401` alone is never called revocation proof.

The signed identity verifier was rebuilt independently twice from exact
`fbc7ff5a` archives plus one reviewed `http.Client` no-redirect line. The two
binaries are byte-equal at SHA-256
`3511c9f66f80367e4c31db735cafc84cfb0f6ec31fe295b94d4cf0a5c1d1887c`;
the sole patch is SHA-256
`afaa760da2a15d30df0fd804aa4e335719f0f9b2bcf0aa07311bd82b29682041`.
The binary and build receipt are installed under the owner-only mode-`0700`
secret directory and are verified before any recovery marker or POST. The CLI
then verifies guest and platform signatures, signer trust, validity, receipt and
route/host/deployment digests; the recovery ceremony retains the full safe
signed envelope.

`narrow-key-probe-review` and `retained-recovery-ceremony-review` returned final
`ACCEPT` only after rejecting and repairing sequencing, redirect, malformed
JSON, freshness, marker durability, terminal-ready, signature/digest,
redaction, binary provenance, hash-to-exec, and prerequisite-order defects.
This is preparation evidence only. The direct payload, ingestion, bearer probes,
recovery POST, verifier, and revocation helper remain unexecuted; no key exists;
effects remain OFF.


### Accepted post-recovery no-effect baseline — 2026-08-09

A fifth unexecuted helper now closes the immediate observation gap between the
accepted retained-computer recovery and the first new acceptance write.
`/tmp/cts-post-recovery-no-effect-baseline.py` is SHA-256
`8835b245be4d4706faa156c0ee03ff5b3cb73b254b5362dcfef48a9962079970` and
remains outside the repository. It opens only the owner mode-`0600` narrow key
and accepted recovery evidence, refuses redirects, performs GET-only requests,
and exclusively fsyncs owner-only baseline evidence.

The first independent review rejected the helper for three false-PASS paths:
weak route syntax and no live immutable-route join, malformed or error-shaped
HTTP-200 bodies, and silently truncated run/acceptance lists. The repaired exact
helper requires an exact 64-lower-hex route digest; recomputes it from the sole
live immutable identity; requires joined route, exact `fbc7ff5a` commit,
positive generation, nonempty receipt, and valid ComputerVersion; and joins the
sole API-key-visible computer to the current computer. It also requires exact
role/provider/model/source policy resolutions without `policy_error`, exact
`{runs:[...]}` and `{acceptances:[...]}` response shapes, and fewer than the
explicit 500/1000 limits. Booleans cannot satisfy integer epoch/generation
checks. `post-recovery-baseline-review` returned final `ACCEPT` on those exact
bytes without inspecting a key, recovery artifact, or live evidence.

This is preparation evidence, not a no-effect result. It must run only after the
accepted one-POST recovery and signed nonce-bound identity pass, and before any
new acceptance write. No bearer, recovery, guest request, or protected effect
was exercised while preparing or reviewing it. Effects remain OFF.


### Post-recovery product-contract audit and public Trace gap — 2026-08-09

A source-only audit of exact deployed source `fbc7ff5a` mapped every public
surface needed after retained-computer recovery without inspecting a key or live
evidence and without making network requests. The deployed product contract can
start durable Texture work through prompt bar, read the resulting document and
lifecycle snapshot, submit CAS-bound `tell`/`correct` instructions and direct
owner revisions, page or stream durable Texture/lifecycle events, inspect exact
revisions and source identities, open immutable source versions, read and cancel
runs, conditionally cancel a trajectory, and synthesize/read RunAcceptance.
Pending Researcher, Super, and CoSuper responsibility is observable only by
filtering canonical snapshot work items/agent state (and CoSuper assignments),
not by a public role-spawn route; this is correct because the owner must not
manually manufacture the required actor topology.

The audit found one exact completion blocker: `fbc7ff5a` mounts no public
`/api/trace/*` handler. `internal/trace/query.go` contains a dormant read handler
for event lists/details, but `internal/apihandler/routes.go` never registers it,
and the `/api/trace/trajectories...` evidence URLs emitted by RunAcceptance have
no implementation. The authenticated proxy therefore forwards those paths to a
guest `404`. No `choir` Trace command exists. Lifecycle events, Texture events,
source objects, and RunAcceptance summaries remain available, but none is raw
Trace authority. The Definition's action 9 requires inspection of each accepted
Texture version against typed packets, source objects, **and Trace**; a dead URL
or derived acceptance summary cannot satisfy that clause. SSH, direct Store/DB,
`/internal/*`, test routes, and manual evidence remain inadmissible substitutes.

This is the required problem checkpoint before any Trace-surface repair. A later
repair must expose only owner-scoped read authority through the ordinary product
route and stable-computer guard, preserve private payload boundaries, and add
public/CLI contract tests; it must not add mutation, a second event tape, or a
special acceptance backdoor. Deploying such a repair changes the frozen product
candidate and therefore requires a new exact identity/CI/deploy/acceptance gate,
not an in-place claim about `fbc7ff5a`.

The same audit reconfirmed a provider-availability risk for action 3: the exact
restored model policy maps CoSuper and text verifier to DeepSeek
`deepseek-v4-flash`, while the retained availability evidence for that provider
is HTTP 402. A temporary owner-computer file-policy selection of the already
product-proved ChatGPT `gpt-5.5`, followed by byte-exact rollback to SHA-256
`7192b8b1600561a331fda32f27628296c3f5b9bd1ba30dd5fb82681985c45e2a`, is being
prepared for independent review. It changes no credential and remains
**NOT EXECUTED**. Full provider-routing ceremony and before/after proof are
required before it can run. Effects remain OFF.


#### Temporary capsule-role model-policy ceremony

**Mutation class:** orange provider/model routing, with full ceremony required.
The mutation is limited to the computer-owned `System/model-policy.toml`; it
changes only the `roles.co-super` and `roles.verifier` provider/model selections
from DeepSeek `deepseek-v4-flash` to the already deployed and product-proved
ChatGPT `gpt-5.5`. It does not touch provider credentials, gateway files,
self-development mode, route projection, ComputerVersion, host services, or any
other role.

**Conjecture delta:** the retained 402 evidence predicts that leaving the exact
base policy unchanged may prevent the real parallel capsule/verifier portion of
action 3 from entering a model turn. Temporarily selecting the same working
ChatGPT provider/model already used by Texture and Super should allow those
assignment-bound actors to execute while leaving capsule network/capability
authority unchanged. Only actual authenticated CoSuper/verifier runs and typed
capsule receipts can confirm this; successful policy resolution alone cannot.

**Protected surfaces:** provider/model routing, the owner computer's exact policy
bytes, model-call metadata, and later capsule/run acceptance. **Admissible
evidence:** a fresh accepted post-recovery baseline; exact stable-computer epoch
and immutable-route digest before and after each write; byte-exact GET/PUT/GET
hashes; strict role/provider/model/source resolutions with no policy error; real
run/capsule evidence; and byte-exact final restoration. No local provider probe,
credential inspection, or response narrative can substitute.

**Rollback:** before the first PUT, persist and fsync the exact 1,124 original
bytes at SHA-256
`7192b8b1600561a331fda32f27628296c3f5b9bd1ba30dd5fb82681985c45e2a`
plus a request-bound apply marker. The temporary bytes are SHA-256
`6624bb112e805ff95974052a5fb808247bd81e5b9582ca81377b7b6297f40202`.
Rollback is the first obligation on any STOP after the apply marker, before API
key revocation or expiry: accept the exact marker/backup even if the apply
response or PASS receipt was lost, classify current bytes as original,
temporary, or foreign, restore only exact temporary bytes, and prove the exact
original file and original DeepSeek resolutions. Foreign bytes stop for human
adjudication. Both apply and rollback require the same baseline route again
after their final protected read and fail if identity changed.

**Heresy delta:** `discovered` — the accepted base policy selects a provider with
retained 402 availability evidence and raw Trace is not publicly mounted;
`introduced` — one bounded, explicitly evidenced temporary policy difference,
not a credential or second routing authority; `repaired` — zero until exact
original bytes and resolutions are restored. Independent `acceptance-policy-ceremony-review` returned final `ACCEPT` for
apply SHA-256 `7520e02b294c445bfe3c5ea5d18db3f5e80549c015367006a719d01f8b7d0e4b` and rollback SHA-256
`fe752b17cde9ee2fdac7a2c47d86ac4b419db2367381303f5913bc0905d9c640` after rejecting and repairing ambiguous-apply rollback,
pre/post route continuity, provenance freshness, partial marker/PASS publication,
and recovered-rollback marker validation. The ceremony remains **NOT EXECUTED**
until this authority checkpoint is committed and the owner gate, recovery,
signed identity, and GET-only baseline all pass.


### Accepted first progressive-loop start ceremony — 2026-08-09

The first red acceptance write after recovery now has an exact, source-reviewed
ceremony. `/tmp/cts-acceptance-start-v1.py` is SHA-256
`677cc8a0bff9ffa58465d01db72774ef6080d17e18d6ec12997baf2c01d6b54c`.
Its outcome-directed prompt is exactly 1,109 UTF-8 bytes at SHA-256
`3726cc5037f15cc6da4bd6db8a9f2eefe07503020b2a140382c53e2b005d4cc1`
and uses command identity `cts-fbc-full-acceptance-79a2722ca3`. The prompt asks
for an immediate useful continuous-prose revision while evidence remains open,
two independent source-backed angles, an executable comparison in isolated
workspaces, independent verification, and exact research/execution
transclusions. It does not name or force Researcher, Super, or CoSuper
choreography, require a first tool, or forbid ordinary read-only source research;
capsule work remains disposable and networkless.

Execution is gated on the accepted exact-owner recovery, signed `fbc7ff5a`
identity, GET-only no-effect baseline, and accepted temporary two-role policy
apply. Before POST it re-proves the stable computer, epoch, immutable route,
self-development OFF, temporary bytes/resolutions, exact rollback backup and
helper, and exact key-cleanup helper. A fsynced atomic marker precedes the one
semantic start; retries use only the same command/request bytes. The exact start
DTO is durably persisted immediately after response and must join completed
Conductor state, one V0 user revision, one initial Texture obligation, reducer
sequence/cursor 1, exact subject/document, and a lifecycle digest.

Observation pages the public `choir.texture_observation.v1` cursor through its
watermark and atomically persists cursor plus events for restart, with a hard
bounded deadline. PASS requires the exact V0 event/work join, one appagent v1
whose event was working/live, the initial Texture obligation still open, a
separate exact Researcher-assigned obligation open, at least 400 UTF-8 bytes,
and zero heading, list, table, or status-template lines. Semantic usefulness
remains bounded for owner inspection rather than being reduced to those
mechanical gates. Exact policy and route are re-proved after v1.

Every post-marker state durably names equal-command recovery, conditional public
trajectory cancellation, the accepted byte-exact policy rollback, and accepted
narrow-key DELETE-204/post-401 cleanup in that order. The script neither hides
nor auto-cancels a failed trajectory. Two independent source reviews first
rejected missing DTO joins, non-open Researcher evidence, non-resumable polling,
weak prose checks, and missing post-POST cleanup authority; final
`acceptance-start-v1-final-review` returned `ACCEPT` on the exact bytes. A local
fully mocked exact-contract run also reached `PASS_ACCEPTANCE_START_V1`; it was
logic validation only, not product evidence. The helper and prompt remain **NOT
EXECUTED** and do not prove v2, correction, Super, capsules, v3, cancellation,
or action 9.


### Accepted owner-correction/v2 ceremony — 2026-08-09

The next red acceptance write after the accepted start/v1 gate now also has an
exact source-reviewed ceremony. `/tmp/cts-acceptance-correct-v2.py` is SHA-256
`c068c1c87aa9534248c5970aa96881de0ee0c2e1a6f0f6aeb8a98a9e73052dd5`.
Its single durable owner occurrence is
`cts-fbc-correct-v2-c77146f860e6`; the exact 412-byte correction is SHA-256
`c77146f860e6cd1ac2a24ee7240203f801ac0c209f2963315f81c2d863444d81`.
It asks the next continuous-prose revision to contain exactly once the ordinary
sentence “Persistence alone is not learning; corrections must causally change
later revisions.”, argue it from at least one newly available canonical source,
avoid workflow formatting, and keep Texture, research, and comparison work open.
The sentence must be absent from v1 before POST.

Before the one semantic `/correct` occurrence, the helper pins the accepted
start evidence and exact helper hash, re-proves owner/computer/route/policy and
rollback/cleanup provenance, resumes the accepted Texture cursor to the current
watermark, refuses any pre-existing owner instruction or v2, and requires v1 to
remain the exact live head with the exact initial Texture and a Researcher work
item open. A mode-`0600`, fsynced marker freezes prior evidence, route, v1,
cursor/watermark, target work, literal request bytes/digest, client occurrence,
and rollback obligations before POST. The first response must be the exact
pending, non-replay `choir.texture_owner_instruction.v1` receipt; crash recovery
may transition only the same immutable receipt from pending to consumed.

PASS joins the queue receipt to both the public Texture projection and public raw
lifecycle-event page by event/cursor/command/digest/request/work/time. It then
joins exactly one later, singly correction-caused `texture_turn_committed`
version event to raw lifecycle artifacts, the exact public revision, owner,
document head, snapshot head, and still-live/open Texture plus Researcher work.
V2 must differ from v1, contain the qualification exactly once, retain at least
400 UTF-8 bytes of continuous prose with zero heading/list/table/status-template
lines, carry valid canonical source identities, and include at least one exact
ref/entity version/hash tuple absent from v1. After consumption the helper makes
one intentional byte-identical occurrence replay; it must return
`replay=true,status=consumed` with identical immutable receipt fields and no
second queue or v2 occurrence. Cursor/event evidence is fsynced after every page,
and every request and sleep is deadline-clamped.

Every post-marker STOP retains exact occurrence recovery, fresh conditional
public trajectory cancellation, accepted byte-exact policy rollback, and
accepted DELETE-204/post-401 key cleanup in that order. PASS carries those
obligations forward because the trajectory must remain live for the second
control, persistent Super, parallel capsules/verifier, and v3. Two independent
source reviews rejected the earlier unverified “new canonical source” clause;
both returned `ACCEPT` on the exact repaired bytes. Python syntax and a fully
mocked exact public-contract run reached `PASS_ACCEPTANCE_CORRECT_V2`, including
pending-to-consumed replay, raw lifecycle joins, and one new source identity.
That mock is logic validation only. The helper remains **NOT EXECUTED** and does
not itself prove the second downward control, persistent Super, capsule
parallelism, exact source-open parity, restart, cancellation, RunAcceptance, or
raw Trace.


### Accepted focused second-control ceremony — 2026-08-09

The next red acceptance write after causal v2 is source-frozen as
`/tmp/cts-acceptance-second-control.py`, SHA-256
`0f09449a67c03160aa004ce481374184b330224518cd9a07df48bd8f7628491f`.
It uses owner occurrence `cts-fbc-second-control-v2-9c6d3392e60b` and exact
465-byte content at SHA-256
`9c6d3392e60b4765bf6f7d5f556869b7dcbe11e532565773b4b3d2fcb4abb1b5`.
The tell asks for one focused, source-backed quantitative comparison on the
existing evidence thread, keeps that obligation open, explicitly defers a
semantic revision until evidence arrives, and forbids self-development, host,
deployment, credential, and routing effects. It names an outcome and constraint,
not Researcher, Super, CoSuper, a tool, or a spawn topology.

Before POST, the helper pins the accepted correction/v2 evidence and helper,
re-proves exact owner/computer/route/policy/rollback/cleanup authority, exhausts
both public Texture and raw lifecycle cursors without expiry, and requires one
shared current watermark equal to snapshot cursor and trajectory reducer
sequence. Exact public document, v2 revision/body/hash, snapshot document/head,
live trajectory, Texture obligation/agent, and subject authority must join. The
complete history must contain exactly the accepted correction instruction and
exactly one prior downward control. That first control is joined across public
and raw events, snapshot update, message sequence, canonical payload, source
Texture agent, and one still-open Researcher work/agent pair with Texture-owned
provenance. New Researcher, Super, arbitrary role targets, unseen owner traffic,
v3, or a second pre-existing control stop the ceremony.

A mode-`0600`, fsynced marker freezes v2, the shared cursor/watermark, the full
first-control target tuple, literal request bytes/digests, and cleanup order.
The one semantic `/tell` occurrence must first return the exact pending,
non-replay owner-instruction receipt. PASS then joins its exact queue event to
raw lifecycle, exactly one non-version `texture_turn_committed` consuming only
that tell, and exactly one later `control_queued` in the same
command/digest/time. The control event intentionally carries no owner request;
the same-turn join is the causal bridge. Fresh snapshot evidence must prove the
new control is distinct, is the only set difference from total control count
one to two, and reuses the exact same still-open Researcher and work item with
canonical update/message/payload/agent/work joins. It does not wait for delivery,
a Researcher result, persistent Super, capsules, or v3.

One byte-identical post-consumption replay must return the same immutable
receipt with `status=consumed,replay=true`, while cursor re-enumeration remains
exactly one queue, one tell-consuming turn, and two controls. Marker recovery
re-runs the complete first-control public object join rather than trusting IDs
alone. Every post-marker STOP carries occurrence recovery, fresh conditional
public cancellation, byte-exact policy rollback, and DELETE-204/post-401 key
cleanup in that order; PASS carries them forward on the live trajectory.

Independent review rejected an earlier runtime-NameError draft, recovery that
retained only first-control IDs, and an owner-wide run query vulnerable to
unrelated history. Both final reviewers returned `ACCEPT` on the exact repaired
bytes. Fresh and marker-recovery mocked public-contract runs reached PASS before
the final one-line source-accepted `channel_id=<doc>` narrowing; syntax passes
on the exact final helper. These are helper-logic checks only. The ceremony
remains **NOT EXECUTED** and persistent Super, capsule assignments, verifier,
v3, restart, cancellation, RunAcceptance, and raw Trace remain unproved.

### Accepted GET-only persistent-Super opener observer — 2026-08-09

The next acceptance gate after the focused second control is source-frozen as
`/tmp/cts-acceptance-persistent-super.py`, 60,854 bytes at SHA-256
`3a045901107dbb43f95bb42aeb22fe68da6b8ea23ac08a316b623ffa74fcf348`.
It is a GET-only public product observer: its request primitive rejects every
method except literal `GET` and every nonempty request body. It issues no owner
instruction, tell, correction, run submission, role/spawn request, assignment,
or other product write. Before observation it re-proves the exact accepted
second-control evidence/helper, retained owner/computer and immutable route,
temporary two-role policy bytes/resolutions, rollback/cleanup helper provenance,
and the unique live narrow-key row. The key row must retain the exact label,
computer binding, eight scopes, and at least 1,500 seconds of life. Complete
response hashes are retained, and the full stable registry view—allowing only
this key's expected `last_used_at` change—must remain identical with at least
300 seconds left before PASS.

A mode-`0600`, fsynced marker freezes the v2 head, accepted Texture and
Researcher run/work identities, second-control command/cursor/digest, both
starting cursors, and cleanup order. The observer exhausts document, history,
revisions, exact-v2, public Texture events, raw lifecycle events, snapshot, and
document-scoped run authorities. Every Texture `(event_id,cursor)` must equal
the complete ordered raw `(event_id,reducer_seq)` set, with equal watermarks,
snapshot cursor, and trajectory reducer sequence; cursor expiry, replay request,
truncation, nonprogress, or scope divergence stops.

The first admitted transition must be the already-bound Researcher's exact
producer report. The observer must capture it while still pending. It joins the
report's producer/target work, source run, second-control binding, message
sequence, payload digest, and owner/computer/channel to the same-command
`update_applied` that incorporates the second control and the following
workless `update_queued` event. Missing that pending phase is an overshoot STOP,
not inferred success.

A later non-version Texture turn must incorporate that exact report, settle the
same Researcher work, and in the same command/digest/time atomically open one
work item and queue one typed `execution_request` control to literal
`super:<owner>`. The Texture work, raw events, source run, open Super work,
canonical packet digest, nonempty actions, and deployed action/safety enums all
must join. Because persistent Super is owner/computer-scoped and lifecycle
version zero, the exact Super must remain absent from `snapshot.agents`; any
lifecycle Super identity or any CoSuper assignment stops.

PASS requires the later `control_delivered` event and snapshot
`delivered_to_loop_id` to join one unique stable public Super run. Both the
`/api/runs` list record and exact raw run detail independently prove owner,
computer, agent/profile/role/channel, empty top-level trajectory, requesting
Texture run, assignment trajectory/work, and exactly one
`lifecycle_control_bindings` tuple; their metadata must be identical on reread.
After key, policy, and route rechecks, a final no-progress public
event/raw/snapshot join makes the snapshot the last network read. It must retain
v2, the active run, the delivered control, identical watermarks, and no CoSuper
assignment. The observer passes immediately only at that fresh boundary. It
deliberately does not wait for Super output, assignment,
capsule, v3, or RunAcceptance.

Fresh and marker-recovery mocked public-contract runs reached
`PASS_ACCEPTANCE_PERSISTENT_SUPER`; a negative overshoot stopped when the pending
report phase was missed, and a terminal-STOP rerun preserved the prior state
byte-for-byte. These are helper-logic checks only. Independent deployed-source
and fail-closed reviews returned `ACCEPT` on the exact bytes. The helper remains
**NOT EXECUTED**. A crash may recover from its fsynced cursors only after strict
field-for-field pending-report and prefix revalidation, but an explicit terminal
STOP is durably latched, immutable, and non-resumable. Recovery reconstructs
exact cancellation authority from the marker, current second-control evidence
digest, and frozen helper hashes before any key or network gate; prior cleanup
fields are never trusted. Every post-marker STOP carries
fresh conditional trajectory cancellation, exact policy rollback, and exact
narrow-key DELETE-204/post-401 cleanup in that order; PASS carries those
obligations forward. Parallel CoSuper/capsule verification, v3, restart,
cancellation, RunAcceptance, and raw Trace remain later gates.

### Corrected v3-before-capsules topology and capsule evidence boundary — 2026-08-09

Source inspection after accepting the first persistent-Super observer corrected
the next acceptance sequence before any helper or product execution. Definition
action 3 is authoritative: the first persistent Super must return an
intermediate report; Texture must incorporate it into owner-visible v3 and send
a materially changed second direction to the same open persistent-Super work;
only then may capsule assignments begin. Jumping directly from the first Super
bind to CoSuper assignments would overshoot the required supervision loop and
must STOP rather than be credited retroactively.

The later capsule topology also cannot be a fixed immediate implementation plus
verifier pair. `assign_co_super` requires an exact completed implementation
candidate before a verification assignment can open. The smallest topology
that satisfies the Definition without introducing the forbidden fixed
one-implementer/one-verifier design is therefore: two implementation
assignments simultaneously bound in distinct writable capsules; one completes
and produces a candidate; then a third verification assignment binds that exact
candidate while the other implementation remains active. Public lifecycle
history must contain three distinct open/bind pairs, and the candidate-producing
report event must bind the verifier's `source_candidate_id`. This proves both
parallel work and a separate verifier rather than manufacturing an impossible
first-two implementation/verifier story.

At `fbc7ff5a`, public trajectory events/snapshot and run detail can prove durable
assignment authority: exact persistent-Super parent, implementation or
verification kind, run/work/agent identity, distinct capsule id, writable=true,
network mode forbidden, assignment-local overlay, source/subject/capability and
execution-handle digests, active bind disposition, and later capsule fate
acknowledgements. The bind is causally downstream of the runtime receiving an
active spawn acknowledgement and minting the exact assignment capability. It is
admissible activation evidence, but it is not by itself the final real-Linux
isolation proof: there is no public capsule-diagnostics or public run-event
route exposing namespace/cgroup/mount/PID enforcement or the granted executor
verbs. `InspectCapsuleRaw`, capability handles, and executor diagnostics are
internal and may not substitute.

Final acceptance must therefore join the durable bind to actual commands,
network-denial and writable-overlay probes, executor receipt refs, an immutable
verification subject, Texture execution-source transclusion/source-open, and
public cancellation fate through `revoke_requested` to `revoked`. If those
facts cannot be surfaced through the later typed CoSuper report, Texture source
projection, and RunAcceptance contracts, the next tracked change must be a
problem-led owner-scoped evidence projection/CLI repair. It should be clustered
with, but must not be conflated with, the already documented missing raw Trace
route. No route repair, capsule claim, or live request has been made here.

### Accepted Super-intermediate → v3 → changed-direction observer — 2026-08-09

The source-defined gate between the first persistent-Super control and capsule
work is now executable without adding a product mutation. The mode-`0700`
helper `/tmp/cts-acceptance-super-v3-direction.py` is 98,479 bytes with exact
SHA-256
`aa396bb607c3458f1055c19d4ff93f33ad6b2ff74c6552f24aa6a259e4dadc88`.
Independent deployed-source and security/fail-closed re-reviews ACCEPT that
exact byte sequence. Its supporting mock is outside the repository at SHA-256
`5c05c0ebdd8edff2f7f4cc0ee72f7093272e45989be6fa1471d95cb0fe8ff91d`.
Neither file has contacted the product or read a secret.

The observer revalidates the entire accepted persistent-Super predecessor and
secure cleanup tuple before network observation. It must catch the exact
persistent Super's first upward report while the report is pending and the
Super work remains open. One report-queue receipt must atomically incorporate
only the first delivered Super control immediately before queuing that report.
A later single Texture command must incorporate the report into exact structured
paragraph-only continuous-prose v3 and queue one materially changed typed
`execution_request` to the same literal `super:<owner>` and same open work. The
observer recomputes the structured document's text projection and validates
source-entity/reference completeness rather than accepting content disconnected
from the structured document projection.

The changed control may bind the still-resident first Super run or a new
persistent-Super run; run identity is not continuity authority. Either case
requires an exact active `pending`/`running` document-scoped run-list/detail
join and a full ordered lifecycle-binding set. Through the reducer sequence of
that control's delivery, the public history must contain no new owner
instruction/direct artifact-head mutation, extra control, work topology
change, later Texture revision, or CoSuper event. CoSuper events strictly after
that boundary are retained rather than rejected so the next capsule observer
can start from the exact prefix. PASS ends at a snapshot-last live trajectory,
exact v3 head, open Super work, pending changed control, and active bound Super
run.

Recovery is tied to one marker-created absolute 1,200-second deadline rather
than a restart-renewed timer. Success/error bodies are capped with a sentinel
byte; raw lifecycle cursor omissions are retained with the deployed
`omitempty` wire semantics rather than rewritten as explicit false; registry,
policy, route, predecessor, helper, cleanup, terminal, and PASS receipts are
locally revalidated. Mocked fresh/recovery observation, same/new Super run,
local marker/PASS/terminal rerun, deadline, response-cap, raw-wire, structured
projection, corrupt source DTO, missed pending phase, identical C2, invalid v3,
and other negative contracts pass. The mocks do not perform a live product
request or a full main-path credential gate. The helper remains **NOT
EXECUTED**, the owner `copied` gate is unchanged, and action 3 remains partial.


### Accepted two-implementation → candidate → third-verifier topology observer — 2026-08-09

The next source-only gate is frozen outside the repository as mode-`0700`
`/tmp/cts-acceptance-cosuper-topology.py`, 149,983 bytes, exact SHA-256
`785ae23b64e7336d7eb0d495ffaf75c4f6c5f58c2cf57f74fcfbd6ce80669e2b`.
Its 16,346-byte mock has SHA-256
`f5b332def5442baf089013a0e556be403a3fdeb63ed59cd13264105d9d6a8f66`.
Independent deployed-source and security/fail-closed reviews ACCEPT the exact
helper after correcting three real public-wire mismatches (list/detail
trajectory projection, public omission of lifecycle reason, and candidate ID
canonicalization), a concurrent fresh/recovery PASS-versus-STOP race, and exact
report/candidate/fate receipt joins. Compile, literal-GET AST audit, fresh,
recovered-PASS, pending-to-running list/detail retry, real lock
contention/no-mutation, and 21 negative fail-closed mock cases pass. Neither
file has contacted the product or read a secret.

The observer revalidates the exact accepted v3/C2-delivery predecessor and its
cleanup authority. It captures Phase A only when exactly two attempt-1
implementation assignments coexist bound/active on the same immutable
comparison subject, in distinct declared network-forbidden writable-overlay
capsules, with exact opened→bound reducer events, lifecycle Agent/Work
projections, and assignment-agent-scoped active run list/detail/prompt proof.
It then accepts Phase B only when one implementation follows
`freeze_requested → frozen → co_super_assignment_reported → revoke_requested →
revoked`, emits an exact changed-subject candidate and canonical report ref,
and carries reducer-emitted revoke intent/ack evidence refs. Only after that
revocation may a third verification assignment open and bind the exact
candidate while the other implementation and verifier runs remain active.
The final snapshot cursor must be exactly the verifier bind: no verifier report,
later Texture head, unrelated lifecycle traffic, or fourth assignment is
allowed.

The helper pages and exact-joins the complete public Texture prefix, raw
lifecycle prefix, and snapshot events; validates deployed raw-cursor omission
semantics; reconstructs deterministic assignment/report/candidate identities;
and normalizes the real run-list millisecond versus run-detail nanosecond
finished time. A safe nonblocking file lock serializes both fresh and recovery
invocations, while a contending observer exits without creating STOP, state, or
output. Marker-absolute deadline, response cap, immutable predecessor hashes,
registry/policy/route stability, pre-output terminal/marker/state recheck, and
retained PASS reconstruction remain fail-closed. The final successful network
read is the trajectory snapshot.

This observer deliberately proves only reducer-authorized public
spawn/bind/candidate/revoke topology and declared capsule modes. It cannot prove
live Linux namespaces, cgroups, mounts, PIDs, granted verbs, command execution,
network-denial enforcement, or writable-overlay behavior because `fbc7ff5a`
exposes no admissible owner-scoped capsule execution receipt surface. Typed
later reports/Texture sources/RunAcceptance must still be assessed; otherwise
a separately documented public projection/CLI repair is required. The helper
is **NOT EXECUTED**, the owner `copied` gate is unchanged, and action 3 remains
partial.


### Public capsule execution evidence is not restart-durable or owner-openable — 2026-08-09

Two independent source audits after freezing the topology observer establish a
new red product-surface blocker. This is documented before any repair. At
`fbc7ff5a`, public lifecycle snapshots and event pages expose exact assignment
bindings, declared capsule modes, report refs, current fate refs, and reducer
order. Public run list/detail adds assignment metadata and a terminal report ID.
Those surfaces are sufficient for the accepted three-assignment topology and
nothing stronger.

The full `CoSuperAssignmentReport` and `CoSuperSubjectCandidate` are durable
Store objects, but no public route or CLI opens them. `ExecutionReceipt`,
`GrantedExecutionReceipt`, freeze receipt, and revocation receipt contain useful
runtime bindings, yet their bodies are executor-local and unavailable through
an owner surface. Executor receipt artifacts are rooted under
`/run/choir/capsules`; a computer restart can retain lifecycle assignments,
reports, candidates, and opaque refs while losing the receipt bodies needed to
interpret them. The final assignment row also overwrites freeze intent/ack with
revoke intent/ack, leaving only immutable raw event refs for historical order.
`InspectCapsuleRaw`, executor maps/files, Store getters, and internal tools are
inadmissible substitutes.

The existing Texture evidence path does not repair this. The Super-return
packet reduces a CoSuper report to summary/claim and places generic evidence
refs in notes; it drops commands, outputs, mutations, executor receipt refs,
and typed sources. Therefore the generic source-entity/reference and
source-open machinery has no capsule execution entity to transclude or open.
RunAcceptance likewise does not verify assigned-capsule physics: its durable
CoSuper completion checkpoint and unconditional bounded-effect invariant do not
prove namespace/cgroup/mount/process isolation, granted verbs, command execution,
network denial, overlay isolation, or destroy acknowledgement.

The remaining error field is exact:

- real Linux user/PID/mount/net/UTS/IPC/cgroup isolation is unobserved publicly;
- the exact historical broker verb grant is not durably disclosed;
- command and granted receipt bodies are hidden behind opaque refs;
- `network_mode=forbidden` and the writable-overlay mode are declarations, not
  executed denial/isolation proof;
- implementation candidate and verifier subject joins are only partly public
  because report/candidate/certification bodies are hidden;
- freeze, revoke, destroy, and absence acknowledgements are not one typed
  public history; and
- no exact execution receipt is available as a Texture source identity that a
  later revision can transclude and the owner can source-open.

A route-only wrapper over current refs would be false repair, and no old run may
be retro-certified from deployed source. The current minimal design conjecture is an
additive read-only owner/computer/trajectory-scoped paginated API plus matching
CLI, backed by the existing lifecycle cursor and assignment/report/candidate
object graph. Before executor scratch evidence disappears, the runtime must
sanitize and atomically fate-share the missing grant-policy plus verifier-known probe-result/execution/fate
attestations into that existing authority chain. Old rows without the optional
attestations must return incomplete proof, not inferred success. The projection
must map validated evidence into the existing `CoagentPacketSource` → Texture
source graph rather than introduce a new ledger, table, service, or competing
source document.

Design review must still freeze the exact DTO and write boundary. At minimum it
must preserve the deployed stable-ComputerID API-key guard and owner lookup,
reuse `read:runtime` unless an independently justified narrower scope is needed,
page with existing cursor-expiry/replay semantics, survive Store/runtime and
computer restart without a live executor, and expose no raw capability,
execution handle, token, key, socket, host path, raw host/namespace PID, command secret, or
unbounded output. Kernel/network/overlay claims require fixed runtime probes
whose bytes and expected typed outcomes are verifier-known; declared modes or
arbitrary shell narrative cannot substitute. Linux integration, cross-owner and
wrong-computer auth negatives, lineage/fate races, restart byte-equivalence,
Texture source-open, CLI parity, and RunAcceptance fail-closed behavior are
required before deployment.

Mutation class will be red because the repair touches evidence, lifecycle
conditional writes, capsule receipt binding, stable-computer routing, Texture
sources, and possibly RunAcceptance. Rollback is additive-schema compatible:
revert route/CLI/enrichment readers and writers, retain already-written optional
evidence as inert unknown fields/objects, do not destructively migrate or delete
receipts, and keep effects OFF. Heresy delta so far is `discovered`: public
capsule evidence is neither physically complete nor restart-durable.


### Reviewed two-gate repair contract — 2026-08-10

The problem checkpoint at `54ffd2a7` is accepted and docs CI
`31342864830` passed. A source-aware consensus panel returned two usable
convergent reviews and one timeout with no opinion; two additional independent
route/security reviews challenged the proposed API shape. The result is
`REVISE`, not permission to claim repair: one Definition may own the work, but
two independently landable gates are required and F1 must remain explicitly
incomplete until F2 real-Linux proof.

The route is smaller than the initial conjecture. Existing
`/api/trajectories/{T}/events` already owns pagination, reducer order, cursor
expiry/replay, and watermark, while the trajectory snapshot owns assignment
discovery. Adding a second capsule event page would duplicate that authority
and still could not paginate mutable report bodies historically. The reviewed
public addition is one exact point projection:

```text
GET /api/trajectories/{trajectory_id}/capsule-evidence/{assignment_id}?attempt=N
choir lifecycle capsule-evidence <trajectory_id> <assignment_id> --attempt N
```

It reuses `read:runtime` and the deployed stable-ComputerID API-key guard; no
new key scope or computer selector is introduced. The handler derives owner
only from trusted authentication and computer only from the configured runtime,
loads one serializable owner/computer ObjectGraph snapshot, and joins the exact
trajectory/assignment/attempt within it. `EscapedPath` parsing must accept
exactly the three canonical decoded segments, reject decoded slash/backslash,
dot segments, trimming, malformed escape, extra/trailing components, and parse
exactly one positive canonical decimal `attempt` query with no unknown key.
Only absent or cross-scope route keys `(trajectory, assignment, attempt)`
return uniform `404`. Optional missing grant, execution, fate, probe, or source
fields return `200` with stable deficits and `evidence_complete=false`. A
referenced report, candidate, or attestation that exists but fails exact scope
or lineage makes the whole response corrupt/ambiguous and returns one
non-oracular fail-closed error; it is never treated as optional. Method mismatch
and response cardinality/encoded-size limits fail explicitly. The read path never opens the executor, `/run`,
`InspectCapsuleRaw`, or a raw Store/object endpoint.

The response schema is a manual allowlist, never JSON-marshaled Store structs.
It contains current sanitized assignment identity/binding, **all** reports,
candidates, and attestations in reducer-sequence then stable-ID order, expanded
to sanitized projections, with exact report→candidate
and implementation→verifier joins, grant policy, sanitized per-command
execution attestations, append-only typed fate history, Texture source refs,
`snapshot_cursor == watermark`, a server-derived versioned verifier contract,
`evidence_complete`, and stable-sorted deficit enums. It excludes free-form
summary/reason/notes, arbitrary evidence/output/mutation refs, raw execution
refs, command/argv/env/stdin/stdout/stderr, capability/handle material, tokens,
keys, sockets, host paths, and raw host/namespace PIDs. Duplicate/conflicting
refs, identities, report/candidate joins, or attestation keys are errors, never
first/last-wins. Old rows are readable only as incomplete and may never be
retro-certified.

#### F1 — durable evidence substrate and incomplete public projection

F1 adds no table, ledger, service, object kind, or second cursor. Three optional
runtime-authored DTO families ride existing authority objects and participate in
existing command digests so equal replay is byte-equal and changed evidence
conflicts:

1. `CoSuperGrantPolicyAttestation` is derived after spawn only after the
   runtime validates the returned signed capability's role, bound run,
   capsule/target, and exact verb set against the compiled CoSuper policy. The
   Store revalidates the canonical sorted verb set and digest. It is
   fate-shared into the existing Bind conditional batch and retains a typed
   spawn/active/grant acknowledgement joined to that exact Bind event and
   reducer sequence, because Bind itself changes the capsule from unbound to
   active. It contains schema/ref, role, exact sorted granted verbs, verb-set
   and policy digests, declared network/filesystem/writable modes, bound run,
   and safe timestamps/digests—never the handle or capability bytes.
2. `CoSuperExecutionAttestation` is constructed while
   `ResolveGrantedExecutionReceipts` already holds each validated receipt and
   is fate-shared into the existing RecordReport batch. It contains the report
   command identity/digest, exit code, bounded stdout/stderr/source/final/worktree
   digests, granted/frozen status, occurred time, and safe content-addressed
   attestation/ref joins, never command text, cwd, raw output, or raw handle.
3. `CoSuperCapsuleFateStep` is append-only on the existing assignment row and
   fate-shared with each SetCapsuleDisposition batch. It retains ordered
   freeze-requested/frozen/revoke-requested/revoked transition identity,
   reducer/event/command joins, intent/ack refs, safe subject digests, and typed
   absence facts. Revoke must not overwrite freeze. The executor revocation
   receipt must be extended only with the already-verified safe process/cgroup/
   mount/state absence booleans needed to interpret `capsule_absent`.

The Store validates exact assignment/run/capsule/report/command/candidate scope,
attestation schema/ref/digest, order, uniqueness, and disposition-specific
fields; model-authored fields are cleared and cannot supply evidence. A crash
before the conditional commit produces a deterministic deficit, never a
receipt-only partial success. The F1 detail route/CLI reads only the one Store
snapshot, and must be byte-identical after Store reopen, service/runtime
restart, and retained-computer restart/recovery with no executor or `/run`
receipt available. It exposes existing logical lineage plus newly durable
grant/execution/fate data. F1's verifier
contract deliberately reports `isolation_probe_missing`,
`texture_source_missing`, and `run_acceptance_gate_missing`; it cannot complete
Linux isolation and effects remain OFF.

#### F2 — fixed probes, existing Texture source graph, and acceptance gate

F2 is gated on deployed F1 passing that whole-computer restart/recovery
proof. It adds one versioned, in-repo, digest-pinned verifier-known probe bundle executed by the trusted runtime under
the exact minted capsule grant and persisted through the F1 authority fields.
The receipt says only which fixed probe/version ran and whether its typed
expected and observed outcome digests match. It may not synthesize a generic
“kernel attestation.” The final probe contract must cover safe evidence for
user/PID/mount/net/UTS/IPC/cgroup namespace/process separation, cgroup-v2
membership/limits, a fixed network-denial attempt, assignment-local overlay
write/read plus cross-capsule miss and unchanged immutable source, exact granted
and refused verbs, exec-after-freeze refusal, and revoke/destroy absence. It
uses digest-only namespace identities and no raw PIDs or host paths. Each
probe attestation names its existing carrier command and conditional batch:
pre-bind spawn/grant/static probe facts ride Bind, execution/result probe facts
ride RecordReport, and freeze/revoke/destroy facts ride SetDisposition. No
probe-only write may create or advance evidence outside an existing lifecycle
event/cursor. Probe scratch must be outside the canonical subject or provably
deleted without changing it; exact broker `/proc` behavior requires Linux
design validation.

The authenticated report commit atomically places the sanitized evidence
identity in a `CoagentPacketSource` on the durable return packet, not a new
execution document or source API. The existing Texture source entity/ref graph
is materialized only when Texture later authenticates and incorporates that
packet. A later Texture revision must then actually transclude the exact source
ref, and existing source-open must return the byte-identical sanitized evidence
under owner/revision/ref scope.
RunAcceptance gains an explicit fail-closed verifier contract/invariant: any
missing/unknown/invalid grant, probe, execution, fate, lineage, or source
transclusion is `blocked`; declarations and trace summaries cannot upgrade it.

Acceptance uses existing events/snapshot plus one detail per discovered
assignment: page events completely, fetch all details, then fetch the final
trajectory snapshot last. PASS requires exact final assignment-key equality,
each detail cursor/watermark equal to the final snapshot cursor, canonical
assignment projection equality, complete gap-free event history, no deficits,
and exact cross-assignment candidate/verifier joins; otherwise retry the whole
GET-only set only to a bounded quiescent boundary, then STOP.

#### Red ceremony

- **Conjecture delta:** persisting only sanitized, runtime-derived grant,
  execution, and fate facts inside existing assignment/report authority makes
  those facts restart-openable; fixed probes and existing Texture source
  transclusion can then turn declared isolation into admissible physical proof.
- **Protected surfaces:** stable-computer/API-key routing, lifecycle command
  digests and conditional ObjectGraph writes, capsule capability and receipt
  binding, freeze/revoke/destroy, evidence/source graph, RunAcceptance, public
  API/CLI, deployment identity.
- **Admissible evidence:** focused Store replay/restart and auth tests; Linux
  integration for the fixed probes and concurrent capsules; CI/SBOM; exact
  deployed identity; authenticated public route/CLI parity; computer restart
  byte-equivalence; Texture transclusion/source-open; cancellation and no-effect
  proof. Local Darwin/stubs and source reasoning cannot prove Linux physics.
- **Rollback:** revert additive readers/writers/routes/CLI/source/acceptance
  gates, preserve any optional evidence fields as inert unknown retained
  history, do not delete/migrate receipts, keep old rows incomplete and effects
  OFF. F1 and F2 each need their own rollback commit/ref.
- **Heresy delta:** `discovered`—public capsule effects and scratch receipts
  were incomplete and not computer-restart durable; `introduced`—none yet;
  `repaired`—none until fresh deployed F2 proof.

F1 and F2 are separate landing loops. Do not start F2 capture before F1 Store
validation/projection is deployed and restart-proved; do not let F1, a mode
boolean, arbitrary shell narrative, or a runtime-authored generic assertion
claim isolation.
