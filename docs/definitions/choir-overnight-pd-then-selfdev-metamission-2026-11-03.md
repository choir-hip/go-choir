---
definition_version: 5

# Overnight metamission: progressive delivery, then self-development, run
# with every concurrency the dependencies allow.
#
# Owner, 2026-11-03 ~02:15 UTC: "i actually want a metamission that runs
# progressive delivery then the self-dev mission overnight. and with any
# concurrency possible. i will go to bed soon."
# Then: "calibration in particular is the bridge between the 2 missions."
# The bridge: every change that rolls out (a platform release or a
# self-development candidate) carries the author's precommitments about its
# effect (health stays green, the named metric moves by about this much,
# the owner approves, rounds to approval); the ring walk and the approval
# path settle them; the settled record is the engineering desk's
# calibration digest, and calibration in turn can weight how fast a change
# walks. Progressive delivery is the settlement mechanism; self-development
# is where the predictions come from.
# And: "we can and likely will make changes in the self-dev mission which
# then use the prog delivery rings." Platform code changes made during the
# self-development mission (the Sonnet monitors' fixes to Choir) land on
# main and reach the fleet through the rings. Self-development candidates
# do not go through git: an arm applies its own candidate, publishes it to
# the Library, and the platform owner decides what goes on the base image.
# Owner, 2026-11-03: "self-dev isnt supposed to run through git like this. the
# library is meant as a library of stuff made from individual computers. and
# then i, the platform owner, can decide what is on the base image."
# Overnight (owner): FP0 approves candidates on the four test arms; no
# spend caps and no pausing an arm for spend ("dont pause any arm. no spend caps"); per-arm spend reported in the morning letter;
# the original Slides XSS waits for Slides-next.
# Operating tempo (CLAUDE.md): no blocking asks; escalate on dead ends with
# a shift of approach first; never wait idle on CI or a deploy.
readiness: executable  # on the owner's word above

review:
  reviewer: none  # executable on the owner's word; each landed change takes its own tier
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-11-03T02:15:00Z
  source:
    canonical_ref: origin/main 1e836b78d
    deploy_identity: staging; 3e99c7307 (Claude arm delegate grant) in CI at 01:3x
  parked:
    - branch selfdev-screens-as-files at 4d305c7d9 (both round-1 blockers fixed; needs tier 2 re-review)
    - branch cli-agent-onboarding at 2920ada52 (tier 2 accepted, Devin re-review 2026-11-03)
    - branch cli-texture-export landed (64b4fd9fb)
    - worktree /Users/wiz/Dev/go-choir-switches-settings: discardable half-done WIP
  scheduled:
    - store split S6 judgment and the paper's .presplit finalize at 05:07 UTC (owner asked for the delete)

finish:
  deliver: >-
    By morning: progressive delivery's ring-ordered deploys and automatic
    rollback are live on staging, the 2026-11-04 lint rows are resolved by
    the owner's calls, the backlog is moving in batches, and the
    self-development arms are running app rounds against the feature spec,
    with every station's state in its own goal file.
  artifact:
    - station 1 at least through its P1 and P2 (ring refresh, rollback), with deployed proof
    - station 2 started: S1 builds landed, arms running rounds
    - a morning letter (reporter skill) naming what landed, what is running, and what waits on the owner
  acceptance: each station's own acceptance; this file settles when both stations settle or are superseded
  rollback: each station's own; the metamission adds none
  landing: required per station

value:
  better_means: >-
    Maximize landed, deployed, proved work by morning across both stations
    while no computer is stuck, the paper keeps publishing, and no review
    tier is skipped.
  goodharting_would_be: >-
    Many open branches and few deployed proofs; skipping tier 2 to go
    faster; counting a pushed commit as done.

homotopy:
  realism_axis: per station

boundaries:
  mutation_class: per station (red work in both)
  review: per station's tiers; tier 2 runs in the background while the next build proceeds
  must_preserve:
    - the paper refreshes only at a planned, marker-backed moment; never mid self-development apply or with a live capsule
    - crash restarts close work; planned restarts resume
    - adversarial drills only on d@e; b@c never deleted; the owner's computer never deleted
    - no secrets, keys or owner email addresses in the repo or output
    - Node B memory above vmctl's floor: at most two concurrent arm builds until S4 lands
  excluded: the Autopaper section (event wakes investigation) after both stations

metamission:
  stations:
    - id: pd
      path: docs/definitions/choir-progressive-delivery-2026-11-03.md
      readiness: executable
      depends_on: []
    - id: selfdev
      path: docs/definitions/choir-selfdev-apps-feature-complete-2026-11-03.md
      readiness: executable (resumed from hold by this metamission)
      depends_on: [pd P1 and P2 for app-round deploys; the rest runs concurrently]
  lanes:
    A_pd_core: "pd P1 ring-ordered refresh, then P2 automatic rollback (one Opus builder; tier 2 each)"
    B_lint_rows: "before 2026-11-04: delete the gateway scheduler; story expansion for every signed-in user; host fetch and capsule mirrors operational, proved at d@e first (Sonnet builder)"
    C_selfdev_landings: "screens-as-files tier 2 re-review then land; onboarding land; then app rounds on the arms (staggered, at most two builds at once)"
    D_backlog: "pd P3 low-risk deletion batches that need no fix (levers, dead keys, DAMON, leftovers, event table, group commit) once P1 can ride them; until then held"
    F_calibration_bridge: "the precommitment shape a release and a candidate carry (effects on ring health and named metrics, approval, rounds), settled by the ring walk (pd P1/P2 history) and the approval path; engineering's predictions mandatory per round; the engineering digest and within-desk A/B as the test bed for delivery (injected or REPL), content, configuration and cost, before the seed widens to the other desks (docs/design/calibration-seed-2026-11-03.md, owner rulings); designed tonight so pd's ring history records what settlement needs"
    E_research: "selfdev S4 design review and build; calibration engineering claims; evidence.graph measurement on a paper store copy (read-only)"
  order: >-
    A, B and F start now (F as design feeding A's history format). C starts now (reviews and landings), and its app
    rounds start once screens-as-files deploys. D waits for A's P1. E runs
    whenever a lane is idle. Compiles are staggered on the Mac and the
    agents VM.

now:
  status: working
  slice: station 2 (selfdev) paused by owner order 20:05Z; first the architecture for maximal modularity (plugin research, system services, forced modularity), built with Sonnet builders, pushed and expedited; then self-development rounds resume (Files and context menus first)
  conjecture: >-
    C1: with five lanes split by package boundary and reviews in the
    background, one night lands pd P1 and P2 and starts self-development
    rounds without a stuck computer. Test: morning state per station. Edge:
    resource (review rounds and CI serialize the red work). dO: start every
    review the moment a build freezes.
  decision: >-
    blocking fixes ship expedited (owner policy); docs pushes batched
    between deploys; problem 21 rebuilt to need no work item (the notice is
    the wake) after tier 2 found the work item reusable and orphanable
  believed_state: >-
    staging 8e695263c (deployed 16:44Z, security epoch 39, main green):
    open install, any signed-in user installs any listed Library release,
    by owner ruling, no switch; the crouton-synthetic-luna pool (005d77300)
    is the claude arm's policy for every role, read back matching. Apps as
    a layer is on main (43745079a, 6b61e3660, 3c48a7ac2, 9f3970680,
    9858baf1e). The owner reviewed the Library on their phone at about
    16:30Z: it lists computer ids, track hashes and duplicate packages,
    hides screenshots behind a second drill down and buries them under a
    metadata panel. A Sonnet builder is rebuilding it as an app store
    (screenshots required to list, publisher shown by account email, one
    card per app, one app page with screenshots, install and
    precommitments, no hex). remat-serve (the pool serving its rebased
    build) got SEND BACK from both tier 2 seats (a stale test assertion,
    an open operation spending a notice attempt, stale contract text); a
    Sonnet fixer is on it, then one Devin seat. Node B 18 GB available,
    171 GB disk free, load 2.3.
  main_uncertainty: >-
    whether the app store rebuild lands clean through review and reaches
    the owner's computer the same day; whether remat-serve's re-review
    clears in one round
  next_observation: the Library builder's report; remat-serve fixes and Devin re-review
  blocker: none
  next_action: >-
    owner order (2026-11-04 15:50Z): 1 tie off loose ends, 2 make
    self-development reliable, 3 app agents, 4 Autopaper as an app agent,
    5 Autopaper features (op-eds, text-driven personalization). Loose ends
    now: the owner's account receives platform updates and can install
    test-account apps from the Library (cross-owner install is refused
    until S13; find the smallest safe path); self-development single
    threaded on one test computer with crouton by default, falling back
    to Synthetic DeepSeek then ChatGPT, with the Claude delegate; quota
    cancellations recorded; laptop and host storage; worktrees; restore
    or retire arm policies. Reliability bar (owner): simple apps such as
    a timer land first time in under 20 minutes, longer for complex ones;
    modifying an app in place by conversation works; app updates through
    the Library and rebase work; a forced app update (security) exists;
    the app layer has its own security model, separate from the base
    computer. Token budget: about 15% of the week left, so Sonnet over
    Opus for about 30 hours
receipts: []
