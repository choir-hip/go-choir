---
definition_version: 4

# Station LU of the ledger metamission
# (docs/definitions/choir-ledger-metamission-2026-10-13.md): guest updates.
# Owner 2026-10-14, in session: "we do have to just improve the updating
# system in general. Like, I want to be pushing safe updates out to guest
# computers. And similarly, I think this is going to relate to the
# self-development system." On the six-point direction (pull not push;
# drain with a deadline; work-state migrations; lighter restarts; staged
# rollout with health check and automatic rollback; one apply path shared
# with self-development): "Yes, this all makes sense as a new station."
# Specification: docs/reports/lu-guest-updates-design-2026-10-14.md. This
# file is the executable station over that design; the design stays the
# specification.
readiness: drafted

review:
  reviewer: none
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-14T10:47:00Z
  source:
    canonical_ref: main 97d7a573
    deploy_identity: staging 97d7a573 (proxy /health build.deployed_commit, deployed 10:40:40 UTC)
  worktrees:
    - path: /Users/wiz/go-choir/.claude/worktrees/agent-a81c481b0c103459a (branch docs/lu-guest-updates-2026-10-14)
      status: clean
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: branch
    - path: /Users/wiz/go-choir and its sibling worktrees (go-choir-l1-deploy, go-choir-hibernate, go-choir-d7 and others)
      status: unknown
      class: other_agent_wip
      owner: not this session
      touch: forbidden
      recovery: leave in place
    - path: stash@{0..4}
      status: unknown
      class: other_agent_wip
      owner: not this session
      touch: forbidden
      recovery: leave in place
  observed:
    - >-
      A deploy asks each running computer once, at deploy time, for a
      planned restart; a busy computer is deferred to the next deploy and
      nothing retries it in between (.github/workflows/ci.yml:1466-1469,
      1528; internal/vmctl/deploy_refresh.go:282-283).
    - >-
      The live paper is never idle and did not take ce59d406 by itself; it
      took e3437f1c only through an owner-approved manual refresh that ran
      as a crash boot and closed its in-flight runs
      (docs/evidence/owner-approved-refreshes-2026-10-14.json).
    - >-
      At 05:48 UTC the paper's first refresh after two busy deferrals
      crossed work-state epoch 1 -> 2 (C1e B1); the guest read the planned
      restart as a crash (internal/agentcore/planned_restart.go:133-136)
      and closed all 14 schedules for about an hour
      (docs/problems/planned-refresh-closes-paper-schedules-2026-10-14.md).
      Schedules now survive crashes (ce59d406); runs, rounds and pending
      arrivals still close.
    - >-
      The planned-restart endpoint closes no admission gate and does not
      sync filesystems before the host's SIGKILL
      (planned_restart.go:395-415; internal/vmmanager/manager.go:1447). The
      hibernate fence is the only admission gate
      (internal/agentcore/hibernate.go:68-172).
    - >-
      An image refresh has no health gate and no automatic rollback; the
      previous guest image is not retained per computer. The guest updater
      already has stage, swap, restart, probe and restore-prior with signed
      receipts for layered releases (internal/updater/updater.go:192-418,
      943-994), used by self-development and the M9a platform-update offer;
      CI's app-layer push was deleted in a47122de after version skew and
      0-of-2 bind failures.

finish:
  deliver: >-
    Platform updates reach every guest computer safely and by themselves:
    each computer takes a deploy at its next quiet moment or after a
    bounded drain, as a planned restart that keeps its work, under a
    health check that rolls a bad build back automatically, staged from
    test computers to the paper to the owner's computer, with every update
    recorded on the computer's event chain, through the same apply path
    approved self-development uses.
  artifact: >-
    vmctl's update sweep and per-computer update state in `vmctl list` and
    the owner's computer page; the guest's drain endpoint and work-state
    migration registry; the probation fence, retained previous image and
    ring gate; the layered runtime swap; one ReleaseApply record shape for
    platform and self-development applies; the deployed acceptance
    receipts below
  acceptance:
    - action: >-
        LU1. Deploy a guest-runtime change while the paper is busy; do not
        touch the paper. Read vmctl's update state, the paper's
        /api/runtime/obligations restart block, its schedules and its trace.
      proves: >-
        the live paper takes a platform update within 60 minutes of the
        deploy (O7) with restart kind planned, 14 of 14 schedules active at
        their pre-update generation, no run, round or arrival closed as
        interrupted_by_restart, and a story revision within 30 minutes
        after the boot
      evidence_class: deployed proof
    - action: >-
        LU1. On a minted test computer, start a fixture run that outlives
        the drain deadline, then deploy.
      proves: the drain times out, admission reopens, no restart and no marker remain, the attempt is recorded, and the sweep retries later; after K attempts the "update stuck" signal shows
      evidence_class: deployed proof
    - action: >-
        LU1. On a minted test computer, a run that is executing when the
        drain begins needs a continuation (a child result) to finish.
      proves: the continuation is admitted, new work is held, the run finishes, and the boot is planned (no drain deadlock)
      evidence_class: deployed proof
    - action: >-
        LU2. Deploy a build that bumps the work-state epoch (the next real
        bump, or a fixture build on a minted test computer with queued
        wakes and an open schedule).
      proves: the boot is planned, a work_state_migrated record names exactly what the migration closed, and the queued wakes are delivered after the boot
      evidence_class: deployed proof
    - action: >-
        LU3. Deploy a deliberately bad build (fixture: /health never
        reaches ready) with ring 0 = one minted test computer.
      proves: the test computer rolls back to its previous image automatically within T_ready plus one boot, exactly once; the bad build is refused for it; the rollout never reaches the paper; the rollback is a lifecycle_observed update record on its chain
      evidence_class: deployed proof
    - action: >-
        LU3. A good build rolls out with rings on.
      proves: ring 0, then the paper, then the owner's computer each pass probation and soak in order, each recorded on its chain
      evidence_class: deployed proof
    - action: >-
        LU4. A runtime-only change on a minted test computer.
      proves: the runtime is swapped by the guest updater with runtime unavailability of 10 s or less, the Firecracker process unchanged, restart kind planned, and the next VM boot runs the new base with no skew
      evidence_class: deployed proof
    - action: >-
        LU5. On one test computer, an approved self-development apply and a
        platform update, then a restore to the pinned prior head.
      proves: both use one ReleaseApply record, the same admission, health and rollback, and the restore returns the computer to the prior head
      evidence_class: deployed proof
  rollback: >-
    Per slice (boundaries.slice_rollback). Tag the last-good commit before
    each slice's first push.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the time from a deploy to every active computer running it
    (update lag, worst computer first) and the work lost per update (runs,
    rounds, arrivals closed; schedules stopped; minutes silent), while no
    update forces a busy computer, a crash stays a crash, and a bad build
    reaches at most ring 0 before it is rolled back.
  goodharting_would_be: >-
    Low lag because updates force busy computers or kill mid-drain; "no
    work lost" because the drain refuses work that is then silently
    dropped instead of held; planned boots that are crashes relabelled;
    "rollback works" shown only on a build that never ran work; a fleet
    "up to date" because pending computers are hidden or skipped as
    divergent.

homotopy:
  realism_axis: >-
    unit tests and nix eval -> one minted test computer on staging with
    fixture builds -> ring 0 with real deploys -> the live paper -> the
    owner's computer -> every active computer

boundaries:
  mutation_class: red
  authority_sources:
    - owner 2026-10-14 in session (the framing quoted above; "Yes, this all makes sense as a new station")
    - CLAUDE.md "Restarts End Work (Crash) Or Resume It (Planned Update)", "Deploys never restart a busy computer", Landing Loop "Product restore vs platform deploy"
    - docs/restart-subsystem-design-2026-10-13.md v3 (Invariants R, W, P; §9 "wait, then defer")
    - docs/computer-ontology.md (self-development contract, restore-set boundary)
    - docs/reports/lu-guest-updates-design-2026-10-14.md
  must_preserve:
    - a crash restart never resumes work; a planned restart resumes once
    - no automatic path restarts a guest with work inside its admission fence; busy is never forced
    - schedules (standing intents) survive every update
    - no rollback across a work-state epoch or store-schema window
    - one automatic rollback per computer and build; the same build is never re-applied automatically after it
    - the base image is rebuilt on every guest-runtime deploy (a47122de stands)
    - self-development authority stays the owner's decision; a platform update enters only under a platform-signed follow policy and never on a divergent computer
    - pending is derived from booted and target image identity, never stored as a second truth
  excluded:
    - forced updates of busy computers by any automatic path
    - rebasing platform updates onto self-developed (divergent) computers
    - zero-downtime host-service deploys (L1 D3, D4)
    - A/B image slots beyond keeping the previous image for rollback
  protected_surfaces:
    - deployment routing (CI deploy-refresh step, deploy-impact classification)
    - vmctl (refresh, idle sweep, ownership, retained images, ring state)
    - VM lifecycle and boot (vmmanager refresh, boot overrides)
    - restart semantics (planned marker, work-state epoch, crash closure)
    - run admission (the guest fence and drain)
    - self-development apply (materializer, guest updater, checkpoint and route)
    - the computer event chain (lifecycle receipts)
  admissible_evidence: >-
    deployed proof on staging (minted test computers, then the paper and
    the owner's computer); unit tests and nix eval shape a slice but never
    prove vmctl, boot, restart or update behavior
  slice_rollback:
    LU1: revert; interim ownership fields are ignored by older vmctl; the guest drain endpoint is additive; CI's deploy-time refresh stays until LU3
    LU2: revert; epoch steps without a migration fall back to D7 (crash boot)
    LU3: VM_UPDATE_HEALTH_GATE=off returns to LU1 behavior; the retained-image GC root is harmless
    LU4: the classifier flag off sends every update down the VM path; a layer is dropped at the next boot by the base binding
    LU5: revert per commit; record shapes are additive
  dependencies:
    - LU1 needs nothing new; it lands between C1e slices (one shared-runtime cutover at a time)
    - LU2 lands before the next work-state epoch bump (expected in C1e B2-B5)
    - LU3 needs L1c (host-asserted planned boots, for the probation flag and the rollback boot) and L1b (grants and the recovery ledger)
    - LU5 needs C1e's Freeze and Verify path working end to end on staging
  heresy_delta:
    discovered:
      - deferred computers retried only at the next deploy (a busy computer never updates)
      - planned-restart endpoint with no admission fence and no sync before the kill
      - a marker that outlives a refresh that never happened
      - epoch changes turning planned refreshes into crashes
      - image refreshes with no health gate, no rollback and no ring order
      - updates absent from the computer's event chain
    introduced: []
    repaired: []

standing_questions:
  q1_who_settled: >-
    The station and its six-point direction: owner, 2026-10-14 in session.
    Every default in the design's §6 (drain deadline, rings, epoch
    forward-only, LU4, hibernated computers, quiet wait, CLAUDE.md wording)
    is an orchestrator proposal until the owner rules.
  q2_conformance: >-
    Conforms to: crash never resumes (U3 keeps crashes crashes); deploys
    never restart a busy computer (U2: restart only after a drain reports
    nothing inside; never forced); restart design v3 §9 "wait, then defer";
    Invariant P (LU3 waits for L1c); a47122de (base always rebuilt); the
    ontology's self-development contract (owner decision stays the
    authority; divergent computers excluded). Tension named: the drain
    pauses new work on a computer for up to the deadline, which the
    CLAUDE.md sentence does not describe; O8 asks the owner to amend it.
  q3_deletion_citers: >-
    LU deletes nothing in LU1-LU3. LU5 retires the materializer's separate
    soft hold (SelfDevelopmentApplyMaterializing) into the shared gate; its
    only citer is the actor handler (internal/actorruntime/handler.go),
    repaired in the same commit. CI's deploy-time refresh loop is removed only in LU3, with
    the workflow contract test updated.
  q4_production_consumers: >-
    deploy-refresh: CI's deploy job (sole caller). planned-restart endpoint:
    vmctl deploy refresh. updater.Apply: the self-development materializer
    and the M9a platform-update offer handler (no live producer of offers
    since a47122de). Hibernate fence: the actor handler and S3 hibernate.
  q5_single_authority: >-
    Pending: derived from vmctl's booted-image record and the host's target
    image, never stored. Attempt history: one host ledger (L1b's recovery
    ledger; interim fields on the ownership until it lands, as L1b-0 did).
    Boot kind: the guest's consumed marker until L1c, then the host's
    attestation; the guest's verdict is reported back so the two cannot
    silently disagree. Release identity on a computer: its event chain.
  q6_success_artifact: >-
    Each acceptance names the artifact read: vmctl's update state, the
    guest's restart block, the schedule list with generations, the
    trace (closed runs and arrivals), the lifecycle_observed update
    records on the chain, the Firecracker pid. A CI line saying
    "refreshed" is not proof.
  q7_fate_sharing: >-
    An update fate-shares with vmctl (sweep), the guest (drain, marker),
    the host's image store (target and retained image) and corpusd (event
    chain append). A vmctl restart mid-drain is survived by a guest-timed
    drain; a corpusd outage delays the record, not the update (the receipt
    stays pending in the guest, as lifecycle receipts do today).
  q8_restart: >-
    Drain state is guest-side with a TTL; the marker is consumed once;
    fence expiry clears an unused marker; interim attempt fields are on
    the host ownership; ring state is a projection of chain records and is
    rebuilt after a vmctl restart.
  q9_no_ssh: >-
    Update state in `vmctl list` and the owner's computer page; the drain
    and restart blocks through the runtime obligations route; rollouts
    halted and resumed by owner acts through the product API. No step of
    any acceptance uses SSH.
  registry_hygiene: >-
    Spine station list, docs/mission-graph.yaml and docs/ACTIVE.md updated
    in the commit that adds this file. The ledger stations are not listed
    in docs/doc-authority-manifest.yaml (none of L1, L2a, LR are); LU
    follows that precedent, recorded as a residual.

now:
  status: working
  slice: >-
    Definition: this file and the design note are drafted; a convergent
    panel reviews both before LU1 starts. LU1 (pending, quiet moment,
    drain) is the first build slice.
  source_ref: 97d7a573
  deploy_identity: staging 97d7a573
  candidate: {id: none, state: none, ref: none, base: none, digest: none, scope: []}
  conjecture:
    id: LU-1
    claim: >-
      The paper has quiet moments or drains within 15 minutes once new
      work is held, so a pull sweep with a bounded drain updates it within
      60 minutes of a deploy without forcing it; and if update boots stay
      planned (LU2), an update costs the paper no closed work.
    test: >-
      Sample the paper's executing runs and fenced occurrences every 60 s
      for 6 h through the runtime obligations route (quiet-gap
      distribution, what is executing at each refusal); then LU1's first
      acceptance on staging
    edge: missing_oracle
    delta_o: the guest reports why it refused (executing run ids, kinds, ages), so a refusal names its cause
    scope_if_supported: the live paper and the owner's computer on staging
    status: proposed
    evidence_refs: [docs/problems/planned-refresh-closes-paper-schedules-2026-10-14.md, docs/evidence/owner-approved-refreshes-2026-10-14.json]
  decision:
    what: "slices LU1 -> LU2 -> LU3 -> LU4 -> LU5; defaults of design §6 until the owner rules"
    kind: operational
    status: proposal
    evidence_ref: docs/reports/lu-guest-updates-design-2026-10-14.md
    owner_ratification_ref: "owner 2026-10-14 in session approved the station and its six-point direction; O1-O8 open"
  belief:
    believed_state: >-
      Deploys push once and defer busy computers; the paper updates only by
      owner act; an epoch bump makes a planned refresh a crash; image
      refreshes have no health gate or rollback; the guest updater already
      has health and rollback for layered releases.
    main_uncertainty: whether the paper's executing work drains within a bounded deadline (or a continuation deadlock, F15, blocks every drain)
    next_observation: the paper's quiet-gap and refusal-cause sample
  blocker_or_risk: >-
    LU3 waits on L1c and L1b. LU2 must land before the next epoch bump or
    the next paper refresh is again a crash boot. LU1's guest half shares
    the runtime with C1e and lands between its slices.
  next_action: run the convergent panel on this file and the design note; adjudicate; amend to round 2

receipts: []
---

# LU: guest updates

The design note is the specification; this file tracks its execution.

- **LU1** pending, quiet moment, drain: vmctl's sweep offers each pending
  computer its update at every pass; the guest restarts at a quiet moment
  or after a bounded drain that holds new work and never forces.
- **LU2** work-state migrations: an epoch change runs a registered
  migration at a planned boot instead of reading as a crash.
- **LU3** health, rollback, rings: a probation fence and health contract,
  automatic rollback to the retained previous image, rollout from test
  computers to the paper to the owner's computer.
- **LU4** binary-only swap: the guest updater swaps the runtime and
  restarts one unit when nothing else changed.
- **LU5** one apply path: platform updates and approved self-development
  share admission, apply, health, rollback and the record on the chain.
