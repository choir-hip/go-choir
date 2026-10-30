definition_version: 4

# Station FP10 / ARCH-08 of the launch first pass
# (choir-launch-first-pass-metamission-2026-10-21.md): budgets passed down
# agent trees. Authority: the metamission's FP10 entry (P2 items, each
# decided item by item) and SDC ARCH-08.
readiness: drafted

review:
  reviewer: none yet; FP0 sends the code commit to a panel on the frozen diff
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-21T23:30:00Z
  source:
    canonical_ref: e4b792b7f (lane/fp10-arch08-budgets base)
    deploy_identity: unknown to this lane; FP0 reads staging identity
  worktrees:
    - path: /home/agent/lanes/fp10-arch08-budgets
      status: clean
      class: goal_candidate
      owner: FP10 ARCH-08
      touch: goal_owned
      recovery: branch lane/fp10-arch08-budgets
    - path: the lane/fp8-image-jailer worktree
      status: unknown
      class: other_agent_wip
      owner: FP8
      touch: forbidden
      recovery: leave in place; this station touches no vmctl, vmmanager or image file

finish:
  deliver: >-
    A design for budgets passed down agent trees that reconciles the
    settled carve-out (M6.5) with "track spending, never cap" (DEC-07);
    and the first observe-only slice: every research tree's token spending
    totalled and visible to the desk that started it.
  artifact: >-
    Commits on lane/fp10-arch08-budgets: this file, the problem record and
    the design (green); a failing test (yellow); the tally and the stale
    comment fixes (orange).
  acceptance:
    - action: go test ./internal/agentcore -run 'TestResearch' ; go test ./internal/yaegikernel
      proves: children return their token usage and the existing depth and time pins still hold
      evidence_class: local test
    - action: the deployed proof in docs/design/agent-tree-budgets-2026-10-21.md, run by FP0 on staging
      proves: a deployed research tree reports its spending
      evidence_class: deployed proof
  rollback: git revert of the code commit; the docs stay.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Spending by agent trees becomes tracked and visible to the desk that
    caused it, without any new refusal of work.
  goodharting_would_be: >-
    A counter nobody can read; a cap shipped before the owner decides
    DEC-07; or "depth 2-3" restored on paper while the code stays at 1.

homotopy:
  realism_axis: >-
    Unit test with a scripted provider in the lane, then a panel on the
    frozen diff, then one research tree on a staging test computer.

boundaries:
  mutation_class: green (docs); yellow (test); orange (result fields and a log line)
  authority_sources:
    - SDC ARCH-08 and DEC-07
    - docs/archive/reviews/rlm-implementation-consensus-2026-08-29.md D7b/M6.5
    - owner PD1 2026-10-11 (one level of fan-out)
    - CLAUDE.md and docs/choir-doctrine.md
  must_preserve:
    - No refusal, cap or kill of a tree beyond what exists (DEC-07 is open).
    - Depth stays 1; the time carve-out and the computer-wide call spacer are unchanged.
    - No desk prompt or surface text changes (prompt caches).
    - No vmctl, vmmanager or VM image file (FP8's lane).
    - No boot-path step; no store schema change.
  excluded:
    - Egress-byte, child-count and memory slices (design only).
    - Gateway fair-share scheduling (RES-11).
  protected_surfaces: []

now:
  status: checkpoint_incomplete
  slice: >-
    Slice 1 built and committed on lane/fp10-arch08-budgets, not deployed:
    491ce4275 (docs), d8990fdee (test, does not build alone), 6580f9916
    (code). go test ./internal/agentcore -run TestResearch passes. go test
    ./internal/yaegikernel has 10 failures in this lane, every one a
    spawned worker failing to load libicui18n.so.76 (the worker runs with
    a cleaned environment, without this machine's Nix ICU path); the diff
    there is a comment and three omitempty fields. FP0: run the yaegikernel
    tests where the worker finds ICU (CI), send 6580f9916 to a panel, land
    the three commits together, and run the deployed proof in the design.
    Slices 2 to 5 wait on DEC-07 or other lanes (design table).
  source_ref: 6580f9916
  deploy_identity: unknown to this lane
  candidate:
    id: fp10-arch08-slice1
    state: ready
    ref: lane/fp10-arch08-budgets
    base: e4b792b7f
    digest: none
    scope:
      - internal/agentcore/research_children.go
      - internal/agentcore/research_children_test.go
      - internal/yaegikernel/broker_protocol.go
      - internal/yaegikernel/choir.go
  conjecture:
    id: C-budget-is-scheduling
    claim: >-
      A tree budget can honor "track spending, never cap" if spending is
      always tracked, shape is bounded, and over-budget trees queue rather
      than fail.
    test: owner answer to DEC-07; slice 1 deployed shows trees' real spending.
    edge: missing_oracle
    status: proposed
    evidence_refs: [docs/design/agent-tree-budgets-2026-10-21.md]
  decision:
    what: DEC-07, budgets as scheduling limits; whether a per-call child bound is a cap; depth 1 as doctrine
    kind: product
    status: open
    owner_ratification_ref: none
