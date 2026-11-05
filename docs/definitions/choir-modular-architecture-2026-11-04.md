---
definition_version: 4

# Modular architecture: every app and every desk is a package with privilege
# only from grants, the kernels are small and structurally gated,
# precommitment records are the only channel between desks, custody holds per
# kind, and the Library distributes every package kind. Then self-development
# restarts on it.
#
# Owner, 2026-11-04 ~20:55Z: "This is a whole mission here. Let's supersede
# the current metamission and make this a new throughline."
# Supersedes docs/definitions/choir-overnight-pd-then-selfdev-metamission-2026-11-03.md.
# The self-development goal (choir-selfdev-apps-feature-complete-2026-11-03.md)
# stays paused; Phase 8 resumes it.
#
# THE PLAN is docs/design/modular-architecture-plan-2026-11-04.md (branch
# modplan; phases 0 to 8, slices MA*, the decisions table R1 to R38, open
# questions). This file does not restate it. Where this file and the plan
# disagree on a slice, the plan wins; where they disagree on authority,
# boundaries or the finish, this file wins.
#
# Rewritten 2026-11-04 ~21:30Z (the morning version planned "waves W0 to W8"
# from the sysmod and appdesk designs; the consolidated plan replaced them).
readiness: executable  # owner MA0.5 sign-off 2026-11-04 ("Yes I sign off")

review:
  reviewer: Devin (one seat)
  frozen_ref: 84b99adbc
  verdict: accept  # 0 blocking findings; 8 non-blocking amendments, folded
  evidence_ref: "Devin goal-file review 2026-11-04 (scratchpad review-goal/devin.out)"

metamission:
  # Stations are the plan's phases. stations[].path is a plan locator (a plan
  # section), not a goal-file path. One file carries them; a phase gets its
  # own goal file only when it needs more than its plan section.
  stations:
    - {id: P0_evidence, path: "plan section 3", readiness: drafted, status: working, depends_on: []}
    - {id: P1_records_only_channel, path: "plan section 4", readiness: drafted, status: pending, depends_on: [P0_evidence]}
    - {id: P2_kernels_manifest, path: "plan section 5", readiness: drafted, status: pending, depends_on: [P0_evidence]}
    - {id: P3_custody_labels, path: "plan sections 7 and 8", readiness: drafted, status: pending, depends_on: [P1_records_only_channel, P2_kernels_manifest]}
    - {id: P4_desks_packages, path: "plan sections 6 and 9", readiness: drafted, status: pending, depends_on: [P3_custody_labels]}
    - {id: P5_apps_services_packages, path: "plan section 11", readiness: drafted, status: pending, depends_on: [P2_kernels_manifest, P4_desks_packages]}
    - {id: P6_isolation, path: "plan section 12", readiness: drafted, status: pending, depends_on: [P5_apps_services_packages]}
    - {id: P7_bayesian_trust, path: "plan sections 10 and 13", readiness: drafted, status: pending, depends_on: [P3_custody_labels, P6_isolation]}
    - {id: P8_selfdev_restart, path: "plan section 14; resumes docs/definitions/choir-selfdev-apps-feature-complete-2026-11-03.md", readiness: drafted, status: pending, depends_on: [P7_bayesian_trust]}
  order: >-
    The owner's roadmap order (R17) is the order of completion. File and data
    dependencies are in plan section 15; Phase 5's front-end services may
    overlap Phases 3 and 4 once MA2.2 and MA2.3 land. Every phase opens with a
    committed ledger round (plan section 0 item 7) before its first slice
    lands. Nothing in Phases 1 to 8 lands before MA0.5 (R12).

start:
  # Original capture, immutable.
  captured_at: 2026-11-04T21:10:00Z
  source:
    canonical_ref: origin/main 93706d53c1cbd170529a50466add51f8a586599e (fetched at capture)
    deploy_identity: >-
      staging https://choir.news/health build.deployed_commit
      789a141d99dc107b805f153e9ff5e3df9ac83ade; security epoch 41. Main is ahead of
      the deploy by docs commits and CI-queued work; identity is rechecked at the first landing.
  landed_today: >-
    clean rebase on every computer (8ae42dd3b, epoch 41); the screens rule
    (c4c88868f, b8ff66f59, 10404b48e); the Library app store (86abfe8f4, tier 2
    round 2); Files as a first-party package (0353a5b3f, 582e30f22); rejected
    rounds sent back, not settled complete (41eb0e336, f32966551); provider quota
    switches models instead of parking (7e2d36fd7, 7d2cc392a); the
    forced-modularity design (docs/design/forced-modularity-build-2026-11-04.md,
    23db7bd83..e063b3d9b).
  self_development: paused by owner order (2026-11-04 ~20:05Z) until the modular architecture has landed.
  inputs:
    - ref: branch sysmod @ 69753f045, docs/research/plugin-architectures-2026-11-04.md
    - ref: branch sysmod @ 73e64110c, docs/design/modular-architecture-2026-11-04.md
    - ref: branch appdesk, docs/design/app-agent-desks-2026-11-04.md
    - ref: main, docs/design/forced-modularity-build-2026-11-04.md (e063b3d9b)
  worktrees:
    - {path: /Users/wiz/Dev/go-choir-sysmod, status: clean, class: goal_candidate, owner: this mission, touch: goal_owned, recovery: "branch sysmod, 3 docs commits unpushed"}
    - {path: /Users/wiz/Dev/go-choir-appdesk, status: dirty, class: goal_candidate, owner: this mission, touch: goal_owned, recovery: "commit the design file on branch appdesk; do not discard"}
    - {path: /Users/wiz/Dev/go-choir, status: dirty, class: user_wip, owner: owner and other agents, touch: forbidden, recovery: "leave in place; never commit here"}
    - {path: /Users/wiz/Dev/go-choir-modgoal, status: clean, class: goal_candidate, owner: this authoring pass, touch: goal_owned, recovery: "branch modgoal"}

  start_addendum_2026-11-04T21:28Z:
    # An addendum, not a correction: the original observations stand. It
    # records the state the rewritten file starts from.
    why: the consolidated plan replaced the sysmod and appdesk designs as the build plan, and the owner's rulings R1 to R37 changed the finish (line budgets dropped, R1)
    canonical_ref: origin/main 9b5eb066b5beef0e4b0036776d7661e467c0651c (fetched 21:28Z)
    deploy_identity: staging build.deployed_commit 93706d53c1cbd170529a50466add51f8a586599e, deployed 2026-11-04T18:25:03Z
    plan_ref: branch modplan @ 9f32a1a90 (unpushed), docs/design/modular-architecture-plan-2026-11-04.md
    evidence_on_modplan: docs/evidence/gvisor-desk-capsule-x86-2026-11-04.md, docs/evidence/custody-inventory-2026-11-04.md, docs/design/bayesian-trust-2026-11-04.md (copy)
    worktrees:
      - {path: /Users/wiz/Dev/go-choir-plan, status: clean, class: goal_candidate, owner: FP0 and doc agents, touch: goal_owned, recovery: "branch modplan; lands on main after MA0.5"}
      - {path: /Users/wiz/Dev/go-choir-d0, status: clean, class: goal_candidate, owner: this mission, touch: read_only, recovery: "branch d0-protocol (632d98385, ad80f2330, e63e2f536), unpushed; bakes MessageTargets into the lock; re-cut by MA1.4 (cherry-pick tooling, drop the address surface); never merge as is"}
      - {path: /Users/wiz/Dev/go-choir-land-docs, status: dirty, class: goal_candidate, owner: FP0, touch: read_only, recovery: "untracked docs/design/bayesian-trust-2026-11-04.md, left unpushed by owner order; the plan's copy supersedes it (Devin amendment 5)"}
      - {path: /Users/wiz/Dev/go-choir-apl, status: clean, class: other_agent_wip, owner: Autopaper learning design, touch: forbidden, recovery: "branch autopaper-learning, equal to origin/main 9b5eb066b; belongs to the later Autopaper mission"}
      - {path: /Users/wiz/Dev/go-choir-sysmod, status: clean, class: goal_candidate, owner: this mission, touch: read_only, recovery: "branch sysmod 73e64110c; superseded designs, referenced by the plan, not landed separately"}
      - {path: /Users/wiz/Dev/go-choir-appdesk, status: clean, class: goal_candidate, owner: this mission, touch: read_only, recovery: "branch appdesk 069c18f84; as sysmod"}
      - {path: /Users/wiz/Dev/go-choir, status: clean, class: user_wip, owner: owner and other agents, touch: forbidden, recovery: "local main 303eff7c0; never commit here for this mission"}

finish:
  deliver: >-
    Choir's base is fully modular, on main and deployed through every ring of
    staging: two small kernels held by structural gates; every app, shell
    surface and desk, Texture and the former system desks included, an
    installed package whose privilege comes only from its grants; desks
    reaching each other only through typed, scored precommitment records;
    custody per object kind with leases for cross-desk reads; the sandboxes
    in place before any community desk; Bayesian trust over the records;
    and the Library distributing every package kind. Then self-development
    restarts on it.
  artifact:
    - kernels internal/kernel and frontend/src/lib/kernel with the three structural gates (imports, names, responsibility list) in the repo gates and CI (R1 replaced the line budgets)
    - desk protocol 1.0.0 locked with no address allowlist, the required CI job desk-protocol, emit and message channels deleted, every request carrying p_success, weight, settlement, reducer-stamped provenance and a materiality_procedure ref (custodian catalogue, frozen and hashed; required by MA1.3)
    - manifest v2 with typed capability ceilings, publishing levels, behaviour-surface hash; v1 reader and adopt deleted
    - every WriterRuntime kind with one custodian; writer and reader gates enforcing; leases on the tape; labels propagating; the disclosure path
    - every desk a desk package; CHOIR_APP_DESKS, app_packages.objects and the System 1 caps deleted; the desk-names gate at zero after Texture moves
    - every system app and shell surface a package; the static rows of frontend/src/lib/apps/registry.ts gone; Autopaper moved as a package (C8) and nothing more
    - iframe sandbox, broker hardening and fuzzing, and the worker sandbox Phase 0 selects, before any non-first-party desk package installs
    - the derived trust view, leases sized by trust inside the ceiling, the act gate, and the MA7.9 evidence harness
    - GET /api/plugins/state on a fresh computer showing only the base image's default packages
    - the theory and practice docs (MA8.0)
  acceptance:
    # Per phase: the deployed observation that closes it. Slice proofs are in
    # the plan's tables and run per slice. Every phase also closes with its
    # ledger round settled by a settler other than FP0.
    - action: "P0: the gVisor evidence committed; Devin's plan verdict; the owner's walkthrough and sign-off recorded here (MA0.5); MA0.6 pre-registration and MA0.7 ledger round 1 committed"
      proves: the plan is owner-ratified and its predictions precede their evidence
      evidence_class: committed docs and the owner statement
    - action: "P1 on d@e: a Texture-to-research cast and a research fan-out each leave request records with p_success, weight, settlement and provenance, read back through the product API; a research-to-engineering cast lands in management's inbox, not as an assignment; the frame self-test shows no emit or message action; desk-protocol job green and required; Texture acceptance unchanged"
      proves: precommitment records are the only inter-desk channel and the lock enforces it
      evidence_class: CI plus deployed acceptance on d@e
    - action: "P2 on d@e: GET /api/plugins/state with no SSH; kernel gates with a negative fixture each; the K0 leak differential; runtime-install.spec.js; no adopt route; the four desks run an ordinary day with identical routing"
      proves: the kernels and manifest v2 carry the package path
      evidence_class: CI plus deployed acceptance
    - action: "P3 on d@e: a wrong-writer write and a non-custodian read are refused; a desk without a lease is refused, the owner issues and revokes a lease and the next read fails; a labelled run's disclosure decisions are on the tape; Files no longer lists mail/"
      proves: custody holds per kind at the frame, the store and the service layer
      evidence_class: adversarial drill on d@e plus deployed acceptance
    - action: "P4 on staging: health lists the derived desk set; a fixture desk asks Mail for one email by ref through a record, gets a per-task lease, is refused a second; the prompt router records its decision distribution and a judge failure takes the default route; a platform update applied by the packaged management desk; a full self-development operation by the packaged engineering desk on d@e; a crash during a capsule closes it interrupted by a restart"
      proves: every desk runs through the one package path with no identity privilege
      evidence_class: deployed acceptance by ring
    - action: "P5 on choir.news by a fresh test account, no SSH: plugins state lists the defaults; install a context-menu provider from the Library and see its menu on a file in Files; make it throw and see it un-ranked with the base menu served; yank it (platform break-glass) and see the fallback; uninstall and reinstall Calendar with its data intact; Texture app and desk land as one package and the names gate reads zero"
      proves: the modular path works end to end, including failure, yank, data survival and the desk lifecycle, with no special-cased system app
      evidence_class: deployed acceptance with before and after screenshots and the plugin-state snapshot
    - action: "P6 on d@e: a hostile fixture app in the iframe cannot read the shell DOM, the session or another app's routes; broker fuzz corpus in CI; the worker sandbox's cell benchmark within the owner's accepted ratio and its boot selftest visible through the product API (POST /api/runtime/isolation-selftest); only then a non-first-party desk package installs"
      proves: isolation precedes community desks (R24)
      evidence_class: adversarial drill on d@e plus CI
    - action: "P7 on d@e: replay reproduces the trust view and a later outcome leaves a settled score unchanged; a strong-record fixture gets a durable lease and a thin one stays per-task; a planted-bad package is gated; the MA7.9 harness reports the pre-registered tests"
      proves: trust is computed from records and governs leases inside the ceiling
      evidence_class: replay plus adversarial drill on d@e
    - action: "P8: the plan section 14 entry criteria hold; then one Files and one context-menu self-development round land as in-place updates with before and after screenshots"
      proves: self-development works on the modular base
      evidence_class: deployed acceptance and round receipts
  rollback: per-slice revert (each slice table names its rollback); M2 and M3 revert together; revert C13 first if it has landed (MA4.7's desk and UI halves are split); MA6.4 lowers isolation only by an owner act; tape records written stay decodable; planned-restart markers for any deploy-time restart
  landing:
    required: true
    environment: staging (choir.news), d@e first, then by ring per the tempo rules below
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the privileged, hard-coded surface of the base (code that names a
    desk or app, privilege keyed on identity or a closed list, state reachable
    without a grant or lease, messages outside the record protocol) while
    every first-party app and desk keeps working and every package change
    reaches the user through the same install, grant and rollback path.
  goodharting_would_be: >-
    Moving files into packages while privilege still keys on identity (a
    closed list, textureCellAuthorizer, a hard-coded desk name); kernel gates
    passing because logic moved into an unlisted service; a protocol lock that
    nothing enforces at runtime; records carrying p_success that is constant
    or inert and never scored; custody gates in observe mode counted as
    enforced; registry rows deleted without the leak differential and the
    installed-mode specs green; first-party packages that cannot be
    uninstalled or replaced; a switch added to land a slice.

homotopy:
  realism_axis: >-
    the share of the real system whose authority runs through grants and
    records: from one channel (records only, Phase 1), through kernels and
    manifests, enforced custody, every desk, every app and surface, sandboxed
    third-party code, to trust-sized leases, with the first party never
    special-cased at any step

boundaries:
  mutation_class: varies per slice as the plan's tables state (green Phase 0 and docs; orange and red most build slices; red / tier 2 for routing, tape types, custody, boot and isolation)
  operating_tempo:
    status: settled (owner, 2026-11-04)
    owner_quote: >-
      "I still want it aggressive. Code review is good, especially for
      security stuff, but we can often pipeline it as a concurrent check, not
      blocking... the bad scenario is multiple iterations on code review and
      fixes without actually running it. Since the runtime checks are the key
      anyway. We should be expediting most things. Security critical can go
      through progressive delivery and code review simultaneously."
    rules:
      - Runtime checks are the verifier. Code review never blocks merge or the first deploy; it runs concurrently with CI and deploy, one round per change, at the plan's tier. Findings become fixes verified by runtime checks, not re-review loops; re-review only for a blocking-real defect that would be silent.
      - Most changes land with Ring-Expedite (mechanical moves, app and service packaging, channel deletions, kernels, manifest, tooling).
      - "Security-critical changes (custody gates MA3.1 to MA3.3, leases MA3.4 and MA7.4, the disclosure path MA3.6, sandboxes MA6.x, record routing MA1.2, anything that could leak data silently) walk the rings instead of expediting, with a Devin review running at the same time. Hinge: such a change does not leave the test ring (reach the owner's computers or the paper) until its review has reported and any blocking-real finding is fixed. Its runtime proof includes an adversarial negative drill on d@e (for example a desk without a lease is refused)."
      - Unchanged - one-way migrations and data deletes are proved on d@e before merge (CLAUDE.md); the keep set; the restart rules; the cheap gates run first on every slice.
    provenance_note: the hinge was written by the orchestrator and put to the owner on 2026-11-04; the owner confirmed it ("Sounds good", 2026-11-04)
  authority_sources:
    - the owner rulings R1 to R38 in plan section 1 (quoted rulings are owner authority; rows marked "recorded summary" are owner authority as summarized, and a quote supersedes them)
    - the operating_tempo ruling above
    - docs/design/modular-architecture-plan-2026-11-04.md as the build plan, Devin-accepted; ratified by the owner at MA0.5 (2026-11-04)
    - docs/choir-doctrine.md, CLAUDE.md, docs/standing-questions.md
  must_preserve:
    - the keep set (newspaper content, the Autopaper object graph, the owner's emails, Textures, precommitment records, the agent record)
    - '"No cross reads!" (owner): an object kind is read and written by its custodian; others only through a lease or a record request; Texture is the sole writer of canonical document revisions'
    - authority never travels with a message; any desk may send a record to any desk; the recipient acts on its own grants
    - restart rules - a crash restart closes open obligations as "interrupted by a restart" and never resumes; only a consumed planned-restart marker resumes; standing intents survive a crash, occurrences do not; never restart mid self-development apply or with a live capsule
    - no switches; every slice ships on at d@e and widens by ring; a correctness refusal (including the Phase 6 isolation refusal for non-first-party desks) is not a switch
    - one trust model for generated and third-party code; privilege from grants and tier, never from being first-party
    - app updates per user per app, manual by default; forced update and yank platform-only break-glass; builders publish without approval; publisher identity is the maild numbered address with no fallback
    - no spend ceiling; cost attributed and monitored per desk (R8)
    - code review never uses luna (R12)
    - no secrets or owner email addresses in the repo or output
    - never act on the paper or the owner's computers except through deploys; adversarial drills and red teaming only on d@e; b@c and the owner's computer never deleted
    - Node B memory above vmctl's floor; stagger compiles on the Mac and the agents VM
    - committed precommitment records are never edited; FP0 never settles its own records
  excluded:
    - "Autopaper: moving the Autopaper app to a package (C8) is in scope, as the precursor to the Autopaper mission. Everything else (BM25 and graph search, self-citation, the Texture cite rule, typed relations and relies_on, research.related_story, Autopaper app agents, Autopaper floors) waits for the Autopaper mission, even small items (R37)"
    - self-development rounds before Phase 8; self-development deleting base code
    - out-of-process tool servers (removed by the owner, 2026-11-04; a package backend is its desk); third-party Go modules outside the R42 capability gate (import closure with no unsafe, syscall, os/exec or raw sockets; curated list first)
    - zero-downtime or compatibility machinery beyond a slice's rollback; interim layers (R16)
    - the tape app, update-now-or-later prompt and email-as-app-agent ideas beyond MA4.7
  protected_surfaces: >-
    the tape, the commitment reducer and the record protocol lock; Texture
    canonical writes; the object-graph writer and reader gates and store
    handles; leases and the disclosure path; auth/session and grants; the
    package install path and manifest validator; boot, reconcile and
    planned-restart handling; vmctl and ring deployment; the desk worker
    confinement and sandbox; gateway attribution. Red slices carry conjecture
    delta, admissible evidence, rollback and heresy delta in their commit or
    ledger line.

now:
  status: working
  slice: >-
    Phases 1 and 2 closed 2026-11-05 04:31Z (every MA1 and MA2 slice on main,
    staging 4b8cf26ad). Now: land 222d946f8 (desks see their own inert count)
    after the committed baseline, then the Phase 3 prediction round (plan
    rule 7), then Phase 3 custody and labels from MA3.1.
  source_ref: origin/main 4b8cf26ad
  deploy_identity: staging 4b8cf26ad35918109d7335a06185b284728151f2
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: C1 (bridge)
    claim: >-
      If every app and desk is a package whose privilege is its grants, under
      structurally gated kernels, with records as the only channel and custody
      enforced per kind, then self-development becomes reliable and secure (a
      change is an install, a grant and a revert, not a base edit) and app
      agents follow as desk packages.
    test: >-
      Phase 8 rounds land Files and context-menu updates with no base-code
      edit and no request needing a privilege a grant does not name; the
      hostile fixture package (throws, over budget, asks for a privileged
      grant, writes another app's kind, asks Mail for a second email) is
      contained on d@e.
    edge: missing_oracle
    delta_o: >-
      the names gate and the grep gates for deleted names; the reader and
      writer gates in enforce mode with counters readable through the product
      API; the hostile fixture as a standing drill; the leak differential.
    scope_if_supported: first-party apps and desks migrated here and the Library's package kinds; not community desks at scale nor Autopaper as an app agent
    status: proposed
    evidence_refs: [plan modplan 9f32a1a90, precommit R0-08 and R0-09]
  # Key conjectures beside the bridge (tested inside their phases):
  #  C2 (P1): every inter-desk interaction expressed as a typed request record
  #    loses no product behaviour. Test: MA1.2's contract test and unchanged
  #    Texture, research, management and engineering acceptance on d@e.
  #    Edge frame_lock: Note's information-only use may have no typed kind
  #    (open question 3).
  #  C3 (P3): custody can enforce without breaking the desks. Test: R0-11 and
  #    R0-12 predict new writers and cross reads will surface; counters over a
  #    recorded d@e day. Edge missing_oracle: unexercised paths.
  #  C4 (P6): the worker sandbox costs inside an acceptable ratio. Evidence:
  #    cold build 1.5x, warm 4.5x (R0-01 contradicted), runsc must be the
  #    outer layer (R0-07). Owner settled gVisor 2026-11-04 (R39 to R41); the ratio is the measured one.
  #  C5 (P7): materiality-weighted posteriors predict the next material
  #    failure better than an allowlist or popularity (R0-15; MA0.6). Edge
  #    resource: few settled material events (n < 30 resolves unresolved).
  decision:
    what: adopt the consolidated plan (phases 0 to 8, the MA slices, the defaults of its open questions) as the build plan
    kind: architecture
    status: ratified
    evidence_ref: plan modplan 9f32a1a90; Devin ACCEPT (precommit R0-08)
    owner_ratification_ref: MA0.5, owner 2026-11-04 ("Yes I sign off"), after the walkthrough that settled R39 to R42 and removed tool servers
  belief:
    believed_state: >-
      Desks talk only through records; one data-driven resolver routes them;
      requests carry p_success, weight and settler; sub-calls and delegations
      are scored Casts with a blind asynchronous judge; inertness is counted;
      the desk protocol 1.0.0 is locked with every registry authority field;
      a final Report cannot close a round over an unread Cast. Security epoch
      51. The sub-call path is not exercised on d@e (no prompt teaches
      choir.Research); every typed request on d@e is inert (no typed question).
    main_uncertainty: >-
      whether custody enforcement (Phase 3) breaks desks on paths d@e never
      exercises, and whether the judge should settle desk-to-desk Casts
      (owner question d)
    next_observation: the Phase 3 prediction round committed, then MA3.1's custodian assignment against the writer registry
  blocker_or_risk: >-
    Owner questions open (none blocking): (a) other publishers' front-end
    packages before the iframe sandbox, (b) the 19 hidden v1 releases,
    (d) judge on desk-to-desk Casts. Stranded d@e trajectory 5c725f71 needs
    an owner revise. MA1.4 residuals N5, N6, N8, N9.
  next_action: >-
    Land 222d946f8; author and commit the Phase 3 prediction round; start
    MA3.1 (custodian per WriterRuntime kind) with a Sonnet builder.

receipts:
  - id: P0_plan_review
    boundary: define
    identity: modplan 9f32a1a90 (plan reviewed at 611b526af)
    proof_refs: [precommit R0-08 settled (ACCEPT), R0-09 settled (0 blocking)]
    rollback_ref: not_applicable
    disposition: plan accepted by Devin; 7 non-blocking amendments to fold
  - id: P0_gvisor_evidence
    boundary: define
    identity: docs/evidence/gvisor-desk-capsule-x86-2026-11-04.md (modplan)
    proof_refs: [precommit settlements R0-01 to R0-07 (1876a56da)]
    rollback_ref: not_applicable
    disposition: measured; R0-04 still with Devin; gVisor settled by the owner (R39 to R42); MA0.8 arm64 experiment and the capsule pilot (MA6.7) remain
---

# Modular architecture

The base becomes two small kernels and a locked record protocol, and
everything else is an installed package whose privilege is its grants. The
build plan is
[modular-architecture-plan-2026-11-04.md](../design/modular-architecture-plan-2026-11-04.md);
the companion ledgers are
[the move ledger](choir-modular-architecture-2026-11-04.ledger.md) and
[the precommitment ledger](choir-modular-architecture-2026-11-04.precommit.md).

## Standing questions

**1. Decision provenance.** The owner settled the direction (R2, R3, R5, R17)
and the rulings in plan section 1. Rows marked "recorded summary" are the
orchestrator's summaries of owner speech: binding as summarized, demoted
where a quote disagrees. The plan itself, its slice boundaries, and the
recommended answers to open questions 3, 4, 5, 7, 8, 13 and 16 are
orchestrator proposals until MA0.5. The tempo hinge is an orchestrator
proposal put to the owner.

**2. Settled-decision conformance.** The plan conforms to no switches (it
deletes CHOIR_APP_DESKS, app_packages.objects and the System 1 caps), the
restart rules (MA4.2, MA4.4) and the keep set. Gaps covered: the doctrine I26 and CLAUDE.md "system desks" amendment is
carried by MA8.0 (or an earlier green slice); the `docs/precommitment-records.md`
3.1 erratum pointer to R32 is in the plan (erratum, covered).

**3. Deletion citers.** MA1.1, MA1.2, MA2.4 and MA4.11 carry grep gates for
code names. MA1.1's list now covers the emit receive path (MA1.7). Residual
obligation: each deletion slice's first step greps all four surfaces (code,
docs, specs, skills) and repairs citers in the same commit. Known doc citers
to repair: doctrine I26, `docs/ACTIVE.md` ("wave W8"),
`docs/mission-graph.yaml:1215` ("resumed by wave W8"), the sysmod and appdesk
designs (superseded, plan section 2).

**4. Single state authority.** The tape is the authority for records,
leases, labels and install events. The trust view is derived and never stored
(MA7.2). Desired desk state is the fold of install events plus the
image's required set (MA4.4). Gap covered: MA3.1 names one winner for the custodian declaration. During Phase 5, `registry.ts` rows and installed
packages coexist per app until each C slice. Each C slice deletes its row,
so no app is served by both.

**5. Artifact-verified success.** Each phase closes on the deployed
observations in `finish.acceptance`, read through the product API, never on
a builder's report. Gap covered: the positive artifact (a request record on the d@e tape with
p_success and provenance, fetched back) is in MA1.3's proof.

**6. Fate-sharing.** The prompt router fate-shares with the System 1 judge.
The plan handles that: a judge failure or timeout takes the default route
(section 9.2). MA1.5 makes the judge the settler of sub-RLM casts. Gap covered (MA1.5): settlement is asynchronous, so research answers do not
fate-share with the judge; the role gate and caps of `system1_verb.go` are stated.

**7. Restart durability.** Kernels rebuild activation at boot and never
persist a pending call (MA2.1 boot fold). MA4.2 closes open obligations in one
pass. MA4.4 re-seeds the required set. Leases are tape records. Gap covered (MA3.4): a per-task lease ends when a crash closes its run; a
durable lease (MA7.4) survives a restart only as a tape record re-validated at
boot. Residual: open question 11 (a planned-restart tray carrying a retired message
intent) must be checked before MA1.1 lands.

**8. No-SSH operability.** `GET /api/plugins/state`, the derived desk set in
health, the owner lease view and the per-desk cost view are product paths.
Gaps covered (MA3.2, MA6.4): the counters and the boot selftest are exposed
through the product API. The Phase 0 gVisor measurement was done over SSH on
choir-agents. That is acceptable as evidence, not as an operation.

**9. Registry hygiene.** `docs/ACTIVE.md` and `docs/mission-graph.yaml`
point at this file. `docs/doc-authority-manifest.yaml` has no entry for it,
nor for the plan. ACTIVE.md and `docs/mission-graph.yaml:1215` still speak of waves. The landing of modplan on
main updates all three. MA8.0 repoints them again at Phase 8.

## Notes: plan defects found while writing this file

These are editing defects to fix when Devin's amendments are folded, not
design changes:

- Section 1 (decisions table) contains slice rows. MA1.5 and MA1.6 sit between
  R4 and R5. MA6.5 is merged into R13 and MA4.8 into R18. MA1.5 and MA1.6 are
  missing from the 4.3 slice table.
- Section 0 numbers its principles 1, 2, 3, 4, 6, 7, 8, 9, 5.
- Section 8.4's X3 row has an MA4.6 slice row merged in. That version (deps
  MA3.4 and MA1.5, two child roles) differs from MA4.6 in 9.1.
- Section 2 points gate G at "section 11". Phase 8 is section 14.
- MA4.7 (Phase 4) depends on S1, S2 and S7 (Phase 5). Either those services
  land early through the allowed overlap, or Mail moves to Phase 5.
- MA1.5 (Phase 1) uses "the kernel judge service", which only exists after
  MA2.1 and MA3.6.
- Section 12.1 and MA6.4 said "if Phase 0 holds". Phase 0 measured runsc as the
  outer layer, with warm builds at 4.5x; the owner settled gVisor on 2026-11-04
  (R39 to R42, open questions 2 and 9 settled).
