---
definition_version: 3
definition_id: choir-rectification-spine-2026-09-25
execution_mode: mission_orchestrator
draft: false

start:
  captured_at: '2026-09-25T00:00:00Z'
  source:
    canonical_ref: main@79d0b52b
    deploy_identity: staging https://choir.news build.commit=537fce04
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: git
  predecessor:
    mission: choir-commitment-ledger-desk-carrier-draft-2026-09-23
    disposition: >-
      R2 closed on substrate 2026-09-25 (landed+deployed 537fce04, resolve
      5404d2c7). The desk-RLM rectification spine was restructured by
      consensus and ratified by the owner 2026-09-25
      (desk-rlm-rectification-plan-2026-09-23.md §11).

finish:
  deliver: >-
    The desk-RLM spine is complete: desks are live root RLMs on the in-cell
    carrier, management casts engineering, Texture authors genuine
    AuthorAppAgent revisions metabolizing the commitment ledger under
    editorial discretion, scores and materiality surface to the owner, and
    the self-development gate (M11) has run a reversible-selfdev episode
    whose receipts are scored commitment_records readable in the live
    Texture doc — the owner-visible product point.
  artifact: >-
    The ratified mission spine in desk-rlm-rectification-plan-2026-09-23.md
    §11 executed to M11: every mission's terminal receipt landed and
    deployed; the live Texture doc is the self-development supervision
    surface.
  acceptance:
    - action: >-
        On choir.news, an owner opens a live engineering-bound doc mid-task
        and sees genuine AuthorAppAgent revisions advancing — citing
        commitment records — while a self-development operation drives a
        real code change with no external harness, and its episode receipts
        are scored commitment_records visible in the doc.
      proves: >-
        the supervision loop is live (R3d+R4) and self-development is real
        and supervised (M7+M11) — the product point.
      evidence_class: deployed proof + human inspection
    - action: >-
        Every mission in the §11 spine (R2x, R3a, R3b, R3c, R3d, R3r, R4,
        R5a, M7, M9a, M11) has a terminal receipt in mission-graph.yaml with
        status settled and a deployed-acceptance ref.
      proves: the spine closed end-to-end, not as a subset.
      evidence_class: registry + receipts
  rollback: >-
    git revert per mission + the computer's acceptance-fenced restore; each
    mission lands independently so the rollback boundary is per station.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the gap between "agents run Choir" and "Choir runs and
    supervises its own development visibly" — maximize the share of desk
    work that executes as ledger-recorded cells consumed by a live Texture
    doc, while preserving canonical-evidence and restart-durability
    invariants.
  goodharting_would_be: >-
    Desks that run cells but still write the doc via a host-side
    per-milestone projection, or a self-dev episode that produces records
    but no owner-readable doc — both satisfy "records exist" while missing
    the supervision product.

homotopy:
  realism_axis: >-
    Desk autonomy: low = engineering-only cells behind a tool-loop
    management (current); high = all desks as root RLMs on derivable wakes
    driving and supervising a reversible self-dev episode. Each station is
    a projection of the full spine, never a fake island.

boundaries:
  mutation_class: red
  authority_sources: [owner, doctrine, desk-rlm-rectification-plan-2026-09-23 §11]
  must_preserve:
    - the document channel is the owner-input path; a revision is the event
    - canonical evidence only — Dolt channel log + OG records
    - the derivable-wake dispatcher is the sole wake authority
    - the live Texture doc is the supervision surface and the M11 gate
    - resolver is never the claimant for engineering claims (D7)
    - scores never enter the acting agent's context (epistemic boundary)
  excluded:
    - world-wire M12-M16 (processor/reconciler RLM-ify, publication tx)
    - vmctl, sourcecycled, install_frontend_pointer (deferred planes)
    - M9b/M10 federation publish (after M11)
    - R5b durable rename-migrate (deferred indefinitely)
  protected_surfaces:
    - canonical event/commit path (rlm_reduce commit + CommitInboxCursor)
    - assignment fate saga + dispatcher due-index
    - Texture canonical writes + AuthorAppAgent authoring
    - checkpoint/route projection, auth/session renewal
    - run acceptance and deployment routing

now:
  status: working
  slice: >-
    station R2x — carrier substrate integrity. Wake-coverage audit done:
    stranded-frozen sweep is redundant (watchdog + boot reconcile cover it);
    deadline-cancel needs a new wake armed at bind. Atomic-commit repair
    still open. Live station goal:
    docs/definitions/choir-carrier-substrate-integrity-2026-09-25.md
  source_ref: main@79d0b52b
  deploy_identity: staging https://choir.news build.commit=537fce04
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: spine-produces-supervised-selfdev
    claim: >-
      Executing the ratified §11 spine (carrier integrity -> desk cells ->
      live supervision -> scores -> harness-skip -> push/restore -> gate)
      produces a self-development gate whose receipts are scored commitment
      records readable in the live Texture doc — real, supervised
      self-development, not a harness artifact.
    test: >-
      The M11 episode on staging: a self-dev operation promotes a candidate
      under reversible-selfdev consensus, falsifies candidate B, restores to
      the pinned head — and the owner reads every step in the live doc;
      the M11 acceptance binds the live doc, not only the ledger.
    delta_o: >-
      R3d+R4 land idea-level doc state before the gate, so the observer can
      read supervision state — not just event traffic — when M11 runs.
    scope_if_supported: the desk-RLM rectification spine through M11
    status: active
    evidence_refs:
      - docs/desk-rlm-rectification-plan-2026-09-23.md §11
  decision:
    what: >-
      One durable meta-goal covers the whole spine; a `now.slice` station
      pointer tracks the live mission. Each station is a separate
      throughline /goal file authored at handoff; this file's `now` is
      rewritten to point at it (AGENTS.md Goal-Station Handoff).
    kind: operational
    status: settled
    evidence_ref: docs/desk-rlm-rectification-plan-2026-09-23.md §11
    owner_ratification_ref: owner 2026-09-25 — ratified the spine; asked for
      one durable /goal file updated per station
  belief:
    believed_state: >-
      R0 and R2 landed; K's substrate is landed with only a deletion-sweep
      ruling pending; the restructured spine is ratified; R2x is the live
      station. The two substrate seams R2x repairs (non-atomic commit,
      selection-gated fate) are real and verified.
    main_uncertainty: >-
      Whether host-side sessionWorker spawn for non-capsule desks (R3b) is
      a lift or a build — the sharpest execution risk the panel named; the
      wake-coverage audit (R2x) bounds the fate-deletion side meanwhile.
    next_observation: >-
      R2x's wake-coverage audit is done — now watching the deadline-wake
      build and the commit-atomicity decision (transaction vs recovery).
  blocker_or_risk: >-
    Station pointer must be rewritten at every terminal receipt (the /goal
    string itself stays constant — this file is the entrypoint). Stale
    stations are the named failure mode the Goal-Station Handoff guards.
  next_action: >-
    Run R2x: deadline-wake at bind + sweep deletions + atomic commit.

receipts: []
---

# Rectification Spine — durable meta-goal

This is the single `/goal` entrypoint for the post-R2 desk-RLM spine. It is
intentionally thin: the **station** — the live mission — lives in `now.slice`
and points at a dedicated throughline `/goal` file for the current mission.

**How this file stays fresh (the discipline):**

- The `/goal` argument never changes — always this file.
- `now.slice` always names the current station and links its goal file.
- On each terminal receipt (mission lands / closes / is superseded):
  author the next station's `/goal` file, rewrite `now.slice` +
  `now.next_action` here, and update `docs/ACTIVE.md` + `mission-graph.yaml`
  (AGENTS.md "Goal-Station Handoff"). Delete superseded draft goal files.
- The full spine, mission definitions, owner rulings, and contested-join
  outcomes live in `docs/desk-rlm-rectification-plan-2026-09-23.md` §11 —
  read it before chartering the next station.

**Current station:** R2x →
[`choir-carrier-substrate-integrity-2026-09-25.md`](choir-carrier-substrate-integrity-2026-09-25.md)
— atomic act commit + fate-sweep ungating.
