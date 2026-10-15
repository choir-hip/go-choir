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
  captured_at: 2026-10-15T21:05:00Z
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
  corrections:
    - at: 2026-10-15T21:20:00Z
      by: review round 1 (gpt-6-luna, gpt-6.1-sol, gemini-3.8-flash)
      what: >-
        Observation 3, "Independent scoring exists", overstates the code:
        POST /api/calibration/settlements is an owner-authenticated endpoint
        that records scorer-labelled assessments; the scorer and model labels
        come from the owner's request, and no independent scorer is invoked
        or authenticated. Independent scoring is still to be built (L3-3).
        Observation 9's overlay path is
        internal/runtimeprompts/overlays/rlm_management_runtime.yaml:23-24.
        Observation 2's settlement has no settler or time of its own, but
        the enclosing Report record carries both.
    - at: 2026-10-15T21:20:00Z
      by: review round 1
      what: >-
        Sample behind now.belief: 38 hypotheses from the Cast records of the
        27 Texture inboxes in the paper's newest 100 runs (GET /api/runs,
        GET /api/inboxes/{id}/records), classified by a tense and date
        regex. It is a convenience sample, not a population estimate.
        It was taken at about 21:11 UTC. f658017ce said 21:45 and the first
        draft said captured 21:30; both were agent clock errors.

finish:
  deliver: >-
    A dated, resolved hypothesis ledger on the paper, with two kinds.
    - Forecasts: future events with a resolving date and a criterion,
      committed before the evidence that settles them.
    - Assertions: present-tense claims with a corroboration criterion.
    A designated settler, never the committing desk, settles each with an
    outcome and evidence refs. Scorers write separate, pinned score records
    against the committed version. Readers and the owner see per-desk and
    per-scorer results with their denominators, kind by kind, and
    per-source analytics.
  artifact: >-
    kind ("assertion" | "forecast", the vocabulary of
    docs/precommitment-records.md §5.5 and types.CalibrationClaim),
    criterion, and resolves_by (forecasts only) on CastHypothesis and on
    HypothesisState, projected by the inbox fold into ConversationHead.
    - A settlement record with settler identity and evidence refs,
      admissible after its round closes, still never by the opener.
    - An ordering witness comparable across inboxes for a commitment and
      each capture its settlement cites.
    - Score records per scorer, model-version pinned; self-scores marked
      non-governing; disagreement kept, never averaged away.
    - Per-scorer track records against an adjudicated audit sample.
    - Per-desk analytics by kind, with denominators: settled, overdue,
      withdrawn and interrupted.
    - The spine's per-source analytics: lead time, independent
      corroboration by owner and lineage, and corrections.
    - The off-host ledger copy.
  acceptance:
    - action: on staging, the paper's ledger read through /api routes and trace export after at least 7 days of operation
      proves: ten or more prospective forecasts (kind forecast, resolves_by of 7 days or less; re-dated assertions do not count) were committed before their resolving evidence by the ordering witness, and at least half are settled with evidence refs by a settler other than the committing desk. Ten is an operational floor, not calibration evidence
      evidence_class: deployed proof
    - action: GET the per-desk analytics view
      proves: per desk and kind, Brier is shown against a stated base rate with n, settled, overdue, withdrawn and per-scorer counts; disagreements are listed, not averaged; assertions never enter forecast figures
      evidence_class: deployed proof
    - action: GET per-scorer and per-source views; owner audit of a sample of settlements
      proves: each scorer has its own track record against the audit labels with denominators, and per-source lead time, corroboration and corrections are shown
      evidence_class: deployed proof
    - action: restore the ledger from the off-host copy into a scratch store and compare heads
      proves: the entry criterion holds (a recoverable off-host copy)
      evidence_class: deployed proof
  rollback: >-
    Per slice, git revert. Records committed with the new fields stay on the
    tape. A revert must keep reading them, so the declaration change ships
    with a test that the old code path ignores the fields and replay
    still matches. No record is ever rewritten or backfilled (the tape is
    append-only). A live-paper smoke check after each slice deploys: story
    Casts are still admitted and stories still publish.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Maximize informative settled forecasts (resolution-weighted Brier gain
    against a stated base rate, with traceable evidence) per dollar, with
    assertions measured on their own (corroboration rate and lead time).
    Throughout, the committing desk never settles its own hypotheses, and
    every score reads the committed version and its original probability.
  goodharting_would_be: >-
    Any of these:
    - forecasts written after the evidence, or assertions re-dated to pass
      as forecasts;
    - scheduled facts padded for count;
    - criteria that cannot fail;
    - a probability band forced on agents, which rewards artificial
      uncertainty;
    - the committing desk or its self-score acting as the settler;
    - Brier at n of about 10 read as calibration;
    - disagreeing scorers averaged into one number.

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
    - replay of existing tapes is unchanged; replay and head verification never apply admission-time checks (no time.Now comparison on a historical record)
    - the Cut 2 freeze receipt (docs/reports/reeng-cut2-freeze-receipt-2026-10-13.md) is kept; the CastHypothesis change appends a compatible-extension receipt
    - no backfill of kind, criterion or resolves_by into existing records; a record without kind is legacy and outside the forecast cohort
    - inbox sequence numbers are per inbox and never compared across inboxes
  excluded:
    - quality ranking of stories (owner 2026-10-15: think, don't build)
    - the Taiwan vertical's question set (L4)
    - reader-facing rendering (L5)
  protected_surfaces:
    - the commitment ledger (Cast and Report record schema, inbox fold, command identity digests)
    - the Cut 2 freeze receipt and the frozen CastHypothesis declaration
    - calibration scoring and its feedback into desk context
    - management and Texture prompt overlays

now:
  status: working
  slice: >-
    L3-1, which starts the clock:
    - Add kind ("assertion" | "forecast"), criterion and resolves_by to
      CastHypothesis and HypothesisState, with the fold projecting them.
    - Admission validates a new Cast only:
      - kind is required on new hypotheses;
      - a criterion is required for both kinds;
      - resolves_by is required for forecasts only: RFC 3339, after the
        first-commit time and at most 7 days after it, by the runtime's
        commit clock.
    - A malformed hypothesis refuses the Cast with a cell error naming the
      field, which management can retry. It never wedges the desk.
    - The new fields join the identity of new commands. Old commands and
      records replay unchanged.
    - The management overlays ask for the kinds, with the exact format.
    - No change to settlement or scoring yet.
  source_ref: 1f4ed5055
  deploy_identity: staging b244507f4
  candidate: {id: none, state: none, ref: none, base: none, digest: none, scope: []}
  conjecture:
    id: L3-C1
    claim: >-
      If management's forecasts carry a resolving date of 7 days or less
      and a criterion, at least half of them can be settled with evidence
      refs from the paper's own later captures, without new sources.
    test: count, after 7 days on the paper, forecasts past resolves_by with a settlement citing captures that the ordering witness places after the commitment
    edge: missing_oracle
    delta_o: the cross-inbox ordering witness and settlement evidence refs (L3-2) make "settled from later evidence" observable rather than asserted
    scope_if_supported: the paper's current 28 sources, one week
    status: proposed
    evidence_refs: [docs/precommitment-records.md]
  decision: {what: "two kinds (assertion, forecast) per precommitment-records §5.5; L3-1 (clock) may start before the off-host copy as the spine's clock-only exception; L3-2 (settler records, ordering witness) waits for the recoverable off-host copy; L3-3 (scorer records, views) after L3-2", kind: operational, status: proposal, evidence_ref: .agentic-consensus/l3-drafted-review-2026-10-15/, owner_ratification_ref: not_applicable}
  belief:
    believed_state: >-
      The ledger already separates who may settle (not the opener) and
      has an owner-authenticated endpoint that records scorer-labelled
      assessments. What is missing: the kinds and the clock (kind,
      criterion, resolves_by), the settler's evidence, an ordering witness
      across inboxes, real independent scorers, and keeping settlement out
      of the scoring average.
    main_uncertainty: >-
      Whether management, asked for forecasts beside assertions, writes
      informative ones: future, dated, checkable and not scheduled facts.
      The sample (start.corrections) found 32 of 38 assertions and 1 dated
      forecast; 18 of 38 have p >= 0.85.
    next_observation: >-
      After L3-1 deploys, the kind mix and the forecast share that name a
      checkable event within 7 days, on a week of new Casts.
  blocker_or_risk: >-
    Two risks.
    - The off-host ledger copy is not built, and waits on an owner go-ahead
      for the backup target. It gates L3-2, not L3-1.
    - Live-paper risk in L3-1: the paper's management turns 24/7. A strict
      admission check with a prompt the model misreads could refuse story
      Casts and stop publishing. The guards are the per-field cell error,
      the smoke check, and reverting the overlay without the schema.
    Red ceremony:
    - conjecture delta: a hypothesis states how it settles (kind,
      criterion, date);
    - heresy delta: discovered that inline settlements are averaged in as
      scorer protocol:<inbox>; introduced none so far.
  next_action: >-
    Review round 2 on this repaired file. Round 1 sent it back; its
    findings are adopted here. Then L3-1, tests first, in a worktree.

receipts: []
---

# L3: hypotheses settled and scored

Slices, in order:

1. **L3-1, start the clock.** `kind` (`assertion` | `forecast`),
   `criterion` and, for forecasts, `resolves_by` on `CastHypothesis` and
   `HypothesisState`.
   - Admission validates new Casts only. Replay never validates.
   - Old records are neither backfilled nor re-hashed. A compatible-extension
     receipt is appended after the Cut 2 freeze receipt.
   - Management overlays ask for both kinds.
   - Nothing settles differently yet.
2. **L3-2, the settler's record.** It starts once the off-host copy is
   recoverable.
   - A settlement carries settler identity and evidence refs, and is
     admissible after its round closes. The opener still never settles.
   - An ordering witness comparable across inboxes (not `InboxSeq`) must
     show the commitment preceding the earliest capture a settlement cites.
     If not, the settlement is recorded as not admissible for scoring.
   - A deadline passing is not a contradiction: a forecast past its
     `resolves_by` with no settlement is counted as overdue.
   - Recaptured or syndicated copies cannot launder evidence that was
     already known.
3. **L3-3, scorers and the views.**
   - Score records per scorer, pinned to the model version, against the
     committed version (Jev and one other model first). Self-scores are
     non-governing.
   - Inline outcomes stop being averaged in as `protocol:<inbox>` scores,
     without breaking the desk calibration digest injector. Disagreement
     is kept.
   - Per-desk and per-kind views with denominators.
   - Per-scorer track records against an owner-audited sample.
   - The spine's per-source analytics.

The off-host ledger copy is L3's entry criterion and an L0 parallel item;
it is tracked there, not built here.
