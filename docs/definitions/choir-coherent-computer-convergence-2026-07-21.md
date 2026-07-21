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
  key_contract:
    scope: "Every canonical lookup and mutation is owner-scoped. Durable keys are artifact=(OwnerID,ComputerID,DocID), revision=(OwnerID,ComputerID,DocID,RevisionID), subject=(OwnerID,ComputerID,AgentID), trajectory=(OwnerID,ComputerID,TrajectoryID), work=(OwnerID,ComputerID,TrajectoryID,WorkItemID). No unscoped GetAgent or object lookup may authorize a transition."
    operation_identity: "Start carries caller CommandID scoped by owner/computer; first success stores its response and retries return it. TrajectoryID is allocated once by that reducer and is never derived from RunID. An update key is (OwnerID,ComputerID,TrajectoryID,TargetAgentID,ProducerAgentID,ProducerUpdateID); PayloadDigest covers canonical coagent_source_packet.v1 bytes. Reusing the key with a different digest is a conflict, while an exact retry returns the stored disposition."
    versions: "Document head revision, subject lifecycle version, trajectory version/status, work-item version/status, and update disposition/version are reducer-checked CAS inputs. Terminal records are immutable except for projection rebuild metadata outside their signed/content hash."
  authority_inventory:
    artifact_head:
      canonical_owner: "Embedded-Dolt object graph: choir.texture_document.CurrentRevisionID plus immutable choir.texture_revision/source objects."
      only_writer: "StartDurableWorkAuthority and ApplyTypedUpdateAuthority call one private Texture-head batch reducer. Store.CreateRevision/CreateRevisionWithSourceGraph become wrappers or are made private; their source graph, revision, parent/document edges, stale-parent check, and head CAS commit in that batch."
      mutation_limits: "Store.UpdateDocument/UpdateTextureDocumentOG may change title-only fields and cannot accept CurrentRevisionID. PatchRevisionMetadata cannot mutate immutable revision/content/provenance metadata; operational delivery notes move to rebuildable projection objects. Platform publication mirrors remain post-commit read projections."
      projections: "Texture DTO/streams, Trace/events, publication mirrors, and public manifests may report revisions but never advance the private head."
    durable_subject:
      canonical_owner: "Embedded-Dolt AgentRecord keyed by (OwnerID,ComputerID,AgentID); Texture AgentID is deterministic texture:<DocID> within that owner/computer."
      only_writer: "StartDurableWorkAuthority creates or verifies the subject and advances its lifecycle version. Subsequent semantic subject-ref changes occur only inside ApplyTypedUpdateAuthority, CancelTrajectoryAuthority, or SettleTrajectoryAuthority. UpsertAgent callers and actor recovery may update explicitly projection-only activation/health fields through that reducer facade, never owner, computer, subject identity, trajectory, obligation, or terminal state."
      activation_boundary: "RunRecord, AgentMutation, provider state, and actor SQLite memory/processed_at describe replaceable activations only. Run terminality, passivation, snapshots, and transcripts cannot settle the subject or acknowledge evidence."
    typed_update:
      canonical_owner: "Embedded-Dolt object graph choir.worker_update under the key and payload-digest rules above."
      state_machine: "Disposition is pending, incorporated, rejected, cancelled, or late. Pending includes never-delivered and delivered-to-activation evidence; delivery never changes semantic disposition. The other four states are terminal and carry exactly one artifact/work/refusal/terminal-trajectory ref plus reducer sequence. Exact duplicates return the record; conflicting duplicates refuse."
      only_writers: "AppendTypedUpdateAuthority creates pending or post-terminal late; ApplyTypedUpdateAuthority writes incorporated/rejected; CancelTrajectoryAuthority changes every non-terminal update to cancelled. Delivery/cursor/checkpoint code has no disposition write."
      projections: "Actor SQLite rows, DeliveredToRunID, coagent mailbox cursors, texture_controller_checkpoint, texture_agent_mutations, run/revision worker_updates_* metadata, channel messages, and UI cursors are delivery/recovery/audit projections. They are deleted or rebuilt from canonical updates and may never acknowledge incorporation, rejection, cancellation, or settlement."
    work_item:
      canonical_owner: "Embedded-Dolt WorkItemRecord keyed by the tuple above."
      only_writers: "StartDurableWorkAuthority mints at least one material open obligation; ApplyTypedUpdateAuthority records its explicit complete/refuse/keep-open consequence; CancelTrajectoryAuthority cancels every open item. CreateWorkItemOG and UpdateWorkItemStatusOG become private batch primitives; boot sweeps, publication, run completion, and projections cannot close work."
    obligation_and_settlement:
      canonical_owner: "Embedded-Dolt TrajectoryRecord plus canonical WorkItem and worker-update records."
      only_writers: "Connect and extend existing CancelTrajectoryAuthority/CancelTrajectoryAuthorityOG; add SettleTrajectoryAuthority as the only settlement entrypoint over TrajectoryObligations/EvaluateTrajectorySettlement. Raw UpdateTrajectoryStatus(...settled|cancelled) is forbidden on every production path."
      settlement_rule: "The stored SettlementRuleVersion names a closed predicate vocabulary. In the settlement transaction the reducer re-reads a live trajectory, zero open work, zero non-terminal updates, and every non-empty RequiredSubjectRef, then writes settled plus the lifecycle event. A missing or unknown predicate/rule version refuses."
      cancellation_rule: "Cancellation locks the live trajectory first, assigns its canonical reducer sequence, and in the same transaction writes trajectory cancelled, all open work cancelled, all non-terminal updates cancelled, and the lifecycle event. Revisions and evidence already committed remain inspectable as pre-cancellation history."
    effect_authorization:
      canonical_owner: "ComputerEventAppender plus corpusd ComputerEventCAS remain the separate per-computer semantic-event authority."
      effects_off: "The lifecycle package receives no capsule, EventEffectAccepted, updater, checkpoint, ComputerVersion, or route-mutation capability. It neither starts nor acknowledges self-development materialization. Existing protected effect implementations remain unchanged."
  reducer_contract:
    storage_boundary: "All semantic lifecycle objects and the durable lifecycle-event outbox live in the same embedded-Dolt object graph. Extend objectgraph.BatchStore.PutBatch with a conditional serializable transaction/CAS seam; do not use compensating deletes as correctness. Aliases, actor SQLite, Trace, EventBus, and platform publication synchronize only after commit."
    ordering: "Reducers lock/compare in deterministic order: owner/computer/trajectory, artifact head, subject, then sorted work/update IDs. Each accepted transaction receives one monotonic per-trajectory ReducerSeq. Cancellation or another terminal transition that obtains the trajectory CAS first wins; losers re-read and return the stored terminal/refusal result."
    retries: "A crash or error before commit leaves no semantic sub-write. After commit, retry by CommandID/update key returns the stored result without reapplying. CAS conflicts are explicit retryable responses; validation/terminal conflicts are durable non-retryable refusals. Post-commit wake and stream notification may repeat or disappear because durable replay closes every gap."
    start: "StartDurableWorkAuthority atomically writes V0 revision/head, durable subject, live trajectory with SettlementRuleVersion and required refs, at least one material open WorkItem, idempotency receipt, and lifecycle outbox event."
    append_update: "AppendTypedUpdateAuthority atomically writes pending update plus outbox event when the trajectory is live. If the trajectory is already cancelled or settled it writes terminal late with that trajectory ref plus event and does not wake. Only after commit may a best-effort wake target the subject."
    incorporate: "ApplyTypedUpdateAuthority with disposition incorporated atomically rechecks live trajectory and expected head, writes source graph + immutable revision + edges + head, terminal update disposition, declared WorkItem consequence, required subject refs, and outbox event. It either commits all or none."
    reject: "ApplyTypedUpdateAuthority with disposition rejected atomically rechecks live trajectory, writes durable refusal reason/ref, terminal update disposition, declared WorkItem consequence, and outbox event. It creates no revision and either commits all or none."
    cancel: "CancelTrajectoryAuthority performs the cancellation rule above in one transaction. A delivered-but-undisposed update is non-terminal and is cancelled. After the trajectory CAS commits, no revision, incorporation, rejection, work completion, or settlement can commit."
    settle: "SettleTrajectoryAuthority performs the settlement rule above in one transaction and is idempotent. A settlement attempt never repairs predicates or silently closes work."
  transition_contract:
    - "The owner-scoped Texture route calls StartDurableWorkAuthority to create V0, subject, trajectory, and a material delayed obligation under one CommandID."
    - "Replaceable activations may propose zero or more revisions and passivate; only a reducer commit changes artifact, evidence, work, or trajectory state."
    - "Typed evidence remains pending across delivery and restart until atomically incorporated or rejected. Cancellation terminalizes all non-terminal evidence; post-terminal new evidence becomes late without wake."
    - "Restart reconstructs subject, artifact head, obligations, dispositions, terminality, evidence refs, reducer sequence, and replay cursor from embedded Dolt without RunID, actor SQLite, event bus, UI state, or transcript."
  public_protocol:
    schema: "choir.durable_work.v1 is a narrow lifecycle DTO, not a universal Texture schema. IDs and reads are owner/computer scoped by authenticated context; clients cannot supply another owner."
    start_command: "The existing authenticated POST /api/prompt-bar Texture-selected path must delegate its first materialization to StartDurableWorkAuthority and return CommandID, TrajectoryID, DocID, V0 RevisionID, subject ID, obligation IDs, reducer sequence, and snapshot cursor. A headless cmd/choir start command calls this same endpoint/DTO; no client-side multi-call assembly is accepted."
    snapshot: "GET /api/trajectories/{id} returns a versioned lifecycle snapshot containing artifact head, subject identity/version, activation projection clearly labeled non-authoritative, obligations, update dispositions/refs, trajectory rule/status, reducer sequence, evidence refs, and a watermark captured in the same read transaction."
    replay: "GET /api/trajectories/{id}/events?after=<cursor>&limit=<n> pages the durable outbox without a fixed catch-up cap. Stream connection subscribes to notifications before replay, repeatedly replays through the latest durable watermark, and reports cursor-expired only with a replacement snapshot cursor. EventBus overflow is allowed only as a notification loss because clients always recover by durable replay."
    commands: "Cancel and typed-update commands use the reducers and return the same snapshot/result DTO with explicit idempotency, expected-version/head, disposition, refusal, and conflict fields. Revision writes outside a bound live lifecycle keep existing manual-document semantics but cannot impersonate lifecycle settlement."
    clients: "Desktop and cmd/choir render/print the same DTO and replay protocol. Texture park/passivation maps to activation_parked, never work_completed. UI session/device state and CLI polling remain non-authoritative."
    identity:
      endpoint: "Dedicated authenticated /api/acceptance/execution-identity, not general /api/compute/status. Access requires owner/admin acceptance:read scope; raw host paths, credentials, environment, and unrelated computer inventory are never returned."
      envelope: "choir.execution_identity.v1 contains issuer/key_id, audience, caller nonce, issued_at/expires_at, deployed commit, canonical GitHub ref plus CI-signed clean-tree provenance, host Nix closure digest and role-named executable digests/embedded commits, guest image/config/build digests, ComputerID/RealizationID/epoch, ComputerVersion/route receipt digest, join verdict, and one signature over canonical bytes. Paths are role labels plus digests, not filesystem disclosure."
      trust: "CI/deploy publishes a signed immutable deployment manifest; Node B's configured platform-attestation key joins that manifest to runtime executable/Nix measurements and vmctl's read-only guest/route/realization inventory. Acceptance clients pin the deployment/attestation public-key IDs; rotation requires an overlap manifest signed by an already trusted key. The verifier checks signature, nonce, audience, expiry, CI provenance, measurement equality, and one common deployed commit."
      refusal: "Missing source oracle, manifest, key, measurement, guest/route join, authorization, or any conflict returns a typed fail-closed unsigned error and no acceptance verdict. Collection uses service APIs/manifests only: no SSH and no mutable checkout inspection."
  migration_and_deletion:
    artifact_paths:
      migrate: "internal/store/texture.go CreateRevision, CreateRevisionWithSourceGraph, createRevision; internal/store/graph_store.go CreateTextureRevisionOG and UpdateTextureDocumentOG; internal/textureowner/tools_texture.go commitTextureToolEdit; all direct CreateRevision callers. Connect objectgraph.BatchStore.PutBatch conditional transaction."
      restrict_or_delete: "Store.UpdateDocument head mutation; PatchRevisionMetadata on immutable fields; createRevision compensating deletes. Keep title-only update and rebuildable publication/alias projections."
    subject_paths:
      migrate: "internal/store/graph_store.go UpsertAgent and GetAgentOG plus internal/agentcore runtime/runtime_persistence callers to owner/computer-scoped reducer/projection APIs. Delete unscoped authority reads from the exercised path."
    trajectory_work_paths:
      connect: "internal/store/graph_store.go CancelTrajectoryAuthorityOG, CreateWorkItemOG, UpdateWorkItemStatusOG; internal/agentcore/trajectory.go TrajectoryObligations and EvaluateTrajectorySettlement; internal/agentcore/api_trajectory.go owner-scoped reads/cancel."
      migrate_or_delete: "internal/store/trajectory.go and graph_store.go raw UpdateTrajectoryStatus settled/cancelled paths; internal/agentcore/wire_publication.go direct settlement; any run-completion, boot-sweep, publication, or test helper used as a production WorkItem closer. Retain raw primitives only private to reducers."
    update_paths:
      migrate: "DispatchWorkerUpdateOG; ListCoagentMailboxBacklog; MarkWorkerUpdateDeliveredOG/MarkWorkerUpdatesDeliveredOG; UpdateRunAndMarkWorkerUpdatesDeliveredOG and rollback; internal/textureowner texture_controller_checkpoint, texture_agent_mutations, textureWorkerUpdateCommitSeq/markTextureWorkerUpdatesDelivered and revision worker_updates_consumed/skipped/pending metadata; internal/actorruntime/handler.go handleCoagentResult; internal/agentcore/super_controller.go reconcileUpdatedCoagentActor."
      disposition: "Canonical worker-update objects gain disposition/version/digest/refs. Delivery marking and checkpoints become rebuildable projections or are deleted. A terminal trajectory consumes late actor messages with nil handler error after durable late/cancelled disposition, preventing retry loops."
    stream_paths:
      migrate: "internal/agentcore/live_ws.go and event handlers plus frontend Texture stream client to snapshot-watermark + paged durable replay; EventBus becomes notification only. Delete park/passivation-to-completion mapping."
    legacy_residue:
      delete_after_proof: "Dead SQL worker_updates readers/scanners, inbox_deliveries, schema/index residue; RunContinuation production surfaces; legacy SQL Texture document/revision residue only after exact caller/migration proof."
      retain: "Canonical Texture/Agent/Trajectory/WorkItem/worker-update object graph, actor recovery log as projection, public publication projections, and independent computer-event/effect chain."
  rejection_criteria:
    - "Reject any partial or compensating semantic write, second head/store, unscoped authority read, RunID-derived trajectory, delivery-as-incorporation, raw terminal writer, open-work/non-terminal-update settlement, lost restart state, late wake, or lifecycle effect capability."
    - "Reject public catch-up with a fixed cap or subscribe gap, snapshot without an atomic watermark, UI-only state, client-composed lifecycle creation, or desktop/CLI DTO divergence."
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
  status: working
  slice: B-observe-and-freeze-kernel-contract
  question: "Does repaired contract scope 02 close every reproducible transactional, ownership, replay, migration, cancellation, and identity blocker without authorizing implementation?"
  reconciliation:
    observed_at: 2026-07-21T21:03:29Z
    source_ref: refs/remotes/origin/main@9d887494c230a5276529066c7f1e049349d933c9
    deploy_identity: "Public https://choir.news/health reported proxy build/deployed commit 832ae951e84400a54bd7f8ef52a312e872b5c3ef; exact host/guest joined identity remains unavailable without the new product path."
    authority_identities: [docs/choir-doctrine.md, docs/ACTIVE.md, docs/mission-graph.yaml, docs/doc-authority-manifest.yaml, docs/definitions/choir-coherent-computer-convergence-2026-07-21.md]
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: "2026-07-21T21:03:29Z git worktree/status inventory: canonical main clean; architecture-recovery clean; terminal-outcome-closure and definition-v1-1 dirt preserved forbidden; other clean/historical worktrees untouched"
    status: reconciled
  candidate:
    id: convergence-kernel-contract-02
    state: frozen_by_scoped_content_digest
    ref: refs/heads/convergence/kernel-contract-01
    owner: owner-and-current-session
    base: b05ed30bf3a3cc43a3d1aff707f30dcdce74a130
    rejected_predecessor: "b05ed30bf3a3cc43a3d1aff707f30dcdce74a130 — independent panel unanimously returned REPAIR; blockers were transaction linearization, writer/key ownership, lossless replay, identity trust/disclosure, durable inventory, and internal freeze state"
    digest: sha256:1eb323dc16ffdb31ff03d40661901b5bf82ad15fe9ec64e182ebc783d6e0f29f
    scope: [docs/definitions/choir-coherent-computer-convergence-2026-07-21.md#kernel_contract]
  decision:
    selected: "Supersede the incomplete self-development mission and first prove one generic durable-work lifecycle; do not repair Round 72 or start a comprehensive Texture redesign."
    kind: purpose
    status: settled
    source: owner
    evidence_ref: "Owner statements in the 2026-07-21 conversation"
    owner_ratification_ref: "Owner directed: step back and supersede the current defined mission with a new one"
    recorded_at: 2026-07-21T19:41:58Z
    consequence: "Documentation may cut over sole mission authority; subsequent runtime work is limited to the bounded generic lifecycle after the code-free contract gate."
  evidence_refs: [b05ed30bf3a3cc43a3d1aff707f30dcdce74a130, /tmp/choir-kernel-contract-review-b05ed30b/manifest.tsv, /tmp/choir-kernel-contract-review-b05ed30b/codex.out, /tmp/choir-kernel-contract-review-b05ed30b/cursor.out, /tmp/choir-kernel-contract-review-b05ed30b/omp-gpt55.out, /tmp/choir-kernel-contract-review-b05ed30b/omp-gemini35.out]
  blocker_or_risk: "Runtime mutation remains unauthorized. Contract scope 01 was rejected for repair; scope 02 incorporates every reproducible blocker but must be digest-frozen and independently accepted before code."
  next_action: "Compute and record the scope-02 contract digest, commit the code-free repair, then rerun independent adversarial review against that exact commit; implement nothing unless no reproducible blocker remains."

receipts:
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
  path: none
  generator: "node skills/definition/scripts/dashboard.mjs docs/definitions/choir-coherent-computer-convergence-2026-07-21.md --output /tmp/choir-convergence-definition.html"
---

# Converge Choir on One Durable Agentic Computer

The immediate goal is one honest durable-work kernel, not universal Texture or immediate self-development. Artifacts may revise many times; intelligence may adapt route and depth; obligations survive model and process boundaries; typed evidence wakes reconstructible subjects; native work authority settles; effects remain independently authorized. That bounded product proof is the cleanest route toward deeper Texture, safe self-development, MCP, World Wire, and Choir-in-Choir.
