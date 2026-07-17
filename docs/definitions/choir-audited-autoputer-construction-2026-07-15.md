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

  corrections:
    - corrected_at: 2026-07-15
      preserves_original_observation: true
      clarification: "The affected staging owner computer is the existing yusefnathanson@me.com account, owner ID 5bd6de97-3b58-408c-bf89-c42c81b083de. The failure is account-specific: other accounts boot, so execution must not diagnose it as a platform-wide boot outage."
      initial_control_account: "The owner reports a@b.com is a nearly new account available for early constructor and boot comparison. Reconcile its current identity and health before use; success there is supporting control evidence and cannot substitute for reconstruction and acceptance on yusefnathanson@me.com."
      evidence_ref: "Owner clarification in this 2026-07-15 conversation."
    - corrected_at: 2026-07-15
      preserves_original_observation: true
      clarification: "Owner-settled completion scope is fleet-wide, not an account-specific repair: every Choir computer must have ComputerVersion as its durable identity, and every served computer must run in a Firecracker realization produced and accepted through the production ComputerVersion materializer. yusefnathanson@me.com is the required failed-state recovery proof; a@b.com is an initial near-new control."
      evidence_ref: "Owner clarification in this 2026-07-15 conversation."
    - corrected_at: 2026-07-15
      preserves_original_observation: true
      clarification: "Owner-settled data scope: preserving recoverable legacy data from yusefnathanson@me.com is desirable and warrants a bounded read-only extractor attempt, but it is not an acceptance prerequisite; no other legacy account data requires migration. Mandatory completion is that data.img and the entire Firecracker realization are disposable. data.img may remain a writable materialization and cache, but no acknowledged durable state may depend on its survival."
      evidence_ref: "Owner clarification in this 2026-07-15 conversation."
    - corrected_at: 2026-07-15
      preserves_original_observation: true
      clarification: "Owner-settled scope expansion: intelligent disk-image instantiation is part of this mission. ComputerVersion and its semantic materializer remain substrate-independent; a subordinate disk-instantiation backend turns a typed capacity/allocation/reclamation policy into a fresh Firecracker-compatible block device and receipt. Backend format, path, sparsity mechanism, and compaction strategy never enter ComputerVersion identity."
      evidence_ref: "Owner clarification in this 2026-07-15 conversation."
    - corrected_at: 2026-07-15
      preserves_original_observation: true
      clarification: "Owner-settled execution environment: Firecracker, vmctl lifecycle, disk allocation/geometry, corruption/deletion recovery, route CAS, and fleet acceptance are proven only by the origin/main deployment on staging Node B. Local macOS may compile and run pure/focused contract tests but cannot supply VM acceptance. Canonical main is the serialized integration and landing surface; linked worktrees are optional isolation for genuinely disjoint or risky candidates, never an alternative runtime or acceptance environment."
      evidence_ref: "Owner clarification in this 2026-07-15 conversation."
    - corrected_at: 2026-07-15
      preserves_original_observation: true
      clarification: "Owner-settled invocation precondition: /goal starts only after the owner commits and pushes all intended mission/registry changes and canonical main is clean. Startup verifies that clean main equals origin/main; unexpected dirtiness is a reconciliation blocker to classify and preserve, not normal mission input."
      evidence_ref: "Owner clarification in this 2026-07-15 conversation."
finish:
  deliver: "Make ComputerVersion the durable identity and substrate-independent production construction boundary for every Choir computer. Every served computer runs in a disposable Firecracker realization constructed, booted, verified, and accepted from one immutable ComputerVersion = (CodeRef, ArtifactProgramRef). A subordinate typed disk-instantiation backend—not ComputerVersion logic—selects and optimizes the Firecracker-compatible block-device realization. Initial creation and recovery use the same materializer, and no served realization reads or clones a prior mutable data.img."
  artifact: "Every existing Choir computer is inventoried and bound to an immutable ComputerVersion; every routable staging computer, including yusefnathanson@me.com (owner ID 5bd6de97-3b58-408c-bf89-c42c81b083de), is served by a Firecracker realization produced by the production ComputerVersion materializer through an independently testable disk-instantiation contract, with durable construction, disk-allocation, acceptance, observation, promotion, route-transition, and per-route rollback receipts. Newly created and recovered computers use that same path."
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
    - action: "For every constructed realization, compare the configured capacity, backing data.img apparent and allocated size, Firecracker block-device size, partition geometry if present, ext4 block count and block size, and guest statfs capacity/headroom. Refuse acceptance unless the filesystem spans the intended device capacity minus only explicitly bounded partition/filesystem metadata and configured reserve; a 32 GiB data.img exposing only a 16 GiB filesystem is a hard failure."
      proves: "The constructor creates coherent usable storage geometry and cannot repeat the current oversized-image/undersized-filesystem failure."
      evidence_class: deployed_storage_geometry_acceptance
    - action: "Resolve one typed disk-instantiation policy independently of ComputerVersion identity, then have the production materializer request a fresh Firecracker-compatible block device only through the disk backend contract. Verify the request and receipt bind logical capacity, filesystem, allocation strategy, reclaim/rebuild policy, cache budget, headroom thresholds, backend/version, generated device identity, apparent and allocated bytes, and geometry. Prove changing backend implementation or allocation policy for the same ComputerVersion changes only realization evidence, not ComputerVersion identity or required semantic observations."
      proves: "ComputerVersion construction is abstracted over disk-image instantiation; raw sparse files, host paths, mkfs commands, and backend-specific optimization do not leak into durable computer identity or semantic equivalence."
      evidence_class: disk_instantiation_contract_acceptance
    - action: "Using a 32 GiB logical-capacity fixture with at most 1 GiB of selected materialized state, construct a fresh realization and prove host allocated bytes remain at or below 2 GiB while the guest sees the full configured capacity. Drive cache write/delete churn, then invoke the backend's supported reclaim or disposable-reconstruction optimization and prove allocated bytes return to the same bound without changing ComputerVersion or required observations. Verify capacity pressure selects a receipted larger fresh realization rather than in-place mutation of the current disk, and no operation scans, copies, or allocates the full logical image merely because of its apparent size."
      proves: "Disk virtualization is intelligently thin, measurable, reclaimable by trim/hole-punch or clean reconstruction, and independent of logical capacity; a mostly empty 32 GiB computer does not consume 32 GiB of host storage."
      evidence_class: deployed_disk_allocation_optimization
    - action: "After first acceptance, record the realization ID, state root, data-image path and inode/device or equivalent content identity, boot epoch, generated image hashes, and construction receipt. Stop it; delete or quarantine its entire state root and embedded-Dolt workspace; reclaim its TAP, firewall, port, and lease resources; and prove the old paths are absent or unreadable. Reinvoke the same production construction-and-boot function for the same ComputerVersion in a fresh state root."
      proves: "The second receipt has a distinct realization identity and fresh generated state root, records no source image or first-realization path access, and produces identical required typed observations. Reconstruction therefore depends only on immutable ComputerVersion inputs and named external authorities—not reboot, refresh, hardlink, sparse clone, snapshot, or source-VM copy."
      evidence_class: deployed_zero_realization_reconstruction
    - action: "Promote the accepted ComputerVersion through D-ROUTE with one vmctl-owned compare-and-swap route transition; prove the served route identity is that immutable ComputerVersion, proxy reads it only through vmctl's route-ledger contract, and traffic reaches only its accepted realization; then roll back to the prior accepted ComputerVersion and verify prior state."
      proves: "Routes are over ComputerVersion; lineage owner/desktop routing and hard-coded platform fallback are absent; realization identity is subordinate; promotion and rollback are atomic and bounded."
      evidence_class: deployed_route_promotion_rollback
    - action: "For yusefnathanson@me.com, run a bounded read-only extractor against a preserved clone and include any independently verified recoverable typed state in its ArtifactProgramRef; record omissions and corruption honestly. Whether extraction fully, partly, or not at all succeeds, construct and boot that account through the production materializer, perform exact readback of the selected ComputerVersion, route CAS, rollback, and no-SSH inspection. The nearly new a@b.com account may be the first control but cannot substitute for serving the owner account through the constructor."
      proves: "The deployed owner route is served by the audited constructor and no longer depends on the failed image; recovered legacy owner data is a best-effort migration result, not a hidden completion gate or constructor input."
      evidence_class: deployed_owner_cutover_acceptance
    - action: "Inventory every existing Choir computer and route in staging, including active, stopped, hibernated, failed, and unrouted records. Prove each computer's durable identity resolves one immutable ComputerVersion; for every served route, join the route to an accepted Firecracker realization and its production-materializer construction receipt. Exercise creation of a new computer and recovery of an existing disposable computer through the same materializer, then prove no legacy constructor or route-over-VM fallback can serve traffic."
      proves: "Fleet completion is universal: yusefnathanson@me.com is not a one-account exception, every served computer is materialized from ComputerVersion, and future creation and recovery cannot bypass that boundary."
      evidence_class: deployed_fleet_cutover_acceptance
    - action: "Inject full-disk, failed-GC, corrupted-local-journal, missing-blob, bad-hash, and unavailable-CodeRef cases into disposable candidates; verify refusal/quarantine and construction of a clean replacement where the typed roots remain valid."
      proves: "Local realization failure cannot be reported as active health or force in-place mutation of canonical state."
      evidence_class: adversarial_failure_acceptance
    - action: "On a routed disposable staging computer whose ComputerVersion contains verifier-known durable markers, record the active realization and route receipt, stop it through the supported lifecycle path, deliberately corrupt its data.img through the scoped fault-injection path, and request normal lifecycle recovery. Verify vmctl detects and quarantines the damaged realization without repair, clone, resize, or source-image reads; invokes the production ComputerVersion constructor into a distinct fresh state root; independently accepts it; atomically routes traffic to it; and returns the exact marker observations through authenticated product reads. Perform diagnosis and recovery without SSH or manual host repair and record the bounded interruption."
      proves: "Intentional destruction of realization-local disk state triggers seamless product-path reconstruction from ComputerVersion rather than in-place recovery, with no loss of state promised by that ComputerVersion."
      evidence_class: deployed_corrupted_data_img_recovery
    - action: "For the pushed origin/main SHA, verify staging Node B reports that deployed identity, then invoke the production Firecracker constructor, lifecycle recovery, disk-backend geometry/allocation probes, route CAS/rollback, and authenticated readback on Node B. Treat local macOS builds and tests as supporting evidence only."
      proves: "The mission works on the only current Firecracker acceptance host and did not substitute local contracts, host-process fallback, an isolated worktree, or an undeployed commit for production behavior."
      evidence_class: node_b_firecracker_acceptance
    - action: "From an external client with a scoped Choir key and no SSH, using only supported public/product APIs or Choir CLI commands backed by those APIs—not /internal/*, host files, journalctl, systemctl, or direct database reads—inspect the active ComputerVersion, construction/acceptance receipt, exact artifact bytes, current health, and scoped mutation refusal."
      proves: "The audited computer and its route identity are externally inspectable through supported authority boundaries."
      evidence_class: deployed_no_ssh_product_path
  rollback: "Retain each prior accepted ComputerVersion and realization for a bounded per-route TTL while fleet cutover proceeds serially. On constructor, verifier, deploy, or product-path failure, refuse that route's promotion or CAS it back to its prior accepted ComputerVersion; stop further fleet transitions, quarantine the failed realization, and preserve typed receipts. Never roll back by booting or modifying the failed owner data.img."
  landing:
    required: true
    environment: staging_node_b
    required_receipts: [pushed_origin_main_commit, ci, deploy, node_b_environment_identity, node_b_firecracker_acceptance, deployed_constructor_receipt, disk_instantiation_contract_acceptance, deployed_disk_allocation_optimization, independent_verifier, boot_readback, deployed_storage_geometry_acceptance, zero_realization_reconstruction, deployed_corrupted_data_img_recovery, promotion_rollback, fleet_inventory, deployed_fleet_cutover_acceptance, no_ssh_acceptance]
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
    - "Any existing computer lacks an immutable ComputerVersion identity, any served route lacks an accepted production-materializer construction receipt, or any initial-creation or recovery path can serve a realization constructed outside that boundary."
    - "Any constructed data.img, Firecracker block device, partition, ext4 filesystem, or guest statfs view disagrees with the configured capacity beyond explicitly bounded metadata/reserve, including any recurrence of a 32 GiB image exposing only a 16 GiB filesystem."
    - "Corrupting a disposable routed test computer's data.img requires SSH/manual host repair, mutates or reuses the damaged image, loses state promised by its ComputerVersion, or fails to produce and route an independently accepted fresh realization through the normal lifecycle path."
    - "ComputerVersion, artifact-program resolution, semantic observation, route identity, or promotion logic depends on a raw-image format, host path, truncate/mkfs/resize command, sparse-file mechanism, or concrete disk backend."
    - "A fresh 32 GiB logical-capacity fixture with at most 1 GiB selected materialized state allocates more than 2 GiB on the host; cache churn cannot be reclaimed through a supported optimization or disposable reconstruction; growth mutates the current disk in place; or any path scans/copies/allocates the full logical image solely because of apparent size."
    - "A Firecracker, vmctl lifecycle, disk allocation/geometry, corruption/deletion recovery, route, or fleet claim is supported only by local macOS, host-process fallback, an unpushed worktree/commit, or any environment other than the matching origin/main deployment on staging Node B."

boundaries:
  mutation_class: red
  conjecture_delta: "Replace 'the mutable VM disk is the computer' with 'ComputerVersion is the durable computer; a production function deterministically resolves, constructs, boots, verifies, and realizes it.'"
  protected_surfaces: [ComputerVersion, artifact_program, embedded_Dolt, blobs, actor_recovery, disk_instantiation_backend, disk_capacity_policy, vmctl, Firecracker, all_computer_routes, promotion_rollback, auth_session, staging_deploy]
  admissible_evidence_class: "Deployed constructor invocation through the typed disk-instantiation boundary, disk allocation/geometry/reclaim receipts, independent typed-join verification, zero-realization and corrupted-disk reconstruction, authenticated exact readback, and route rollback. Local tests and opaque image health are supporting evidence only."
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
    - "data.img is realization-local writable machine state: it may contain the generated filesystem, embedded-Dolt materialization, package/build caches, temporary files, logs, and other recomputable acceleration state. It is never a ComputerVersion input, route identity, rollback authority, or sole copy of acknowledged durable state."
    - "Legacy pre-cutover data recovery is optional under the owner's explicit authority. After cutover, any user-visible mutation reported as durable must be captured in the settled typed authority and pinned by a new immutable ArtifactProgramRef/ComputerVersion before its persistence is acknowledged; data.img-only writes are disposable and must not be represented as durable."
    - "ComputerVersion and semantic materialization are independent of disk representation. The disk backend receives a typed realization-local plan and returns a Firecracker-compatible device plus immutable receipt; backend choice and policy revision are realization evidence, not CodeRef, ArtifactProgramRef, route identity, or semantic state."
    - "Disk optimization is policy-driven and observable: logical capacity, physical allocation, geometry, cache budget, headroom, reclaim capability, and rebuild/grow decisions are receipted. Prefer discard/reclaim when verified and cheaper; otherwise destroy and reconstruct. Never copy or resize an accepted realization into the next one."
    - "The failed owner image and the pre-e2fsck/rollback images remain read-only evidence through one bounded extractor attempt and explicit disposition. Failure to recover legacy owner data does not block constructor or fleet acceptance."
    - "A constructed realization is not routable until an independent verifier recomputes all required observations and the promotion certificate."
    - "Problem-documentation-first is satisfied by this Definition commit; implementation commits must reference this recorded substrate failure."
    - "Canonical main is the one integration authority and origin/main is the deployed source of truth. Local worktrees may isolate bounded candidates and pure checks, but no worktree identity or local result substitutes for the pushed SHA, CI, Node B deployment identity, or Node B product-path evidence."
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
    materializer: "Resolves immutable inputs, requests realization-local resources through typed subordinate backend contracts, constructs a candidate realization, and emits evidence; it cannot depend on a concrete disk representation or publish a route."
    disk_instantiation_backend: "Owns only realization-local block-device creation, geometry, allocation, reclaim/rebuild, and its receipt under a resolved policy. It cannot read semantic ComputerVersion state, define identity, clone prior mutable disks, accept a realization, or publish a route."

conjectures:
  - id: C1-resolvable-computer-version
    claim: "Every active route can resolve one immutable CodeRef and one immutable ArtifactProgramRef without reading a VM disk."
    falsifier: "Any state required by the selected ComputerVersion exists only inside data.img or an unreceipted mutable alias."
    decision: "For post-cutover acknowledged durable state, assign a canonical typed authority and repair the write/pinning path before cutover. For legacy yusefnathanson@me.com data, make one bounded read-only extraction attempt and include only independently verified results; owner-authorized omission or baseline reset is admissible. No other legacy account data requires migration."
  - id: C2-complete-construction
    claim: "The production materializer can create all boot inputs and typed state for a healthy Firecracker guest from those refs."
    falsifier: "Boot or authenticated readback requires an inherited opaque file, secret, database, branch, or host mutation not named by the capability manifest."
    decision: "Make the dependency an immutable typed input with a receipt, or declare the capability unsupported and block acceptance."
  - id: C3-equivalence
    claim: "Independent post-boot observations are sufficient to accept semantic equivalence for the requested scope."
    falsifier: "A file, blob, Dolt head, object-graph fact, provenance answer, actor recovery state, or product operation required by the selected ComputerVersion cannot be recomputed."
    decision: "Repair the extractor/generator/verifier contract for state promised by the selected ComputerVersion. Legacy owner data outside that explicitly selected scope may be omitted under the recorded owner authority; never claim omitted state was preserved."
  - id: C4-disposable-realization
    claim: "Deleting one realization cannot delete the durable computer."
    falsifier: "Zero-realization reconstruction loses accepted state or needs bytes from the deleted disk."
    decision: "Reject cutover and move the missing state into its settled typed authority."
  - id: C5-route-rollback
    claim: "vmctl can route atomically between accepted ComputerVersions while realizations remain replaceable."
    falsifier: "Route truth names VM/desktop identity, split-brain is observable, or rollback requires mutable disk repair."
    decision: "Keep the legacy route frozen, repair CAS/acceptance joins, and repeat in a disposable candidate."
  - id: C6-disk-instantiation-independence
    claim: "The same ComputerVersion can be realized through any conforming Firecracker-compatible disk backend without changing required semantic observations, while logical capacity remains decoupled from host allocation."
    falsifier: "ComputerVersion or semantic construction code names data.img paths/formats or filesystem commands; backend choice changes semantic identity; a mostly empty logical disk allocates near full capacity; reclaim requires cloning or mutating an accepted disk; or backend replacement cannot be verified from receipts."
    decision: "Move representation-specific operations behind the typed disk-instantiation contract, reject the backend/policy, and repeat construction from immutable inputs. Do not add backend fields to ComputerVersion."

execution:
  - phase: A-contain-and-extract
    outcome: "Freeze in-place recovery of the failed yusefnathanson@me.com realization, preserve source images, run one bounded read-only extractor attempt from a stopped clone, record verified recovered state and explicit omissions, confirm the account-specific contrast with booting accounts, reconcile a@b.com as a nearly new initial control, and enumerate every state class required for future ComputerVersion construction."
    gate: "No source image mutation; extraction results and omissions are explicit; extractor failure does not block baseline reconstruction; every state class promised by a selected ComputerVersion has one canonical authority or a named blocker."
  - phase: B-resolve-immutable-inputs
    outcome: "Wire route lookup to ComputerVersion and implement durable resolvers that pin CodeRef and ArtifactProgramRef to immutable, hash-verified construction inputs."
    gate: "Delete LineageBasedRouteResolver, PROXY_RUNTIME_DB_PATH/RuntimeDBPath, static or hard-coded owner/desktop fallback routing, and every route use of ActiveSourceRef or RouteProfile. Inventory every route and activation writer; app-adoption and candidate-package switch/rollback/roll-forward may remain only as non-route source/build metadata and cannot publish routable state. The active D-ROUTE slot resolves exactly one immutable ComputerVersion; resolver failure is product-visible refusal, never fallback. No caller-trusted journal binding, mutable-ref race, JSON-registry route authority, or third state store remains."
  - phase: C-construct-and-boot
    outcome: "Implement one production construction-and-boot function whose ComputerVersion semantics are independent of disk representation. It resolves a typed realization-local disk policy, asks a subordinate backend for a fresh optimized Firecracker-compatible block device, verifies the backend receipt and coherent capacity-policy device/partition/ext4/statfs geometry, installs the immutable code closure, verifies the replayed journal/root against ArtifactProgramRef, replays typed state, boots Firecracker, and emits a joined construction receipt."
    gate: "The production materializer contains no raw-image path/format, truncate, mkfs, resize, sparse-copy, discard, or compaction logic; those operations live behind the typed disk-instantiation contract. The backend rejects missing policy pins, never reads/clones a prior data.img or invokes SourceVMID/copySparseFile, exposes logical capacity without eager physical allocation, receipts apparent/allocated bytes and geometry, supports verified reclaim or disposable reconstruction, and produces a new larger realization rather than resizing an accepted disk. The constructor remains the lifecycle path for creation and replacement, classifies every input, and proves arbitrary-valid-tape Generate-then-extract equivalence; host filesystem extraction remains test-only."
  - phase: D-verify-and-route
    outcome: "Independently extract and compare post-boot observations, implement the promotion certificate and vmctl-owned D-ROUTE transition, and freeze the complete verifier/promotion/route candidate. This phase stops at the frozen candidate and executes no route compare-and-swap."
    gate: "Exact authenticated readback, health, capacity-policy geometry and headroom, provenance, rollback rehearsal, and all deterministic verifier and route checks pass on the frozen candidate. A backing-image/device/filesystem mismatch, including 32 GiB versus 16 GiB, is refusal. G3 acceptance is required before every D-ROUTE CAS; the proposed route is the immutable ComputerVersion and VM identity is absent from durable route authority."
  - phase: E-destroy-and-reconstruct
    outcome: "After accepted G3, execute the reviewed test-route CAS and rollback, then prove deletion, corruption, and allocation-pressure modes. Remove an entire accepted state root and reconstruct it; separately corrupt a routed test data.img and invoke normal recovery; separately drive cache churn and capacity pressure, then reclaim verified holes or reconstruct into a fresh policy-sized device. Every replacement uses the same ComputerVersion through the disk backend contract in a distinct state root."
    gate: "The verifier proves prior paths were unavailable or quarantined; replacement receipts name distinct realization/device identities, pinned disk policy/backend revisions, coherent geometry, and bounded physical allocation; required observations match; reclaim/rebuild returns the 32 GiB logical fixture with at most 1 GiB selected state to at most 2 GiB host allocation; growth creates a fresh realization; and no case uses SSH, manual repair, in-place mutation, clone, full-image scan/copy, or eager full allocation."
  - phase: F-cutover-fleet-and-close
    outcome: "Execute fleet cutover only after accepted G4: inventory every Choir computer, serialize per-route transitions to accepted ComputerVersions and materialized Firecracker realizations, reconstruct yusefnathanson@me.com through the audited constructor with whatever legacy state the bounded extractor independently verified, and collect deployed no-SSH product-path and restart/reconstruction evidence. a@b.com may be the first near-new control, but completion requires every served computer and the initial-creation and recovery paths. Then freeze the post-cutover closure packet for G5; registry updates, the terminal receipt, and status complete remain a separate boundary after accepted G5."
    gate: "Cutover execution requires accepted G4 and serialized per-route fleet CAS transitions with bounded rollback. Terminal closure separately requires accepted G5 over the frozen post-cutover evidence; CI and deploy are green for the pushed SHA, staging reports that SHA, every existing computer resolves one immutable ComputerVersion, every served route joins an accepted production-materializer realization, initial creation and recovery cannot bypass the materializer, all acceptance actions pass, and protected images and rollback refs have explicit dispositions. Legacy in-place owner recovery, route-over-VM fallback, candidate-as-VM authority, vmctl JSON ownership-registry route writes, and duplicate app-adoption/candidate-package activation writers are deleted or hard-refusal-gated. Adoption, lineage, UI, Trace, and acceptance projections advance only after readback of the matching vmctl D-ROUTE receipt; missing, stale, or failed CAS leaves them unchanged. Generic stop/resume/diagnostic VM lifecycle may remain only subordinate to ComputerVersion construction and unable to publish route authority."

orchestration:
  orchestrator: OMP
  integration_authority: "One OMP integration authority owns the reconciled canonical main worktree, protected integration, coherent commits, push to origin/main, adjudication, Node B deployment evidence, promotion, rollback, landing, and updates to this Definition."
  topology:
    - order: 1
      stage: base-reconciliation
      mode: serialized_read_only
      rule: "Verify canonical main is clean and equals origin/main, registries name this sole executable goal, and staging Node B has a known deploy identity before dispatch or mutation. Unexpected dirtiness or identity mismatch blocks implementation until classified and preserved; the mission does not begin by absorbing WIP."
    - order: 2
      stage: mapping-and-candidates
      mode: parallel_bounded
      rule: "Default to the serialized canonical-main integration path. Fan out read-only mapping or use isolated linked-worktree candidates only when work is genuinely disjoint or a risky/frozen candidate needs isolation; pin each to the reconciled base. Worktrees run pure/focused checks only and never claim Firecracker or staging acceptance."
    - order: 3
      stage: protected-integration
      mode: serialized
      rule: "The integration authority applies accepted candidate changes to canonical main, commits coherent boundaries, pushes origin/main, and requires CI plus matching staging Node B deployment before protected acceptance. It serializes shared-path mutation, promotion, rollback, fleet cutover, Definition/registry updates, and landing. The first test-route CAS and entry to E require accepted G3; fleet transitions and entry to F require accepted G4, proceed one route at a time with rollback, and include the affected owner route. Terminal closure requires G5."
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
    - phase: F-cutover-fleet-and-close
      depends_on: [G4-frozen-deployed-cutover-packet]
      cutover_execution: "Accepted G4 precedes serialized per-route fleet CAS transitions. a@b.com may transition first as the near-new control; yusefnathanson@me.com must be served through the constructor after the bounded legacy-data extraction attempt, but full legacy recovery is not required; deployed product-path probes may fan out only for routes whose CAS receipt has been read back."
      terminal_closure: "After every computer is bound to ComputerVersion, every served route joins an accepted materialized realization, the owner account is served independently of its old image, data.img disposability is proven, and initial-creation and recovery bypasses are absent, freeze the G5 packet. Registry updates, the terminal receipt, and now.status complete require G5 adjudication accept."
      fan_out: "yes: read-only deployed acceptance probes may fan out after each serialized route CAS; route transitions, rollback, registry updates, terminal closure, and landing are serialized"
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
      deterministic_first: "Build, focused contract tests, generative Generate-then-extract equivalence, source-access refusal, disk-backend substitution conformance, capacity-policy image/block-device/partition/ext4/statfs geometry joins, fresh sparse-allocation bound, churn reclaim/reconstruction bound, no-full-image-scan/copy checks, and receipt joins run before panel review."
      builder_obligation: "Present the substrate-independent production materializer, typed disk-instantiation contract, production optimized backend, policy resolver, joined receipts, backend-substitution evidence, arbitrary-valid-tape round-trip evidence, coherent geometry, and bounded physical allocation/reclaim proof."
      falsifier_obligation: "Attack backend details leaking into ComputerVersion or semantic construction, hidden source-image access, mutable policy/ref races, unbound journals, incomplete replay, 32 GiB/16 GiB geometry mismatch, sparse allocation accounting, eager/full-image I/O, unreclaimed deleted cache blocks, in-place growth, and fixture-only equivalence."
      verifier_obligation: "Independently substitute a conforming backend, rerun deterministic gates, and recompute ComputerVersion/input/output/observation joins separately from disk-policy/backend/device/allocation/geometry receipts, proving backend changes affect realization evidence only."
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
      changes_decision: "Whether frozen deployed reconstruction evidence and the complete computer/route inventory authorize serialized fleet cutover."
      after: E-destroy-and-reconstruct
      before: [F-cutover-fleet-and-close, any-fleet-D-ROUTE-CAS]
      frozen_input_required: [base_ref, candidate_ref, path_scope, content_digest, evidence_refs]
      deterministic_first: "Pushed-SHA, CI, deploy identity, constructor/readback, disk-backend abstraction and substitution, coherent storage geometry, bounded sparse allocation and churn reclamation/reconstruction, no-full-image-I/O, zero-realization reconstruction, corrupted-data.img recovery, adversarial refusal, complete computer/route inventory, per-route rollback, creation/recovery checks, bounded owner extractor receipt, and no-SSH readiness run before review."
      builder_obligation: "Present one immutable fleet-cutover candidate joining deployment and inventory to the substrate-independent constructor, disk policy/backend/device/allocation receipts, deletion/corruption/pressure reconstruction, serialized transitions, rollback, bounded owner extraction, owner baseline reconstruction, and product-path readiness."
      falsifier_obligation: "Seek backend coupling in ComputerVersion/semantics, omitted computers, missing materializer or disk receipts, eager/full allocation, unreclaimed cache churn, geometry mismatches, corruption recovery that reuses the disk or needs SSH, acknowledged data whose sole copy is data.img, hidden image reads/copies, lifecycle bypasses, stale identities, split brain, unbounded rollback, and dashboard-derived claims."
      verifier_obligation: "Independently recompute inventory and semantic joins separately from disk policy/backend/allocation/geometry joins; verify backend substitution, bounded allocation after creation/churn, deletion/corruption/pressure replacement identities and observations, owner extraction disposition, deployed rollback, and every proposed vmctl-owned serialized fleet CAS."
      adjudication: [accept, repair, reject, escalate]
      minority_rule: "A reproducible minority blocker overrides an unsupported majority pass."
      durable_evidence_ref: "required:G4 frozen review packet, independence telemetry, and adjudication receipt"
    - id: G5-frozen-post-cutover-closure
      review_kind: agentic-consensus
      changes_decision: "Whether completed fleet cutover, owner-account construction independent of its old image, proven data.img disposability, and deployed product-path evidence authorize registry terminal closure, the terminal receipt, and status complete."
      after: [fleet-D-ROUTE-cutover, owner-account-materializer-cutover, deployed-no-SSH-product-path-evidence]
      before: [registry-terminal-closure, terminal-receipt, status-complete]
      frozen_input_required: [base_ref, candidate_ref, path_scope, content_digest, evidence_refs]
      deterministic_first: "Complete inventory and route receipts, exact ComputerVersion and materializer joins, disk-backend abstraction/substitution, pinned policy/backend/device receipts, coherent geometry, bounded sparse allocation and churn optimization, no-full-image-I/O, deletion/corruption/pressure recovery, bounded owner extraction and independent owner boot, new-computer creation/recovery, no-SSH acceptance, restart reconstruction, rollback, source/CI/deploy identity, candidate disposition, and registry checks run before review."
      builder_obligation: "Present a frozen closure packet joining every computer and route to ComputerVersion and accepted realizations, with substrate-independent materializer and disk-backend evidence, bounded physical allocation/reclaim, coherent geometry, deletion/corruption/pressure recovery, owner cutover, deployed product paths, lifecycle enforcement, rollback, candidate disposition, and proposed terminal receipts."
      falsifier_obligation: "Seek backend details in durable identity or semantic code, omitted routes, missing receipts, full-capacity host allocation for sparse fixtures, unreclaimed churn, full-image scans/copies, in-place growth, geometry mismatch, damaged-disk reuse/manual repair, lifecycle bypasses, unsupported product evidence, SSH dependence, unsafe disposition, registry divergence, rollback loss, or dashboard-derived claims."
      verifier_obligation: "Independently recompute computer/ComputerVersion/route/materializer semantic joins separately from disk-policy/backend/device/allocation/geometry and destroyed/corrupted/pressure replacement receipts, plus source/deploy/product/rollback/registry joins; terminal closure remains absent until acceptance."
      adjudication: [accept, repair, reject, escalate]
      minority_rule: "A reproducible minority blocker overrides an unsupported majority pass."
      durable_evidence_ref: "required:G5 frozen review packet, independence telemetry, and adjudication receipt"
  prohibitions:
    - "No per-slice consensus or model vote authorizes mutation, acceptance, or completion."
    - "No dashboard-only change or generated-view refresh earns a standalone commit."
    - "Parallel workers cannot publish routes, promote, roll back, land, or mutate this Definition's current state."


now:
  status: working
  slice: "F-cutover-fleet-and-close"
  question: "Can the accepted 150-plan G4 packet execute one exact route at a time, beginning with control revalidation and a@b.com, while every transition proves construction, independent verification, signed authority, product-path health, restart durability, and complete rollback before the next route?"
  reconciliation:
    observed_at: 2026-07-17T08:55:00Z
    source_ref: refs/heads/main@6db85e456e2ec2817555c5efd9e1167ec5c2be45_equals_refs/remotes/origin/main@6db85e456e2ec2817555c5efd9e1167ec5c2be45
    deploy_identity: "Node B deployed runtime 42e50b6b1fa3ae7461bb789ec173521a768b548d; CI run 29565482629 succeeded; documentation identity 6db85e45 adds only durable deployed proof"
    authority_identities:
      - definition:docs/definitions/choir-audited-autoputer-construction-2026-07-15.md#definition_version=2
      - doctrine:docs/choir-doctrine.md@d87bdc446ecc28585c3bc08d4d469b9f94d3c246
      - doctrine:docs/agent-product-doctrine.md@d87bdc446ecc28585c3bc08d4d469b9f94d3c246
      - mission_graph:docs/mission-graph.yaml@d87bdc446ecc28585c3bc08d4d469b9f94d3c246
      - authority_manifest:docs/doc-authority-manifest.yaml@d87bdc446ecc28585c3bc08d4d469b9f94d3c246
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: "canonical main and origin/main equal at 6db85e456e2ec2817555c5efd9e1167ec5c2be45 before the G4 adjudication candidate; only the reviewed frozen G4 packet, adjudication receipt, and Definition transition are intentional"
    status: reconciled
  candidate:
    id: g4-fleet-cutover-candidate-2026-07-17
    state: accepted_for_strictly_serialized_execution
    ref: docs/evidence/g4-fleet-cutover-candidate-2026-07-17.json
    owner: integration-authority
    base: refs/heads/main@42e50b6b1fa3ae7461bb789ec173521a768b548d
    digest: "b7f4529535990596b54d2f97e230470cc7d6ba4ec4e17f1c00d67beeed345b1f"
    scope: [150-exact-route-plans, 149-legacy-mutations, one-retained-control, recovered-owner-only-artifact-program, serialized-rollback-before-next]
  decision:
    selected: "Use the skill-owned live dashboard as a non-authoritative projection and OMP orchestration with frozen G1 immutable-authority, G2 constructor/disk-backend round-trip, G3 pre-route-CAS, G4 pre-fleet-cutover, and G5 pre-terminal-closure gates. Completion is fleet-wide substrate-independent ComputerVersion materialization through an intelligent optimized disk-instantiation backend with disposable realizations; legacy yusefnathanson@me.com data gets one bounded best-effort extraction attempt. Canonical main is the serialized integration surface, origin/main is the deploy source, and staging Node B is the sole Firecracker acceptance host."
    kind: architectural
    status: settled
    source: owner
    evidence_ref: "Owner choices covering fleet scope, disposable data.img, legacy-data scope, corruption recovery, geometry, disk-instantiation abstraction/optimization, canonical-main integration, and Node B-only Firecracker acceptance in this 2026-07-15 conversation."
    owner_ratification_ref: not_applicable
    recorded_at: 2026-07-15T22:36:30Z
    consequence: "ComputerVersion and semantic materialization cannot depend on disk representation. Invocation requires clean canonical main equal to origin/main. Every served realization must join an accepted materializer receipt to a pinned disk-policy/backend/device receipt; deletion, corruption, cache churn, and capacity pressure reconstruct through that boundary without losing promised state. Implementation lands through origin/main and protected claims require the matching staging Node B deployment. Worktrees are optional isolation, not acceptance. No fleet CAS or terminal closure may cross its preceding accepted gate."
  evidence_refs:
    - start.observed_artifact
    - docs/ACTIVE.md
    - docs/mission-graph.yaml
    - docs/doc-authority-manifest.yaml
    - "GitHub Actions CI run 29468123745: success; deploy-impact deploy_needed=false"
    - "https://choir.news/health at 2026-07-16T04:36:01Z: proxy/vmctl ok, deployed commit 9d9945e65f5b54069e1a86a530cb0960d96b3474"
    - docs/evidence/audited-construction-phase-a-2026-07-16.md
    - "/tmp/choir-audited-construction-contained/node-b-storage-report.json: protected snapshots verified, no deletion authorized"
    - "/tmp/choir-g1-consensus-final-22a2/omp-gemini35.out: accept, no blockers, high confidence"
    - "/tmp/choir-g1-consensus-final-alternates/omp-gpt55.out: accept, no blockers, high confidence"
    - "GitHub Actions CI 29480269240 attempt 2: success; Node B deploy job 87564091427: success"
    - "https://choir.news/health?g1=7d310551: matching commit and proxy/vmctl healthy"
    - "Node B missing D-ROUTE probe: HTTP 404 route ledger slot not found"
    - "/tmp/choir-g2-construction-churn-proven.patch SHA-256 439da6ce2c7e20f451c185e9d377868693b28d201ec0cac45b74fbcb95de4278"
    - "/tmp/choir-g2-consensus-churn-proven: Codex, Cursor, GPT-5.5, and OpenCode accept; no blocking findings; Gemini unavailable"
    - "nix shell nixpkgs#e2fsprogs -c go test ./internal/diskinstantiation: pass including real ext4 churn/reclaim/same-ID reconstruction"
    - "focused affected Go suites and go vet: pass; Node B Nix vmctl environment evaluation: pass"
    - "/tmp/choir-g3-sealed-writer.patch SHA-256 8754f601425e0e10f0759d32a9502fb51d079469abdcbef2d0c6ea2a583253f3"
    - "/tmp/choir-g3-consensus-sealed-writer-final: Codex, OpenCode, GPT-5.5, Gemini 3.5, and GLM 5.2 accept with no reproducible blocker; Devin and Cursor timed out without a verdict"
    - "commit ab89a200: G3-accepted sealed SQL writer, independent verifier, typed disk receipt, fresh materializer/launcher joins, signed frozen bootstrap/promote/rollback envelopes; no route CAS executed"
    - "GitHub Actions CI run 29536138235: success; Node B activation receipt 514c540203695ca5511524016b3242568f858473 at 2026-07-16T21:42:03Z with vmctl active"
    - "owner-controlled Ed25519 private key stored outside the repository at ~/.config/choir/promotion-authority-ed25519.pem; staged Node B configuration contains only public key oEdoKFfUCLiOsxNX5J8bT3PQDrjqjVeuU8usTLHSYZ4="
    - "Node B activation receipt 7122f2799be4458f4b925be11990321c7e70ffc4; vmctl signed promotion authority and production constructor active"
    - "construction candidate-control-20260716-d: SHA-256 00802b8018459b62f57b4d913c04d5dd642b89c1b43bbc5c5b776df4d02b1984; disk receipt c96eded7; healthy epoch 1; equivalent"
    - docs/evidence/audited-construction-phase-d-2026-07-16.md
    - "owner authorization, 2026-07-16: blanket approval limited to synthetic route computer:autoputer-control:control-20260716 through independently accepted bootstrap, second-version promote/rollback, restart proof, and hibernated rollback retention; no real-user/platform/fleet route authorized"
    - docs/evidence/g3-bootstrap-e-frozen-review-2026-07-17.md
    - "G3 bootstrap-E initial panel: Codex, Cursor, and Gemini accept; GPT-5.5 repair governed because current facts existed only in prompt and /tmp; no G3 signature or route CAS executed"
    - "/tmp/choir-g3-bootstrap-e-consensus-repair: Codex, Cursor, GPT-5.5, and Gemini accept with no blockers and high confidence; OpenCode no usable verdict; Devin and GLM 5.2 timed out"
    - "bootstrap transition receipt c3490ed2-287f-4b9f-a3c3-85c5055a50a0: generation 0 -> 1 at 2026-07-17T03:34:44.112910201Z; exact immutable ComputerVersion and certificate joined; independent route readback matched"
    - docs/evidence/g3-promotion-b-frozen-review-2026-07-17.md
    - "distinct B construction ee63449c7fac; independent verification 0587e697; frozen promotion candidate 193c793e; generation 1 A route readback unchanged; no promote or rollback CAS executed"
    - "/tmp/choir-g3-promotion-b-consensus: Codex, Cursor, GPT-5.5, and Gemini 3.5 accept with no blocking findings and high confidence; OpenCode emitted no verdict; Devin and GLM 5.2 timed out"
    - "promotion receipt f1df7f6f-31df-46da-83b8-ffd5d9a78e40: generation 1 A -> generation 2 B; routed lookup and authenticated product readback selected candidate I and audit-b.txt"
    - "rollback receipt 8ae4f21a-14fd-4800-b044-76edef9604e7: generation 2 B -> generation 3 A, exact target receipt c3490ed2; fresh J reconstruction equivalent and routed A product state restored"
    - "stale generation-1 promotion replay after generation 3: HTTP 409 with byte-identical route readback"
    - docs/evidence/g4-fleet-inventory-2026-07-17.json
    - docs/evidence/g4-fleet-cutover-blocker-2026-07-17.md
    - "G4 inventory: 150 persisted computers/state directories, one accepted ComputerVersion route/ownership, and 149 unversioned legacy ownerships; no deletion authorized"
    - "legacy detach/restore candidate diff f375de11568f: signed exact inventory binding, route-lock/absence checks, preserved VM state, durable hash-addressed receipt in the existing registry, restart-safe exact restore, and stale/constructed/tamper refusal"
    - "go test ./internal/vmctl -run ^TestLegacyOwnershipDetach, full internal/vmctl tests, and go vet ./internal/vmctl: pass"
    - "CI 29559210595: success; Node B deploy receipt commit b746fd71756fd9d0ed84a1317bda549cf351fd22 activated 2026-07-17T06:22:35Z"
    - "deployed disposable legacy lifecycle: signed detach, durable receipt, detached vmctl restart, exact restore, restored restart, invariant data.img device/inode/geometry/timestamps, route absence, and forged-signature HTTP 409 with byte-identical registry"
    - "g4-legacy-detach-restart-restore packet SHA-256 57b1a26f866d36c1ce72f2d0a9af492d528014ce8778ae5b21f754251e7b0083"
    - "G4 bootstrap rollback blocker: 149 absent legacy routes can bootstrap but the signed route ledger has no exact generation-1 rollback-to-absence; legacy restore correctly requires absence, so post-bootstrap failure is not rollback-safe"
    - "bootstrap rollback candidate diff f777e852c566: frozen deterministic bootstrap receipt, paired signed bootstrap/rollback plans, generation-1-only memory/SQL delete CAS, immutable generation-2 receipt, route absence, cross-slot/stale refusal, restart-durable and HTTP-idempotent replay"
    - "go test ./internal/routeledger ./internal/vmctl and go vet both packages: pass"
    - "CI 29561357407 success; Node B deploy receipt e6fa53f10db3ba9499175d7a1d7912a0cbe2f876 activated 2026-07-17T07:07:37Z"
    - "deployed disposable first-bootstrap rollback: signed detach, construct/independent verify, paired-plan acceptance, generation-1 bootstrap, routed readback/healthy guest, stop, signed generation-2 rollback-to-absence, idempotent replay, exact unrouted disposal, exact legacy restore, restart durability, invariant legacy image metadata, and forged-plan HTTP 409 without mutation"
    - "g4-bootstrap-rollback-deployed-proof packet SHA-256 76c6521bba9df23683164d696f8577fb37ef0f0c6071e81c0e96fad37cec2f87"
    - "owner recovered-state ArtifactProgram artifact-program:sha256:9d90c8666a1d9a69f46daca644bb9470505831bb9926e21d2a577d0bd9aa5a6f: 2,076 files / 609,636,416 bytes; verified Texture/VText/actor hashes; independent owner candidate verification receipt verification:sha256:88b3e33d7cf700fa349ff900226a4a64c17e85b93b668f0d610096962774902a; allocated 627,773,440 bytes"
    - "G4 staged lifecycle root-cause cluster and active unrouted disposal candidate diff 159e5f2cf6f: exact pre-stop validation, unrouted-only active stop capability, stop-failure preservation, routed terminal-only contract, and focused/full vmctl tests plus vet pass"
    - "deployed owner zero-realization reconstruction receipt: first disk 5cd0eb3a / verification 88b3e33d; second disk 38d19298 / verification ac873a6d; identical observation 68360005 and product hashes; active disposal receipt 066202bc with route absent; final candidate ownership and data.img absent"
    - docs/evidence/g4-fleet-cutover-candidate-2026-07-17.json
    - docs/evidence/g4-fleet-cutover-accepted-review-2026-07-17.md
    - "G4 repaired panel: Cursor, Gemini 3.5, GPT-5.5, and OpenCode accept with no blockers and high confidence; Devin, Codex CLI, and GLM 5.2 timed out without verdict"
    - docs/evidence/g4-first-fleet-canary-tap-collision-2026-07-17.md
    - "first fleet canary containment: control route reverified; a@b.com exact detach aa54eac3; Firecracker refused shared vm-candidat-tap with EBUSY; candidate cleanup complete; route absent; exact legacy restore and data.img stat invariants pass"
    - "TAP repair candidate diff 99364aebbd48: complete VM ID SHA-256 to 60-bit lowercase base32 within Linux 15-byte vm- interface namespace; explicit control/fleet collision regression; full vmmanager tests and vet pass"
    - "pre-deploy TAP migration: accepted control realization hibernated at epoch 2 under old runtime; old vm-candidat-tap confirmed absent"
  blocker_or_risk: "Accepted G4 execution stopped safely on its first mutable canary. a@b.com was exactly detached, but Firecracker rejected TAP vm-candidat-tap with EBUSY before construction returned. Candidate cleanup completed, route remained absent, and exact legacy restore preserved the reviewed disk tuple. This is a network-allocation substrate blocker, not a route/owner symptom; no other fleet row may move until repaired and deployed."
  next_action: "Commit the collision-resistant full-identity TAP repair. Before deployment, hibernate the sole active old-name control route so the old binary removes its TAP; then push, require green CI and matching Node B identity, resume/reverify control on its hashed TAP, prove distinct long-prefix disposable candidates, refresh the restored a@b.com row, and resume sequence 1 only after exact re-freeze."

successor:
  status: unauthorized_until_this_definition_complete
  candidate_goal: "Make real Choir development run as capsule effects against a forked ComputerVersion, controlled through the supported Choir CLI by an external agent with no SSH."
  prerequisite_receipts: [zero_realization_reconstruction, route_over_computer_version, no_ssh_inspection, scoped_authority_refusal, promotion_rollback]
  note: "This mission makes that successor possible but does not implement or activate external-agent capsule development."

view:
  path: none
  endpoint: "http://127.0.0.1:8788"
  generator: "node skills/definition/scripts/dashboard.mjs docs/definitions/choir-audited-autoputer-construction-2026-07-15.md --serve 127.0.0.1:8788 --watch"
  generator_version: "definition-dashboard-js/v1"
  authority: "The skill-owned live dashboard is a human-readable, non-editable projection only; this Markdown/YAML Definition is the sole execution and completion authority. Localhost is transport for owner inspection, not an authority source."
  lifecycle: "The dashboard process is an in-memory OMP session process, not a system LaunchAgent, so it does not survive a host reboot and must be restarted manually with the generator command above. A Tailscale serve tcp forward on tailnet port 8788 maps this loopback endpoint to the owner's iPhone; that forward survives reboot because Tailscale itself runs as a LaunchAgent, but nothing listens on 8788 until the dashboard process is restarted. Port 8788 is used because 8787 is occupied by HermesWebUI on this host."
  refresh: "The skill-owned server renders the owner view in memory rather than dumping YAML or creating a repository artifact; while served, server-sent events move the view from current to explicitly unavailable on Definition or repository-state invalidation and back to current only after successful regeneration. Every successful regeneration recomputes repository metadata from the dashboard process's worktree without fetching or mutating Git state. Under --watch, changes to dashboard-view.mjs and dashboard-git.mjs hot-reload into the running server; dashboard.mjs server changes still require a process restart. It never serves stale content as current or infers state."
  projection_contract:
    mode: "Responsive prose-first editorial layout with the most important information first; neither card-grid composition nor a narrow single-column desktop reader. Mission authority and evidence have no disclosure/expand interactions; the repository metadata strip alone may collapse its uncommitted-file inventory."
    repository_metadata:
      position: "A compact status strip directly below the title and before the finish."
      source: "Observed read-only from the Git worktree in which the dashboard process runs; never copied from start.worktree_inventory or treated as mission authority."
      identity: "Show the checked-out branch, or detached HEAD explicitly, the abbreviated HEAD commit, the worktree path, and whether it is the primary or a linked worktree."
      upstream: "Show the configured upstream ref and HEAD's commit counts ahead and behind that locally available tracking ref. Do not fetch. Show no-upstream or unknown explicitly, and include the observed tracking-ref identity so cached remotes are not presented as current network truth."
      changes: "Place branch/HEAD and worktree/upstream on one meta row, then a full-width disclosure beneath them. Keep the native triangle on the same line as the file-count label and total +/- LOC so collapsing never moves that summary; open by default. Its compact, separator-free inventory lists every staged, unstaged, conflicted, and untracked non-ignored path with its state and individual +/- LOC; identify binary files separately because they have no meaningful LOC count. Compute deltas relative to HEAD across staged, unstaged, and untracked files. Never report an unreadable or unsupported state as clean or zero."
      refresh: "Recompute on initial render and every successful live refresh; repository-state changes must invalidate and refresh the strip independently of Definition content changes."
      authority: "This strip is situational metadata only. It cannot update start, now.reconciliation, candidate identity, acceptance, or completion."
    session_log:
      position: "Compact footer strip below the projection-only note."
      scope: "Ephemeral process events for this dashboard session only: start, successful refresh cause, script reload, and failure. Also track each currently dirty path's first-seen-dirty time in this session and filesystem last-modified time. No Git fetch; no durable mission authority."
      presentation: "Show the newest five events inline. Collapse earlier events and the dirty-file timestamp inventory behind one closed disclosure so the page stays calm."
      authority: "Session-only projection aid. It cannot update start, now, acceptance, or completion, and it disappears when the process exits."
    responsive_layout:
      desktop: "At 1280–1440px, use the available width for a dense, legible editorial composition with aligned information regions; structural phase and gate sections are allowed where they improve scanning."
      mobile: "At 480px and below, preserve the same semantic reading order in one clean column with zero horizontal overflow."
    hierarchy: "Use font family, scale, weight, bold, italics, and restrained semantic color as the primary hierarchy; borders and containers remain secondary."
    content_shape:
      scalar_and_map_fields: "Render as prose or compact definition groups, never decorative bullet lists."
      lists_only_for_source_lists: [finish.acceptance, finish.not_done_when, now.evidence_refs, start.worktree_inventory, execution, view.projection_contract.repository_metadata.changes]
      structural_sections_allowed: [execution, orchestration.decision_gates]
    header: [title, repository_metadata, provenance, non_authority_note]
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
