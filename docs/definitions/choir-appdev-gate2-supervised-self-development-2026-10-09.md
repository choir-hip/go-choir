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
  source_ref: 79edc337
  deploy_identity: 19834f68
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
      Seven M11 reruns on 2026-10-09/10, each stopping one layer deeper.
      Fixed on the way: capsule session worker (733bec77), freeze on a new
      source directory (2c68cc18), Texture rejecting delegated-cast
      reports (028446a5), the verifier's in-cell bundle mirror (3e69567b),
      and the inference breaker counting one computer's 400s against
      everyone (589a68bc; rerun 6 failed on that outage). Rerun 7
      (2a16a6db): implementation froze, the independent verifier passed,
      the operation reached awaiting_approval, the probe approved it, and
      apply began with a planned restart. Apply then stalls in
      materializing: first resumed desk work keeps the replay-completeness
      checkpoint from seeing a quiet chain, and once quiet the replay is
      not equivalent to live state
      (problems/selfdev-apply-checkpoint-starved-by-resumed-work-2026-10-10.md).
      Rerun 8 (5704ace6): the bounded apply hold (7ea0f66d) worked: no desk
      turn between the apply restart and the checkpoint. The replay diff
      named the cause: the engineering path minted co-super-* ids that the
      from-genesis replay upcasts, so 24 lifecycle commands and 24 events
      were re-keyed. Fixed in f9531177 (engineering-* ids, legacy refs
      still accepted). Rerun 9 (77406673): the engineering lifecycle
      replays exactly; two recorded tool-result events still re-keyed
      because the upcast rewrites prose leaves (edge-only whitespace
      guard). Fixed for probe and restore in 6e7e3c5e: the staged replay
      keeps the live store's deposit mode (deposit_mode in the report).
      Rerun 10 (cf0969cf): Node B ran out of memory at 06:05Z, from QA
      computers that vmctl was not tracking. The vmctl host-capacity
      guards fixed that (cf0969cf). The rerun then froze, and the
      independent verifier rejected the bundle correctly: source.patch
      creation hunks were diffed against a phantom empty line, so the
      builder's git apply refuses them. Reruns 8 and 9 shipped the same
      defect and their verifiers passed it. Fixed in 1ae758ef: the freeze
      now proves the patch with git apply and a byte compare. A trace
      review of every log
      (problems/trace-review-desk-protocol-friction-2026-10-10.md)
      found desks guessing enum values against the reducer. Fixed in
      26c288c4 (enum rejections name the accepted values) and 81104b9c
      (refused dispositions name their reason). Owner 2026-10-10 raised
      the tool-loop budgets 100x (dbed74e5) and set no call limit before
      the system works. A haiku loop-watch reviews traces each run. Rerun
      11 (dbed74e5) reached the first completed apply of Gate 2. It froze,
      the independent verifier passed, and it reached awaiting_approval at
      09:20, was approved, and was applied at 09:21. Candidate B started.
      That happened despite two mid-run deploys I pushed (cc72a94c,
      19834f68), each of which restarted vmctl twice and rebooted the busy
      guest. The second deploy killed the working guest after apply. Probe
      scripts are tooling now (a820beea). Resolve refuses
      guest_reattach_pending (cfc17e05). But the kill at 09:25:48 came
      through a boot path the resolve guard does not cover: the frontend's
      own cold-recover or compute recovery. The chokepoint is
      vmmanager.StartVM's orphan kill
      (problems/vmctl-restart-reboots-busy-computer-2026-10-10.md,
      correction and clustering assessment). Track D landed in 19834f68:
      an ordinary engineering freeze opens its own promotion candidate
      when the signed mode is armed, so no special start API is needed
      on that path. F13 (79edc337): every desk prompt carries its
      generated REPL surface. choir.Help() is in the cell, and compile
      errors name what exists.
    main_uncertainty: >-
      Whether apply completes now that the bundle is faithful and the
      deposit mode is retained. Then reject and restore.
    next_observation: M11 rerun 12 after the StartVM chokepoint fix, with no push during the run, reading Texture's turn count to first useful act (F13), the verifier verdict, then replay-completeness at apply.
  blocker_or_risk: >-
    Red surfaces (checkpoint, route projection, apply). Every proof runs
    on a disposable first; the owner computer only with owner approval.
    Residuals: vocab-guard-ids, vocab-recovery-prefix, upcast-legacy-refs,
    upcast-rewrites-prose, base-upcast-vs-live, live-rescan-mixed-spelling,
    verifier-apply-check, verifier-implementer-channel, refresh-log-reason,
    texture-budget-burn-while-waiting, identical-rejection-loop,
    probe-self-stop, one-candidate-per-document-trajectory,
    owner-recovery-reboot, guest-degraded-after-forced-reboot.
  next_action: >-
    Deploy 79edc337; run M11 rerun 12 with the haiku loop-watch; if the
    replay is eligible, continue to applied, reject and restore;
    otherwise read the named rows.
receipts:
  - id: m11-rerun-2026-10-09
    ref: docs/evidence/m11-rerun-2026-10-09T20-30-55Z.json
    result: partial (5/16; blocked at awaiting_approval)
  - id: m11-rerun-4-2026-10-09
    ref: docs/evidence/m11-rerun-2026-10-09T22-01-41Z.json
    result: partial (5/16; capsule worked; freeze failed on a new source directory)
  - id: m11-rerun-5-2026-10-09
    ref: docs/evidence/m11-rerun-2026-10-09T23-15-26Z.json
    result: partial (5/16; frozen; verifier failed closed on the in-cell draft mirror)
  - id: m11-rerun-6-2026-10-10
    ref: docs/evidence/m11-rerun-2026-10-10T00-19-25Z.json
    result: partial (5/16; failed on the shared inference breaker opened by 400s)
  - id: m11-rerun-7-2026-10-10
    ref: docs/evidence/m11-rerun-2026-10-10T00-49-59Z.json
    result: partial (10/16; approved; apply stalled in materializing)
  - id: m11-rerun-8-2026-10-10
    ref: docs/evidence/m11-rerun-2026-10-10T02-27-23Z.json
    result: partial (apply hold worked; replay diverged on V1-spelled engineering ids)
  - id: m11-rerun-9-2026-10-10
    ref: docs/evidence/m11-rerun-2026-10-10T03-54-51Z.json
    result: partial (10/16; engineering replays exactly; 2 recorded events upcast)
  - id: m11-rerun-2026-10-09-2132-2148
    ref: docs/evidence/m11-rerun-2026-10-09T21-32-17Z.json, docs/evidence/m11-rerun-2026-10-09T21-48-21Z.json
    result: partial (5/16; capsule worker floor — engineering exhausted its 200-call budget, then a completion guard; Landlock/seccomp fixed in e3225820, 733bec77)
  - id: m11-rerun-10-attempts-2026-10-10
    ref: docs/evidence/m11-rerun-2026-10-10T05-54-57Z.json, docs/evidence/m11-rerun-2026-10-10T06-00-20Z.json
    result: blocked (0/16; first page load timed out during the Node B OOM)
  - id: m11-rerun-10-2026-10-10
    ref: docs/evidence/m11-rerun-2026-10-10T07-00-32Z.json
    result: partial (5/16; verifier correctly rejected an unappliable source.patch)
  - id: m11-rerun-11-2026-10-10
    ref: docs/evidence/m11-rerun-2026-10-10T08-36-30Z.json (pending probe exit); progress log in problems/vmctl-restart-reboots-busy-computer-2026-10-10.md
    result: partial (first completed apply, 09:21; guest killed after candidate B started, by a mid-run deploy)
---

# Gate 2 — self-development with live Texture supervision

The exit test is the owner's: ask Texture for a change, watch engineering
build it in a capsule, see the change before it lands, approve it, and be
able to take it back. Stations S1 remainder, S4, S5 and S6 are the parts;
this file holds the gate together and keeps the order honest.

## Order (revised 2026-10-10, owner: "get more aggressive")

The breathtaking moment is the target: the owner asks Texture, on their
own computer, for a change they can see. It applies, they use it, and
they can take it back. Everything is cut to that path. M11's evidence
file change is a scaffold, not the goal: it never rebuilds the SPA or
the binary from the patch.

Critical path. Tracks B to E run in parallel with A.

- **A. Apply, reject and restore on a disposable computer.** M11
  reruns. Rerun 11 is in flight on dbed74e5.
- **B. Layered checkpoint identity.** The owner computer runs a layered
  release, and its self-development checkpoint reports "served SPA is
  underivable" (residual 1 of `problems/layered-release-spa-underivable-2026-10-09.md`).
  M11's fresh computers may not be layered, so passing A does not prove
  the owner path. This is a hard blocker for F.
- **C. A visible change.** An M11 variant whose request changes UI
  text, then adds a tiny app through the existing registry. The host
  builder rebuilds SPA and binary from the patch, and the applied
  computer serves the new surface. It proves what the owner will see.
- **D. Any request is promotable (owner, 2026-10-10).** Self-development
  is not a special operation. Any prompt, from Texture or the prompt bar
  (which already enters through Texture), can end in a promotable
  change. Examples: "make an app that does x", "change the desktop
  background", "email me a daily digest", "change an app to …".

  The route is the ordinary one: Texture, then management, then
  engineering in a capsule. Whenever a capsule result would change the
  computer, it becomes a frozen candidate by itself. The engineering
  assignment opens the promotion; there is no separate entry API. This
  is doctrine C3/C8 (effects stay inert frozen bundles until an
  authorized acceptance event), not a new design.

  Where the special API came from: the July
  `choir-cli-self-development-2026-07-16` mission (authorized 5483a082,
  built 7d635330 on July 19). Its deliverable was a disposable computer
  "develop[ing] Choir itself end to end through the supported public
  choir CLI", an operator-driven transaction (`choir self-dev
  approve …`) built before the desks could drive the path themselves. It
  was forwarded to the guest in August (4543624b), and M11 inherited it.
  Underneath, it already wakes an ordinary engineering desk
  (`ensureSelfDevelopmentRun`). The special part is the entry and the
  separate operation record, which the August effects mission flagged
  as a direct-write table outside the reducer.

  Deletion plan: the engineering assignment opens the candidate.
  Freeze, verify, decision, materialization and rollback become the
  generic promotion events behind the decision card. Then
  `POST …/self-development/operations` and its operation store are
  deleted. Its only remaining caller is the M11 probe, which moves to
  prompting Texture.

  D design: freeze opens the candidate. Today
  `freezeCapsuleEffectBundle` (`agentcore/tools_capsule.go`) looks up an
  operation pre-created for the trajectory and refuses without one. An
  ordinary engineering assignment from Texture can therefore build a
  change but never promote it. The change:

  - when an engineering assignment freezes and no candidate exists, the
    runtime opens one;
  - it is keyed by the assignment, not the trajectory, so one document
    can carry several changes;
  - its heads are pinned at freeze;
  - its request is the assignment's own objective;
  - it opens only if the computer's signed mode authorizes proposals.

  Verification, decision, materialization and rollback already key off
  the operation and are unchanged. The engineering prompt says: a change
  meant for this computer is frozen. The start API then has no caller
  but the probe, which moves to prompting Texture.

  The lightest effect wins. A background is a setting, a digest is an
  automation, and an app is a release, but all three are typed
  candidates on one promotion path. Texture's "effects-OFF" framing
  follows the computer's armed mode.
- **E. A changes app (owner, 2026-10-10).** A desktop app for
  managing changes, not a one-off card. It lists every candidate
  (requested or speculative) with its request, diff, verifier verdict
  and state. It is where the owner approves, rejects and rolls back, and
  where release history and restore points live. The Texture document
  links to the candidate in this app. The app also needs a rollback
  that works when the desktop itself is broken; the updater's health
  rollback is the backstop.

  Its second job is publishing: share an approved change as source
  that another owner can inspect, customize, rebuild and adopt. That is
  S8's deliverable, which arrives through this app, after F.

  Today approve, reject and roll back exist only as API calls and key
  scopes. The slides app was one example of an app change, not a
  target.
- **F. Owner computer.** Arm the mode with the owner, who then asks
  in their own words, through D and E.
- **G. Speculative development (owner, 2026-10-10), right after F.**
  The system finds better approaches and builds them unasked.
  Management (computer-wide coherence) opens speculative candidates.
  They are built and verified in capsules and wait on the same decision
  card. Nothing applies without the owner. A rate policy for
  speculation is an open design question; tracks B to F come first.

Cut from the path:

- **Preview before approval (S5).** Restore after apply is the safety
  net. Preview is added after F.
- **Full recorded egress (S4).** The authoritative build is the host
  builder, which has network and pins dependency hashes. Capsules need
  network only to add a new dependency. Minimal S4 comes after F:
  - an allowlist of lockfile-verified registries (Nix cache, Go proxy,
    npm), recorded by URL and hash;
  - no full byte capture, since the package managers already verify
    integrity.
- **App packages (S7).** These follow F. Until then a new app goes into
  the monolith's registry, and its rollback is whole-release.
- **Texture suite items not on this path** (T5b and the research
  stopping rule). These stay residuals.

Loop speed: each M11 attempt costs 30 to 90 minutes. Run variants in
parallel now that host memory is guarded. Add a harness that replays
apply from a known frozen bundle, so an apply fix does not wait on a
fresh build.

The S1 remainder items (non-root runtime, Yaegi floor proofs) stay on S1's
file. They gate S4 egress, not this gate's network-closed v1.
