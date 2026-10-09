---
definition_version: 4

# Gate 2 — self-development with live Texture supervision. Second gate of the
# owner-ratified v6 roadmap (metamission "v6 plan", 2026-10-09: "Gate 1
# today, Gate 2 tomorrow"). Owner 2026-10-09 (autonomous run): "now let's
# continue with gate 1 and into gate 2 ... you can go autonomous". This file
# composes stations S1 remainder -> S4 -> S5 -> S6 around the gate's exit
# test; the station files keep their own detail.
readiness: drafted

review:
  reviewer: none
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-09T18:40:00Z
  source:
    canonical_ref: main b2a76845 + harness commit (local; pushed after the Texture suite run)
    deploy_identity: staging choir.news x-choir-build-commit e3f2d560
  worktrees:
    - path: /Users/wiz/go-choir skills/agentic-consensus/SKILL.md
      status: dirty
      class: other_agent_wip
      owner: unknown (pre-existing)
      touch: forbidden
      recovery: leave in place
  observed:
    - >-
      The self-development episode machinery landed once: M11
      (definitions/choir-selfdev-gate-2026-09-27.md) satisfied all six legs
      on staging 2026-09-29 — stage an operation, bind approval to a
      qualified-consensus receipt, apply, render engineering evidence,
      reject a second candidate, restore the pinned head
      (scripts/m11_selfdev_episode_probe.mjs). It has not run since; S2
      layering, SH, SL and the restart rule landed after it.
    - >-
      S2 closed 2026-10-05: builder-produced no-reboot app-layer apply,
      rollback atomicity, provenance. Layered computers cannot derive a
      checkpoint frontend identity (problems/layered-release-spa-underivable-2026-10-09.md
      residual 1), which self-development checkpoints depend on.
    - >-
      Capsules run chrooted in their own user, mount and network namespaces
      (internal/capsule/executor.go startBrokerLocked) with a three-variable
      environment: no egress and no gateway token today. S4 (recording
      egress proxy, capsule-private Nix store) has not started.
    - >-
      No preview path exists: nothing serves a candidate's frontend or a
      capsule dev server to the owner's desktop (S5 not started).
    - >-
      M11 starts operations through POST /api/computers/{id}/self-development/operations
      with a prompt; the gate's exit test starts from the owner asking
      Texture. The Texture-to-engineering path for a self-development
      request has not been exercised end to end.
    - >-
      Owner cold-recover needs an immutable route slot, created only by a
      first self-development transition (SH; deferred to this gate).

finish:
  deliver: >-
    The owner asks Texture for a change to their computer; engineering
    develops it in a capsule; the owner sees a preview of the change,
    approves it, the release applies, and the owner can roll it back — all
    from the product, with no SSH.
  artifact: >-
    A deployed self-development path on staging: Texture turns an owner
    request into an engineering self-development operation; the capsule
    freezes a candidate and builds its release; a preview route shows the
    candidate's frontend to the owner before approval; an owner decision
    (product UI, not a test harness receipt) approves or rejects; apply and
    rollback work on layered computers with a derivable checkpoint identity.
  acceptance:
    - action: >-
        Re-run the M11 episode probe on a fresh disposable on the current
        build.
      proves: >-
        The September machinery still works after S2/SH/SL; any regression
        is named before new construction.
      evidence_class: deployed proof (disposable)
    - action: >-
        On a fresh disposable, the owner asks Texture for a small visible
        change (e.g. a label on the desktop). Observe the engineering
        operation, the frozen candidate, the preview route, the owner
        approval in the product, the applied release (the label is served),
        then roll back (the label is gone).
      proves: the gate's exit test, end to end, on the product path.
      evidence_class: deployed proof (disposable)
    - action: >-
        Same request on the owner computer (layered), with the owner's
        approval.
      proves: >-
        The path works on a real layered computer with history, and the
        checkpoint identity is derivable there.
      evidence_class: deployed proof (owner computer, owner approval)
  rollback: >-
    Revert the gate commits; self-development mode stays off by default
    (propose_only must be armed explicitly), so a reverted path cannot act.
    Product rollback of an applied release is the restore leg itself.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the number of exit-test steps that need SSH, a harness-minted
    receipt or manual repair, while preserving owner approval as the only
    path to apply, the capsule boundary, tape authority and rollback.
  goodharting_would_be: >-
    A harness that mints the approval or consensus receipt itself and calls
    it owner approval; a preview that is a screenshot or a description
    rather than the candidate's served frontend; skipping rollback because
    apply worked; proving only on a fresh disposable and calling the owner
    computer done.

homotopy:
  realism_axis: >-
    M11 harness episode on a disposable -> Texture-initiated operation with
    a frozen-candidate preview and product approval on a disposable -> the
    same on the owner's layered computer -> a live dev-server preview while
    engineering is still working (S5 proper) -> capsule egress for changes
    that need fetches (S4).

boundaries:
  mutation_class: red
  authority_sources:
    - owner v6 roadmap 2026-10-09 (metamission "v6 plan")
    - owner 2026-10-09 "continue with gate 1 and into gate 2 ... go autonomous"
    - docs/choir-doctrine.md (Texture sole canonical writer; engineering capsule-bound)
  must_preserve:
    - no release applies without an owner decision bound to the frozen candidate
    - capsules keep no egress and no gateway token until S4 lands its recording proxy
    - crash restart never resumes work; a planned update restart may (AGENTS.md)
    - deploys never restart busy computers
    - the tape is never rewritten; rollback is a forward restore transaction
  excluded:
    - S4 open-world egress (after this gate unless a change needs fetches)
    - S3 fast resume, S7-S11, SP production infrastructure
  protected_surfaces:
    - self-development operation state and decisions
    - checkpoint and route projection
    - updater apply and restore
    - Texture canonical writes (request intake only)

now:
  status: working
  slice: >-
    reality — re-run the M11 episode probe on a fresh disposable on the
    current staging build and record which legs still pass.
  source_ref: b2a76845
  deploy_identity: e3f2d560
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: G2-bridge
    claim: >-
      If the existing M11 machinery still works, Gate 2 is three bounded
      additions — Texture intake, a frozen-candidate preview route, and
      product approval — plus the layered checkpoint identity, not a
      rebuild.
    test: >-
      The M11 re-run passes its legs on the current build; failures, if
      any, are local regressions with named causes.
    edge: missing_oracle
    delta_o: the M11 re-run receipt on the current build
    scope_if_supported: staging disposables on the current single host
    status: proposed
    evidence_refs: [docs/definitions/choir-selfdev-gate-2026-09-27.md]
  decision:
    what: >-
      Gate 2 v1 keeps capsules network-closed (S4 egress after the gate)
      and previews the frozen candidate's built frontend before a live
      dev-server bridge. Both are the conservative choices: no new egress
      and no new socket bridge on the critical path.
    kind: scope
    status: proposal
    evidence_ref: this file, start.observed
    owner_ratification_ref: pending (stated per AGENTS.md No Blocking Asks)
  belief:
    believed_state: >-
      Operations, approval, apply, reject and restore exist and passed in
      September; Texture intake, preview and product approval do not exist;
      layered checkpoints are underivable.
    main_uncertainty: >-
      Whether the M11 path survived S2 layering and the SL/SH changes, and
      whether layered checkpoint identity blocks apply or only restore.
    next_observation: the M11 re-run receipt.
  blocker_or_risk: >-
    Red surfaces (checkpoint, route projection, apply). Every proof runs
    on a disposable first; the owner computer only with owner approval.
  next_action: >-
    Run scripts/m11_selfdev_episode_probe.mjs against staging after the
    Texture acceptance suite finishes; write the receipt; file a problem
    doc for each failed leg before any fix.
receipts: []
---

# Gate 2 — self-development with live Texture supervision

The exit test is the owner's: ask Texture for a change, watch engineering
build it in a capsule, see the change before it lands, approve it, and be
able to take it back. Stations S1 remainder, S4, S5 and S6 are the parts;
this file holds the gate together and keeps the order honest.

## Order

1. **Reality.** Re-run M11 on the current build. It passed all six legs on
   September 29th; S2 layering and the Gate 1 work landed since.
2. **Layered checkpoint identity** (S6, residual 1 of the layered-release
   problem doc). Self-development checkpoints on a layered computer need a
   trusted frontend identity; today they report "underivable".
3. **Texture intake.** An owner request in Texture becomes an engineering
   self-development operation (Texture writes the request; management
   admits engineering; engineering opens the operation).
4. **Preview** (S5 v1). A route on the owner's desktop serves the frozen
   candidate's built frontend before approval.
5. **Product approval** (S6). The owner approves or rejects in the product;
   the decision binds to the frozen candidate.
6. **Owner computer.** The whole path on the owner's layered computer, with
   the owner's approval.
7. **Later on the axis:** a live dev-server preview while engineering works
   (S5 proper), then capsule egress through the recording proxy (S4).

The S1 remainder items (non-root runtime, Yaegi floor proofs) stay on S1's
file. They gate S4 egress, not this gate's network-closed v1.
