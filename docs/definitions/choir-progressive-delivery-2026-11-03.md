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
    A switch is temporary by construction. Each one is registered, walks
    R0 to R4 on its own when its guards pass and its soak ends, reverts on
    its own when a guard breaks, reaches the paper by one owner act, and is
    deleted (code default on, switch gone) once it is everywhere. The
    backlog of off switches is gone: every row in the audit is either
    retired into the code default or deleted with the code it guards.
  artifact:
    - the ring controller on Node B, acting, with a history file per switch
    - settings and selectors that follow the registry ring (no hand nix edit per step)
    - the audit backlog cleared: zero release switches off for every computer; live release switches under the cap of 12
    - the retire path: a switch at all rings is deleted in one commit with its registry row
  acceptance:
    - a planted-fault walk on staging: a harmless test switch promoted R0 to R1 by the controller, then reverted when a synthetic guard breaks; the history file is the artifact (design section 8)
    - one real switch walked R0 to R4 by the controller with no person, then retired
    - the lint shows no release switch off everywhere and the live release count at or under 12
  rollback: >-
    The controller has a freeze; reverting its commits returns rings to the
    registry file as written. Retired switches are code defaults; reverting
    the retire commit restores the switch.
  landing: required (commit, push, CI, staging deploy identity, deployed acceptance)

value:
  better_means: >-
    Minimize the number of switches that are off somewhere for a reason
    nobody is measuring, and the person-hours each rollout takes, while no
    computer is left stuck or broken by a flip and the paper never moves
    without the owner.
  goodharting_would_be: >-
    Flipping defaults to on without guards, or deleting switches whose
    features were needed, to make the count drop; or a controller that
    advances on "no alarm" with no fresh samples.

homotopy:
  realism_axis: >-
    hand walk with the controller's readouts, then controller in shadow
    beside the hand walk, then controller acting on R0 and R1, then through
    R4, then retire at all rings.

boundaries:
  mutation_class: red (host controller, VM lifecycle, settings plane); per-switch flips take their own class
  review: tier 2 for the controller (S3b) and live follow (S4); tier 1 for S3a shadow and F-0 completion; tier 0 for registry data and the lint
  must_preserve:
    - the paper advances only by an owner act; never restarted outside a planned, marker-backed refresh
    - crash restarts close work; planned restarts resume (CLAUDE.md)
    - adversarial drills only on d@e; b@c is never deleted
    - "upgrading things should never leave a computer in a stuck state" (owner)
    - no secrets or owner email addresses in the repo
  excluded:
    - self-development app rounds (on hold)
    - the simulated-user fleet (owner: "not this week")

slices:
  P0: registry truth. Reconcile switches/registry.yaml with the audit (stale rows, expiries, the delegate expiry, texture.hold_trajectory on everywhere per the owner); finish F-0 (file_object.go:373 and every reader honours the default); decide C1 to C13 into registry entries (revert mode one_way where off is not the inverse of on).
  P1: deletes. Every DELETE row in the audit with the code it guards (intake.event_wakes, store.event_table switch and writer arming, management.serial_tasks, store.compact_force, retired keys, DAMON, VMCTL_STOP_MANAGED_ON_EXIT, VMCTL_PRIMARY_KEEPALIVE_MODE, docs-only names, env twins).
  P2: the hand walk. The ON-NOW rows walked R0 to R4 by hand with the readouts the controller will use; the four 2026-11-04 rows on at d@e first.
  P3: the controller in shadow (design S3a): follow:<id> in selectsComputer, state.json, switches status endpoint and CLI, guards with replayed known-bad fixtures; compared with the hand walk.
  P4: the controller acting (S3b), then the planted-fault acceptance; then live follow for settings and guest env (S4).
  P5: the chains. Root desk on its own merit (owner: system-wide understanding for each user), after its F4/F5 excerpt-order failures are re-proved; source projection with it; the rebase offer once one non-upstreamed rebase is proved on d@e (the upstreamed case was solved 2026-10-27 by patch-identity containment, train 34); package changes take the Library's S6 path (design/library-package-propagation-2026-10-26.md 10.3); the gateway scheduler cap against wide slots; evidence.graph's bounded import.
  P6: retire. Every switch at all rings deleted into the code default; the cap ratcheted to 12.

now:
  status: working
  slice: P0
  conjecture: >-
    C1: if the registry is true and F-0 holds for every reader, a hand walk
    of the ON-NOW rows reaches R4 inside a day with no stuck computer, and
    its readouts are enough to calibrate the controller's guards. Test: the
    walk's per-ring readouts and zero stuck computers. Edge: missing_oracle
    (a regression only the paper's load or long uptime shows). dO: the
    late-regression window after R5.
  decision: self-development goal on hold at its checkpoint; its parked branches stay unmerged
  believed_state: registry and lint exist; nothing walks; most rows are off or on for one or two ids
  main_uncertainty: which guards have known-bad fixtures today
  next_observation: list each ON-NOW row's promotion metric and whether /health or the journal exposes it now
  blocker: none
  next_action: P0 registry reconciliation and F-0 completion in one worktree off origin/main

receipts: []
