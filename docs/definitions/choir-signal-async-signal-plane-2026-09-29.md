---
definition_version: 4
definition_id: choir-signal-async-signal-plane-2026-09-29
execution_mode: mission_orchestrator
readiness: drafted
member_of: choir-jev-supervision-metamission-2026-09-29

start:
  captured_at: '2026-09-29T22:40:00Z'
  source:
    canonical_ref: main@ac54317c
    deploy_identity: staging https://choir.news (post-M0 build)
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: source
      owner: this session
      touch: read_write
      recovery: git
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
  artifact: 'choir.Emit verb + ActionEmit broker action; cell_fate record
    on every cell exit + armed terminal deadline; boundary-drain notice
    injection + repl-variable refresh; durable cursor rekeyed to
    (channel, desk); advisory piggyback watermark on s.call responses'
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
  rollback: 'feature-gate ActionEmit to refuse; revert seam wiring;
    cell_fate is additive (no rollback harm); git revert the station
    commits'
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
  mutation_class: red (yaegikernel broker protocol + rlm_reduce + channel
    store + injection seam — protected surfaces)
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
  status: pending
  slice: 'not started — promote after M0 settles'
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
    test: 'redline test: a malicious emission body never reaches the
      recipient''s model context as instructions; the notice is always
      renderer output'
    edge: 'missing_oracle — whether the snippet itself can smuggle
      instructions in 140 chars'
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
  next_action: 'after M0: ship cell_fate+deadline first (independent
    sub-cut), then Emit verb, then injection seam, then piggyback'

receipts: []
---
