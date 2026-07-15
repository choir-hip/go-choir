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
    outcome: "Independently extract and compare post-boot observations, issue a promotion certificate, and commit D-ROUTE's corpusd route-slot and transition receipt with vmctl-owned CAS only after acceptance."
    gate: "Exact authenticated readback, health, geometry/headroom, provenance, and rollback all pass; the served route is the immutable ComputerVersion and VM identity is absent from durable route authority."
  - phase: E-destroy-and-reconstruct
    outcome: "Discard the accepted test realization, remove or render unreadable every backing-state path and reclaim host network resources, then reconstruct equivalent state from the same ComputerVersion in a fresh state root; exercise corruption/full-disk/refusal cases."
    gate: "The verifier proves first-realization paths were unavailable, the second receipt names distinct realization/state identities and no source image, and all required observations match. Every injected local failure yields refusal/quarantine or clean typed reconstruction."
  - phase: F-cutover-owner-and-close
    outcome: "Recover the affected owner through the audited constructor, cut over staging, verify no-SSH operation and restart/reconstruction durability, delete obsolete in-place recovery/candidate-as-VM authority paths, and record terminal receipts."
    gate: "CI and deploy are green for the pushed SHA; staging reports that SHA; all acceptance actions pass; protected images and rollback refs have explicit dispositions. Legacy in-place owner recovery, route-over-VM fallback, candidate-as-VM authority, vmctl JSON ownership-registry route writes, and duplicate app-adoption/candidate-package activation writers are deleted or hard-refusal-gated. Adoption, lineage, UI, Trace, and acceptance projections advance only after readback of the matching vmctl D-ROUTE receipt; missing, stale, or failed CAS leaves them unchanged. Generic stop/resume/diagnostic VM lifecycle may remain only subordinate to ComputerVersion construction and unable to publish route authority."

now:
  status: working
  slice: "A-contain-and-extract"
  question: "Can the affected owner computer be reconstructed from settled typed authorities without reading its failed mutable realization as canonical state?"
  next_action: "Before code mutation, reconcile canonical main, origin, staging deploy identity, dirty work, and registry state; then inventory constructor inputs and resolvers, record the stopped-clone recovery manifest, and freeze one minimal production constructor contract."
  blocker_or_risk: "The failed owner image may contain the only copy of some accepted state, and its Dolt semantic integrity is unknown. Preserve it; do not confuse forensic extraction with the permanent constructor input model."
  reconciliation:
    base_ref: refs/heads/main@9d9945e65f5b54069e1a86a530cb0960d96b3474
    origin_ref: refs/remotes/origin/main@9d9945e65f5b54069e1a86a530cb0960d96b3474
    deploy_identity: unknown
    wip_inventory_ref: start.worktree_inventory
  candidate_disposition: "No active candidate; stale candidates are excluded and preserved."
  evidence_refs:
    - start.observed_artifact
    - docs/ACTIVE.md
    - docs/mission-graph.yaml
    - docs/doc-authority-manifest.yaml

successor:
  status: unauthorized_until_this_definition_complete
  candidate_goal: "Make real Choir development run as capsule effects against a forked ComputerVersion, controlled through the supported Choir CLI by an external agent with no SSH."
  prerequisite_receipts: [zero_realization_reconstruction, route_over_computer_version, no_ssh_inspection, scoped_authority_refusal, promotion_rollback]
  note: "This mission makes that successor possible but does not implement or activate external-agent capsule development."

view:
  path: none
  generator: none
---

# Make the Autoputer Real — Audited Computer Construction

The computer is the function result, not the disk that happened to survive yesterday. Completion requires the production system to construct a fresh realization from one immutable `ComputerVersion`, boot it, prove its typed state through the product path, destroy it, and do the same thing again.
