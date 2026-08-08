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
