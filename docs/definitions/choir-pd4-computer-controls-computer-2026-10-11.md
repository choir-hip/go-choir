---
definition_version: 4

# PD4 — one autoputer controls another, each with its own four desks.
# Station of definitions/choir-persistent-desks-metamission-2026-10-11.md.
# Owner 2026-10-11: "once we get everything working in the same computer
# setting, we will enable one autoputer to control another. And the second
# autoputer will have its own four desks."
readiness: intent

review:
  reviewer: none
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-11T01:35:00Z
  source:
    canonical_ref: main b3dbc686
    deploy_identity: staging choir.news b3dbc686
  worktrees: []
  observed:
    - >-
      CLAUDE.md Safety: a forked computer is a sibling with its own
      ComputerID, event chain and identity material; results return to
      another computer only through publication and adoption, never by
      direct merge; forks wait on fork re-keying and isolation.
    - The choir CLI and product API already let an agent drive a computer under a scoped key.

finish:
  deliver: >-
    A desk on one computer can ask a second computer to do work, follow its
    progress, and adopt its published results; the second computer runs its
    own four desks.
  artifact: to be shaped after PD3
  acceptance: []
  rollback: not_applicable (intent)
  landing: {required: unknown, environment: staging, required_receipts: []}

value:
  better_means: to be shaped
  goodharting_would_be: to be shaped

homotopy:
  realism_axis: >-
    one computer's desks -> a second computer driven through the product
    API -> desk-to-desk requests across computers with adoption.

boundaries:
  mutation_class: unknown
  authority_sources: [owner statement 2026-10-11]
  must_preserve: [results cross computers only by publication and adoption]
  excluded: []
  protected_surfaces: []

now:
  status: working
  slice: intent only; waits on PD3
  source_ref: b3dbc686
  deploy_identity: unknown
  candidate: {id: none, state: none, ref: none, base: none, digest: none, scope: []}
  conjecture: {id: none, claim: none, test: none, edge: missing_oracle, delta_o: none, scope_if_supported: none, status: proposed, evidence_refs: []}
  decision: {what: none, kind: none, status: none, evidence_ref: none, owner_ratification_ref: not_applicable}
  belief:
    believed_state: single-computer desks are not yet persistent
    main_uncertainty: the control surface (product API keys vs a desk verb)
    next_observation: PD3 landed
  blocker_or_risk: none
  next_action: none until PD3

receipts: []
---

# PD4 — computer controls computer (intent)

Once one computer's four desks work, one computer can drive another. The
second computer has its own four desks. Results cross between computers only
by publication and adoption. This file keeps the intent so it isn't lost.
