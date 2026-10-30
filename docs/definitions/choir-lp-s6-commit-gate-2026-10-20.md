---
definition_version: 4

# Station LP, first half: S6 the commit gate. Run as
# `/goal docs/definitions/choir-lp-s6-commit-gate-2026-10-20.md`.
# Owner 2026-10-20 (docs/reports/owner-notes-2026-10-14.md section 28): "yes,
# we should do the missions in order through app packages as recommended".
# Order: S6, then S7-0 (every first-party app becomes a package) with a
# frontend self-development drill on one rewrite candidate in parallel, then
# app rewrites one package at a time. This moves LP ahead of the 2026-10-14
# order (efficiency, Autopaper scaling, then LP) and relaxes LP's spine edge
# to LU, LR and L2a. This file is the executable station. The reviewed
# station definition
# (docs/archive/definitions/choir-appdev-s6-commit-gate-full-release-2026-10-01.md)
# stays the source of the six acceptance legs and the divergence addendum;
# where the two differ, this file governs execution.
readiness: reviewed

review:
  reviewer: >-
    convergent panel 2026-10-20 ~10:20 UTC, devin + omp-gemini38 (owner-approved
    panelists). omp-gemini38: ready with edits, all four code gaps verified,
    agrees with both owner-question defaults. devin: incomplete (stopped on a
    non-interactive tool-confirmation refusal before a verdict).
  frozen_ref: this file at the commit that applies the three panel edits
  verdict: accept with edits (applied - witness gets its own read-only route; baseline moved to 09e98e9b2; this block)
  evidence_ref: session scratchpad panel-s6/run (manifest.tsv, omp-gemini38.out, devin.out); not in the repo

start:
  captured_at: 2026-10-20T10:05:00Z
  source:
    canonical_ref: main ea7cdda0b (docs-only over 4c2d66c38)
    deploy_identity: staging https://choir.news x-choir-build-commit 09e98e9b2 (proxy /health header, 2026-10-20T10:20Z; 4c2d66c38 at drafting)
  worktrees:
    - path: /Users/wiz/go-choir/.claude/worktrees/agent-ac665835b21f89e29 (branch worktree-agent-ac665835b21f89e29)
      status: clean
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: branch
    - path: branch feat/s7-0-app-packages (3 commits, unpushed, no checkout used here)
      status: clean
      class: other_agent_wip
      owner: not this session
      touch: read_only
      recovery: leave in place; S7-0 reconciles it, not S6
    - path: /Users/wiz/go-choir and its other worktrees and stashes
      status: unknown
      class: other_agent_wip
      owner: not this session
      touch: forbidden
      recovery: leave in place
  observed:
    - >-
      The loop runs end to end on test computers: drill go-served-1920 passed 18
      of 18 checks (freeze, approval, host build, layered apply, route served,
      restore with the witness matched) (docs/archive/reports/handoff-2026-10-15.md s2).
    - >-
      The decision binds operation, capsule, bundle digest, verifier ref,
      authority and expected heads (internal/agentcore/self_development_decision_binding.go);
      the bundle digest covers the patch hash, runtime files and test receipt
      references (internal/capsule/transaction/builder.go:27-56).
    - >-
      No admission or materializer code reads a test receipt's exit status.
      Freeze requires only that receipts exist (rlm_reduce.go:152-155); a nonzero
      exit settles a say-do precommitment "no" (types/commitment_rules.go:283-298)
      and refuses nothing. Leg 3 is unmet by construction (code reading; slice 0
      confirms on staging).
    - >-
      The host build runs after acceptance (self_development_materializer.go:330-360);
      the decision and MaterializationStarted events carry the bundle digest only
      (:393-412). Whether activation refuses a build whose receipt names a
      different patch is not asserted.
    - >-
      The updater-crash proof is one unit leg (selfdev_derivable_materialize_test.go:1058-1096);
      no deployed fault sweep exists.
    - >-
      Divergence is not derived: vmctl holds a manual DivergenceStatus
      (internal/vmctl/ownership.go:139-151), CI writes `divergence_status:
      tracking` into every offer, and no `divergent_release` refusal exists.
    - >-
      Owner-act receipts are in observe mode (nix/node-b.nix:92); the flip waits
      for real owner acts (docs/problems/hostauth-enforce-flip-2026-10-20.md).
    - >-
      The release record reserves `Declarations.Packages`, empty
      (internal/release/record.go); the release draft is built only for an
      applied operation (internal/agentcore/api_release.go). LU is docs only. S5
      preview was cut by the owner 2026-10-10 (Gate 2 definition:430-436).

finish:
  deliver: >-
    When the owner (or, on a test computer, standing consensus) approves a
    change, the computer runs exactly what was approved. A failing test, a
    changed patch, a build that does not match the approval, a stale head or a
    missing decision each refuse before anything activates; an updater killed at
    any step settles to one named release; a platform update never overwrites a
    computer's own release.
  artifact: >-
    On staging: (1) a release witness, a read joining decision, bundle,
    declared test result, host-build digests, applied release, heads,
    checkpoint, route and effective release, naming each join as compared at
    the gate or merely recorded; (2) the gate comparing the joins the witness
    shows are only recorded; (3) a tracer scenario that runs the six legs on a
    minted test computer and keeps the witness at each step in its receipt;
    (4) divergence derived from the tape with the typed `divergent_release`
    refusal and CI's `held (divergent)`, the vmctl flag and offer field deleted.
  acceptance:
    - action: Slice 0, the go-served drill on a fresh test computer at the deployed commit; fetch the witness at frozen, accepted, applied and restored.
      proves: The release identity is observable on the working loop; the gap table (compared vs recorded) scopes slices 1 and 2.
      evidence_class: deployed proof
    - action: Legs 1 and 2, a backend change applies; the changed endpoint answers after activation, reboot and cold resume; a pinned restore removes it.
      proves: The approved release is the running release and restore removes it.
      evidence_class: deployed proof
    - action: Leg 3, a declared test that exits nonzero in the frozen bundle, then attempted approval and activation, in owner mode and standing-consensus mode.
      proves: A failing declared test blocks activation under either authority.
      evidence_class: deployed negative proof
    - action: Legs 4 and 5 negatives, no decision; an explicit rejection; a patch byte changed after approval; the host asked to build a different patch; an expected head advanced after approval.
      proves: Each refuses before mutation with a typed reason on the tape.
      evidence_class: deployed negative proof
    - action: Fault sweep, kill the updater at each activation boundary (before apply, after apply, before checkpoint, before route, after route) on a test computer, let the crash boot settle, read the witness.
      proves: Each kill settles to applied or rolled back from the journal; no mixed release is observed; nothing resumes (CLAUDE.md restart rule).
      evidence_class: deployed proof
    - action: Leg 6, with a self-developed release active on a test computer, push a newer platform release through the CI app-layer path, restore the pinned head, push again.
      proves: The first push is held as `divergent_release` while the owner's endpoint keeps answering; after restore the computer tracks and the next push applies.
      evidence_class: deployed proof
  rollback: >-
    Revert the slice commit; the witness and each refusal are additive and read
    the tape. Slice 1 and 2 gates sit behind a mode that defaults to the old
    behavior until the drills pass; revert the mode in nix config. A released
    change is undone by the pinned-head restore. Slice 4 deletes the vmctl flag
    only after the derived path has held a push on staging; until then the
    derived value is logged beside the flag.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the identity joins between what was approved and what runs that
    are merely recorded rather than compared at the activation boundary, across
    every fault boundary, while preserving one canonical event authority,
    owner or declared-consensus approval on every commit, pinned restore, and
    "a crash never resumes work".
  goodharting_would_be: >-
    A witness that prints digests no gate compares; a green 18 of 18 tracer run
    counted as the gate; refusals proven with fixtures that never reach the
    production gate; killing the updater only at the boundary the unit test
    covers; "held" reported as applied; a gate on only for the tier it was
    proven on.

homotopy:
  realism_axis: >-
    Observer first, then gate strictness and fault depth on the same real loop:
    the witness on the happy-path drill -> one join refuses a mutated field ->
    every join refuses -> the updater killed at each boundary -> a platform push
    held against a diverged computer -> the same gate on a persistent non-paper
    computer. Every step is the production materializer on a minted test
    computer; no mock gate.

boundaries:
  mutation_class: red
  authority_sources:
    - owner 2026-10-20, owner notes section 28 (order through app packages)
    - owner 2026-10-09, rebase question goes into S6 (S6 definition addendum, not panel reviewed)
    - owner 2026-10-10 S5 cut from Gate 2; owner 2026-10-14/15 distribution plan and D1 (owner notes sections 15, 16, 23)
    - docs/choir-doctrine.md, docs/computer-ontology.md, CLAUDE.md
    - docs/archive/reports/selfdev-update-architecture-2026-10-15.md (P5 to P9, LU relation)
  must_preserve:
    - Capsules own no semantic state; only an accepted event changes desired code.
    - Owner or declared-consensus approval gates every commit; the paper and the owner's computer stay propose-only (owner notes section 27 item 4).
    - Only source is published; the host is the sole builder.
    - One canonical event chain; the witness is a projection, never an authority.
    - A crash restart never resumes work; a planned restart may, once.
    - Every committed change restores through the pinned-head path.
  excluded:
    - S5 preview (the original preview-digest binding is dropped, see decision), S4, S3, S2 base and layering, S8, S9, S11 auto-rebase
    - app packages: package presence is a tape fact (D1), so `Packages` stays empty and S7-0 supplies them
    - any drill on the paper computer or the owner's computer
  protected_surfaces: [canonical event commit path, decision admission, updater trust boundary and release manifest, checkpoint and route projection, deployment routing (vmctl refresh, CI app-layer push)]
  per_slice_class: {S6-0: "orange (a read, no canonical write)", S6-1: "red (decision admission, materializer)", S6-2: "red (activation boundary, updater trust)", S6-3: "red (updater and checkpoint under fault; kill hook test-tier only)", S6-4: "red (deployment routing: vmctl, CI)"}
  ceremony: >-
    Red slices carry conjecture delta, admissible evidence, rollback and heresy
    delta. Problem Documentation First: a gap found commits a problem doc before
    its fix. Failure modes are written before gate code; the proof is the tracer
    receipt on staging, not after-the-fact unit tests.

standing_questions:
  q1_who_settled: >-
    Owner-settled: the order, S5 cut, D1, source-only and host-only builds,
    divergence in S6, test-tier autonomy. Orchestrator proposals (binding for
    ordering, not authority): the slice cut, "a declared test's exit status is
    part of the approval's binding", the relaxed LP edges.
  q2_conformance: >-
    No third store; the witness reads the tape through its own read-only route;
    shared content stays in Store B behind corpusd. D1 contradicts the S6 text
    "S7 supplies the package S6 commits", so S6 carries no packages.
  q3_deletion_citers: >-
    Slice 4 deletes vmctl `DivergenceStatus`, `/internal/vmctl/divergence` and
    CI's `divergence_status` and `platform_follow_policy`. First grep docs/,
    specs/, skills/, scripts/, nix/, .github/ and code (vmctl divergence_test.go,
    deploy_refresh*.go, platformrelease/types.go, selfdevprotocol/platform_update.go);
    repair every citer in the same commit.
  q4_consumers: >-
    Approval is consumed by the tracer, `choir self-dev approve|reject|rollback`,
    the desktop card and Texture's card; a gate change keeps one admission function.
  q5_single_authority: >-
    The tape (and the updater journal for a crash boot). Divergence derives from
    the effective release on the tape; the witness, vmctl's reported release and
    CI's offer are projections that never win.
  q6_artifact: >-
    A tracer receipt holding the witness fetched from the deployed computer, the
    refusal reason read from the tape, and the staging build identity.
  q7_fate_sharing: >-
    The witness fate-shares with the guest runtime; acceptable for an operator
    diagnostic. The durable facts (decision, release digest, divergence) are on
    the tape and survive restore.
  q8_restart: >-
    No process-local witness state. A killed updater is a crash boot that settles
    from the journal and never re-applies (L1a-3, 18d01cc0); the kill hook must
    leave no planned marker or the sweep tests resume.
  q9_no_ssh: >-
    Witness and refusals go through the guest API, `choir` and the tracer. If a
    test-tier kill hook cannot be built without SSH, slice 3 names the residual
    and uses vmctl's existing kill route.

heresy_delta:
  discovered: >-
    Approval binds the bundle digest while the closure that runs is bound
    afterwards; a test's exit status is evidence, not a gate; divergence has a
    second, manual, default-unsafe authority.
  introduced: none
  repaired: none until slices land

now:
  status: working
  slice: >-
    S6-0, the release witness. First commit: a problem doc recording the three
    gaps as hypotheses (code reading, not yet trace-confirmed). Then the
    witness, then a go-served drill that fetches it.
  source_ref: main ea7cdda0b
  deploy_identity: staging 09e98e9b2
  candidate: {id: none, state: none, ref: none, base: none, digest: none, scope: []}
  conjecture:
    id: S6-C1
    claim: >-
      Activation today is by trust, not comparison: at least one of (declared
      test exit, host-build closure against the approved patch, expected heads at
      apply) is recorded but never compared. Binding the compared set into the
      decision's witness and refusing on mismatch makes a bad approved change
      unable to activate, without a second approval authority.
    test: >-
      The witness on the go-served drill shows each join and whether the gate
      compares it; then one mutation per join on a minted test computer and the
      production gate's refusal reason on the tape.
    edge: missing_oracle
    delta_o: >-
      The release witness: decision, bundle, test result, host build, applied
      manifest, heads, checkpoint, route and effective release, with a
      compared-or-recorded flag per join.
    scope_if_supported: >-
      Test-tier computers on staging, frontend and Go-input changes, owner and
      standing-consensus authority; not the paper, the owner's computer,
      packages, publication or fleet push.
    status: proposed
    evidence_refs: [internal/agentcore/self_development_decision_binding.go, internal/agentcore/self_development_materializer.go:330-412, internal/agentcore/rlm_reduce.go:152-155]
  bridge_conjecture: >-
    If the gate refuses every unbound activation (S6-C1 supported over its
    scope), app rewrites by self-development can land one package at a time
    without the owner reading every diff, because a bad rewrite cannot activate
    and restore removes it. Falsified if a rewrite activates that the owner would
    have refused, or restore is needed for what the gate should have caught;
    weakened if only the paper's propose-only rule is what protects it.
  decision:
    what: >-
      Drop S5's preview-digest binding from S6; make the declared test exit
      status and the host-build digests part of the approval's bound witness;
      derive divergence from the tape.
    kind: architecture
    status: proposal
    evidence_ref: docs/archive/definitions/choir-appdev-gate2-supervised-self-development-2026-10-09.md:430-436; docs/archive/reports/selfdev-update-architecture-2026-10-15.md:689
    owner_ratification_ref: >-
      The S5 cut is owner-stated 2026-10-10 and the divergence addendum is
      owner-directed 2026-10-09. The gate semantic is not ratified: open
      question 1.
  belief:
    believed_state: >-
      The loop works on test computers and the decision machinery is sound, but
      activation compares fewer joins than the S6 definition promises; S6 needs
      none of LU, LR or L2a.
    main_uncertainty: >-
      Whether the host build can be bound before approval or only checked at
      apply, and whether owner-act enforce flips in time for leg 4 to mean anything.
    next_observation: The slice 0 witness on a fresh go-served drill, and the count of real owner acts in the observe window.
  blocker_or_risk: >-
    Owner-act observe mode weakens a "no owner decision" refusal; the slice 3
    kill hook may not exist without SSH; a platform deploy during a drill
    contaminates it (the tracer's --expect-sha fence).
  next_action: >-
    Commit the problem doc (the three gaps as hypotheses with code refs), then
    build S6-0 on a branch from main ea7cdda0b.

slices:
  S6-0:
    what: >-
      Release witness: a dedicated read-only witness route for any operation in
      any state (`bindings: [{pair, compared_at_gate, bound, reason}]`), a tracer
      step that fetches it at frozen, accepted, applied and restored, and a check
      `witness_joined`. The release draft's applied-only refusal
      (internal/agentcore/api_release.go:52-55) stays as is: Store B release
      records are statements of applied releases only (panel finding 1).
    proof: >-
      Landing Loop, then `node scripts/reeng_tracer.mjs --label s6-0-<HHMM>
      --scenario engineering --change go-served --expect-sha <deployed sha>` on a
      fresh test computer; the receipt holds the witness and the gap table.
    exit: Gap table committed as a problem doc; S6-C1 set supported, weakened or falsified; slices 1 and 2 re-scoped.
    code_reading_2026_10_20: >-
      Built on branch feat/s6-0-release-witness (draft PR 77), not yet
      deployed or drilled. The code reading in
      docs/problems/s6-activation-trusts-not-compares-2026-10-20.md corrects
      two gaps above (hypotheses until the drill). (a) A failing test receipt
      is already refused, at Freeze (freezeReceiptOutcomesError,
      tools_capsule.go), so a bundle with a failing test cannot form and leg 3
      as written is unreachable through the product path; what is missing is a
      declared test command and a re-read after Freeze. Leg 3 is re-scoped to
      a post-freeze mutation of the test result. (b) The host build is mostly
      compared (hostbuild.VerifyJob binds operation, bundle digest, patch hash,
      base commit and base image; the host compares the builder's patch); only
      builder-receipt.json's content is hash-pinned and never parsed on the
      guest. (c) splits: heads are compared at the decision and held by the
      reducer's pending-transition lock; the candidate's frozen surface against
      the decision's heads is not compared, and divergence is not derived.
      Gap table: 17 pairs, 12 compared at or before the gate, 5 recorded-only
      (test_receipts_exit at the gate, builder_receipt_patch,
      manifest_builder_receipt, frozen_surface_decision_heads,
      divergence_derived). S6-1 and S6-2 are re-scoped from the drill's table,
      not from this reading alone.
  S6-1:
    what: >-
      Declared tests are a gate: the digested bundle names the declared test
      command and its receipts' exit statuses; owner approval, standing consensus
      and the materializer refuse a nonzero or missing result with a typed reason
      on the tape, through one admission function.
    proof: Leg 3 in both authority modes, plus a passing control.
  S6-2:
    what: >-
      Bind and compare the build: the host-build receipt names the approved patch
      hash and base commit, the applied manifest names that receipt, expected
      heads are re-checked at apply; mismatch refuses before the swap. Owner-act
      enforce flips here if a clean window with real owner acts exists (owner
      notes s27 item 3). While owner-act is still in observe, the leg 4
      negatives are scoped to the guest's decision-binding authority check
      (self_development_decision_binding.go), not the proxy signature (panel risk 2).
    proof: Legs 1, 2 and the leg 4 negatives, including a patch byte changed after approval.
  S6-3:
    what: Fault sweep at each activation boundary, killed as a crash (no planned marker), settled from the journal; the kill hook is test-tier only.
    proof: Five kills, five settled releases, witness read after each; a mixed observation fails the run.
  S6-4:
    what: >-
      Derived divergence: the effective release carries an owner-approved patch
      (its tree is not contained in the platform's) means diverged. The guest
      refuses a platform-follow offer with `divergent_release` on the tape; CI
      reports `held (divergent)`; the vmctl flag and offer field are deleted once
      the derived path has held a push. A rebase stays an ordinary S6 change.
    proof: Leg 6 on a test computer.

lu_dependency:
  verdict: >-
    S6 does not depend on LU. Slices 0 to 3 run on minted test computers through
    the tracer, which needs no pull, ring or drain. Slice 4 produces the
    divergence signal LU3a must consume (rings hold diverged computers), so S6
    supplies LU, not the reverse. Two soft couplings. LU2's persisted work-state
    epoch is not on main, so the gate keeps refusing an epoch-bumping release
    (docs/archive/reports/selfdev-update-architecture-2026-10-15.md, "LU2: a soft
    prerequisite"). LU5 later folds the materializer's apply hold into LU's
    single gate. Leg 6 is proven through today's CI push and vmctl refresh, not
    LU's pull. The ledger spine's LP `depends_on: [L2a, LR, LU]` is relaxed for
    S6 and S7-0; LU is needed by S7-1, when package bytes reach other computers.

s7_0_successor:
  note: >-
    Stub. S7-0 gets its own goal file when S6-2 lands; its design and the
    frontend drill may start earlier, in parallel, as the owner ordered.
  inherits:
    - the witness and tracer scenario, extended with package inventory pairs
    - the gate: a converted app's release is approved and refused like any change, with no package-only activation path
    - the release record's `Declarations.Packages`, left empty by S6, and package presence as a tape fact (D1)
    - derived divergence, so a converted app diverges per package later
    - the fault sweep, rerun for the frontend-only class first
    - the unpushed branch feat/s7-0-app-packages (its design note s7-0-app-packages-2026-10-14.md under docs/design/, and internal/apppackage)
  amendments_s7_owes: >-
    Update the S7 definition for D1 (owner notes s23: install-time API version
    check; host-built catalog only until a sandboxed frame; installed packages in
    the P6 identity report). The frontend drill on one rewrite candidate (never
    mail, settings, autopaper or texture) runs in parallel as a drill of the
    slice 0 witness, not a new station.

receipts: []
---

# S6: the commit gate

The six legs of the reviewed S6 definition stand. Since it was written on
2026-10-01 the loop exists (P1, P4, P5, P6 to P8), the preview was cut,
packages moved to the tape, and the owner ordered this station first. Start by
observing: the cheapest wrong move is to build a gate for joins already
compared, or to trust a join that is not.

Order of work: problem doc, S6-0 and its drill, re-scope S6-1 and S6-2 from the
gap table, S6-3, S6-4. Between slices: `git status --short`, push, CI, deploy,
identity check, deployed proof. No platform deploy while a drill runs.

## Completion cutover

`goal.complete` requires, besides the six legs on staging:

- the agentic-consensus panel on the frozen landed candidate (diff identity and
  deployed receipts), iterated to approval or an accepted, named edge;
- the S6 definition updated in the landing commit: status, the S5 preview
  binding marked dropped with the owner's cut cited, its legs pointing at the
  tracer scenario;
- vmctl `DivergenceStatus` and the offer fields deleted with every citer
  repaired (standing question 3), or the deletion recorded as a named residual
  with the derived value still logged;
- `docs/ACTIVE.md`, `docs/mission-graph.yaml` and
  `docs/doc-authority-manifest.yaml` repointed to S7-0 as the active LP station
  and the S7-0 goal file authored (Goal-Station Handoff); this file kept as the
  landed receipt;
- owner-act mode recorded as enforce, or as a residual with the evidence that
  blocked the flip;
- the CLAUDE.md "Final Evidence" fields: accepted run ids, rollback refs,
  heresy delta, next realism axis.

## Open for the owner (no blocking ask; defaults in force)

1. Ratify the gate semantic: a declared test's nonzero exit refuses activation
   under every authority, the owner's own approval included. Default: refuse,
   with an owner override only as a recorded `owner_waives_test` act on the tape.
2. Should the owner approve the host-built closure (build before approval) or
   the patch (build after, refuse on mismatch)? Default: the patch, with the
   refusal at apply; revisit if S6-0 shows builds are fast enough to precede
   the approval card.
