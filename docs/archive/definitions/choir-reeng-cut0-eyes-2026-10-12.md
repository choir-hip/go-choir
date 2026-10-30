---
definition_version: 4

# Cut 0, "eyes": the first cut of the re-engineering metamission
# (docs/archive/definitions/choir-reeng-metamission-2026-10-12.md, station C0).
# Authority: docs/re-engineering-decisions-2026-10-12.md §10 item 0 and
# §12 (cutover decisions win over everything else, including the studies
# and the superseded drafts R1 and R5 at 03499ef9, whose transport and
# trace-export findings are carried here).
readiness: reviewed

review:
  reviewer: convergent agentic-consensus panel (codex, claude opus, gpt-6.1-sol, gemini-3.8-flash, deepseek-v4.1-flash, glm-5.3-flash; gpt-6-luna and muse-spark timed out)
  frozen_ref: 2d2b864e
  verdict: send_back (codex, gpt-6.1-sol, deepseek), accept (claude, gemini, glm); repaired in the following commit, re-review pending
  evidence_ref: docs/re-engineering-decisions-2026-10-12.md §13 (synthesis)
  rereview: claude opus and gemini-3.8-flash at deb6f760 (codex and gpt-6.1-sol hit a usage limit); accept (gemini), send_back on one amendment from claude (tracer load mechanism; owner-scope acceptance row), applied

start:
  captured_at: 2026-10-12T19:30:00Z
  source:
    canonical_ref: main 03499ef9
    deploy_identity: staging choir.news b172895b
  worktrees:
    - path: /Users/wiz/go-choir
      status: dirty (this session's doc edits in progress; decisions, metamission, ACTIVE.md, mission-graph.yaml)
      correction_2026-10-12: those edits landed at 2d2b864e and the tree was clean at review; Go source is unchanged between 03499ef9 and 2d2b864e, so every cite below holds at both
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
      Documented: docs/archive/problems/parallel-choir-calls-cross-wire-results-2026-10-12.md.
    - >-
      Second REPL: acquire re-keys a busy persistent session to
      key+"#run:"+runID (internal/agentcore/tools_desk.go:129-131), and
      TestDeskSessionPoolSeparatesBusySessionsAndBoundsIdleOnes asserts that
      behaviour (tools_desk_session_test.go:102-133). The deadlock comment
      (tools_desk.go:102-107) is about research children: they run with
      RunRecord nil (research_children.go:182), so they key on their own
      activation (tools_desk.go:108-117), in a fresh pool
      (research_children.go:194). Serializing the parent's pool cannot make
      a child wait on its parent's session. Tool calls in one model turn run
      in parallel (internal/toolregistry/batch_executor.go:52-60, called
      from toolloop.go:662) unless one of them is on the sequential list,
      which includes desk_go_eval (batch_executor.go:145-152). So one run
      never holds two cells on a session at once, but parallel non-session
      tools and the goroutines inside one cell do share the worker socket,
      which is the cross-wire case.
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
      yet run_acceptance.go:573, scripts/texture_revision_cadence_probe.mjs:87,
      scripts/m3_vmctl_refresh_probe.mjs, three frontend/check-*.mjs scripts
      and nine frontend/tests/*.spec.js request /api/trace/trajectories/...
      (a different shape from the unmounted handler's /api/trace/events).
      The internal/trace store also receives runtime event projections
      (runtime.go:4654-4658, 4693-4699), best-effort, so it is not a
      completeness guarantee.
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
      docs/problems/crash-boots-redrive-applies-2026-10-13.md.
    - >-
      A per-computer hold exists: POST /internal/vmctl/hold (host-internal
      only; internal/vmctl/handlers.go:530-556, 1716) sets a durable
      maintenance hold (ownership.go:181-205). While held, vmctl refuses
      every automatic lifecycle action (boot, recover, refresh, reattach,
      resume, reclaim, retention; pressure_reclaim.go:633,
      warmness_policy.go:165) and the deploy refresh skips it
      (.github/workflows/ci.yml:1434). A held computer that is running keeps
      running its current code. One that boots under the hold gets
      RUNTIME_MAINTENANCE_HOLD=1 and refuses run admission
      (agentcore/runtime.go:738, 891-896); /internal/vmctl/maintenance-serve
      makes it readable under that fence (handlers.go:704-748). Unhold
      clears it (ownership.go:207-218). An earlier note in Cut 1 said no hold
      mechanism was visible; that was wrong. The owner chose not to use it
      (decisions §13.1): the owner's computer takes each cut.
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
    publications, and search arrivals once they exist. Each layer is marked
    present, empty or unserved. Missing server reads get one owner-scoped
    read route over the primary run-event reader, /api/runs/{id}/events
    (internal/trace's best-effort store is not used as the source of
    completeness). Each cell's egress calls are run events carrying request
    id, verb, the request id echoed in the result, and the worker id, so a
    bundle shows which caller got which answer. Every /api/trace citer listed
    above is repointed to the new route or deleted in the same commit.
    PII (owner 2026-10-12): a bundle is the owner's own data, read by the
    owner and kept outside the repo, so the export is unredacted. Anything
    that leaves the owner (a shared bundle, a review panel, an external
    model) is redacted first by a private decision model, Jev-style and
    run privately, never by an external API call; internal/pii's Ollama
    redactor is not that path. The cut records which bundles may leave
    and leaves the redactor to the station that ships it.
    (2) Tracer: scripts/reeng_tracer.mjs mints a fresh computer
    (qa_test_computer.mjs with the SSH lookup replaced by
    /api/runtime/observability), submits a fixed AI-news story script and a
    follow-up, restarts the computer through the owner route mid-run, runs
    to publication, exports the trace, and keeps the bundle (outside the
    repo, under ~/.config/choir-qa/traces/<label>/, sha256 in the receipt).
    Once the tracer runs, the landed-mission probes it replaces are
    deleted (dead-code census A11, owner-approved 2026-10-12):
    scripts/s0b_*, s0m_*, s1a_*, s2_*, smg_*, m0_qa_probe.mjs (its timing
    baseline moves into the tracer), m3_vmctl_refresh_probe.mjs,
    m4_jev_roundtrip.mjs, m9a_platform_update_probe.mjs,
    texture_revision_cadence_probe.mjs, conductor_jev_experiment.mjs and
    selfdev-turn-census.sh; read the s0m and smg probes first, since they
    hold the shortest recorded form of each desk handshake. The cut also
    decides the Dolt trace store (internal/trace, off everywhere because
    RUNTIME_TRACE_PERSISTENCE_ENABLED is never set): used by the export or
    deleted (census B4).
    It also drives the transport under load: the tracer's script asks a
    desk for a cell that issues eight parallel tagged choir.* reads, and
    submits a second run to the same desk while the first is mid-cell.
    What the model writes is not under the tracer's control, so the bundle
    is checked for at least two concurrent egress calls in one cell and
    two runs waiting on one desk; the tracer retries up to three times,
    and if the load never appears the I5 verdict is unknown, not pass.
    Correctness under load is proven by the race test in (4); the deployed
    run shows it holds on the product path.
    (3) Invariant checker: scripts/reeng_invariants.mjs reads a bundle
    offline and checks substrate facts only, never content quality:
    I1 every revision was written by the Texture desk; I2 every round (today
    every run and trajectory) ends closed or open with a named reason; I3
    no dispatch after a markerless boot whose cause predates the boot; I4
    every citation names a source and a quote, and the quote is found in
    that source's body when the bundle carries the body (unknown
    otherwise; the checker never fetches); I5 every egress result's
    request id equals its caller's, and one desk has one worker at a time;
    I6 inbox cursors only move forward (added when Cut 1 creates them, not
    stubbed).
    (4) Request-id fix: one reader loop owns the worker's socket and routes
    StreamBrokerEgressResult frames to waiters by RequestID and StreamCell
    frames to the session loop; study A's probe becomes
    TestEgressConcurrentCallersGetOwnResults in internal/yaegikernel, run
    under -race.
    (5) One REPL owner per desk: the "#run:" fallback is deleted; a second
    run's cell waits for the session (bounded by its context, without
    holding the pool mutex) and gets the same worker; the existing busy
    test is inverted.
    (6) Broker handle. The handle is issued once per worker for an hour
    and checked on every verb, so the code already says a desk loses its
    choir.* verbs after the hour. This cut reissues the handle when a
    cell is bound, with a test that fails on an expired handle first. No
    live session confirms it (owner pace ruling 2026-10-13, metamission
    boundaries.pace: nothing runs live to confirm what the code says).
    (7) Measurements, in docs/archive/reports/reeng-cut0-measurements-2026-10-12.md:
    push to healthy fresh test computer (split into CI, deploy, mint, first
    ready prompt bar); per-turn token and latency figures from the three
    tracer bundles kept so far (Cut 2's baseline); and the run-acceptance level today's
    system reaches on the tracer story (Cut 1's baseline).
    Nothing is preserved across the cuts (decisions §13.1): no survival
    register, no owner hold, no Texture export.
  acceptance:
    - action: go test ./internal/yaegikernel -race -run TestEgressConcurrentCallersGetOwnResults (and the full package), then the race workflow dispatched on the landed SHA
      proves: concurrent and late (post-cell) egress callers each get their own result or a clean error, never another caller's; no data race
      evidence_class: local test plus CI receipt
    - action: go test ./internal/agentcore -run TestDeskSession (inverted busy test plus a cancelled-waiter test)
      proves: a busy persistent session is waited for, never duplicated; a waiter whose context ends returns without the session
      evidence_class: local test
    - action: the invariant checker run against planted-violation copies of a real bundle (one per rule)
      proves: each rule fails when its fact is broken, before any pass is trusted
      evidence_class: local test over a deployed artifact
    - action: one fast tracer run on choir.news at the landed SHA on a fresh test computer, crash drill and the concurrency load included, bundle kept (owner pace ruling 2026-10-13; a second run only if the first is ambiguous)
      proves: every listed layer is marked present, empty or unserved, all without SSH, and no rule passes on an unserved layer; an export with a key scoped to a different test computer is refused; I5 holds wherever the bundle shows concurrent egress and overlapping cells on one desk (unknown if the load never appeared); the checker's verdict per rule on today's system (an I3 failure on the known restart defect is a finding, not a tracer failure)
      evidence_class: deployed proof
    - action: go test ./internal/yaegikernel -race -run TestDeskHandleOutlivesItsFirstIssue (a handle issued with a short lifetime, a cell after it expires; fails before the fix)
      proves: a desk keeps its choir.* verbs for its whole life (the 70-minute live session this replaced was stopped by the owner's pace ruling)
      evidence_class: local test
  rollback: >-
    git revert per commit. No persisted state changes: the transport fix and
    the session pool are process memory, the new read route is read-only,
    scripts and the measurements report are additive. Test computers are disposable.
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
    one fast tracer run with the crash drill on a fresh staging computer -> the same tracer and checker gating Cut 1 and Cut 2 (rules
    added, never a second harness).

boundaries:
  mutation_class: >-
    red for the trace export (a new read of the Trace/evidence surface,
    CLAUDE.md mutation classes);
    orange for the desk cell transport (internal/yaegikernel) and the desk
    session pool (internal/agentcore); green/yellow for scripts, tests and
    the measurements report. Rollback: git revert.
  red_ceremony:
    conjecture_delta: C0-1 below (proposed); no prior conjecture is retired
    protected_surfaces: Trace/evidence (read only, owner-scoped)
    admissible_evidence: >-
      deployed bundles from choir.news at the landed SHA with sha256 in the
      receipt; the checker's verdict on planted violations. Local tests are
      admissible only for the transport and pool.
    heresy_delta:
      discovered: [cross-wired parallel egress, a second REPL per busy desk, /api/trace citers pointing at no route, the restart defect the drill will show]
      introduced: []
      repaired: [cross-wired parallel egress, a second REPL per busy desk, /api/trace citers pointing at no route]
    export_scope: >-
      the read route answers only the owner of the run's computer (or a
      key scoped to it); an export from a computer the key does not own is
      refused, which the acceptance shows with a key for a different test
      computer
  authority_sources:
    - docs/re-engineering-decisions-2026-10-12.md §10 item 0, §12.4, §12.5, §12.10, §13 (owner 2026-10-12)
    - CLAUDE.md (landing loop, staging acceptance, restart rule, mutation classes)
    - docs/standing-questions.md (3 deletion citers, 9 no-SSH operability)
  must_preserve:
    - a crash restart never resumes work; the drill observes, it adds no resumer and no marker
    - the tracer acts only on computers it minted
    - Texture is the sole writer of canonical revisions
    - handle verification on every broker verb (the fix routes frames, it does not bypass checks)
    - the 60-minute run limit (a guardrail against runaway spend; nothing is tested past it)
  excluded:
    - records, rounds, hypothesis ids, cursors, packets, work items, crash closure, prompts (Cut 1)
    - fixing the restart defect the drill exposes (Cut 1; decisions §12.12)
    - host-side egress concurrency and async search (Cut 2; host egress stays serial)
    - research children onto records (Cut 1; decisions §12.11)
    - content-quality scoring of any kind
  protected_surfaces: [internal/yaegikernel/sidecar.go, internal/yaegikernel/session_loop.go, internal/yaegikernel/transport.go, internal/agentcore/tools_desk.go]

metamission:
  parent: docs/archive/definitions/choir-reeng-metamission-2026-10-12.md
  depends_on: []
  feeds: [C1 authority, C2 evidence to publication]

now:
  status: landed
  slice: >-
    All seven artifacts landed (receipts docs/archive/evidence/c0-eyes-acceptance-2026-10-13.json;
    measurements docs/archive/reports/reeng-cut0-measurements-2026-10-12.md). The
    70-minute session was replaced by a handle test (owner pace ruling);
    the second-run desk wait was dropped from the I5 load because runs of
    one desk agent are serialized. Acceptance read on c0-fast-1 (287734c5):
    every layer present, empty or unserved without SSH; I1 pass; I2 and I3
    fail on the restart defect (Cut 1, which L1 then fixed); I4 fail
    (Cut 2); I5's request-id and one-worker halves hold; a key for another
    test computer reads nothing. Plan review 2026-10-13 added tracer
    scenarios, the identity fence and the I3 quiet window
    (docs/archive/reports/reeng-plan-review-2026-10-13.md).
  source_ref: af92cb94
  deploy_identity: staging 6d676094
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
    status: supported (the drill found the restart defect and Cut 1 breakage B1 within one run each, without SSH; the race test failed before the request-id fix)
    evidence_refs: [docs/archive/evidence/c0-eyes-acceptance-2026-10-13.json, docs/archive/problems/parallel-choir-calls-cross-wire-results-2026-10-12.md, docs/archive/reports/re-engineering-studies-2026-10-12/A-async-search-yaegi.md]
  decision:
    what: >-
      Export in the choir CLI (agent-operable under a scoped key), tracer
      and checker as scripts; checker reads bundles offline; trace bundles
      live outside the repo with digests in receipts; the handle-TTL result, if confirmed, gets a problem doc and
      is fixed in this cut (artifact 6); nothing is preserved across the
      cuts (decisions §13.1)
    kind: operational
    status: proposal
    evidence_ref: docs/re-engineering-decisions-2026-10-12.md §10, §12
    owner_ratification_ref: not_applicable
  belief:
    believed_state: both defects exist as cited at 03499ef9; run events and the causal chain are not reachable through /api today
    main_uncertainty: none for this cut; push to healthy test computer is 8-9 min plus a 12 s mint with Node B memory free
    next_observation: Cut 1's tracer scenarios
  blocker_or_risk: none
  next_action: none (landed); Cut 1 executes

receipts: []

weak_measures:
  - name: push_to_healthy_test_computer
    kind: telemetry
    baseline: 8-9 min push to deployed; mint 12 s with memory free, 574 s when finished test computers hold Node B memory
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
violations (3). Land, deploy, run the tracer. Then the handle reissue
by test (6) and the measurements (7). During the cut window, problems fixed in
the same commit go into `docs/archive/problems/reeng-cut0-breakage-2026-10-12.md`
(decisions §12.4). Others get their own problem doc first.

**Standing questions.** 1: owner-settled scope (§10, §12, §13); the operational
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
- the tracer acts on a computer it did not mint (it refuses any such id);
- the export leaks another owner's run (the route is owner-scoped; a
  wrong-owner key is refused in the acceptance);
- the handle test passes because the lifetime it sets never elapses (it
  must fail on today's code, where the handle is issued once).
