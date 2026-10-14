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
#
# Changelog. Round 1 (346d968c): first draft. Round 2: a convergent panel
# (claude, gemini, glm, muse, deepseek accept with repairs; gpt-6-luna
# send_back) adjudicated in
# docs/reports/lu-guest-updates-adjudication-2026-10-14.md; every blocking
# finding adopted. Slices re-cut: LU1a (host-only retry) and LU2 first,
# then LU1b (drain), LU3a (rings), LU3b (probation and rollback), LU5, LU4
# only if measured. Acceptance re-scoped (the 60-minute lag is conditional
# on the work draining; same-epoch builds until LU2); standing questions
# q7-q9 rewritten as obligations; doc-authority-manifest entry added.
# Round 3: the round-2 panel on cca944f7 (claude, gemini, glm, deepseek
# accept with repairs; muse, gpt-6-luna send_back) found every round-1
# blocker repaired and named new gaps (S1-S31 in the adjudication): the
# final call becomes a recoverable protocol, the drain's admission a
# defined fail-closed predicate with its own deferral class, LU2 reads the
# epoch at open and advances it only after each step (actor log included,
# downgrades hold), LU1a names its interfaces, rings pin the image, and
# the station no longer depends on L1 as a whole. Acceptance gains the
# final-call recovery, grandchild and cross-trajectory, downgrade and
# failed-migration cases.
readiness: drafted

review:
  reviewer: agentic-consensus convergent round 2 on cca944f7 (claude, omp-gemini38, omp-glm53-flash, omp-deepseek-v41-flash accept with repairs; omp-muse-spark, omp-gpt6-luna send_back); round 1 on 346d968c (5 accept with repairs, 1 send_back)
  frozen_ref: commit cca944f7 (round 2); round 3 (this file) not yet reviewed
  verdict: send_back
  evidence_ref: docs/reports/lu-guest-updates-adjudication-2026-10-14.md, sections "Round 2" and earlier (raw outputs in the session scratchpad panel-lu/ and panel-lu/round2/)

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
  worktree_edge: >-
    The authoring agent was isolated to its own worktree and could not
    inspect the others. They are touch-forbidden and no LU move depends on
    them; the first build slice reconciles them before any code commit.
  observed:
    - >-
      A deploy asks each running computer once, at deploy time, for a
      planned restart; a busy computer is deferred to the next deploy and
      nothing retries it in between (.github/workflows/ci.yml:1466-1469,
      1528; internal/vmctl/deploy_refresh.go:282-283).
    - >-
      The paper accepted planned refreshes at 05:48, 06:03 and 06:36 on
      10-14, was not refreshed after intake.labels went on (06:50), and
      took e3437f1c only through an owner-approved manual refresh that ran
      as a crash boot and closed its in-flight runs
      (docs/evidence/owner-approved-refreshes-2026-10-14.json).
    - >-
      At 05:48 UTC the paper's refresh crossed work-state epoch 1 -> 2 (C1e
      B1); the guest read the planned restart as a crash
      (internal/agentcore/planned_restart.go:133-136) and closed all 14
      schedules for about an hour
      (docs/problems/planned-refresh-closes-paper-schedules-2026-10-14.md).
      Schedules now survive crashes (ce59d406); runs, rounds and pending
      arrivals still close.
    - >-
      The planned-restart endpoint closes no admission gate and does not
      sync filesystems before the host's SIGKILL
      (planned_restart.go:395-415; internal/vmmanager/manager.go:1447).
      Four separate admission holds exist (hibernate fence, self-development
      apply hold, pre-boot gate, maintenance hold); only the apply hold
      defers instead of blocking (internal/actorruntime/handler.go:61-171).
    - >-
      An image refresh has no health gate and no automatic rollback; the
      previous guest image is not retained per computer. The guest updater
      already has stage, swap, restart, probe and restore-prior with signed
      receipts for layered releases (internal/updater/updater.go:192-418,
      943-994); CI's app-layer push was deleted in a47122de after version
      skew and 0-of-2 bind failures.

finish:
  deliver: >-
    Platform updates reach every guest computer safely and by themselves:
    each computer takes a deploy at its next quiet moment or after a
    bounded drain, as a planned restart that keeps its work, staged from
    test computers to the paper to the owner's computer, rolled back
    automatically when the build fails its health check, recorded on the
    computer's event chain, through the same apply path approved
    self-development uses.
  artifact: >-
    vmctl's update sweep and per-computer update state in `vmctl list` and
    the owner's computer page; the persisted work-state epoch and migration
    registry; the guest's admission gate with reasons, drain and final
    call; ring gating, refusal records and lifecycle update receipts; the
    probation hold and retained previous image; one ReleaseApply record
    shape for platform and self-development applies; the layered runtime
    swap only if O5 rules it in; the deployed acceptance receipts below
  acceptance:
    - action: >-
        LU1a. With the sweep on ring 0, deploy a same-epoch guest change
        while a minted test computer is busy, then let it go quiet. Read
        `vmctl list` update state, the guest's restart.kind on
        /api/runtime/obligations, and the sweep log.
      proves: the computer is refreshed at its first quiet moment after the deploy (vmctl's idle test, desk mutations included), not at the next deploy; restart kind planned; exactly one driver attempted it; its booted-image record names the target store path; other sweep passes ran on time during the refresh
      evidence_class: deployed proof
    - action: >-
        LU2. On a minted test computer with queued wakes, actor-log
        occurrences of a changed kind and an active schedule, deploy a
        fixture build that bumps the work-state epoch with a registered
        migration; separately, wake a hibernated test computer onto that
        build; then kill the guest mid-migration once; then boot an older
        fixture binary on the migrated store, and a build whose migration
        fails.
      proves: the planned boot stays planned and a work_state_migrated record names exactly what it closed in both databases; the queued wakes are delivered; the markerless wake runs the migration before closure; the killed migration re-runs on the next boot and the epoch advanced only after it; the downgrade and the failed migration hold the runtime with no admission and no closure, visible on /health
      evidence_class: deployed proof
    - action: >-
        LU1b. Deploy a same-epoch guest change while the live paper is
        busy; do not touch the paper. Read vmctl's update state, the
        paper's restart block, its schedules with generations, its trace
        and its publications.
      proves: >-
        when the paper's work drains within D, it takes the update within
        60 minutes of the deploy (O7) with restart kind planned, 14 of 14
        schedules at their pre-update generation, no run, round or arrival
        closed as interrupted_by_restart, and a story revision within 30
        minutes after the boot; when it does not, the state is visibly
        pending or stuck and nothing was cut. The update-lag distribution
        is reported either way
      evidence_class: deployed proof
    - action: >-
        LU1b. On a minted test computer, a run executing at drain start
        spawns a child on another trajectory, the child spawns a grandchild,
        and results flow back, one queued behind held work in the same
        mailbox; a new capture, a schedule tick and a self-development
        retry for an idle operation arrive during the drain; one
        occurrence's lineage lookup is made to fail.
      proves: the child, grandchild and their results are admitted without delay; the new work, the retry and the failed-lookup occurrence are deferred (not blocked, not poisoned, defer counts unchanged in the poison budget); the run finishes; the boot is planned with the deferred work delivered after it
      evidence_class: deployed proof
    - action: >-
        LU1b. On a minted test computer, a fixture activation outlives the
        drain deadline; separately, the final call is made to fail before
        its 200; separately, vmctl is restarted between the final call's
        200 and the kill.
      proves: the drain times out and admission reopens with no marker; a failed final call leaves no marker, no update_final and no kill; after the vmctl restart the sweep completes the kill and the boot is planned; attempts are recorded and retried; after K timeouts the "update stuck" signal shows
      evidence_class: deployed proof
    - action: >-
        LU3a. A deliberately bad build (fixture: /health never reaches
        ready) with ring 0 = one minted test computer; then a good build.
      proves: the bad build stops at ring 0, is refused for that computer (a refusal record on its chain), halts fleet-wide and alerts, and never reaches the paper, including when the target moves to it mid-drain; the good build passes ring 0, then the paper, then the owner's computer in order, each with a lifecycle_observed update record on its chain
      evidence_class: deployed proof
    - action: >-
        LU3b. A deliberately bad same-epoch, same-store-schema build on a
        minted test computer.
      proves: the computer boots its previous image automatically within T_ready plus one boot, exactly once, as a planned boot; the image is refused; held wakes are delivered after the rollback boot; a bad build that changed the epoch or store schema is not rolled back but held (U7)
      evidence_class: deployed proof
    - action: >-
        LU5. On one test computer, an approved self-development apply and a
        platform update, then a restore to the pinned prior head.
      proves: both use one ReleaseApply record with the same admission, health and rollback; the restore joins event head, content witness, VM-local projection and route
      evidence_class: deployed proof
    - action: >-
        LU4, only if O5 rules it in. A runtime-only change on a minted test
        computer.
      proves: the guest updater swaps the runtime with runtime unavailability of 10 s or less, the Firecracker process unchanged, restart kind planned, and the next VM boot runs the new base with no skew
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
    reaches at most ring 0 before it is stopped.
  goodharting_would_be: >-
    Low lag because updates force busy computers or kill mid-drain; "no
    work lost" because the drain drops or poisons held work instead of
    deferring it; planned boots that are crashes relabelled; "rollback
    works" shown only on a build that never ran work; a fleet "up to date"
    because pending computers are hidden or skipped as divergent.

homotopy:
  realism_axis: >-
    the one update path (vmctl sweep -> guest gate and drain -> final call
    -> boot -> health -> record) exercised on: fixture builds on one minted
    test computer -> ring 0 with real deploys -> the live paper -> the
    owner's computer -> every active computer

boundaries:
  mutation_class: red
  authority_sources:
    - owner 2026-10-14 in session (the framing quoted above; "Yes, this all makes sense as a new station")
    - CLAUDE.md "Restarts End Work (Crash) Or Resume It (Planned Update)", "Deploys never restart a busy computer", Landing Loop "Product restore vs platform deploy"
    - docs/restart-subsystem-design-2026-10-13.md v3 (Invariants R, W, P; §9 "wait, then defer")
    - docs/computer-ontology.md (self-development contract, restore-set boundary)
    - docs/reports/lu-guest-updates-design-2026-10-14.md (round 3)
  must_preserve:
    - a crash restart never resumes work; a planned restart resumes once
    - no automatic path restarts a guest before its final call succeeded; busy is never forced
    - held work is deferred and delivered later, never blocked into a slot, dropped or poisoned; update holds do not spend the poison budget
    - an unreadable or newer work-state epoch, or a failed migration, holds the runtime; nothing runs on state it cannot read
    - a host final-call record exists before every final call, so a vmctl restart never strands a computer
    - schedules (standing intents) survive every update
    - no automatic rollback across a work-state epoch or store-schema version (U7)
    - one automatic rollback per computer and image; a refused image is never re-applied automatically
    - vmctl's sweep is the only update driver, one computer at a time per host
    - the base image is rebuilt on every guest-runtime deploy (a47122de stands)
    - self-development authority stays the owner's decision; a platform update enters only under a platform-signed follow policy and never on a divergent computer
    - pending is derived from booted and target image identity, never stored as a second truth
  excluded:
    - forced updates of busy computers by any automatic path
    - rebasing platform updates onto self-developed (divergent) computers
    - zero-downtime host-service deploys (L1 D3, D4)
    - A/B image slots beyond keeping the previous image for rollback
    - amending CLAUDE.md (O8) inside this station without the owner
  protected_surfaces:
    - deployment routing (CI deploy-refresh step, deploy-impact classification)
    - vmctl (refresh, idle sweep, ownership, retained images, ring state)
    - VM lifecycle and boot (vmmanager launch and refresh, boot overrides)
    - restart semantics (planned marker, work-state epoch, crash closure)
    - run admission (the guest gate, drain and final call)
    - self-development apply (materializer, guest updater, checkpoint and route)
    - the computer event chain (lifecycle receipts)
  admissible_evidence: >-
    deployed proof on staging (minted test computers, then the paper and
    the owner's computer); unit tests and nix eval shape a slice but never
    prove vmctl, boot, restart or update behavior
  slice_rollback:
    LU1a: VM_UPDATE_SWEEP=off; revert; CI's per-computer loop is a revert away
    LU2: revert only before any migration has run (older binaries cannot read the persisted epoch); after that, fix forward. Epoch steps without a migration fall back to D7 (crash boot)
    LU1b: revert; the drain endpoint is additive and unused while the sweep runs without drains
    LU3a: ring gating off returns to LU1 behavior; the skip-unknown-action guest change stays
    LU3b: VM_UPDATE_HEALTH_GATE=off returns to LU3a behavior; retained-image GC roots are harmless
    LU5: revert per commit; record shapes are additive
    LU4: the flag off sends every update down the VM path; a layer is dropped at the next VM boot by the base binding
  dependencies:
    - the station as a whole does not depend on L1 (only LU3b does); LU1a and LU2 need nothing new and are built in parallel; LU2 reaches computers through LU1a's sweep; LU2 lands before C1e's next work-state epoch bump, which does not land until it does (recorded for C1e's own file when C1e next updates)
    - LU1b and LU2 are shared-runtime changes and land between C1e slices, one cutover at a time
    - LU3b needs L1c (host-asserted planned boots) and L1b (grants and the recovery ledger)
    - LU5 needs C1e's Freeze and Verify working on staging and the self-development effects gate
    - the paper's drains finish reliably only once management's work is bounded (delegation design S1 and S2); until then a drain may time out, visibly
  heresy_delta:
    discovered:
      - deferred computers retried only at the next deploy
      - planned-restart endpoint with no admission gate and no sync before the kill
      - a marker that outlives a refresh that never happened
      - epoch changes turning planned refreshes into crashes
      - image refreshes with no health gate, no rollback and no ring order
      - updates absent from the computer's event chain
      - an unknown lifecycle action stopping the guest's receipt processing
      - an unreachable guest read as busy forever by the idle test (a computer stranded after its runtime exits)
    introduced: []
    repaired: []

standing_questions:
  q1_who_settled: >-
    The station and its six-point direction: owner, 2026-10-14 in session.
    The design's defaults (O1-O3b, O5, O7-O9) are orchestrator proposals
    until the owner rules; U7 (forward-only across state) is a technical
    constraint, not a preference.
  q2_conformance: >-
    Conforms to: crash never resumes (U3); deploys never restart a busy
    computer (U2: restart only after the final call; never forced);
    restart design v3 §9 "wait, then defer"; Invariant P (LU3b waits for
    L1c); a47122de (base always rebuilt; LU4 conditional); the ontology's
    self-development contract (owner decision stays the authority;
    divergent computers excluded). Tension named: a drain defers new work
    for up to D, which the CLAUDE.md sentence does not describe; O8 is
    deferred to the owner.
  q3_deletion_citers: >-
    LU1a removes CI's per-computer refresh loop; its citer is the workflow
    contract test, updated in the same commit. LU1b replaces the hibernate
    fence's single flag with reasons; citers are hibernate.go, the actor
    handler and their tests. LU2 replaces the marker's epoch check; citers
    are planned_restart.go, its tests and the problem doc above (annotated,
    not deleted). LU5 folds the materializer's apply hold into the gate; its
    non-test citer is internal/actorruntime/handler.go.
  q4_production_consumers: >-
    deploy-refresh: CI's deploy job (sole caller today; the sweep after
    LU1a). planned-restart endpoint: vmctl deploy refresh. updater.Apply:
    the self-development materializer and the M9a platform-update handler
    (no live producer of offers since a47122de). Hibernate fence: the actor
    handler and S3 hibernate.
  q5_single_authority: >-
    Pending: derived from the booted-image record (the resolved store path
    passed through bootVM and ResumeVM to launchFirecracker) and the host's
    target (the resolved store path of /var/lib/go-choir/guest); image
    identity wins over CI's served-commit reading. Final-call progress: the
    host's final-call record. Update driving: vmctl's sweep only.
    Attempt history: one host ledger (L1b's; interim ownership fields until
    it lands). Boot kind: the guest's consumed marker until L1c, then the
    host's attestation; the host records the guest's verdict. Release
    identity on a computer: its event chain.
  q6_success_artifact: >-
    Each acceptance names the artifact read: vmctl's update state, the
    guest's restart.kind on /api/runtime/obligations, the schedule list
    with generations, the trace (closed runs and arrivals), the
    lifecycle_observed update records on the chain, the Firecracker pid. A
    CI line saying "refreshed" is not proof.
  q7_fate_sharing: >-
    An update fate-shares with vmctl (sweep), the guest runtime (gate,
    drain, final call), the host's image store (target and retained
    images, GC-rooted by LU1a) and corpusd (event chain append). Obligation
    (LU1b): a vmctl restart mid-drain is survived because the drain is
    guest-timed and a repeated request joins it; a vmctl restart between
    the final call's 200 and the kill is completed by the next pass from
    the host's final-call record; a guest runtime restart aborts the drain
    and the host reads it so. Obligation (LU3a): update receipts wait in the platform's
    pending lifecycle receipts until the guest appends them, as start and
    stop receipts do today.
  q8_restart: >-
    Obligations: the drain is in-memory guest state with a deadline and is
    lost on a runtime restart (read by the host as aborted); the marker is
    written only in the final call and cleared on abort; the host's
    final-call record survives a vmctl restart; the work-state epoch is
    persisted in the store, read at open and advanced only after each
    migration step (LU2); interim attempt fields live on
    the host ownership (no fsync until L1b, a named residual); ring state
    is a projection of chain records, rebuilt after a vmctl restart.
  q9_no_ssh: >-
    Today's read paths: /api/runtime/obligations (restart.kind) and the
    product computer routes. Deliverables: update state in `vmctl list`
    (read by CI on the host and by the owner's computer page), drain state
    on /health, halt and un-halt as owner acts through the product API.
    Every acceptance names one of these; none uses SSH.
  registry_hygiene: >-
    Spine station list, docs/mission-graph.yaml, docs/ACTIVE.md and
    docs/doc-authority-manifest.yaml updated. Residual: the other ledger
    stations (L1, L2a, LR) are still absent from the manifest.

now:
  status: working
  slice: >-
    Definition, round 3: the design note and this file are amended from the
    round-2 panel (adjudication S1-S31; round 1 R1-R39). A round-3
    convergent panel on this digest decides `reviewed`. The first build
    slices are LU1a (host-only retry with one driver) and LU2 (persisted
    epoch and migrations).
  source_ref: branch docs/lu-guest-updates-2026-10-14 (round-3 commit) on main cca944f7
  deploy_identity: staging 97d7a573
  candidate: {id: none, state: none, ref: none, base: none, digest: none, scope: []}
  conjecture:
    id: LU-1
    claim: >-
      With intake.labels on, the paper still has quiet moments within an
      hour of most deploys, or drains within D once new work is deferred,
      so a pull sweep with a bounded drain updates it within 60 minutes of
      a deploy without forcing it; and with the epoch persisted and
      migrated (LU2), an update costs the paper no closed work.
    test: >-
      Sample the paper's executing activations, desk pending mutations and
      gate occupancy every 60 s for 6 h through /health and the runtime
      obligations route (quiet-gap distribution; activation durations;
      what is executing at each refusal); then LU1a's and LU1b's
      acceptances on staging
    edge: missing_oracle
    delta_o: the guest reports why it refused (executing run ids, roots, kinds, ages), so a refusal names its cause
    scope_if_supported: the live paper and the owner's computer on staging
    status: proposed
    evidence_refs: [docs/problems/planned-refresh-closes-paper-schedules-2026-10-14.md, docs/evidence/owner-approved-refreshes-2026-10-14.json, docs/reports/lu-guest-updates-adjudication-2026-10-14.md]
  decision:
    what: "slices LU1a and LU2, then LU1b, LU3a, LU3b, LU5; LU4 only if O5 rules it in; defaults of design §6 until the owner rules"
    kind: operational
    status: proposal
    evidence_ref: docs/reports/lu-guest-updates-adjudication-2026-10-14.md
    owner_ratification_ref: "owner 2026-10-14 in session approved the station and its six-point direction; O1-O3b, O5, O7-O9 open"
  belief:
    believed_state: >-
      Deploys push once and defer busy computers; the paper had quiet
      moments before intake.labels and updated only by owner act after;
      an epoch bump makes a planned refresh a crash; image refreshes have
      no health gate or rollback; the guest updater already has health and
      rollback for layered releases; the apply hold already shows how to
      defer work without holding a dispatcher slot.
    main_uncertainty: whether the paper's executing activations finish within D once new work is deferred (activations may run up to 60 min)
    next_observation: the paper's quiet-gap and activation-duration sample
  blocker_or_risk: >-
    LU2 must land before C1e's next epoch bump. LU3b waits on L1c and L1b.
    LU1b's final call is new distributed protocol; its round-3 text is the
    part most likely to draw further findings.
    Until management's work is bounded (delegation S1, S2), paper drains
    may time out; the stuck signal makes that visible rather than forced.
  next_action: run the round-3 convergent panel on this file and the design note at the round-3 commit

receipts: []
---

# LU: guest updates

The design note is the specification; this file tracks its execution.

- **LU1a** pending and retry, host only: vmctl's sweep is the one update
  driver and offers each pending computer its update at every pass, through
  today's planned-restart endpoint. This is also how drain-capable code
  reaches a busy computer.
- **LU2** persisted epoch and migrations: an epoch change runs a
  registered migration on any boot, so a planned update stays planned.
- **LU1b** the drain: new work deferred (never blocked), work already in
  flight admitted by lineage, a synchronous final call, never forced.
- **LU3a** rings and refusal: test computers, then the paper, then the
  owner's computer; a failure halts and alerts; updates recorded on the
  chain.
- **LU3b** probation and rollback: automatic return to the retained
  previous image, same epoch and store schema only.
- **LU5** one apply path: platform updates and approved self-development
  share admission, apply, health, rollback and the record on the chain.
- **LU4** binary-only swap, only if measured downtime warrants it.
