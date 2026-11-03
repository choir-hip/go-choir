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
  slice: station 1 (pd) landed and proved; store split settled complete; station 2 (selfdev) rounds running on four test arms with blocker fixes expedited
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
    staging a595321f2 (luna delegate grant; lean rebase notice; problems
    17 and 21 through 24). 7800706b5 (problem 25: the delegate relay
    compared a VM id with a computer-id grant list, so no computer ever
    delegated; tier 2 accepted) in CI; delegate computers need a cold boot
    after it deploys. The claude arm's rounds so far ran with no delegate
    (DeepSeek alone); its next round after the cold boot is the first real
    Claude-arm round. Owner 16:00Z: luna delegates to Claude Sonnet under
    GPT token pressure; luna held until the delegate is available. Stacks
    now re-graft mechanically at refresh (stack_rematerialized), and pool's
    Slides round was admitted on the re-grafted stack. Engineering
    assignments end at a hard 60 minutes. deepseek Slides r1 rejected for
    missing screenshots; r2 (theme set by boot cache and one route mock)
    running. Node B 15 GiB available, PSI 0.
  main_uncertainty: >-
    whether the delegate works end to end after the cold boot (relay call,
    capsule egress); whether Slides rounds fit inside 60 minutes
  next_observation: claude_delegate.available=true on luna after its cold boot; first delegate run ids in a luna round
  blocker: none
  next_action: on 7800706b5 deploy, cold-boot luna and then claude (when idle), verify the grant, send luna round 10; carry Slides rounds on all arms and publish Files from each
receipts: []
