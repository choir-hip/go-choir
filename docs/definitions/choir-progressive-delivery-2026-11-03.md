---
definition_version: 5

# Progressive delivery, built: every switch walks the rings on measured
# evidence and retires, so no feature stays off and no switch outlives its
# rollout. Puts choir-selfdev-apps-feature-complete-2026-11-03.md on hold.
#
# Owner, 2026-11-03: "i hate the pattern of all these switches. ahh, i
# remember, we were supposed to build progressive delivery. hmm, we should
# put this self-dev mission on hold to handle progressive delivery." Earlier
# the same night: "i dont want anything to stay off. i want to switch
# features on or delete them"; "audit the switches before going ham ... i
# dont even know what we have. and features may conflict"; and
# "texture hold trajectory is good. i thought these were already turned on."
# Carried: the design's owner answers (2026-10-28): every class but black
# auto-advances through R4 once guards pass and soak ends; the paper (R5)
# only by a human act; a revert may request a planned refresh everywhere;
# the owner's computers are R2; registry and hand walk first, controller
# after. And: "we shouldnt keep shipping code in off state."
readiness: executable  # on the owner's word above

review:
  reviewer: none  # executable on the owner's word; each landed change takes its own tier
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-11-03T01:30:00Z
  source:
    canonical_ref: origin/main 790af661d
    deploy_identity: staging deploying 3e99c7307 (Claude arm delegate grant), CI in progress
  worktrees:
    - path: /Users/wiz/Dev/go-choir
      status: dirty
      class: other_agent_wip
      disposition: untouched (branch cutover/data-system; never commit there)
    - path: /Users/wiz/Dev/go-choir-switches-settings
      status: dirty, uncommitted, half-done (graph_home.go does not compile)
      class: abandoned builder WIP
      disposition: discard (git checkout -- .) before reuse
    - path: /Users/wiz/Dev/go-choir-screens, /Users/wiz/Dev/go-choir-cli-onboarding
      status: pushed branches selfdev-screens-as-files, cli-agent-onboarding
      class: parked self-development work
      disposition: held with the self-development goal
  observed:
    - Design docs/design/progressive-delivery-2026-10-26.md, ratified 2026-10-28.
    - Built: S0 registry switches/registry.yaml (86 entries; 67 release, 16 ops, 2 permission, 1 experiment; ring all 37, none 17, the rest a few ids); S1 lint scripts/check-switch-registry in CI; F-0 partly (computer_settings.go:44 and the file_objects and app_package_objects readers fall back to the map default; file_object.go:373 does not).
    - Not built: the hand walk; the ring controller (S3a shadow, S3b acting); live follow for settings and guest env (S4); ring-ordered refresh (S5); auto rollback (S6).
    - Release cap baseline 64 against the design's 12.
    - Four host rows (host_fetch_vms, choir_capsule_mirrors, proxy_expand_requesters, gateway_model_scheduler) hit their grandfathered_off_until date 2026-11-04 and fail the lint after it.
    - Audit: docs/evidence/switch-audit-2026-11-03.md (decision sheet, guest and host audits, conflicts C1 to C13).
    - Owner ruling 2026-11-03: texture.hold_trajectory stays (on).

finish:
  deliver: >-
    No switches exist. A release reaches every computer by ring-ordered
    refresh (d@e, test computers, the owner's computers, every other
    computer, then the paper, automatically and last), each ring after its
    soak, and rolls back automatically when a health check breaks. Every
    switch in the backlog is gone: its code runs everywhere, or the code is
    deleted. Nothing the owner asked for sits built but off.
  artifact:
    - zero switches in the code and nix (registry empty, then deleted); the lint refuses any new switch
    - ring-ordered guest refresh in vmctl's refresh sweep (design slice S5), paper last after the soak
    - automatic code rollback on a broken post-deploy health check (design slice S6)
    - host fetch and capsule mirrors operational on every computer; story expansion for every signed-in user; the gateway model scheduler deleted
  acceptance:
    - a release walks the rings on staging with soaks, and the paper refreshes last at a planned moment
    - a planted bad release is rolled back automatically at d@e and never reaches the next ring; the history is the artifact
    - grep finds no switch; check-switch-registry passes with an empty registry and refuses an added one
  rollback: >-
    Revert the refresh-order and rollback commits (deploys return to
    refresh-all). A deleted switch whose code turns out wrong is fixed
    forward, or its feature commit is reverted.
  landing: required (commit, push, CI, staging deploy identity, deployed acceptance)

value:
  better_means: >-
    Minimize code that exists but does not run, and person-hours per
    rollout, while no computer is left stuck or broken by a release and the
    paper keeps publishing.
  goodharting_would_be: >-
    Deleting switches by deleting features the owner asked for, or flipping
    everything on without proof so a broken feature runs everywhere; or a
    rollback that never fires because its health check cannot fail.

homotopy:
  realism_axis: >-
    backlog walked group by group with the owner; then ring order for
    refresh on staging; then automatic rollback with a planted bad release;
    then the paper last, automatically.

boundaries:
  mutation_class: red (vmctl refresh order, deploy rollback, every switch removal that changes behaviour)
  review: tier 2 for refresh order and automatic rollback; per switch removal by its class
  must_preserve:
    - crash restarts close work; planned restarts resume (CLAUDE.md)
    - the paper refreshes only at a planned, marker-backed moment, never mid self-development apply or with a live capsule
    - adversarial drills only on d@e; b@c is never deleted
    - "upgrading things should never leave a computer in a stuck state" (owner)
    - no secrets or owner email addresses in the repo
  excluded:
    - the switch controller (dropped, owner 2026-11-03)
    - self-development app rounds (on hold)
    - the simulated-user fleet (owner: "not this week")

slices:
  P0: rulings into the docs (done 2026-11-03: CLAUDE.md and AGENTS.md, design rulings 1 to 8, the audit's backlog walk groups 1 to 7, the calibration plan).
  P1: ring-ordered refresh (design S5): vmctl's refresh sweep refreshes d@e, then test computers, then the owner's computers, then every other computer, then the paper last at a planned, marker-backed moment, each ring after its soak.
  P2: automatic code rollback (design S6): a post-deploy health check per ring; a breach returns that ring to the previous release and stops the walk; a planted bad release proves it.
  P3: the backlog, in batches that ride ring deploys, as walked with the owner (docs/evidence/switch-audit-2026-11-03.md, "Backlog walk"): deletions with code on everywhere; the named fixes first where the walk says so (files.object reader, partition GC, checkpoint append hold, delegate tunnel pass, root desk ordering, debounce check, carry pruning).
  P4: named builds from the walk: the gateway scheduler deleted; host fetch and capsule mirrors operational everywhere; story expansion for every signed-in user; the Claude delegate as an owner act; research reads of the paper's APIs from any computer; calibration feedback on with the engineering A/B; evidence.graph measured on a paper store copy.
  P5: the Library pulled forward (package-aware containment and re-materialization, design/library-package-propagation-2026-10-26.md S6, with app desks wired), so source projection runs everywhere and the rebase offer is deleted.
  P6: the lint refuses any switch; the registry deleted.

now:
  status: working
  slice: P1
  conjecture: >-
    C1: if every switch is deleted into on-everywhere code and releases
    roll out by ring with automatic rollback, the owner never again finds
    asked-for code not running, and no computer is stuck by a release.
    Test: zero switches by grep; a planted bad release stopped at d@e.
    Edge: missing_oracle (a regression only the paper's load shows, after
    its own refresh). dO: the paper's health checks and the late-regression
    window gate its automatic refresh and roll it back.
  decision: self-development goal on hold at its checkpoint; its parked branches stay unmerged
  believed_state: registry and lint exist; nothing walks; most rows are off or on for one or two ids
  main_uncertainty: which guards have known-bad fixtures today
  next_observation: list each ON-NOW row's promotion metric and whether /health or the journal exposes it now
  blocker: none
  next_action: P1 ring-ordered refresh: read vmctl's deferred-refresh sweep and the design's S5, then build in a worktree off origin/main (tier 2)

receipts: []
