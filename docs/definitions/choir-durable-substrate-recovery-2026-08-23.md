---
definition_version: 2
definition_id: choir-durable-substrate-recovery-2026-08-23
execution_mode: mission_orchestrator

start:
  captured_at: "2026-08-23T23:00:00Z"
  source:
    canonical_ref: "main@7109b070"
    deploy_identity: "staging https://choir.news; retained computer computer-03335285269bdba4f94377e56879f9e6 (VM candidate-fleet-e15cb89f25d963c220319b7b) state stopped, epoch file 707, no live firecracker process (stale pid); canonical head sequence 132,436 canonical_event_head 8df7efbba8617b37cc17cab9695fe62e2870b36b1ee6ec8fcbfeef8467927777 reducer v1 frozen since 2026-08-22; DELTA 2026-08-24 (A2/A3 experiment run 3): the post-replay runtime auth-reporting appended 2 key_revoked + 101 projection_batch_recorded events; canonical head is now sequence 132,539, canonical_event_head acc54c39ee05d89af13223e3b8cca195e04d7dfc8f137ce1bb27b96f657b7201. Chain integrity intact; the experiment consumed no semantic event and the pre-delta head 132,436/8df7efbba... was reached and verified before the appends. "
  worktree_inventory:
    status: reconciled
    evidence_ref: "git status 2026-08-23: worktree at main@7109b070 with M docs/ACTIVE.md + M docs/definitions/choir-durable-substrate-recovery-2026-08-23.md (this Define-boundary update) and M docs/evidence/recovery-consensus-review-2026-08-23.md; skills/agentic-consensus/* committed at 7109b070; .agentic-consensus/ gitignored"
    preservation_rule: "Preserve every non-primary worktree and all unrelated WIP; this Definition owns recovery of 0333528. The offline ProjectionBase substrate is a SEPARATE successor Definition (owner-ratified 2026-08-23); design note: docs/designs/choir-durable-substrate-2026-08-23.md."
  worktrees:
    - path: /Users/wiz/go-choir
      status: dirty
      class: goal_candidate
      owner: owner-and-session
      touch: goal_owned
      recovery: commit the docs at the first Define boundary
  candidates:
    - id: none
  observed_artifact:
    - claim: "Retained computer 0333528 is stopped. Original observation (2026-08-23T02:00:00Z) recorded epoch 361; dated correction at 2026-08-23T22:30:00Z records epoch 707 after later boot attempts, no live firecracker (stale pid). Initial break: capsule memory exhaustion + assignment supersession loop, then boot timeouts because autoputer replays the full event chain before opening :8085."
      evidence_ref: "docs/evidence/effects-red-capsule-memory-budget-exhaustion-2026-08-21.md; docs/evidence/effects-red-assignment-supersession-loop-2026-08-21.md; docs/evidence/effects-red-guest-dependent-restore-2026-08-22.md"
    - claim: "Canonical event chain intact at head seq 132,436; Dolt computer_event_heads: canonical_event_head 8df7efbba8617b37cc17cab9695fe62e2870b36b1ee6ec8fcbfeef8467927777, reducer_version 1, updated 2026-08-22. DELTA 2026-08-24 (A2/A3 experiment run 3): the post-replay runtime auth-reporting appended 2 key_revoked + 101 projection_batch_recorded events; canonical head is now sequence 132,539, canonical_event_head acc54c39ee05d89af13223e3b8cca195e04d7dfc8f137ce1bb27b96f657b7201. Chain integrity intact; the experiment consumed no semantic event and the pre-delta head 132,436/8df7efbba... was reached and verified before the appends. All projection batch bodies present (~756 MiB in computer-event-payload/)."
      evidence_ref: "docs/evidence/effects-red-recovery-projections-present-2026-08-23.md"
    - claim: "Privacy DEK (32-byte XChaCha20) at /mnt/persistent/choir-credentials/privacy-key in the guest disk; debugfs trusted-guest read is the sole documented exception to 'host never parses guest ext4'. Extracted + verified: computer_id matches, DEK hex 1a55efbbaf764fbc0ce17c7ccd537cdb1fae432c75bff82dc8c7f7870c704593."
      evidence_ref: "internal/computerevent/privacy.go; docs/evidence/effects-red-recovery-trusted-guest-copy-authority-2026-08-22.md"
    - claim: "Predecessor choir-host-orchestrated-recovery-2026-08-22.md is superseded by this Definition."
      evidence_ref: "docs/definitions/choir-host-orchestrated-recovery-2026-08-22.md"
    - claim: "VERIFIED boot root cause: a replay that exceeds the boot readiness window cannot converge. vmmanager/manager.go:265 BootReadyTimeout (default 20s) hard-kills firecracker on expiry (:769-776); autoputer/run.go:222 runs full-tape appender.Reconstruct before the HTTP server opens; store/computer_events.go:182 + appender.go:677 defer the Dolt checkpoint to one commit after the whole chain; appender.go:588 resumes only from localHead.Sequence. Epoch 361 -> 707 with zero progress is the predicted signature. NIX STAGING MAY OVERRIDE THE 20s DEFAULT (VM_BOOT_READY_TIMEOUT) — the LIVE value is Phase-0 question and is NOT yet observed."
      evidence_ref: "internal/vmmanager/manager.go:247-265,769-776; internal/autoputer/run.go:155,222; internal/store/computer_events.go:145-182; internal/computerevent/appender.go:580-600,668-678; nix/node-b.nix:544"
    - claim: "The offline ProjectionBase rebuild (6185910a) is NOT the recovery blocker and was mis-targeted as primary: DiskEventSource linear-reduce of the flat computer-event/ dir fails at seq 2 (multiple key_revoked at seq 2), production replay reads the canonical chain from corpusd Dolt computer_event_append_receipts (event_replay.go:14-27,92-101), the rebuilder fabricates TransitionInput for every kind, performs zero real receipt verification, and self-compares the content witness (rebuilder.go:134). Owner-ratified route (2026-08-23): boot/replay-contract fix FIRST; ProjectionBase substrate = separate successor Definition."
      evidence_ref: "internal/projectionbase/source.go; internal/projectionbase/rebuilder.go; internal/platform/event_replay.go; node-b /var/lib/go-choir/rebuild-0333528.log"
    - claim: "A1 ANSWERED (2026-08-23): the deployed guest image PREDATES the RUNTIME_STORE_PATH wiring by 11 days. Wiring added 2026-04-20 (git blame d4a5f160 in nix/sandbox-vm.nix:727); deployed guest image is nixos-system-go-choir-autoputer-26.05.20260409.4c1018d (build date 2026-04-09, still the image referenced by fc-config.json at the last boot epoch 707). The guest therefore runs the store at DefaultStorePath /tmp/go-choir-m3/runtime.db (provideriface/config.go:14,105), so ALL replay progress died with every VM — the empty state.texture repo and zero progress across 361->707 are fully explained, and no checkpoint code can help until the guest is rebuilt with the wiring. The guest image is also ~4.5 months stale vs repo HEAD (many refactors since) — the Phase-1 guest rebuild replaces it wholesale; the canonical chain and persistent files are unaffected by guest-version drift, and store runtime-schema bootstrap handles migrations."
      evidence_ref: "git blame nix/autoputer-vm.nix:727 (d4a5f160, 2026-04-20); fc-config.json init path (26.05.20260409.4c1018d); internal/provideriface/config.go:14,105"
    - claim: "OWNER DECISIONS 2026-08-23 (recorded for Phase-1): B5 ship guest+host together (guest image rebuild REQUIRED per A1); B6 host-side gate (vmctl refuses route CAS + assignment until head+witness verified); B7 save progress at 30m boundary — treat 30m as one resume quantum (checkpoint, exit cleanly, boot again); B8 discard half-finished/prepared records on replay and re-apply from the log, never CAS on the replay path; B9 finish = head >= 132,539 (revised 2026-08-24 per the A2/A3 delta) with intact witness (lifecycle appends allowed after replay); B10 checkpoint cadence + stall timeout per recommendation (finalized after A3 measurement: cadence ~60s/200-500 events, stall 120-300s); B11 Dolt GC OFF during recovery; B12 AGENTIC/manual driver (agent drives ~20 x 30m sessions with receipts); B13 NO protective backup — on non-convergence, ABANDON this recovery and start anew (fresh computer, owner ratifies at that point); canonical chain is never rewound either way; the 'no data lost' streak is maintained platform-wide but explicitly relaxed for 0333528 by owner choice."
      evidence_ref: "owner direction 2026-08-23 (walkthrough decisions B5-B13)"

finish:
  deliver: "The retained stopped computer computer-03335285269bdba4f94377e56879f9e6 is recovered to canonical head 132,539 (canonical_event_head acc54c39ee05d89af13223e3b8cca195e04d7dfc8f137ce1bb27b96f657b7201) via the fixed boot/replay contract: a replay is never hard-killed while making progress (liveness = progress, readiness = head+witness match), replay state is durable/resumable per Phase-0 findings, and the computer boots to a quiesced active state on staging with no rewind and no second semantic writer."
  artifact: "A deployed staging trajectory showing 0333528 stopped -> Phase-0 probe results -> boot/replay-contract fix (liveness/readiness split; stall-gated kill; resumable durability; RecoverPrepared replay-safe) -> BootVM of the existing data.img -> resumable replay converges to head acc54c39... @ 132,539 -> final head+witness match -> route CAS under fencing token -> computer active in quiesced state, with no-rewind refusal receipts, quarantine preserved, and fleet healthy-boot regression green."
  acceptance:
    - action: "Recover 0333528 via the product boot path (B9 = head >= 132,539 with intact witness): BootVM a REBUILT guest image (B5) on the EXISTING data.img (ordinary boot; never recover_current wipe), resumable replay (30m resume quanta, B7) converges to head acc54c39... @ seq 132,539 (lifecycle appends after replay allowed; final head >= 132,539 with intact witness), final head+witness match, host-side gate (B6) then route CAS under fencing token -> active in quiesced state. Show ReplayCompleteness equivalent and effective ComputerVersion/frontend serving-join before route CAS."
      proves: "The down computer is recovered to current canonical head without memory exhaustion, event rewinds, or a missing-snapshot dependency."
      evidence_class: deployed proof on staging 0333528, stopped -> recovered -> active
    - action: "Prove liveness != readiness: guest serves a replay-progress signal (e.g. 503 ReplayInProgress seq=N) during Reconstruct; product /health is 200 only after head+witness match; waitForGuestReady does not mark the computer Running on 200-during-replay; a stalled replay (no sequence advance for N seconds) is still hard-killed; healthy small-chain computers still boot normally (fleet regression)."
      proves: "The boot/replay contract fix repairs the actual defect without weakening liveness or readiness gates."
      evidence_class: deployed boot observation + fleet regression + no-progress kill test
    - action: "Prove resumable durability per Phase-0 findings (if required): a replay that is killed mid-way resumes from localHead.Sequence on the next boot (e.g. seq advances 500 -> 1000 across an intentional SIGKILL), with no double-apply, no orphan prepared rows, and end-of-replay platform-vs-local head equality still enforced. Record the boot_contract_fix_proven receipt from these logs."
      proves: "Replay is crash-safe and resumable; progress is never silently lost."
      evidence_class: SIGKILL-resume demo + idempotence tests + receipt logs
    - action: "Verify no rewind and structural single-appender: requests bearing checkpoint_digest/authorization_ref/mode rejected 400/409; capability is read-events only; host recovery path never constructs a semantic CASRequest; canonical event tape has zero host-appended semantic events."
      proves: "Recovery cannot rewind history and introduces no second semantic writer."
      evidence_class: refusal tests + chain inspection
    - action: "Verify quarantine preservation & structural pruning disable: pruneCompletedQuarantines disabled during active recovery; data.img quarantine preserved and not pruned."
      proves: "Forensic and rollback evidence survives the recovery run."
      evidence_class: quarantine inspection
    - action: "Verify quiesced state: after route CAS, no immediate CoSuper cancellation/supersession churn for a bounded observation window (assignment fenced until head+witness+route proven)."
      proves: "Recovery does not re-trigger the original assignment supersession loop."
      evidence_class: bounded-window churn observation

rollback: "Revert mission commits; a failed recovery preserves quarantine and leaves route safely unavailable. Canonical events are never rewound."
landing:
  required: true
  environment: staging
  required_receipts: [pushed_commit, ci, deploy, staging_build_identity, recover_0333528_to_head, boot_contract_fix_proven, no_rewind, quarantine_preserved]
not_done_when:
  - recover_current accepts a checkpoint digest/authorization_ref, or any historical rewind is reachable
  - host parses or writes guest ext4 directly (trusted-guest read of privacy key via debugfs is the sole documented exception)
  - route or assignment is published before ReplayCompleteness + effective ComputerVersion/frontend verification
  - the recovery re-enters recover_current (blank-seed wipe) for the retained data.img; the existing data.img must be the booted volume
  - a replay in progress is killed because wall-clock exceeded while sequence was still advancing (liveness != readiness is violated)
  - a stalled replay is allowed to occupy the VM indefinitely (no-progress kill is absent)
  - 99949fe2 is exposed as a recoverable target before the scheduling Definition reaches E2
  - a ProjectionBase is published or rebuilt as part of this recovery (deferred to successor Definition)
  - quarantine is deleted or pruned while recovery is unfinished
  - computer 0333528 is SQL-emptied, replaced, or its canonical chain rewound
  - the computer boots into an active supersession loop rather than a quiesced state
  - the recovery claims completion while the guest never reaches head acc54c39... @ seq 132,539

boundaries:
  mutation_class: red
  authority_sources: [owner direction 2026-08-23 (boot-contract-fix-first route), docs/evidence/recovery-consensus-review-2026-08-23.md, docs/designs/choir-durable-substrate-2026-08-23.md, docs/evidence/effects-red-recovery-no-differential-base-2026-08-23.md, docs/evidence/effects-red-recovery-projections-present-2026-08-23.md, docs/choir-doctrine.md, docs/computer-ontology.md, AGENTS.md]
  must_preserve:
    - Single guest ComputerEventAppender remains the sole semantic event writer; host recovery path never constructs a semantic CASRequest.
    - Canonical events never rewound; recovery targets final head acc54c39... @ 132,539.
    - Checkpoint 99949fe2 untouched until scheduling Definition reaches E2.
    - Effects remain OFF; assignment/capsule admission fenced until head+witness+route CAS (quiesce).
    - Quarantine never deleted or pruned during active recovery; maxRetained pruning bypassed during recovery.
    - The fix only changes checkpoint cadence + readiness semantics; per-event Reduce + sameHead + VerifyEventHeadReceipt + end-of-replay platform-vs-local head equality remain intact.
    - Replay checkpointing idempotent and durable; killed boot resumes from localHead.Sequence, never restarts at 0, never double-applies, never leaves an orphan prepared row that trips RecoverPrepared.
    - Phase-0 probes are read-only (debugfs read + Dolt read-only on a copied working set + live-process env reads); no boot, no mutation.
  not_goals:
    - Offline ProjectionBase rebuild substrate (DiskEventSource fix, receipt verification, TransitionInput derivation, hydrate + delta=0 boot) is OWNED BY A SEPARATE SUCESSOR Definition; NOT required for this recovery.
    - Substrate overhauls (Track K keys, Track F file commit protocol, Track M mail, Assurance/Scale): choir-durable-substrate-overhauls-2026-08-23.md.
    - Autonomy / candidate proof: choir-scheduling-and-candidate-proof-2026-08-21.md.
  protected_surfaces: [canonical computer event chain, guest ComputerEventAppender, checkpoint/route projection, platform-artifacts blob store, privacy-key file, corpusd HeadCAS, recover_current, vmctl lifecycle, boot readiness contract, guest assignment/capsule admission]
  completion_evidence_floor: [deployed proof, no-rewind refusal, boot/replay-contract fix proven (liveness/readiness split + stall-kill + resumable durability), fleet healthy-boot regression, quarantine preservation]

measures:
  - name: recover_0333528_to_head
    kind: gate
    baseline: "stopped, canonical head 8df7efbba... @ seq 132,436 (pre-delta state recorded 2026-08-22)"
    desired: "0333528 active at head 132,539 via fixed boot/replay contract + route CAS in quiesced state"
    decision_use: gates the recovery finish claim
    cannot_prove: O(delta) recovery on future writes (ProjectionBase successor Definition / Track F overhaul)
  - name: boot_contract_fix_proven
    kind: gate
    baseline: "replay hard-killed on wall-clock expiry; deferred single DOLT_COMMIT; restart-from-zero"
    desired: "a replay is never killed while sequence is advancing; a stalled replay is still killed; replay is resumable/durable per Phase-0; fleet healthy-boot still green"
    decision_use: gates whether the boot/replay-contract defect is repaired at the substrate
    cannot_prove: the computer reaches active (recover_0333528_to_head)
  - name: phase0_probe_complete
    kind: gate
    baseline: "probe questions unanswered (store path, persistence across SIGKILL, live timeout, RAM, boot path, leftover prepared rows)"
    desired: "Phase-0 read-only probes answered and recorded; decision table applied (resumable vs fresh-workspace; DOLT_COMMIT needed or not; timeout-override live value)"
    decision_use: gates Phase-1 implementation direction
    cannot_prove: the recovery completes

phases:
  - name: Diagnose State Persistence
    status: pending
    items:
      - "Phase-0 READ-ONLY probes on node-b (no boot, no mutation): (a) guest store path — nix/autoputer-vm.nix RUNTIME_STORE_PATH (= /mnt/persistent/state) vs provideriface/config.go default (/tmp/...); (b) data.img state/ layout via debugfs: does state/.dolt exist, what does computer_event_projection_heads hold (sequence/canonical_event_head), any leftover computer_event_index status='prepared' rows; (c) live VM_BOOT_READY_TIMEOUT of the running vmctl process (read /proc/<pid>/environ or service env, not the default constant); (d) guest MachineMemSizeMib for this VM (fc-config.json / manager config); (e) boot path: recovery journals (rec-*.journal phase/operation) vs ordinary BootVM — confirm 707-epoch boots were cold-recover wipes or ordinary; (f) free space in the VM volume."
      - "Record each answer plus the decision table: store persists -> resumable replay sufficient; store wiped/non-persistent -> fresh-workspace/single-boot path; working set survives SIGKILL (SQL head advances across boots) -> DOLT_COMMIT optional; working set lost -> periodic DOLT_COMMIT required; live timeout override -> which clock is the real boundary."
  - name: Fix Boot & Replay Contract
    status: pending
    items:
      - "Split liveness from readiness: guest serves a replay-progress signal (503 ReplayInProgress seq=N) during Reconstruct before product /health is meaningful; /health returns 200 only after head+witness match; waitForGuestReady must not mark Running on progress alone; kill is gated on replay STALL (no sequence advance for N seconds), not wall-clock."
      - "Make RecoverPrepared replay-safe on the replay path: discard (or locally finalize after receipt verify) prepared rows already <= localHead on the canonical tape, rather than ErrNeedsProjectionRepair on a mid-replay kill."
      - "Add resumable durability ONLY if Phase-0 shows the SQL working set is lost on SIGKILL (or a single pass cannot finish in the budget): periodic commitDoltCheckpoint with durable fsync at a cadence measured from events/sec (e.g. every 60s or 5-10k events, never per-event); advance after only after a successful durable commit; keep per-event Reduce + sameHead + VerifyEventHeadReceipt + end-of-replay equality."
      - "If Phase-0 shows that a single uninterrupted replay fits the guest budget and the working set survives kills: ship ONLY the liveness/readiness split and stall-kill, defer periodic DOLT_COMMIT."
      - "Tests (disk-backed, real kill): SIGKILL mid-replay resumes from localHead.Sequence; no double-apply; leftover prepared row discarded; stalled replay killed; /health 200 only at canonical head; fleet healthy-boot regression."
  - name: Recover 0333528
    status: pending
    items:
      - "BootVM the EXISTING data.img of 0333528 (ordinary boot; never recover_current). Watch sequence advance across at least one intentional kill. Converge to head acc54c39... @ 132,539."
      - "Verify final head + content witness + ReplayCompleteness + effective ComputerVersion/frontend join, THEN route CAS under fencing token; assignment/capsule admission fenced until then."
      - "Observe quiesced state: no CoSuper cancellation/supersession churn for a bounded window."
      - "Confirm quarantine preserved; confirm no-rewind refusals; confirm no host-appended events."
  - name: Acceptance
    status: pending
    items:
      - "Record deployed acceptance receipts (pushed_commit, ci, deploy, staging_build_identity, recover_0333528_to_head, boot_contract_fix_proven, no_rewind, quarantine_preserved) plus fleet healthy-boot regression and quiesce observation."
      - "Fold receipts into the Define/Implement boundary; close the Definition against completion_evidence_floor."

now:
  status: working
  slice: "Phase-1 IMPLEMENTED and LANDED (commit 63168fb3, CI run 32676579657, push 2026-08-24, deploy receipt target_commit 63168fb3 activated 2026-08-24T00:38Z, guest-image build.json commit 63168fb3, vmctl active with VM_REPLAY_STALL_TIMEOUT=120s): replay is resumable (periodic durable checkpoints plus 30m resume quantum), guest serves 503 ReplayInProgress so the host stall-gates instead of wall-clock-killing, RecoverPrepared is replay-safe (no CAS), lifecycle receipts fence until head+witness, Dolt GC defers to post-replay, and host + guest image were deployed together (B5)."
  question: "RESOLVED by probes 2026-08-23T23:45Z (see observed_artifact): (1) no persisted working set -> fresh-replay-with-durable-checkpoints; (2) periodic DOLT_COMMIT required (nothing persists today); (3) 30m live; (4) 4096 MiB; (5) ordinary boots after rec-2. Remaining open: why /mnt/persistent/state is a 0-byte file rather than a Dolt workspace (deployed guest open/commit path), RecoverPrepared replay semantics, quiesce fence mechanism, checkpoint cadence from measured throughput."
  reconciliation:
    observed_at: "2026-08-24T00:00:00Z"
    source_ref: "main@7109b070"
    deploy_identity: "staging proxy+vmctl ok (health 200); computer 0333528 stopped, epoch file 707, no live firecracker; canonical head 132,436/8df7efbba... pre-delta; post A2/A3 delta head acc54c39... seq 132,539"
    authority_identities: [docs/choir-doctrine.md, docs/computer-ontology.md, AGENTS.md]
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: "working tree owns recovery definition + design + consensus evidence doc; dirty docs categorized goal_owned"
    status: reconciled
  candidate:
    id: none
    state: none
  decision:
    selected: "Owner-ratified route 2026-08-23: boot/replay-contract fix FIRST (liveness/readiness split, stall-gated kill, resumable durability per Phase-0), then recover 0333528 to head 132,436 via BootVM of the existing data.img. Offline ProjectionBase rebuild substrate = separate successor Definition. Consensus (3 rounds, 8 agents) confirmed the route and sharpened Phase-0/Phase-1; details in docs/evidence/recovery-consensus-review-2026-08-23.md."
    kind: architecture
    status: settled
    source: "owner direction 2026-08-23 (ask selection: boot-contract-fix-first)"
    evidence_ref: "docs/evidence/recovery-consensus-review-2026-08-23.md; docs/designs/choir-durable-substrate-2026-08-23.md"
    owner_ratification_ref: "owner confirmed 2026-08-23 (boot-contract-fix-first route)"
    recorded_at: "2026-08-24T00:00:00Z"
    consequence: "Execution unblocks 0333528 first; ProjectionBase substrate deferred. Real root cause (boot/replay contract) repaired at the substrate; Phase-0 answers decide the durability branch."
  evidence_refs:
    - "docs/designs/choir-durable-substrate-2026-08-23.md"
    - "docs/evidence/recovery-consensus-review-2026-08-23.md"
    - "docs/evidence/effects-red-recovery-no-differential-base-2026-08-23.md"
    - "docs/evidence/effects-red-recovery-projections-present-2026-08-23.md"
    - "internal/vmmanager/manager.go; internal/autoputer/run.go; internal/store/computer_events.go; internal/computerevent/appender.go"
    - ".agentic-consensus/recovery-definition-review-20260823-r3/ (gitignored raw outputs)"
  blocker_or_risk: "Confirmed: live timeout 30m, guest 4096 MiB, ordinary BootVM path (rec-2 last cold-recover), no persisted runtime working set (0-byte state file; empty texture repo) => durable checkpoints mandatory; ~3.8 ev/s (approx 10h single pass) means multiple-boot resumable replay is the only path unless throughput improves. Risks: store open at /mnt/persistent/state currently produces a 0-byte file (must be fixed/verified before checkpoints have any effect); RecoverPrepared ErrNeedsProjectionRepair trap on partial-replay kill; bootstrapCtx 30m guest budget; RSS ~1.8 GiB near kernel kill at ~15m in past evidence; stale pending lifecycle receipts could append events during recovery."
  next_action: "B14 HOST-DRIVE 2026-08-24: with the guest stopped, run RUNTIME_RECOVERY_REPLAY_ONLY=1 (same-commit runtime, fresh credential envelope, exclusive RO-mount-able loop of the retained data.img, rollback reflink first) to materialize the embedded projection to head 132,539/acc54c39...; verify no platform appends (head unchanged before+after), witness integrity hard-check; then BootVM the guest (epoch 715+): local==platform -> replay no-op -> head+witness verify -> startup runtime -> host stall-gated probe sees 200 -> route CAS under fencing token -> computer active quiesced. Receipts + consensus adjudication in the final report."



receipts:
  - id: define-recovery-boundary
    boundary: define
    commit_or_artifact: "committed in 9b807f69 (recovery definition + design + evidence docs); rebuilder implemented in 6185910a; vmctl resolve/deadlock fixes f1e56a2e, 51d0646b"
    proof_refs: [docs/evidence/effects-red-recovery-no-differential-base-2026-08-23.md, docs/evidence/effects-red-recovery-projections-present-2026-08-23.md]
    rollback_ref: revert docs commit
    disposition: "Recovery scope isolated and ratified by owner"
    problem_ref: recovery-no-differential-base-2026-08-23
    authorization_ref: "owner ratification 2026-08-23"
    candidate_or_evidence_refs: []
    landing:
      source_commit: github.com/yusefmosiah/go-choir@7109b070
      ci_ref: pending
      deploy_ref: pending
      environment_identity: staging https://choir.news
      deployed_acceptance: pending
    registry_conformance_ref: registry update in 9b807f69
  - id: route-boot-contract-fix-first
    boundary: define
    commit_or_artifact: "(pending docs commit)"
    proof_refs: [docs/definitions/choir-durable-substrate-recovery-2026-08-23.md]
    rollback_ref: revert docs commit
    disposition: "Owner ratified the boot-contract-fix-first route; offline ProjectionBase rebuild moved to a separate successor Definition."
    problem_ref: boot-replay-contract-defect-2026-08-23
    authorization_ref: "owner direction 2026-08-23 (ask selection: boot-contract-fix-first)"
    candidate_or_evidence_refs: []
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: not_applicable
  - id: offline-rebuild-real-tape-failure
    boundary: define
    commit_or_artifact: "(pending evidence doc; failure logged node-b /var/lib/go-choir/rebuild-0333528.log)"
    proof_refs: [node-b /var/lib/go-choir/rebuild-0333528.log, internal/projectionbase/source.go, internal/projectionbase/rebuilder.go]
    rollback_ref: "(none — read-only replay; no artifact published)"
    disposition: "Offline ProjectionBase rebuilder DiskEventSource cannot reduce the real 0333528 tape; production replay reads corpusd Dolt receipts; rebuilder fabricates TransitionInput, skips receipt verification, self-compares witness. Deferred to separate successor Definition; NOT this recovery."
    problem_ref: rebuilder-real-tape-reduction-2026-08-23
    authorization_ref: "owner direction 2026-08-23 (update definition + consensus)"
    candidate_or_evidence_refs: []
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: node-b /opt/go-choir @7109b070
      deployed_acceptance: not_applicable
    registry_conformance_ref: not_applicable
  - id: consensus-review-2026-08-23
    boundary: review
    commit_or_artifact: "(pending docs commit; raw outputs gitignored under .agentic-consensus/)"
    proof_refs: [docs/evidence/recovery-consensus-review-2026-08-23.md, .agentic-consensus/recovery-definition-review-20260823-r1/prompt.md, .agentic-consensus/recovery-definition-review-20260823-r2/prompt.md, .agentic-consensus/recovery-definition-review-20260823-r3/prompt.md]
    rollback_ref: revert docs commit
    disposition: "Three agentic-consensus rounds (8 unique agents: devin, claude, muse-spark, hy3, nemotron-3-ultra, x-preview-f, grok46, cursor, opencode) reviewed the recovery route; all approved boot-contract-fix-first; Phase-0 sharpened into a decision table; Phase-1 split into liveness/readiness, stall-kill, resumable durability, RecoverPrepared-safe."
    problem_ref: boot-replay-contract-defect-2026-08-23
    authorization_ref: "owner direction 2026-08-23 (run consensus + update docs)"
    candidate_or_evidence_refs: []
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: not_applicable
