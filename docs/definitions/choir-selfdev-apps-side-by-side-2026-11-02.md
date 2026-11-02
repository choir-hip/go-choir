---
definition_version: 5

# Return to self-development on the new data system: rewrite the desktop
# apps through b@c's loop, each rewrite a new package beside its original.
# Supersedes the T1a draft (choir-t1a-app-self-development-2026-10-26.md,
# kept as marked superseded because eight docs cite it), whose base, candidate and
# delegate window went stale over the data system cutover.
# Owner 2026-11-02: "let's tie up the odds and ends and get back to
# self-development. We should also make sure we have our Claude and Sonnet
# delegate working as well. And we have a bunch of different apps to
# rewrite. We should keep the originals as well while we rewrite them. We
# can delete them later."
readiness: executable  # on the owner's word above

review:
  reviewer: none  # executable on the owner's word; each promotion gets tier 1 (Devin)
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-11-02T17:30:00Z
  source:
    canonical_ref: main 84ad441df
    deploy_identity: staging 27210150c (guest), main 84ad441df (docs only after it)
  worktrees:
    - path: /Users/wiz/Dev/go-choir
      status: this file and docs/ACTIVE.md
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: leave in place
    - path: ../go-choir-delegate-renew (branch delegate-renew-2026-11, 002b25e25)
      status: pushed, unmerged
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: the pushed branch
  observed:
    - >-
      The store split (choir-store-header-body-split-2026-11-02.md) is
      checkpoint_incomplete on its meter day, judged 2026-11-03T05:00Z; no
      guest deploy before then. Held for that wave: the split follow-ups
      (a8c841b82), the delegate renewal (002b25e25), the wedge-in-health fix
      (branch health-projection-wedge, building), the nixpkgs bump PR 105
      (being pruned of packages the cutover retired).
    - >-
      The Claude delegate code is untouched by the cutover. It always runs
      claude-sonnet-5-5 (internal/claudedelegate/spec.go:33). Its host grant
      covers computer-ad1b46d4… and b@c (computer-6c50d6e5…) and expires
      2026-11-03T00:00Z; the renewal moves it to 2026-11-17. The token file
      dates from 2026-10-15 and is unproven since; the last "claude delegate
      available" console line is 2026-10-28, before the cutover, and no
      claude-mode capsule run is on record since.
    - >-
      Apps (frontend/src): the Reader package passed the design bar; the
      window manager was promoted (d550f8edd); the four themes were
      rejected on d@e; Slides (lib/SlidesApp.svelte) fails the bar; Files,
      Calendar and Podcasts (lib/FileBrowser.svelte, CalendarApp.svelte,
      PodcastApp.svelte) are not rebuilt. Their API routes still exist after
      the cutover (route grep only). App desks are off (CHOIR_APP_DESKS).
    - >-
      A package id is its directory under frontend/src/packages; a name or
      alias equal to a static id fails check-packages.mjs, and a file
      extension has one opener. So a rewrite can live beside its original
      only as a new id with no `opens` until its switch.

finish:
  deliver: >-
    The owner opens rewritten Slides, Files (with Open with), Calendar and
    Podcasts, and the four desktop themes, each built by self-development on
    b@c with the Claude (Sonnet) delegate and held to the design bar; the
    original of each app still opens and works beside it until the owner
    deletes it.
  artifact: >-
    Per app, a package frontend/src/packages/<app>-next built by
    engineering on b@c, approved by FP0 against the design bar's checks
    from real-shell renders, promoted with scripts/selfdev-promote and a
    tier 1 review, deployed and upstreamed; the original's static entry,
    component, routes and data untouched. The delegate proven by a
    claude-mode capsule run on b@c after the cutover.
  acceptance:
    - action: >-
        After the post-meter-day deploy and a cold boot of b@c: its
        observability reports claude_delegate.available until 2026-11-17,
        and one engineering assignment runs with network_mode=claude and
        returns a patch; the console shows the delegate's lines.
      proves: the delegate works on the new data system
      evidence_class: deployed product path (b@c)
    - action: >-
        For each app, FP0 renders the -next candidate in the real shell at
        three presets and two widths and records pass or fail per check in
        docs/evidence/selfdev-apps-design-judgments-2026-11.md; one fail
        rejects and the round iterates.
      proves: the design bar is held, not asserted
      evidence_class: rendered screenshots plus code read
    - action: >-
        After each promotion deploys: both the original and the -next app
        open on staging from the launcher, the original's file routes still
        go to the original, and b@c reports the operation upstreamed.
      proves: side by side, and the loop lands for every computer
      evidence_class: deployed identity and product path
  rollback: >-
    Per app: revert its promotion commit and deploy (the original is
    untouched, so nothing is lost). The delegate: revert 002b25e25 or empty
    the allowlist and deploy; revoke the token.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the apps on the list without an approved -next rewrite
    (baseline: Slides, Files, Calendar, Podcasts and the themes, five) and
    the design-bar checks each fails, while keeping every original working
    unchanged and the loop's authority intact.
  goodharting_would_be: >-
    Editing an original in place and calling it a rewrite; approving on the
    engineer's report or a signed-out capture; a -next app that only
    restyles one preset; FP0 hand-editing a candidate; counting a capsule
    that never called the delegate as a delegate round.

homotopy:
  realism_axis: >-
    the delegate proven on one b@c assignment -> one -next app (Slides)
    approved and promoted beside its original -> each remaining app ->
    the themes -> the owner switches routes app by app and deletes
    originals on the owner's word.

boundaries:
  mutation_class: orange (self-developed frontend packages); red for the delegate grant (vmctl) and any deploy wave
  review_tier: tier 1 (Devin) per promotion and for the delegate renewal
  authority_sources:
    - owner 2026-11-02 (header quote)
    - owner 2026-10-25 "you approve the reader" (FP0 approves on b@c with standing authority)
    - owner 2026-10-26 design statements carried from T1a: content first, judicious screen use, DeepSeek with the right constraints
  must_preserve:
    - upgrades never leave a computer stuck (owner 2026-11-02: "upgrading things should never leave a computer in a stuck state"): every open self-development operation has a terminal path an owner or the platform can reach, a hold is bounded and visible to the desks, and an apply refusal settles the operation instead of wedging the next one
    - every original app's static entry, component, id, routes, recents and data, until the owner deletes it
    - a -next package declares no `opens` and no `desk` until its switch; data stays on its existing kinds and stores
    - only engineering changes b@c's platform tree; FP0 never hand-edits a patch
    - one app per request; a trace review after every round
    - no guest deploy before the store split's meter day is judged (2026-11-03T05:00Z)
    - adversarial or risky runs on d@e or b@c only, never the paper or the owner's computer
  excluded:
    - deleting any original (owner act, later)
    - switching file routes to a -next app (owner act, per app)
    - Mail and the apps T1a excluded; app desks (CHOIR_APP_DESKS stays as is)
  protected_surfaces: [vmctl delegate grant, self-development approval and promotion, frontend/src/app.css shared overrides, Desktop.svelte media routing]

now:
  status: working
  slice: >-
    A0 odds and ends: wait for the 05:00Z S6 judgment, then one deploy
    wave with the split follow-ups, the wedge-in-health fix, the delegate
    renewal and (if green) the pruned nixpkgs bump; cold-boot b@c; prove
    the delegate (acceptance 1). Then A1: Slides-next on b@c.
  source_ref: 84ad441df
  deploy_identity: staging 27210150c
  candidate: {id: none, state: none, ref: none, base: none, digest: none, scope: []}
  conjecture:
    id: SD-1
    claim: >-
      With the design bar in the request and the delegate writing the code,
      a -next app passes the bar in one or two rounds; the earlier poor
      designs came from briefs without a bar, not from the model.
    test: rounds per app to an approved candidate, read from the traces, delegate rounds against DeepSeek rounds
    edge: missing_oracle
    delta_o: FP0 renders each candidate in the real shell instead of trusting the capsule's signed-out capture
    scope_if_supported: frontend packages on b@c with the delegate available
    status: active
    evidence_refs: [docs/reports/overnight-self-development-run-2026-10-26.md]
  decision:
    what: a rewrite is a new package <app>-next beside the untouched original; originals are deleted later on the owner's word
    kind: architecture
    status: ratified
    evidence_ref: this file
    owner_ratification_ref: owner 2026-11-02 "We should keep the originals as well while we rewrite them. We can delete them later."
  belief:
    believed_state: >-
      The loop and the delegate code survived the cutover untouched; what
      is unproven is the token, the grant at b@c's next cold boot, and
      engineering's path on SQLite-backed assignment state.
    main_uncertainty: whether the October 15 token still authenticates
    next_observation: b@c's first claude-mode capsule after the deploy wave
  blocker: none (time gate 2026-11-03T05:00Z for any deploy)
  next_action: >-
    At the 05:00Z check-in, after the store split settles: merge the held
    branches in one wave, watch CI and the deploy, cold-boot b@c, run the
    delegate proof.
  residuals:
    - model choice per Cast (the delegate is always Sonnet 5.5); add only if the owner wants Opus for some work
    - check-packages.mjs still says kinds must be empty "in P1"
    - the ChatGPT login on the judging host was noted good until about 2026-11-02T02:00Z; re-check before FP0 renders
    - the Reader's residual nits ride a later round confined to packages/reader

receipts: []
