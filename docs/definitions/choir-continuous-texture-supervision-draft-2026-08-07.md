---
definition_version: 2
definition_id: choir-continuous-texture-supervision-draft-2026-08-07
execution_mode: draft_non_executable

start:
  captured_at: 2026-08-07T20:10:30Z
  source:
    canonical_ref: main@f64784e88b42abbf7d87fee058c989537b686d58
    deploy_identity: 460c142394e12b6e307949d0180da08d1b058745 observed at https://choir.news/health
  worktree_inventory:
    status: reconciled
    evidence_ref: 2026-08-07 read-only git status and git worktree inventory
    preservation_rule: Preserve every non-primary worktree and all unrelated WIP; this draft owns only its Definition, review evidence, and three registry entries.
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: owner-and-current-session
      touch: goal_owned
      paths_or_digest: [docs/definitions/choir-continuous-texture-supervision-draft-2026-08-07.md, docs/ACTIVE.md, docs/mission-graph.yaml, docs/doc-authority-manifest.yaml]
      recovery: revert the docs-only draft commit
    - path: /Users/wiz/.codex/worktrees/eb3c6a2a-cb9f-4067-8cd8-e8ec6224cb6f/go-choir
      status: dirty
      class: other_agent_wip
      owner: unknown
      touch: forbidden
      paths_or_digest: [.context/]
      recovery: leave in place
    - path: /Users/wiz/go-choir-terminal-outcome-closure
      status: dirty
      class: user_wip
      owner: unknown
      touch: forbidden
      paths_or_digest: [internal/objectgraph/dolt_store.go, internal/objectgraph/registry.go, internal/objectgraph/store.go, internal/store/graph_store.go, internal/store/store.go]
      recovery: leave branch autoputer-terminal-outcome-closure and worktree in place
    - path: clean secondary worktrees listed by the 2026-08-07 inventory
      status: clean
      class: other_agent_wip
      owner: unknown
      touch: forbidden
      paths_or_digest: [/private/tmp/choir-head-review, /Users/wiz/.codex/worktrees/5fbb817f-e1ac-4f37-8976-9ec4e94afb99/go-choir, /Users/wiz/go-choir-autoputer-v2, /Users/wiz/go-choir-s0-ratchet, /Users/wiz/go-choir-s3i17, /Users/wiz/go-choir-worktrees/chatgpt-search-provider]
      recovery: leave in place
    - path: prunable worktree registrations listed by the 2026-08-07 inventory
      status: unknown
      class: generated_temp
      owner: unknown
      touch: forbidden
      paths_or_digest: [choir-g1-judge, choir-g1-review, choir-g1-review-22a2, go-choir-architecture-recovery, go-choir-ci-doccheck-pr1, go-choir-ci-goal, go-choir-ci-integration-owner, choir-g1-review-yheq91cz]
      recovery: leave registration metadata untouched
  candidates:
    - id: continuous-texture-supervision-definition-v1
      ref: main working tree
      base: f64784e88b42abbf7d87fee058c989537b686d58
      scope: [docs/definitions/choir-continuous-texture-supervision-draft-2026-08-07.md, docs/ACTIVE.md, docs/mission-graph.yaml, docs/doc-authority-manifest.yaml]
      disposition: active
      evidence_ref: frozen review digest 880c00cb8f7a6c546825689a7ba37c02a551f897a93080f5e1e2dbbda2cb89ed
  observed_artifact:
    - claim: Texture's production registry omits update_coagent although its profile allows coagent tools and its prompt describes Researcher follow-up.
      evidence_ref: internal/agentcore/tool_profiles.go InstallDefaultAgentTools; internal/agentprofile/agentprofile.go PolicyFor
    - claim: Texture may spawn only Researcher; adding Super to AllowedDelegateTargets would wrongly allow arbitrary Super creation rather than address the one persistent Super.
      evidence_ref: internal/coagentowner/spawn_tool.go RegisterSpawnTool; internal/agentcore/runtime.go persistentSuperAgentID
    - claim: The typed coagent_source_packet.v1 path carries producer UUID, trajectory, work-item, payload-digest, disposition, and wake identity, but target authorization is not Texture-safe.
      evidence_ref: internal/agentcore/tools_worker_update.go; internal/agentcore/tools_researcher.go; internal/types/evidence.go
    - claim: Target lookup can fail open when AgentRecord lookup errors, and the generic resolver self-targets a Texture channel before honoring explicit agent_id.
      evidence_ref: internal/agentcore/tools_worker_update.go target lookup; internal/agentcore/tools_researcher.go resolveCoagentFindingsTarget
    - claim: Researcher target resolution accepts an arbitrary owner-scoped texture:<doc_id> shape without proving requester, document, or trajectory binding.
      evidence_ref: internal/agentcore/tools_researcher.go resolveResearcherFindingsTarget
    - claim: Lifecycle QueueLifecycleUpdate accepts only Texture targets, while persistent Super drains the legacy mailbox; a lifecycle Texture can therefore reach neither a bound Researcher nor persistent Super through one accepted durable path.
      evidence_ref: internal/store/lifecycle.go QueueLifecycleUpdate; internal/store/store.go DispatchWorkerUpdate; internal/agentcore/super_controller.go reconcilePersistentSuperActor
    - claim: Researcher, Super, and CoSuper updates can already wake Texture and be incorporated into immutable canonical revisions.
      evidence_ref: internal/textureowner/texture_controller.go; internal/textureowner/tools_texture.go; internal/agentcore/coagent_update_packet.go
    - claim: A successful Texture patch or rewrite terminates the activation, and inbound update activations mechanically require a canonical write; this prevents post-revision redirection and can create bogus no-op revisions for redundant evidence.
      evidence_ref: internal/agentcore/runtime.go executeWithToolLoop; internal/toolregistry/toolloop.go; internal/textureowner/tool_loop_policy.go; internal/textureowner/tools_texture.go
    - claim: Owner correction is split: /revise dispatches a legacy packet, while a direct canonical revision changes the head without reliably waking the lifecycle Texture actor.
      evidence_ref: internal/textureowner/texture_agent_revision.go deliverOwnerRevisionToTextureActor; internal/textureowner/texture.go handleTextureCreateRevision
    - claim: Persistent Super and capsule-scoped implementation/verifier CoSuper authority already exist, but lifecycle-requested effects-capable activation is refused while effects are OFF.
      evidence_ref: internal/agentcore/super_controller.go; internal/agentcore/runtime.go StartCoagentRun; internal/agentcore/tools_capsule.go; internal/capsule/roles.go
    - claim: The deleted request_super_execution was a same-channel ChannelCast with no durable trajectory accounting; restoring that implementation would restore the defect.
      evidence_ref: git commit 7b7bba73 historical Texture tool source; removal 0df1412312de; docs/archive/choir-architecture-review-next-moves-2026-06-11.md
  unknowns: []

start_corrections:
  - corrected_at: 2026-08-07T20:34:47Z
    evidence_ref: docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md
    correction: The primary worktree was clean at start capture and became dirty-but-goal-owned when the four-file candidate was frozen. Review reproduced digest 880c00cb8f7a6c546825689a7ba37c02a551f897a93080f5e1e2dbbda2cb89ed; the repaired candidate adds this evidence file and awaits a new self-normalized digest.
    consequence: Start remains immutable; current candidate and reconciliation below, not the start inventory, carry the reviewed dirty state.

finish:
  deliver: Texture continuously supervises one durable trajectory by revising canonical semantic state and issuing mechanically authorized controls to its own Researchers and its owner's one persistent Super; Super may coordinate assigned capsule-only CoSupers and returns intermediate evidence until Texture settles or redirects the work.
  artifact: An authenticated staging trajectory with at least three causally distinct Texture revisions and two downward control cycles, including a Texture-originated Researcher follow-up, a Texture-originated persistent-Super execution request, a Super/CoSuper result incorporated by Texture, durable work/update identities, and an owner-inspectable latest Texture version.
  acceptance:
    - action: Run focused role-registry, fail-closed target authorization, direction-specific lifecycle reducer, atomic Texture transition, race, replay, owner-correction, cancellation, and capsule capability tests.
      proves: Allowed calls work; lookup errors and cross-owner, cross-computer, cross-document, cross-trajectory, arbitrary-Super, direct-CoSuper, non-lifecycle Texture, verifier-mutation, duplicate, stale-head, and late-result calls fail without weakening existing authority.
      evidence_class: focused local contract and race tests
    - action: Reconstruct pending Researcher and Super controls after process restart; replay equal and conflicting command/packet identities; replay pre-cutover fixtures.
      proves: One lifecycle reducer/projection is authoritative, ordered outbound digests are stable, equal retry is idempotent, conflicting retry fails, old state remains readable, and no new Texture traffic enters the legacy mailbox.
      evidence_class: local restart, reducer, and compatibility proof
    - action: Drive prompt bar -> Texture v1 -> Researcher partial result -> Texture v2 and focused Researcher redirect -> persistent Super intermediate result -> Texture v3 and changed Super direction -> networkless implementation CoSuper -> evidence-only verifier CoSuper -> latest Texture on staging through authenticated product APIs.
      proves: Repeated bidirectional supervision uses real actors, lifecycle work/control, and one disposable capsule rather than manually started workers, an immediate generic update tool, or one terminal round trip.
      evidence_class: deployed authenticated product-path proof
    - action: Leave Researcher and Super controls pending, passivate both actors, perform an approved no-SSH same-build staging process restart, and continue the same owner/computer/document/trajectory/work/actor/update identities.
      proves: Pending state, cold reconstruction, and exact causality survive process and residency boundaries.
      evidence_class: deployed no-SSH restart and durability proof
    - action: Commit a direct owner revision through the deployed editor while Texture is parked; separately submit a natural-language revise request; then inspect the next revision and downstream controls.
      proves: Canonical owner edits remain immediate, both correction semantics wake through lifecycle authority, and subsequent supervision binds the corrected head.
      evidence_class: deployed owner-control proof
    - action: Cancel in-flight Super/CoSuper work, deliver a real delayed result and exact retry after cancellation, and fetch trajectory/update/run evidence.
      proves: The result is late and evidence-only; no obligation reopens and no actor wake, semantic revision, or accepted state follows.
      evidence_class: deployed failure-semantics proof
    - action: Compare canonical computer event heads, self-development operation state, materialization, checkpoint, route generation, and relevant host projections before and after capsule work; inspect the latest Texture in the UI against typed packets and Trace.
      proves: The owner sees semantic state without command-log leakage, while verifier and capsule work cause no computer-event, finalization, host, or promotion effect.
      evidence_class: deployed human inspection and no-effect proof
  rollback: Keep all schemas additive and replay-compatible; leave self-development materialization, acceptance, checkpoint, and route effects OFF; revert the behavior commit through origin/main and CI to the last accepted runtime, retaining new packets as pending/late rather than falsely delivered. If lifecycle-native addressing cannot be made single-authority, discard the candidate and revise this Definition instead of adding a bridge or second queue.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]
  not_done_when:
    - The draft lacks owner ratification and three-registry promotion.
    - Only prompt text, registry presence, mocks, local tests, or model narration says the loop works.
    - Acceptance manually starts Researcher, Super, or CoSuper instead of Texture addressing them through product behavior.
    - Only one worker round trip or one post-worker Texture revision exists.
    - New Texture controls write the legacy mailbox, create a second causal log, or permit two stores to disagree about pending/delivered state.
    - Texture can address CoSuper, a foreign Researcher, or an arbitrary Super; Super or verifier CoSuper gains host mutation.
    - A redundant or unusable packet forces a duplicate semantic revision or disappears without a durable disposition.
    - Effects-capable work can materialize, accept, checkpoint, route, or mutate the host while those effects remain OFF.
    - Target lookup errors, a non-lifecycle Texture, or a direct generic update call can enqueue privileged work.
    - One field or reducer rule ambiguously treats producer-owned report work as target-owned control work.
    - A Texture control can commit or wake independently of its revision/no-change/wait/block transition.
    - Direct owner revisions wait for a provider, or owner head commit and actor wake are separate failure domains.
    - Verifier CoSuper can call record_self_development_verification, append a computer event, mutate updater state, or propose an effect.
    - The disposable capsule has host networking or any capability not bound to the exact run/capsule/slot.

boundaries:
  mutation_class: red
  authority_sources: [AGENTS.md, docs/choir-doctrine.md, docs/agent-product-doctrine.md, docs/computer-ontology.md, docs/standing-questions.md, docs/supervision-protocol.md, docs/texture-agentic-invariants-2026-06-13.md, owner direction to draft and review this Definition]
  must_preserve:
    - Texture is the sole canonical document writer among agents and the delegated semantic controller; direct owner revisions remain canonical, while operational directives remain typed packets, work items, and Trace.
    - The lifecycle trajectory, work-item, worker-update object, event sequence, and snapshot reducers are the single authority for new supervision state; no new event log, service, workflow engine, or dual mailbox write.
    - Producer reports and downward controls are distinct reducer commands: producer_work_item_id belongs to the reporting caller; target_work_item_id belongs to the addressed actor; no sender settles target work.
    - Texture may spawn Researcher only, address only its bound Researchers and exact super:<owner> on the current computer, and never address CoSuper directly.
    - Target lookup is fail-closed; channel shape, prompt text, caller-supplied role, and model-authored direction are never authority. Non-lifecycle Texture cannot send this control.
    - The Super identity is one persistent non-lifecycle actor per owner/computer; Super may be an exact lifecycle control target/assignee without becoming a generic lifecycle actor class.
    - A Texture revision/no-change/wait/block outcome, inbound dispositions, target-work changes, and ordered outbound controls commit in one conditional object-graph batch; target wake happens only afterward.
    - Direct owner revisions remain immediately canonical and atomically record a lifecycle correction cursor/obligation; natural-language revise requests remain Texture-authored decisions.
    - Implementation CoSuper is run-bound, assigned-capsule-only, and networkless; verifier CoSuper has immutable evidence access only and returns a typed packet without self-development finalization.
    - Producer and control identity is runtime-derived; owner, computer, document, trajectory, producer work, target work, target, action class, ordered payload digest, and idempotency are validated before durable enqueue.
    - Canonical event/private payload, encryption, Trace/evidence, cancellation, late-result, restart replay, run acceptance, and exact staging identity contracts do not weaken.
    - Semantic delegation remains agentic; no keyword oracle, forced first tool, required role choreography, polling loop, or trivial revision hides background work.
  excluded:
    - Restoring the historical same-channel request_super_execution implementation, its required-first-tool behavior, or the current immediate generic update_coagent implementation for Texture.
    - Adding a generic supervision subsystem, generic actor-mailbox migration, second tape, new public supervision API/CLI, TLA control plane, VM/route controller, or broad write-mode flag.
    - Promoting persistent Super to a generic lifecycle actor class; new Texture traffic through DispatchWorkerUpdate; dual delivery cursors.
    - External coding-agent integration, fleet promotion, ComputerVersion materialization, checkpoint publication, route CAS, self-development verification/finalization/acceptance, or canonical event append from capsule proof.
    - UI redesign, publication/Wire work, and unrelated runtime cleanup.
  protected_surfaces: [Texture canonical writes and source graph, lifecycle trajectory/work/update reducers and conditional object-graph batch, actor mailbox/passivation/restart, private payload encryption and audit, role tool registries and fail-closed target authorization, persistent Super identity and inbox, capsule capability/slot/network namespace enforcement, canonical computer event chain, self-development operation and updater root, cancellation and late results, run acceptance, provider/model calls, deployment routing]
  completion_evidence_floor: [problem-documenting docs-only commit before repair code, frozen implementation candidate, independent security and architecture review, focused race and replay tests, full applicable CI, exact staging commit identity, authenticated repeated-cycle and same-build restart proof, owner-correction proof, actual late-result proof, before/after no-effect receipts, owner-visible Texture inspection, rollback receipt]

measures:
  - name: target_authority
    kind: gate
    baseline: Texture has no registered update_coagent; generic resolver is unsafe if merely enabled.
    desired: Every accepted outbound packet proves exact caller and target owner/computer/document/trajectory/work binding; all skip-level and foreign targets fail.
    decision_use: Blocks implementation or landing on any authority bypass.
    cannot_prove: Semantic usefulness or deployed continuity.
  - name: single_delivery_authority
    kind: gate
    baseline: Lifecycle updates target Texture while persistent Super consumes legacy mailbox rows.
    desired: New Texture-originated control uses one lifecycle-authoritative enqueue/replay/disposition path and no legacy dual-write.
    decision_use: Rejects compatibility bridges that create two pending/delivered truths.
    cannot_prove: Human comprehension of Texture.
  - name: revision_control_causality
    kind: gate
    baseline: Upward packet-to-revision exists; downward post-revision control does not.
    desired: At least two distinct downward controls and at least two later revisions each cite consumed packet/update cursors on one trajectory.
    decision_use: Distinguishes a loop from one-shot generation.
    cannot_prove: Long-horizon quality beyond the accepted trajectory.
  - name: authority_negative_matrix
    kind: gate
    baseline: Registry tests cover Super/CoSuper names but not Texture addressing or slot-complete invocation behavior.
    desired: Role and slot matrix proves forbidden host, spawn, target, capsule, and verifier operations fail at call time.
    decision_use: Blocks removal of effects-capable guard until equivalent narrower enforcement exists.
    cannot_prove: Provider quality.
  - name: owner_read_amplification
    kind: telemetry
    baseline: unknown
    desired: Multiple grounded revisions and control turns may occur between owner reads while the latest Texture remains sufficient to resume supervision.
    decision_use: Guides later cadence and compaction tuning only.
    cannot_prove: Mission completion or correctness.

now:
  status: checkpoint_incomplete
  slice: Await owner ratification of the frozen non-executable Definition.
  question: Will the owner ratify this exact direction-specific lifecycle control, atomic Texture transition, and networkless evidence-only CoSuper boundary?
  reconciliation:
    observed_at: 2026-08-07T20:34:47Z
    source_ref: f64784e88b42abbf7d87fee058c989537b686d58
    deploy_identity: 460c142394e12b6e307949d0180da08d1b058745
    authority_identities: [docs/choir-doctrine.md@f64784e8, docs/agent-product-doctrine.md@f64784e8, docs/supervision-protocol.md@f64784e8, docs/texture-agentic-invariants-2026-06-13.md@f64784e8, docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md]
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: 2026-08-07 reviewed candidate receipt; primary worktree dirty only in five goal-owned docs
    status: reconciled
  candidate:
    id: continuous-texture-supervision-definition-v3
    state: frozen
    ref: >-
      SHA-256 in scope-list order: for each path, hash its UTF-8 path bytes, one
      NUL byte, its file bytes, and one NUL byte; before hashing this Definition,
      replace only the digest field value after sha256-self-normalized: with SELF.
    owner: current orchestrator
    base: f64784e88b42abbf7d87fee058c989537b686d58
    digest: sha256-self-normalized:ff116061fb7e69c430d34d0fba345ee705581b19016becc056535ded2d3a14b3
    scope: [docs/definitions/choir-continuous-texture-supervision-draft-2026-08-07.md, docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md, docs/ACTIVE.md, docs/mission-graph.yaml, docs/doc-authority-manifest.yaml]
  decision:
    selected: Preserve producer-report QueueLifecycleUpdate; add direction-specific lifecycle controls with target work, an exact persistent-Super opener, and one atomic Texture apply transition; keep Super non-lifecycle and CoSuper networkless/evidence-only.
    kind: architecture
    status: proposal
    source: orchestrator
    evidence_ref: docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md
    owner_ratification_ref: none
    recorded_at: 2026-08-07T20:34:47Z
    consequence: All eight completed repaired-candidate reviewers accepted the substantive architecture; five required only the candidate-identity wording repair now applied. No runtime implementation, lifecycle-profile refusal change, registry promotion to active, or /goal execution is authorized.
  evidence_refs: [docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md, reviewed source commit 6681606d4e0f14e83dab89bb808862db82cdd21b]
  blocker_or_risk: Owner ratification is absent. Implementation must discard and re-authorize the capsule slice if evidence-only networkless isolation cannot be separated from self-development finalization.
  next_action: Owner ratifies or rejects candidate v3 against its exactly specified self-normalized digest; only ratification may promote it to an executable /goal.

receipts:
  - id: continuous-supervision-definition-review-v1
    boundary: define
    commit_or_artifact: sha256:880c00cb8f7a6c546825689a7ba37c02a551f897a93080f5e1e2dbbda2cb89ed
    proof_refs: [docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md]
    rollback_ref: main@f64784e88b42abbf7d87fee058c989537b686d58
    disposition: Six REPAIR, two ACCEPT, one route failed before review; blockers adjudicated into candidate v2.
    problem_ref: this Definition observed_artifact
    authorization_ref: owner request to draft and review only
    candidate_or_evidence_refs: [docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md]
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: 460c142394e12b6e307949d0180da08d1b058745 observation only
      deployed_acceptance: not_applicable
    registry_conformance_ref: draft blocked/non-executable in all three registries
  - id: continuous-supervision-definition-review-v2
    boundary: challenge
    commit_or_artifact: git:6681606d4e0f14e83dab89bb808862db82cdd21b
    proof_refs: [docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md]
    rollback_ref: main@f64784e88b42abbf7d87fee058c989537b686d58
    disposition: Eight completed reviewers accepted the substantive repair; five REPAIR verdicts isolated an underspecified digest construction, three returned ACCEPT, and one route failed before review. Candidate v3 applies the exact byte-level identity repair.
    problem_ref: this Definition observed_artifact
    authorization_ref: owner request to draft and review only
    candidate_or_evidence_refs: [reviewed source commit 6681606d4e0f14e83dab89bb808862db82cdd21b, docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md]
    landing:
      source_commit: 6681606d4e0f14e83dab89bb808862db82cdd21b
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/31216848206 success
      deploy_ref: skipped_docs_only
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: draft blocked/non-executable in all three registries

view:
  path: none
  generator: none
---

# Continuous Texture Supervision

## Problem and cause

The durable loop is only half connected. Researcher, Super, and CoSuper packets
can reach Texture and cause immutable semantic revisions. Texture cannot use the
same accepted lifecycle authority to redirect a bound Researcher or assign work
to the one persistent Super. Merely registering `update_coagent` would expose an
unsafe generic resolver, not repair the loop.

The missing edge came from a correct deletion with an incomplete replacement.
The old Super request was a same-channel cast that bypassed trajectories and was
removed during the durable-lifecycle cutover. The typed packet, work-item,
actor-wake, persistent-Super, and capsule pieces now exist, but their authority
boundaries do not join. This mission connects those pieces; it does not restore
the deleted channel cast or build another supervision platform.

## Target capability contract

| Caller | May create | May address | Must never do |
|---|---|---|---|
| Texture | Researcher work; one persistent-Super work obligation through a typed opener | Researchers already bound to its document/trajectory; exact `super:<owner>` | Spawn Super/CoSuper, address CoSuper, mutate host/capsule, or smuggle commands into canonical prose |
| Researcher | Nothing | Requesting `texture:<doc_id>` | Execute, write, spawn, or address arbitrary Texture/Super identities |
| Super | Researcher and assigned implementation/verifier CoSuper work | Requesting Texture and its own children through bound work | Shell/write host state, execute capsule commands, accept/materialize/route |
| Implementation CoSuper | Nothing | Owning Super/Texture result path through bound work | Host execution, another capsule, acceptance/route, verifier verdict |
| Verifier CoSuper | Nothing | Owning Super/Texture result path through bound work | Capsule mutation, implementation, acceptance/route |

Spawn authority and message-address authority remain separate. Texture gaining a
persistent-Super address must not add Super to `AllowedDelegateTargets`.

## Proposed substrate cutover

The new path remains in the existing lifecycle worker-update object, event, and
snapshot authority, but it does not overload one work relationship. Existing
`QueueLifecycleUpdate` retains upward producer-report semantics:
`producer_work_item_id` is open and assigned to the authenticated caller, and
only that producer may report its disposition. A direction-specific lifecycle
control command carries a runtime-derived `target_work_item_id`; the exact target
owns that open obligation, and the sender cannot settle it. Direction is derived
from caller, target, packet kind, and work bindings, never model-authored.

The persistent Super remains the existing non-lifecycle `super:<owner>` actor,
scoped by owner and computer. It may be admitted only as that exact lifecycle
control target/assignee; it is not promoted into a generic lifecycle actor
class. Its reconciler reads pending lifecycle controls by target identity.
Legacy mailbox rows remain compatibility input for old non-lifecycle work, but
new Texture traffic never writes or marks delivery there.

A Texture-only typed Super-work opener creates or reuses target work for the
exact persistent Super and queues `kind=execution_request` with validated
actions. A continuation can address only existing target work for a bound
Researcher or that Super. These are operation variants over the atomic Texture
transition below, not independently committing calls and not additions to
`AllowedDelegateTargets`. The current immediate generic `update_coagent` is not
exposed to Texture.

One Texture-specific conditional lifecycle apply commits expected lifecycle
version and document head plus exactly one semantic outcome:

1. a semantic revision;
2. an explicit no-semantic-change disposition;
3. a deliberate wait; or
4. a durable block.

The same object-graph batch commits inbound dispositions, target-work open/reuse,
zero or more validated ordered outbound controls, events, and command receipt.
The ordered outbound set and target-work bindings participate in its replay
digest. `patch_texture`, `rewrite_texture`, the Texture form of
`update_coagent`, and the typed Super opener are affordance/validation views over
this apply; none queues, wakes, or reports durable success independently. Remove
the synthetic self-queue-before-apply convention. Wake targets only after commit;
restart sweep recovers committed packets if wake delivery fails. The activation
may remain terminal afterward, preserving stale-base safety without fake
revisions or escaped directives.

## Execution sequence

### Define — freeze authority before code

1. Preserve the frozen v1 digest and durable panel receipt; adjudicate every
   blocking finding into this repaired candidate.
2. Freeze the repaired self-normalized digest and obtain owner ratification for
   its direction-specific work semantics, atomic apply, and exact capsule-local
   evidence boundary.
3. Only then promote the Definition through all three registries. The docs-only
   Define commit is the required problem-documentation-first checkpoint.
4. If the direction-specific lifecycle command or evidence-only capsule boundary
   is infeasible, stop and revise this Definition; do not improvise a dual-write,
   generic lifecycle actor migration, broad refusal deletion, or smaller artifact.

### Implement A — make addressing mechanically safe

1. Separate spawn policy from message-address policy in `internal/agentprofile`;
   Texture remains spawn-Researcher-only.
2. Make every target lookup error a hard refusal. Delete channel/self inference
   from Texture authority; reject non-lifecycle Texture, cross-document/
   trajectory/owner/computer targets, arbitrary Super, and every CoSuper target.
3. Freeze the role/slot/target negative matrix and direction-specific packet
   identities before registering any Texture affordance.
4. Land target validation, atomic lifecycle control, and readers in the same
   candidate. Texture must not temporarily receive the immediate generic tool or
   a legacy persistent-Super fallback.

### Implement B — join the lifecycle inboxes

1. Preserve producer-report `QueueLifecycleUpdate`; add a distinct control
   command in the same reducer/object/event/snapshot authority with exact target
   work and no sender-owned target disposition.
2. Admit bound Researcher and exact owner/computer persistent Super as control
   targets/assignees without making Super a generic lifecycle actor.
3. Make Researcher, Texture, and persistent Super warm/cold reconciliation read
   pending lifecycle state and reconstruct it after restart.
4. The Texture Super opener atomically opens/reuses target work and queues one
   execution request. Equal retry produces one packet/work/activation; conflicting
   payload or ordered-control digest fails.
5. Retain legacy readers only for identified pre-cutover rows; add replay fixtures
   and a detector proving new lifecycle traffic cannot enter them.

### Implement C — close the semantic control transaction

1. Extend `ApplyLifecycleUpdateWithSourceGraph` or add one narrow
   `ApplyLifecycleControlTransition`; do not satisfy atomicity with tool batches.
2. Commit revision/no-change/wait/block, inbound dispositions, target-work
   changes, ordered controls, events, and receipt under one expected-head CAS.
3. Replace required-write behavior with required durable transition. Silent
   prose remains forbidden; redundant evidence receives a disposition without a
   fake version.
4. Preserve owner semantics: direct revisions commit canonical `AuthorUser` head
   and correction cursor/obligation atomically; `/revise` queues a lifecycle
   decision request for a Texture-authored revision. Coalesce wakes, not edits.
5. Update prompts only after the capability exists; expose affordances without
   role choreography, first-tool forcing, or unavailable-effects claims.

### Implement D — connect Super and capsule-only CoSuper work

1. Make persistent Super consume target-bound lifecycle execution requests,
   emit intermediate typed updates, and preserve one owner/computer identity.
2. Do not broadly remove the lifecycle-profile refusal. Add only the exact
   persistent-Super -> assigned CoSuper path after the authority matrix passes.
3. Require same owner/computer/trajectory/open target work, one implementation
   and one verifier slot, one pre-existing run-bound disposable capsule, and
   kernel-enforced network isolation (`CLONE_NEWNET`).
4. Implementation alone receives its opaque capsule mutation capability and may
   freeze an inert bundle. Verifier receives immutable read/evidence access and
   returns a typed packet; it receives no mutation handle and no
   `record_self_development_verification`.
5. Prove no ComputerEventAppender/updater-root/effect-proposal/finalization,
   acceptance, materialization, checkpoint, route, VM, host-path, or owner-decision
   effect. If isolation from current finalization is impossible, stop and move
   this slice to a separately ratified successor without weakening the artifact.

### Land — prove the product loop

1. Freeze and independently review the complete runtime candidate, including
   fail-closed authority, conditional batch, replay, private payload, owner
   correction, cancellation, capsule namespace, and no-effect checks.
2. Run focused/race tests and the applicable full CI shards once.
3. Commit, push `origin/main`, monitor CI and staging deployment, and verify the
   exact deployed commit through `/health`.
4. Run the full authenticated repeated-cycle acceptance, including pending
   packets across passivation and no-SSH same-build process restart, direct owner
   correction and changed direction, in-flight cancellation plus actual late
   result/retry, and before/after protected-state comparison.
5. Fetch exact document/revision, owner/computer/trajectory, producer/target
   work, actor, command/update/digest, run, capsule evidence, cancellation/late,
   and acceptance identities; inspect latest Texture.
6. Close the Definition and registries only after the artifact is fetched,
   effects remain unchanged, and rollback is rehearsed.

## Red-mutation ceremony

**Conjecture delta:** C6/C7 gain a deployed test of durable actors supervising
through trajectory/work authority; they remain active conjectures rather than
being promoted by one success. The proposal removes one H011/H015/H018 instance
without claiming those heresy classes globally repaired.

**Protected surfaces:** canonical Texture writes and owner heads; lifecycle
reducers, conditional object-graph batch, replay, and legacy compatibility;
actor delivery/restart; encrypted private payload and audit; fail-closed
role/target/work authority; persistent Super identity; capsule slot, handle, and
network namespace; canonical computer events, updater state, cancellation/late
results, run acceptance, provider calls, and deployment routing.

**Admissible evidence:** deterministic direction/authority negatives, conditional
batch/race/replay and same-build restart proofs, frozen-candidate independent
review, exact CI/deploy identity, authenticated staging trajectory with owner
correction and actual late result, protected-state before/after receipts,
fetched artifacts/IDs, owner-visible latest Texture, and rollback receipt.

**Rollback:** additive compatibility plus source revert. New packets remain
pending/late and inspectable; they are never marked delivered merely because an
older runtime cannot execute them. No speculative capsule effect can cross into
materialization, checkpoint, route, or host state.

**Heresy delta:** discovered — lifecycle/legacy inbox split, absent Texture tool,
fail-open/self-target/cross-document resolution, producer/target work ambiguity,
two-commit Texture mutation, terminal redirection gap, forced no-op revisions,
split owner correction, and verifier self-development effects. Introduced — none
by this draft. Repaired — none until deployed acceptance proves the cutover.
