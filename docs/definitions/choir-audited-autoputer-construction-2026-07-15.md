---
title: "Make the Autoputer Real — Audited Computer Construction"
definition_version: 2

start:
  captured_at: 2026-07-15T19:00:00Z
  source:
    canonical_ref: refs/heads/main@9d9945e65f5b54069e1a86a530cb0960d96b3474
    origin_ref: refs/remotes/origin/main@9d9945e65f5b54069e1a86a530cb0960d96b3474
    relation: canonical_ref_equals_origin_ref
    deploy_identity: unknown
  worktree_inventory:
    path: /Users/wiz/go-choir
    status: clean
    branch: main
    preservation_rule: "Do not import or mutate stale candidate worktrees. Build this mission from canonical main; preserve unrelated worktrees and production recovery images in place."
  observed_artifact:
    - claim: "The deployed owner route is nominally active but its Firecracker guest at http://10.201.201.2:8085 is unreachable; vmctl repeatedly reboots the same mutable realization instead of constructing a replacement."
      evidence_ref: "read-only Node B investigation, 2026-07-15"
    - claim: "The active owner data.img is a 32 GiB host file containing a 16 GiB ext4 filesystem with approximately 16 MiB free. Texture's embedded Dolt journal exhausted that filesystem; runtime writes, emergency Dolt GC, and subsequent boots failed."
      evidence_ref: "read-only Node B statfs/dumpe2fs/journal evidence, 2026-07-15"
    - claim: "The canonical code already defines ComputerVersion, Materializer, StateGenerator, typed observation/equivalence contracts, and candidate-package verification, but VMManagerScopedMaterializer explicitly does not launch a VM and StateGenerator does not enforce ArtifactProgramRef binding."
      evidence_ref: "internal/computerversion/types.go; internal/computerversion/vmmanager_boundary.go; internal/computerversion/state_generator.go"
    - claim: "Ordinary routing and recovery still depend on owner/desktop/VM identity and durable_legacy_opaque data.img state; route-over-ComputerVersion and production construction are not load-bearing."
      evidence_ref: "docs/computer-ontology.md live/target table; docs/agent-product-doctrine.md H031 note"
  problem:
    classification: substrate
    statement: "Choir currently treats an opaque mutable VM disk as the durable computer. The advertised audited-computer contracts classify and verify fragments but do not construct the production computer. A full disk, failed journal, or damaged realization therefore destroys availability and triggers an in-place reboot loop."
    existing_fix_connection: "Connect and complete internal/computerversion as the sole production construction boundary; do not add another parallel constructor or patch the opaque reboot loop into permanence."
  unknowns:
    - "Dolt semantic integrity of the failed owner image; ext4 clean state is not proof."
    - "Which typed artifact-program records currently suffice to reconstruct every required owner-visible state class."
    - "Exact deployed CodeRef and ArtifactProgramRef resolver/storage schemas; they must be discovered from canonical authorities, not invented as a third state store."

finish:
  deliver: "A production function constructs and boots a disposable Choir realization from one immutable ComputerVersion = (CodeRef, ArtifactProgramRef), proves the requested typed state after boot, and makes that accepted realization routable with bounded rollback. The function does not read or clone a prior mutable data.img."
  artifact: "The staging owner computer is served by a fresh Firecracker realization produced by the production ComputerVersion materializer, with a durable construction receipt, observation set, promotion certificate, route transition, and rollback reference."
  acceptance:
    - action: "Create a new test ComputerVersion whose immutable CodeRef and ArtifactProgramRef contain unique verifier-known markers; delete or prove absence of any target realization; call the same production construction-and-boot function used by lifecycle recovery."
      proves: "The computer is generated from identified code and typed durable state, not selected from, cloned from, or repaired inside an opaque VM image."
      evidence_class: deployed_constructor_receipt
    - action: "Before route publication, independently verify the receipt joins the requested ComputerVersion, resolved immutable inputs, generated image/rootfs hashes and geometry, materializer capability manifest, and candidate realization identity."
      proves: "The function's inputs, outputs, and state provenance are durable and recomputable."
      evidence_class: independent_typed_join_verification
    - action: "Boot the generated realization, obtain guest health and readiness, then fetch exact marker bytes, the expected embedded-Dolt head/object-graph facts, blob hashes, and provenance answers through authenticated supported product paths."
      proves: "The function produced a live Choir computer whose observable state matches the requested ComputerVersion."
      evidence_class: deployed_boot_and_product_readback
    - action: "After first acceptance, record the realization ID, state root, data-image path and inode/device or equivalent content identity, boot epoch, generated image hashes, and construction receipt. Stop it; delete or quarantine its entire state root and embedded-Dolt workspace; reclaim its TAP, firewall, port, and lease resources; and prove the old paths are absent or unreadable. Reinvoke the same production construction-and-boot function for the same ComputerVersion in a fresh state root."
      proves: "The second receipt has a distinct realization identity and fresh generated state root, records no source image or first-realization path access, and produces identical required typed observations. Reconstruction therefore depends only on immutable ComputerVersion inputs and named external authorities—not reboot, refresh, hardlink, sparse clone, snapshot, or source-VM copy."
      evidence_class: deployed_zero_realization_reconstruction
    - action: "Promote the accepted ComputerVersion through D-ROUTE with one vmctl-owned compare-and-swap route transition; prove the served route identity is that immutable ComputerVersion, proxy reads it only through vmctl's route-ledger contract, and traffic reaches only its accepted realization; then roll back to the prior accepted ComputerVersion and verify prior state."
      proves: "Routes are over ComputerVersion; lineage owner/desktop routing and hard-coded platform fallback are absent; realization identity is subordinate; promotion and rollback are atomic and bounded."
      evidence_class: deployed_route_promotion_rollback
    - action: "Repeat the production constructor, boot, exact typed readback, route CAS, rollback, and no-SSH inspection path for the staging owner ComputerVersion that will actually serve choir.news."
      proves: "The mission did not stop at a synthetic fixture; the deployed owner route is served by the audited constructor."
      evidence_class: deployed_owner_cutover_acceptance
    - action: "Inject full-disk, failed-GC, corrupted-local-journal, missing-blob, bad-hash, and unavailable-CodeRef cases into disposable candidates; verify refusal/quarantine and construction of a clean replacement where the typed roots remain valid."
      proves: "Local realization failure cannot be reported as active health or force in-place mutation of canonical state."
      evidence_class: adversarial_failure_acceptance
    - action: "From an external client with a scoped Choir key and no SSH, using only supported public/product APIs or Choir CLI commands backed by those APIs—not /internal/*, host files, journalctl, systemctl, or direct database reads—inspect the active ComputerVersion, construction/acceptance receipt, exact artifact bytes, current health, and scoped mutation refusal."
      proves: "The audited computer and its route identity are externally inspectable through supported authority boundaries."
      evidence_class: deployed_no_ssh_product_path
  rollback: "Keep the previous accepted ComputerVersion and realization routable for a bounded TTL. On constructor, verifier, deploy, or product-path failure, refuse promotion or CAS the route back to that version; quarantine the failed realization and preserve typed receipts. Never roll back by booting or modifying the failed owner data.img."
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_constructor_receipt, independent_verifier, boot_readback, zero_realization_reconstruction, promotion_rollback, no_ssh_acceptance]
    registry_hygiene:
      required: true
      must_update: [docs/ACTIVE.md, docs/mission-graph.yaml, docs/doc-authority-manifest.yaml]
      acceptance: "This Definition is the sole executable /goal entrypoint, the predecessor is historical evidence, and the standing-questions dangling-reference check reports no missing paths."
  not_done_when:
    - "A route, promotion record, or recovery decision still targets a VM or desktop as durable identity."
    - "The constructor can silently use a prior data.img, persistent VM directory, mutable branch name, or unverified resolver result."
    - "ArtifactProgramRef is caller-trusted rather than resolved and cryptographically bound to the exact replayed state."
    - "Only unit tests, a generated filesystem directory, a candidate-package manifest, VM launch metadata, or an ext4-clean report exists."
    - "Round-trip verification of the generator exists only for hand-picked fixture states, or a host-side filesystem extractor is reachable from production code."
    - "The VM boots but exact typed state and provenance are not read back through the authenticated product path."
    - "Reboot/restart succeeds only because the first realization disk survived."
    - "The current failed owner computer is made reachable by in-place repair without the production constructor becoming the load-bearing lifecycle path."

boundaries:
  mutation_class: red
  conjecture_delta: "Replace 'the mutable VM disk is the computer' with 'ComputerVersion is the durable computer; a production function deterministically resolves, constructs, boots, verifies, and realizes it.'"
  protected_surfaces: [ComputerVersion, artifact_program, embedded_Dolt, blobs, actor_recovery, vmctl, Firecracker, owner_route, promotion_rollback, auth_session, staging_deploy]
  admissible_evidence_class: "Deployed constructor invocation plus independent typed-join verification, zero-realization reconstruction, authenticated exact readback, and route rollback. Local tests and opaque image health are supporting evidence only."
  rollback_path: "Prior accepted ComputerVersion route; constructor candidates remain isolated until acceptance; failed realizations are quarantined, not repaired into acceptance."
  heresy_delta:
    discovered: ["H031 remains load-bearing", "durable_legacy_opaque data.img is production authority", "in-place reboot masquerades as recovery", "StateGenerator trusts an unenforced ArtifactProgramRef binding"]
    introduced: []
    repaired_when: ["all product routes resolve ComputerVersion", "production lifecycle calls audited constructor", "opaque realization disks are disposable", "candidate-as-VM lifecycle is deleted"]
  authority_sources:
    - "owner direction recorded 2026-07-15: make the autoputer real; proof must be a function constructing and booting it"
    - AGENTS.md
    - docs/standing-questions.md
    - docs/choir-doctrine.md
    - docs/agent-product-doctrine.md
    - docs/computer-ontology.md
    - docs/definitions/og-dolt-heresy-completion-2026-07-08.md
  must_preserve:
    - "Exactly two non-conflated Dolt stores: corpusd world-wire ObjectGraphStore and one VM-local embedded Dolt workspace per user computer. Construction must not create a third semantic current-state store."
    - "D-ROUTE is settled: route-slot and transition-receipt tables live on the corpusd world-wire sql-server; vmctl is the sole CAS writer. Route authority must not live in vmctl's JSON ownership registry, a third Dolt domain, or VM-local application state."
    - "Before promotion acceptance, classify every writer of ActiveSourceRef, RouteProfile, route-slot, or route-equivalent state. Existing promotion/candidatepackage lineage writers may remain only as source/build metadata writers and must be unable to publish or roll back routable state."
    - "CodeRef identifies an immutable executable closure; ArtifactProgramRef identifies an immutable, ordered, tamper-evident typed program. Mutable aliases may resolve only through durable receipts that pin immutable outputs before construction."
    - "Actor recovery uses its narrow recovery log only; it must not become competing app, trajectory, or promotion truth."
    - "Construction receipts, observation sets, verifier joins, and promotion certificates are immutable evidence/provenance keyed by ComputerVersion and realization ID. They cannot override ArtifactProgramRef, embedded-Dolt app state, actor recovery, or the vmctl route slot and are not a third current-state store."
    - "The failed owner image and the pre-e2fsck/rollback images remain read-only evidence until semantic recovery is settled."
    - "A constructed realization is not routable until an independent verifier recomputes all required observations and the promotion certificate."
    - "Problem-documentation-first is satisfied by this Definition commit; implementation commits must reference this recorded substrate failure."
  excluded:
    - "Making ordinary Choir development run in capsules from an external Choir CLI agent; that is a successor mission, not hidden scope here."
    - "Autopaper activation."
    - "Deleting test accounts, orphaned ownerships, manual snapshots, or stale Nix roots except where a separately reviewed safety action is required to admit construction."
    - "Semantic merge of opaque VM memory or local disk accidents."
    - "A second ComputerVersion type, alternate route registry, shadow current-state projection, or third Dolt store."
    - "Treating a clone-and-repair of the failed image as final architecture."
  authority_boundary:
    owner: "Choir product owner settles product identity, destructive recovery, and acceptance-scope decisions."
    orchestrator: "May implement and order work within this Definition; architecture proposals that conflict with owner-settled doctrine remain unratified."
    vmctl: "Owns realization lifecycle and route CAS only; it does not become semantic state authority."
    materializer: "Resolves immutable inputs, constructs a candidate realization, and emits evidence; it cannot publish a route."
    verifier: "Independently recomputes observations and acceptance; it cannot construct or promote."

conjectures:
  - id: C1-resolvable-computer-version
    claim: "Every active route can resolve one immutable CodeRef and one immutable ArtifactProgramRef without reading a VM disk."
    falsifier: "Any required durable owner-visible state exists only inside data.img or an unreceipted mutable alias."
    decision: "Inventory that state, assign its canonical typed authority, and add migration/extraction before constructor cutover; do not bless the image as authority."
  - id: C2-complete-construction
    claim: "The production materializer can create all boot inputs and typed state for a healthy Firecracker guest from those refs."
    falsifier: "Boot or authenticated readback requires an inherited opaque file, secret, database, branch, or host mutation not named by the capability manifest."
    decision: "Make the dependency an immutable typed input with a receipt, or declare the capability unsupported and block acceptance."
  - id: C3-equivalence
    claim: "Independent post-boot observations are sufficient to accept semantic equivalence for the requested scope."
    falsifier: "A required file, blob, Dolt head, object-graph fact, provenance answer, actor recovery state, or product operation cannot be recomputed."
    decision: "Narrow no owner-required scope silently; repair the extractor/generator/verifier contract before promotion."
  - id: C4-disposable-realization
    claim: "Deleting one realization cannot delete the durable computer."
    falsifier: "Zero-realization reconstruction loses accepted state or needs bytes from the deleted disk."
    decision: "Reject cutover and move the missing state into its settled typed authority."
  - id: C5-route-rollback
    claim: "vmctl can route atomically between accepted ComputerVersions while realizations remain replaceable."
    falsifier: "Route truth names VM/desktop identity, split-brain is observable, or rollback requires mutable disk repair."
    decision: "Keep the legacy route frozen, repair CAS/acceptance joins, and repeat in a disposable candidate."

execution:
  - phase: A-contain-and-extract
    outcome: "Freeze in-place recovery of the failed owner realization, preserve all images, produce typed recovery/geometry/Dolt receipts from a stopped clone, and enumerate every state class needed for construction."
    gate: "No source image mutation; semantic unknowns remain explicit; every required state class has one canonical authority or a named blocker."
  - phase: B-resolve-immutable-inputs
    outcome: "Wire route lookup to ComputerVersion and implement durable resolvers that pin CodeRef and ArtifactProgramRef to immutable, hash-verified construction inputs."
    gate: "Delete LineageBasedRouteResolver, PROXY_RUNTIME_DB_PATH/RuntimeDBPath, static or hard-coded owner/desktop fallback routing, and every route use of ActiveSourceRef or RouteProfile. Inventory every route and activation writer; app-adoption and candidate-package switch/rollback/roll-forward may remain only as non-route source/build metadata and cannot publish routable state. The active D-ROUTE slot resolves exactly one immutable ComputerVersion; resolver failure is product-visible refusal, never fallback. No caller-trusted journal binding, mutable-ref race, JSON-registry route authority, or third state store remains."
  - phase: C-construct-and-boot
    outcome: "Implement one production construction-and-boot function that creates a fresh correctly sized filesystem/realization, installs the immutable code closure, verifies the exact replayed journal/root against ArtifactProgramRef, replays typed file/blob/Dolt/app/actor state, boots Firecracker, and emits a construction receipt."
    gate: "The function rejects missing or mismatched pins and is the lifecycle path for initial construction and failed-realization replacement. It never reads or clones a prior data.img and cannot invoke vmmanager SourceVMID/copySparseFile or existing-image resize/recovery paths. Its receipt classifies every input as immutable CodeRef closure, immutable ArtifactProgramRef tape, platform-owned service dependency, ephemeral realization credential/config, generated state, cache, or unsupported blocker; gateway/network credentials are ephemeral and provider secrets remain platform-owned. A generative round-trip harness proves Generate-then-extract equivalence against journal-derived observations for arbitrary valid tapes, not only hand-picked fixtures; host-side filesystem extraction is test scaffolding only and is not admissible acceptance evidence."
  - phase: D-verify-and-route
    outcome: "Independently extract and compare post-boot observations, implement the promotion certificate and vmctl-owned D-ROUTE transition, and freeze the complete verifier/promotion/route candidate. This phase stops at the frozen candidate and executes no route compare-and-swap."
    gate: "Exact authenticated readback, health, geometry/headroom, provenance, rollback rehearsal, and all deterministic verifier and route checks pass on the frozen candidate. G3 acceptance is required before every D-ROUTE CAS; the proposed route is the immutable ComputerVersion and VM identity is absent from durable route authority."
  - phase: E-destroy-and-reconstruct
    outcome: "After accepted G3, the integration authority first executes the reviewed test-route CAS and bounded rollback, then discards the accepted test realization, removes or renders unreadable every backing-state path and reclaims host network resources, and reconstructs equivalent state from the same ComputerVersion in a fresh state root; corruption/full-disk/refusal cases are exercised."
    gate: "The verifier proves first-realization paths were unavailable, the second receipt names distinct realization/state identities and no source image, and all required observations match. Every injected local failure yields refusal/quarantine or clean typed reconstruction."
  - phase: F-cutover-owner-and-close
    outcome: "Execute owner cutover only after accepted G4: recover the affected owner through the audited constructor, perform the staging owner CAS, and collect deployed no-SSH product-path and restart/reconstruction evidence. Then freeze the post-CAS closure packet for G5. Registry updates, the terminal receipt, and status complete are a separate terminal-closure boundary and occur only after accepted G5."
    gate: "Cutover execution requires accepted G4 and a serialized owner CAS. Terminal closure separately requires accepted G5 over the frozen post-CAS evidence; CI and deploy are green for the pushed SHA, staging reports that SHA, all acceptance actions pass, and protected images and rollback refs have explicit dispositions. Legacy in-place owner recovery, route-over-VM fallback, candidate-as-VM authority, vmctl JSON ownership-registry route writes, and duplicate app-adoption/candidate-package activation writers are deleted or hard-refusal-gated. Adoption, lineage, UI, Trace, and acceptance projections advance only after readback of the matching vmctl D-ROUTE receipt; missing, stale, or failed CAS leaves them unchanged. Generic stop/resume/diagnostic VM lifecycle may remain only subordinate to ComputerVersion construction and unable to publish route authority."

orchestration:
  orchestrator: OMP
  integration_authority: "One OMP integration authority owns protected mutations, adjudication, promotion, rollback, landing, and updates to this Definition."
  topology:
    - order: 1
      stage: base-reconciliation
      mode: serialized_read_only
      rule: "Reconcile canonical/origin refs, deploy identity, dirty work, registries, protected images, and candidate ownership before dispatch or mutation."
    - order: 2
      stage: mapping-and-candidates
      mode: parallel_bounded
      rule: "Fan out read-only mapping and isolated candidates only when their worktrees, path scopes, state roots, external effects, and evidence outputs are disjoint and pinned to the reconciled base."
    - order: 3
      stage: protected-integration
      mode: serialized
      rule: "The integration authority serializes shared-path and protected-surface mutation, promotion, rollback, owner cutover, Definition/registry updates, and landing. The first disposable test-route D-ROUTE CAS and entry to E require accepted G3; the staging-owner D-ROUTE CAS and entry to F require accepted G4; no other D-ROUTE CAS is authorized by this Definition; registry terminal closure requires accepted G5."
  phase_topology:
    - phase: A-contain-and-extract
      depends_on: [base-reconciliation]
      fan_out: "yes: read-only authority/state mapping and stopped-clone evidence collection may fan out; source-image mutation may not"
    - phase: B-resolve-immutable-inputs
      depends_on: [A-contain-and-extract]
      fan_out: "yes: read-only writer/resolver mapping and isolated disjoint candidates may fan out; protected integration is serialized"
    - phase: C-construct-and-boot
      depends_on: [G1-immutable-input-state-authority]
      dependency_condition: "G1 adjudication is accept; any other outcome prevents constructor mutation and prevents entry to C."
      fan_out: "yes: isolated disjoint constructor and round-trip candidates may fan out; production-path mutation is serialized"
    - phase: D-verify-and-route
      depends_on: [G2-frozen-constructor-round-trip]
      dependency_condition: "G2 adjudication is accept; any other outcome prevents promotion work and prevents entry to D."
      fan_out: "yes: independent read-only verification may fan out; integration only freezes the verifier/promotion/D-ROUTE candidate, and no route CAS is allowed before accepted G3"
    - phase: E-destroy-and-reconstruct
      depends_on: [G3-frozen-verifier-promotion-route-candidate]
      dependency_condition: "G3 adjudication is accept; any other outcome prevents every D-ROUTE CAS and prevents entry to E."
      fan_out: "yes: after the reviewed test-route CAS, disposable disjoint failure scenarios may fan out; accepted-state destruction, rollback, and shared host-resource mutation are serialized"
    - phase: F-cutover-owner-and-close
      depends_on: [G4-frozen-deployed-cutover-packet]
      cutover_execution: "Accepted G4 precedes the serialized staging owner CAS; deployed product-path probes may fan out only after readback of that CAS."
      terminal_closure: "After owner CAS and deployed product-path evidence, freeze the G5 packet. Registry updates, the terminal receipt, and now.status complete require G5 adjudication accept."
      fan_out: "yes: read-only deployed acceptance probes may fan out after owner CAS; owner CAS, rollback, registry updates, terminal closure, and landing are serialized"
  review_policy:
    mechanism: agentic-consensus
    role_independence: "Builder, falsifier, and verifier are distinct agents with distinct obligations; none may self-approve its own output."
    independence_telemetry: "Every durable review receipt records distinct agent identity, model family and version, context-or-memory lineage, tool/search source, obligation, failures, latency, cost, and unique finding yield for each participant. Missing or conflated independence fields fail the review packet; telemetry never proves acceptance."
    deterministic_gates_first: true
    panel_scope: "Only the five frozen decision gates below; no per-slice consensus, moving-target review, or dashboard-only commit is authorized."
    acceptance_limit: "A panel supplies challenge evidence and an adjudicated decision receipt; panel agreement does not prove product acceptance or completion."
    adjudication: [accept, repair, reject, escalate]
    blocker_precedence: "Any reproducible minority blocker overrides an unsupported majority pass and prevents advancement until repaired, rejected with evidence, or escalated to the named authority."
  decision_gates:
    - id: G1-immutable-input-state-authority
      review_kind: agentic-consensus
      changes_decision: "Whether the immutable-input and canonical-state boundary is sufficient to authorize constructor implementation."
      after: B-resolve-immutable-inputs
      before: C-construct-and-boot
      frozen_input_required: [base_ref, candidate_ref, path_scope, content_digest, evidence_refs]
      deterministic_first: "Resolver, writer-inventory, immutable-binding, authority-domain, and no-third-store gates run before panel review."
      builder_obligation: "Present the minimal constructor boundary and a complete typed map from each required state/input class to its settled immutable authority."
      falsifier_obligation: "Seek state available only from mutable aliases, prior data.img, hidden host dependencies, competing route writers, or a third semantic store."
      verifier_obligation: "Independently resolve the pinned CodeRef and ArtifactProgramRef and check every authority join and refusal against canonical contracts."
      adjudication: [accept, repair, reject, escalate]
      minority_rule: "A reproducible minority blocker overrides an unsupported majority pass."
      durable_evidence_ref: "required:G1 frozen review packet, independence telemetry, and adjudication receipt"
    - id: G2-frozen-constructor-round-trip
      review_kind: agentic-consensus
      changes_decision: "Whether the frozen constructor and generative round-trip candidate is fit to begin promotion work."
      after: C-construct-and-boot
      before: D-verify-and-route
      frozen_input_required: [base_ref, candidate_ref, path_scope, content_digest, evidence_refs]
      deterministic_first: "Build, focused contract tests, generative Generate-then-extract equivalence, source-access refusal, and receipt-join gates run before panel review."
      builder_obligation: "Present the production construction-and-boot candidate, capability/input classification, receipts, and arbitrary-valid-tape round-trip evidence."
      falsifier_obligation: "Attack hidden source-image access, mutable-ref races, unbound journals, incomplete state replay, geometry/headroom, and fixture-only equivalence."
      verifier_obligation: "Independently run the deterministic gates and recompute input/output, receipt, observation, and no-prior-realization joins from the frozen candidate."
      adjudication: [accept, repair, reject, escalate]
      minority_rule: "A reproducible minority blocker overrides an unsupported majority pass."
      durable_evidence_ref: "required:G2 frozen review packet, independence telemetry, and adjudication receipt"
    - id: G3-frozen-verifier-promotion-route-candidate
      review_kind: agentic-consensus
      changes_decision: "Whether the implemented verifier, promotion certificate, and vmctl D-ROUTE candidate authorizes the first route CAS and entry to E."
      after: D-verify-and-route
      before: [any-D-ROUTE-CAS, E-destroy-and-reconstruct]
      frozen_input_required: [base_ref, candidate_ref, path_scope, content_digest, evidence_refs]
      deterministic_first: "Focused verifier and promotion contract checks, exact typed joins, route-writer inventory, vmctl-only CAS checks, stale-base and split-brain refusal, rollback rehearsal, and immutable ComputerVersion route checks run before panel review."
      builder_obligation: "Present the implemented verifier/promotion/D-ROUTE candidate, immutable promotion and transition receipts, bounded CAS/rollback plan, and deterministic evidence without executing a route CAS."
      falsifier_obligation: "Attack forged or stale verifier joins, mutable aliases, competing route writers, VM-identity route leakage, split-brain transitions, missing CAS preconditions, and rollback that depends on mutable disk repair."
      verifier_obligation: "Independently rerun deterministic checks, recompute ComputerVersion/realization/promotion/route joins from the frozen candidate, and prove the proposed first CAS is vmctl-owned, atomic, bounded, and still unexecuted."
      adjudication: [accept, repair, reject, escalate]
      minority_rule: "A reproducible minority blocker overrides an unsupported majority pass."
      durable_evidence_ref: "required:G3 frozen review packet, independence telemetry, and adjudication receipt"
    - id: G4-frozen-deployed-cutover-packet
      review_kind: agentic-consensus
      changes_decision: "Whether frozen deployed reconstruction evidence authorizes the staging owner route CAS and cutover execution."
      after: E-destroy-and-reconstruct
      before: [F-cutover-owner-and-close, staging-owner-D-ROUTE-CAS]
      frozen_input_required: [base_ref, candidate_ref, path_scope, content_digest, evidence_refs]
      deterministic_first: "Pushed-SHA, CI, deploy identity, constructor/readback, zero-realization reconstruction, adversarial refusal, prior-route rollback, CAS-precondition, and no-SSH product-path readiness gates run before panel review."
      builder_obligation: "Present one immutable owner-cutover candidate joining the deployed SHA and environment to constructor, verification, reconstruction, proposed owner route transition, rollback, and product-path readiness evidence."
      falsifier_obligation: "Seek stale or missing identities, unsupported readiness claims, unsafe destruction, split-brain owner CAS, unbounded rollback, legacy route writers, and evidence that depends on SSH or dashboard state."
      verifier_obligation: "Independently recompute the packet joins, confirm deployed reconstruction and prior-route rollback, and prove the still-unexecuted owner CAS uses vmctl's frozen D-ROUTE authority."
      adjudication: [accept, repair, reject, escalate]
      minority_rule: "A reproducible minority blocker overrides an unsupported majority pass."
      durable_evidence_ref: "required:G4 frozen review packet, independence telemetry, and adjudication receipt"
    - id: G5-frozen-post-cutover-closure
      review_kind: agentic-consensus
      changes_decision: "Whether the completed owner CAS and deployed product-path evidence authorize registry terminal closure, the terminal receipt, and status complete."
      after: [staging-owner-D-ROUTE-CAS, deployed-no-SSH-product-path-evidence]
      before: [registry-terminal-closure, terminal-receipt, status-complete]
      frozen_input_required: [base_ref, candidate_ref, path_scope, content_digest, evidence_refs]
      deterministic_first: "Owner CAS and route-receipt readback, exact served ComputerVersion identity, deployed no-SSH acceptance, restart and zero-realization reconstruction, rollback availability, pushed-SHA/CI/deploy identity joins, candidate disposition, and registry-hygiene preflight checks run before panel review."
      builder_obligation: "Present a frozen post-cutover closure packet joining the executed owner CAS to served identity, deployed product-path results, restart/reconstruction durability, rollback, candidate disposition, and proposed registry and terminal receipts."
      falsifier_obligation: "Seek stale routes or deployments, product evidence from unsupported paths, hidden SSH dependence, missing acceptance classes, unsafe image/candidate disposition, registry divergence, rollback loss, or any claim inferred from dashboard health."
      verifier_obligation: "Independently recompute all source/deploy/route/product-path/rollback/registry joins and confirm terminal closure changes are absent and status remains non-complete until this adjudication accepts."
      adjudication: [accept, repair, reject, escalate]
      minority_rule: "A reproducible minority blocker overrides an unsupported majority pass."
      durable_evidence_ref: "required:G5 frozen review packet, independence telemetry, and adjudication receipt"
  prohibitions:
    - "No per-slice consensus or model vote authorizes mutation, acceptance, or completion."
    - "No dashboard-only change or generated-view refresh earns a standalone commit."
    - "Parallel workers cannot publish routes, promote, roll back, land, or mutate this Definition's current state."


now:
  status: working
  slice: "A-contain-and-extract"
  question: "Can the affected owner computer be reconstructed from settled typed authorities without reading its failed mutable realization as canonical state?"
  reconciliation:
    observed_at: 2026-07-15T22:36:30Z
    source_ref: main/origin@2ca45c1a6823f23978c9ca1b415abd9789f97152
    deploy_identity: staging@9d9945e65f5b54069e1a86a530cb0960d96b3474
    authority_identities:
      - definition:docs/definitions/choir-audited-autoputer-construction-2026-07-15.md#definition_version=2
      - doctrine:docs/choir-doctrine.md@2ca45c1a6823f23978c9ca1b415abd9789f97152
      - doctrine:docs/agent-product-doctrine.md@2ca45c1a6823f23978c9ca1b415abd9789f97152
      - mission_graph:docs/mission-graph.yaml@2ca45c1a6823f23978c9ca1b415abd9789f97152
      - authority_manifest:docs/doc-authority-manifest.yaml@2ca45c1a6823f23978c9ca1b415abd9789f97152
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: start.worktree_inventory
    status: reconciled
  candidate:
    id: none
    state: none
    ref: none
    owner: none
    base: none
    digest: none
    scope: []
  decision:
    selected: "Use the skill-owned live dashboard as a non-authoritative projection and OMP orchestration with frozen G1 immutable-authority, G2 constructor-round-trip, G3 pre-route-CAS, G4 pre-owner-CAS, and G5 pre-terminal-closure consensus gates."
    kind: operational
    status: settled
    source: owner
    evidence_ref: "Owner choice in this 2026-07-15 conversation, incorporated into this Definition."
    owner_ratification_ref: not_applicable
    recorded_at: 2026-07-15T22:36:30Z
    consequence: "OMP may execute this Definition while optionally serving the skill-owned live dashboard as a non-authoritative projection, but no D-ROUTE CAS, owner CAS, registry terminal closure, terminal receipt, or status complete may cross its named preceding accepted frozen gate."
  evidence_refs:
    - start.observed_artifact
    - docs/ACTIVE.md
    - docs/mission-graph.yaml
    - docs/doc-authority-manifest.yaml
  blocker_or_risk: "The failed owner image may contain the only copy of some accepted state, and its Dolt semantic integrity is unknown. Preserve it; do not confuse forensic extraction with the permanent constructor input model."
  next_action: "Before code mutation, inventory constructor inputs and resolvers from the reconciled source, record the stopped-clone recovery manifest without mutating source images, and freeze one minimal production constructor contract."

successor:
  status: unauthorized_until_this_definition_complete
  candidate_goal: "Make real Choir development run as capsule effects against a forked ComputerVersion, controlled through the supported Choir CLI by an external agent with no SSH."
  prerequisite_receipts: [zero_realization_reconstruction, route_over_computer_version, no_ssh_inspection, scoped_authority_refusal, promotion_rollback]
  note: "This mission makes that successor possible but does not implement or activate external-agent capsule development."

view:
  path: none
  endpoint: "http://127.0.0.1:8787"
  generator: "node skills/definition/scripts/dashboard.mjs docs/definitions/choir-audited-autoputer-construction-2026-07-15.md --serve 127.0.0.1:8787 --watch"
  generator_version: "definition-dashboard-js/v1"
  authority: "The skill-owned live dashboard is a human-readable, non-editable projection only; this Markdown/YAML Definition is the sole execution and completion authority. Localhost is transport for owner inspection, not an authority source."
  refresh: "The skill-owned server renders the owner view in memory rather than dumping YAML or creating a repository artifact; while served, server-sent events move the view from current to explicitly unavailable on source invalidation and back to current only after successful regeneration, without serving stale content as current or inferring state."
  projection_contract:
    mode: "Responsive prose-first editorial layout with the most important information first; neither card-grid composition nor a narrow single-column desktop reader, and no disclosure/expand interactions."
    responsive_layout:
      desktop: "At 1280–1440px, use the available width for a dense, legible editorial composition with aligned information regions; structural phase and gate sections are allowed where they improve scanning."
      mobile: "At 480px and below, preserve the same semantic reading order in one clean column with zero horizontal overflow."
    hierarchy: "Use font family, scale, weight, bold, italics, and restrained semantic color as the primary hierarchy; borders and containers remain secondary."
    content_shape:
      scalar_and_map_fields: "Render as prose or compact definition groups, never decorative bullet lists."
      lists_only_for_source_lists: [finish.acceptance, finish.not_done_when, now.evidence_refs, start.worktree_inventory, execution]
      structural_sections_allowed: [execution, orchestration.decision_gates]
    header: [title, provenance, non_authority_note]
    top_order: [finish, current_next, blocker_question, path, gates, proof, start, secondary_context]
    current_next: [now.status, now.slice, now.next_action]
    blocker_question: [now.blocker_or_risk, now.question]
    path: execution
    gates:
      source: orchestration.decision_gates
      obligations_always_visible_under_plain_language_headings: true
      dissent_evidence_from: [orchestration.decision_gates.durable_evidence_ref, now.evidence_refs]
    proof:
      required_from: finish.acceptance
      referenced_from: now.evidence_refs
      completion_inference: forbidden
    start: [start.source, start.worktree_inventory]
    secondary_context:
      sources: [now.reconciliation, now.decision, now.candidate, now.evidence_refs, successor]
      dissent_from: orchestration.decision_gates
      weak_measures_from: measures
    candidate_none_explicit: true
    provenance: [source_sha256, generator_version, generated_at]
    second_authority: forbidden
---

# Make the Autoputer Real — Audited Computer Construction

The computer is the function result, not the disk that happened to survive yesterday. Completion requires the production system to construct a fresh realization from one immutable `ComputerVersion`, boot it, prove its typed state through the product path, destroy it, and do the same thing again.
