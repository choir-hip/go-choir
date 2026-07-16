---
title: "DRAFT — ComputerVersion Realization Performance Optimization"
definition_version: 2
draft: true
executable: false

start:
  captured_at: 2026-07-16T02:26:31Z
  source:
    canonical_ref: refs/heads/main@d82f6322135d5fdd5da2d2152bb55cbf5f24e5da
    origin_ref: refs/remotes/origin/main@d82f6322135d5fdd5da2d2152bb55cbf5f24e5da
    relation: canonical_ref_equals_origin_ref
    deploy_identity: unknown
  worktree_inventory:
    status: reconciled
    evidence_ref: "git status --short observed 2026-07-16T02:26:31Z"
    preservation_rule: "This draft may touch only itself and the three definition registries. Preserve the active construction Definition and all unrelated dashboard/context work in place."
  worktrees:
    - path: /Users/wiz/go-choir
      status: dirty
      class: user_wip
      owner: owner-and-current-session
      touch: read_only
      paths_or_digest: "docs/definitions/choir-audited-autoputer-construction-2026-07-15.md"
      recovery: "Preserve in place; it is the active predecessor Definition and remains the sole /goal entrypoint."
    - path: /Users/wiz/go-choir
      status: dirty
      class: unknown
      owner: unknown
      touch: forbidden
      paths_or_digest: "skills/definition/scripts/dashboard-view.mjs; skills/definition/scripts/dashboard.mjs; skills/definition/scripts/dashboard.test.mjs; skills/definition/scripts/dashboard-git.mjs; CLAUDE.md"
      recovery: "Preserve in place without inspection or mutation."
  candidates:
    - id: none
      ref: none
      base: d82f6322135d5fdd5da2d2152bb55cbf5f24e5da
      scope: []
      disposition: paused
      evidence_ref: none
  observed_artifact:
    - claim: "The audited-construction predecessor is still working and has not produced the deployed constructor, disk-backend, reconstruction, or fleet-cutover receipts this draft must benchmark."
      evidence_ref: docs/definitions/choir-audited-autoputer-construction-2026-07-15.md
    - claim: "The current vmmanager creates a 32 GiB logical raw sparse data.img with os.Truncate plus mkfs.ext4, grows old images with resize2fs, and has a userspace copySparseFile path that reads the logical image in 1 MiB chunks."
      evidence_ref: "internal/vmmanager/manager.go:67-71,639-670,1869-1981"
    - claim: "Historical staging evidence measured approximately 6.9 seconds for one new-account bootstrap and 45-55 ms for repeated warm bootstrap calls; those observations predate the audited ComputerVersion constructor and are not its baseline."
      evidence_ref: docs/archive/incident-vm-bootstrap-stale-route-2026-06-09.md
    - claim: "Firecracker is available on Node B, not in the local macOS development environment. Local timing cannot prove VM realization performance."
      evidence_ref: "owner clarification 2026-07-15; internal/vmmanager/config.go IsFirecrackerAvailable"
  unknowns:
    - "The deployed predecessor constructor's stage-by-stage latency, throughput, allocation, cache-hit, and recovery distributions on Node B."
    - "Which proposed optimizations remain Pareto-improving after correctness, isolation, operability, storage, and complexity costs are measured."
    - "The final owner-approved latency/allocation SLOs; this draft deliberately does not invent thresholds before the predecessor baseline exists."

finish:
  deliver: "A measured Node B production realization path makes new accounts cheap forks of the immutable stock ComputerVersion, reduces cold construction and recovery latency and host storage/IO cost, and preserves every audited-construction invariant."
  artifact: "The staging production ComputerVersion materializer and disk-instantiation backend use an empirically selected Pareto portfolio, with Node B benchmark receipts, stage telemetry, stock-fork behavior, rollback controls, and deployed product-path proof."
  acceptance:
    - action: "On Node B, pin one deployed predecessor SHA, host/storage identity, ComputerVersion fixture set, disk policy, workload, and benchmark harness. Run repeated cold-cache, warm-cache, new-account, stopped-resume, deleted-realization, corrupted-disk, and allocation-pressure baselines before enabling any optimization."
      proves: "Optimization decisions compare against the real Firecracker constructor on its acceptance host rather than local macOS timing, historical bootstrap anecdotes, or synthetic non-VM work."
      evidence_class: node_b_predecessor_baseline
    - action: "For each candidate optimization, run paired baseline/candidate trials on Node B under the same pinned fixture and controlled host-load classes; record sample count, median, p95, p99, variance, failures, CPU, memory, host allocated bytes, bytes read/written, cache hits, and every construction-stage duration."
      proves: "Claims are empirical and attributable; warm, cold, storage, and contention effects are not conflated."
      evidence_class: node_b_paired_benchmark
    - action: "Create a new account by forking the immutable stock ComputerVersion and instantiating its private writable device from a content-addressed verified stock disk seed through the disk backend. Prove no semantic in-guest bootstrap, mutable source computer, account identity, credential, lease, or route state is inherited from the seed."
      proves: "New-account setup is a safe stock ComputerVersion fork and disk-cache optimization, not replayed application bootstrap or mutable data.img cloning."
      evidence_class: deployed_stock_fork_acceptance
    - action: "Admit an optimization only when Node B measurements place it on the observed Pareto frontier for user latency, recovery latency, physical allocation/IO, and operational complexity while all predecessor construction, deletion, corruption, geometry, equivalence, no-SSH, route-CAS, and rollback checks remain green."
      proves: "Performance work does not trade away correctness, isolation, auditability, or operability and does not cargo-cult the proposal list."
      evidence_class: pareto_admission_receipt
    - action: "Run the accepted portfolio on staging through the normal product path for new-account creation, warm access, stopped realization wake, zero-realization reconstruction, and corrupted-data.img recovery; verify the served ComputerVersion and exact required observations before and after every transition."
      proves: "The selected portfolio improves the deployed product path, not only a benchmark binary."
      evidence_class: deployed_performance_acceptance
  rollback: "Every optimization is independently disableable by a pinned runtime/storage policy revision. On regression, stop admission, CAS affected routes to the prior accepted ComputerVersion realization where needed, restore the predecessor materializer/backend policy, and retain benchmark and failure receipts. Never roll back by cloning or repairing mutable user data.img state."
  landing:
    required: true
    environment: staging-node-b
    required_receipts: [pushed_commit, ci, deploy, environment_identity, node_b_predecessor_baseline, node_b_paired_benchmark, pareto_admission_receipt, deployed_stock_fork_acceptance, deployed_performance_acceptance, rollback_rehearsal]
  not_done_when:
    - "The predecessor construction Definition is incomplete or its deployed identities and invariants have not been reconciled into a revised, owner-ratified successor."
    - "Any performance claim comes from local macOS, a host-process fallback, a non-Firecracker fixture, or historical timing rather than the pinned Node B harness."
    - "Thresholds remain draft_to_be_set_from_baseline, a candidate is accepted because it sounds fast, or only best-case/warm-cache results are reported."
    - "New accounts run semantic bootstrap work that should already be represented by the stock ComputerVersion, or inherit a mutable stock data.img."
    - "An optimization changes ComputerVersion identity semantics, weakens exact observations, requires full-image copy/scan, hides physical allocation, or bypasses production lifecycle and route authority."

activation_requirements:
  executable_only_after:
    - "docs/definitions/choir-audited-autoputer-construction-2026-07-15.md reaches complete with pushed SHA, CI, Node B deploy identity, constructor/disk-backend receipts, deletion and corruption reconstruction, fleet cutover, and no-SSH acceptance."
    - "This draft is reconciled against that final implementation and deployed baseline rather than its current conjectural interfaces."
    - "Node B benchmark fixtures, host-load controls, sample sizes, statistical comparison rules, and stage instrumentation are frozen."
    - "Numerical SLOs and the first experiment portfolio are set from observed baselines and ratified by the owner."
    - "All three registries promote exactly one revised successor to executable entrypoint status; this draft is never invoked directly."

boundaries:
  mutation_class: red
  conjecture_delta: "Replace 'correct reconstruction may be slow and new accounts bootstrap themselves' with 'construction cost is measured, stock accounts fork immutable ComputerVersion state, and only empirically Pareto-efficient realization mechanisms reach production.'"
  protected_surfaces: [ComputerVersion, artifact_program, materializer, disk_instantiation_backend, stock_computer_version, stock_disk_seed, vmctl, Firecracker, route_CAS, auth_account_creation, staging_node_b, benchmark_authority]
  completion_evidence_floor: [node_b_predecessor_baseline, node_b_paired_benchmark, deployed_stock_fork_acceptance, pareto_admission_receipt, deployed_performance_acceptance, rollback_rehearsal]
  authority_sources:
    - "owner direction recorded 2026-07-15: create a draft follow-up mission; ideas and alternatives must be empirically verified; benchmarks run on Node B, not local; revise before execution after construction completes"
    - AGENTS.md
    - docs/standing-questions.md
    - docs/choir-doctrine.md
    - docs/agent-product-doctrine.md
    - docs/computer-ontology.md
    - docs/definitions/choir-audited-autoputer-construction-2026-07-15.md
  must_preserve:
    - "ComputerVersion remains exactly (CodeRef, ArtifactProgramRef); performance policy, disk seed, backend, cache, snapshot, and benchmark identity are realization evidence, not durable computer identity."
    - "A stock disk seed is immutable, content-addressed, reproducible from the stock ComputerVersion, free of account-specific identity/secrets/state, and optional for correctness."
    - "All authoritative benchmarks execute on Node B with Firecracker. Local tests may validate pure contracts and instrumentation only and cannot support performance claims."
    - "Correctness gates run before performance comparison. A faster candidate with any equivalence, isolation, route, rollback, corruption, geometry, or no-SSH regression is rejected."
    - "Every candidate reports warm and cold distributions, failures, resource cost, physical allocation, IO, and stage timing; no single aggregate or best-case number governs admission."
    - "Optimization policy is runtime-resolved and receipted. One candidate cannot silently change shared host behavior for other computers."
  excluded:
    - "Executing this draft or mutating production before predecessor completion, baseline reconciliation, numerical SLO ratification, and registry promotion."
    - "Weakening data.img disposability, typed reconstruction, or ComputerVersion/disk-backend abstraction to win benchmarks."
    - "Local Firecracker performance claims on macOS, host-process fallback timing, or extrapolation from CI runners."
    - "Provider/model performance, agent reasoning latency, application query optimization, or unrelated frontend performance."

measures:
  - name: new_account_ready_latency
    kind: telemetry
    baseline: "unknown until pinned Node B predecessor run"
    desired: "draft_to_be_set_from_baseline; move the Pareto frontier without correctness regression"
    decision_use: "Compare stock-fork, disk-seed, warm-pool, and construction-pipeline candidates."
    cannot_prove: "ComputerVersion equivalence, isolation, or production acceptance."
  - name: construction_stage_latency
    kind: telemetry
    baseline: "unknown: ref resolution, disk instantiation, state replay, Firecracker launch, guest ready, verification, route CAS"
    desired: "eliminate dominant measured stages rather than optimize guessed bottlenecks"
    decision_use: "Choose the next experiment and stop work on non-dominant stages."
    cannot_prove: "End-to-end user benefit or safe routing."
  - name: recovery_interruption
    kind: telemetry
    baseline: "unknown on the predecessor's deleted/corrupted realization paths"
    desired: "draft_to_be_set_from_baseline"
    decision_use: "Compare immediate detection, preconstruction, warm resources, checkpoint replay, and routing overlap."
    cannot_prove: "Recovered semantic state is correct."
  - name: physical_allocation_and_io
    kind: telemetry
    baseline: "unknown under pinned stock, churn, deletion, and pressure fixtures"
    desired: "lower allocated bytes and bytes read/written at equal logical capacity and observations"
    decision_use: "Compare sparse seed, reflink/thin mechanisms, reclaim, and disposable reconstruction."
    cannot_prove: "The storage backend is operationally simpler or generally faster."
  - name: operational_complexity
    kind: weak_signal
    baseline: "predecessor implementation after completion"
    desired: "smallest mechanism that achieves material latency/allocation improvement with bounded rollback"
    decision_use: "Reject marginal gains requiring fragile snapshot identity, privileged host machinery, or extra authorities."
    cannot_prove: "Correctness or performance by itself."

experiment_portfolio:
  rule: "These are conjectures, not commitments. Baseline first; test cheap/high-leverage candidates first; retain, revise, or reject each from Node B evidence. Newly discovered candidates may replace them when they dominate."
  candidates:
    - id: immutable-stock-computer-fork
      idea: "New accounts fork the immutable stock ComputerVersion; no semantic guest bootstrap."
      experiment: "Compare current new-account path with stock fork under identical Node B account/product readiness probes."
      principal_risk: "Hidden account identity or mutable state in the stock artifact."
    - id: verified-stock-disk-seed
      idea: "Cache an immutable content-addressed preformatted stock disk seed behind the disk-instantiation backend; use reflink, thin snapshot, immutable-base overlay, or another conforming Firecracker-compatible mechanism only after host capability measurement."
      experiment: "Compare regenerate, seed instantiate, and available host-native copy-on-write mechanisms for latency, allocated bytes, IO, isolation, and deletion behavior."
      principal_risk: "Backend coupling, full-image fallback, or inherited mutable identity."
    - id: artifact-program-checkpoint-delta
      idea: "Materialize from a verified ArtifactProgram checkpoint/root plus immutable delta rather than replaying full history."
      experiment: "Measure replay latency and verification cost across tape lengths and delta sizes while comparing exact final observations."
      principal_risk: "Checkpoint becomes competing authority or masks omitted history."
    - id: shared-immutable-artifacts
      idea: "Reuse verified read-only Nix/store, runtime, frontend, package, and content-addressed blob artifacts across realizations."
      experiment: "Measure cold/warm bytes copied, startup latency, page-cache effects, and cross-account isolation."
      principal_risk: "Mutable or account-specific bytes enter shared state."
    - id: parallel-construction-dag
      idea: "After immutable pins, run independent code/artifact resolution, disk instantiation, blob resolution, and network preparation concurrently."
      experiment: "Instrument the dependency DAG and compare critical-path latency and contention under idle and busy Node B load."
      principal_risk: "Races, duplicate effects, or shared-host resource oversubscription."
    - id: listener-first-startup
      idea: "Start minimal health/product readiness before optional cache warming and deferred non-semantic work."
      experiment: "Compare ready latency and first real operation latency; verify no route publishes before required semantic observations."
      principal_risk: "False readiness or deferred mandatory migration."
    - id: incremental-root-verification
      idea: "Join immutable Merkle/blob/Dolt roots and verify changed material rather than rescanning every byte on ordinary construction."
      experiment: "Compare full and incremental verifier cost with periodic full-audit falsification."
      principal_risk: "Trusted receipt chain admits stale or missing state."
    - id: construct-before-cas
      idea: "Keep the old accepted realization serving while constructing and verifying its replacement, then CAS."
      experiment: "Measure user-visible interruption during upgrade, pressure, and planned replacement."
      principal_risk: "Split brain or writes landing after the replacement input was pinned."
    - id: bounded-warm-resources
      idea: "Maintain a bounded pool of account-neutral preformatted devices, leases, or—only if proven safe—identity-resettable stock guests."
      experiment: "Measure hit rate, idle resource cost, tail latency, entropy/identity isolation, and pool exhaustion behavior."
      principal_risk: "Identity, credential, entropy, or network leakage; persistent idle cost."
    - id: adaptive-capacity-policy
      idea: "Choose logical capacity from observed workload class and replace with a larger fresh realization under pressure."
      experiment: "Compare metadata, construction, allocation, and pressure-recovery costs for candidate capacity classes."
      principal_risk: "Policy churn or frequent reconstruction for real development workloads."
    - id: reconstruction-as-compaction
      idea: "When cache allocation bloats, reconstruct from ComputerVersion instead of coordinating fragile in-place compaction."
      experiment: "Compare trim/hole-punch and reconstruction latency, allocation recovery, IO, and failure behavior."
      principal_risk: "Reconstruction cost dominates or hidden durable state exists only locally."
    - id: immutable-resolution-cache
      idea: "Cache CodeRef closures, ArtifactProgram checkpoints, blob sets, disk seeds, and verifier schemas strictly by immutable digest."
      experiment: "Measure hit/miss latency, storage cost, invalidation behavior, and digest-verification overhead."
      principal_risk: "Mutable alias poisoning or unbounded cache growth."
    - id: remove-linear-image-io
      idea: "Delete userspace full-logical-image scans/copies such as zero-chunk copySparseFile; use clean generation or measured host-native extent/COW mechanisms."
      experiment: "Trace bytes read/written and wall time against logical capacity and actual allocated extents."
      principal_risk: "Host-specific mechanism lacks portable fallback or changes sparsity semantics."
    - id: immediate-recovery-trigger
      idea: "Coalesce Firecracker exit, block IO failure, guest-health, and request-path liveness signals into one in-flight recovery per route instead of waiting only for periodic polling."
      experiment: "Measure detection-to-ready latency, false positives, duplicate suppression, and host load during fault injection."
      principal_risk: "Recovery storms or replacing a slow but healthy realization."
    - id: firecracker-snapshot-restore
      idea: "Evaluate Firecracker snapshot restore only after cheaper candidates; snapshot state must be account-neutral or rigorously identity-reset before routing."
      experiment: "Compare end-to-end latency and operational/security complexity with verified stock disk seed plus normal boot."
      principal_risk: "Stale process, entropy, identity, credential, network, or kernel state; high operational complexity."

now:
  status: blocked_incomplete
  slice: "draft-only-successor-definition"
  question: "After audited construction is complete and measured on Node B, which candidate portfolio is actually Pareto-optimal and what owner-ratified SLOs should govern it?"
  reconciliation:
    observed_at: 2026-07-16T02:26:31Z
    source_ref: main/origin@d82f6322135d5fdd5da2d2152bb55cbf5f24e5da
    deploy_identity: unknown
    authority_identities:
      - definition:docs/definitions/choir-audited-autoputer-construction-2026-07-15.md#working
      - doctrine:docs/computer-ontology.md@d82f6322135d5fdd5da2d2152bb55cbf5f24e5da
      - doctrine:docs/agent-product-doctrine.md@d82f6322135d5fdd5da2d2152bb55cbf5f24e5da
      - mission_graph:docs/mission-graph.yaml@d82f6322135d5fdd5da2d2152bb55cbf5f24e5da
      - authority_manifest:docs/doc-authority-manifest.yaml@d82f6322135d5fdd5da2d2152bb55cbf5f24e5da
    policy_resolution_ref: not_applicable_until_activation
    worktree_inventory_ref: start.worktree_inventory
    status: reconciled
  candidate:
    id: none
    state: paused
    ref: none
    owner: none
    base: none
    digest: none
    scope: []
  decision:
    selected: "Retain this document as a non-executable draft successor. Benchmark only on Node B after the construction predecessor completes; empirically test the listed and newly discovered ideas before selecting a portfolio or numerical SLOs."
    kind: purpose
    status: settled
    source: owner
    evidence_ref: "Owner direction in this 2026-07-15 conversation."
    owner_ratification_ref: not_applicable
    recorded_at: 2026-07-16T02:26:31Z
    consequence: "No implementation, benchmark claim, candidate optimization, deployment, or /goal invocation is authorized. The next action is deferred until predecessor completion, then this draft must be reconciled, revised, numerically bounded, owner-ratified, and promoted in all registries."
  evidence_refs:
    - docs/definitions/choir-audited-autoputer-construction-2026-07-15.md
    - internal/vmmanager/manager.go
    - docs/archive/incident-vm-bootstrap-stale-route-2026-06-09.md
    - docs/computer-ontology.md
  blocker_or_risk: "The load-bearing constructor does not exist in deployed accepted form, so there is no admissible Node B baseline and every optimization ranking remains conjectural."
  next_action: "After the predecessor reaches complete, reconcile this draft against its pushed/deployed identities and receipts, run a read-only Node B baseline, replace draft thresholds with observed owner-ratified SLOs, choose the first bounded experiment set, and promote a revised successor through registry hygiene before invoking /goal."

receipts: []

view:
  path: none
  generator: "node skills/definition/scripts/dashboard.mjs docs/definitions/choir-computerversion-performance-optimization-draft-2026-07-15.md --serve 127.0.0.1:8788 --watch"
  authority: "Any rendered view is a non-authoritative projection. This file is itself a non-executable draft until revised and promoted."
---

# DRAFT — ComputerVersion Realization Performance Optimization

This is a successor design surface, not a command. Do not run `/goal` on it. The audited-construction mission must finish first; then measure its real Firecracker path on Node B, revise this draft from evidence, ratify numerical SLOs, and promote exactly one executable successor.
