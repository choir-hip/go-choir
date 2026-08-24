---
definition_version: 2
definition_id: choir-durable-substrate-preflight-2026-08-24
execution_mode: mission_orchestrator

start:
  captured_at: "2026-08-24T06:10:00Z"
  source:
    canonical_ref: "main@502ced50"
    deploy_identity: "staging https://choir.news; recovery definition complete (0333528 active, head 132,539, epoch 715, build f7b3ccd2)"
  worktree_inventory:
    status: reconciled
    evidence_ref: "git status 2026-08-24 (clean); .agentic-consensus/ gitignored; pre-existing prunable agent worktrees under /private/tmp/choir-* (ownership: earlier sessions; not touched)"
    preservation_rule: "Preserve every non-primary worktree and unrelated WIP. node-b retains data.img.pre-hostdrive-20260824 (rollback reflink) and quarantine images; pre-existing read-only loop mounts (/tmp/test-mount, /tmp/final-mount, /tmp/quin2-ro) predate this mission and are not touched."
  observed_artifact:
    - claim: "Dolt embedded pins are ~5 months stale: github.com/dolthub/driver v1.84.1, github.com/dolthub/dolt/go v0.40.5-0.20260326074512-005921bdd8ca; Go 1.25.6 (go.mod); build via Nix flake (buildGoModule + ICU cgo)."
      evidence_ref: "go.mod; flake.nix commonGoArgs; research agent://DoltTwoResearch (version map, compat matrix)"
    - claim: "Dolt 2.0 (CLI milestone v2.0.0 GA 2026-05-07; embedded driver/v2 v2.2.0 2026-07-15) requires Go >= 1.26.2 and module path github.com/dolthub/driver/v2; API/DSN unchanged; 1.x stores open under 2.x; 2.x writes not readable by 1.x in all cases; per-root journal Sync unchanged; embedded path does not inherit server auto-GC; archive-index memory (approx 350MB-1GB) not configurable via embedded Config; use late v2.x graph (PR 11058 branch-control fix after v2.0.0)."
      evidence_ref: "agent://DoltTwoResearch (full output, 18 primary sources)"
    - claim: "Guest replay per-event apply costs 3.2-6.5s at the 7-9 GiB workspace on the 4096 MiB guest (0.2-0.3 ev/s; recovery completed via the B14 host drive). Host-side same-binary path runs 27-82 ev/s and completed in ~30 min."
      evidence_ref: "docs/evidence/recovery-replay-guest-io-ceiling-assessment-2026-08-24.md; docs/evidence/recovery-complete-2026-08-24.md"
    - claim: "The other active computer (candidate-fleet-d03dacaa7404b1e4412b2e6f) loops ~3s cadence with 'invalid computer event transition: invalid genesis' on trajectory mint/agent persist (observed >=7h 2026-08-24)."
      evidence_ref: "docs/evidence/fleet-d03dacaa-invalid-genesis-loop-2026-08-24.md"
    - claim: "Active-guests Dolt GC is off (RUNTIME_DOLT_GC_DISABLED=1 in the guest unit + 5 GiB size guard in MaybeRunDoltGC); recovered computer health shows persistent_disk warning: true (used 11.6 GiB / 33.5 GiB, above the 8 GiB default cap); no guest-side reclaim path exists for large stores."
      evidence_ref: "nix/autoputer-vm.nix; internal/store/dolt_maintenance.go; guest /health 2026-08-24"
    - claim: "Yaegi private-Go actor kernel definition (choir-private-go-actor-kernel-2026-08-12.md) pins no Go/yaegi version; yaegi v0.16.1 requires go 1.21; the Dolt bump's Go >= 1.26.2 requirement is a strict superset and unblocks rather than conflicts."
      evidence_ref: "docs/definitions/choir-private-go-actor-kernel-2026-08-12.md; proxy.golang.org yaegi v0.16.1.mod"

finish:
  deliver: "The durable substrate is on a current, supported Dolt graph with measured write behavior; the live fleet genesis-loop is fixed; active guests have a decided GC/reclaim policy; and the guest replay per-event-cost ceiling is resolved by evidence (the v2 measurement) or by the appender batch fix, so the Tracks K/F/M/Assurance overhauls start from measured ground."
  artifact:
    - "PF-1: repo on Go >= 1.26.2 (Nix flake + guest image + CI verified) with github.com/dolthub/driver/v2 v2.2.0 + the matching graph (dolt/go v0.40.5-0.20260715172757-a6690826d767, go-mysql-server v0.20.1-0.20260713210757-6d01d00bbbf3, vitess v0.0.0-20260624214226-81d034e0fde8, go-icu-regex v0.0.0-20260610153742-72563bc7ca83, gozstd unchanged); all driver imports migrated to /v2; DSN/root+per-database connector semantics unchanged; embedded smoke suite green (root DB, per-db connect, SHOW DATABASES, CREATE DATABASE, multi-statements, client-found-rows, DOLT_GC close/reopen); a retained-workspace snapshot opens cleanly under v2 with schema/count/head checks."
    - "PF-2: like-for-like A2-style measurement report (v1.84.1 vs driver/v2 v2.2.0, same 4 GiB Firecracker guest, same retained-store copy): p50/p95 per-event apply + checkpoint latency, RSS, GC pauses/OOM events, archive (.darc) behavior + archive-index memory, commit-root/sync + fsync trace, disk growth. Verdict: the Dolt bump does not fix the 3-6s ceiling (expected per research) or it demonstrably does."
    - "PF-3: d03dacaa invalid-genesis loop root-caused with a problem receipt and fixed + deployed; the live computer shows zero mint-fail errors over a monitoring window; its failed-run churn cleaned per the fix."
    - "PF-4: active-guest Dolt GC/reclaim policy decided and implemented: per-guest bounded GC, host-side GC, or an explicitly accepted warning + document; no guest OOM from GC remains possible, and the standing persistent_disk warning disposition is recorded."
    - "PF-5 (gated by PF-2): if the v2 measurement leaves the per-event cost above the operational target, the appender per-page batching fix implements and the same measurement repeats at <= target; if not, the measurement receipt records why it is moot."
  acceptance:
    - action: "PF-1 landed: pushed SHA + CI green + deploy identity; Nix-built guest binary runs with the /v2 driver; embedded smoke suite passes on staging; retained-workspace open check passes on a snapshot (never on the live store without a snapshot)."
      proves: "The substrate builds and runs on the current Dolt graph with no behavioral drift in the store open/DSN/connector path."
      evidence_class: deployed + smoke receipts
    - action: "PF-2 report published with the v1/v2 tables and the verdict; the measurement uses the retained-store copy in the same guest image; RSS and archive-index numbers bound the 4 GiB guest memory risk."
      proves: "The Dolt upgrade's write-path impact is measured, not assumed; the archive memory risk is quantified."
      evidence_class: measurement receipt (disposable copy; no live-store mutation)
    - action: "PF-3 landed + deployed: journal shows the invalid-genesis error cadence at zero for >= 1h post-deploy; the computer remains active/healthy; problem documentation first (receipt precedes the fix commit)."
      proves: "The live fleеt loop is gone without a second semantic writer or tape mutation."
      evidence_class: deployed receipt + monitoring window
    - action: "PF-4 landed: the active-guest GC policy is implemented; a guest GC attempt at a large store neither OOMs nor stalls the runtime (tested on the disposable guest path); the health-warning disposition is in the receipt."
      proves: "No guest can be OOM-bricked by Dolt GC and the reclaim story for active computers is explicit."
      evidence_class: deployed + guest-path test receipts
    - action: "PF-5 (if so gated): per-event apply at the 105k-132k band <= 1s p95 in the same 4 GiB guest; the ceiling receipt names the resolved outcome."
      proves: "The recovery follow-up ('guest replay I/O ceiling') is closed or demonstrably moot."
      evidence_class: measurement receipt
  rollback: "Git revert mission commits via origin/main + CI to the last accepted runtime; canonical events are never rewound. Dolt-specific: 2.x-written stores are not readable by 1.x in all cases, so every v2 open/write happens on a snapshot (reflink) or candidate disk; the retained pre-upgrade snapshot + an untouched v1 binary are kept and proven readable before the flip; downgrade tests run only on untouched snapshots."
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, staging_build_identity, smoke, measurement, d03_fix, gc_policy, ceiling_verdict]
not_done_when:
  - any event tape or canonical head is mutated by this mission (read-only store checks only)
  - the live d03dacaa computer is disrupted by its fix rollout (the fix lands through its guest image/restart with the fence intact)
  - a v2 store flip happens on the retained live store without a snapshot + untouched-v1-binary proof
  - the Dolt upgrade is done without the Go >= 1.26.2 toolchain verification through the Nix guest build + CI
  - the ceiling verdict is reached without the like-for-like 4 GiB-guest measurement (host-only numbers do not count)
  - the effects definition's mode state changes (effects remain OFF)
boundaries:
  mutation_class: red
  authority_sources: [owner direction 2026-08-24, agent://DoltTwoResearch, docs/evidence/recovery-replay-guest-io-ceiling-assessment-2026-08-24.md, docs/evidence/fleet-d03dacaa-invalid-genesis-loop-2026-08-24.md, docs/evidence/recovery-complete-2026-08-24.md, AGENTS.md, docs/choir-doctrine.md]
  must_preserve:
    - Single guest ComputerEventAppender remains the sole semantic event writer.
    - The canonical event chain at head 132,539/acc54c39... is read-only to this mission.
    - DSN/root+per-database connector semantics (commitname/commitemail/database/multistatements/clientfoundrows) stay exactly as today.
    - The 0303528 recovery state (active, epoch 715) is not regressed.
  protected_surfaces: [canonical computer event chain, guest ComputerEventAppender, Dolt store/workspace, vmctl lifecycle, live d03dacaa computer, platform-artifacts store]
  not_goals:
    - The overhauls themselves: Tracks K (key escrow), F (file-CAS), M (mail), and Assurance (docs/definitions/choir-durable-substrate-overhauls-2026-08-23.md) — they begin after this pre-flight's receipts.
    - The effects mission continuation (CoSuper authoring) — separate, still in the effects definition's scope after this pre-flight.
    - The guest machine-size bump (4096 -> 8-12 GiB) — a fallback option only if PF-2/PF-5 cannot reach the target (then a separate decision, not a silent scope add).
  completion_evidence_floor: [deployed PF-1..PF-5 receipts, independent panel review of the PF-2 verdict and the PF-5 scope decision]

measures:
  - signal: "per-event apply p95 at the 105k-132k band"
    baseline: "3.2-6.5s (v0.40, 4 GiB guest, 7-9 GiB workspace)"
    direction: "<= 1s after PF-2/PF-5"
    decides: "whether PF-5 (appender batching) is required"
    limits: "measures only the guest replay write path; does not prove store correctness (covered by the smoke suite)"
  - signal: "archive-index RSS delta under v2"
    baseline: "no archives today (noms chunk + journal)"
    direction: "< 1 GiB incremental in the 4 GiB guest"
    decides: "whether archives are safe on the guest or must be disabled via a config/DSN affordance"
    limits: "a memory-sample proxy; the guest's total budget is the real constraint"
  - signal: "d03dacaa invalid-genesis error cadence"
    baseline: "~3s cadence (>=7h)"
    direction: "0 over >= 1h post-fix"
    decides: "PF-3 closure"
    limits: "does not prove the fix's generality — that is a separate root-cause receipt"
phases:
  - name: Reconcile
    items:
      - "Reconcile start receipt: git state, staging identity, d03 loop evidence, guest disk warning, retained snapshot present."
  - name: PF-1 — Dolt 2.0 Upgrade
    items:
      - "Bump the Nix flake toolchain to Go >= 1.26.2; verify guest image + CI builds (ICU/cgo/zstd)."
      - "Migrate imports to github.com/dolthub/driver/v2 v2.2.0; let tidy resolve the matching graph."
      - "Embedded smoke suite + retained-workspace snapshot open (v1 binary + snapshot retained for rollback proof)."
      - "Land (CI + deploy) and record build identity."
  - name: PF-2 — Like-for-like Measurement
    items:
      - "Build the v2 runtime package and run the A2-style 4 GiB-guest measurement against the retained-store copy (v1 baseline vs v2)."
      - "Publish the measurement receipt with the ceiling verdict (fix-D required or moot)."
  - name: PF-5 — Ceiling Resolution (gated)
    items:
      - "If gated: implement appender per-page batching (per-page transaction, commit at checkpoints; durability authority stays the checkpoint); re-measure to <= 1s p95; receipt."
  - name: PF-3 — d03dacaa Genesis Loop
    items:
      - "Root-cause the genesis validation rejection on the live computer (receipt first per problem-documentation-first)."
      - "Fix, land (guest image rebuild + restart), monitor zero-error window."
  - name: PF-4 — Active-guest GC Policy
    items:
      - "Decide bounded-guest GC vs host-side GC vs accepted warning; implement; guest-path OOM-free test."
      - "Record the persistent_disk warning disposition."
  - name: Close
    items:
      - "Independent panel review of the PF-2 verdict and PF-5 scope decision; final receipt + landing loop summary."
now:
  status: working
  slice: "Definition drafted 2026-08-24; awaiting panel approval round (agentic-consensus on the draft) before execution."
  question: none
  reconciliation:
    observed_at: "2026-08-24T06:10:00Z"
    source_ref: "main@502ced50"
    deploy_identity: "staging https://choir.news; recovery complete at f7b3ccd2"
    authority_identities: [docs/choir-doctrine.md, docs/computer-ontology.md, AGENTS.md]
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: "clean main; prunable agent worktrees preserved"
    status: reconciled
  decision:
    selected: "Scope the loose ends (Dolt 2.0, d03 genesis loop, active-guest GC policy, replay ceiling) into one pre-flight mission executed before the overhauls; Dolt update first in the sequence (Go >= 1.26.2 is a strict superset of the yaegi kernel's go 1.21 requirement); the ceiling fix (fix-D) is gated by the PF-2 measurement."
    kind: architecture
    status: settled
    source: owner direction 2026-08-24
    evidence_ref: "agent://DoltTwoResearch; docs/evidence/recovery-replay-guest-io-ceiling-assessment-2026-08-24.md"
    owner_ratification_ref: "owner confirmed 2026-08-24 (pre-flight before overhauls)"
    recorded_at: "2026-08-24T06:10:00Z"
    consequence: "The overhauls definition (choir-durable-substrate-overhauls-2026-08-23.md) starts its now-card sequencing from this pre-flight's completion."
  evidence_refs:
    - "agent://DoltTwoResearch"
    - "docs/evidence/recovery-replay-guest-io-ceiling-assessment-2026-08-24.md"
    - "docs/evidence/fleet-d03dacaa-invalid-genesis-loop-2026-08-24.md"
    - "docs/evidence/recovery-complete-2026-08-24.md"
  blocker_or_risk: "Dolt v2 one-way format (downgrade unsafe after writes) — controlled by snapshot discipline; archive-index memory in the 4 GiB guest — quantified in PF-2; d03 fix touches a LIVE computer — rollout through its guest image with the fence intact."
  next_action: "Run agentic-consensus on this draft (convergent, default panel), adjudicate findings, iterate until panel approval; then bind the approved definition and begin Reconcile -> PF-1."

receipts:
  - id: define-preflight-draft
    boundary: define
    commit_or_artifact: "(pending panel approval)"
    proof_refs: [agent://DoltTwoResearch, docs/evidence/recovery-replay-guest-io-ceiling-assessment-2026-08-24.md, docs/evidence/fleet-d03dacaa-invalid-genesis-loop-2026-08-24.md]
    rollback_ref: revert docs commit
    disposition: "Pre-flight definition drafted; sequenced before the overhauls"
    problem_ref: "fleet-d03dacaa-invalid-genesis-loop-2026-08-24.md; recovery-replay-guest-io-ceiling-assessment-2026-08-24.md"
    authorization_ref: "owner direction 2026-08-24"
    candidate_or_evidence_refs: []
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: not_applicable
