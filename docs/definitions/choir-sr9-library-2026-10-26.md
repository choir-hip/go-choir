---
definition_version: 5

# SR9 — the Choir Library: an accepted package change on one computer
# becomes available to other computers by an internal mechanism, not by an
# external coding agent pushing main. Station of
# definitions/choir-selfdev-readiness-metamission-2026-10-24.md.
# Owner 2026-10-26, in session: "after selfdev succeeds on an individual
# computer, the way that we get that running on every computer shouldn't be
# that you, an external coding agent, pushes to main. We need to have our
# internal mechanism of propagating changes"; "the internal propagation
# system is tonight's tentpole"; "I think we really need to do an end-to-end
# redesign of that, because I don't think it was made aware of the app
# packages. We need to be thinking in terms of packages"; "it should only
# carry its own change"; "changes should be private until they're published
# by a user. I suppose you could prompt a model to publish things when
# you're asking for the feature, but to me that's still published by the
# user"; "we shouldn't have host with a GitHub push"; on clean rebases
# applying without approval, "not yet"; the name, "to a mainstream audience,
# we can say that this is the choir library". The owner wants all of it
# built, in steps.
readiness: drafted

review:
  reviewer: none yet (tier 2, Devin and DeepSeek; the design governs red steps)
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-26T02:00:00Z
  source:
    canonical_ref: origin/main 355c603ad
    deploy_identity: unknown (staging last recorded a03d5266a in the metamission now card)
  worktrees:
    - path: /Users/wiz/Dev/go-choir
      status: dirty (M go.sum, ?? scratchpad/), the owner's
      class: user_wip
      owner: owner
      touch: forbidden
      recovery: leave in place
    - path: /Users/wiz/Dev/lanes/sr9-design
      status: this lane (lane/sr9-design), docs only
      class: goal_candidate
      owner: SR9 design worker
      touch: goal_owned
      recovery: branch lane/sr9-design
  observed:
    - >-
      The release system (choir.software_release, owner-act publish, P8
      adoption) is on main and was proved once (adopt-1523, 2026-10-15). It
      knows nothing about packages (Declarations.Packages is reserved and must
      be empty, internal/release/record.go:117,203). Under U1 its patch is
      cumulative over the computer's stack (internal/sourceproj/compose.go).
      Its adoption hands the apply to the desks as a prompt
      (internal/agentcore/release_adoption.go:199,336).
    - >-
      Release reads are not owner-scoped: corpusd lets any bound guest read
      any release (internal/platform/releases.go:71), and the proxy lists
      every owner's releases to any caller with the read scope and reads
      single releases as the host (internal/proxy/releases.go:192,204).
    - >-
      Packages exist at build time only: frontend/src/packages/{image,reader},
      choir-package.json v1, check-packages.mjs, package-boundary-plugin.mjs,
      app-sdk.ts. Main is reached by scripts/selfdev-promote plus FP0's push.
    - >-
      T1a's Reader redesign candidate touched paths outside its package
      (app.css, read.html, reader.js, reader-chrome.css, theme.ts), so a real
      candidate today would be refused by a package-confined unit.

finish:
  deliver: >-
    An owner's accepted change to an app package on one computer reaches the
    owner's other computers by subscription, without anyone pushing main: it
    is private until the owner publishes it, carries only its own change, is
    adopted deterministically through each adopter's own gate, is checked on
    probation and reverted once on a regression, is visible in a read-only
    Updates app, and later reaches main verbatim through a reviewed export,
    after which every carrier reports upstreamed. Protected computers adopt
    last and never automatically at first.
  artifact: >-
    The Library as built in steps S0 to S14 of
    docs/design/library-package-propagation-2026-10-26.md: the
    choir.package_release and choir.package_listing kinds with owner-scoped
    reads; choir package publish|share|withdraw|search|history|show|
    subscribe|adopt|revert|export; the deterministic package_adopt job;
    package-aware containment; probation with forward revert and
    suspension; the Updates app; auto mode with rings; the support window;
    signed listings and the security yank; runtime install (module/v1);
    cross-owner adoption in a sandboxed frame; the retired release-adoption
    code deleted.
  acceptance:
    - action: >-
        A second account's key gets 404 on the list, the single read and
        the blob of T-A's releases; b@c's key and T-B read them (S1).
      proves: owner-scoped reads on the live path
      evidence_class: deployed API proof
    - action: >-
        T-A (computer-6c50d6e566143ae32cdb660faa60e24d) publishes a
        package-confined applied operation; a mixed one is refused with its
        outside paths named; the record carries only the package tree (S2).
      proves: the unit carries only its own change
      evidence_class: deployed API proof plus record read
    - action: >-
        T-B, subscribed in propose mode, shows an adoption awaiting approval
        with its own screenshots and zero model calls in its trace, without
        any adopt command; after approval it serves the release and its lock
        names it; pinned adopts nothing; a local edit yields conflict and an
        engineering item (S5).
      proves: discovery and deterministic adoption through the adopter's gate
      evidence_class: deployed product path, trace read
    - action: >-
        choir package export puts the release on a lane, the CI check
        passes, the reviewed landing loop deploys it, and T-A and T-B both
        report upstreamed; a deploy in between showed package_base_moved on
        T-B (S4, S6).
      proves: main receives the change verbatim without a host push, and carriers retire
      evidence_class: deployed identity
    - action: >-
        A release that throws on mount fails T-B's probation smoke and is
        reverted once, forward, with an outcome recorded (S7); T-B in auto
        mode adopts the next release with no human act, while a
        primary-class computer gets it only as a proposal (S9).
      proves: probation, single revert, the class floor
      evidence_class: deployed product path
    - action: >-
        T-B's Updates app lists every update with evidence and has no write
        route (S8); a yank sets T-B's package aside and survives a restore
        (S11); T-B installs a release without a frontend build or restart
        and a deploy leaves it untouched (S12); T-C of a second owner adopts
        a shared release only inside the sandboxed frame (S13).
      proves: the rest of the system as designed
      evidence_class: deployed product path
  rollback: >-
    Per step, its default-off switch (named in the design's section 17) or
    a git revert; rows written stay inert (replay projects any kind). The
    release system on main keeps working until S5 retires its adoption path.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the human hand work between an accepted package change on one
    computer and that change running on the owner's other computers
    (baseline: FP0 runs selfdev-promote, pushes main and runs the landing
    loop for every change, and no second computer gets it except by a full
    platform deploy), while preserving privacy by default, only-own-change
    releases, each adopter's own gate, no host GitHub credential, no
    unapproved rebase, and protected computers last.
  goodharting_would_be: >-
    Adoption that still goes through FP0 or main under another name; a
    release that is package-confined only by a guest's claim; "auto" that
    is really a fleet push without per-computer gates; propose mode whose
    evidence is the publisher's screenshots of the publisher's data; a
    proof on one computer adopting its own release; an Updates app that
    lists only what the Library did.

homotopy:
  realism_axis: >-
    one package release at owner scope, two test computers of one owner,
    propose with FP0 approving -> export retires both as upstreamed ->
    probation and one forward revert -> auto on the test ring with soak ->
    support window, signed listings, security yank -> runtime install, base
    moves no longer touch packages -> cross-owner adoption in the frame ->
    primary auto, the owner's computers propose, the paper last.

boundaries:
  mutation_class: >-
    green for this file and the design; the build steps are red (S1, S2,
    S3, S5, S6, S7, S9, S11, S12, S13) or orange (S4, S8, S10, S14 items)
  review_tier: >-
    the design, tier 2 (Devin and DeepSeek); per step as the design's
    section 17 names it
  authority_sources:
    - owner 2026-10-26 statements quoted in the header
    - owner rulings 2026-10-24 18:5x and 19:0x (first-pass ledger) on updates, the Updates app, Q1 to Q5
    - owner 2026-10-14 distribution plan (docs/reports/selfdev-update-architecture-2026-10-15.md:46-71)
    - SR8 staged auto-approve (metamission station note)
    - the readiness metamission spine
  must_preserve:
    - a package release carries only its package directory; a platform change never rides the Library
    - private by default; every scope change is an owner act with an owner-act receipt; no desk widens a scope
    - the host holds no GitHub push credential; main changes only by the reviewed human or coding-agent path
    - no clean rebase or re-materialization applies without approval until the owner rules (design Q1)
    - the adopter's own gate decides; auto-apply is never auto-promote
    - protected computers adopt last, never automatically at first, enforced in code
    - a forced security change sets a package aside and never contains or stops a computer
    - crash restarts close adoptions; subscriptions are standing intents
    - source only; the host is the sole builder
    - the paper and the owner's computers are never used for proof steps
  excluded:
    - platform (non-package) change propagation other than the existing reviewed main path
    - third-party publishers outside this cloud before S13
    - package data migration across schema versions (P6) beyond SR7's additive rules
  protected_surfaces:
    - auth on release and catalog read routes (S1, S3)
    - Store B kinds and owner acts (S2, S3, S11)
    - a new apply path and capsule seam that bypasses the desks (S5)
    - boot-path source projection (S6)
    - run acceptance: probation and the auto gate (S7, S9)
    - a new code-loading path and a security frame (S12, S13)

now:
  status: working
  slice: >-
    S0 then S1: write the four problem records (green), then build S1,
    owner-scoped release reads, tests first, behind CHOIR_RELEASE_READS.
  source_ref: 355c603ad
  deploy_identity: unknown
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: [internal/proxy/releases.go, internal/platform/releases.go]
  conjecture:
    id: SR9-1
    claim: >-
      If a release is one package directory's tree (own change only) and
      adoption is a deterministic three-way tree swap through the adopter's
      gate, then an accepted package change reaches a second computer with
      no FP0 hand work, and main later absorbs it verbatim so every carrier
      retires as upstreamed.
    test: >-
      S5 and S6 on T-A and T-B: adoption awaiting approval with zero model
      calls and no adopt command, served after approval, upstreamed on both
      after the export lands.
    edge: frame_lock
    delta_o: >-
      measure the share of real self-development candidates confined to one
      package directory (T1a's Reader redesign was not); if most cross the
      boundary, the unit is right but the requests and the SDK are not, and
      the conjecture's scope shrinks until S0's boundary steering lands
    scope_if_supported: frontend packages adopted between computers of one owner
    status: proposed
    evidence_refs:
      - docs/design/library-package-propagation-2026-10-26.md
      - docs/definitions/choir-t1a-app-self-development-2026-10-26.md (now.candidate.scope)
  decision:
    what: >-
      the unit is a package tree, not a patch; platform changes keep the
      reviewed main path; export is a client verb, pushed by the agent's own
      credential
    kind: architecture
    status: proposal
    evidence_ref: docs/design/library-package-propagation-2026-10-26.md
    owner_ratification_ref: >-
      the owner's 2026-10-26 rulings cover packages, own change only,
      privacy, and no host push; the design's open questions Q1 to Q6 await
      the owner
  belief:
    believed_state: >-
      The record, owner-act publish, verified adoption ladder, host builder,
      standing consensus, source projection and package checker exist; the
      unit, privacy, discovery, deterministic adoption, probation, export
      and the Updates app do not.
    main_uncertainty: >-
      Whether a capsule can run a fixed recipe under a runtime-owned
      assignment without an engineering run (design 6.4 [H]), and how often
      real candidates stay inside one package.
    next_observation: >-
      S1's failure tests, then a read of the capsule executor's open and
      Freeze entry points for S5.
  blocker_or_risk: >-
    Until S12 or an owner ruling on Q1, every deploy asks each adopter to
    approve a re-materialization; the export (S4) keeps that window short.
  next_action: >-
    Land S0's four problem records (green, tier 0), then build S1 in a lane
    on current main with its tests first and send it to tier 1 review.

receipts: []
---

# SR9: the Choir Library

The design is
[docs/design/library-package-propagation-2026-10-26.md](../design/library-package-propagation-2026-10-26.md).
Its section 17 is the step list this station runs, in order, each step
landed alone behind its switch with its deployed proof on T-A
(`computer-6c50d6e566143ae32cdb660faa60e24d`) and T-B (a second b@c test
computer, described in its section 18). Its section 19 holds the owner's
open questions, and each has a conservative default the build uses until
the owner rules.
