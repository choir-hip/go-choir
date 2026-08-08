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

## Next safe action

Update the active Definition's sole `now` card to reference this landed receipt,
repair its deleted problem citer, reconcile exact source/deploy/worktree
identity, then build and test one frozen A–C candidate. Do not register the
Texture affordance until store-side validation, atomic turn, readers, replay,
and the negative matrix are all present in that same candidate.
