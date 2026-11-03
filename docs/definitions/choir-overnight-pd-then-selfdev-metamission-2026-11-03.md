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
# then use the prog delivery rings." Platform changes that come out of
# self-development (Sonnet monitors' fixes, promoted candidates) land on
# main and reach the fleet through the rings like any release; their
# precommitments are settled by the same ring walk.
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
    F_calibration_bridge: "the precommitment shape a release and a candidate carry (effects on ring health and named metrics, approval, rounds), settled by the ring walk (pd P1/P2 history) and the approval path; engineering's predictions mandatory per round; the engineering digest and within-desk A/B (docs/design/calibration-seed-2026-11-03.md, owner rulings); designed tonight so pd's ring history records what settlement needs"
    E_research: "selfdev S4 design review and build; calibration engineering claims; evidence.graph measurement on a paper store copy (read-only)"
  order: >-
    A, B and F start now (F as design feeding A's history format). C starts now (reviews and landings), and its app
    rounds start once screens-as-files deploys. D waits for A's P1. E runs
    whenever a lane is idle. Compiles are staggered on the Mac and the
    agents VM.

now:
  status: working
  slice: lanes A, B, C
  conjecture: >-
    C1: with five lanes split by package boundary and reviews in the
    background, one night lands pd P1 and P2 and starts self-development
    rounds without a stuck computer. Test: morning state per station. Edge:
    resource (review rounds and CI serialize the red work). dO: start every
    review the moment a build freezes.
  decision: self-development resumes from hold inside this metamission
  believed_state: docs coherent; nothing of pd built; selfdev branches parked and nearly ready
  main_uncertainty: whether ring order fits vmctl's deferred-refresh sweep without a new state store
  next_observation: the lane A builder's reading of the refresh sweep
  blocker: none
  next_action: launch lanes A and B builders; start the screens re-review; land onboarding

receipts: []
