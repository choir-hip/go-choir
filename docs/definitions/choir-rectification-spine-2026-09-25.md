---
definition_version: 3
definition_id: choir-rectification-spine-2026-09-25
execution_mode: mission_orchestrator
draft: false

start:
  captured_at: '2026-09-25T00:00:00Z'
  source:
    canonical_ref: main@77f22ced
    deploy_identity: staging https://choir.news build.commit=53035642
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
    station M7 CHARTERED 2026-09-26 — skip the harness: selfdev
    operations advance to materialization on derivable continuations,
    driven by the management cell (goal docs/definitions/
    choir-selfdev-derivable-continuations-2026-09-26.md). R4 terminal
    2026-09-26 (deployed build 68a2e023): the ledger read surface is
    live — derived accrual, falsification-visible materiality
    projection feeding the texture doc's evidence surface, score-free
    acting-desk packs on the cell frame (choir.Pack), and the
    learning-claims gate (flag posture) on the selfdev verification
    event. M9a unblocks in parallel once M7 produces something
    signable; R5a before M11.
  source_ref: main@68a2e023
  deploy_identity: staging https://choir.news build.commit=68a2e023
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
    believed_state: >-
      R0, R2, R2x, R3a, R3b, R3c, R3d, R3r, and R4 landed; K's substrate
      is landed; the restructured spine is ratified; M7 is the live
      station. Every non-wire desk is on the cell carrier; D2 is
      discharged; the commitment ledger has its read surface — derived
      accrual, falsification-visible materiality on the texture doc,
      score-free acting packs, and the learning-claims gate. The next
      lift is M7 — derivable selfdev continuations driven by the
      management cell (on spine); M9a unblocks in parallel once M7
      produces something signable; R5a before M11.
    next_observation: >-
      M7 boundary probe: which event heads (selfdev transitions, capsule
      freezes, verification records) should wake the materializer
      reconciler, and how the management cell's pack/inbox surfaces
      carry operation state; then the advance-without-driver and
      crash/recovery proofs.

  blocker_or_risk: >-
    Station pointer must be rewritten at every terminal receipt (the /goal
    string stays constant — this file is the entrypoint). Owner directive
    (No Blocking Asks, in AGENTS.md): on non-obvious decisions prefer
    cognitive transforms / agentic-consensus / conservative default /
    documented deferral — not a blocking ask. PR 67 (CI build-cache + race
    compile fix) integrated: merged as 7d8e455 and already an ancestor of
    main at R3d's landing.
  next_action: >-
    Execute M7 (goal docs/definitions/
    choir-selfdev-derivable-continuations-2026-09-26.md): boundary
    probe first, then event-head-triggered reconcile, management
    carrier observation of op state, crash/recovery proof. M9a
    unblocks in parallel once M7 produces something signable; R5a
    must land before M11.
receipts:
  - "R4 terminal 2026-09-26: landed+deployed 68a2e023 (ci 36233806473,
    staging build.commit=68a2e023); goal docs/definitions/
    choir-commitment-scores-packs-2026-09-26.md now.status=settled.
    The ledger read surface is live: derived accrual views, a
    falsification-visible materiality projection feeding
    textureAvailableSourceEntities, score-free acting packs on the
    cell frame (choir.Pack), learning-claims gate (flag posture) on
    the selfdev verification payload. now.slice -> M7."

  - "R3r terminal 2026-09-26: landed+deployed 3b56c34e (dispatch run
    36230314065, staging build.commit=3b56c34e); goal docs/definitions/
    choir-research-live-cell-2026-09-26.md now.status=settled. Research
    on the carrier unconditionally: desk_go_eval + 13-tool typed surface,
    per-activation egress budget on all host-mediated network calls,
    8GiB RLIMIT_AS worker cap (1GiB OOM'd the autoputer baseline — the
    cap binds growth over baseline). Two superseded CI failures:
    linux-only unused-import vet (0683fd5d) and race-shard worker OOM at
    1GiB (759e66a4). now.slice -> R4."
  - "R3d terminal 2026-09-26: landed+deployed 119e0edd (ci 36222906276,
    staging build.commit=119e0edd); goal docs/definitions/
    choir-texture-live-authoring-2026-09-26.md now.status=settled. Texture
    full-RLM: desk_go_eval only, cell-authored ApplyTextureTurn commits
    mint AuthorAppAgent revisions citing ledger records, typed tools +
    worker_updates consumer deleted. now.slice -> R3r."
  - "R3c terminal 2026-09-26: landed+deployed b7ae7596 (ci 36216782941,
    staging build.commit=b7ae7596); goal docs/definitions/
    choir-management-live-cast-2026-09-26.md now.status=settled. Management
    on the carrier unconditionally; lifecycle controls kept beside
    desk_go_eval; deployed full-resolution rides the R2 cast saga on
    staging's substrate. now.slice -> R3d."
  - "R3b terminal 2026-09-26: landed+deployed b9f43583 (ci 36214084659,
    staging build.commit=b9f43583); goal docs/definitions/
    choir-desk-cell-carrier-2026-09-25.md now.status=settled. Live-desk
    deployed acceptance deferred to R3c (actuator=rlm not live on staging
    desks — same deferral discipline as R3a). now.slice -> R3c."
  - "R3a terminal 2026-09-25: landed 4cf82057 (ledger evidence dual-read
    over commitment_record); deployed live-record proof deferred to a doc
    desk report-cast. now.slice -> R3b."

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

**Current station:** M7 → skip the harness (selfdev ops advance on
derivable continuations driven by the management cell). R4 landed+deployed
(`68a2e023`, ledger read surface: accrual, falsification-visible
materiality, score-free acting packs, learning-claims gate). Station goal:
`choir-selfdev-derivable-continuations-2026-09-26.md`. Also unblocked:
M9a (once M7 produces something signable); R5a before M11.
