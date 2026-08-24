---
definition_version: 2
definition_id: choir-durable-substrate-preflight-2026-08-24
execution_mode: mission_orchestrator

start:
  captured_at: "2026-08-24T06:40:00Z"
  source:
    canonical_ref: "main@1fad1da0"
    deploy_identity: "staging https://choir.news; recovery definition complete (0333528 active, head 132,539, epoch 715, build f7b3ccd2)"
  worktree_inventory:
    status: reconciled
    evidence_ref: "git status 2026-08-24 (clean); .agentic-consensus/ gitignored; pre-existing prunable agent worktrees under /private/tmp/choir-* (ownership: earlier sessions; not touched)"
    preservation_rule: "Preserve every non-primary worktree and unrelated WIP. node-b retains data.img.pre-hostdrive-20260824 (rollback reflink) and quarantine images; pre-existing read-only loop mounts (/tmp/test-mount, /tmp/final-mount, /tmp/quin2-ro) predate this mission and are not touched."
  observed_artifact:
    - claim: "Dolt embedded pins are ~5 months stale: github.com/dolthub/driver v1.84.1, github.com/dolthub/dolt/go v0.40.5-0.20260326074512-005921bdd8ca; Go 1.25.6 (go.mod); build via Nix flake (buildGoModule + ICU cgo)."
      evidence_ref: "go.mod; flake.nix commonGoArgs; docs/evidence/dolt-v2-research-2026-08-24.md (promoted from agent://DoltTwoResearch)"
    - claim: "Dolt 2.0 (CLI milestone v2.0.0 GA 2026-05-07; embedded driver/v2 v2.2.0 2026-07-15) requires Go >= 1.26.2 and module path github.com/dolthub/driver/v2; API/DSN unchanged; 1.x stores open under 2.x; 2.x writes not readable by 1.x in all cases; per-root journal Sync unchanged in v2.3.1 source; embedded driver does not inherit server auto-GC; archive-index memory ~350MB-1GB not configurable via embedded Config; use late v2.x graph (PR 11058 branch-control fix after v2.0.0)."
      evidence_ref: "docs/evidence/dolt-v2-research-2026-08-24.md (promoted from agent://DoltTwoResearch, 18 primary sources)"
    - claim: "Guest replay per-event apply costs 3.2-6.5s at the 7-9 GiB workspace on the 4096 MiB guest (0.2-0.3 ev/s; recovery completed via the B14 host drive). Host-side same-binary path runs 27-82 ev/s and completed in ~30 min."
      evidence_ref: "docs/evidence/recovery-replay-guest-io-ceiling-assessment-2026-08-24.md; docs/evidence/recovery-complete-2026-08-24.md"
    - claim: "The other active computer (candidate-fleet-d03dacaa7404b1e4412b2e6f) loops ~3s cadence with 'invalid computer event transition: invalid genesis' on trajectory mint/agent persist (observed >=7h 2026-08-24; still looping at observation)."
      evidence_ref: "docs/evidence/fleet-d03dacaa-invalid-genesis-loop-2026-08-24.md; node-b journal 2026-08-24"
    - claim: "Active-guests Dolt GC is off (RUNTIME_DOLT_GC_DISABLED=1 in the guest unit + 5 GiB size guard in MaybeRunDoltGC); recovered computer health shows persistent_disk warning: true (used 11.6 GiB / 33.5 GiB, above the 8 GiB default cap); no guest-side reclaim path exists for large stores."
      evidence_ref: "nix/autoputer-vm.nix; internal/store/dolt_maintenance.go; guest /health 2026-08-24"
    - claim: "Yaegi private-Go actor kernel definition (choir-private-go-actor-kernel-2026-08-12.md) pins no Go/yaegi version; yaegi v0.16.1 requires go 1.21; the Dolt bump's Go >= 1.26.2 requirement is a strict superset (unblocks, not conflicts)."
      evidence_ref: "docs/definitions/choir-private-go-actor-kernel-2026-08-12.md; proxy.golang.org yaegi v0.16.1.mod"

finish:
  deliver: "The durable substrate is on a current, supported Dolt graph with measured write behavior and a rehearsed one-way-format migration; the live fleet genesis-loop is fixed; active guests have a decided GC/reclaim policy; and the guest replay per-event-cost ceiling is resolved by evidence (the v2 measurement) or by the appender batch fix — so the Tracks K/F/M/Assurance overhauls start from measured ground."
  artifact:
    - "PF-1 candidate: repo on Go >= 1.26.2 (Nix flake + guest image + CI verified) with github.com/dolthub/driver/v2 v2.2.0 + the matching graph as resolved by go mod tidy and then pinned + recorded (dolt/go v0.40.5-0.20260715172757-a6690826d767, go-mysql-server v0.20.1-0.20260713210757-6d01d00bbbf3, vitess v0.0.0-20260624214226-81d034e0fde8, go-icu-regex v0.0.0-20260610153742-72563bc7ca83, gozstd unchanged; PR 11058 included by using the late graph); all driver imports migrated to /v2; DSN/root+per-database connector semantics unchanged; embedded smoke suite green (root DB, per-db connect, SHOW DATABASES, CREATE DATABASE, multi-statements, client-found-rows, DOLT_GC close/reopen) on snapshot disks only; Nix-built guest image verified (ICU/cgo/zstd). NO live-store v2 touch."
    - "PF-2: like-for-like A2-style measurement published from frozen pre-state clones: identical Firecracker image digest (kernel/rootfs, 2 vCPU, 4096 MiB, same disk backend + host-load policy), one clone per runtime (retained v1 binary/package vs the v2 candidate package), the same event band (105k-132k via checkpoint-restore to ~105k + a bounded sample, per the defined sample protocol), per-event apply p50/p95 + checkpoint latency, RSS, GC pauses/OOM, archive (.darc) behavior from an explicit archive-producing act (CALL DOLT_GC on a second clone) + archive-index memory, commit-root/sync + fsync trace, disk growth. Verdict (owner-adjudicated): accept v2 / repair-re-measure / reject."
    - "PF-1 flip (only after the PF-2 verdict and the one-way rollback rehearsal): pushed source + CI + deploy; the v2 runtime package and guest image deployed with an isolated canary first; the canary proves the per-computer snapshot-fence protocol; then the active computers refresh. Downgrade proof: the untouched pre-flip snapshot reads under the retained pre-bump v1 binary/package."
    - "PF-3: d03dacaa invalid-genesis loop root-caused (investigation started in Reconcile; receipt precedes any fix code — the existing receipt stands) and fixed; the fix lands in the same guest-image rebuild as the flip (if the flip gate has passed) or in an earlier isolated image (if bounded and independent); the live computer shows zero mint-fail errors over >= 1h post-landing; failed-run churn cleanup touches store rows only, never the event tape; the computer stays active/healthy throughout."
    - "PF-4: active-guest Dolt GC/reclaim policy decided and implemented (bounded-guest GC, host-side GC including .darc under v2, or an explicitly accepted warning + document); no guest OOM from GC remains possible; the standing persistent_disk warning disposition recorded; the policy re-validated under v2's GC semantics."
    - "PF-5 (gated by PF-2 and owner-adjudicated): if the v2 verdict is accept but the per-event p95 at the band exceeds the skip-gate, implement the appender per-page batching (option D; option C per-event nonfsync evaluated first as the lower-risk alternative per the assessment) and re-measure to the done-line; the ceiling receipt names the resolved outcome, or the named escalation (option B guest machine 8-12 GiB) is taken per the pre-authorized stop/escalate."
  acceptance:
    - action: "PF-1 candidate: a snapshot-only open of the retained store under the v2 binary shows the same databases/schema/rows/commits/branch behavior; the smoke suite passes; the snapshot remains readable by the retained v1 binary (downgrade-proof receipt)."
      proves: "The store open/DSN/connector path survives the v2 graph with no behavioral drift; rollback proof exists before any flip."
      evidence_class: snapshot + smoke + downgrade receipts (no live-store write)
    - action: "PF-2: the measurement receipt publishes v1/v2 tables from identical frozen pre-state clones, the archive/OOM/RSS results, and an owner-adjudicated verdict (accept / repair-re-measure / reject)."
      proves: "The upgrade's write-path and memory impact on the 4 GiB guest is measured, not assumed; the archive memory risk is quantified."
      evidence_class: measurement receipt (disposable clones; live stores untouched)
    - action: "PF-1 flip: canary first (isolated computer or disposable disk), then the live refresh; per-computer snapshot-fence receipts; downgrade-proof receipt in the required set; deployed build identity recorded."
      proves: "The one-way format flip is rehearsed and gated; a failed flip can be restored from the pre-flip snapshot under v1."
      evidence_class: deployed canary copy + per-computer flip receipts + downgrade receipt
    - action: "PF-3: post-landing journal shows zero invalid-genesis errors for >= 1h; the computer remains active/healthy; no tape mutation (head unchanged); problem-documentation-first preserved."
      proves: "The live fleet loop is gone without a second semantic writer or tape mutation."
      evidence_class: deployed receipt + monitoring window + head-unchanged check
    - action: "PF-4: the GC policy is implemented; a guest GC attempt at a large store neither OOMs nor stalls the runtime (guest-path test); the warning disposition recorded; .darc GC coordinated under v2 if archives are used."
      proves: "No guest can be OOM-bricked by Dolt GC; the reclaim story for active computers is explicit and v2-aware."
      evidence_class: deployed + guest-path test receipts
    - action: "PF-5 (if gated): p95 at the band <= the done-line (150ms; the band's full guest-native replay then completes within ~2 resume quanta); otherwise the named escalation (option B) executes per the pre-authorized stop/escalate."
      proves: "The recovery follow-up ('guest replay I/O ceiling') is closed by implementation, by measurement, or by the named escalation."
      evidence_class: measurement receipt + implemented-fix receipt (if gated)
  rollback: "Git revert mission commits via origin/main + CI; canonical events are never rewound. Dolt one-way-format protocol (per computer): before the first v2 write on any store, (1) fence/quiesce the guest, (2) take and identify a crash-consistent reflink snapshot with digest + head + epoch + schema/count witness, (3) prove the untouched snapshot reads under the retained pre-bump v1 binary/package, (4) run the flip; failure handling is forward repair retaining v2 OR snapshot restore plus reconstruction through the existing canonical event authority under v1; snapshots are retained until the next accepted major boundary, deletion requires owner authorization; a staging deployment flips live stores only after one disposable/canary computer proves the procedure. Downgrade tests run only on untouched snapshots; never on a store that saw v2 writes."
  landing:
    required: true
    environment: staging
    required_receipts: [pf1a_smoke, downgrade_proof, measurement, owner_verdict, pf2_independent_review, canary_rehearsal, per_computer_flip, d03_fix, gc_policy, ceiling_verdict, independent_panel_review]
    per_slice_landing: "EACH red slice (PF-1a candidate is evidence-only; the flip PF-1b, PF-3b, PF-5-if-gated, PF-4 are behavior changes) carries its own pushed SHA -> CI -> deploy identity -> scoped acceptance; the landing requirements are tracked per slice, not as one flat receipt bag."
not_done_when:
  - any v2 binary opens a live computer's store before the PF-2 verdict and the one-way rollback rehearsal
  - a live staff flip happens without the canary proof + per-computer snapshot-fence receipt
  - the event tape or canonical head (132,539/acc54c39...) is mutated by this mission (the mission performs no tape mutation; the computers' own normal runtime appends during the mission are not mission mutation)
  - the d03dacaa computer is disrupted by its fix rollout (guest image + fence intact; head unchanged)
  - the Dolt upgrade is done without the Go >= 1.26.2 toolchain verification through the Nix guest build + CI and the graph pinned + recorded (incl. PR 11058 lineage)
  - the ceiling verdict is reached without the like-for-like 4 GiB-guest measurement from frozen pre-state clones (host-only numbers do not count)
  - the effects definition's mode state changes (effects remain OFF)
  - the pre-flight is executed without the standing-questions conformance answer set (Reconcile) and the registry entry recorded
boundaries:
  mutation_class: red
  authority_sources: [owner direction 2026-08-24, docs/standing-questions.md, docs/evidence/dolt-v2-research-2026-08-24.md, docs/evidence/recovery-replay-guest-io-ceiling-assessment-2026-08-24.md, docs/evidence/fleet-d03dacaa-invalid-genesis-loop-2026-08-24.md, docs/evidence/recovery-complete-2026-08-24.md, AGENTS.md, docs/choir-doctrine.md]
  must_preserve:
    - Single guest ComputerEventAppender remains the sole semantic event writer; this mission performs no tape mutation, while the live computers' own normal runtime appends continue to be legitimate.
    - The canonical event chain at head 132,539/acc54c39... and computer 0333528's recovered state (active, epoch 715) are preserved (its normal runtime activity is not frozen by this mission).
    - DSN/root+per-database connector semantics (commitname/commitemail/database/multistatements/clientfoundrows) stay exactly as today.
    - d03dacaa stays active/healthy through its fix, with the fence intact and head unchanged.
    - Pre-upgrade snapshots, the retained v1 binary/package, and the 103 authentic tape events 132,437-132,539 are never overwritten or "cleaned".
  protected_surfaces: [canonical computer event chain, guest ComputerEventAppender, Dolt store/workspace, vmctl lifecycle, live d03dacaa computer, platform-artifacts store]
  not_goals:
    - The overhauls themselves: Tracks K (key escrow), F (file-CAS), M (mail), and Assurance (docs/definitions/choir-durable-substrate-overhauls-2026-08-23.md) — they begin after this pre-flight's receipts.
    - The effects mission continuation (CoSuper authoring) — separate.
    - The guest machine-size bump (4096 -> 8-12 GiB) — pre-authorized ONLY as the named stop/escalate of PF-5, not a silent scope add.
    - No other live-store mutation: d03's store is the only live store the fix legitimately touches, under its fence protocol.
  completion_evidence_floor: [deployed PF-1..PF-5 receipts, downgrade-proof receipt, independent panel review of the PF-2 verdict and the PF-5 scope decision]

measures:
  - signal: "per-event apply p95 at the 105k-132k band (sample protocol: checkpoint-restore the clone to >=105k, then apply a defined 200-event sample; p95 reported over the sample; same image/runtime/disk for v1 and v2)"
    baseline: "3.2-6.5s (v0.40, 4 GiB guest, 7-9 GiB workspace)"
    direction: "skip-gate: <= 1s p95 (decides whether PF-5 is required); done-line: <= 150ms p95 (the band's guest-native replay then fits ~2 resume quanta)"
    decides: "PF-5 scope + closure; the owner-adjudicated v2 accept/repair/reject"
    limits: "measures the guest replay write path only; does not prove store correctness (smoke suite covers that); the sample p95 is a bounded estimate, not a full-band scan"
  - signal: "guest RSS + OOM during the v2 replay measurement and over an archive-producing act (CALL DOLT_GC on a second clone)"
    baseline: "no archives today (noms chunk + journal); RSS peak 1.9 GiB on the host-side measurement"
    direction: "no guest OOM; archive-index RSS delta bounded so the 4 GiB budget holds under replay + runtime" 
    decides: "whether archives are safe on the guest or must be disabled (a config/DSN affordance if one exists; otherwise the verdict records it)"
    limits: "a memory-sample proxy; the guest's total budget is the real constraint"
  - signal: "d03dacaa invalid-genesis error cadence"
    baseline: "~3s cadence (>=7h)"
    direction: "0 over >= 1h post-fix; head unchanged"
    decides: "PF-3 closure"
    limits: "does not prove the fix's generality — that is the root-cause receipt's record"
  - signal: "guest GC OOM count + disk-used trajectory under the PF-4 policy"
    baseline: "the OOM crash-loop (recovery boot #2) and the standing warning (11.6/33.5 GiB)"
    direction: "0 GC-induced guest OOMs; the warning disposition recorded"
    decides: "PF-4 closure"
    limits: "long-run disk trend is not fully observable in the pre-flight window; the policy's bound is the deliverable, not a full-trend proof"
phases:
  - name: Reconcile
    items:
      - "Answer docs/standing-questions.md for this Definition (recorded in the receipt); register the id in the mission registry hygiene surface; reclassify dirty paths if any."
      - "Promote the Dolt research into docs/evidence/dolt-v2-research-2026-08-24.md with the graph + PR 11058 lineage."
      - "Take pre-upgrade snapshots of ALL live computer disks (0333528, d03dacaa, any other active) + identify and retain the pre-bump v1 binary/package + image digest; verify the retained snapshot reads under v1."
      - "Start PF-3 read-only root-cause investigation (concurrent, no repair code) with the escalation gate."
  - name: PF-3a — d03 Root-Cause Investigation
    items:
      - "Read-only causal locus: reducer/genesis validation (internal/computerevent reducer + projection tape), store ancestry state, caller construction, or projection drift; containment if a non-repair knob stops the churn (policy-level, no store code)."
      - "Escalate per the repo dead-end rule if the cause is substrate-level (shared genesis/projection code, no narrow fix): stop, record the structural assessment, owner decision."
  - name: PF-1a — Dolt 2.0 Candidate
    items:
      - "Bump the Nix flake toolchain to Go >= 1.26.2; verify guest image + CI builds (ICU/cgo/zstd); bump ALL go-toolchain surfaces together and verify: root go.mod, cmd/desktop/go.mod (1.25.6 today), the CI matrix (ci.yml pins 1.26.1 today — must be >= 1.26.2), and the Nix flake's go package."
      - "Migrate imports to github.com/dolthub/driver/v2 v2.2.0; let go mod tidy resolve the matching graph; then pin + record the result (incl. the PR 11058 lineage)."
      - "Embedded smoke suite + retained-workspace snapshot open + downgrade-proof (v1 reads the untouched snapshot) — snapshot disks only, NO live-store touch; the candidate build NEVER reaches the staging service pointers or the deploy's active-VM refresh (local nix-build + local runtime package run only) until PF-1b."
      - "Build the candidate guest image + runtime package; freeze the v1 baseline package."
  - name: PF-2 — Like-for-like Measurement
    items:
      - "Freeze the measurement pre-state (digest + head + schema/count witness + quiescence evidence); clone one disk per runtime (v1 package vs v2 candidate package); identical machine shape + telemetry."
      - "Run the band measurement (checkpoint-restore to ~105k + 200-event sample), archive act on a second v2 clone, RSS/OOM/GC/syscall traces."
      - "Publish the measurement receipt; owner-adjudicated verdict: accept v2 / repair-re-measure / reject."
  - name: PF-1b — Flip (gated)
    items:
      - "Canary: deploy the v2 runtime/image to an isolated/disposable computer first; REHEARSE the full per-computer fence -> snapshot -> flip -> restore-from-snapshot-under-v1 cycle on the canary (not just the protocol on paper); record the rehearsal receipt with digests."
      - "Independent PF-2 review BEFORE any live flip: the measurement receipt + verdict bound to frozen digests (pre-state snapshot digest, clone digests, image digest, sample protocol); a panel review of the verdict + the flip boundaries precedes PF-1b; any candidate change after the review requires re-measurement."
      - "Live flip: deploy to 0333528 + d03 (with the PF-3 fix if coalesced) with per-computer snapshot-fence receipts; verify builds + heads."
  - name: PF-5 — Ceiling Resolution (gated)
    items:
      - "The v2 verdict and the ceiling gate are SEPARATE questions; conflating them deadlocks the mission (the research predicts v2 will NOT lower the 3-6s p95, so an absolute p95 bar on the v2 accept would block the flip while PF-5 sits after it). V2 VERDICT (upgrade soundness only — owner-adjudicated, recorded): (i) REJECT if correctness (schema/count/heads/branch behavior on the snapshot) regressed, the measurement OOM'd, or the archive-index memory exceeded the 4 GiB budget — no flip, PF-1b does not run, the pre-flight re-baselines; (ii) repair-re-measure if the v2 measurement shows a RELATIVE write-path regression vs the v1 baseline beyond the comparability protocol (fresh clones, same band, p95_v2/p95_v1 > 1.5x) — the ABSOLUTE p95 is NOT part of the v2 verdict; (iii) accept if sound — flip authorized. CEILING GATE (independent, always evaluated): p95 <= 150ms done-line closes the ceiling by evidence; p95 > 150ms REQUIRES PF-5 wherever it lands (the batching applies to either runtime graph); the owner ratifies either outcome with the measured full-band recovery-time implication named in the receipt."
      - "If the PF-2 verdict passes but p95 > 150ms: implement the appender per-page batching (D; C per-event nonfsync evaluated as the lower-risk first option per the assessment), freeze + panel-review the implementation, re-measure to the done-line."
      - "If the done-line is unreachable: execute the pre-authorized named escalation (option B, 8-12 GiB guest) — owner-visible, receipted — or stop + escalate."
  - name: PF-3b — d03 Fix (if not independently landed)
    items:
      - "Problem receipt first (the existing receipt stands); fix + landing in the same image as the flip (or an earlier isolated image if bounded and independent; internal/store mutations serialized with PF-1)."
      - "Land + monitor zero-error window + head-unchanged check."
  - name: PF-4 — Active-guest GC Policy
    items:
      - "Decide bounded-guest GC vs host-side GC (incl. .darc under v2) vs accepted warning; implement; re-validate under v2's GC semantics (no server auto-GC inheritance); guest-path OOM-free test."
      - "Record the persistent_disk warning disposition."
  - name: Close
    items:
      - "Independent panel review of the PF-2 verdict + PF-5 scope decision (evidence receipt with adjudicated outcome); final receipt + landing loop summary; overhauls definition now-card sequencing update."
now:
  status: working
  slice: "Definition v4 after panel rounds 1-3 (candidate/flip split; PF-3 forward; measurement + rollback rigor; per-slice landing; toolchain surfaces; v2-verdict vs ceiling-gate separation fixing the round-3 deadlock; PF-1a never touches staging pointers). Awaiting round-4 confirmation before execution."
  question: none
  reconciliation:
    observed_at: "2026-08-24T06:40:00Z"
    source_ref: "main@1fad1da0"
    deploy_identity: "staging https://choir.news; recovery complete at f7b3ccd2"
    authority_identities: [docs/choir-doctrine.md, docs/computer-ontology.md, AGENTS.md, docs/standing-questions.md]
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: "clean main; prunable agent worktrees preserved"
    status: reconciled
  candidate:
    id: none
    state: none
  decision:
    selected: "Scope the loose ends (Dolt 2.0, d03 genesis loop, active-guest GC policy, replay ceiling) into one pre-flight mission before the overhauls; the Dolt update is executed as candidate-first (no live-store v2 touch before the measurement gate + one-way rehearsal); PF-3 investigation begins in Reconcile; the ceiling fix (fix-D) is gated by the PF-2 measurement and owner-adjudicated; Go >= 1.26.2 is a strict superset of the yaegi kernel's go 1.21 requirement."
    kind: architecture
    status: settled
    source: owner direction 2026-08-24
    evidence_ref: "docs/evidence/dolt-v2-research-2026-08-24.md; docs/evidence/recovery-replay-guest-io-ceiling-assessment-2026-08-24.md"
    owner_ratification_ref: "owner confirmed 2026-08-24 (pre-flight before overhauls); panel round-1 amendments folded in"
    recorded_at: "2026-08-24T06:40:00Z"
    consequence: "The overhauls definition (choir-durable-substrate-overhauls-2026-08-23.md) starts its now-card sequencing from this pre-flight's completion."
  evidence_refs:
    - "docs/evidence/dolt-v2-research-2026-08-24.md"
    - "docs/evidence/recovery-replay-guest-io-ceiling-assessment-2026-08-24.md"
    - "docs/evidence/fleet-d03dacaa-invalid-genesis-loop-2026-08-24.md"
    - "docs/evidence/recovery-complete-2026-08-24.md"
  blocker_or_risk: "Dolt v2 one-way format (controlled by snapshot-fence + canary + downgrade proof); archive-index memory in the 4 GiB guest (quantified in PF-2); d03 fix touches a LIVE computer (fence + image rollout; escalation if substrate-level); PF-3's shared genesis code also backs 0333528's runtime (cross-contamination risk — the fix's regression suite covers both)."
  next_action: "Run agentic-consensus round 2 on this amended draft (convergent, default panel), adjudicate, iterate until panel approval; then bind the approved definition, promote the research receipt, and begin Reconcile (standing-questions answers, snapshots, PF-3 investigation)."

receipts:
  - id: define-preflight-round1
    boundary: define
    commit_or_artifact: "(pending round-2 approval)"
    proof_refs: [docs/evidence/recovery-replay-guest-io-ceiling-assessment-2026-08-24.md, docs/evidence/fleet-d03dacaa-invalid-genesis-loop-2026-08-24.md]
    rollback_ref: revert docs commit
    disposition: "Pre-flight definition drafted and amended after panel round 1 (6 repair / 4 approve-with-caveats adjudicated; sequence candidate-first, PF-3 forward, measurement + rollback rigor)"
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
