---
definition_version: 4

# The launch first pass: every agent-doable batch of the launch checklist,
# built concurrently and landed in order, then reviewed with the owner.
# Owner 2026-10-21, in session: "zoom back out into our nested checklist so
# that we can set a really aggressive meta mission to really bang out this
# work promptly with as much concurrency as possible. I mean, of course,
# security is paramount, but I think the most effective approach is to do it
# all as aggressively as we can. I think we have like a 5 to 7 concurrent
# agent limit on our 16 gigabytes of RAM right now." Then: "Don't plan for
# the second pass. I want you to work autonomously for the first pass, and
# then I will review that with you, and then we'll make a new plan for the
# second pass." Then (owner notes §38): "Don't worry about iPhone or voice.
# Just let's stick with the security and density checklist"; "we can make
# progress with self-development as well ... maximizing our acceleration";
# "the rule is 5 to 7 agents running locally. We can run more agents on
# node B and as many as we have resources for." The batch plan it runs is
# docs/launch-checklist-2026-10-21.md "Batch plan"; the security and
# density items are docs/security-density-architecture-checklist-2026-10-21.md.
readiness: drafted

review:
  reviewer: none yet; a convergent round (Gemini 4 Argon, GPT-6.1-sol, Devin read-only) runs on this file's first commit
  frozen_ref: none
  verdict: none
  evidence_ref: none

metamission:
  stations:
    - id: FP0
      path: this file (the orchestrator's own work)
      readiness: drafted
      status: working
      depends_on: []
      note: >-
        The orchestrator (Claude Opus 5.5, this session). It alone pushes to
        main, watches CI and the deploy, uses Node B, runs deployed proofs,
        and sends red candidates to a panel. Its own items: B1 (time each
        landing: queue, build, image, refresh, proof; script the staging
        proof), L1-02 (measure the paper's memory: daemon RSS, per-desk
        workers, page cache), and the capture-label read (c893a956a).
    - id: FP1
      path: docs/definitions/choir-launch-fp1-store-throughput-2026-10-21.md (the lane's first commit)
      readiness: intent
      status: pending
      depends_on: []
      note: >-
        Batch B2: L1-01, SDC RES-18, L3-05. The three slow JSON reads that
        remain (run agent_id, record addressee, source_ref doc_id) move to a
        rollback-safe side table built online: indexes created while it is
        empty, a batched backfill with progress logs, reads on the JSON path
        until the table is complete and its rows validated against
        content_hash. Then boot-path progress logs, then engine concurrency
        so desk slots can rise above 8. The panel findings on the first
        design are in docs/problems/stored-column-add-blocks-paper-boot-2026-10-21.md.
        Lane worktree lanes/b2-store, branch lane/b2-store.
    - id: FP2
      path: docs/definitions/choir-launch-fp2-desk-boundary-2026-10-21.md (the lane's first commit)
      readiness: intent
      status: pending
      depends_on: []
      note: >-
        Batch B3, red: SDC ARCH-02 (the desk manifest, L1-04), ARCH-05,
        ARCH-07, SEC-01, SEC-02, SEC-13, SEC-15, RES-01 to RES-04. The
        manifest is the one table that confinement, role checks and budgets
        read. Proof: forged-frame, escape and out-of-memory probes fail
        safely, and real desk turns still work. Lane worktree
        lanes/b3-desk-boundary.
    - id: FP3
      path: docs/definitions/choir-launch-fp3-host-floor-2026-10-21.md (the lane's first commit)
      readiness: intent
      status: pending
      depends_on: []
      note: >-
        Red, host. First the two vmctl defects behind today's outage
        (docs/problems/stored-column-add-blocks-paper-boot-2026-10-21.md):
        the 24-hour wait before a failure-stopped always-on computer is
        retried (now two outages; 2026-10-15 item 2), and a rollback deploy
        that cannot refresh a protected computer that is unhealthy outside
        the registry. Then batch B4 (SDC SEC-03, SEC-09, SEC-10, NET-07),
        then B5 (REC-02, SEC-11, SEC-12). Built together, landed one batch
        per deploy. Lane worktree lanes/host-vmctl-b4-b5.
    - id: FP4
      path: docs/definitions/choir-launch-fp4-selfdev-architecture-2026-10-21.md (the lane's first commit)
      readiness: intent
      status: working
      depends_on: []
      note: >-
        The self-development items of the security and density checklist:
        ARCH-10 (its launch half is L3-08: engineering admission queues
        instead of cancelling live work; then N capsules per assignment
        with apply serialized, which lets the loop build several things at
        once), ARCH-04 (one writer per object kind), ARCH-03 (an app
        package carries its app desk). Rescoped 2026-10-21 (owner notes
        §38): the S6 slices are not on the checklist and wait. Lane
        worktree lanes/b10-selfdev.
    - id: FP5
      path: none
      readiness: intent
      status: superseded
      depends_on: []
      note: >-
        Superseded 2026-10-21 (owner notes §38: no iPhone or voice work in
        this pass); its worker moved to FP6. Was: the server half of batches B11 and B12. L6-02 first (the
        capabilities endpoint and an OpenAPI contract, frozen so the app can
        build against it); then the L4-04 security design for Sign in with
        Google and Apple, reviewed by a panel before any code (red: auth
        and session); then L6-01 (mobile redeem with device-bound tokens)
        and L6-03 (the supervision feed with a durable cursor). Lane
        worktree lanes/b11-mobile-server.
    - id: FP6
      path: docs/definitions/choir-launch-fp6-network-2026-10-21.md (the lane's first commit)
      readiness: intent
      status: working
      depends_on: []
      note: >-
        Batch B8's first half, red: the Obscura deletion (L2-06 steps 1 to
        4; step 5, the image change, is a separate commit and deploy), SDC
        NET-01's guest network inventory, then SEC-06 (P0). Lane worktree
        lanes/b8-network.
    - id: FP7
      path: docs/definitions/choir-launch-fp7-gateway-identity-2026-10-21.md (the lane's first commit)
      readiness: intent
      status: working
      depends_on: []
      note: >-
        Batch B7, red: SDC SEC-05 (the gateway bearer bound to its
        computer), SEC-08 Option K (the gateway holds the delegate's key),
        RES-11, then SEC-16 and RES-12. With SEC-01 (FP2) it is what lets
        agents run on Node B beyond the one delegate test computer. Lane
        worktree lanes/b7-gateway.
    - id: FP8
      path: to be authored when a slot frees
      readiness: intent
      status: pending
      depends_on: [FP3]
      note: Batch B6, red. SDC RES-05 then SEC-04, NET-08 then RES-16, RES-06, RES-09, RES-14; L4-03.
    - id: FP9
      path: to be authored when a slot frees
      readiness: intent
      status: pending
      depends_on: []
      note: Batch B9. SDC NET-06, NET-09; L2-04.
    - id: FP10
      path: to be authored when a slot frees
      readiness: intent
      status: pending
      depends_on: []
      note: >-
        The rest of the checklist when a slot frees: SEC-14 (CI deploys as
        root, with FP0's B1), RES-07 and RES-19 (FP0 measures), RES-17,
        RES-18 (with FP1), and the P2 items (NET-03, NET-04, RES-08, RES-10,
        RES-13, RES-15, RES-20 to RES-22, ARCH-06, ARCH-08, ARCH-09,
        ARCH-11 to ARCH-15), each decided item by item. Autopaper, accounts
        and Autoradio items are out of this pass (owner notes §38).

start:
  captured_at: 2026-10-21T17:50:00Z
  source:
    canonical_ref: origin/main c893a956a (capture labels read by key)
    deploy_identity: >-
      staging serves f2c4a3ec3 (the revert of the stored columns); the
      paper booted 17:10:07 UTC and had polled its sources 24 times by
      17:28:44. The deploy of c893a956a is in flight.
  worktrees:
    - path: /Users/wiz/go-choir
      status: dirty
      class: other_agent_wip
      owner: unknown (untracked sourcecycled, ograph-bench, cmd/desktop/frontend/)
      touch: forbidden
      recovery: leave in place
    - path: /Users/wiz/go-choir/.claude/worktrees/agent-afa78ad83c0aab149
      status: unknown
      class: other_agent_wip
      owner: another agent (branch ops/patch-lanes)
      touch: forbidden
      recovery: leave in place
    - path: scratchpad/backup-wt
      status: clean
      class: goal_candidate
      owner: FP0
      touch: goal_owned
      recovery: branch ops/offhost-backup-path tracks origin/main
    - path: scratchpad/lanes/{b2-store,b3-desk-boundary,host-vmctl-b4-b5,b10-selfdev,b11-mobile-server}
      status: clean
      class: goal_candidate
      owner: FP1 to FP5, one worker each
      touch: goal_owned
      recovery: branches lane/<name>, created from c893a956a

finish:
  deliver: >-
    Every launch-checklist batch that agents can do without the owner or a
    physical iPhone is either landed on staging with its deployed proof, or
    stopped at a named blocker with the smallest next probe; and the owner
    has one first-pass report to review.
  artifact: >-
    For each station FP1 to FP10: pushed commits on main, a green CI run, the
    deployed sha seen on staging, and the station's deployed proof; or a
    blocked_incomplete card naming the blocker. The launch and security
    checklists ticked to match. A first-pass report (reporter letter) for
    the owner's review.
  acceptance:
    - action: per station, the deployed proof its goal file names, run by FP0 against https://choir.news
      proves: the batch's claim on the deployed product, not on a worktree
      evidence_class: deployed proof
    - action: the paper's schedules and polls read after every deploy
      proves: no batch took the paper down
      evidence_class: deployed proof
    - action: an independent panel on every red candidate's frozen diff before it lands
      proves: no worker verified its own red work
      evidence_class: review receipt
  rollback: >-
    Each batch is one rollback unit: git revert of its commits, landed by
    the Landing Loop. A deploy that hangs a protected computer is reverted
    first and repaired second (the 2026-10-21 outage).
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the agent-doable pre-launch batch items not yet landed and
    proved on staging, while preserving every security boundary, the
    paper's availability, and the restart rules.
  goodharting_would_be: >-
    Boxes ticked on merged code with no deployed proof; many unproved small
    landings; a worker checking its own red work; red changes landed
    without ceremony; or a boot-path migration that takes the paper down.

homotopy:
  realism_axis: >-
    A candidate in a lane worktree with focused local tests, then a frozen
    and reviewed candidate, then landed with a deployed proof on staging,
    then a day of the paper's real load on it.

boundaries:
  mutation_class: green (this file); each station names its own, most are red
  authority_sources:
    - owner in session 2026-10-21 (the quotes above), and owner notes §37
    - docs/choir-doctrine.md and CLAUDE.md
    - the launch checklist's batch plan and owner-decision defaults OD-01 to OD-09
  must_preserve:
    - Locally five to seven agents in all (owner, 2026-10-21): six Sonnet workers and the orchestrator. Workers work only in their own lane worktree and branch; they never push, never use Node B or staging credentials, never read secrets, never touch another worktree, never run git commit -a, never bypass hooks.
    - Only FP0 lands. A deploy carries at most one red batch; orange and green work may ride with it.
    - Problem documentation first; the test before the code (CLAUDE.md Testing).
    - No boot-path step that rewrites or scans all of og_objects; schema changes are online and rollback-safe.
    - No boot-time resumer without a planned-restart marker.
    - Self-development and tracers only on test computers or the allowlisted delegate computer.
    - The standing security rules of this session (no secrets printed, no SQL against production Dolt, no tape edits, no guest block-device access).
    - Broad Go test runs go through scratchpad/bin/heavy (two slots machine-wide) with GOMAXPROCS=2 GOFLAGS=-p=2.
  excluded:
    - The second pass; the owner plans it after reviewing this one.
    - Everything outside the security and density checklist (owner notes §38), including the iPhone app (B13), the voice pipeline (the owner's own project in ~/mike), accounts and Autoradio's server (B11, B12), Autopaper phase 5, and the S6 slices. Xcode is installed (corrected 2026-10-21; xcodebuild failed only because the Nix shell's SDK is the selected developer directory).
    - Owner items (B0 lead times, OD rulings); L2-05 red teaming (needs fork re-keying and the vault); running an account deletion (black).
    - Agents on the Node B host. Owner 2026-10-21: "We can run more agents on node B and as many as we have resources for." They run inside computers, never as host processes (the host holds every tenant and the secrets). Today that is the one Claude delegate test computer (SDC DEC-03); SEC-01 (FP2) and SEC-08 Option K (FP7) unlock more.
  protected_surfaces: [vmctl, guest desk confinement, gateway and provider calls, auth and session, the object store schema, deployment routing]

now:
  status: working
  slice: FP1, FP2, FP3, FP4, FP6 and FP7 in parallel; FP0 lands c893a956a and reviews this file
  source_ref: c893a956a
  deploy_identity: f2c4a3ec3 serving; c893a956a deploying
  candidate:
    id: none
    state: none
    ref: none
    base: c893a956a
    digest: none
    scope: []
  conjecture:
    id: C-FP-landing
    claim: >-
      In the first pass the bottleneck is landing, not building: five lanes
      produce candidates faster than serialized red deploys (about 25
      minutes each plus about 30 for the proof) can land them.
    test: >-
      B1's timing per landing, and the queue of frozen candidates waiting
      for a deploy slot at each FP0 pass.
    edge: resource
    delta_o: time each landing's queue, build, image, refresh and proof (B1), and script the staging proof
    scope_if_supported: the first pass's landing order and batch sizes
    status: active
    evidence_refs: []
  decision:
    what: six lanes now (FP1 to FP4, FP6, FP7); FP8 to FP10 fill slots as they free
    kind: operational
    status: settled
    evidence_ref: owner in session 2026-10-21
    owner_ratification_ref: not_applicable
  belief:
    believed_state: >-
      The paper is up and publishing on f2c4a3ec3; the store's capture-label
      scan is gone at c893a956a; every other batch is open.
    main_uncertainty: >-
      whether six lanes on 16 GB stay inside memory while they run Go
      builds: at 18:15 the Mac already had 15.7 GB of swap in use and
      about 4.9 GB in the compressor, before the first build wave.
    next_observation: the Mac's memory pressure and the heavy-slot wait times once all five lanes are building.
  blocker_or_risk: >-
    Two red batches waiting for one deploy slot; a worker straying outside
    its lane; the paper's boot path (any store schema change).
  next_action: start the five lane workers and the panel on this file.

receipts:
  - id: capture-label-key-read
    boundary: implement
    identity: c893a956a
    proof_refs: [internal/store/capture_label_key_read_test.go]
    rollback_ref: git revert c893a956a
    disposition: pushed; CI and deploy pending
---

# The launch first pass

This is the outer mission over the launch checklist's batch plan. It runs
the batches that agents can do on their own, as many at once as the Mac
allows, and stops when each is landed or honestly blocked. Then the owner
reviews the whole pass and plans the second one. This file does not plan
the second pass.

**How the lanes work.** Each lane is one worker in its own worktree, on its
own branch. Its first commit is its station goal file and, where the
problem is not yet written down, the problem record. Then tests, then code.
When a batch is ready, the worker freezes it and reports to FP0. FP0 sends
a red candidate to a panel, rebases it onto main, lands it with the Landing
Loop, and runs its deployed proof. A failed proof goes back to the lane
with the evidence.

**Why landing is the constraint.** Building is parallel. Landing is not:
each red batch gets a deploy of its own, so the order of landing is the
order of the batch plan, and B1 measures where each landing's time goes.
