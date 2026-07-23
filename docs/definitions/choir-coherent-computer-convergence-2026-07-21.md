---
title: "Converge Choir on One Durable Agentic Computer"
definition_version: 2

start:
  captured_at: 2026-07-21T19:41:58Z
  source:
    canonical_ref: refs/remotes/origin/main@7913a3da0343ee03cf32b7622aaf9f2de35ee887
    deploy_identity: "Node B staging host and active guests reported 832ae951e84400a54bd7f8ef52a312e872b5c3ef; Node A is a proof harness, not staging acceptance"
  worktree_inventory:
    status: reconciled
    evidence_ref: "git worktree list --porcelain plus git status --short --branch for every extant worktree, observed 2026-07-21T19:41:58Z"
    preservation_rule: "Preserve every rejected candidate, unrelated worktree, accepted ComputerVersion, rollback realization, and production recovery image by exact ref. Build only from a clean canonical-main descendant. Never import rejected runtime code or infer deployed identity from checkout state."
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: owner-and-current-session
      touch: read_only
      paths_or_digest: refs/heads/selfdev/g1-round72-signed-activation@5517c2eb5c94678eb4ec323fef2cec34b96f7c6a
      recovery: "Preserve exact remote branch as rejected evidence; never merge or deploy it."
    - path: /tmp/go-choir-architecture-recovery
      status: dirty
      class: goal_candidate
      owner: owner-and-current-session
      touch: goal_owned
      paths_or_digest: "docs-only successor Definition and authority/architecture registry cutover based on origin/main@7913a3da"
      recovery: "Branch selfdev/architecture-recovery; discard or revert without touching runtime state if review rejects it."
    - path: /Users/wiz/go-choir-terminal-outcome-closure
      status: dirty
      class: user_wip
      owner: unknown
      touch: forbidden
      paths_or_digest: "five modified internal/objectgraph and internal/store files"
      recovery: "Preserve in place on autoputer-terminal-outcome-closure; exclude from this mission."
    - path: /Users/wiz/.codex/worktrees/eb3c6a2a-cb9f-4067-8cd8-e8ec6224cb6f/go-choir
      status: dirty
      class: other_agent_wip
      owner: unknown
      touch: forbidden
      paths_or_digest: "untracked .context/"
      recovery: "Preserve in place; exclude from this mission."
  candidates:
    - id: round72-signed-activation
      ref: refs/heads/selfdev/g1-round72-signed-activation@5517c2eb5c94678eb4ec323fef2cec34b96f7c6a
      base: refs/remotes/origin/main@7913a3da0343ee03cf32b7622aaf9f2de35ee887
      scope: [historical_self_development_runtime_candidate]
      disposition: discarded
      evidence_ref: "Frozen G1 panel: mutable root/updater authority, exact inventory, and capsule freeze ingress blockers; branch clean at exact remote ref"
    - id: convergence-definition-docs-01
      ref: /tmp/go-choir-architecture-recovery
      base: refs/remotes/origin/main@7913a3da0343ee03cf32b7622aaf9f2de35ee887
      scope: [docs]
      disposition: active
      evidence_ref: "scripts/doccheck live/full, dashboard parser, and frozen review after candidate commit"
  observed_artifact:
    - claim: "Choir already has persistent computers, embedded Dolt artifacts and trajectories, versioned Texture documents, agents, guest-local capsules, a canonical computer-event path, immutable updater/checkpoint/route components, and public UI/API/CLI surfaces."
      evidence_ref: "Canonical main source plus accepted audited-construction receipts"
    - claim: "Artifact revision, provider run, durable actor, trajectory/work settlement, and authorized effect do not yet compose into one honest product lifecycle."
      evidence_ref: "Texture delayed-research V1-without-V2 trace and Round 72 G1 rejection recorded in the superseded Definition"
    - claim: "Round 72 built and passed focused checks but remained undeployed and rejected because mutable agentcore could request updater authorization using caller-computed commitments without independent accepted-event proof."
      evidence_ref: refs/heads/selfdev/g1-round72-signed-activation@5517c2eb5c94678eb4ec323fef2cec34b96f7c6a
    - claim: "Texture has a broad same-origin API used by the desktop, while the public CLI is mostly read-only and current lifecycle projections can mistake run/passivation signals for document completion."
      evidence_ref: "frontend/src/lib/texture.js; internal/textureowner; cmd/choir; staged delayed-research trace"
    - claim: "Actor updates/snapshots and coagent/work state span overlapping SQLite and Dolt stores; run continuation and run acceptance remain transitional surfaces."
      evidence_ref: "internal/actor; internal/actorruntime; internal/agentcore; internal/types"
    - claim: "Executable identity is fragmented across canonical source, Node B Nix closure/services, guest image, and active guest build."
      evidence_ref: "2026-07-21 Node A/B checkout, systemd executable, health, ownership, and guest-health inventory"
  unknowns:
    - "Which existing store/reducer should own each durable subject/update/work transition after the writer/caller inventory; the invariant is one authority per object, not a preselected database."
    - "The narrowest public snapshot/event/command contract that can prove the lifecycle without freezing a universal Texture schema."

finish:
  deliver: "One stable staging Choir computer executes one honest durable-work lifecycle through a supported public typed product path. An artifact may advance through many revisions; an unresolved obligation survives activation completion and runtime restart; a later typed update wakes a reconstructible durable subject; native work authority alone settles or cancels; desktop and headless clients observe the same resumable state; authorized effects remain separately gated and OFF."
  artifact: "A deployed generic durable-work kernel and public lifecycle contract joining artifact head, durable subject/activation, obligations, typed update dispositions, settlement/cancellation, restart reconstruction, and exact executable identity without using RunID, provider transcript, UI state, or effect receipts as semantic authority."
  acceptance:
    - action: "From a clean client, retrieve a signed/bound no-SSH inventory joining canonical GitHub ref, Node B checkout cleanliness, NixOS closure, service executable paths and embedded commits, deployed commit, guest image/config digests, active ComputerID/realization/epoch, and guest builds."
      proves: "The exercised staging system is exactly identified; missing or conflicting identity refuses acceptance."
      evidence_class: deployed_no_ssh_identity_inventory
    - action: "Through the supported public protocol, create an initial artifact, allow a replaceable activation to revise zero or more times and open one real delayed evidence obligation, then finish/passivate that activation without terminalizing the work."
      proves: "Artifact revision and activation completion do not imply work settlement; no placeholder revision or predefined researcher stage is required."
      evidence_class: deployed_durable_work_lifecycle
    - action: "Restart the relevant runtime and reconstruct the same durable subject, artifact head, obligation, pending-update dispositions, cancellation state, and evidence refs without a RunID or provider transcript as authority."
      proves: "Continuity is durable product state rather than process ancestry or conversational memory."
      evidence_class: deployed_restart_reconstruction
    - action: "Deliver one typed evidence update exactly once; let a new activation incorporate or explicitly reject it in a later revision; settle only after every material obligation and update has a terminal disposition. Replay duplicate and late delivery."
      proves: "Delivery is non-lossy and deterministic and native work authority alone settles."
      evidence_class: deployed_delivery_and_settlement
    - action: "Exercise owner cancellation before the first revision, while waiting, and after update delivery; reconnect both desktop and `choir` clients from an earlier cursor."
      proves: "Cancellation wins, retained revisions/evidence remain inspectable, and both clients observe identical snapshots, events, refusals, and receipts."
      evidence_class: deployed_cancellation_and_protocol_conformance
    - action: "Run one trivial grounded request and one evidence-heavy request through the same operation kernel."
      proves: "The harness permits proportional adaptive depth: no forced workflow for the trivial task and no premature settlement while material evidence obligations remain."
      evidence_class: deployed_adaptive_depth_product_trace
  rollback: "Before deployment, discard the docs/runtime candidate and restore accepted main. After deployment, use the retained prior ComputerVersion/checkpoint/route realization and canonical rollback evidence. Never recover by importing host-local mutable state or the rejected Round 72 branch."
  landing:
    required: true
    environment: staging_node_b_and_choir_news
    required_receipts: [pushed_origin_main_commit, ci, staging_deploy, exact_host_and_guest_identity, deployed_lifecycle, restart_reconstruction, delivery_dispositions, cancellation, ui_headless_conformance, effects_off, rollback_ref]
  not_done_when:
    - "Only architecture prose, a source candidate, local tests, model review, dashboard rendering, or deployment identity is green."
    - "Any run/tool/provider completion can settle work, any update needs an old RunID/transcript, restart loses or duplicates delivery, or UI/headless clients disagree."
    - "Self-development apply, checkpoint/route mutation, MCP, Choir-in-Choir, or a generalized universal Texture schema is introduced as a shortcut."
    - "A dirty candidate lacks disposition or unrelated WIP is modified, deleted, merged, or silently reclassified."

boundaries:
  mutation_class: red
  authority_sources: [owner_direction_2026-07-21, AGENTS.md, docs/choir-doctrine.md, docs/standing-questions.md, docs/agent-product-doctrine.md, docs/computer-ontology.md, this_Definition]
  must_preserve:
    - "Each durable object has exactly one canonical head/reducer; projections and caches cannot acknowledge or settle semantic state."
    - "Artifact revision, durable subject, activation, obligation/work settlement, semantic-event authorization, and applied effect are distinct state machines."
    - "Models and role packages propose facts and actions; deterministic reducers enforce stale-head checks, typed delivery, cancellation, settlement preconditions, and effect authorization."
    - "Rendering never executes or inherits ambient authority. Deeper research comes from adaptive reasoning over durable state, not a larger predefined workflow."
    - "Effects require independent canonical authorization. This mission proves no self-development, updater, checkpoint, or route effect."
  excluded:
    - "Repairing, landing, cherry-picking, or deploying Round 72."
    - "Self-development apply or Genesis import; candidate/worker VM topology; raw vmctl; SSH as product proof; host-local or mutable-branch authority."
    - "Universal Texture schema/lens implementation, MCP, World Wire control, Choir-in-Choir, or a fixed deep-research orchestration graph."
    - "Compatibility shims that preserve dual semantic truth after cutover."
  protected_surfaces: [active_Definition, embedded_Dolt, durable_actor_delivery, artifact_head, trajectory_obligations, work_settlement, Texture_public_lifecycle, ComputerEventAppender, capsule_ingress, updater_apply, deployment_identity]
  completion_evidence_floor: [deployed_no_ssh_identity_inventory, pushed_CI_deploy_identity, deployed_durable_work_lifecycle, restart_reconstruction, deterministic_delivery_and_cancellation, UI_headless_conformance, effects_off_receipt, independent_frozen_candidate_review, rollback_ref]
  conjecture_delta:
    retired: "Self-development is the next direct implementation/deployment mission."
    active: "A small generic durable-work cutover removes more uncertainty and reusable complexity than another self-development repair round."
    deferred: "Texture as universal hypermedia lens, exact storage selection, stateless versus addressable activation optimization, MCP, and Choir-in-Choir remain hypotheses until product evidence selects them."
  heresy_delta:
    discovered: "The former mission optimized the hardest effect path before the generic artifact/actor/work authority seams were coherent; Texture research depth and self-development authorization failed at that same boundary."
    introduced: none
    repaired: "Not until the deployed generic lifecycle passes and competing exercised-path authorities are removed. This docs cutover repairs mission authority only."

kernel_contract:
  frozen_scope: "One Texture-backed durable-work lifecycle on one stable computer. This is a code-free implementation contract, not evidence that the lifecycle exists."
  digest_frame: "SHA-256 over exact UTF-8 bytes beginning with the k in 'kernel_contract:\\n' and ending with the final newline immediately before the exact delimiter '\\nmeasures:\\n'; no YAML parse, line-number range, frontmatter tail, or newline normalization."
  key_contract:
    scope: "Every canonical lookup and mutation is owner-scoped. Durable keys are artifact=(OwnerID,ComputerID,DocID), revision=(OwnerID,ComputerID,DocID,RevisionID), subject=(OwnerID,ComputerID,AgentID), trajectory=(OwnerID,ComputerID,TrajectoryID), work=(OwnerID,ComputerID,TrajectoryID,WorkItemID). No unscoped GetAgent or object lookup may authorize a transition."
    operation_identity: "Start key is (OwnerID,ComputerID,CommandID) and its StartRequestDigest covers canonical command bytes (prompt/input, initial artifact, requested material obligations, and policy-independent identifiers). Exact digest retries return the stored response; reuse with a different digest is a durable conflict. TrajectoryID is allocated once and never derived from RunID. An update key is (OwnerID,ComputerID,TrajectoryID,TargetAgentID,ProducerAgentID,ProducerUpdateID); ProducerUpdateID is stable producer-command identity and MUST NOT contain RunID, SourceRunID, activation ID, timestamp, or provider identity. PayloadDigest covers canonical packet payload bytes excluding delivery/run bindings. Conflicting key/digest reuse refuses; exact retry returns stored disposition."
    versions: "Document head revision, subject lifecycle version, trajectory version/status, work-item version/status, and update disposition/version are reducer-checked CAS inputs. Terminal records are immutable except for projection rebuild metadata outside their signed/content hash."
  authority_inventory:
    artifact_head:
      canonical_owner: "Embedded-Dolt object graph: choir.texture_document.CurrentRevisionID plus immutable choir.texture_revision/source objects."
      only_writer: "CommitTextureHeadAuthority is the sole private conditional head reducer. StartDurableWorkAuthority, ApplyTypedUpdateAuthority, and every manual/import/merge/restore revision command call it inside their transaction; Store.CreateRevision/CreateRevisionWithSourceGraph become guarded wrappers or private. Source graph, immutable revision, parent/document edges, stale-parent check, trajectory binding check when present, and head CAS commit together."
      mutation_limits: "ManualTextureDocumentAuthority may create a headless owner/computer-scoped document or change title/archive projection only; it cannot create a lifecycle subject, trajectory, work, revision, or terminal state. Store.UpdateDocument/UpdateTextureDocumentOG cannot accept CurrentRevisionID. PatchRevisionMetadata cannot mutate immutable revision/content/provenance metadata; operational notes are projection objects. Publication mirrors are post-commit projections."
      projections: "Texture DTO/streams, Trace/events, publication mirrors, and public manifests may report revisions but never advance the private head."
    durable_subject:
      canonical_owner: "Embedded-Dolt AgentRecord keyed by (OwnerID,ComputerID,AgentID); Texture AgentID is deterministic texture:<DocID> within that owner/computer."
      only_writer: "StartDurableWorkAuthority creates/verifies a bound Texture subject and advances LifecycleVersion/LastReducerSeq. AgentRecord fields OwnerID, ComputerID, AgentID, Profile, Role, ChannelID, SandboxID, CreatedAt, UpdatedAt, LifecycleVersion, and LastReducerSeq are reducer-only; CreatedAt is immutable and UpdatedAt is reducer-derived. AgentRecord holds no settlement SubjectRefs and no activation/health allowlist: trajectory refs live only on TrajectoryRecord; RunRecord, AgentMutation, and actor projections own activation. UpsertAgent/recovery route bound Texture subjects through the reducer; other-profile subjects cannot write texture:<DocID>."
      activation_boundary: "RunRecord, AgentMutation, provider state, and actor SQLite memory/processed_at describe replaceable activations only. Run terminality, passivation, snapshots, and transcripts cannot settle the subject or acknowledge evidence."
    typed_update:
      canonical_owner: "Embedded-Dolt object graph choir.worker_update under the key and payload-digest rules above."
      state_machine: "Disposition is pending, incorporated, rejected, cancelled, or late. Pending includes never-delivered and delivered-to-activation evidence; delivery never changes semantic disposition. The other four states are terminal and carry exactly one artifact/work/refusal/terminal-trajectory ref plus reducer sequence. Exact duplicates return the record; conflicting duplicates refuse."
      only_writers: "AppendTypedUpdateAuthority creates pending or post-terminal late; ApplyTypedUpdateAuthority writes incorporated/rejected; CancelTrajectoryAuthority changes every non-terminal update to cancelled. Delivery/cursor/checkpoint code has no disposition write."
      projections: "Actor SQLite rows, DeliveredToRunID, coagent mailbox cursors, texture_controller_checkpoint, texture_agent_mutations, run/revision worker_updates_* metadata, channel messages, and UI cursors are delivery/recovery/audit projections. They are deleted or rebuilt from canonical updates and may never acknowledge incorporation, rejection, cancellation, or settlement."
    work_item:
      canonical_owner: "Embedded-Dolt WorkItemRecord keyed by the tuple above."
      only_writers: "StartDurableWorkAuthority creates initial material work; OpenWorkItemAuthority adds a task-declared obligation; AmendWorkItemAuthority CAS-updates non-terminal objective/details/budget fields; ResolveWorkItemAuthority writes terminal completed or refused plus durable result/ref; ApplyTypedUpdateAuthority may invoke Amend/Resolve in its same transaction; CancelTrajectoryAuthority writes cancelled. CreateWorkItemOG, UpdateWorkItemDetails, and UpdateWorkItemStatusOG become private batch primitives. Runs, sweeps, and publication may propose mutations but cannot write work directly."
    obligation_and_settlement:
      canonical_owner: "Embedded-Dolt TrajectoryRecord plus canonical WorkItem and worker-update records."
      only_writers: "TrajectoryRecord is the sole home for settlement SubjectRefs. RecordTrajectorySubjectRefsAuthority writes refs on a live trajectory with CAS/outbox; Start/Apply/Resolve may invoke it in their transaction. Connect existing CancelTrajectoryAuthority/CancelTrajectoryAuthorityOG; SettleTrajectoryAuthority is the only settlement entrypoint over TrajectoryObligations/EvaluateTrajectorySettlement, which read only TrajectoryRecord.SubjectRefs. Raw UpdateTrajectorySubjectRefs and UpdateTrajectoryStatus(...settled|cancelled) are private reducer primitives."
      settlement_rule: "The stored SettlementRuleVersion names a closed predicate vocabulary. In one transaction Settle re-reads live trajectory, zero open work, zero non-terminal updates, and every required TrajectoryRecord.SubjectRefs entry; it captures TerminalArtifactHeadRef and writes settled plus event. Missing/unknown predicates or refs refuse."
      cancellation_rule: "Cancellation locks the live trajectory first, assigns reducer sequence, captures TerminalArtifactHeadRef, and atomically writes trajectory cancelled, all open work cancelled, all non-terminal updates cancelled, and event. Committed revisions/evidence remain inspectable pre-cancellation history."
    effect_authorization:
      canonical_owner: "ComputerEventAppender plus corpusd ComputerEventCAS remain the separate per-computer semantic-event authority."
      effects_off: "The lifecycle package receives no capsule, EventEffectAccepted, updater, checkpoint, ComputerVersion, or route-mutation capability. It neither starts nor acknowledges self-development materialization. Existing protected effect implementations remain unchanged."
  reducer_contract:
    storage_boundary: "All semantic lifecycle objects and the durable lifecycle-event outbox live in the same embedded-Dolt object graph. Extend objectgraph.BatchStore.PutBatch with a conditional serializable transaction/CAS seam; do not use compensating deletes as correctness. Aliases, actor SQLite, Trace, EventBus, and platform publication synchronize only after commit."
    ordering: "Reducers lock/compare in deterministic order: owner/computer/trajectory, artifact head, subject, then sorted work/update IDs. Each accepted transaction receives one monotonic per-trajectory ReducerSeq. Cancellation or another terminal transition that obtains the trajectory CAS first wins; losers re-read and return the stored terminal/refusal result."
    retries: "A crash or error before commit leaves no semantic sub-write. After commit, retry by CommandID/update key returns the stored result without reapplying. CAS conflicts are explicit retryable responses; validation/terminal conflicts are durable non-retryable refusals. Post-commit wake and stream notification may repeat or disappear because durable replay closes every gap."
    start: "StartDurableWorkAuthority atomically writes V0 revision/head, durable subject, live trajectory with SettlementRuleVersion and required refs, at least one material lifecycle WorkItem proportional to the request, StartRequestDigest/idempotency receipt, and lifecycle outbox event. It does not force research, delayed evidence, delegation, or any fixed workflow; a request that declares delayed evidence material opens that obligation, while a trivial grounded request need not."
    work: "OpenWorkItemAuthority atomically checks live trajectory, writes a unique open item, bumps trajectory version/sequence, and emits outbox. AmendWorkItemAuthority atomically checks live trajectory and open item, CAS-writes allowed body fields, bumps sequence, and emits outbox. ResolveWorkItemAuthority atomically checks live trajectory/open item and durable result/ref, writes completed or refused, applies any TrajectoryRecord.SubjectRefs, bumps sequence, and emits outbox. Every retry/CAS/cancel rule above applies."
    append_update: "AppendTypedUpdateAuthority atomically writes pending update plus outbox event when the trajectory is live. If the trajectory is already cancelled or settled it writes terminal late with that trajectory ref plus event and does not wake. Only after commit may a best-effort wake target the subject."
    incorporate: "ApplyTypedUpdateAuthority with incorporated atomically rechecks live trajectory/head, writes source graph + immutable revision + edges + head, terminal update disposition, declared WorkItem Amend/Resolve consequence, TrajectoryRecord.SubjectRefs, and event; all or none."
    reject: "ApplyTypedUpdateAuthority with rejected atomically rechecks live trajectory, writes durable refusal/ref, terminal update disposition, declared WorkItem Amend/Resolve consequence, any TrajectoryRecord.SubjectRefs, and event; no revision, all or none."
    cancel: "CancelTrajectoryAuthority performs the cancellation rule above in one transaction. A delivered-but-undisposed update is non-terminal and is cancelled. After the trajectory CAS commits, no revision, incorporation, rejection, work completion, or settlement can commit."
    settle: "SettleTrajectoryAuthority performs the settlement rule above in one transaction and is idempotent. A settlement attempt never repairs predicates or silently closes work."
  transition_contract:
    - "The owner-scoped Texture route calls StartDurableWorkAuthority to create V0, subject, trajectory, and request-proportional material work under one CommandID/StartRequestDigest. OpenWorkItemAuthority adds delayed evidence only when the task declares it material."
    - "Replaceable activations may propose zero or more revisions and passivate; only a reducer commit changes artifact, evidence, work, or trajectory state."
    - "Typed evidence remains pending across delivery and restart until atomically incorporated or rejected. Cancellation terminalizes all non-terminal evidence; post-terminal new evidence becomes late without wake."
    - "Restart reconstructs subject, artifact head, obligations, dispositions, terminality, evidence refs, reducer sequence, and replay cursor from embedded Dolt without RunID, actor SQLite, event bus, UI state, or transcript."
  public_protocol:
    schema: "choir.durable_work.v1 is a narrow lifecycle DTO, not a universal Texture schema. IDs and reads are owner/computer scoped by authenticated context; clients cannot supply another owner."
    start_command: "The existing authenticated POST /api/prompt-bar Texture-selected path must delegate its first materialization to StartDurableWorkAuthority and return CommandID, TrajectoryID, DocID, V0 RevisionID, subject ID, obligation IDs, reducer sequence, and snapshot cursor. A headless cmd/choir start command calls this same endpoint/DTO; no client-side multi-call assembly is accepted."
    snapshot: "GET /api/trajectories/{id} returns a versioned lifecycle snapshot containing artifact head, subject identity/version, activation projection clearly labeled non-authoritative, obligations, update dispositions/refs, trajectory rule/status, reducer sequence, evidence refs, and a watermark captured in the same read transaction."
    replay: "GET /api/trajectories/{id}/events?after=<cursor>&limit=<n> pages the durable outbox without a fixed catch-up cap. Stream connection subscribes to notifications before replay and tails durable outbox from its last committed cursor on every notification, heartbeat/poll interval, and overflow signal; notification payloads never advance the cursor. It repeatedly replays through the latest watermark and reports cursor-expired only with a replacement snapshot cursor. EventBus may lose/coalesce notifications because heartbeat polling and durable replay guarantee eventual catch-up while connected."
    commands: "Cancel and typed-update commands use reducers and return the same snapshot/result DTO with idempotency, expected-version/head, disposition, refusal, and conflict fields. CommitTextureHeadAuthority also owns manual revisions. If no trajectory binding is supplied, a manual revision changes the document head only and cannot alter lifecycle/work state; while a trajectory is live its head CAS emits a lifecycle-visible artifact-head event; a revision bound to a terminal trajectory refuses. After terminality, an unbound manual/new-trajectory revision may advance the current document, but the old trajectory snapshot remains pinned to TerminalArtifactHeadRef and reports any newer CurrentDocumentHead separately."
    deletion: "DELETE /api/texture/documents/{id} calls ArchiveTextureDocumentAuthority. A lifecycle-bound document with a live trajectory refuses until canonical cancellation/settlement. Archive writes an owner-visible tombstone/archive projection and event but preserves document identity, immutable revisions, source graph, decisions, and terminal snapshots. Raw DeleteDocument/ogDelete are private and unreachable for lifecycle artifacts."
    clients: "Desktop and cmd/choir render/print the same DTO and replay protocol. Texture park/passivation maps to activation_parked, never work_completed. UI session/device state and CLI polling remain non-authoritative."
    identity:
      endpoint: "Dedicated authenticated /api/acceptance/execution-identity, not general /api/compute/status. Access requires owner/admin acceptance:read scope; raw host paths, credentials, environment, and unrelated computer inventory are never returned."
      envelope: "choir.execution_identity.v1 contains issuer/key_id, audience, caller nonce, issued_at/expires_at, deployed commit, canonical GitHub ref plus CI-signed clean-tree provenance, host Nix closure digest and role-named executable digests/embedded commits, guest image/config/build digests, ComputerID/RealizationID/epoch, ComputerVersion/route receipt digest, join verdict, and one signature over canonical bytes. Paths are role labels plus digests, not filesystem disclosure."
      trust: "CI/deploy publishes a signed immutable deployment manifest; Node B's configured platform-attestation key joins that manifest to runtime executable/Nix measurements and vmctl's read-only guest/route/realization inventory. Acceptance clients pin the deployment/attestation public-key IDs; rotation requires an overlap manifest signed by an already trusted key. The verifier checks signature, nonce, audience, expiry, CI provenance, target provenance, active host-runtime/package equality, guest build/deployment equality, independent ComputerVersion route identity, and selected-component target equality."
      refusal: "Missing source oracle, manifest, key, measurement, guest/route join, authorization, or any conflict returns a typed fail-closed unsigned error and no acceptance verdict. Collection uses service APIs/manifests only: no SSH and no mutable checkout inspection."
  migration_and_deletion:
    production_caller_table:
      - "artifact | internal/textureowner/coagent_route.go coagentTextureTargetDocument; texture_handoff.go ensureConductorTextureRoute | StartDurableWorkAuthority"
      - "artifact | internal/textureowner/tools_texture.go commitTextureToolEdit/CreateRevisionWithSourceGraph | ApplyTypedUpdateAuthority + CommitTextureHeadAuthority"
      - "artifact | internal/textureowner/texture.go HandleTextureImportMarkdownLineage, create-aliased-initial, HandleTextureRevisions, rebase-user-revision, HandleTextureRestoreRevision; texture_merge.go HandleTextureAcceptMerge | CommitTextureHeadAuthority manual mode; no work/trajectory terminal write"
      - "artifact | internal/store/texture.go CreateRevision/CreateRevisionWithSourceGraph/createRevision; graph_store.go CreateTextureRevisionOG/UpdateTextureDocumentOG; UpdateDocument; PatchRevisionMetadata | private/guarded head or title/projection APIs; delete compensation and immutable metadata mutation"
      - "artifact create/delete | internal/store/texture.go CreateDocument/CreateTextureDocumentOG/DeleteDocument/ogDelete; internal/textureowner/texture.go create/import/alias plus handleTextureDeleteDocument | StartDurableWorkAuthority or ManualTextureDocumentAuthority for create; ArchiveTextureDocumentAuthority for delete; raw physical delete private"
      - "subject | internal/textureowner/coagent_route.go, texture_controller.go, texture_handoff.go UpsertAgent | StartDurableWorkAuthority or bound-subject projection lookup; delete body upsert"
      - "subject | internal/agentcore/runtime.go persistent/coagent UpsertAgent; runtime_persistence.go; email_lifecycle.go | other-profile subject path retained behind owner/computer scope and guard forbidding texture:<DocID>; activation stays RunRecord/AgentMutation"
      - "subject read | internal/store/store.go GetAgent/GetAgentOG; internal/actorruntime/handler.go ownerForAgent; internal/agentcore/tools_researcher.go, tools_worker_update.go, researcher_checkpoint_fallback.go, super_controller.go and runtime.go reconcileAssignedWorkItemActor; internal/textureowner/texture.go and texture_controller.go Start/ReconcileActorWake | replace with GetAgentByScope(OwnerID,ComputerID,AgentID); delivery/work/wake envelopes carry owner/computer; global AgentID or ListAllDocuments scans never authorize a transition"
      - "Texture wake | internal/textureowner/texture_controller.go Start, scheduleTextureWorkerWake, ReconcileActorWake, ReconcileAgentWake | boot enumerates scoped (OwnerID,ComputerID,DocID) records only; queued wake carries that tuple; scoped document/subject lookup; no cross-owner ListAllDocuments or AgentID rediscovery authority"
      - "trajectory creation | internal/agentcore/trajectory.go ensureTrajectory/CreateTrajectoryIfAbsent | StartDurableWorkAuthority for bound Texture route; generic other-profile path guarded out of scope"
      - "work open | internal/agentcore/runtime.go spawn work item; wire_publication.go work-item creation at source/publication/edition paths | OpenWorkItemAuthority with live-trajectory CAS and event"
      - "work close | internal/agentcore/super_controller.go run work completion; wire_publication.go all UpdateWorkItemStatus completion paths | ResolveWorkItemAuthority with durable result/artifact/refusal ref; no raw status write"
      - "work amend | internal/store/trajectory.go UpdateWorkItemDetails and internal/agentcore/wire_publication.go callers | AmendWorkItemAuthority or ResolveWorkItemAuthority in live-trajectory transaction; raw body upsert private"
      - "trajectory terminal | internal/agentcore/wire_publication.go direct cancelled and settled writes; internal/store/trajectory.go UpdateTrajectoryStatus; graph_store.go UpdateTrajectoryStatusOG | CancelTrajectoryAuthority or SettleTrajectoryAuthority; raw primitives private"
      - "trajectory read | internal/agentcore/trajectory.go TrajectoryObligations/EvaluateTrajectorySettlement; api_trajectory.go reads/cancel | connect to canonical snapshot and reducers"
      - "trajectory refs | internal/store/trajectory.go UpdateTrajectorySubjectRefs; internal/agentcore/wire_publication.go recordWirePublicationTrajectoryRef | RecordTrajectorySubjectRefsAuthority; Settle/Evaluate read TrajectoryRecord only"
      - "update append | internal/agentcore/tools_worker_update.go deriveWorkerUpdateID/DispatchWorkerUpdate; researcher_checkpoint_fallback.go synthetic dispatch; internal/textureowner/texture_agent_revision.go, texture_proposals.go, tools_texture.go dispatch | AppendTypedUpdateAuthority; delete SourceRunID-derived identity"
      - "update terminal outcome | internal/store/store.go BindWorkerUpdateTerminalOutcome; internal/agentcore/researcher_checkpoint_fallback.go caller | delete canonical packet mutation; write immutable projection object keyed by update/outcome digest, with no identity/disposition/settlement authority"
      - "update delivery | MarkWorkerUpdateDeliveredOG/MarkWorkerUpdatesDeliveredOG, UpdateRunAndMarkWorkerUpdatesDeliveredOG/rollback, ListCoagentMailboxBacklog; texture_controller_checkpoint, texture_agent_mutations, textureWorkerUpdateCommitSeq/markTextureWorkerUpdatesDelivered, revision worker_updates_consumed/skipped/pending metadata | rebuildable delivery projection or delete; never disposition"
      - "actor late/cancel | internal/actorruntime/handler.go handleCoagentResult and handleCancel; internal/agentcore/super_controller.go reconcileUpdatedCoagentActor | terminal update consumed after reducer with nil handler error; handleCancel projects RunCancelled, never RunFailed, and never impersonates trajectory cancellation"
      - "stream | internal/agentcore/live_ws.go/event handlers; frontend Texture stream client | snapshot-watermark + heartbeat/paged durable tail; EventBus notification only; activation_parked never completion"
    private_primitives_after_cutover: "objectgraph conditional transaction; CommitTextureHeadAuthority; CreateTextureDocumentOG; CreateTextureRevisionOG; CreateWorkItemOG; UpdateWorkItemDetails; UpdateWorkItemStatusOG; UpdateTrajectorySubjectRefs; UpdateTrajectoryStatusOG; canonical worker-update put. DeleteDocument/ogDelete are removed from Texture product paths and may exist only in isolated tests/migration tooling that refuses lifecycle kinds. Package visibility plus a structural ratchet prevents product callers outside reducers."
    legacy_delete_after_proof: "createRevision compensating deletes; immutable PatchRevisionMetadata writes; dead SQL worker_updates readers/scanners, inbox_deliveries and schema/index residue; RunContinuation production surfaces; legacy SQL Texture document/revision residue only after exact caller/migration proof."
    retained_out_of_scope: "Non-Texture AgentRecords/trajectories behind scoped guards; canonical Texture/Agent/Trajectory/WorkItem/update object graph; actor recovery as projection; publication projections; independent computer-event/effect chain."
  rejection_criteria:
    - "Reject any partial/compensating semantic write, physical lifecycle-history deletion, second head/store, unscoped authority read, dual Agent/Trajectory settlement refs, RunID/SourceRunID-derived identity, Start CommandID without digest, delivery-as-incorporation, raw work/ref/terminal writer, relabeled AgentRecord health, RunFailed cancellation projection, invalid settlement, lost restart state, late wake, or lifecycle effect capability."
    - "Reject public catch-up with a fixed cap, subscribe gap, or notification-only tail; snapshot without atomic watermark; UI-only state; client-composed lifecycle creation; or desktop/CLI DTO divergence."
    - "Reject identity assembled from mutable checkout state, unsigned fragments, unpinned keys, stale/no nonce, raw path disclosure, client joins, SSH, or a non-failing missing/conflicting measurement."
    - "Reject compatibility dual truth, fixed research workflow, universal Texture redesign, Round 72 runtime import, or proof only by local tests, source shape, dashboard, model agreement, or deployment metadata."
  evidence_refs:
    - "contract review /tmp/choir-kernel-contract-review-b05ed30b: four independent REPAIR verdicts; reproducible blockers adjudicated into this scope"
    - "internal/store/texture.go; internal/store/graph_store.go; internal/objectgraph/store.go; internal/objectgraph/dolt_store.go"
    - "internal/types/evidence.go; internal/types/trajectory.go; internal/actor; internal/actorruntime; internal/agentcore/trajectory.go; internal/agentcore/api_trajectory.go; internal/agentcore/live_ws.go"
    - "internal/textureowner/texture_revision_metadata.go; internal/textureowner/tools_texture.go; internal/textureowner/texture_agent_revision.go; internal/proxy/compute_status.go"

measures:
  - name: executable_definition_count
    kind: gate
    baseline: "one old active Definition plus a proposed successor"
    desired: "exactly one active entrypoint in ACTIVE.md, mission-graph.yaml, and doc-authority-manifest.yaml"
    decision_use: "Refuse all runtime work until registry conformance passes."
    cannot_prove: "Product lifecycle correctness or deployed acceptance."
  - name: semantic_authority_count_per_object
    kind: gate
    baseline: "overlap observed across run, actor SQLite, Dolt coagent/work, Texture lifecycle, and effect paths"
    desired: "one named reducer/head per exercised durable object"
    decision_use: "Select migration and deletion boundaries during the frozen contract gate."
    cannot_prove: "Restart durability or product usability without deployed traces."
  - name: texture_revision_count
    kind: telemetry
    baseline: "current requests often terminate after one agent-authored revision"
    desired: "unbounded by harness; proportional to task, evidence, and future inference/search economics"
    decision_use: "Detect forced workflow or premature terminalization; do not optimize a fixed count."
    cannot_prove: "Research depth, truth, or artifact quality."

north_star:
  statement: "Choir is becoming a persistent sovereign agentic computer whose state and authority outlive any model session. Texture may become its versioned hypermedia lens and usually accumulate many more revisions as inference and search become cheaper, but this mission does not freeze that larger interface architecture."
  emergence_rule: "Let the generic kernel and deployed evidence determine later Texture, MCP, World Wire, self-development, and Choir-in-Choir shape. Keep the harness strict about authority, provenance, delivery, budget, cancellation, and settlement while permissive about model choice, search, delegation, reasoning route, and revision count. Not every task should be deep."

execution:
  - id: A-reconcile-and-supersede
    purpose: "Create one clean authority and source baseline without losing rejected work or unrelated WIP."
    exit: "Canonical main is clean, equals origin/main, names this file as sole `/goal` authority, preserves exact rejected/rollback refs, and authorizes no runtime effect."
  - id: B-observe-and-freeze-kernel-contract
    purpose: "Map every writer/reader; freeze one-authority lifecycle, transactional/replay protocol, identity trust contract, deletion inventory, rejection criteria, and rollback before runtime implementation."
    exit: "Independent review accepts a frozen code-free contract and no runtime or identity candidate exists yet."
  - id: C-build-disabled-candidate
    purpose: "Implement the smallest generic artifact/activation/obligation/settlement lifecycle and UI/headless contract with effects OFF; delete superseded exercised-path authorities only after migration proof."
    exit: "A frozen source candidate passes focused transition/refusal tests and a product smoke trace; independent review accepts authority and deletion inventory."
  - id: D-land-deploy-and-prove
    purpose: "Run the normal landing loop, verify exact Node B host/guest identity, and execute lifecycle, restart, delivery, cancellation, adaptive-depth, and conformance acceptance from clean clients."
    exit: "Every finish acceptance passes with immutable receipts, effects OFF, and prior rollback refs."
  - id: E-close-and-select-next
    purpose: "Record terminal identities, clean all temporary proof output, update registries, and let evidence choose one separately owner-ratified next mission."
    exit: "This Definition is complete, main is clean, staging is identified and healthy, and exactly one successor may become executable."

now:
  status: problem_documented
  slice: D-land-deploy-and-prove
  question: "How can Node B restart vmctl during a runtime deployment without trying to recreate a retained guest on a TAP device still held by the surviving Firecracker process?"
  reconciliation:
    observed_at: 2026-07-23T19:00:00Z
    source_ref: refs/remotes/origin/main@d845c56a
    deploy_identity: "Settlement repair d845c56a passed every selected CI source/build gate in GitHub Actions 30034969790, but Node B deployment failed before activation receipt. The last accepted joined deployment remains 32302b652ea7522e1d3cd0b21fde8b82f0449b40."
    authority_identities: [docs/choir-doctrine.md, docs/ACTIVE.md, docs/mission-graph.yaml, docs/doc-authority-manifest.yaml, docs/definitions/choir-coherent-computer-convergence-2026-07-21.md]
    worktree_inventory_ref: "Canonical worktree /Users/wiz/go-choir contains only this code-free problem receipt after pushed source commit d845c56a; protected unrelated worktrees remain untouched."
    status: reconciled
  accepted_deployment:
    source_commit: 32302b652ea7522e1d3cd0b21fde8b82f0449b40
    ci_ref: "GitHub Actions 30020798551 passed every selected source/build gate and Node B deployment."
    environment_identity: "choir.execution_identity.v1 joined=true on 32302b652ea7522e1d3cd0b21fde8b82f0449b40 at retained-computer epoch 127 before the failed d845c56a deployment attempt."
    rollback_ref: "Node B deploy-receipt-previous.json identifies 696b118a; accepted deployment 32302b65 is the pre-attempt product baseline; origin/main@5cd42558 is the settlement-repair source rollback."
  observed_product_evidence:
    - "The settlement candidate d845c56a makes an admitted pending/running activation's first terminal projection durably mark a reducer-attempt trigger in the same run/agent CAS; immediate and boot callers invoke only canonical SettleLifecycleTrajectory readiness authority. Focused race/restart loops, full store/agentcore/textureowner tests, vet, runtime shards, and unanimous frozen review accepted SHA-256 30af0971ebedaccd6a694322ab9dc89fe388e9cdf415b6b048c7f9cb31ab01cb."
    - "GitHub Actions 30034969790 passed Plan CI, docs truth, heresy detector, every race shard, Go vet/build, differential SBOM acceptance, and rolling-flake publication."
  observed_problem: "Deploy attempt 1 restarted vmctl after a shutdown timeout and failed recreating the retained VM because its TAP remained EBUSY. Attempt 2 restored vmctl and epoch 129, but the classifier had selected sandbox as a service-pointer artifact without the canonical guest boot closure; the workflow called nonexistent /internal/runtime/refresh, then full-refreshed the old boot closure and correctly refused its 32302b65 identity. Frozen repair review then found that the workflow's constructed ComputerVersion exclusion reads snapshot_kind/construction fields that /internal/vmctl/list does not expose, so an immutable active candidate would be misclassified as mutable and sent to refresh."
  policy_resolution_ref: "Problem-documentation-first requires this code-free receipt before changing the vmctl list contract or deployment route. Use the existing authoritative ownership snapshot fields; no second registry or client-side inference."
  blocker_or_risk: "Mission completion remains blocked. The source repair passed CI but is not accepted on staging. Deployment must not refresh constructed ComputerVersion realizations, and it must deploy sandbox runtime changes through an artifact route the guest can actually execute."
  latest_blocker_or_risk: "Mutation class red. Protected surfaces: vmctl ownership inventory, constructed ComputerVersion immutability, retained-computer realization, guest boot/runtime identity, deployment receipt, and staging health. Conjecture delta: a workflow exclusion is not protection unless the authoritative API carries the discriminator it tests. Heresy delta: discovered nonexistent hot-refresh route and unwired immutable-candidate exclusion; introduced none in this receipt; repaired none."
  next_action: "Expose the existing persisted constructed-ownership discriminator and immutable construction refs through /internal/vmctl/list, ratchet the real handler response, then complete the frozen deployment repair: every sandbox artifact uses the canonical guest boot closure, nonexistent hot refresh is deleted, constructed candidates are excluded, mutable computers full-refresh, and exact identity gates activation."

receipts:
  - id: durable-work-contract-gate-2026-07-21
    boundary: define
    commit_or_artifact: 9f725b9bd2e38b6079b23eb265f081bc91d1835f
    proof_refs: ["kernel_contract sha256 6a661560d7a2459c68becaa908e37a5c85622763ab29d81dbe9cf7ab12199589", "panel manifest sha256 5cd3ec70c47844840910fc458e6ffaf8ea83b27a1e57ebd99d9b972b4b369163", "Codex ACCEPT sha256 9b6a25bb93b9f23bbcda6e91760145b6ce0212779f98e49e0b7370ae624f55ef", "Cursor ACCEPT sha256 aa9165b985c4cc10565af10300678b0c4bddf9fb484eb6975242dee724b0bf72", "Gemini ACCEPT sha256 04f73acd86456b4c283f4f0de08c048cfe74f7e0955d992edfffa7372fa74532", "GPT-5.5 ACCEPT sha256 6e8f28acec146b88d7b1ac2943ab89b8988e907161c6edb97cde2af694fe2e8c", "scripts/doccheck live passed", "Definition dashboard parsed"]
    rollback_ref: refs/remotes/origin/main@9d887494c230a5276529066c7f1e049349d933c9
    disposition: "Accepted after five code-free scopes repaired every reproducible minority blocker. Phase C runtime implementation is authorized only within the frozen contract; effects remain OFF."
    problem_ref: "One lifecycle previously had compensating writes, unscoped subject reads, delivery-as-disposition projections, raw work/trajectory terminals, lossy replay, and unbound execution identity."
    authorization_ref: "Owner-ratified bounded generic lifecycle plus accepted contract scope 05"
    candidate_or_evidence_refs: [/tmp/choir-kernel-contract-review-9f725b9b/manifest.tsv]
  - id: architecture-interrogation-2026-07-21
    boundary: define
    commit_or_artifact: /tmp/choir-texture-lens-panel/manifest.tsv
    proof_refs: ["manifest sha256 67a79be274417f601b080cadd3c0974b0d3247d2fa2c263e9fd0007d194621f1", "five substantive reviewers found artifact/activation/work/effect authority conflation and accepted Texture only as a declarative lens over native authority"]
    rollback_ref: refs/remotes/origin/main@7913a3da0343ee03cf32b7622aaf9f2de35ee887
    disposition: "Use generic object/authority kernel first; defer lens implementation. Reviewer dissent on first probe was resolved by owner priority."
    problem_ref: "Texture delayed-research V1-without-V2 trace and Round 72 authority rejection in the superseded Definition"
    authorization_ref: "Owner supersession and bounded generic-lifecycle direction 2026-07-21"
    candidate_or_evidence_refs: [refs/heads/selfdev/architecture-recovery@2526f108a36c498f2f90ac89fcc6e4140685d9d9]
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: "landed at origin/main@c972ce1b6ab4bf4c1d03e7590773082c92c4e9dc; docs truth workflow 29865854776 passed"

  - id: definition-supersession-gate-2026-07-21
    boundary: define
    commit_or_artifact: refs/heads/selfdev/architecture-recovery@2eb9129577aeb19e515b8c9b0ad549b077ffafa7
    proof_refs: ["final panel manifest sha256 d14f011ab042d7a8de09d8348cb91b66eed2f88089ff6c499a8aa63097b917f3", "Codex ACCEPT sha256 a9de8091cbb8b97c995b070507432eb37997ce8eb92f21c649bf4963dc32c71b", "Cursor ACCEPT sha256 04ea1de79feee572f1c512ab490055a7a7d13938d73bca21c4950ec9c64b2e95", "OMP GPT-5.5 ACCEPT sha256 a3bdbaa5e43ffe3506587e9c44eacfe27d604e8a119b9f089af4dd038fc6bbd3", "all three Definition dashboards parsed", "doccheck live passed and full report retained 101 warnings", "git diff --check passed"]
    rollback_ref: refs/remotes/origin/main@7913a3da0343ee03cf32b7622aaf9f2de35ee887
    disposition: "Accepted after repairing the old live now-card, unsupported mission execution mode, stale deployment claims, removed scratch-file baseline, and blocked successor reconciliation. No reproducible blocker remains."
    problem_ref: "Former mission remained operationally executable and some platform prose laundered rejected candidate deletion/effect claims"
    authorization_ref: owner_supersession_2026-07-21
    candidate_or_evidence_refs: [refs/heads/selfdev/architecture-recovery@51836f329d53feaed768c9566323dcd77931efdc, refs/heads/selfdev/architecture-recovery@2eb9129577aeb19e515b8c9b0ad549b077ffafa7]
    landing:
      source_commit: c972ce1b6ab4bf4c1d03e7590773082c92c4e9dc
      ci_ref: "GitHub Actions 29865854776: Plan CI Lanes, Docs Truth Check, and Go Vet + Test + Build passed"
      deploy_ref: not_applicable_docs_only
      environment_identity: "No deploy by design; choir.news remained at 832ae951e84400a54bd7f8ef52a312e872b5c3ef"
      deployed_acceptance: not_applicable_docs_only
    registry_conformance_ref: "origin/main@c972ce1b6ab4bf4c1d03e7590773082c92c4e9dc; one working mission entrypoint; one active_product_mission; live doccheck passed"

  - id: durable-work-disabled-candidate-2026-07-23
    boundary: build
    commit_or_artifact: accepted-uncommitted-code-diff@sha256:8e8129453dc9c929b13c903ba9135373d3c0962db3fc7ffa3b8d517a62d22abc
    proof_refs: ["panel manifest sha256 cfe8fae629b112a446123bdaf87f7ae8d3b0a990f4c367f9f6ff62f1490c4a91", "Codex ACCEPT sha256 8430922c66c99d78899e8afbd959f845c4b24bb277ae63bb90aaf280c0086cd6", "OMP GPT-5.5 ACCEPT sha256 2820655880a9b0b14c414ca8681b47247b963e385dd1625aa09419b8aec772e7", "scripts/go-test-runtime-shards passed", "focused race loops passed", "TestDurableWorkLifecycleSmokeTrace passed across Store reopen", "go vet and git diff --check passed"]
    rollback_ref: refs/remotes/origin/main@676f0772a06f9121ace3d014b853b8f8de844a04
    disposition: "Accepted after v13-v23 frozen review repaired every reproduced authority, admission, restart, and matching-release blocker. Effects remain OFF. Landing and deployed acceptance are still required."
    problem_ref: "Completed lifecycle activation could strand open canonical work; generic writers and restart projections could duplicate, suppress, or retain activation ownership."
    authorization_ref: "Owner-ratified bounded generic lifecycle plus accepted contract scope 05"
    candidate_or_evidence_refs: [/tmp/agentic-consensus-20260723-043935/manifest.tsv, artifact://1473, artifact://1478, artifact://1480]
  - id: legacy-mailbox-migration-candidate-2026-07-23
    boundary: build
    commit_or_artifact: accepted-uncommitted-code-diff@sha256:b5eed3da687aa7a3d03a03b100cd8d1b10b28ca29f0b4f6de583f153deb27d34
    proof_refs: ["panel manifest sha256 7c6295cedfd3b7a17fd7d9732cc6dc74b3b4829e57257d6f0b8a13f3cf688e65", "Codex ACCEPT sha256 d1a8d56a413280859237611f9163f0463b55d326c794305f2d0ffe9449cf89a9", "OMP GPT-5.5 ACCEPT sha256 673bdcc92c18095a99c55f09f09f748f5d9a5bdffb6212842a6105355a2633a7", "focused race tests count=10", "scripts/go-test-runtime-shards passed", "go vet and git diff --check passed"]
    rollback_ref: refs/remotes/origin/main@676f0772a06f9121ace3d014b853b8f8de844a04
    disposition: "Accepted after independent review exposed and candidate repaired incomplete identity discovery and per-mailbox partial commits. All legacy identities are now planned before one atomic SQLite rebind; effects remain OFF."
    problem_ref: "Node B deployment 29992782043 failed on retained unscoped mailbox processor-v2:processor-climate-us-rss."
    authorization_ref: "Owner-ratified bounded generic lifecycle plus code-free problem receipt 42a517bc"
    candidate_or_evidence_refs: [/tmp/agentic-consensus-20260723-054649/manifest.tsv, artifact://1522, artifact://1527]
  - id: restart-acceptance-oracle-problem-2026-07-23
    boundary: build
    commit_or_artifact: refs/remotes/origin/main@8063b5ff
    proof_refs: ["GitHub Actions run 29997288005 race non-runtime shard 1", "artifact://1536", "local go test -race count=10 failed 2/10 with distinct follow-on update IDs"]
    rollback_ref: refs/remotes/origin/main@676f0772a06f9121ace3d014b853b8f8de844a04
    disposition: "Problem documented before repair. The seeded restart dispatch was consumed, but the test rejected a different follow-on dispatch sharing the mailbox."
    problem_ref: "TestAdapterRestartResumesRunningLifecycleActivationFromDurableBacklog asserted mailbox-wide emptiness instead of seeded update consumption."
    authorization_ref: "Owner-ratified bounded generic lifecycle plus hosted CI evidence"
    candidate_or_evidence_refs: [artifact://1536, artifact://1539]
  - id: restart-acceptance-oracle-candidate-2026-07-23
    boundary: build
    commit_or_artifact: accepted-uncommitted-code-diff@sha256:dc38362bf0b979e45bab67ee18cfa425df94f8c81b0c2d68618cad9230dfca82
    proof_refs: ["panel manifest sha256 0ee4a5b9e640518eb0b8c43442cc82d8f3521eb5d92db939dd867b8b08351fc2", "Codex ACCEPT sha256 6e6bdb5b7960fe26de540473ad9ee221f36b655085a58f79107de60ca1d2408c", "OMP GPT-5.5 ACCEPT sha256 0fb15d825d5f7e29cb0878e78c733284b1bcb4e9ee2cde32114f70fb9751fd68", "focused race count=20", "complete actorruntime race package", "git diff --check"]
    rollback_ref: refs/remotes/origin/main@676f0772a06f9121ace3d014b853b8f8de844a04
    disposition: "Accepted test-only correction: restart delivery is proved by target completion and consumption of the seeded deterministic dispatch, not by unrelated mailbox quiescence."
    problem_ref: refs/remotes/origin/main@8063b5ff
    authorization_ref: "Owner-ratified bounded generic lifecycle plus code-free problem receipt 8063b5ff"
    candidate_or_evidence_refs: [/tmp/agentic-consensus-20260723-061202/manifest.tsv, artifact://1544, artifact://1549]
  - id: hosted-store-shard-timeout-problem-2026-07-23
    boundary: build
    commit_or_artifact: pending_code_free_receipt
    proof_refs: ["GitHub Actions run 29998711216 job 89178528327", "artifact://1558", "focused local race count=3 passed in 24 seconds"]
    rollback_ref: refs/remotes/origin/main@676f0772a06f9121ace3d014b853b8f8de844a04
    disposition: "Problem documented before repair. Evidence is a package wall-clock timeout without a failed assertion; unchanged retry is required before changing source or CI."
    problem_ref: "internal/store race package exhausted its 600-second hosted timeout while one test remained active."
    authorization_ref: "Owner-ratified bounded generic lifecycle plus hosted CI evidence"
    candidate_or_evidence_refs: [artifact://1558, artifact://1561]
  - id: retained-computer-recovery-blocked-2026-07-23
    boundary: land
    commit_or_artifact: refs/remotes/origin/main@696b118ac874545219ba6d13c440e3d9f3d47bb6
    proof_refs: ["GitHub Actions forced-full run 3001019521 passed and refresh timed out", "GitHub Actions recovery-diagnostic run 30014414949 passed", "public compute status at 2026-07-23T14:20:03Z: current failed epoch 116, runtime null, recovery failed code recovery_timeout", "independent final review /tmp/agentic-consensus-20260723-100608/manifest.tsv", "accepted recovery candidate sha256 b1848241e0c2c38429ab9619e9fb01e3db9b12dfb0dd6cb9b581c1e946a25f57", "temporary acceptance key ak_48823802-38a8-493f-acc7-486f25e25874 revoked"]
    rollback_ref: "accepted deployment 676f0772a06f9121ace3d014b853b8f8de844a04 epoch 114"
    disposition: "Blocked incomplete after the third recovery iteration. False-ready behavior is repaired and deployed; the retained computer remains failed. No further blind refresh or symptom repair is authorized by Dead-End Escalation."
    problem_ref: "Owner-visible recovery lacks bounded boot-stage and console evidence; exact failure remains unlocalizable without forbidden SSH/journal inspection."
    authorization_ref: "Owner-ratified durable-computer mission; AGENTS.md Dead-End Escalation; standing no-SSH operability question"
    candidate_or_evidence_refs: [/tmp/choir-recovery-diagnosed.json, /tmp/agentic-consensus-20260723-100608/manifest.tsv, artifact://1711]
    landing:
      source_commit: 696b118ac874545219ba6d13c440e3d9f3d47bb6
      ci_ref: "GitHub Actions 30014414949: success"
      deploy_ref: "Node B proxy deployed by run 30014414949; sandbox migration installed by forced-full run 3001019521"
      environment_identity: "Host proxy 696b118a; guest identity unavailable because retained computer has no healthy runtime"
      deployed_acceptance: "Recovery truthfulness passed; lifecycle/restart/delivery/cancellation/client acceptance blocked"
    registry_conformance_ref: "Definition remains sole active mission and is explicitly blocked_incomplete; registries require no successor change."


  - id: deployed-settlement-authority-unreachable-2026-07-23
    boundary: land
    commit_or_artifact: refs/remotes/origin/main@32302b652ea7522e1d3cd0b21fde8b82f0449b40
    proof_refs: ["GitHub Actions 30020798551 passed and deployed 32302b65", "signed identity /tmp/choir-postcancel-identity.json joined host/guest/vmctl/deployment at epoch 127", "trivial lifecycle /tmp/choir-trivial-snapshot.json remained live at version 3/reducer sequence 4 after work completed and update incorporated", "repository-wide SettleLifecycleTrajectory caller inspection found only store implementation and tests"]
    rollback_ref: "origin/main@9dcbb949 for mailbox convergence; accepted deployment 676f0772 epoch 114"
    disposition: "Problem documented before repair. Retained restart, typed delivery, cancellation at three phases, idempotent replay, CLI snapshot, and desktop observation passed, but deployed settlement did not."
    problem_ref: "The canonical settlement reducer has no production caller, so a product-created trajectory can satisfy every settlement predicate and remain live indefinitely."
    authorization_ref: "Owner-ratified durable-work Definition; AGENTS.md problem-documentation-first; frozen kernel contract requiring native work authority settlement"
    candidate_or_evidence_refs: [/tmp/choir-trivial-start.json, /tmp/choir-trivial-snapshot.json, /tmp/choir-pre-revision-cancel.json, /tmp/choir-waiting-cancel.json, /tmp/choir-retained-cancel.json, /tmp/choir-postcancel-snapshot.json, /tmp/choir-postcancel-identity.json]
    landing:
      source_commit: 32302b652ea7522e1d3cd0b21fde8b82f0449b40
      ci_ref: "GitHub Actions 30020798551: success"
      deploy_ref: "Deploy to Staging job 89255796672: success"
      environment_identity: "Joined host/guest/vmctl/deployment commit 32302b65, retained computer epoch 127, route digest sha256:c2c219a1c9a7b311fad8567b128a12b1456a2178de62088a5ec0dea11edcfe6a"
      deployed_acceptance: "Identity, restart reconstruction, delivery, cancellation, adaptive depth, and UI/headless observation passed; settlement failed."
    registry_conformance_ref: "Definition remains sole active mission at problem_documented; no successor is promoted."

  - id: durable-settlement-trigger-candidate-2026-07-23
    boundary: build
    commit_or_artifact: refs/remotes/origin/main@d845c56a
    proof_refs: ["frozen diff sha256 30af0971ebedaccd6a694322ab9dc89fe388e9cdf415b6b048c7f9cb31ab01cb", "final panel /tmp/agentic-consensus-20260723-143523/manifest.tsv: Codex ACCEPT 0.94, Cursor ACCEPT 0.88, OMP GPT-5.5 ACCEPT high", "focused settlement race tests count=10", "runtime boot/product tests count=10", "full store+agentcore+textureowner tests and vet", "scripts/go-test-runtime-shards passed", "GitHub Actions 30034969790 source/build gates passed"]
    rollback_ref: refs/remotes/origin/main@5cd42558bd60161c529277f34cd2fe2b6c52a8cf
    disposition: "Source candidate accepted and pushed. Run terminality remains only a durable attempt trigger; SettleLifecycleTrajectory retains sole semantic authority. Staging acceptance is blocked by vmctl restart failure."
    problem_ref: refs/remotes/origin/main@5cd42558
    authorization_ref: "Owner-ratified durable-work Definition; code-free deployed settlement problem receipt; unanimous frozen red-surface review"
    candidate_or_evidence_refs: [/tmp/agentic-consensus-20260723-143523/manifest.tsv, artifact://1907, artifact://1909]
  - id: staging-vmctl-tap-restart-failure-2026-07-23
    boundary: land
    commit_or_artifact: pending_code_free_receipt
    proof_refs: ["GitHub Actions run 30034969790 job 89303540955", "incomplete deployment receipt /var/lib/go-choir/deploy-failures/30034969790-1.json", "deploy diagnostics: vmctl shutdown context deadline exceeded; surviving candidate-fleet Firecracker killed; replacement failed opening vm-rvnt3drdofhm with EBUSY; vmctl health timed out"]
    rollback_ref: "accepted deployment 32302b652ea7522e1d3cd0b21fde8b82f0449b40 epoch 127; source rollback origin/main@5cd42558"
    disposition: "Problem documented before recovery or repair. Source/build gates passed, but d845c56a has no staging activation receipt and no deployed acceptance claim."
    problem_ref: "vmctl restart cannot safely transition retained Firecracker/TAP ownership when shutdown times out and the old process survives for reattach."
    authorization_ref: "Owner-ratified durable-computer mission; AGENTS.md problem-documentation-first; deploy failure evidence"
    candidate_or_evidence_refs: [artifact://1919, artifact://1920]

  - id: constructed-computer-refresh-exclusion-unwired-2026-07-23
    boundary: land
    commit_or_artifact: pending_code_free_receipt
    proof_refs: ["GitHub Actions run 30034969790 attempt 2 job 89305362383", "signed identity after failed attempt: retained mutable computer active at epoch 129 on old commit 32302b65", "frozen deployment review /tmp/agentic-consensus-20260723-151406/manifest.tsv OMP GPT-5.5 REPAIR", "internal/vmctl/handlers.go ownershipResponse omits snapshot_kind, construction_version, and construction_disk_receipt_id consumed by .github/workflows/ci.yml"]
    rollback_ref: "accepted deployment 32302b652ea7522e1d3cd0b21fde8b82f0449b40 epoch 127; recovered old-code realization epoch 129"
    disposition: "Problem documented before API/deploy repair. The current workflow's jq exclusion cannot distinguish immutable constructed candidates from mutable active computers."
    problem_ref: "Deployment refresh protection reads fields absent from the authoritative vmctl list response, so constructed ComputerVersion realizations can enter the mutable refresh path."
    authorization_ref: "Owner-ratified durable-computer mission; AGENTS.md problem-documentation-first; independent frozen review"
    candidate_or_evidence_refs: [artifact://1929, artifact://1930, /tmp/agentic-consensus-20260723-151406/manifest.tsv]

view:
  path: http://127.0.0.1:8788/
  generator: "node skills/definition/scripts/dashboard.mjs docs/definitions/choir-coherent-computer-convergence-2026-07-21.md --serve 127.0.0.1:8788 --watch --output /tmp/choir-convergence-definition.html"
---

# Converge Choir on One Durable Agentic Computer

The immediate goal is one honest durable-work kernel, not universal Texture or immediate self-development. Artifacts may revise many times; intelligence may adapt route and depth; obligations survive model and process boundaries; typed evidence wakes reconstructible subjects; native work authority settles; effects remain independently authorized. That bounded product proof is the cleanest route toward deeper Texture, safe self-development, MCP, World Wire, and Choir-in-Choir.
