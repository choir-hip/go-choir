---
definition_version: 5

# Self-development at scale: four model arms on fresh stock computers, each
# reinventing the desktop apps to feature complete, supervised by Sonnet
# monitors that fix Choir as they go, measured for performance throughout.
# Supersedes choir-selfdev-apps-side-by-side-2026-11-02.md (its slides-next
# and files-next rounds on b@c are evidence; its b@c/d@e arms are retired).
#
# Owner, 2026-11-03 (the charter, verbatim where quoted):
# - "make a new through line plan, which includes all the work described
#   above, from the auth to the choir CLI and MCP to self-development for
#   both the Claude delegate and synthetic DeepSeek, and where we have the
#   Sonnet sub-agents watching them and making changes to choir code as
#   needed. And we should just run this and hill climb on this continuously
#   for like the next 12 hours at least, maybe 24, 36 hours".
# - "reinvent the apps mentioned, but like we really need to make them
#   feature complete ... all of the obvious features need to be there, and
#   the UI design all needs to make sense. And in fact, the files app
#   itself needs to be reinvented."
# - "if we can do 3 VMs at once, let's do that. If we can do 4, even better
#   ... the third one should use ChatGPT Luna ... if we can do 4, we can
#   have that running a pool of models ... the performance statistics are a
#   major thing to consider. And if we find performance optimizations along
#   the way, we should do those too."
# - "have many capsules running as long as whatever heavy work, like
#   front-end build, is scheduled correctly."
# - "a feature in the CLI and MCP, which lets you easily download texture
#   documents, including especially attachments like images ... a coding
#   agent that's driving choir, and it can be supervised both in the coding
#   agent and in the choir app itself."
# - "We should change the default to actually give people what they need.
#   Really it should be easy to give the choir cli or mcp to an agent and
#   its auto setup."
# - "No platform capture ... That's why the engineering desk takes the
#   screenshots" of real content; "get rid of all that junk".
# - "we should be working from the stock guest image just with the Claude
#   delegate for one of them"; "every computer should have visual capture.
#   That shouldn't be anything special."
# - "how we are integrating PSA and pre-commitment records ... maybe not yet
#   deep enough. So that's something to investigate and understand."
# - (2026-11-02, carried) "upgrading things should never leave a computer in
#   a stuck state".
readiness: executable  # on the owner's word above; ON HOLD 2026-11-03 for choir-progressive-delivery-2026-11-03.md (owner: "put this self-dev mission on hold to handle progressive delivery")

review:
  reviewer: none  # executable on the owner's word; each landed change takes its own tier
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-11-03T01:10:00Z
  source:
    canonical_ref: origin/main 93424c258
    deploy_identity: staging ee34d53c1 (deployed 2026-11-03T00:35:19Z); af97a890d and 93424c258 in CI
  worktrees:
    - path: /Users/wiz/Dev/go-choir
      status: dirty
      class: other_agent_wip
      owner: monitors (evidence appends) on branch cutover/data-system
      touch: read_only
      recovery: evidence files are copied to main by docs commits; never commit here
    - path: ../go-choir-screens (selfdev-screens-as-files)
      status: dirty
      class: goal_candidate
      owner: builder (platform capture deletion, engineer screenshots as files)
      touch: goal_owned
      recovery: branch
    - path: ../go-choir-cli-onboarding (cli-agent-onboarding)
      status: dirty
      class: goal_candidate
      owner: builder (default full-access keys, choir login, mcp install)
      touch: goal_owned
      recovery: branch
    - path: ../go-choir-tcc (test-computer-create, d333c010a)
      status: clean
      class: goal_candidate
      owner: builder (scripts/test-computer-create; four arms)
      touch: goal_owned
      recovery: branch
    - path: cli-texture-export (worktree by its builder)
      status: dirty
      class: goal_candidate
      owner: builder (choir texture export, MCP image blocks)
      touch: goal_owned
      recovery: branch
    - path: ../go-choir-lane-slides-next (lane/slides-next a18f22b3b)
      status: clean
      class: goal_candidate
      owner: FP0
      touch: goal_owned
      recovery: branch; Devin send_back (HTML deck XSS, CSS scope, PPTX images, pdf.js teardown, f hotkey)
  observed:
    - Test computers selfdev-arm-claude (computer-a875023a6379c41f6ddb438f02e76a02) and selfdev-arm-deepseek (computer-e80d229b0d78c35d47656b8c66426a43) exist on the stock image with all-scope keys in ~/.config/choir-e2e/ (0600); selfdev-arm-luna (computer-2bf3cab1e3ebf3eb24a515c4603cf954) and selfdev-arm-pool (computer-4d38420cf85ac490a36baa0ef5d7a67a) likewise. Node B at 00:46Z: 9 computers, 17.0 GB available of 32, idle arm about 0.77 GB; four concurrent compiles could reach the 4.7 GB vmctl floor.
    - Platform package captures (db2ee2dd6) and the Texture embed ordering (bf89cc66c) are reverted (ee34d53c1); the sample front stories are reverted (af97a890d). The older platform capture path (choir-capture, visual evidence, the judge's visual_captured precommit) still exists; its deletion is in build.
    - No per-app feature spec exists in the repo; the v7 briefs live outside git. Digest of every documented plan: scratchpad app-plans-digest.md (to be committed as docs/design/apps-feature-complete-spec-2026-11-03.md in S0).
    - The four desktop themes are on main (670f9ea50, Aero Chrome Heart default); the side-by-side goal's "themes unbuilt" was stale.
    - Open loop defects (problem docs on main): nothing re-Casts a request after a blocker clears; management bound a Cast to the wrong parent record (93424c258); HTML deck markup runs in the shell origin in the original Slides (live exposure).

finish:
  deliver: >-
    The owner opens Files, Slides, Calendar and Podcasts, each reinvented as
    a feature-complete -next app that any user would find obviously whole
    (upload, Open with, preview, rename and move in Files; authoring and
    presenting in Slides; creating and editing events in Calendar; a real
    queue and played state in Podcasts), designed to the bar and shown on
    real content, built by self-development on four arms (Claude delegate,
    Synthetic DeepSeek, gpt-6-luna, the default pool) running at once on
    fresh stock computers; a coding agent drives and supervises it through
    `choir login`, the CLI and MCP, seeing the same screenshots the owner
    sees in Texture.
  artifact: >-
    (1) Auth: full-access keys by default, `choir login` device flow, `choir
    mcp install`. (2) `choir texture export` and an MCP tool that returns a
    document's images. (3) The platform capture path deleted; engineering's
    own screenshots of real content saved as files on the computer, bound to
    the frozen candidate, embedded by Texture, read by the judge. (4)
    scripts/test-computer-create and four arm computers. (5) Heavy capsule
    work (frontend build, Playwright) scheduled across the host so many
    capsules run at once. (6) Per app, frontend/src/packages/<app>-next
    feature complete against docs/design/apps-feature-complete-spec-2026-11-03.md,
    applied on its arm and published to the Library as a package release
    from that computer; what reaches the base image is the platform owner's
    decision (owner 2026-11-03), not a git promotion. (7) A performance record
    per round and per host sample, and the optimizations it found, landed.
    (8) A finding on PSA and precommitment records in self-development, and
    the deeper integration it names, landed where it is safe.
  acceptance:
    - action: >-
        A fresh agent with only the CLI runs `choir login`, the owner approves,
        then `choir self-dev list` and `choir texture export` on an arm's app
        document return the screenshots as PNG files.
      proves: the agent operating model works end to end
      evidence_class: deployed product path (staging)
    - action: >-
        Per app, FP0 judges the -next candidate from the engineer's real-content
        screenshots plus its own export, against every feature in the spec and
        every design-bar check, in the seven presets at 1100x750 and 390px;
        pass or fail per item in docs/evidence/selfdev-apps-feature-judgments-2026-11.md.
      proves: feature complete and designed, not asserted
      evidence_class: screenshots of real content plus code read
    - action: >-
        After each promotion deploys, both the original and the -next app
        open on staging, and the feature checklist passes on a real account.
      proves: the apps land for everyone, beside their originals
      evidence_class: deployed identity and product path
    - action: >-
        Three, then four arms run rounds at the same time; the performance
        record shows round latency and host headroom at each concurrency.
      proves: how much self-development the host can run at once
      evidence_class: host meters and per-round timings
  rollback: >-
    Per change: revert its commit and deploy. Per app: revert its promotion;
    the original is untouched. Arm computers are disposable (delete the test
    accounts). Auth defaults: revert; existing keys keep their scopes.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the spec features and design-bar checks each -next app fails on
    real content (baseline: every app below the spec; Files has upload but no
    Open with, preview, rename, search), and the wall time and owner effort
    per accepted round, while every original keeps working and no computer
    is ever left stuck.
  goodharting_would_be: >-
    Screenshots of fixtures or empty states; a feature present in code but
    unreachable in the UI; approving on the engineer's report; FP0 or a
    monitor hand-editing a candidate; counting a delegate-free round as a
    delegate round; a faster round bought by skipping verification.

homotopy:
  realism_axis: >-
    one arm, one app, one feature on real content -> four arms in parallel
    on Files and Slides -> every app on its spec -> promotion beside the
    originals -> the owner switches routes and deletes originals on their word.

boundaries:
  mutation_class: orange (app packages, CLI, scheduling); red (auth defaults and device login, capsule evidence and judge, vmctl scheduling, any deploy wave)
  review_tier: tier 1 (Devin) for orange and per promotion; tier 2 (Devin + DeepSeek) for auth, judge/evidence deletion, scheduling
  authority_sources:
    - owner statements quoted in the header (2026-11-03)
    - CLAUDE.md operating contract; docs/choir-doctrine.md (I26, I27)
    - docs/design/app-design-bar-2026-10-26.md
  must_preserve:
    - upgrades never leave a computer stuck (owner 2026-11-02)
    - originals untouched until the owner switches a route or deletes one; -next packages declare no opens until their switch
    - engineering is the only writer of an arm's source; FP0 and monitors never hand-edit a candidate
    - screenshots are of real content, taken by the engineering desk; no platform capture
    - one home per kind (I27); authored decks and events get a declared kind, not a file second home
    - adversarial drills only on d@e; never on the paper or the owner's computer
    - no secrets or owner email addresses in the repo or output
  excluded:
    - deleting originals or switching routes (owner's word, per app)
    - Mail (its own desk mission) and app desks (CHOIR_APP_DESKS stays off unless a slice needs it and says so)
    - the four themes as a rewrite (on main since 670f9ea50); only their perf and per-app checks
  protected_surfaces: [auth key minting and sessions, capsule evidence and the judge, vmctl capsule scheduling, Texture canonical writes]

operating_model:
  orchestrator: FP0 (Opus) integrates, adjudicates, lands, judges candidates, writes the spec, sends revisions.
  arms:
    - {id: claude, computer: computer-a875023a6379c41f6ddb438f02e76a02, engineering: Synthetic DeepSeek V4.1 Flash + Claude Sonnet delegate (grant)}
    - {id: deepseek, computer: computer-e80d229b0d78c35d47656b8c66426a43, engineering: Synthetic DeepSeek V4.1 Flash}
    - {id: luna, computer: computer-2bf3cab1e3ebf3eb24a515c4603cf954, engineering: openai gpt-6-luna}
    - {id: pool, computer: computer-4d38420cf85ac490a36baa0ef5d7a67a, engineering: pool:default}
  monitors: >-
    One Sonnet subagent per arm reads traces (choir trace tail/export,
    lifecycle capsule-evidence, texture export), reports at awaiting
    approval / failure / stall > 20 min, and may fix Choir platform code in
    its own worktree off origin/main (problem doc first, tests first, branch
    pushed); FP0 reviews and lands. Monitors never edit an arm's source.
  cadence: an hourly FP0 check-in drives the hill climb for at least 12 hours (owner may extend to 24-36).
  performance: >-
    Host samples (memory, CPU, IO, swap, guest memory) every minute from the
    growth meter; per round: Cast, delegate, build, screenshots, freeze,
    judge, apply durations; concurrency raised 3 -> 4 -> more while round
    latency and headroom hold.

slices:
  S0: commit the feature spec (docs/design/apps-feature-complete-spec-2026-11-03.md) from the digest plus obvious-feature gaps; this goal
  S1: land the in-flight builds - test-computer-create; platform capture deletion + screenshots as files; texture export (CLI/MCP); default full-access keys + choir login + mcp install
  S2: arms up - luna and pool computers; delegate grant on the claude arm; identical model policies except engineering model; monitors started
  S3: loop defects - re-Cast when a blocker clears; Cast parent binding; an open engineer question holds revocation; Slides HTML deck XSS in the original (owner decision on timing)
  S4: heavy-work scheduling across capsules (build/Playwright slots, warm caches), then raise concurrency
  S5: apps - Files (reinvented) and Slides first on all arms, then Calendar, Podcasts, the Library app (owner 2026-11-03: "library should actually be an app, not just cli/mcp") and the desktop background picker; kinds for authored decks and events (lift check-packages "kinds empty in P1" for the slices that need it)
  S6: PSA and precommitment records in self-development - investigate, then deepen (finding in scratchpad psa-precommit-selfdev.md first); the calibration seed (owner 2026-11-03) per docs/design/calibration-seed-2026-11-03.md: prior-run seed removed first, baseline recomputed, then one experiment per desk at a time (management push arm first; engineering claims defined and collecting from the arms now, tested later)
  S6b: superseded 2026-11-03 by choir-progressive-delivery-2026-11-03.md (no switches; ring-ordered deploys)
  S7: publish, not promote. An approved candidate is applied on its arm and published to the Library as a package release from that computer; the platform owner decides what goes on the base image (owner 2026-11-03: "self-dev isnt supposed to run through git like this. the library is meant as a library of stuff made from individual computers. and then i, the platform owner, can decide what is on the base image."). No scripts/selfdev-promote to main. The comparison report across the four arms.

now:
  status: working  # resumed 2026-11-03 as station 2 of the overnight metamission; parked branches selfdev-screens-as-files, cli-agent-onboarding unmerged
  slice: S0 + S1 (spec committed; four builds in flight; arms being created)
  source_ref: origin/main 93424c258
  deploy_identity: staging ee34d53c1
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: C1-arms-parallel
    claim: >-
      With the platform plumbing fixed (screens as files, re-Cast, heavy-work
      slots), four arms can run app rounds in parallel and at least one arm
      reaches a spec-passing Files and Slides within 12 hours.
    test: per-arm round timings and feature-judgment pass rates over the first 12 hours
    edge: resource
    delta_o: host meters plus per-stage round timings from the monitors
    scope_if_supported: Node B staging, four test computers, Files and Slides
    status: active
    evidence_refs: []
  decision:
    what: retire b@c and d@e from the comparison; four fresh stock arms; platform capture deleted
    kind: operational
    status: settled
    evidence_ref: owner 2026-11-03 statements in the header
    owner_ratification_ref: not_applicable
  belief:
    believed_state: >-
      The loop works end to end on one arm (b@c applied slides-next with the
      delegate), but evidence was fake (fixtures), rounds lose time to
      harness building and re-Cast gaps, and no app is near feature complete.
    main_uncertainty: >-
      Host capacity for four concurrent capsule builds, and whether app
      authoring features need kinds the package checker still forbids.
    next_observation: the first parallel round's timings and the host meters
  blocker_or_risk: >-
    The judge still requires platform capture evidence until S1's deletion
    lands; arms started before then may be rejected for the error-state
    capture. Start app rounds after S1 deploys.
  next_action: land S1's builds as each passes review; start the luna and pool arms; then the first Files round on all arms

receipts: []
---

# Self-development at scale: four arms, feature-complete apps

The header holds the owner's charter. This file is the authority for
the run; the ledger beside it records every move.
