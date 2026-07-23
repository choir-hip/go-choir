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
      trust: "CI/deploy publishes a signed immutable deployment manifest; Node B's configured platform-attestation key joins that manifest to runtime executable/Nix measurements and vmctl's read-only guest/route/realization inventory. Acceptance clients pin the deployment/attestation public-key IDs; rotation requires an overlap manifest signed by an already trusted key. The verifier checks signature, nonce, audience, expiry, CI provenance, measurement equality, and one common deployed commit."
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
  status: blocked_incomplete
  slice: D-land-deploy-and-prove
  question: "How must a delegated lifecycle activation that returns a final result without a terminal update_coagent disposition be reactivated without letting RunRecord terminality settle native work?"
  reconciliation:
    observed_at: 2026-07-23T03:15:00Z
    source_ref: refs/remotes/origin/main@676f0772a06f9121ace3d014b853b8f8de844a04
    deploy_identity: "GitHub Actions run 29975038438 deployed 676f0772a06f9121ace3d014b853b8f8de844a04; signed no-SSH identity joined clean refs/heads/main, all host service embedded commits, guest executable/image/config digests, active computer-03335285269bdba4f94377e56879f9e6, realization epoch 113, and the deployment receipt. Public stop/start receipts advanced the same computer to epoch 114 on the same commit."
    authority_identities: [docs/choir-doctrine.md, docs/ACTIVE.md, docs/mission-graph.yaml, docs/doc-authority-manifest.yaml, docs/definitions/choir-coherent-computer-convergence-2026-07-21.md]
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: "Canonical worktree convergence/kernel-contract-01 was clean at origin/main@676f0772 before this problem receipt; protected unrelated worktrees remain untouched."
    status: reconciled
  candidate:
    id: convergence-durable-work-runtime-14
    state: independent_review_repair_required
    ref: reviewed-rejected-uncommitted-code-diff@sha256:ea1e51ff8a09f65ef950e3bab896d8d26a627a42c5042cf2022de423260fb6fa
    owner: owner-and-current-session
    base: 70bcddf01e64965a5e1f7cb203d70b8c8a415429
    accepted_contract: "9f725b9bd2e38b6079b23eb265f081bc91d1835f#kernel_contract sha256:6a661560d7a2459c68becaa908e37a5c85622763ab29d81dbe9cf7ab12199589"
    prior_runtime_candidate: refs/remotes/origin/main@676f0772a06f9121ace3d014b853b8f8de844a04
    scope: "Phase D terminal-activation/open-work recovery repair; effects OFF."
    observed_problem: "Staging trajectory ba70399f-fe93-55cd-80d0-888aa75fb26b proved that a delegated researcher activation can complete after only work_disposition=open, leaving its native work correctly open but without a live replacement. Frozen review v13 then proved that the first repair still used a lifecycle-excluding legacy boot inventory and could duplicate a producer while its terminal typed update was pending. Frozen review v14 proved a second structural defect: boot grouped multiple open work items into one activation but update_coagent could bind only singular lifecycle_work_item_id, so that activation could settle none of them."
    repair_evidence: "Rejected candidate ea1e51ff locally repairs the deployed single-item failure, exhaustive lifecycle boot inventory, and pending-terminal suppression. Immediate and boot regressions passed under race count=5; lifecycle store tests, CLI smoke, and go vet passed. Cursor accepted v14, while Codex and OMP GPT-5.5 independently traced the same multi-item binding blocker; OpenCode timed out."
    remaining_error: "Make multi-item recovery executable without weakening typed work authority: every lifecycle update must select an assigned work item, replacement metadata must preserve all open bindings, and terminal continuation must recover every still-open binding while pending terminal dispositions suppress only their own item. Add an exact two-item regression, then freeze and rerun independent review before landing."
  decision:
    selected: "Supersede the incomplete self-development mission and first prove one generic durable-work lifecycle; do not repair Round 72 or start a comprehensive Texture redesign."
    kind: purpose
    status: settled
    source: owner
    evidence_ref: "Owner statements in the 2026-07-21 conversation"
    owner_ratification_ref: "Owner directed: step back and supersede the current defined mission with a new one"
    recorded_at: 2026-07-21T19:41:58Z
    consequence: "Documentation may cut over sole mission authority; subsequent runtime work is limited to the bounded generic lifecycle after the code-free contract gate."
  evidence_refs:
    - "/tmp/agentic-consensus-20260722-150725/manifest.tsv: Codex and Cursor REPAIR against exact candidate digest e8d5545563ac8a99bdda40e2ad21d8588d09c19e52591a1537392eb72b4bdddf"
    - "Codex and Cursor reproduced lifecycle rows leaking through legacy RunRecord metadata queries; Codex independently traced source-graph canonical IDs, object fields, writers, and readers that omit ComputerID."
    - "go test ./internal/store ./internal/agentcore ./internal/textureowner ./internal/runtimeprompts ./internal/textureprompts ./internal/types passed"
    - "TOTAL_SHARDS=1 scripts/go-test-runtime-shards passed all agentcore and textureowner tests"
    - "Focused store race suite count=10 and computer-scope Texture restart proof passed"
    - "TestDurableWorkLifecycleSmokeTrace passed across Store reopen; go vet and git diff --check passed"
    - "/tmp/agentic-consensus-20260722-155553/manifest.tsv: Codex REPAIR and Cursor ACCEPT against exact candidate digest b8a9631134c18eeda7c5d2424fdc6af02068ceba489cbbeb70e973d0007707d0; OpenCode failed."
    - "Codex traced production actorruntime initial dispatch/resume/cancel to legacy-only GetRunByOwner and traced texture_agent_mutations as owner/document-scoped across computers."
    - "/tmp/agentic-consensus-20260722-163318/manifest.tsv: Codex REPAIR and Cursor ACCEPT against exact candidate digest 00ee0cc76a3d99e1edb9042afe1ff897252210953881587a24fe35082cf371cd; OpenCode failed."
    - "Codex reproduced terminal artifact/document-head conflation in the public manual revision path; Cursor independently flagged owner-only lifecycle work/update scans as residual cross-computer risk."
    - "/tmp/agentic-consensus-20260722-170337/manifest.tsv: Cursor, Codex, and OMP GPT-5.5 REPAIR against exact candidate digest 114a3a14b33f1495f339079975d5ecba21a4844b86bd90183c8ac840964336b2; OpenCode timed out."
    - "Cursor reproduced browser dual-head collapse; OMP GPT-5.5 traced lifecycle activation to the lifecycle-excluding legacy mailbox injector; Codex reproduced caller-trusted source canonical IDs and lifecycle exposure through legacy direct source getters/existence checks."
    - "/tmp/agentic-consensus-20260722-195434/manifest.tsv: Codex and Cursor ACCEPT against repaired authority candidate; OMP GPT-5.5 reported three new reproducible client/authority defects before timeout."
    - "/tmp/agentic-consensus-20260722-204954/manifest.tsv: Cursor ACCEPT and Codex REPAIR against digest fff973f3eae9ee42bab7882a7d3fba2d51bf9c2c09be226b1e5f89257645a0a9; Codex reproduced lifecycle title PUT using a forbidden writer, projection-before-dispatch restart loss, and cross-computer actor-memory reconstruction."
    - "/tmp/agentic-consensus-20260722-212414/manifest.tsv: Cursor ACCEPT and Codex REPAIR against digest d69400445a291448faedc9eed5b4f4095051ff996cb69d6af4f670324e082663; Codex reproduced RunRunning initial-dispatch restart replay being marked processed without resuming."
    - "/tmp/agentic-consensus-20260722-215257/manifest.tsv: Codex and Cursor ACCEPT against digest 12d668ac59a0601662a49d7fbe15838f331dc5cc4bcea6295e3ed80f5b4e34f7; OMP GPT-5.5 completed without a substantive verdict and OpenCode timed out."
    - "Exact regressions now cover conditional lifecycle title projection, computer-scoped memory reconstruction, pending projection-before-dispatch recovery, deterministic dispatch identity, and end-to-end RunRunning recovery from an unprocessed actor-log row."
    - "GitHub Actions 29975038438 passed every selected race/spec/frontend gate and deployed origin/main@676f0772a06f9121ace3d014b853b8f8de844a04 to Node B."
    - "Signed public identity joined host, guest, vmctl route, deployment receipt, and platform attestation at epochs 113 and 114 on commit 676f0772a06f9121ace3d014b853b8f8de844a04."
    - "Staging trajectory ba70399f-fe93-55cd-80d0-888aa75fb26b retained interim head 1dbf4d2a-fef5-5b23-b745-0374556e6a00, two open obligations, and pending evidence update upd-7440d21e83d22deddcac6349a77bccd7 across a signed public stop/start; boot woke activation a0c70767-522f-4acd-ae5c-c37ee95df809 and incorporated that update exactly once into head 1c9bcfff-a108-537e-929a-82ae7b1f65fe."
    - "/tmp/agentic-consensus-20260722-233443/manifest.tsv: Cursor and OMP GPT-5.5 REPAIR against digest 4975fce746b0e43e0b31df7f88390075c044af3cc70742a60034822f008d0172; Cursor traced lifecycle work through the legacy-only boot inventory, and OMP traced redundant boot activation while terminal work disposition remained pending. Codex and OpenCode accepted."
    - "/tmp/agentic-consensus-20260723-000323/manifest.tsv: Codex and OMP GPT-5.5 independently REPAIR against digest ea1e51ff8a09f65ef950e3bab896d8d26a627a42c5042cf2022de423260fb6fa; Cursor accepted and OpenCode timed out. Both blockers traced multi-item activation metadata to update_coagent's singular lifecycle_work_item_id requirement."
  blocker_or_risk: "Rejected red repair candidate: single-item immediate/boot recovery works, but an agent with two open items receives one activation carrying only work_item_ids while update_coagent requires singular lifecycle_work_item_id. The activation cannot submit a reducer-native disposition for either item."
  latest_blocker_or_risk: "Problem documentation first. No multi-item repair code follows before this receipt is committed. Protected surfaces: durable actor delivery, activation metadata, lifecycle update identity, restart recovery, and work settlement. Admissible repair evidence is a two-item update-selection/continuation regression, focused race/package proof, frozen independent acceptance, and staging repetition. Rollback remains origin/main@676f0772 plus deployed ComputerVersion epoch 114."
  next_action: "Commit this code-free review receipt alone. Then add explicit validated work-item selection to update_coagent, make terminal continuation recover all bound open items, freeze the repaired candidate, and rerun the independent gate before CI/staging."

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

view:
  path: http://127.0.0.1:8788/
  generator: "node skills/definition/scripts/dashboard.mjs docs/definitions/choir-coherent-computer-convergence-2026-07-21.md --serve 127.0.0.1:8788 --watch --output /tmp/choir-convergence-definition.html"
---

# Converge Choir on One Durable Agentic Computer

The immediate goal is one honest durable-work kernel, not universal Texture or immediate self-development. Artifacts may revise many times; intelligence may adapt route and depth; obligations survive model and process boundaries; typed evidence wakes reconstructible subjects; native work authority settles; effects remain independently authorized. That bounded product proof is the cleanest route toward deeper Texture, safe self-development, MCP, World Wire, and Choir-in-Choir.
