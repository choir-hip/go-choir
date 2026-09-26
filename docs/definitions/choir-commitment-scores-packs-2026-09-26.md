---
definition_version: 3
definition_id: choir-commitment-scores-packs-2026-09-26
execution_mode: mission_orchestrator

start:
  captured_at: '2026-09-26T08:10:00Z'
  source:
    canonical_ref: main@dc340620
    deploy_identity: staging https://choir.news build.commit=<pending R3r deploy>
    worktrees:
      - path: /Users/wiz/go-choir
        status: clean
        class: source
        owner: this session
        touch: read_write
        recovery: git
  predecessor:
    mission: choir-research-live-cell-2026-09-26
    disposition: >-
      R3r landed+deployed 2026-09-26. Research is a full-RLM desk on the
      cell carrier: desk_go_eval + the typed research/evidence/memory
      surface under a per-activation egress budget (calls + fetched bytes)
      and a 1GiB worker address-space cap; generic host tools retired.
      Every non-wire desk is on the carrier; only processor/reconciler
      (D4-deferred) remain on the tool loop.
    evidence_ref: docs/definitions/choir-research-live-cell-2026-09-26.md
  spine_meta_goal: docs/definitions/choir-rectification-spine-2026-09-25.md
  station: R4
  owner_directive: >-
    Plan §11 R4 (on spine, after R3d): scores + surfacing + packs —
    derived accrual views, materiality projection, context packs,
    learning-claims gate. Proof: a falsified commitment stays visible in
    the doc; the acting desk's pack contains zero own-score fields.
    Owner ruling 2026-09-25: the live Texture doc gates M11 — "texture doc
    as live supervision is extremely important … the delivery and UX for
    both the agent control plane and world wire articles — living
    documents." Records-only supervision is rejected.

finish:
  outcome: >-
    The commitment ledger gains its read surface: derived accrual views
    computed from the append-only score stamps (no new write authority —
    scores stay stamp-shaped on linked records); a materiality projection
    surfacing top-level claims, falsified claims, and overdue commitments
    (§2 foliation — Texture renders under editorial discretion, not the
    org chart); desk context packs that carry observations and
    discrepancies to the activation while carrying zero own-score fields
    (the epistemic-boundary invariant — feeding scores into context
    recreates reward hacking); and the learning-claims gate binding
    self-development outcome claims to scored commitment_records.
  artifact: >-
    A derived accrual view over choir.commitment_record bodies;
    a materiality projection (claims/falsified/overdue) readable by the
    texture desk's evidence surface; a context-pack builder with the
    own-score exclusion asserted by construction; gate enforcement on
    learning claims. No schema changes to commitment_record.v1; accrual
    is a read, not a rewrite of the stamp.
  acceptance:
    - action: >-
        Seeded Precommit + linked Resolve records (multiple scorers,
        disagreement preserved) read through the derived accrual view and
        return per-agent/per-doc accrual without mutating any stored
        record.
      proves: derived accrual over stamps, not a second write authority.
      evidence_class: local test
    - action: >-
        The materiality projection over seeded records surfaces a
        falsified claim, a top-level open claim, and an overdue
        commitment distinctly — and a falsified commitment remains
        visible (never dropped, never rewritten).
      proves: materiality projection is real and falsification-visible.
      evidence_class: local test
    - action: >-
        The context pack built for the acting desk contains its
        observations and discrepancies and zero own-score fields; a pack
        built for the supervising surface does carry scores.
      proves: the epistemic boundary holds by construction, not by
        prompt instruction.
      evidence_class: local test
    - action: >-
        A self-development outcome claim (learning claim) that cites no
        scored commitment_record is refused or flagged by the gate; one
        citing resolved records passes.
      proves: learning-claims gate binds claims to scored evidence.
      evidence_class: local test
    - action: >-
        Boundary probe first: name exactly which fields of the texture
        doc's evidence surface the projection feeds, and whether a live
        doc render on staging is observable this station — deployed
        acceptance is scoped to what the probe finds reachable; nothing
        unreachable is promised.
      proves: R2-boundary discipline — acceptance names only reachable
        evidence.
      evidence_class: boundary probe + local test
  rollback: >-
    git revert + redeploy. Reverting removes the accrual view, the
    materiality projection, pack filtering, and the gate; commitment
    records stay intact and rescored reads reproduce — no stored body
    changes, so the revert is fully reversible. Desks on the carrier
    (R3b-R3r) are unaffected.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    The ledger stops being write-only. Supervision reads idea-level
    state — what claims stand, what was falsified, what is overdue —
    instead of event traffic; the acting desk gets honest feedback
    (observations + discrepancies) without the score leakage that turns
    optimization into reward hacking; and M11's live-doc gate has the
    doc surface it was ratified to require.
  goodharting_would_be: >-
    Building accrual that rewrites the stamps, a projection that hides
    falsified claims to look clean, packs that quietly carry own-scores
    back to the acting desk, or a gate that accepts unbacked learning
    narration. Acceptance insists the stamp is append-only, falsified
    claims stay visible, the pack boundary is asserted by construction,
    and the gate binds claims to scored records.
  falsifiers:
    - 'Falsified: accrual mutates or supersedes stored records — it is a
      hidden second write authority.'
    - 'Falsified: a falsified commitment disappears from the projection —
      supervision airbrushes the log.'
    - 'Falsified: the acting desk''s pack contains own-score fields — the
      epistemic boundary leaks.'
    - 'Falsified: an unbacked learning claim passes the gate — claims are
      still narration.'

now:
  status: working
  slice: >-
    station R4 CHARTERED 2026-09-26. Substrate (re-verified at charter):
    choir.Resolve writes one CommitmentScore stamp on a linked record
    (commitmentRecordForIntent IntentResolve branch, rlm_reduce.go) —
    accrual does not exist; the CommitmentRecord body (v1) already
    carries TypedQuestion probabilities, Consequence weights (materiality
    = likelihood x impact x relevance), Scores plural, Provenance; OG
    objects are append-only so derived views read bodies without write
    authority. Texture desk evidence surface exists post-R3d (ledger
    dual-read R3a); the projection feeds through that surface, not a new
    store. Boundary probe item 5 runs before implementation.
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: accrual-projection-packs-gate
    claim: >-
      Derived accrual over the append-only score stamps, a
      falsification-visible materiality projection, own-score-free desk
      packs, and a learning-claims gate bound to scored records make the
      commitment ledger supervision-readable without leaking scores into
      the acting context.
    test: >-
      Local: seeded records read through each surface and return the
      derived material; falsified claims stay visible; acting-desk packs
      carry zero own-score fields; unbacked claims fail the gate.
      Deployed: platform healthy at R4 head; live doc render scoped by
      the boundary probe.
    delta_o: >-
      With R4 landed, R3d+R4 complete idea-level doc state; the M11 gate
      can require the live doc to carry scored commitment evidence and
      M7's management-driven selfdev has supervision that reads world
      models, not just events.
    scope_if_supported: ledger read surface complete; M11 live-doc gate unblocked
    status: active
    evidence_refs:
      - docs/desk-rlm-rectification-plan-2026-09-23.md §2 (foliation), §11 (R4)
      - docs/Precommitment Records — Engineering Memo.md (score semantics)
  belief:
    believed_state: >-
      CHARTERED. Every non-wire desk is on the carrier; the ledger
      accumulates commitment_records with score stamps; nothing reads
      them back for supervision — the gap R4 closes. The record body
      already carries the materiality fields; accrual is derivation,
      not schema work.
    next_observation: >-
      Boundary probe: the texture evidence-surface field names the
      projection must feed, and whether staging doc traffic makes a live
      render observable this station.
  blocker_or_risk: >-
    Scope risk: "learning-claims gate" has no settled enforcement point —
    charter-time probe must locate it (selfdev operation claims, texture
    doc claim surfaces, or ReportPacket claims) and bound it to one
    reachable seam rather than a blanket filter. Second risk: accrual
    must not become a query-time rescore engine beyond what the plan
    ratified — views derive from stored stamps; rescoring bodies is R5b
    adjacency, excluded.
  next_action: >-
    Boundary probe (evidence-surface feed + live-render observability),
    then: (1) accrual view; (2) materiality projection; (3) pack builder
    with own-score exclusion; (4) claims gate at the probed seam;
    (5) tests; (6) landing loop.

receipts:
  - "charter: R4 = scores + surfacing + packs per plan §11 (on spine,
    after R3d; owner ruling — live doc gates M11, records-only
    supervision rejected). Substrate re-verified: scores are stamps on
    linked records; CommitmentRecord v1 carries materiality weights;
    accrual/projection/packs do not exist. Boundaries: accrual derives,
    never rewrites; falsified claims stay visible; acting-desk packs
    carry zero own-score fields; gate binds claims to scored records.
    Boundary probe on the texture evidence feed + live-render
    observability runs before implementation."
---

# R4 — Commitment Scores + Surfacing + Context Packs (station on the rectification spine)

Live station under [`choir-rectification-spine-2026-09-25.md`](choir-rectification-spine-2026-09-25.md).
Scope per [`desk-rlm-rectification-plan-2026-09-23.md`](../desk-rlm-rectification-plan-2026-09-23.md) §11
(R4, on spine): derived accrual views, materiality projection, context packs, learning-claims gate.
Proof: a falsified commitment stays visible in the doc; the acting desk's pack contains zero
own-score fields.
