---
definition_version: 4

# Self-development readiness: the platform upgrades the self-development
# loop needs, landed in waves, then the first real loop, run end to end.
# Supersedes docs/definitions/choir-launch-first-pass-metamission-2026-10-21.md
# (its open items carry over by name, station SR6).
# Owner 2026-10-24, in session (ledger 19:3x to 20:1x of the first pass,
# and this file's ledger): "self-development is our priority"; "we should do
# all of the platform upgrades that relate to self-development now. And
# we'll try to do self-development in a few hours after we do these platform
# upgrades"; "we really need to accelerate app packages ... with agentic
# speed, you could probably do it in an hour or two"; "accelerate the one
# home migration for the object graph and file system"; "when we update
# them, we're also updating the platform code's Git history"; "the real
# bottleneck for our development pace is running CI and testing things on
# staging"; "we should do the new meta mission. We should supersede our
# current one and start the new one."
# Authority to execute: the owner's statements above (a named authority
# under skills/throughline/SKILL.md "Readiness"). File review: one Devin
# seat, recorded in `review` when it returns.
readiness: executable

review:
  reviewer: Devin, one seat (bunch 200)
  frozen_ref: the first commit of this file, ca26f13d0
  verdict: accept  # after repair: bunch 200 sent back on three findings (worktree recovery handles, SR6's carried list, the switch-authority invariant), each fixed with the reviewer's text; doc-only, so tier 0 after
  evidence_ref: /Users/wiz/Dev/lanes/_runs/bunch200/devin.out

metamission:
  stations:
    - id: SR0
      path: this file (the orchestrator's own work)
      readiness: executable
      status: working
      depends_on: []
      note: >-
        FP0, the orchestrator (Claude Opus 5.5, this session). It alone
        lands on main, watches CI and the deploy, runs deployed proofs, and
        sends candidates to review by tier. It owns the promote step (gap
        A, a script first), the deploy waves and the switch-on order.
    - id: SR1
      path: docs/design/computer-update-architecture-rnd-2026-10-24.md §4.6 (U1, U1b)
      readiness: reviewed
      status: working
      depends_on: []
      note: >-
        The source follows the base. U1 (lane/fp0-u1-source-projection):
        the guest source is base plus the current applied stack, rebuilt
        from the tape; tier 2 bunch 199: Devin approve, DeepSeek send back
        (an unreadable manifest resets to base; containment test 2 is
        circular); fixes in progress. U1b (lane/fp0-u1b-base-objects):
        each computer receives the booted platform commit's git history,
        so the projection can move to the new base and an accepted change
        retires as upstreamed. Switches per computer, at runtime.
    - id: SR2
      path: docs/design/app-packages-replan-2026-10-24.md
      readiness: reviewed
      status: working
      depends_on: []
      note: >-
        App packages under I27. P1 (lane/fp0-pkg-p1, orange, tier 1):
        in-tree packages, build-time discovery, the check script, the image
        viewer as template. P2a and P2b (lane/fp0-pkg-p2, red): package-
        declared kinds choir.app.<package>.<kind>, one writer
        desk:<package>. Then P3 (convert the other apps, one lane each),
        P4 (the app-desk join), P5 (runtime install). The replan converts
        one app, then the Reader, then the rest the same day; this reorders
        the owner's 2026-10-14 "convert every existing app first", stated
        to the owner 2026-10-24.
    - id: SR3
      path: docs/design/app-packages-replan-2026-10-24.md §5 (the loop's gaps)
      readiness: drafted
      status: working
      depends_on: []
      note: >-
        The loop's gaps, each problem-documented first. In flight: C
        engineering Reports never reach Texture (lane/fp0-reports-texture);
        F a reject refused after a used approval, K the idle sweep
        hibernating a key-driven computer, H WriteFile mangling Svelte with
        backticks (lane/fp0-loop-gaps). Next: A the promote path (FP0, a
        script: an operation's frozen patch to a reviewed lane on main);
        B the owner's preview of a candidate (screenshots through the CLI);
        G the model policy as an owner act. Later: D (the Texture document
        stops at the decision), I (engineering work saved only at
        Complete), J (dead runs stay running), L (cast material beyond
        16 KiB, in the ICRL design).
    - id: SR4
      path: docs/problems/data-has-more-than-one-home-2026-10-24.md (migration order)
      readiness: drafted
      status: working
      depends_on: []
      note: >-
        The one-home migration, accelerated (I27, H038). Step 1, the file
        object (lane/fp0-file-object): choir.file with a digest and a path
        view, written on every files API mutation, behind a switch. Step 3,
        agent_evidence and co_super_slots to one home each
        (lane/fp0-evidence-slots), which also lifts the checkpoint block on
        a computer whose research saved evidence. Step 0 (no new file
        conventions in candidates) binds SR5's brief.
    - id: SR5
      path: this file, finish.acceptance (the first real loop)
      readiness: drafted
      status: pending
      depends_on: [SR1, SR2, SR3]
      note: >-
        The Reader app, one package combining the PDF and EPUB readers,
        requested by a coding agent (Claude) through the choir CLI on a
        test computer, judged by the owner, promoted to main through
        review, deployed, and reaching every computer as the base; the test
        computer's stack then retires as upstreamed. Its reading progress
        uses the existing media progress kind (I27), not a file or browser
        storage.
    - id: SR6
      path: docs/definitions/choir-launch-first-pass-metamission-2026-10-21.md (carried items)
      readiness: executable
      status: working
      depends_on: []
      note: >-
        Carried from the first pass by name: R7 trace log S2 (lane/fp0-r7-s2,
        PR #103; tier 2 bunch 194, Devin approve, DeepSeek pending) then S3
        to S6; D37's decisive proof at the paper's 04:00 UTC compaction;
        jailer wave 2b (the paper and public_platform, alone, at a watched
        planned refresh after D37); more D43 memory readings through the
        02:30 backup and the compaction; the legacy tape delete (46 GB, its
        own window, preconditions first); the idle sweep after an apply
        restart; the status page and the overnight report brought up to
        D44; the computer-event table rollout (its next step names one
        test computer in CHOIR_EVENT_TABLE_COMPUTERS); SEC-10 step 2's fleet
        close (UDP 53 still open off the test class); the held jobpark wired
        test (lane/fp0-jobpark-wired 9d9fdc884); the telemetry.batch lead on
        the paper (unverified); the roadmap recut (lane/fp0-roadmap-recut),
        to close or land.
    - id: T1
      path: none yet (intent)
      readiness: intent
      status: pending
      depends_on: [SR5]
      note: Tent pole 1, apps become app agents with their own RLM desks (I26), through P4 and P5.
    - id: T2
      path: none yet (intent)
      readiness: intent
      status: pending
      depends_on: [SR5]
      note: >-
        Tent pole 2, an agentic red-teaming loop on the sandbox layers
        (docs/reports/boundary-strength-ladder-2026-10-15.md): break, fix,
        break again, each boundary a precommitted claim settled by attack.
    - id: T3
      path: docs/design/computer-update-architecture-rnd-2026-10-24.md U2 to U7 (intent)
      readiness: intent
      status: pending
      depends_on: [SR1]
      note: >-
        Tent pole 3, distribution: divergence kept, security updates forced
        and fast (security, then availability, then customizations; a forced
        fix never contains a computer), the read-only Updates app, propose
        with Playwright evidence once diverged.

start:
  captured_at: '2026-10-24T20:15:16Z'
  source:
    canonical_ref: main@e2d8fca61
    deploy_identity: >-
      staging https://choir.news deployed_commit c72cc2e07 (D44, CI
      40121748442, vmctl active since 18:47:43 UTC); docs-only commits
      since do not deploy
  worktrees:
    - path: /Users/wiz/Dev/go-choir
      status: dirty
      class: user_wip
      owner: unknown (M go.sum, ?? scratchpad/)
      touch: forbidden
      recovery: leave in place; never stage go.sum
    - path: /Users/wiz/Dev/lanes/fp0-u1, fp0-r7-s2
      status: clean
      class: goal_candidate
      owner: FP0's lane workers
      touch: goal_owned
      recovery: pushed branches origin/lane/fp0-u1-source-projection (61d0f2863) and origin/lane/fp0-r7-s2 (f4bc762c6)
    - path: /Users/wiz/Dev/lanes/fp0-u1b, fp0-pkg-p1, fp0-pkg-p2, fp0-loop-gaps, fp0-reports-texture, fp0-file-object, fp0-evidence-slots
      status: dirty
      class: goal_candidate
      owner: FP0's lane workers (in progress at capture)
      touch: goal_owned
      recovery: >-
        unpushed at capture (local commits and dirty files only); each
        worker pushes its lane branch when it reports, and nothing is
        relied on before it is pushed
    - path: /Users/wiz/Dev/lanes/fp0-* (the other first-pass lanes)
      status: unknown
      class: goal_candidate
      owner: FP0 (first pass)
      touch: read_only
      recovery: their pushed branches; pruned only after their items are landed or recorded
  observed_baseline:
    - >-
      Every capsule builds from its computer's creation commit: the guest
      source root is seeded once and never advances
      (docs/problems/self-development-candidates-build-from-the-creation-commit-2026-10-24.md);
      today's v6 and v7 five-app candidates on b@c were rejected for it.
    - >-
      An accepted candidate applies to its one computer; there is no path
      to main or to other computers except FP0 hand-landing its patch.
    - >-
      Apps are compiled into one SPA through a static registry; no app
      package exists on main (docs/design/app-packages-replan-2026-10-24.md §1).
    - >-
      Object graph kinds are compiled into the platform registry
      (internal/objectgraph/registry.go DefaultRegistry); the files tree,
      Mail, agent_evidence and co_super_slots are second homes (H038).
    - >-
      The default API key covers owner acts (D41), so a coding agent can
      approve and reject through the choir CLI on its test computer.

finish:
  deliver: >-
    The owner's self-development loop works end to end: a coding agent asks
    a test computer for a change through the choir CLI; the computer's
    engineering builds it on current main; the owner sees it in Texture and
    approves it; it is promoted to main through review and the landing
    loop; every computer receives it as the new base, and the test
    computer's stack retires as upstreamed.
  artifact: >-
    On main and deployed: U1 and U1b (source = base plus applied stack,
    with the base's history delivered), app packages P1 and P2, the loop
    gap fixes A, B, C, F, H and K, and one-home steps 1 and 3; plus the
    Reader app as a package, written by self-development on b@c and
    promoted to main.
  acceptance:
    - action: >-
        On staging, with the switches on for b@c only: the coding agent
        submits the Reader brief with the choir CLI; engineering's capsule
        builds from b@c's booted base; Freeze and Complete; the candidate's
        reports reach its Texture document; the owner (or the agent with
        the owner's standing authority for b@c) approves; the applied
        Reader opens PDF and EPUB files on b@c.
      proves: a candidate is built on current main, visible to the owner, and applied on the test computer.
      evidence_class: deployed proof
    - action: >-
        Promote: the operation's frozen patch becomes a lane on main, is
        reviewed by its tier, lands, passes CI and deploys; b@c boots the
        new base, its projection retires the Reader operation as
        upstreamed, and a second computer (a fresh test computer) serves
        the Reader from the base.
      proves: an accepted change reaches every computer as the base without divergence on the source computer.
      evidence_class: deployed proof
    - action: >-
        Restore b@c to a checkpoint taken before the Reader apply; the
        projection rewinds; a new candidate builds on the restored stack.
      proves: the loop keeps the rollback point the owner relies on.
      evidence_class: deployed proof
  rollback: >-
    Every slice ships switched off with a per-computer runtime switch; turn
    the switch off for the computer. Code rollback is git revert of the
    slice's commits. A promoted change reverts on main like any change.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the hand steps (owner and FP0) and the wall time from a
    request to every computer running the accepted change, while keeping
    the authority boundaries (I1, I26), one home per kind of data (I27),
    and the four S's: secure, scalable, stable self-development.
  goodharting_would_be: >-
    FP0 writing or fixing the candidate by hand and calling it
    self-development; a candidate accepted on a computer whose source is
    not current main; counting landed slices or deploys instead of running
    the loop; switches left on fleet-wide without a per-slice proof.

homotopy:
  realism_axis: >-
    From one frontend-only package change on one test computer, promoted by
    an FP0 script, through Go changes built by the host, to packages
    installed at runtime and app agents, to computers that keep divergence
    across platform updates (T3).

boundaries:
  mutation_class: red
  authority_sources:
    - the owner's statements in session (this file's header and ledger)
    - docs/choir-doctrine.md (I1, I25, I26, I27)
    - CLAUDE.md (mutation classes, review tiers, landing loop, problem documentation first)
  must_preserve:
    - crash restarts close work; only a planned-restart marker resumes it
    - Texture is the sole agent writer of documents; desks reach each other only by semantic act
    - one home and one registered writer per kind of data (I27)
    - the paper and the owner's computers are never used for adversarial or proof steps
    - security first, availability second, customizations third
    - a switch is enabled by FP0 (owner-delegated, 2026-10-24) or the owner, through an owner act or a host selector, never from a guest; FP0 turns proved switches on promptly and aggressively, test computers first, the paper and public_platform last
  excluded:
    - the ICRL build and Autopaper forecasting (designed; built after SR5)
    - owner-act enforcement (deprioritized by the owner)
    - node-a changes
  protected_surfaces:
    - boot path and tape projection (U1, U1b)
    - VM lifecycle and the source seed (U1b)
    - object graph write gate and store schema (P2a, one-home steps)
    - run acceptance and Texture canonical writes (gap C)

now:
  status: working
  slice: >-
    Wave 1: land U1 and U1b, P1 and P2, the loop gaps C, F, H, K and
    one-home steps 1 and 3, switched off, in as few deploys as the
    switched-off rule allows; then turn each on for b@c one at a time and
    prove it. In parallel, SR6's carried items.
  source_ref: e2d8fca61
  deploy_identity: c72cc2e07 (D44)
  candidate:
    id: wave-1
    state: rehearsing
    ref: lanes U1, U1b, P1, P2, loop-gaps, reports-texture, file-object, evidence-slots
    base: e2d8fca61
    digest: none until frozen per lane
    scope: [internal/sourceproj, internal/agentcore, internal/capsule, internal/vmmanager, frontend/src/packages, internal/objectgraph, internal/apppackage, internal/autoputer, internal/store]
  bridge_conjecture: >-
    If the wave-1 slices land with their per-computer switches proved on
    b@c, then the Reader brief submitted through the choir CLI produces a
    candidate built on current main that the owner can judge and FP0 can
    promote without hand edits; so the next changes are self-developed,
    not hand-built. It fails if the Reader candidate needs FP0 to edit its
    patch, or if a computer that boots the promoted base still reads as
    diverged.
  conjecture:
    id: C-SR-seed
    claim: >-
      The creation-commit seed (with the missing base history) is the main
      reason today's candidates were unusable; with U1 and U1b on, a
      candidate's patch applies to current main unchanged.
    test: >-
      On b@c with U1 and U1b on, a candidate's frozen bundle names b@c's
      booted base as its base commit, and its patch applies cleanly to
      origin/main at promotion time.
    edge: missing_oracle
    delta_o: the projection facts on b@c's /health and the bundle's base_commit, read at Freeze
    scope_if_supported: frontend-only candidates on a computer with an empty or upstreamed stack
    status: active
    evidence_refs:
      - docs/problems/self-development-candidates-build-from-the-creation-commit-2026-10-24.md
  decision:
    what: >-
      switched-off slices share a deploy wave; switches are per computer and
      runtime; one slice is turned on at a time; a slice whose code is not
      inert when off ships alone
    kind: operational
    status: settled
    evidence_ref: first-pass ledger 20:0x (owner: CI and staging are the bottleneck)
    owner_ratification_ref: not_applicable
  belief:
    believed_state: >-
      D44 is live; U1 is built and in fix after tier 2 review; the other
      wave-1 lanes are being built; the paper runs normally; D37's proof is
      due at 04:00 UTC.
    main_uncertainty: >-
      Whether U1b can deliver the base's git history through an existing
      channel without a new host service.
    next_observation: U1b's investigation report.
  blocker_or_risk: wave-1 lanes are unpushed until their workers report; nothing is relied on before it is pushed
  next_action: >-
    Land U1's fixes after a one-seat re-review; collect U1b, P1, P2 and the
    gap lanes as they finish; mirror and review each by tier; freeze wave 1.

receipts: []
---

# Self-development readiness

This metamission replaces the launch first pass. The first pass was a
launch checklist: security, density and the store, landed one deploy at a
time (deploys D17 to D44). Its closing receipt and the items it carries
over are in its own file. This one has a single acceptance: the owner's
self-development loop, run end to end on the Reader app.

The tent poles after it, in the owner's order: apps become app agents
(T1), an agentic red-teaming loop on the sandboxes (T2), and distribution
with forced security updates (T3). Then the focus returns to Autopaper, with
prospective self-alignment scoring it first
(docs/design/tasks-precommit-icrl-2026-10-24.md), then Autoradio.

Ledger: `choir-selfdev-readiness-metamission-2026-10-24.ledger.md`, beside
this file.
