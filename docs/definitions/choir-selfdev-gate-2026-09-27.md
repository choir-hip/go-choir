---
definition_version: 3
definition_id: choir-selfdev-gate-2026-09-27
execution_mode: mission_orchestrator

start:
  captured_at: '2026-09-27T01:30:00Z'
  source:
    canonical_ref: main@bc58fa8f
    deploy_identity: staging https://choir.news build.commit=bc58fa8f
    worktrees:
      - path: /Users/wiz/go-choir
        status: clean
        class: source
        owner: this session
        touch: read_write
        recovery: git
  predecessor:
    mission: choir-vocab-decoders-seed-freeze-2026-09-26
    disposition: >-
      R5a landed+deployed 2026-09-26/27 (bc58fa8f, ci 36283101087, all
      gating lanes green). Every stratum-B family has a frozen decoder
      routed at its interpretation boundary; NormalizeHistoricProfile is
      the single stratum-C point (repaired two latent compare gaps);
      identity seeds pinned (keep-v3); the pre-migration fold is proven
      identical. The proof-tape precondition for M11 is discharged.
    evidence_ref: docs/definitions/choir-vocab-decoders-seed-freeze-2026-09-26.md
  spine_meta_goal: docs/definitions/choir-rectification-spine-2026-09-25.md
  station: M11
  owner_directive: >-
    Plan §11.2 M11 (red/black, last spine station): the
    reversible-selfdev-v1 episode — expected on material actions,
    qualified consensus, promotion, live-play, candidate-B
    falsification, restore to a pinned head. Receipts are scored
    commitment_records readable in the live Texture doc (owner ruling
    2026-09-25: records-only supervision rejected; the live doc is the
    supervision surface). Depends: M7 + M9a + R3d + R4 + R5a — all
    settled.
  substrate_verified: >-
    M7 (722b49bf): derivable selfdev continuations — canonical decision
    commit drives materialize->apply with no API driver. M9a (44e4169e):
    platform-signed update push + pinned-head restore via
    tape_reconstruct on a baseless computer. R3d (119e0edd): texture
    live on cell carrier, authors revisions from the ledger. R4
    (68a2e023): accrual views, materiality projection, own-score-free
    packs, learning-claims gate. R5a (bc58fa8f): frozen decoders + fold
    proof.

finish:
  outcome: >-
    One real self-development episode executes end to end on staging
    under supervision: a candidate change is authored through desk
    cells, every material action carries expected-heads, a qualified
    consensus receipt under reversible-selfdev-v1 ratifies the decision,
    the operation promotes and goes live, a second candidate is
    falsified and stays falsified-visible in the doc, and the computer
    restores to its pinned pre-episode head. Every step mints scored
    commitment_records readable in the live Texture doc — supervision
    receipts, not a harness log.
  artifact: >-
    A terminal episode record on staging: the selfdev operation's
    durable state transitions (proposed -> decided under
    qualified-consensus -> materialized -> applied -> verified ->
    restored), the falsified candidate B record, and the live Texture
    doc rendering the scored commitment_records for the episode. Plus
    the probe script that runs it and the evidence file.
  acceptance:
    - action: >-
        Boundary probe first: map the exact API/cell steps of a
        reversible-selfdev-v1 episode on staging — operation open, owner
        (or consensus) decision, materialization, apply, verification,
        falsification of candidate B, restore — and name which steps the
        platform exposes end to end vs. which require desk-cell
        authorship. Scope deployed acceptance to reachable steps;
        document any step that cannot execute on staging and why.
      proves: R2-boundary discipline — acceptance names only reachable
        evidence; the episode shape is settled before execution.
      evidence_class: boundary probe
    - action: >-
        A selfdev operation opens on a staging computer with expected
        heads on every material action; the owner's decision (or a
        qualified-consensus receipt under reversible-selfdev-v1)
        finalizes it; M7's derivable continuation drives
        materialize->apply with zero external API calls after the
        decision commits.
      proves: expected-heads + consensus ratification + harness-skip —
        the operation advances itself between decisions.
      evidence_class: deployed acceptance + canonical event chain
    - action: >-
        The candidate promotes and the verifier's live-play check lands
        a verification_recorded event; then the computer restores to the
        pinned pre-episode head through the M9a path (route demotion +
        tape reconstruct), witness matched.
      proves: promotion + live-play + restore — the episode is
        reversible under the spine's own machinery.
      evidence_class: deployed acceptance
    - action: >-
        A second candidate (B) is proposed and falsified; the falsified
        commitment remains visible in the live Texture doc's materiality
        projection alongside the episode's other scored
        commitment_records — readable in the doc, not only in storage.
      proves: falsification-visible supervision on the live doc — the
        owner ruling's gate.
      evidence_class: deployed acceptance + doc render
    - action: >-
        The episode's commitment_records score through R4 accrual —
        per-agent accrual over the episode's Precommit/Resolve stamps —
        and every commitment decodes under R5a's frozen vocabulary
        (proof tape readable under frozen decoders).
      proves: receipts are scored commitment records on the frozen
        vocabulary — the gate's full criterion.
      evidence_class: deployed acceptance + local test
  rollback: >-
    The episode is reversible by construction (restore to pinned head is
    an acceptance leg, not a recovery drill). Repo changes are limited
    to the probe script + evidence; git revert restores. If the episode
    wedges mid-flight, the computer restores via the M9a path and the
    operation record carries terminal_error — the tape is the audit.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    The whole spine exists for this: a computer that develops itself
    under supervision whose receipts a human reads. If M11 runs, every
    prior station's mechanism composes in product — cell-authored
    change, consensus-ratified decision, driverless materialization,
    reversible apply, falsification-visible supervision — and the gate
    is real, not demonstrated on a fixture.
  goodharting_would_be: >-
    Running the episode with a hidden driver (API calls between
    decisions), faking the consensus receipt, falsifying candidate B in
    a way the doc never shows, restoring by snapshot outside the event
    chain, or counting a partial episode. Acceptance requires the live
    doc to carry the falsified record and the canonical event chain to
    carry every transition.
  falsifiers:
    - 'Falsified: any operation transition required an external API call
      after the decision committed — M7 derivable continuations did not
      drive it.'
    - 'Falsified: candidate B''s falsified record is absent from the
      live doc — supervision cannot see the loss.'
    - 'Falsified: the restore minted a new head without reconstructing
      the tape — reversibility is cosmetic.'
    - 'Falsified: any episode commitment_record fails the frozen-vocab
      decode or misses a score stamp — receipts are not scored records.'

now:
  status: working
  slice: >-
    Chartered 2026-09-27 on R5a's settled receipt — all dependencies
    landed+deployed. First: boundary probe over the selfdev API surface
    and desk-cell authorship path to settle the exact episode steps and
    the staging-reachable subset.
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: reversible-selfdev-gate
    claim: >-
      The ratified spine (carrier integrity -> desk cells -> live
      supervision -> scores -> harness-skip -> push/restore -> frozen
      vocab) composes into a supervised self-development episode that
      runs on staging with no hidden driver, ends at a falsified-
      visible, restored terminal state, and leaves scored commitment
      records readable in the live Texture doc.
    test: >-
      Deployed: the episode's durable transitions + restore + doc
      render. Local: accrual over the episode's stamps + frozen-vocab
      decode of every episode record.
    delta_o: >-
      M11 is the wire gate: once it settles, the automatic computer
      demonstrably develops itself under supervision and the M14-M16
      world-wire missions unlock.
    scope_if_supported: self-development gate satisfied; spine complete
    status: active
    evidence_refs:
      - docs/desk-rlm-rectification-plan-2026-09-23.md §11.2 (M11), §11.3 (owner rulings)
      - docs/world-wire-mission-stack-2026-09-22.md (3e gate)
  belief:
    believed_state: >-
      Every dependency is landed and proven: management/engineering/
      texture/research desks on cells; ledger read surface with
      materiality projection; derivable continuations drive selfdev ops
      with no post-decision API calls; platform push + pinned restore
      proven tonight; frozen decoders make the proof tape permanently
      readable. Open at charter: which episode steps stage end to end —
      consensus receipt minting, candidate authoring through cells,
      verifier live-play — the boundary probe settles this first.
    next_observation: >-
      The reachable surface: selfdev API routes on choir.news, the
      decision-binding path (owner vs qualified-consensus receipt), the
      candidate-B path, and whether a live doc render of the episode's
      records is observable this station.
  blocker_or_risk: >-
    Scope risk: the full episode may exceed one station if consensus
    receipt minting or candidate authoring needs new substrate —
    boundary probe names reachable vs deferred steps explicitly rather
    than silently narrowing. Safety: the computer used must be a fresh
    staging computer, not an owner computer; restore target is the
    pre-episode pinned head minted before the op opens.
  next_action: >-
    (1) Boundary probe: enumerate the episode's API + cell steps and the
    staging-reachable subset; (2) probe script; (3) run episode on a
    fresh staging computer; (4) doc-render leg; (5) receipts + spine
    terminal.

receipts:
  - "charter: M11 = self-development gate per plan §11.2 + stack 3e —
    reversible-selfdev-v1 episode on staging: expected-heads on material
    actions, qualified consensus under the gate, promotion, live-play,
    candidate-B falsification, pinned-head restore; receipts = scored
    commitment_records readable in the live Texture doc. All
    dependencies settled (M7 722b49bf, M9a 44e4169e, R3d 119e0edd, R4
    68a2e023, R5a bc58fa8f). Class red/black. Boundary probe precedes
    execution; unreachable steps get named, not silently dropped."
---

# M11 — Self-Development Gate (final spine station)

Live station under [`choir-rectification-spine-2026-09-25.md`](choir-rectification-spine-2026-09-25.md).
Scope per [`desk-rlm-rectification-plan-2026-09-23.md`](../desk-rlm-rectification-plan-2026-09-23.md)
§11.2 and the mission stack's 3e gate: the reversible-selfdev-v1 episode, receipts as
scored commitment_records readable in the live Texture doc.

This is the gate the whole spine was built for. Nothing here is new substrate — every
mechanism is already landed and proven separately; M11 proves they compose.
