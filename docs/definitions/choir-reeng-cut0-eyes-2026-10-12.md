---
definition_version: 4

# Cut 0, "eyes": the first cut of the re-engineering metamission
# (docs/definitions/choir-reeng-metamission-2026-10-12.md, station C0).
# Authority: docs/re-engineering-decisions-2026-10-12.md §10 item 0 and
# §12 (cutover decisions win over everything else, including the studies
# and the superseded drafts R1 and R5 at 03499ef9, whose transport and
# trace-export findings are carried here).
readiness: drafted

review:
  reviewer: none
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-12T19:30:00Z
  source:
    canonical_ref: main 03499ef9
    deploy_identity: staging choir.news b172895b
  worktrees:
    - path: /Users/wiz/go-choir
      status: dirty (this session's doc edits in progress; decisions, metamission, ACTIVE.md, mission-graph.yaml)
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: git
  observed:
    - >-
      Cross-wire: egressRoundTrip writes a request frame, then reads the
      next frame and never matches its RequestID
      (internal/yaegikernel/sidecar.go:325-350), though FramedConn reads are
      single-owner (transport.go:57-58) and the host already echoes the id
      (desk_session_worker.go:414-427, serviced inline at :331-334). Study A's
      probe: four goroutines, results rotated a->c, b->a, c->b, all waiting
      ~300 ms. The research prompt still invites parallel calls
      (internal/runtimeprompts/overlays/rlm_research_runtime.yaml:25).
      Documented: docs/problems/parallel-choir-calls-cross-wire-results-2026-10-12.md.
    - >-
      Second REPL: acquire re-keys a busy persistent session to
      key+"#run:"+runID (internal/agentcore/tools_desk.go:129-131), and
      TestDeskSessionPoolSeparatesBusySessionsAndBoundsIdleOnes asserts that
      behaviour (tools_desk_session_test.go:102-133). The deadlock comment
      (tools_desk.go:102-107) is about research children: they run with
      RunRecord nil (research_children.go:182), so they key on their own
      activation (tools_desk.go:108-117), in a fresh pool
      (research_children.go:194). Serializing the parent's pool cannot make
      a child wait on its parent's lease. Tool calls in one run are
      sequential (no goroutines in internal/toolregistry/toolloop.go).
    - >-
      Broker handle: issued once per worker with time.Hour
      (internal/yaegikernel/choir.go:119), checked on every broker verb
      (choir.go:208-216 -> broker.go:150 -> handles.go:113), never
      refreshed; a session lives while it gets a cell every 30 min
      (tools_desk.go:42-50). Code predicts egress verbs (search, fetch,
      product API, Research, Emit) refused after minute 60; unobserved.
    - >-
      Trace export has no complete no-SSH path. GET
      /api/trajectories/{id}/events serves lifecycle events
      (api_trajectory.go:122-140; `choir lifecycle events`,
      cmd/choir/main.go:844-860). Run progress events (tool_loop with tokens,
      passivating_tool_success; toolloop.go:604-619, :810) are only on
      /internal/runtime/runs/{id}/events (agentcore/api.go:1012-1015); the
      public /api/runs/{id} returns the run record only (api.go:879-899).
      internal/trace has a causal-chain HTTP handler (trace/query.go:97-122,
      ParentChain) that nothing mounts; /api/trace/* is served by nothing,
      yet scripts/texture_revision_cadence_probe.mjs:87 and
      internal/agentcore/run_acceptance.go:573 point at it.
    - >-
      Existing no-SSH reads: /api/runtime/obligations (restart kind and
      reason, open work; obligations_surface.go:12-27, 129-132),
      /api/runtime/observability (computer id, build commit, boot log;
      api.go:990-1006), texture revisions (`choir texture revisions`),
      /api/universal-wire/stories (`choir wire stories`).
    - >-
      Fresh test computers: scripts/qa_test_computer.mjs mints owner, computer
      and a scoped key, but resolves the computer id over SSH to node-b
      (qa_test_computer.mjs:29-34), against standing question 9.
    - >-
      Crash drill route: `choir computer restart` -> POST
      /api/computers/{id}/lifecycle/restart (cmd/choir/main.go:1331, 1674),
      which stops and re-resolves the guest (internal/proxy/computer_lifecycle.go:147-157).
      Planned-restart markers are written only by platform update and
      self-development apply/rollback (agentcore/platform_update.go:263,
      self_development_materializer.go:315, 390), so the owner restart boots
      markerless: a crash. The known violation it should expose:
      docs/problems/crash-restart-leaves-work-open-and-recasts-engineering-2026-10-12.md.
    - >-
      The -race workflow runs weekly and on dispatch, not on push
      (.github/workflows/race.yml:3-9).

finish:
  deliver: >-
    Any agent with a scoped key can see, without SSH, exactly what one run
    did, can mint a fresh staging computer, run a fixed news story through
    a crash to publication, and get a pass/fail on the substrate rules.
    Parallel choir.* calls get their own answers, and each persistent desk
    has one REPL.
  artifact: >-
    (1) Trace export: `choir trace export <trajectory>` writes one JSON
    bundle of the run's causal chain from product API routes only: records,
    deliveries, cells and tool-loop events, turn commits, revisions,
    publications, and search arrivals once they exist. Missing server reads
    get one owner-scoped read route: wire internal/trace's unmounted handler
    if its store carries the run events, otherwise expose the internal run
    events reader under /api/runs/{id}/events. Dead /api/trace citers are
    repaired or deleted in the same commit.
    (2) Tracer: scripts/reeng_tracer.mjs mints a fresh computer
    (qa_test_computer.mjs with the SSH lookup replaced by
    /api/runtime/observability), submits a fixed AI-news story script and a
    follow-up, restarts the computer through the owner route mid-run, runs
    to publication, exports the trace, and keeps the bundle (outside the
    repo, under ~/.config/choir-qa/traces/<label>/, sha256 in the receipt).
    (3) Invariant checker: scripts/reeng_invariants.mjs reads a bundle
    offline and checks substrate facts only, never content quality:
    I1 every revision was written by the Texture desk; I2 every round (today
    every run and trajectory) ends closed or open with a named reason; I3
    no dispatch after a markerless boot whose cause predates the boot; I4
    every citation resolves to a fetchable source and its quote; I5 inbox
    cursors only move forward (added when Cut 1 creates them, not stubbed).
    (4) Request-id fix: one reader loop owns the worker's socket and routes
    StreamBrokerEgressResult frames to waiters by RequestID and StreamCell
    frames to the session loop; study A's probe becomes
    TestEgressConcurrentCallersGetOwnResults in internal/yaegikernel, run
    under -race.
    (5) One REPL owner per desk: the "#run:" fallback is deleted; a second
    run's cell waits for the session (bounded by its context, without
    holding the pool mutex) and gets the same worker; the existing busy
    test is inverted.
    (6) Survival register: docs/reports/reeng-cut0-survival-register-2026-10-12.md
    lists what crosses the cuts (published versions; the owner's computer
    and Textures untouched until the cut is proven; staging test computers
    disposable; old execution state archived, never migrated), and records
    an export of the owner's Textures through the product API (bundle kept
    outside the repo, digests and counts in the register) plus counts and
    states only for the rest of the owner's computer.
    (7) Measurements in the register: push to healthy fresh test computer
    (split into CI, deploy, mint, first ready prompt bar), and whether a
    desk session older than one hour loses its choir.* verbs.
  acceptance:
    - action: go test ./internal/yaegikernel -race -run TestEgressConcurrentCallersGetOwnResults (and the full package), then the race workflow dispatched on the landed SHA
      proves: concurrent and late (post-cell) egress callers each get their own result or a clean error, never another caller's; no data race
      evidence_class: local test plus CI receipt
    - action: go test ./internal/agentcore -run TestDeskSession (inverted busy test plus a cancelled-waiter test)
      proves: a busy persistent session is waited for, never duplicated; a waiter whose context ends returns without a lease
      evidence_class: local test
    - action: the invariant checker run against planted-violation copies of a real bundle (one per rule)
      proves: each rule fails when its fact is broken, before any pass is trusted
      evidence_class: local test over a deployed artifact
    - action: three tracer runs on choir.news at the landed SHA, each on a fresh test computer, crash drill included, bundles kept
      proves: the export reaches every listed layer without SSH; the checker's verdict per rule on today's system (an I3 failure on the known restart defect is a finding, not a tracer failure)
      evidence_class: deployed proof
    - action: one test-computer desk session driven past 70 minutes with cells under 30 minutes apart, exported
      proves: whether the one-hour handle expiry refuses choir.* verbs in a live desk session
      evidence_class: deployed proof
    - action: survival register reviewed against the owner's computer through the API only
      proves: the owner's Textures are exported with digests, and the owner computer was read, not changed
      evidence_class: deployed observation
  rollback: >-
    git revert per commit. No persisted state changes: the transport fix and
    the session pool are process memory, the new read route is read-only,
    scripts and the register are additive. Test computers are disposable.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, race_dispatch, deploy, environment_identity, deployed_acceptance, consensus_review]

value:
  better_means: >-
    Minimize the time and access needed to see what a run actually did and
    which substrate rule broke (today: SSH and the gateway journal), while
    removing the two defects that falsify any measurement (cross-wired
    egress, a second REPL per desk) and touching nothing Cut 1 or Cut 2 owns.
  goodharting_would_be: >-
    A checker that passes because its rules read nothing (a rule never shown
    failing on a planted violation); an export that silently omits layers
    the API does not serve; a tracer that "publishes" without the crash
    drill; a lock that serializes egress and is reported as request-id
    correlation.

homotopy:
  realism_axis: >-
    focused unit tests (socketpair, pool) -> checker on planted bundles ->
    one tracer run on a fresh staging computer -> three runs with the crash
    drill -> the same tracer and checker gating Cut 1 and Cut 2 (rules
    added, never a second harness).

boundaries:
  mutation_class: >-
    orange (desk cell transport in internal/yaegikernel; desk session pool
    in internal/agentcore; one read-only owner API route) plus green/yellow
    (scripts, tests, register). Rollback: git revert.
  authority_sources:
    - docs/re-engineering-decisions-2026-10-12.md §10 item 0, §12.1, §12.4, §12.5, §12.10 (owner 2026-10-12)
    - CLAUDE.md (landing loop, staging acceptance, restart rule, mutation classes)
    - docs/standing-questions.md (3 deletion citers, 9 no-SSH operability)
  must_preserve:
    - a crash restart never resumes work; the drill observes, it adds no resumer and no marker
    - the owner's computer and Textures are read only; no restart, prompt or write on them
    - Texture is the sole writer of canonical revisions
    - handle verification on every broker verb (the fix routes frames, it does not bypass checks)
    - the 60-minute run limit (prototyping safety constraint)
  excluded:
    - records, rounds, hypothesis ids, cursors, packets, work items, crash closure, prompts (Cut 1)
    - fixing the restart defect the drill exposes (Cut 1; decisions §12.12)
    - host-side egress concurrency and async search (Cut 2; host egress stays serial)
    - research children onto records (Cut 1; decisions §12.11)
    - content-quality scoring of any kind
  protected_surfaces: [internal/yaegikernel/sidecar.go, internal/yaegikernel/session_loop.go, internal/yaegikernel/transport.go, internal/agentcore/tools_desk.go]

metamission:
  parent: docs/definitions/choir-reeng-metamission-2026-10-12.md
  depends_on: []
  feeds: [C1 authority, C2 evidence to publication]

now:
  status: drafted
  slice: authored for file review; nothing built
  source_ref: 03499ef9
  deploy_identity: staging choir.news b172895b
  candidate: {id: none, state: none, ref: none, base: none, digest: none, scope: []}
  conjecture:
    id: C0-1
    claim: >-
      With a no-SSH trace bundle and a substrate checker on fresh test
      computers, each later cut's breakage is found within one tracer run of
      the push, and no later measurement is falsified by cross-wired egress
      or a second REPL.
    test: >-
      the checker flags a planted violation of every rule; on today's system
      it reports the documented restart defect (I3) from the crash drill
      without SSH; the race-run probe test passes after the fix and fails
      before it
    edge: missing_oracle
    delta_o: run progress events and the causal chain reachable through one owner-scoped API read
    scope_if_supported: staging test computers, the fixed AI-news script, one owner restart per run
    status: proposed
    evidence_refs: [docs/problems/parallel-choir-calls-cross-wire-results-2026-10-12.md, docs/reports/re-engineering-studies-2026-10-12/A-async-search-yaegi.md]
  decision:
    what: >-
      Export in the choir CLI (agent-operable under a scoped key), tracer
      and checker as scripts; checker reads bundles offline; trace bundles
      and the owner Texture export live outside the repo with digests in
      receipts; the handle-TTL result, if confirmed, gets a problem doc and
      is fixed here only if it is a per-cell handle reissue with a test,
      otherwise it is a named residual for Cut 1
    kind: operational
    status: proposal
    evidence_ref: docs/re-engineering-decisions-2026-10-12.md §10, §12
    owner_ratification_ref: not_applicable
  belief:
    believed_state: both defects exist as cited at 03499ef9; run events and the causal chain are not reachable through /api today
    main_uncertainty: whether internal/trace's store holds the run events and parent links the export needs, or only submission events
    next_observation: read internal/trace writers and one staging trajectory's lifecycle events against the internal run events of the same run
  blocker_or_risk: the owner Texture export needs a read:texture key on the owner's computer (owner-minted)
  next_action: agentic-consensus file review of this station

receipts: []

weak_measures:
  - name: push_to_healthy_test_computer
    kind: telemetry
    baseline: unknown
    desired: known, split by CI, deploy, mint, ready
    decision_use: sets the pace and batch size of every cut
    cannot_prove: that a cut works
  - name: invariant_failures_per_tracer_run
    kind: weak_signal
    baseline: unknown (first runs on today's system)
    desired: none outside documented defects
    decision_use: where to look; feeds the redesign tripwire (decisions §12.5)
    cannot_prove: content quality or that a rule set is complete
---

# Cut 0: eyes

Cut 1 replaces work items and packets with records and cursors, and Cut 2
replaces search, Texture's write path and publication. Neither can be
judged by reading the gateway journal over SSH. Cut 0 builds the eyes first:
a trace bundle any scoped agent can pull, a tracer that drives a fresh
computer through a crash to publication, and a checker for substrate
facts. It also fixes the two defects that would corrupt every later
measurement: cross-wired parallel egress and a second REPL per desk.

**Order.** (4) and (5) first, test-first, since every trace after them is
trusted more. Then the read route and `choir trace export` (1), the
SSH-free mint and the tracer (2), and the checker proven on planted
violations (3). Land, deploy, run the tracer three times. Then the TTL
session and the register (6, 7). During the cut window, problems fixed in
the same commit go into `docs/problems/reeng-cut0-breakage-2026-10-12.md`
(decisions §12.4). Others get their own problem doc first.

**Standing questions.** 1: owner-settled scope (§10, §12); the operational
choices in `now.decision` are proposals. 3: deleting the `#run:` fallback
and any unmounted trace route needs a grep of tests, docs and scripts in
the same commit. 5: the trace bundle is a projection; the guest store
stays the authority. 6: success is a kept bundle plus a checker verdict,
never a narrated run. 8: the pool and transport are process memory, and a
restart ends them by design. 9: every step goes through the product API or
the CLI; the SSH lookup in the mint script is removed.

Ways it can fail, written before the code:

- the reader loop drops or reorders a StreamCell frame, or a late
  goroutine's result lands in the next cell (the probe covers late callers
  and the cell frame);
- a waiter on a busy desk session deadlocks behind an egress call that
  waits on another run of the same desk (bounded by the waiter's context;
  the tracer's desk timings show waits);
- interleaved cells from two runs in one REPL collide on variable names
  (accepted under one owner per desk; the notice names the other run);
- acquire waits while holding the pool mutex and stalls every desk
  (waiting happens outside the mutex; test with two keys);
- the export omits a layer the API does not serve and the checker passes
  on absence (the bundle lists each layer as present, empty or unserved,
  and a rule over an unserved layer reports unknown, never pass);
- the crash drill hits a computer that is between runs and proves nothing
  (the tracer restarts only after it observes an in-flight run);
- the owner's computer is touched (the tracer refuses any computer id it
  did not mint; owner reads use a read-only key);
- the TTL session dies for another reason first (30-minute idle reclaim,
  failed cell), so "no expiry seen" is not a negative (the export must show
  one worker for the session's full age).
