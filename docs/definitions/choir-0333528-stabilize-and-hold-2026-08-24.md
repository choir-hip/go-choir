---
definition_version: 2
definition_id: choir-0333528-stabilize-and-hold-2026-08-24
execution_mode: mission_orchestrator

start:
  captured_at: "2026-08-24T22:10:00Z"
  source:
    canonical_ref: "main@e9a0be7f"
    deploy_identity: "staging https://choir.news; computer 0333528 pre-genesis restart loop (epoch 774), guest autoputer-runtime pid 1032 alive, self-dev orchestration topology active"
  worktree_inventory:
    status: reconciled
    evidence_ref: "git status 2026-08-24 after docs receipt commit e9a0be7f; .agentic-consensus/ gitignored"
  observed_artifact:
    - claim: "computer-03335285269bdba4f94377e56879f9e6 (owner yusefnathanson, id 5bd6de97-3b58-408c-bf89-c42c81b083de) is pre-genesis in a restart loop: tape has no canonical genesis after the 21:13Z deploy image-refresh; runtime refuses run admission every ~3s; vmctl epoch 771->774, health check on :8085 fails, credential issue on :8086 refused; gateway bootstrap probe lost contact (vmctl resolve context canceled)."
      evidence_ref: "docs/evidence/0333528-recurring-corruption-loop-2026-08-24.md"
    - claim: "Self-dev driver: effects propose_only / effects OFF is NOT sufficient to stop the Super/CoSuper/capsule authoring loop (conductor=2 super=24 texture=6 alive in guest); that loop was the original OOM/corruption trigger (capsule memory exhaustion + assignment supersession loop). Any unblock without a fence re-triggers it."
      evidence_ref: "docs/evidence/0333528-recurring-corruption-loop-2026-08-24.md; docs/definitions/choir-durable-substrate-recovery-2026-08-23.md line 25; agentic consensus persistent-stability-20260824"
    - claim: "Platform canonical head recorded 132,539/acc54c39.../epoch 715 (preflight); whether it survives the image refresh is the read-only audit question. In-guest bootstrap-chain/replay on 0333528 re-triggers the restart loop (credential TTL ~4min < boot ~5min; guest replay 3.2-6.5s/event at the 107k+ band)."
      evidence_ref: "docs/definitions/choir-durable-substrate-preflight-2026-08-24.md line 10; docs/evidence/recovery-replay-guest-io-ceiling-assessment-2026-08-24.md"
    - claim: "No existing maintenance-hold primitive: RecoveryFencingToken (internal/vmctl/cold_recover.go) only fences competing recovery, not auto-restart / deploy-refresh / self-dev admission. vmctl states: booting/active/degraded/stopping/stopped/hibernated/failed (internal/vmctl/ownership.go)."
      evidence_ref: "internal/vmctl/ownership.go; internal/vmctl/cold_recover.go"

finish:
  deliver: "0333528 is held in a durable, host-authoritative maintenance hold: preserved, recoverable to known head 132,539/acc54c39..., no run admitted while held (the guest-visible hold gates all run admission + runtime auto-recovery at the START of Runtime.Start, so no self-dev OR other run executes and nothing is written), excluded from vmctl auto-restart/recover/refresh/reattach/resume/reclaim (lifecycle matrix), deploy active-VM image refresh, and self-dev admission/auto-rewake (host + guest visible)."
  artifact: "A deployed hold + recovery proof that the hold gate is passed: (1) the fence landed + tested (host hold consulted by the vmmanager health/recover/reattach/resume/refresh/reclaim/retention entrypoints, deploy-refresh exclusion on deployed vmctl + CI loop, guest-visible hold boot channel gating run admission + Super re-warm + ensureSelfDevelopmentRun + the hardcoded selfdev rewake; selfdev op left unmutated); (2) hold set, clean stop, and identified reflink snapshot data.img.stable-hold-20260824 of the STOPPED image; (3) read-only audit receipt (platform head 132,539/acc54c39... survives? which image matches local head); (4) 0333528 recovered under hold to local==platform (delta 0, boot <30s, valid credentials, no restart loop), servable and mutation-fenced; (5) stable-state gate passed (servable, mutation-fenced, local==platform, no restart loop, disk headroom, guest-visible hold)."
  acceptance:
    - action: "Fence: set the host hold for 0333528 -> vmctl refuses auto create/start/boot/recover/refresh/reattach/restart/reclaim/prune/delete across the lifecycle matrix (no further epoch increments); deploy active-VM refresh (RefreshVMForDesktop) refuses it and the CI loop filters it; the guest runtime gates self-dev run admission, Super re-warm (reconcilePersistentSuperActor / rewarmInterruptedPersistentSuperActors), ensureSelfDevelopmentRun, and the selfdev-ccf0f1ec rewake via the guest-visible hold channel; selfdev-ccf0f1ec is left executing and unmutated (no state transition, no tape write)."
      proves: "The driver (self-dev loop) and every restart/refresh/reattach/resume/reclaim/retention path are durably held before any unblock."
      evidence_class: deployed hold + unit tests on each consulted surface
    - action: "Preserve: hold set first, realization cleanly stopped (never a mid-write looping disk), then identified reflink snapshot data.img.stable-hold-20260824 of the STOPPED image."
      proves: "The held state is recoverable to a known, crash-consistent, stopped point."
      evidence_class: host snapshot receipt
    - action: "Audit: read-only receipt — does platform canonical head 132,539/acc54c39... survive, and which on-disk image (data.img.pre-hostdrive-20260824 vs live data.img vs data.img.quarantine-1-89c24...) carries the matching local head?"
      proves: "Decides B14 rematerialization-to-head vs reuse-head-image vs escalate (head lost)."
      evidence_class: read-only audit receipt (no mutation)
    - action: "Recover under hold: B14 host-side replay-only rematerialization (RUNTIME_RECOVERY_REPLAY_ONLY=1) to exact head, or reuse an image matching head+witness; boot under hold (guest-visible hold on); local==platform (delta 0), <30s boot, valid credentials, no restart loop; servable and mutation-fenced."
      proves: "The computer is back to a known head without re-triggering the restart loop or the self-dev driver."
      evidence_class: deployed recovery under hold + head/witness match + guest-visible-hold verification
    - action: "Stable-state gate: servable, mutation-fenced, local==platform (delta 0), no restart loop, disk headroom (Dolt GC / .darc reclaim decision), guest-visible hold active."
      proves: "0333528 is safe to leave quiesced while the overhauls run on a test computer."
      evidence_class: stable-state gate receipt (health + head + disk + hold-visibility)
  rollback: "Revert mission commits via origin/main + CI; canonical events are never rewound. Any failed fence/release leaves 0333528 in the held or stopped state (safe). Recovery to pre-hold state uses data.img.stable-hold-20260824 or data.img.pre-hostdrive-20260824 (rollback reflink); quarantine images preserved."

boundaries:
  mutation_class: red
  authority_sources: [owner direction 2026-08-24, docs/evidence/0333528-recurring-corruption-loop-2026-08-24.md, agentic-consensus persistent-stability-20260824, docs/choir-doctrine.md, docs/computer-ontology.md, AGENTS.md]
  must_preserve:
    - Single guest ComputerEventAppender remains the sole semantic event writer; this mission performs no tape mutation.
    - Canonical event chain at head 132,539/acc54c39... preserved; the runtime run-admission gate stays servable (health + bootstrap-chain route reachable).
    - selfdev-ccf0f1ec operation prompt/trajectory/resume metadata preserved (the op record is NOT state-transitioned or tape-written; it is held by the admission gate).
    - Pre-upgrade snapshots, retained v1 binary/package, and the 103 authentic tape events 132,437-132,539 never overwritten or cleaned.
    - DSN/root+per-database connector semantics unchanged.
  protected_surfaces: [canonical computer event chain, guest ComputerEventAppender, vmctl lifecycle, deploy active-VM refresh routing, self-dev admission/Super reconciler, platform-artifacts store]
  not_goals:
    - The overhauls themselves (Tracks K/F/M/Assurance): they run on a test computer after the hold (this definition owns the hold gate, not the tracks).
    - The effects mission continuation: it is held by this mission (self-dev work held via the admission gate; the op record is unmutated) and resumes after release; no candidate authoring here.
    - Production environment deployment.
    - Any live-store writes: the recovered 0333528 is mutation-fenced; no self-dev writes.
    - Re-baselining: if the audit shows the platform head did NOT survive, this escalates to the owner (a fresh genesis is a separate, owner-approved decision, not a silent default).
  completion_evidence_floor: [fence landed + tested, hold-set + clean-stop snapshot, read-only audit receipt, recovery-under-hold receipt, stable-state gate (servable, mutation-fenced, local==platform, no restart loop, disk headroom, guest-visible hold)]

phases:
  - name: Define (this doc) — fence contract + hold Definition
    items:
      - "Author the stabilize-and-hold Definition + the fence surfaces; run agentic-consensus confirm (returned REPAIR); adjudicate the must-fixes; owner-ratify; update registry-hygiene surfaces (docs/ACTIVE.md, docs/mission-graph.yaml, docs/doc-authority-manifest.yaml)."
  - name: Fence — implement the host-authoritative maintenance hold
    items:
      - "Add a durable host-side hold: a per-computer HoldStatus / maintenance flag as a first-class field on VMOwnership and persistedOwnershipState (carried across restart + deploy; set only by an authorized owner/recovery op). On a host restart the hold must not be silently dropped back into the unhealthy->kill->reboot loop."
      - "Lifecycle matrix — the hold is consulted at EVERY realization-affecting and state-deleting entrypoint, not only the sweeper's unhealthy path. For a held computer: refuse any auto create/start/boot/recover/refresh/reattach/restart/stop/delete/prune EXCEPT under the authorized maintenance capability. Covered: vmmanager health loop recoverOrRestartActiveVM / ensureActiveVMReady / RecoverVMForDesktop / MarkUnhealthyForDesktop (resolve-on-health-fail — the live epoch 771->774 kill path); resolveDesktopContext stopped|hibernated|degraded|failed|booting -> startExistingVM (internal/vmctl/ownership.go:1282-1344, 870-886 — THIS fires after this mission stops the computer, on the next resolve) and proxy ResolveDesktopContext -> RefreshDesktopContext (internal/proxy/compute_status.go:567-588); warmness / pressure-reclaim / idle-sweep / hibernation; WarmAlwaysOnDesktops -> ResumeVMForDesktop (always-on owner forced resume); ReattachManagedVMs (after a vmctl restart); RefreshVMForDesktop (force-reboot of stopped|hibernated|degraded|failed); retention-prune (DestroyVMState, internal/vmctl/retention_prune.go) and RemoveOwnershipForDesktop / logout; vmmanager shutdown Manager.stop(true) (internal/vmmanager/manager.go:511-543) leaves held VMs running (or uses StopHealthChecks/leave-running); HandleColdRecover (internal/vmctl/cold_recover.go:526-583) only proceeds for a held computer under the authorized maintenance capability."
      - "Deploy-refresh exclusion — RefreshVMForDesktop itself refuses a held computer (so refusal is an intentional skip, not a deploy failure), AND the CI active-VM refresh loop (ci.yml ~1125-1193) filters held records. The hold must be live on the DEPLOYED vmctl before any hold bit is set, or it is a no-op."
      - "Guest-visible hold channel — a held computer boots with the hold visible to the guest runtime (boot env RUNTIME_MAINTENANCE_HOLD=1, or a resolve/override payload), so the guest gates self-dev run admission, reconcilePersistentSuperActor / rewarmInterruptedPersistentSuperActors (boot-time Super re-warm), ensureSelfDevelopmentRun, and the hardcoded selfdev-ccf0f1ec rewake — AND short-circuits automatic runtime recovery/dispatch at the START of Runtime.Start, before rewarmInterruptedLifecycleActivations / reconcilePersistentSuperActor / CoSuper reconcile / actor-item and update sweeps, so no run (self-dev or otherwise) is admitted or rewarmed while held. After B14 the computer is no longer pre-genesis, so admission would otherwise SUCCEED and Super re-warm starts the driver under hold."
      - "No self-dev op-state mutation — do NOT transition the op to a non-existent StatePaused/StateBlocked: internal/selfdev/operations.go defines only Requested/Executing/Frozen/Verified/AwaitingApproval/Accepted/Materializing/Applied/RollbackPending/RolledBack/Failed/Degraded; Executing transitions only to Frozen or Failed; every Transition writes a tape projection (violates must_preserve). Leave selfdev-ccf0f1ec executing and unmutated; the driver is held by the host+guest admission gate, not an op-state change."
      - "Unit tests per consulted surface; the hold is host-authoritative and cannot be cleared by a guest-side op."
      - "**Authorized maintenance exception** — the hold refuses ALL auto lifecycle actions, but allows a narrowly-scoped set of explicitly authorized maintenance operations, each requiring an authorized recovery capability (an owner/recovery operation carrying a fencing/recovery token — NOT an ordinary resolve/BIOS): (1) the hold-stop (StopVMForDesktop); (2) the B14 replay-only recovery boot (RUNTIME_RECOVERY_REPLAY_ONLY=1, no runtime start); (3) a second boot with guest-visible hold. Nothing else boots/stops/recovers/refreshes a held computer. This resolves the refuse-all vs must-stop/boot contradiction."
  - name: Apply hold to 0333528
    items:
      - "Set the host hold for 0333528 FIRST (so health/resolve cannot reboot it during the stop); verify no further epoch increment / no auto-restart / no deploy-refresh / no self-dev auto-rewake."
      - "Cleanly stop the realization (quiesce under hold, then stop, never stop a mid-write looping disk), THEN take the identified reflink snapshot data.img.stable-hold-20260824 of the STOPPED image."
  - name: Read-only audit
    items:
      - "Confirm platform canonical head (132,539/acc54c39...) survival; identify which on-disk image carries the matching local head; record the audit receipt (no mutation). Decide B14-to-head vs reuse-image vs escalate."
  - name: Recover under hold
    items:
      - "B14 host-side replay-only rematerialization (RUNTIME_RECOVERY_REPLAY_ONLY=1) to exact head, or reuse the matching image; boot under hold (guest-visible hold on); local==platform (delta 0), <30s boot, valid credentials, no restart loop."
      - "Verify mutation-fenced: servable + bootstrap-chain route reachable, no run admitted while held (guest-visible hold gates all run admission + runtime auto-recovery at Runtime.Start), so the computer cannot write while held."
  - name: Stable-state gate
    items:
      - "Verify servable + mutation-fenced + local==platform (delta 0) + no restart loop + disk headroom (Dolt GC/.darc reclaim decision); record the gate receipt."
  - name: Overhauls, adoption, release (HAND-OFF)
    items:
      - "This gate is a hand-off, not this mission's finish: run Tracks K/F/M/Assurance on a test computer / isolated realization per choir-durable-substrate-overhauls-2026-08-23.md (OWNED by that definition); 0333528 stays held. Then, as a separate gated step, apply accepted substrate slices onto 0333528 under per-slice snapshot fences (Track K then Track F before broad reactivation), and finally release the hold + resume the effects mission only after full durable-substrate acceptance. These are recorded as the successor boundary; this mission ends at the stable-state gate."
  - name: Release + resume (HAND-OFF)
    items:
      - "This is a successor boundary, not this mission's finish: the hold is explicitly released and the effects operation resumes from preserved state only after the overhauls definition completes full durable-substrate acceptance and the adopted slices are applied under snapshot fences. On this mission's completion, update the now card to the HAND-OFF and surface the successor boundary (overhauls + effects definitions)."

now:
  status: working
  slice: "Phase Fence landed + deployed: host-authoritative maintenance hold (VMOwnership.HoldStatus + lifecycle-matrix guards on resolve/ready/refresh/reattach/warm/idle/retention/pressure-reclaim; commit e1742675), guest-visible hold channel (Runtime.Start rewake gate + run-admission + ensureSelfDevelopmentRun gate; 87a71119 + ci.yml fix c25ca7d7). Staging deployed at c25ca7d7 via force-deploy 32792957766. selfdev-ccf0f1ec left executing + unmutated. NEXT: Apply-hold needs an owner-scoped product-path hold route (proxy + corpusd + vmctl client + CLI), since /internal/vmctl/hold is Node-B-internal only."
  question: none
  reconciliation:
    observed_at: "2026-08-24T22:10:00Z"
    source_ref: "main@e9a0be7f"
    deploy_identity: "staging https://choir.news; 0333528 pre-genesis restart loop epoch 774"
    status: reconciled
  candidate:
    id: none
    state: none
  decision:
    selected: "Hold-then-unblock: fence 0333528 first (host hold + guest admission gate; selfdev op unmutated), then preserve stop-snapshot + read-only audit, then B14-recover under hold, then overhauls on a test computer, then adopt + release only after full acceptance. This inverts the prior recover-then-hope sequence that kept re-corrupting."
    kind: architecture
    status: proposed (owner ratification pending; supported by agentic-consensus persistent-stability-20260824 + the define-confirm round)
    source: owner direction 2026-08-24 + agentic consensus 2026-08-24
    owner_ratification_ref: pending
  blocker_or_risk: "Platform head 132,539/acc54c39 survival is unverified (audit decides); in-guest bootstrap-chain/replay on 0333528 re-triggers the restart loop (credential TTL < boot; replay 3-6s/event) so the hold must precede any tape write; unblocking before the fence is the documented recurrence trap."
  evidence_refs:
    - docs/evidence/0333528-recurring-corruption-loop-2026-08-24.md
    - docs/definitions/choir-durable-substrate-recovery-2026-08-23.md
    - docs/definitions/choir-durable-substrate-preflight-2026-08-24.md
    - docs/definitions/choir-durable-substrate-overhauls-2026-08-23.md
    - docs/designs/choir-durable-substrate-2026-08-23.md
  next_action: "Apply-hold: add owner-scoped product-path hold/unhold (CLI computer hold -> proxy lifecycle -> corpusd -> vmctl SetHold/ClearHold) + deploy; then set the host hold on 0333528 FIRST, verify no epoch increment / no auto-restart / no redeploy-refresh / no selfdev rewake, cleanly stop the realization, reflink snapshot data.img.stable-hold-20260824, read-only audit (head 132,539/acc54c39 survival), B14 recover-under-hold, stable-state gate."

receipts:
  - id: define-stabilize-hold
    boundary: define
    commit_or_artifact: "docs/definitions/choir-0333528-stabilize-and-hold-2026-08-24.md (pending panel confirm + owner ratification)"
    proof_refs: [docs/evidence/0333528-recurring-corruption-loop-2026-08-24.md]
    rollback_ref: revert docs commit
    disposition: "Stabilize-and-hold Definition authored with the fence mechanism action plan; agentic-consensus define-confirm returned REPAIR; five must-fixes adjudicated into the Definition; proportionate re-confirm next."
    problem_ref: docs/evidence/0333528-recurring-corruption-loop-2026-08-24.md
    authorization_ref: owner direction 2026-08-24
    candidate_or_evidence_refs: []
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: pending (update ACTIVE.md / mission-graph.yaml / doc-authority-manifest.yaml on accept)

  - id: fence-stabilize-hold
    boundary: fence
    commit_or_artifact: "e1742675 (host hold), 87a71119 + c25ca7d7 (guest-visible hold + CI filter); staging deployed c25ca7d7 via force-deploy 32792957766"
    proof_refs: [maintenance_hold_test.go (vmctl + agentcore), lifecycle-matrix guard grep, staging /health build.deployed_commit c25ca7d7]
    rollback_ref: revert origin/main + CI
    disposition: "Host fence + guest-visible hold channel + CI held filter implemented, tested (go test -mod=mod internal/vmctl + agentcore green), built (autoputer/gateway/choir/proxy/vmctl), and deployed to staging c25ca7d7. selfdev op left executing + unmutated. Next: product-path hold route then Apply-hold."
    problem_ref: docs/evidence/0333528-recurring-corruption-loop-2026-08-24.md
    authorization_ref: owner direction 2026-08-24
    candidate_or_evidence_refs: []
    landing:
      source_commit: c25ca7d7
      ci_ref: "32787344984 (host, success); 32792957766 (force-deploy, success)"
      deploy_ref: workflow_dispatch force_staging_deploy=true
      environment_identity: "staging https://choir.news deployed_commit c25ca7d7"
      deployed_acceptance: "internal/vmctl + agentcore maintenance-hold unit tests green; host fence e1742675; guest hold 87a71119; CI filter c25ca7d7"
