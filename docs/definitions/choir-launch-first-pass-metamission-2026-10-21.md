---
definition_version: 4

# The launch first pass: the security and density checklist, built
# concurrently and landed in order, then reviewed with the owner.
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
# Authority to execute: the owner's statements above (an owner statement is a
# named authority under skills/throughline/SKILL.md "Readiness"). Review
# round 1 ran on git blob 9a813197 (commit 26c4e1ed0), before the owner's
# rescope: GPT-6.1-sol send_back, Gemini 4 Argon accept with edits, Devin
# failed (a tool call needing confirmation). Adjudication is the section
# "Review round 1, adjudicated" below; round 2 runs on the repaired file.
readiness: executable

review:
  reviewer: agentic-consensus round 2 (omp-gpt61-sol, omp-gemini38 pinned to gemini-4-argon-high)
  frozen_ref: git blob 6225eee4c17250795c3268fe185bbc8572132a7b (this file at cf9751f36)
  verdict: accept
  evidence_ref: >-
    scratchpad/panel-fp/run-r2 (session diagnostics). Argon accepted;
    GPT-6.1-sol sent back with two bounded repairs, each given as exact
    text (the landing paragraph's order; the finish contract's
    settlement), applied verbatim with its three non-blocking suggestions.
    Round 1 (blob 9a813197, send_back) is adjudicated below.

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
        and sends tier 1 and tier 2 candidates to review. Its own items: B1 (time each
        landing: queue, build, image, refresh, proof; script the staging
        proof) with SDC SEC-14 (CI deploys as root with host-key checking
        off), SDC RES-07 and RES-19 (measure the paper's memory: daemon
        RSS, per-desk workers, page cache; before and after), the live
        dual-model census for L3-05, and the capture-label read
        (c893a956a, landed).
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
        RES-11, then SEC-16 and RES-12. Option K moves the owner's Claude
        token off the guest credential disk; with SEC-01 (FP2) it is the
        precondition for the engineering desk's delegate beyond its one
        test computer. Lane worktree lanes/b7-gateway.
    - id: FP8
      path: to be authored when a slot frees (the first slot that frees)
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
        RES-18 (with FP1), the second half of B8 (NET-01's host fetch
        service, NET-02, NET-05, NET-10, closing general guest egress; it
        follows FP6), and the P2 items (NET-03, NET-04, RES-08, RES-10,
        RES-13, RES-15, RES-20, RES-22, ARCH-06, ARCH-08, ARCH-09,
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
    # Recaptured 2026-10-21 22:45 UTC after the owner's Mac restart and
    # cleanup (owner notes §39); the lane worktrees under /private/tmp were
    # lost with the restart and every lane branch survived.
    - path: /Users/wiz/Dev/go-choir (also reached as ~/go-choir)
      status: clean
      class: goal_candidate
      owner: FP0
      touch: goal_owned
      recovery: main tracks origin/main; every local branch bundled to node-a /root/choir-lane-backups
    - path: lane worktrees, recreated by FP0 from branches lane/<name> (on the Mac or in the node-a agents VM)
      status: absent until recreated
      class: goal_candidate
      owner: FP1 to FP4, FP6, FP7, FP9, one worker each
      touch: goal_owned
      recovery: branches lane/b2-store, lane/b3-desk-boundary, lane/host-vmctl-b4-b5, lane/b10-selfdev, lane/b8-network (and stage/fp6-batch1), lane/b7-gateway, lane/b9-keys

finish:
  deliver: >-
    Every item of the security and density checklist that agents can do
    without the owner is either landed on staging with its deployed proof,
    or stopped at a named blocker with the smallest next probe; and the
    owner has one first-pass report to review.
  artifact: >-
    For each station FP1 to FP10: pushed commits on main, a green CI run, the
    deployed sha seen on staging, and the station's deployed proof; or a
    blocked_incomplete card naming the blocker. The launch and security
    checklists ticked to match. A first-pass report (reporter letter) for
    the owner's review. The report covers every station, including FP0 and
    explicitly superseded FP5, with its disposition and evidence. The
    metamission settles complete only when every station is complete or
    explicitly superseded. If unresolved blockers remain, deliver the owner
    report with aggregate status checkpoint_incomplete or
    blocked_incomplete, preserving each blocker and smallest safe next
    probe.
  acceptance:
    - action: per station, the deployed proof its goal file names, run by FP0 against https://choir.news
      proves: the batch's claim on the deployed product, not on a worktree
      evidence_class: deployed proof
    - action: the paper's schedules and polls read after every deploy
      proves: no batch took the paper down
      evidence_class: deployed proof
    - action: independent review, by risk tier, on every tier 1 and tier 2 candidate's frozen diff before it lands, run in bunches
      proves: no worker verified its own high-risk work
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
    Load and adversarial depth on the same deployed paths: from focused
    tests on a candidate, to the paper's real traffic after the deploy, to
    adversarial probes (forged frames, escapes, cross-tenant and metadata
    destinations, out-of-memory) run by FP0 against named disposable test
    computers, to a day of the paper's load.

boundaries:
  mutation_class: green (this file); each station names its own, most are red
  authority_sources:
    - owner in session 2026-10-21 (the quotes above), and owner notes §37
    - docs/choir-doctrine.md and CLAUDE.md
    - the launch checklist's batch plan and owner-decision defaults OD-01 to OD-09
  must_preserve:
    - 'Locally five to seven agents in all (owner, 2026-10-21: "the rule is 5 to 7 agents running locally"): six Sonnet workers and the orchestrator. Workers work only in their own lane worktree and branch; they never push, never use Node B or staging credentials, never read secrets, never touch another worktree, never run git commit -a, never bypass hooks.'
    - Workers in the node-a agents VM (owner notes §39) follow the same rules, with one exception: they push their own lane branch to GitHub so FP0 can land it, and never push main. Sonnet for plan-linear or metric work, Opus where the design is open. The VM's slots do not count against the Mac's five to seven.
    - "Only FP0 lands. A deploy carries at most one live red batch; green work and inert red batches may ride with it (owner, 2026-10-22). A red batch is inert when, with its switches as deployed, it changes no running path: a new unwired package, or a feature off unless a selector names a computer, and no schema, boot-path, gateway, auth or deploy-routing change. Inert batches deploy together under one no-effect proof (the paper healthy and polling, routes and observability flags unchanged, boot logs clean); a failure reverts the inert set as a unit. Turning inert features on is a later deploy that may enable several at once, each on its own test computer, with their proofs in parallel. FP1's store batch counts as red for landing (the store schema is a protected surface) and gets a deploy of its own."
    - 'The landing order: rebase the candidate onto current main, run focused checks, freeze base, scoped paths and digest, independent review on that frozen identity, check it is unchanged, land, deployed proof. Review is risk-tiered and Devin is the go-to reviewer (owner, 2026-10-23 UTC: "it feels silly to run 3 models for code review for this... once we tested it with sonnet we didnt need review. we should be more judicious, allocating code review to hard stuff and trusting that we can handle the easy stuff cold"; "make devin the go-to reviewer. i have high limits with devin."). Cheap checks run before any review, every time: the repo gates (check-v1-writer-purity, check-decode-roots, check-test-hang-guards, selftest-guest-token), a nix eval of both systems when nix or the flake changed, a dry run of every touched proof script, the CI mirror, and a rebase onto the current deploy tree. The tiers are a default and FP0 judges by context. Tier 0, no model review: green and yellow changes by default (an architectural design (a new mechanism, a doctrine change, a design a build will follow) is reviewed like the change it designs), proof tooling, mechanical rebases, and fixes that answer an earlier finding with only doc, nit or test-only items; the checks are the review. Proof, rollback and procedure text for a red deploy is not tier 0: it gets the deploy''s own review. Tier 1, one seat, Devin: orange changes, and red changes that are additive, behind a data/config switch, with a one-act rollback that shows on staging within minutes. Tier 2, two seats (Devin and DeepSeek v4.1 flash; Gemini 3.8 flash added no unique blocking find after bunch 28): red where a defect would be silent or irreversible: deletes, tape/evidence, Texture canonical writes, auth/session, store schema, boot path, new concurrency. A red delta is re-reviewed, one seat (Devin), whenever the previous round found a blocking-real defect, and not when it found only doc, nit or test-only items; after three rounds on one mechanism, stop adding rounds and look at the substrate. Station files that say "the bunch panel" or "a panel" mean this tiered review. The allocation follows the measured review yield (docs/evidence/review-yield-2026-10-23.md, confidence medium). Review runs in bunches (owner, 2026-10-21 night, "i dont think we have enough tokens (or clock time) to run it after each station. should do it in bunches"), one run reviews every tier 1 and tier 2 candidate frozen at that moment, each with its own verdict. Before 2026-10-23 the landing panel was Gemini flash and Devin, with DeepSeek v4.1 Flash (opencode-go/deepseek-v4.1-flash, owner 2026-10-22 about 03:55 UTC) as a third reviewer and MiniMax M3.1 Flash (opencode-go/minimax-m3.1-flash-free, owner about 03:10 UTC: "another code review model") advisory only ("don''t be blocked by Minimax") (owner, 2026-10-22 02:10 UTC: "I don''t think we should be bottlenecked by SOL ... it''s more important that we get a lot of work done pretty well, and we''re going to discover flaws just from running it, testing it, QA ... we can still use Gemini and Devin"); a tier 1 or tier 2 candidate lands on its reviewers'' accept (blocking findings repaired), a tier 0 candidate on its checks, and both on a passing Linux -race mirror run on the agents VM and FP0''s own read. GPT-6.1-sol is not used for review for the rest of the overnight run (owner, 2026-10-22 02:09 UTC: "let''s stop using sol for code review for the overnight run. I''m using the same tokens for chatgpt gpt-6-Luna… I want to pace my usage"); the same ChatGPT quota serves the gateway''s gpt-6-luna; the per-station settle panel is replaced by one end-of-pass panel over the landed candidates and their deployed evidence.'
    - An activation lease: from a deploy to the end of its proof (or its rollback), no other deploy and no drill.
    - Recovery first: FP3's group 1 (a rollback can always reach a protected computer, and a failure-stopped always-on computer retries after a deploy) lands before FP1's store changes, and before any other boot-path risk.
    - Workers never run git fetch, gc, prune, stash, tag, remote, update-ref or config, any worktree command, or any gh command; never read the rest of the scratchpad (it holds credential files); never edit ACTIVE, the mission graph, the doc authority manifest or the two checklists (FP0 ticks them at landing). Red code fails closed. Live adversarial probes are FP0's.
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
    - 'Lane work on Node B. Owner 2026-10-21: "We can run more agents on node B and as many as we have resources for", then: "I don''t want the Claude on Node B to just be doing arbitrary work and making PRs. Instead, we should reserve it for actual work within the scope of the engineering desk." The delegate does engineering-desk self-development only, never lane work; agents never run as Node B host processes. FP7''s SEC-08 Option K changes how the delegate''s key is held, not who uses it.'
    - Node A's own config and services. Since owner notes §39 (2026-10-21, night) lane workers may run in the Choir agents VM there (`ssh choir-agents`, 8 vCPU, 10 GiB), a scratch machine set up by the owner through the other project's config (docs/node-a-shared-host.md). Choir never changes node-a's config or touches its other services; a change to the VM goes to the owner.
  protected_surfaces: [vmctl, guest desk confinement, gateway and provider calls, auth and session, the object store schema, deployment routing]

now:
  status: superseded  # 2026-10-24 20:1x, by docs/definitions/choir-selfdev-readiness-metamission-2026-10-24.md (owner: "We should supersede our current one and start the new one"); terminal receipt fp-terminal-superseded below. The rest of this card is the last working state, kept as written.
  slice: >-
    Overnight platform groundwork (owner 2026-10-23 05:0x: "ok, restarted
    and ready to go overnight!"): land the live-red queue one deploy at a
    time, each with its deployed proof; store schema, boot path,
    auth/session and deploy routing each keep a deploy to themselves.
    Resource use ahead of short-term uptime (owner 2026-10-22). The paper
    runs half Synthetic DeepSeek V4.1 Flash, half ChatGPT luna, no fallback
    (owner 2026-10-23: "I'd rather them fail than fallback"). Reviews are
    risk-tiered, Devin by default; tier 2 adds DeepSeek (CLAUDE.md, Mutation
    Classes). The owner takes the agenda in the morning; the roadmap re-cut
    (lane/fp0-roadmap-recut) waits for that review and does not land before.
  source_ref: 140f3e5f5
  deploy_identity: 140f3e5f5 serving (deploy D16, CI 39951016230, proved 12:26 to 12:28 UTC)
  candidate:
    id: owner-agenda-2026-10-23
    state: no live candidate; D13 to D16 landed and proved overnight; the next acts wait for the owner's agenda (13:00 UTC)
    ref: the owner's morning decisions
    base: 140f3e5f5
    digest: the staged tree plus the ledger and this card
    scope: [docs]
  bridge_conjecture: >-
    If the security and density batches land with deployed proofs and
    independent review, then the owner's second pass starts from a computer
    whose desks are confined, whose network and gateway refuse other
    tenants, and whose store and memory leave room for more desks; so the
    launch's remaining work is the app and owner items, not foundations.
    It fails if the pass ends with most security items still open, or if a
    landed item needs reworking at the second pass.
  conjecture:
    id: C-FP-boot-reads
    claim: >-
      The paper's planned-refresh /health stalls are boot and first-call
      reads that walk a whole kind or owner under the engine mutex; the
      stream_seq seed was the first (104 s at D9's boot), and the next ones
      have the same shape.
    test: >-
      After D13, the paper's boot shows no ListObjects choir.event op and
      the first minute of /health has no 503; the boot-read census
      (lane/fp0-boot-read-census) ranks the per-run seq seed,
      ListEventsByOwnerAfter and the boot run-header list by measured hold.
    edge: resource
    delta_o: hold times per boot read on the paper's console at each planned refresh
    scope_if_supported: the next store deploys after D13
    status: active
    evidence_refs: [docs/problems/stream-seq-seed-scans-the-event-kind-at-boot-2026-10-23.md]
  decision:
    what: overnight lanes are platform groundwork only (storage, memory, security, boot stalls); no new high-level design before the owner's morning agenda
    kind: operational
    status: settled
    evidence_ref: owner in session 2026-10-23 ("do this groundwork before going back high level into ideas")
    owner_ratification_ref: not_applicable
  belief:
    believed_state: >-
      D9 to D16 are live and proved. The paper's boot seed stall is gone
      (first passivation 0.5-0.75 s, was 1m44s). Its steady /health 503s
      come from one read, the management desk's inbox cursor (14,965
      rows; about 3 s warm, 16.7 s cold, on the single write connection);
      D15 halved the calls (post-boot 3.2%, steady 2.6%); D16's kind index,
      built on the paper at 12:30 (a 19.3 s window), cut that read to about
      0.75 s, and the paper's steady /health 503 rate is now 0.17%. The dispatcher's 8 slots are saturated:
      about 3 of them wait at the Synthetic gate of 3, and half of all
      activations wait over 5 minutes. Synthetic's weekly allowance falls
      about 8 points an hour (63% at 10:00).
    main_uncertainty: >-
      how long the kind index build holds the paper's write connection
      (estimated tens of seconds to minutes); which dispatcher option the
      owner takes for the Synthetic half.
    next_observation: the paper's remaining store_stalled holders (selfdev ListByStates, scanSideRows, the online GC) and the Synthetic allowance.
  blocker_or_risk: >-
    The owner restarted the Mac at about 15:05 UTC 2026-10-23: local
    workers, Devin seats and omp seats stopped; the agents-VM mirrors and
    everything on Node B kept running. The paper runs luna alone with no
    fallback since 14:47:30 (owner: "chatgpt luna 100% with synthetic only
    as fallback for now"); the luna-synthetic pool is built but not
    deployed. Owner decisions open: the roadmap re-cut
    (lane/fp0-roadmap-recut), whether the paper runs telemetry.batch.
  next_action: >-
    Resume from the ledger's 15:0x restart handoff, in order: (1) D17 from
    lane/fp0-deployD17 once mirror d17 passes (guest diag probes, the
    online_gc flag, the runs-list scan; orange), then prove SEC-10 step 3
    and body caps step 11 on T with scripts/guest_diag_probe.py; (2) the
    luna-synthetic pool (lane/fp0-luna-synthetic-pool fb9d1d61c) after
    bunch 170 (rerun Devin if it did not finish) and mirror lunapool-r1,
    then put _runs/paper-model-policy/luna-synthetic-1023.toml on the
    paper; (3) D18, the selfdev heal alone (lane/fp0-selfdev-heal
    97a4f55e4, accepted), then GET and POST the heal on T only; (4) the
    stream_seq lease build (lane/fp0-stream-seq-lease) and the run-memory
    index diet's restore action (lane/fp0-run-memory-index-diet) resume
    from their pushed heads, then tier 2 review each.

receipts:
  - id: fp-terminal-superseded
    boundary: terminal
    identity: main@e2d8fca61 (the last first-pass landing); staging c72cc2e07 (D44)
    proof_refs:
      - docs/definitions/choir-launch-first-pass-metamission-2026-10-21.ledger.md (every deploy's proof, D1 to D44)
      - docs/reports/overnight-first-pass-2026-10-24.md
    rollback_ref: per deploy, in the ledger
    disposition: >-
      superseded, not complete. Landed and proved through D44, among them:
      the jailer for test, primary and premium_always_on computers (D42,
      D44), memory protection for the guests (D43), the cast bound and its
      truncation marker (D40), the default key covering owner acts (D41),
      the media tables as event projections (D39), plus the store,
      security and boot-stall deploys before them. Carried to the
      successor's station SR6 by name: R7 S2 to S6, D37's decisive proof,
      jailer wave 2b, D43 readings, the legacy tape delete, the idle sweep
      after an apply restart, the status page and report. Today's design
      landings (I27 and H038, the ICRL design, the update design, the app
      packages replan, the data-homes inventory) feed the successor.
    landing:
      source_commit: c72cc2e0741db92fbfbc7474bc7075ea7e814c78 (last behavior change)
      ci_ref: GitHub Actions run 40121748442, success
      deploy_ref: the same run's Deploy to Staging (Node B), success
      environment_identity: staging /health deployed_commit c72cc2e07
      deployed_acceptance: D44's (ledger 18:4x)
  - id: capture-label-key-read
    boundary: implement
    identity: c893a956a
    proof_refs: [internal/store/capture_label_key_read_test.go]
    rollback_ref: git revert c893a956a
    disposition: >-
      landed; the capture-label scan is gone from the paper's engine
      (70% busy over 122 s from 17:58:57 UTC; the reads that remain are
      FP1's)
    landing:
      source_commit: c893a956a66f3305216cd3b2f73778c65f35296e
      ci_ref: GitHub Actions run 39645339172, success
      deploy_ref: the same run's Deploy to Staging (Node B), success
      environment_identity: the paper's observability build.commit c893a956a, deployed_at 2026-10-21T17:49:03Z
      deployed_acceptance: 27 source polls between 17:49 and 18:01 UTC, all ok; 66 schedules active; no capture-label op in the engine counters
  - id: fp6-batch-1
    boundary: implement
    identity: 53a691be6
    proof_refs: [frontend/tests web-lens spec 13/13 local, internal/agentcore/observability_bounded_test.go]
    rollback_ref: git revert b489ed48f..53a691be6 (the twelve FP6 commits)
    disposition: >-
      landed; the browsercontrol and Obscura routes answer 404, Web Lens
      imports through import-url, memory=detail is bounded and reads the
      workspace; image (step 5) and egress (step 6) are later deploys
    landing:
      source_commit: 53a691be6
      ci_ref: GitHub Actions run 39692787991, success
      deploy_ref: the same run's Deploy to Staging (Node B), success, deployed_at 2026-10-21T23:36:26Z
      environment_identity: the owner computer's and the paper's observability build.commit 53a691be6
      deployed_acceptance: browser routes 404 (GET capabilities, POST sessions), no key 401; import-url example.com 201 with text; memory=detail store root /mnt/persistent/state.texture 3,134,501,771 bytes and a concurrent request busy; the paper booted 23:31:02 and made 42 inference calls in the next 10 minutes
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
When a batch is ready, the worker reports its candidate to FP0. FP0 rebases
it onto current main, runs focused checks, freezes its base, scoped paths
and digest, obtains independent review by risk tier (see the landing block), verifies the reviewed
identity is unchanged, then lands through the Landing Loop and runs
deployed acceptance. Semantic changes after review require re-freezing and
re-review of the delta (one seat) when the earlier round found a blocking-real defect; otherwise the checks answer. A failed proof goes back to the lane with the evidence.

**Why landing is the constraint.** Building is parallel. Landing is not:
each red batch gets a deploy of its own, so the order of landing is the
order of the batch plan, and B1 measures where each landing's time goes.

## Coverage of the security and density checklist

Every item has one owner in this pass or a stated disposition.

| Station | Items |
|---|---|
| FP0, orchestrator | SEC-14, RES-07, RES-19, RES-21; B1 timing; the L3-05 live census |
| FP1, store | L1-01 (the three hot reads, boot-path logs, engine concurrency), RES-18, L3-05 inside `internal/store` |
| FP2, desk boundary | ARCH-02, ARCH-05, ARCH-07, SEC-01, SEC-02, SEC-13, SEC-15, RES-01 to RES-04; the guest side of SEC-09 and SEC-10 |
| FP3, host | the two vmctl recovery defects (group 1); SEC-03, the host side of SEC-09 and SEC-10, NET-07 (group 2, B4); REC-02, SEC-11, SEC-12 (group 3, B5) |
| FP4, self-development | ARCH-10, ARCH-04, ARCH-03 (design until ARCH-02 lands) |
| FP6, network | SEC-07 (by deleting Obscura), NET-01's inventory, SEC-06 |
| FP7, gateway | SEC-05, SEC-08 Option K, RES-11, SEC-16, RES-12 |
| FP8, queued (B6) | RES-05, SEC-04, NET-08, RES-16, RES-06, RES-09, RES-14 |
| FP9, queued (B9) | NET-06, NET-09 |
| FP10, queued | RES-17; B8's second half (NET-01's fetch service, NET-02, NET-05, NET-10); the P2 items NET-03, NET-04, RES-08, RES-10, RES-13, RES-15, RES-20, RES-22, ARCH-06, ARCH-08, ARCH-09, ARCH-11 to ARCH-15 |
| Done before this pass | REC-01, ARCH-01 |
| Owner decisions | DEC-01, DEC-05, DEC-06, DEC-07 (defaults stand) |

## Review round 1, adjudicated

Two of three seats answered: GPT-6.1-sol (send back) and Gemini 4 Argon
(accept with edits). Devin failed again on a tool call that needs
confirmation. Both reviewed the file before the owner's rescope, so the
findings about the mobile lane, the S6 slices and B11 coverage are moot.

Adopted:
- The landing order: rebase first, then freeze and review the rebased
  candidate, land, prove; review again if reviewed lines change; the
  completion gate on landed candidates (both seats).
- FP3's recovery fixes land before FP1's store changes (sol).
- FP1's batch gets a deploy of its own (Argon).
- An activation lease: no deploy or drill during another's proof (both).
- File ownership between lanes: FP2 owns the guest side of
  `nix/autoputer-vm.nix` and the desk-cell confinement commits 07abf0351
  and 9cee80a2d; FP3 owns the host egress commit 61f4da20f, the
  quick-containment commits and the owner-act flip; FP4's ARCH-03 waits
  for FP2's manifest; FP1's L3-05 stays inside `internal/store` (Argon,
  sol). Argon had the two confinement commits the other way round;
  corrected from the commits themselves.
- Worker rules, sent to all six workers at about 18:20 UTC: no shared git
  state or gh commands, nothing outside their worktree (the scratchpad
  holds credential files), no registry or checklist edits, every Dolt link
  through the build limiter (which now sets the Go limits itself), red
  code fails closed, live probes are FP0's (both).
- SEC-14 and the before-and-after memory measurements belong to FP0;
  B8's second half has an owner (FP10); a coverage table (both).
- The queue fills dependency-ready, B6 first (sol).
- Format: a bridge conjecture, a load-and-adversarial realism axis, the
  landing block on the receipt, a catch-all for the scratchpad, a
  companion ledger, the authority manifest entry, and a single valid
  entrypoint (the live doc check passes) (both).

Not adopted:
- "S6-0's commits are not on main" (Argon): they are; checked with
  `git branch -r --contains`.
- "Pause workers during review" (sol): panels run as remote calls with
  small local processes; the build limiter is the memory control. If the
  Mac's pressure rises, workers pause first.
- "Move L4-03 out of FP10" (both): moot after the rescope.
