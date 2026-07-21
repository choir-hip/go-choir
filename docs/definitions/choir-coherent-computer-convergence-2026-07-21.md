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
  authority_inventory:
    artifact_head:
      canonical_owner: "Embedded-Dolt object graph: choir.texture_document.CurrentRevisionID plus immutable choir.texture_revision objects."
      only_writer: "Store.CreateRevision/CreateRevisionWithSourceGraph through the serialized stale-parent head check."
      projections: "Texture DTO/SSE, Trace/events, platform publication mirrors, and public artifact manifests may report revisions but never advance the private head."
    durable_subject:
      canonical_owner: "Store AgentRecord keyed by owner-scoped AgentID; Texture subjects use the deterministic texture:<DocID> identity."
      activation_boundary: "RunRecord and the actor recovery log describe replaceable activations only. Run terminality, actor processed_at, snapshots, AgentMutation state, and provider transcripts cannot settle the subject's work."
    typed_update:
      canonical_owner: "Embedded-Dolt object graph choir.worker_update keyed by owner plus deterministic UpdateID over the normalized coagent_source_packet.v1."
      reducer_contract: "Exactly one durable disposition is pending, incorporated, rejected, cancelled, or late. Incorporated/rejected/cancelled/late are terminal and carry an artifact/work/refusal ref; delivery to an activation is evidence only. Duplicate delivery returns the existing disposition and cannot wake or mutate twice."
      projections: "Actor SQLite delivery rows, coagent mailbox cursors, channel messages, run metadata, and UI stream cursors are recovery/audit projections derived from the canonical update."
    obligation_and_settlement:
      canonical_owner: "Embedded-Dolt object graph TrajectoryRecord and WorkItemRecord."
      reducer_contract: "Only the native trajectory reducer may settle or cancel. Settlement requires a live trajectory, zero open work items, every trajectory update terminal, and every required subject ref present. Cancellation wins by canonical sequence and atomically cancels open work and pending updates while retaining revisions, evidence, and terminal dispositions."
      forbidden_writer: "UpdateTrajectoryStatus(...settled), run completion, activation passivation, UI state, acceptance records, and publication projections may not bypass the reducer."
    effect_authorization:
      canonical_owner: "ComputerEventAppender plus corpusd ComputerEventCAS remain the separate per-computer semantic-event authority."
      effects_off: "The lifecycle kernel receives no capsule, EventEffectAccepted, updater, checkpoint, or route capability. It must neither start nor acknowledge self-development materialization. Existing protected implementations remain unchanged."
  transition_contract:
    - "Create V0, durable subject, live trajectory, and at least one material obligation with owner/computer scope and idempotency identity."
    - "A replaceable activation may create zero or more stale-head-checked revisions and passivate; neither activation completion nor a revision closes the obligation."
    - "A typed update appends once as pending, survives restart, wakes the durable subject, and is incorporated into a later revision or explicitly rejected with a durable reason. Duplicate and late sends return the stored disposition."
    - "Restart reconstructs subject, artifact head, obligations, update dispositions, cancellation, and evidence refs from durable authorities without a RunID or transcript."
    - "Settlement and cancellation are mutually exclusive terminal trajectory transitions; cancellation observed first prevents later revision, incorporation, or settlement writes."
  public_protocol:
    identity: "Extend authenticated /api/compute/status with a fail-closed signed identity block joining canonical GitHub ref and cleanliness, host Nix closure and service executable paths/embedded commits, deployed commit, guest image/config digests and builds, ComputerID/RealizationID/epoch, and ComputerVersion/route receipt. Any missing or conflicting join refuses acceptance."
    snapshot: "GET /api/trajectories/{id} returns one owner-scoped lifecycle snapshot: artifact head, durable subject, activation projection, obligations, typed update dispositions, cancellation/settlement, evidence refs, and stream cursor."
    events: "The existing owner event stream sequence is the reconnect cursor; desktop and CLI replay after the same cursor and receive the same typed lifecycle events. Streams notify and replay but never acknowledge semantic state."
    commands: "Existing Texture revision, trajectory cancellation, and typed coagent update paths gain explicit expected-head/idempotency semantics where missing. No raw event mutation, internal vmctl route, SSH, or new universal Texture schema is exposed."
    clients: "Desktop and cmd/choir consume the same snapshot/event/command DTOs; UI session/device state and CLI polling are non-authoritative."
  migration_and_deletion:
    migrate:
      - "All direct settlement callers to one atomic trajectory settlement reducer that includes pending update dispositions and required refs."
      - "Texture stream mapping so park/passivation is not synthesized as document/work completion."
      - "Actor cancel handling to a projection of trajectory cancellation; remove the competing RunFailed cancellation write."
      - "Worker-update delivery marking to explicit canonical dispositions; actor processed_at and mailbox cursors remain recovery projections."
    delete_after_migration_proof:
      - "Dead legacy SQL worker_updates readers/scanners, inbox_deliveries, and their schema/index residue; live worker updates already use object-graph paths."
      - "Legacy SQL texture document/revision residue only after an exact caller and migration proof; retain the platform publication mirror as a read-only projection."
      - "RunContinuation production surfaces and any direct settled-status writer left without a production caller."
    retain:
      - "Texture object-graph artifact head, AgentRecord, actor recovery log, Trajectory/WorkItem authority, typed worker-update objects, public publication projections, and the independent computer-event/effect chain."
  rejection_criteria:
    - "Reject a candidate that adds a second head/store, treats RunID or delivery as incorporation, settles with open work or pending updates, loses state on restart, lets late work mutate a cancelled trajectory, or gives lifecycle code an effect capability."
    - "Reject identity output assembled by the client, mutable checkout state, unsigned independent health strings, or SSH. The trusted product service must bind and sign the joined inventory and fail closed on mismatch."
    - "Reject any compatibility shim preserving two semantic writers, any fixed research workflow, or any claim proved only by local tests, source shape, dashboard, model agreement, or deployment metadata."
  evidence_refs:
    - "agent://ArtifactTextureMap"
    - "agent://ActorDeliveryMap"
    - "agent://TrajectoryWorkMap"
    - "agent://EffectsAuthorityMap"
    - "agent://IdentityProtocolMap"
    - "internal/store/texture.go; internal/store/graph_store.go; internal/actor; internal/actorruntime; internal/agentcore/trajectory.go; internal/agentcore/tools_worker_update.go; internal/proxy/compute_status.go"

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
    purpose: "Add or connect complete no-SSH executable identity; map every writer/reader; freeze one-authority object contract, public trace, deletion inventory, rejection criteria, and rollback before runtime implementation."
    exit: "Independent review accepts a frozen code-free contract and no runtime candidate exists yet."
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
  question: "Will independent frozen review find any authority bypass, missing fate-sharing boundary, or unbounded public identity disclosure in the code-free kernel contract?"
  reconciliation:
    observed_at: 2026-07-21T21:03:29Z
    source_ref: refs/remotes/origin/main@9d887494c230a5276529066c7f1e049349d933c9
    deploy_identity: "Public https://choir.news/health reported proxy build/deployed commit 832ae951e84400a54bd7f8ef52a312e872b5c3ef; exact host/guest joined identity remains unavailable without the new product path."
    authority_identities: [docs/choir-doctrine.md, docs/ACTIVE.md, docs/mission-graph.yaml, docs/doc-authority-manifest.yaml, docs/definitions/choir-coherent-computer-convergence-2026-07-21.md]
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: "2026-07-21T21:03:29Z git worktree/status inventory: canonical main clean; architecture-recovery clean; terminal-outcome-closure and definition-v1-1 dirt preserved forbidden; other clean/historical worktrees untouched"
    status: reconciled
  candidate:
    id: convergence-kernel-contract-01
    state: drafting
    ref: refs/heads/convergence/kernel-contract-01
    owner: owner-and-current-session
    base: refs/remotes/origin/main@9d887494c230a5276529066c7f1e049349d933c9
    digest: pending_frozen_commit
    scope: [docs/definitions/choir-coherent-computer-convergence-2026-07-21.md]
  decision:
    selected: "Supersede the incomplete self-development mission and first prove one generic durable-work lifecycle; do not repair Round 72 or start a comprehensive Texture redesign."
    kind: purpose
    status: settled
    source: owner
    evidence_ref: "Owner statements in the 2026-07-21 conversation"
    owner_ratification_ref: "Owner directed: step back and supersede the current defined mission with a new one"
    recorded_at: 2026-07-21T19:41:58Z
    consequence: "Documentation may cut over sole mission authority; subsequent runtime work is limited to the bounded generic lifecycle after the code-free contract gate."
  evidence_refs: [refs/remotes/origin/main@9d887494c230a5276529066c7f1e049349d933c9, https://choir.news/health, agent://ArtifactTextureMap, agent://ActorDeliveryMap, agent://TrajectoryWorkMap, agent://EffectsAuthorityMap, agent://IdentityProtocolMap]
  blocker_or_risk: "The inventory found real overlaps: actor SQLite processed state versus Dolt update disposition, direct trajectory settlement without reducer preconditions, passivation rendered as completion, and no bound no-SSH host/guest identity. Runtime mutation remains unauthorized until this code-free contract is frozen and independently accepted."
  next_action: "Validate and freeze the code-free contract candidate, then run an independent adversarial review bound to its exact commit and adjudicate every reproducible blocker before any runtime code."

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
