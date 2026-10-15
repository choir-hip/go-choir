---
definition_version: 4

# Station L3 of the ledger metamission
# (docs/definitions/choir-ledger-metamission-2026-10-13.md): hypotheses
# settled and scored. The design is settled in docs/precommitment-records.md
# §5.1 (settlement is separate from scoring; a committing desk never settles
# its own hypotheses; a score uses the committed version and its original
# probability; the commitment's seq precedes the earliest capture its
# settlement cites) and §5.2 (owner 2026-10-14: several scorers per
# settlement, each with its own track record; self-scores are not
# governance). This file encodes that design over the code observed on
# 2026-10-15; it does not reopen it.
# File name keeps the metamission's 2026-10-13 path; authored 2026-10-15.
readiness: drafted

review:
  reviewer: none
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-15T21:30:00Z
  source:
    canonical_ref: main 1f4ed5055
    deploy_identity: staging b244507f4 (deployed 2026-10-15T21:03:45Z)
  worktrees:
    - path: /Users/wiz/go-choir (untracked `sourcecycled`, `ograph-bench`, `cmd/desktop/frontend/`)
      status: dirty
      class: other_agent_wip
      owner: not this session
      touch: forbidden
      recovery: leave in place
    - path: branch land/store-b-retrieval-rebased (local, ba12a1e0c)
      status: clean
      class: goal_candidate
      owner: ledger metamission (station LR/L0 item, not L3)
      touch: read_only
      recovery: branch
  observed:
    - >-
      CastHypothesis (internal/types/commitment.go:328) has ID, Statement,
      Probability, Stakes, Crux, General, Meaning; no resolves_by and no
      criterion. CastRevision may change Probability.
    - >-
      Inline settlement exists: HypothesisSettlement {ID, Outcome} and
      HypothesisDrop ride a CommitmentReport; the inbox fold lets only the
      side that did not open a hypothesis settle or drop it, while open
      (internal/store/inbox.go:268-299). No evidence refs, settler identity
      or time on the settlement itself.
    - >-
      Independent scoring exists, owner-only: POST /api/calibration/settlements
      (internal/agentcore/calibration_api.go:105) writes one resolve record
      per scorer and target with CommitmentScore {ScorerAgentID,
      ScorerModelID} and EvidenceRefs; outcomes confirmed 1, qualified 0.5,
      contradicted 0.
    - >-
      collectCalibration (calibration_feedback.go:148-260) averages every
      outcome for a hypothesis, including inline Report.Settled outcomes as
      scorer protocol:<inbox>: settlement and scoring are conflated there,
      and two disagreeing scorers are averaged, not kept apart. It scores
      the original Cast probability (inferred from code, no test seen).
    - >-
      GET /api/calibration returns a per-desk Brier/ECE digest (window,
      MinN, 10% held out) built for in-context feedback, not a
      denominators-first analytics view.
    - >-
      A separate agent-claim calibration family (/api/precommit,
      internal/calibration/, types.CalibrationClaim with Criterion and
      Deadline; scorer methods rule, owner, independent, self) already
      separates claim, settlement and scorer. Engineering's choir.Precommit
      rule kinds settle from receipts within one assignment.
    - >-
      Jev judges citation support and intake labels; it does not score or
      settle Cast hypotheses.
    - >-
      Not present: a seq-precedes-cited-capture check; per-source analytics
      (lead time, independent corroboration, corrections); an off-host copy
      of the ledger (L3's entry criterion; docs/reports/choir-state-and-vision-2026-10-14.md:109).
    - >-
      Prompts: management opens stories with choir.Cast("texture:new", ...)
      carrying hypotheses {statement, probability} and is told probability
      is the chance independent research confirms it
      (rlm_management_runtime.yaml:23-24); no overlay mentions a horizon,
      a resolving date or a criterion.

finish:
  deliver: >-
    A dated, resolved hypothesis ledger on the paper: management commits
    short-horizon hypotheses with a resolving date and a criterion before
    the evidence that settles them; a designated settler (never the
    committing desk) settles each with an outcome and evidence refs;
    scorers write separate score records against the committed version;
    readers and the owner see Brier per desk with its denominators.
  artifact: >-
    resolves_by and criterion on CastHypothesis; a settlement record with
    settler identity and evidence refs; score records kept per scorer
    (disagreement kept, never averaged away); the seq ordering check; a
    per-desk analytics view with denominators; the off-host ledger copy.
  acceptance:
    - action: on staging, the paper's ledger read through /api routes and trace export after at least 7 days of operation
      proves: ten or more hypotheses with resolves_by of 7 days or less were committed before their resolving evidence (commitment seq precedes the earliest cited capture), and at least half are settled with evidence refs by a settler other than the committing desk
      evidence_class: deployed proof
    - action: GET the per-desk analytics view
      proves: Brier per desk is shown with n, settled, unsettled-past-resolves_by and per-scorer counts; disagreements are listed, not averaged
      evidence_class: deployed proof
    - action: restore the ledger from the off-host copy into a scratch store and compare heads
      proves: the entry criterion holds (a recoverable off-host copy)
      evidence_class: deployed proof
  rollback: >-
    Per slice, git revert. Schema fields are additive and omitempty, so a
    revert leaves committed records readable; no record is rewritten (the
    tape is append-only).
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Maximize informative settled hypotheses (resolution-weighted Brier gain
    against a base rate, with traceable evidence) per dollar, while the
    committing desk never settles its own hypotheses and every score reads
    the committed version.
  goodharting_would_be: >-
    Hypotheses written after the evidence, or scheduled facts and
    near-certainties padded for count; vague criteria that cannot fail;
    the committing desk or its self-score acting as the settler; Brier at
    n of about 10 read as calibration; disagreeing scorers averaged into
    one number.

homotopy:
  realism_axis: >-
    type and fold unit tests -> a minted test computer with a fixture story
    -> the paper with resolves_by only (clock started, nothing settled) ->
    settlement by the designated settler -> independent scorers (Jev, a
    second model) -> the analytics view over a week of the paper.

boundaries:
  mutation_class: red
  authority_sources:
    - owner rulings in docs/precommitment-records.md §5 (2026-10-14)
    - docs/definitions/choir-ledger-metamission-2026-10-13.md (spine)
    - docs/choir-doctrine.md
  must_preserve:
    - Texture is the sole writer of canonical document revisions
    - a committing desk never settles its own hypotheses
    - committed records are never edited; revisions are new records
    - settlement and scoring are separate records
    - replay of existing tapes is unchanged (new fields omitempty)
  excluded:
    - quality ranking of stories (owner 2026-10-15: think, don't build)
    - the Taiwan vertical's question set (L4)
    - reader-facing rendering (L5)
  protected_surfaces:
    - the commitment ledger (Cast and Report record schema, inbox fold)
    - calibration scoring and its feedback into desk context
    - management and Texture prompt overlays

now:
  status: working
  slice: >-
    L3-1 (start the clock): additive resolves_by (RFC 3339, at most 7 days
    after the Cast's commit time) and criterion on CastHypothesis, validated
    at Cast admission; the management overlays ask for both on short-horizon
    hypotheses; no settlement or scoring change yet.
  source_ref: 1f4ed5055
  deploy_identity: staging b244507f4
  candidate: {id: none, state: none, ref: none, base: none, digest: none, scope: []}
  conjecture:
    id: L3-C1
    claim: >-
      If management's hypotheses carry a resolving date of 7 days or less
      and a criterion, at least half of them can be settled with evidence
      refs from the paper's own later captures, without new sources.
    test: count, after 7 days on the paper, hypotheses past resolves_by with a settlement citing captures whose seq follows the commitment
    edge: missing_oracle
    delta_o: the seq ordering check and settlement evidence refs (L3-2) make "settled from later evidence" observable rather than asserted
    scope_if_supported: the paper's current 28 sources, one week
    status: proposed
    evidence_refs: [docs/precommitment-records.md]
  decision: {what: "L3-1 before L3-2 (settler records) before L3-3 (scorer records and the analytics view); the off-host copy is a parallel prerequisite owned by L0", kind: operational, status: proposal, evidence_ref: docs/precommitment-records.md, owner_ratification_ref: not_applicable}
  belief:
    believed_state: >-
      The ledger already separates who may settle (not the opener) and
      already has an owner-only independent scoring path with scorer
      identity; what is missing is the clock (resolves_by, criterion), the
      settler's evidence, and keeping settlement out of the scoring average.
    main_uncertainty: whether management writes hypotheses that a week of the paper's own captures can settle, or mostly long-horizon ones
    next_observation: the share of current Cast hypotheses on the paper whose statement names a dated, checkable event within 7 days (read-only, from /api inbox records)
  blocker_or_risk: >-
    The entry criterion (an off-host ledger copy) is not built and waits
    on an owner go-ahead for the backup target; L3-1 does not need it, but
    L3 cannot finish without it.
  next_action: >-
    Read-only: sample the paper's Cast hypotheses through the product API
    and classify how many are short-horizon and checkable; then run the
    agentic-consensus review of this file before any L3-1 code.

receipts: []
---

# L3: hypotheses settled and scored

Slices, in order:

1. **L3-1, start the clock.** `resolves_by` and `criterion` on
   `CastHypothesis`, validated at admission (parseable, after commit time,
   at most 7 days ahead; a criterion required when `resolves_by` is set).
   Management overlays ask for them. Nothing settles differently yet.
2. **L3-2, the settler's record.** A settlement carries settler identity
   and evidence refs; the fold keeps the opener-cannot-settle rule; the
   seq of the commitment must precede the earliest capture a settlement
   cites, or the settlement is recorded as not admissible for scoring.
3. **L3-3, scorers and the view.** Score records per scorer against the
   committed version (Jev and one other model first); inline outcomes stop
   being averaged in as `protocol:<inbox>` scores; disagreement is kept;
   a per-desk analytics view shows Brier with its denominators and the
   per-scorer split.

The off-host ledger copy is L3's entry criterion and an L0 parallel item;
it is tracked there, not built here.
