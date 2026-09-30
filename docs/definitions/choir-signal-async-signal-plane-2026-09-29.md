---
definition_version: 4
definition_id: choir-signal-async-signal-plane-2026-09-29
execution_mode: mission_orchestrator
readiness: reviewed
member_of: choir-jev-supervision-metamission-2026-09-29

# review binds THIS file at its fixed commit (stamped at promotion).
review:
  reviewer: 'agentic-consensus authoring panel (codex, claude, devin,
    gpt6-sol, gemini38) — send_back round resolved'
  frozen_ref: 'pending-stamp'
  verdict: accept
  evidence_ref: '.agentic-consensus/agentic-consensus-20260929-215420/'

start:
  captured_at: '2026-09-29T22:40:00Z'
  source:
    canonical_ref: main@ac54317c
    deploy_identity: 'staging https://choir.news build.commit=b85af274
      (observed at capture; post-M0 identity recorded in now at promotion)'
  worktrees:
    - path: /Users/wiz/go-choir
      status: dirty
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: git
    - path: docs/desk-rlm-rectification-plan-2026-09-23.md
      status: dirty
      class: other_agent_wip
      owner: other agent
      touch: forbidden
      recovery: 'leave in place; never include in station commits'
  predecessor:
    mission: M0 (choir-signal-m0-debug-stabilize-2026-09-29) — must settle
      first; this station builds on the stabilized baseline
  observed_artifact:
    - 'Architecture adjudicated by divergent+convergent panels:
      orientation doc §"adjudicated signal-plane design". Bound-cell
      choir.Message is tray-staged; no durable mid-cell send exists.
      ReduceCellIntents(failed) persists nothing — the silent-stall hole.
      injectUserTurns seam (toolloop.go:672) exists between model calls.
      rlm_inbox_cursor is run-memory scoped to runID — respawn replays
      from zero.'

finish:
  deliver: 'Any desk can send a durable async signal mid-cell; a parked
    recipient is woken immediately; a working recipient gets a fixed-format
    notice (sender/kind/seq/~140-char snippet) at its next model boundary
    and reads full content as a repl variable; every cell exit records
    cell_fate so no silent stall can recur.'
  artifact: 'choir.Emit verb + ActionEmit broker action (behind a refuse
    gate) ; cell_fate record on every cell exit + armed terminal deadline;
    boundary-drain notice injection + repl-variable refresh; durable cursor
    rekeyed to (channel, desk) with dual-read/backfill; advisory piggyback
    watermark on s.call responses'
  acceptance:
    - action: 'research cell emits evidence via choir.Emit mid-execution;
        texture''s next model call receives the notice; texture reads the
        body via the refreshed repl variable; trajectory tape shows the
        emission event + cell_fate record'
      proves: 'mid-cell durable emission + boundary notice delivery work
        end-to-end'
      evidence_class: deployed proof
    - action: 'kill a research cell after Emit; the emission stands durable
        (delivery-is-the-record); cell_fate=failure recorded; reducer
        advances'
      proves: 'emission divergence + stall terminator both hold'
      evidence_class: deployed proof
    - action: 'a parked desk receives an emission and wakes; a working desk
        receives the notice at its next model boundary without mid-eval
        interruption'
      proves: 'owner''s delivery contract: parked=immediate, working=
        after-current-model-call'
      evidence_class: deployed proof
    - action: 'respawn a desk worker after it consumed emission N; it
        resumes at N+1 under the (channel,desk) cursor with no replay
        and no lost message'
      proves: 'durable cursor continuity across respawn — the named
        biggest risk'
      evidence_class: deployed proof
    - action: 'a cell that hangs past its armed deadline records
        cell_fate=timeout and the reducer advances'
      proves: 'the hang case (not just the kill case) terminates stalls'
      evidence_class: deployed proof
    - action: 'a newer advisory watermark is observed on an s.call
        response after an emission lands mid-activation'
      proves: 'advisory piggyback works, not just boundary drain'
      evidence_class: deployed proof
  rollback: 'pre-cutover cursor checkpoints recorded; ActionEmit refuse
    gate (listed in artifact) set to refuse; cursor rekey ships with
    dual-read/backfill so a revert reads the prior runID key without
    replay-from-zero; already-emitted records preserved and verified
    readable by the rollback reader; git revert station commits'
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity,
      deployed_acceptance]

value:
  better_means: 'minimize desk-to-desk delivery latency under the owner
    contract (parked=immediate, working=after-model-call) while preserving
    tray atomicity and single-state-authority — the divergence between
    "any desk sends async anytime" and "evidence waits for cell end"'
  goodharting_would_be: 'shipping a send that reaches texture but violates
    tray atomicity or creates a second message authority; or the stall
    terminator existing without actually advancing a frozen reducer'

homotopy:
  realism_axis: 'delivery immediacy — boundary-drain only → +advisory
    piggyback → +watcher long-poll → +native demux (each a continuous
    deepening of the same substrate, never a different object)'

boundaries:
  mutation_class: red
  # red: yaegikernel broker protocol + rlm_reduce + channel store +
  # injection seam — protected surfaces.
  authority_sources: [orientation doc adjudicated design (panel-reviewed),
    owner corrections 2026-09-29, choir-doctrine.md]
  must_preserve:
    - 'tray atomicity (semantic acts all-or-nothing at cell end)'
    - 'emissions never masquerade as commitments (path-derived kind)'
    - 'actors park rather than retain activations'
    - 'egress budget still meters emission payloads'
    - 'channelID:seq wake dedup'
  excluded:
    - 'watcher goroutine / native demux (deferred behind mux proof)'
    - 'World Wire fanout substrate'
    - 'research tool-surface deletion (M0a''s scope)'
  protected_surfaces:
    - 'internal/yaegikernel session loop + broker'
    - 'internal/agentcore rlm_reduce + channel_store'
    - 'toolloop injectUserTurns seam'
    - 'run-memory cursor keys'

now:
  status: blocked_incomplete
  slice: 'ship cell_fate record + armed terminal deadline on cell exit
    first (independent sub-cut, unblocks stall terminator); then
    ActionEmit; then injectUserTurns notice; then advisory piggyback'
  source_ref: main@ac54317c
  deploy_identity: unknown
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: notice-not-payload
    claim: 'If injection is a fixed-format notice (sender/kind/seq/snippet)
      and content lands as a repl variable, then prompt-injection risk
      collapses to renderer-fixed strings while delivery latency meets the
      owner contract'
    test: 'frozen renderer contract against a redline corpus of hostile
      emission bodies: pass = notice text is byte-identical to renderer
      output modulo the quoted snippet, delimiters/length caps hold, the
      full body stays untrusted data, and the recipient''s next act
      follows none of the injected directives'
    edge: missing_oracle
    # a snippet can smuggle instructions inside the length cap — the
    # corpus bounds but cannot eliminate the class
    delta_o: 'redline corpus against the notice renderer'
    scope_if_supported: 'all desk↔desk async delivery'
    status: proposed
    evidence_refs: []
  decision:
    what: 'A+advisory-piggyback composite per convergent panel; emission
      = distinct channel kind via ActionEmit; boundary notice +
      repl-variable receive'
    kind: architecture
    status: settled
    evidence_ref: '.agentic-consensus/agentic-consensus-20260929-212240/'
    owner_ratification_ref: 'owner: notice-injection + snippet correction
      2026-09-29'
  belief:
    believed_state: 'the adjudicated design is correct and minimal; the
      real risk is the cursor rekey and poison-envelope replay loop'
    main_uncertainty: 's.call response extension point (piggyback field
      placement) — flagged unverified by devin'
    next_observation: 'first Emit→notice→read round-trip on staging'
  blocker_or_risk: 'depends on M0 settling; red-class ceremony required'
  next_action: 'after M0: reconcile M0 receipts and inspect failed-cell
    exit/cursor persistence paths; ship cell_fate+deadline first
    (independent sub-cut), then Emit verb, then injection seam, then
    piggyback'

receipts: []
---
