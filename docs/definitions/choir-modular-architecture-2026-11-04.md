---
definition_version: 5

# Modular architecture: every app and every desk is a package, the kernels
# stay small, the desk protocol is locked, and the Library distributes every
# kind of package. Then self-development restarts on it.
#
# Owner, 2026-11-04 ~20:55Z: "This is a whole mission here. Let's supersede
# the current metamission and make this a new throughline."
# Supersedes docs/definitions/choir-overnight-pd-then-selfdev-metamission-2026-11-03.md.
# The self-development goal (choir-selfdev-apps-feature-complete-2026-11-03.md)
# stays paused and is resumed by the last station, not rewritten.
#
# Owner rulings carried as authority (all 2026-11-04):
#  - "more modular is better": maximal modularity benefits self-development
#    and especially security.
#  - Every app and every desk is a package. First-party ones are preinstalled.
#    There are no special system apps or desks; privilege comes only from
#    grants and tiers. The kernel stays small.
#  - A locked, versioned protocol between desks (casts, delegation, the
#    commitment ledger) is enforced regardless.
#  - The migration of system apps into packages is done by us on main, not by
#    self-development ("self-development cannot and is not meant to delete
#    base code"). Self-development restarts only once the full modular
#    architecture has landed.
#  - The Library distributes service providers, themes and command packs, not
#    only apps. Publisher identity is the maild numbered address (never the
#    auth email; no fallback).
#  - App updates are a per-user, per-app setting, manual by default. Builders
#    publish without approval ("the whole point of library is so I don't have
#    to approve"). A forced update or yank is platform-only, manual and
#    break-glass.
#  - No feature switches. A crash closes work; a planned restart marker may
#    resume it.
#  - Screenshots are required for self-development, with before and after
#    for modifications.
#  - Expedite everything in the hill-climbing loop.
#  - Sonnet builders do the work; Opus orchestrates. About 15% of the weekly
#    token budget is left, so prefer Sonnet.
readiness: drafted  # moves to reviewed after the Devin review of the architecture design

review:
  reviewer: none yet; the design (sysmod 73e64110c) is in Devin review; this file is reviewed after it
  frozen_ref: none
  verdict: none
  evidence_ref: none

metamission:
  # Stations are waves. One file carries them; a wave gets its own goal file
  # only when it starts and needs more than its section here.
  stations:
    - id: W0_foundations
      path: this file, "Wave W0"
      readiness: drafted
      status: working
      depends_on: []
      slices: [K0, G0, P0, M1, R1, D0, D1]
    - id: W1_services_d0
      path: this file, "Wave W1"
      readiness: drafted
      status: pending
      depends_on: [W0_foundations]
      slices: [S1, S2, S3, S4, S5, S6, S7, W1, U1, D2]
    - id: W2_app_batches
      path: this file, "Wave W2"
      readiness: drafted
      status: pending
      depends_on: [W1_services_d0]
      slices: [C1-C11, C12, C13]
    - id: W3_shell_surfaces
      path: this file, "Wave W3"
      readiness: drafted
      status: pending
      depends_on: [W1_services_d0]
      slices: [H1, H2, H3, H4]
    - id: W4_desks
      path: this file, "Wave W4"
      readiness: drafted
      status: pending
      depends_on: [W0_foundations, W1_services_d0]
      slices: [D3, D4, D5, D7, D8]
    - id: W5_texture
      path: this file, "Wave W5"
      readiness: drafted
      status: pending
      depends_on: [W2_app_batches, W3_shell_surfaces, W4_desks]
      slices: [T1, D6]
    - id: W6_forced_modularity
      path: this file, "Wave W6"
      readiness: drafted
      status: pending
      depends_on: [W0_foundations]  # M2+M3 may overlap W2; X1 follows W2 and M5
      slices: [M2+M3, M4, M5, X1, F1]
    - id: W7_library_kinds
      path: this file, "Wave W7"
      readiness: drafted
      status: pending
      depends_on: [W0_foundations, W6_forced_modularity]
      slices: [L1, L2, L3, L4]
    - id: W8_selfdev_restart
      path: docs/definitions/choir-selfdev-apps-feature-complete-2026-11-03.md (resumed) plus the gate G
      readiness: drafted
      status: pending
      depends_on: [W5_texture, W6_forced_modularity, W7_library_kinds]
  order: >-
    W0 starts now (group A: K0, G0, P0, with D0 and M1 and R1 beside them, all
    disjoint packages). W1 follows on K0 and P0. W2, W3, W4 and W6 overlap
    once their dependencies land (conflict rules in the design section 5).
    W5 is last of the build. W8 only when gate G holds.

start:
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
    - ref: branch sysmod @ 69753f045, docs/research/plugin-architectures-2026-11-04.md (worktree /Users/wiz/Dev/go-choir-sysmod)
    - ref: branch sysmod @ 73e64110c, docs/design/modular-architecture-2026-11-04.md (the build plan; in Devin review)
    - ref: branch appdesk, docs/design/app-agent-desks-2026-11-04.md (untracked in /Users/wiz/Dev/go-choir-appdesk at capture, still being written; D0 to D10)
    - ref: main, docs/design/forced-modularity-build-2026-11-04.md (e063b3d9b)
  worktrees:
    - path: /Users/wiz/Dev/go-choir-sysmod
      status: clean
      class: goal_candidate
      owner: Sonnet designer, this mission
      touch: goal_owned
      recovery: >-
        branch sysmod, 3 docs commits unpushed (0c87a396b, d11ba0138, 73e64110c); first action pushes the design and research docs to main after the Devin review verdict, then the worktree is removed
    - path: /Users/wiz/Dev/go-choir-appdesk
      status: dirty
      class: goal_candidate
      owner: Sonnet designer, this mission
      touch: goal_owned
      recovery: the design file is untracked; commit it on branch appdesk when the designer reports, then land with the architecture docs; do not discard
    - path: /Users/wiz/Dev/go-choir
      status: dirty
      class: user_wip
      owner: owner and other agents
      touch: forbidden
      recovery: untracked evidence docs (selfdev slides-next compares, trace review, stale-open-operations problem) belong to the paused selfdev station; leave in place; never commit here
    - path: /Users/wiz/Dev/go-choir-modgoal
      status: clean
      class: goal_candidate
      owner: this authoring pass
      touch: goal_owned
      recovery: branch modgoal; one green commit, not pushed by the author

finish:
  deliver: >-
    Choir's architecture is fully modular, on main and deployed to every ring
    of staging: a small front-end kernel and a small Go kernel; every app and
    every desk, Texture and the system desks included, an installed package
    with privilege only from grants and tiers; a locked desk protocol; the
    Library distributing apps, desks, service providers, themes and command
    packs. Then self-development restarts on it, Files and context menus as
    its first rounds.
  artifact:
    - front-end kernel (lib/kernel) at most 800 lines and internal/kernel at most 500 lines, each enforced by a check in the repo gates
    - every app a package: the fourteen static rows gone from frontend/src/lib/apps/registry.ts (except those deleted); every shell surface (H1 to H4) a package; every desk, system desks included, a desk package
    - the desk protocol lock (internal/deskprotocol, lock/1.0.0.json) with a required CI job
    - the SDK surface lock (sdk-surface/*.json) covering the new surfaces (shell functions, commands, themes, uses, ctx), SDK 2.0.0 shipped, legacy_events and the v1 manifest reader retired
    - the Library handling every kind of package (kind, slots, tier, uses, displaces; selection records; tiered review; watchdog failover)
    - forced modularity M1 to M5 and F1 landed (install is the only name; stack refuses package paths; base packages load as modules; per-user per-app update policy; platform yank)
    - GET /api/plugins/state on d@e showing the base image's default packages as the only candidates on a fresh computer
    - self-development restarted: the arms running Files and context-menu rounds as in-place updates
  acceptance:
    - action: >-
        Per slice, the proof its design section names (contract test, Go test,
        Playwright spec or the product path on choir.news): K0 leak
        differential (install, activate, exercise, uninstall; snapshot()
        equals baseline); G0 supervisor intensity and boot-fold tests plus
        GET /api/plugins/state on d@e; P0 v1 reader, strict unknown-key and
        publisher-declared refusals; S3 openersForFile differential before
        and after; each C-slice package-modularity.spec.js installed mode plus
        its named specs; D0 contract tests against testdata/v1.0.0; L2
        selection survives a reboot of d@e; L3 an unsafe pack and a T2 release
        from a published origin are refused; X1 a package dispatching a legacy
        event fails install.
      proves: each slice does what its design says, on the real path, with its rollback shown
      evidence_class: CI plus deployed proof per slice
    - action: >-
        One deployed product path on choir.news, by a fresh test account with
        no SSH: (1) /api/plugins/state lists the default packages; (2) install
        a context-menu theme/renderer provider from the Library, right-click a
        file in Files, see the provider's menu; (3) make the provider throw,
        see it un-ranked with its reason and the base menu served; (4) yank
        the release (platform break-glass) and see it fall back to the base;
        (5) uninstall Calendar and reinstall it from the Library, its data
        intact and its desk retired and re-registered with the sole-writer
        rule held; the checks run on d@e first, then widen by ring.
      proves: the modular path works end to end, including failure, yank, data survival and the desk lifecycle, with no special-cased system app
      evidence_class: deployed acceptance on staging with before and after screenshots and the tape/plugin-state snapshot as artifact
    - action: gate G holds on CI (leak differential and contract runner green; M3's package-only operation run end to end on d@e), then one self-development round on Files and one on context menus run on the new architecture with before and after screenshots
      proves: self-development works on the modular base
      evidence_class: deployed acceptance and the round receipts
  rollback: per-slice revert (each slice is additive or revertable; v1 manifests and static imports stay until X1; M2+M3 deploy together and revert together); planned restart markers per CLAUDE.md for any deploy-time restart
  landing:
    required: true
    environment: staging (choir.news), every ring, d@e first
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the privileged, hard-coded surface of the base (kernel lines, apps
    and desks reached by identity or a closed list, state a package can hold
    that a grant does not name) while self-development and third-party code
    stay fully usable; every package change reaches the user through the same
    install, grant and rollback path.
  goodharting_would_be: >-
    Moving files into packages while privilege still keys on app or desk
    identity (a closed list, textureCellAuthorizer, a hard-coded desk name);
    a kernel that stays under budget because logic moved into an unlocked
    "service" file; a protocol lock that nothing enforces; counting registry
    rows deleted without the leak differential and the installed-mode specs
    green; first-party packages that cannot be uninstalled or replaced; a
    feature switch added to land a slice.

homotopy:
  realism_axis: >-
    the share of the real system running through the package path, from one
    package (Files) loaded as a module, through every app, shell surface and
    desk, to the Library handling third-party packages of every kind under
    grants, with a never-special-cased first party at every step

boundaries:
  mutation_class: varies per slice (green docs; yellow tooling and locks; orange most front-end, kernel and registry slices; red R1, M2+M3, F1, L3, D2, D3, D4, D6, D7, D8, D9)
  review: >-
    Per CLAUDE.md, the data-system cutover suspension does not apply. Tier 0:
    green, yellow tooling. Tier 1 (Devin): orange, and red that is additive
    with a one-act rollback. Tier 2 (Devin and DeepSeek v4.1 flash): red where
    a defect would be silent or irreversible (tape/evidence, Texture canonical
    writes, auth/session, store schema, boot path, new concurrency). Cheap
    gates first every time: check-v1-writer-purity, check-decode-roots,
    check-test-hang-guards, selftest-guest-token, check-switch-registry, the
    new kernel size checks, the SDK surface lock, the CI mirror. After three
    rounds on one mechanism, look at the substrate. Review runs in the
    background while the next build proceeds; expedite, tier by tier.
  authority_sources:
    - the owner rulings quoted in this file's header (2026-11-04)
    - docs/design/modular-architecture-2026-11-04.md (sysmod 73e64110c) as the build plan, once reviewed
    - docs/design/app-agent-desks-2026-11-04.md (appdesk) for D0 to D10
    - docs/design/forced-modularity-build-2026-11-04.md (main)
    - docs/choir-doctrine.md, CLAUDE.md, docs/standing-questions.md
  must_preserve:
    - the keep set (newspaper content, the Autopaper object graph, the owner's emails, Textures, precommitment records, the agent record)
    - Texture is the sole writer of canonical document revisions; an app desk is the sole agent writer of its app's object kinds, now enforced as a kernel service, not by identity
    - the desk protocol (casts, delegation, the commitment ledger) is locked and versioned; a package states the protocol version it speaks
    - restart rules: a crash restart never resumes work (open obligations close as "interrupted by a restart"); only a consumed planned-restart marker resumes; the kernels rebuild activation at boot and never persist a pending call or write a marker; standing intents survive a crash, occurrences do not
    - the paper refreshes only at a planned, marker-backed moment, never mid self-development apply or with a live capsule
    - no feature switches; every slice lands enabled on d@e and widens by ring; a correctness refusal is not a switch
    - app updates are per-user per-app, manual by default; forced update and yank are platform-only, manual break-glass; builders publish without approval
    - publisher identity is the maild numbered address; no fallback
    - no secrets or owner email addresses in the repo or output
    - never act on the paper or the owner's computers except through deploys; adversarial drills only on d@e; b@c and the owner's computer never deleted
    - Node B memory above vmctl's floor; at most two concurrent heavy builds; stagger compiles on the Mac and the agents VM
    - the product-path rule: proof is a contract test, Go test, Playwright spec or the staging path, never an ad-hoc screenshot (screenshots are required for self-development rounds, before and after for modifications)
  excluded:
    - self-development rounds before gate G; self-development deleting base code
    - Autopaper as an app agent (D10) and the Autopaper section; app agents beyond Texture and the first T0 desk (they follow this mission)
    - zero-downtime or compatibility machinery beyond what a slice's rollback needs
    - the tape app, update-now-or-later prompt and email-as-app-agent (deferred owner ideas)
  protected_surfaces: >-
    Texture canonical writes and textureowner, the tape and commitment ledger,
    auth/session and grants, vmctl and deployment routing, the stack and
    candidate overlay (M2, M3, D8), the package install path, boot and
    planned-restart handling, gateway credentials and cost attribution (D7).
    Red slices carry conjecture delta, admissible evidence, rollback and
    heresy delta (discovered, introduced, repaired) in their commit and ledger line.

now:
  status: working
  slice: architecture design review (sysmod 73e64110c in Devin review; appdesk design in progress); first builds are group A (K0, G0, P0, with M1 and R1 beside them) plus D0, all on disjoint packages
  source_ref: origin/main 93706d53c (this file lands on top of it)
  deploy_identity: staging 789a141d99dc107b805f153e9ff5e3df9ac83ade, epoch 41
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
      If every app and every desk is a package with grants, under a small
      kernel and a locked desk protocol, then self-development becomes
      reliable and secure (a change is an install, a grant and a revert, not
      a base edit) and app agents follow as desk packages.
    test: >-
      After gate G, the arms land simple apps and in-place updates (Files,
      context menus) with before and after screenshots, with no base-code
      edit, no rebase wedge, and no request that needs a privilege a grant
      does not name; a hostile fixture package (throws, over-budget, asks for
      T2, writes another app's kind) is contained on d@e.
    edge: >-
      missing_oracle: slices can pass their contract tests while some privilege
      still keys on identity; frame_lock: the design's own slice list may omit
      a hard-coded citer.
    delta_o: >-
      each slice starts with a grep of its deletion citers; a CI check that
      fails if a closed list of app or desk names reappears; the hostile
      fixture package as a standing test; the leak differential.
    scope_if_supported: >-
      the first-party apps and desks migrated here and the Library's package
      kinds; not yet third-party desks under real grants (D7 to D9) nor
      Autopaper as an app agent.
    status: proposed
    evidence_refs: [sysmod 73e64110c, appdesk app-agent-desks-2026-11-04.md]
  decision:
    what: >-
      adopt the modular-architecture design (decisions in its section 8 and the desks design section 11, each with its default) as the build plan once the Devin review accepts; run waves W0 to W8
    kind: architecture
    status: proposal
    evidence_ref: sysmod 73e64110c (in Devin review)
    owner_ratification_ref: the 2026-11-04 rulings in this file's header ratify direction ("more modular is better", every app and desk a package, kernel small, protocol locked); the design's specific decisions are defaults unless the owner overrides
  belief:
    believed_state: >-
      The design is complete enough to build group A: K0, G0, P0 share no
      files; D0 and D1 run beside them; M1 and R1 are independent. Main is
      green at the last observed state; staging runs 789a141d9. Self-
      development is paused. The appdesk design is not yet committed.
    main_uncertainty: >-
      whether Devin's review of the design finds a blocking gap (a hard-coded
      citer, a kernel budget that cannot hold) and whether the Sonnet-builder
      budget (about 15% of the week) covers roughly 45 slices; Desktop.svelte
      and AppHost.svelte contention among S1, W1, X1
    next_observation: Devin's verdict on 73e64110c; the appdesk designer's report; the first K0, G0 and P0 builder reports
  blocker_or_risk: >-
    token budget (prefer Sonnet; Opus only to orchestrate and adjudicate);
    serialized hot files (Desktop.svelte, AppHost.svelte, registry.ts,
    check-packages.mjs, app-sdk.ts); Node B memory with parallel builds
  next_action: >-
    Land the reviewed design docs on main (cherry-pick sysmod's three docs
    commits and commit appdesk's file), then start Sonnet builders in
    separate worktrees on K0, G0, P0, M1, R1, D0 and D1; first step of each
    slice is a grep of its deletion citers; begin Devin tier-1 review of each
    as it freezes, and fold the Devin verdict on the design into this file's
    review block.

receipts: []
---

# Modular architecture

The mission in one paragraph: the base becomes two small kernels and a locked
desk protocol, and everything else, every app, shell surface and desk, is an
installed package whose privilege is its grants and tier. The Library carries
all package kinds. We do the migration on main with Sonnet builders under the
review tiers; self-development restarts only on the finished architecture.

## Waves

- **W0 foundations:** K0 front-end kernel, G0 Go kernel (new extension points
  only), P0 manifest v2 and validators, M1 install is the only name, R1 clean
  rebase on every computer (already landed in part, 8ae42dd3b; the remainder is
  its tier-2 residuals), D0 desk protocol lock, D1 desks without closed lists.
- **W1 services plus D0 enforcement:** S1 SDK 1.1.0 and shell functions, S2
  commands, S3 file-route table, S4 context menu on the slot, S5 theme, S6
  settings, S7 deep links, W1 window API, U1 uses extraction and lock sections,
  D2 object verbs and the reducer-set writer.
- **W2 app batches:** C1 to C13 (Calendar, Pulse, Audio, Video, Slides,
  Autopaper, Compute Monitor, Settings, Mail app and desk and the rest per the
  migration table), each one slice, each deleting only its registry row.
- **W3 shell surfaces:** H1 to H4, serialized on Desktop.svelte.
- **W4 desks:** D3 Go kernel and reconcile, D4 contributes.desks and first-party
  registration, D5 first T0 app desk, D7 grants, quotas, spend, Library block,
  D8 candidate desk overlay.
- **W5 Texture:** T1 Texture app and desk last, with D6 (textureCellAuthorizer
  removed as a field; the committer registers with the kernel).
- **W6 forced modularity:** M2+M3 together, M4, M5, X1 (SDK 2.0.0), F1 (update
  policy, "Update available", platform yank; a widening update needs approval
  for manual installers and is not applied for automatic ones).
- **W7 Library kinds:** L1 then L2 and L3 in parallel, then L4.
- **W8 self-development restart:** gate G (design section 6), then Files and
  context-menu rounds with screenshots; the paused selfdev goal resumes.

Builders: Sonnet, one worktree per slice, split by package boundary; Opus
orchestrates, integrates and adjudicates. Never wait idle on CI or a deploy.
D9 (out-of-process tool servers) and D10 (Autopaper as an app agent) follow
this mission.
