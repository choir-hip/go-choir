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
readiness: reviewed

review:
  reviewer: agentic-consensus convergent rounds 1 (f658017ce; gpt-6-luna send_back, gpt-6.1-sol and gemini-3.8 accept_with_repairs; deepseek and glm 402 out of funds), 2 (8650c8f74; gpt-6.1-sol and gpt-6-luna send_back, codex and gemini-3.8 accept_with_repairs), 3 (3d7630420; gpt-6.1-sol and gpt-6-luna send_back on the capture witness, gemini-3.8 accept_with_repairs, codex accept), 4 (7cb82e902, the capture witness only; gpt-6.1-sol and gpt-6-luna accept)
  frozen_ref: git blob bdf232c5aee0e275cd872373d73c4687950937fb (this file at 7cb82e902)
  verdict: accept
  evidence_ref: docs/evidence/l3-file-review-2026-10-15.md; .agentic-consensus/l3-drafted-review-2026-10-15/, l3-review-round2-2026-10-15/, -round3-, -round4-

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
    - The ordering witness, comparable across inboxes, for a commitment
      and each capture its settlement cites. Two stores are involved.
      - The commitment is an inbox record on the paper's computer event
        chain. Its witness is the chain's Sequence: the first
        EventProjectionBatchRecorded event whose ProjectionOp.CanonicalID
        is the record (internal/store/projection_tape.go interceptOG;
        internal/computerevent/event.go Sequence, PreviousHead).
      - Captures are not on that chain. They are shared content written
        to Store B through corpusd (internal/agentcore/tool_profiles.go
        CorpusdCaptureSink; internal/platform/captures.go). The paper's
        chain holds only the CaptureArrival record naming each CaptureID
        (internal/agentcore/capture_arrival.go), which is a later
        announcement, not the first capture.
      - So a capture counts as after the commitment only when both hold:
        - (a) the first CaptureArrival naming it projects after the
          commitment on the paper's chain;
        - (b) a capture-bound first-creation witness on the Store B path
          places the capture's first write after the commitment.
        The L3-1 precondition establishes which witness exists for (b).
        If none is trustworthy, L3-1 records one at the corpusd write.
      - Timestamps may exclude evidence, never admit it. A capture whose
        publisher published_at precedes the commitment is not resolving
        evidence, whatever its capture order.
      - Unprovable order fails closed: not admissible for scoring. That
        covers equal or missing sequences, a missing arrival or Store B
        witness, and evidence that reached the paper only through another
        computer.
      - The settlement record carries every witness it relied on
        (sequences, event heads, the Store B witness), so scoring survives
        a later truncation of the chain.
    - Score records per scorer, model-version pinned. Every eligible
      settlement gets the committing agent's self-score (non-governing) and
      at least two independent scorers, with coverage denominators (scored,
      pending, failed). Disagreement is kept, never averaged away.
    - Per-scorer track records against an adjudicated audit sample,
      including the self-scorer's.
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
      proves: per desk, forecasts show Brier and skill against a stated base rate, and assertions show corroboration rate and lead time; each kind has its own denominators (n, settled, overdue, withdrawn, interrupted, per-scorer coverage); interrupted work is excluded from forecast error; disagreements are listed, not averaged; assertions never enter forecast figures
      evidence_class: deployed proof
    - action: GET per-scorer and per-source views; owner audit of a sample of settlements
      proves: each eligible settlement has a self-score and at least two independent pinned scorer records (or is counted pending or failed); each scorer, the self-scorer included, has its own track record against the audit labels with denominators; per-source lead time, corroboration and corrections are shown
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
    -> the paper with kind, criterion and resolves_by only (clock started,
    nothing settled) ->
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
    - Precondition, read-only: for one existing paper Cast and one
      capture it could cite, derive (a) both chain sequences (the Cast,
      and the first CaptureArrival naming the capture), and (b) the
      capture's first-creation witness on the Store B path.
      - No API route exposes the chain today. The events are pinned
        artifacts indexed in the guest's computer_event_index. The read
        path is a guest route that lists projection events by canonical
        id, never platform SQL or the guest's block device.
      - Whatever is not derivable, L3-1 records at write time (the Cast
        and the corpusd write), before the forecast cohort starts.
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
      The sample (start.corrections) found 32 of 38 assertions and 6
      forward-looking, only 1 of them dated; 18 of 38 have p >= 0.85.
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
      the smoke check, and a coordinated rollback. The rollback reverts
      the admission enforcement and the overlay together, to the last-good
      contract. It keeps the schema readers, the fold and replay support,
      and every committed record. After rollback, the smoke check must
      see a story Cast admitted and a story published, not merely a cell
      error.
    Red ceremony:
    - conjecture delta: a hypothesis states how it settles (kind,
      criterion, date);
    - heresy delta: discovered that inline settlements are averaged in as
      scorer protocol:<inbox>; introduced none so far; repaired: none in
      L3-1, planned in L3-3 (inline outcomes leave the scorer averages, and
      the settlement records stay).
  next_action: >-
    Witness (b) is decided by the design panel (3 of 4,
    docs/evidence/l3-file-review-2026-10-15.md, section "Witness (b)
    design"). It is a per-computer causal anchor recorded at the corpusd
    write, which is the fallback this reviewed contract already allows.
    - Before attempting a capture's first insert, corpusd reads the
      writing computer's committed canonical event head (platform
      computer_event_heads). It stores a host-signed
      choir.capture_first_creation.v1 witness with the winning INSERT
      only: capture id, doc hash, writer computer, anchor sequence, head
      and receipt digest, key id and signature.
    - The witness rides CapturedItem, the pending list and
      CaptureArrival.
    - A capture counts only when all of these hold:
      - the Cast is a strict ancestor of the anchor on the paper's chain;
      - the writer is the paper;
      - the first arrival follows the Cast (witness a).
    - A failed head read or signature still stores the capture, without
      a witness: unscorable, and never filled in later.
    - The witness is immutable against PutObject upserts and
      DeleteObject. A tombstone stops a deleted capture from earning a
      newer first witness.
    - The off-host copy carries the signatures, the verification keys and
      the chain artifacts.
    Witness (a) needs no new table: a read-only host chain walk on
    projectionbase.DiskEventSource, since Cast and CaptureArrival records
    are written once. Never add a guest index: the content witness covers
    the whole texture DB, so live and replay would diverge.
    Next steps:
    - a problem-first design doc for the corpusd witness (red: Store B
      writes, host signing);
    - then L3-1, tests first, in a worktree. L3-1 and the corpusd witness
      ship before the forecast cohort starts.
    Dissent kept: gpt-6-luna preferred a post-commit Store B watermark
    anchor (D1'), which needs a serialized Store B sequence.
    L3-1 rollout (2026-10-15, orchestrator decision, the conservative
    option under No Blocking Asks): this first deploy validates kind,
    criterion and resolves_by when present but does not yet require kind.
    - A kind-less hypothesis commits as legacy and stays outside the
      forecast cohort.
    - Requiring kind on every new hypothesis, as now.slice says, waits
      until the paper's Casts show the kinds reliably (about a day of
      Casts, with a kinded share of 95% or more). This keeps a prompt the
      model misreads from refusing story opens on the 24/7 paper.
    - Rollback residual: a revert followed by a roll-forward leaves
      conversations touched in between with stored heads that lack the
      fields. VerifyInboxHeads would report those; re-folding them is the
      repair.
    L3-1 red ceremony:
    - Mutation class: red.
    - Protected surfaces: Cast admission and the inbox fold
      (internal/store/inbox.go), CastHypothesis and HypothesisState, the
      management overlays on the live paper, and the Cut 2 freeze receipt.
    - Admissible evidence: focused store tests K1-K9 written first; then,
      deployed, the paper's story Casts are still admitted and published,
      and the kinds appear in new Casts.
    - Rollback: git revert. Heads decode with unknown fields ignored (K9).
    - Conjecture delta: a hypothesis can state how it settles.
    - Heresy delta: discovered none, introduced none, repaired none.
    - Code review (gpt-6.1-sol and codex, both "ship"):
      .agentic-consensus/l3-1-code-review-2026-10-15/.
    - Residual named by that review: the checks are on the inbox
      planner, which every live path uses (management, tasks, Texture
      turns, engineering assignments). Two legacy writers,
      AppendCommitmentRecord (internal/store/commitment.go) and
      lifecycle_commit_act.go, can still store a typed Cast unchecked. No
      live producer uses them for hypotheses. Guard or exclude them before
      L3-3 scoring relies on "every new hypothesis was checked".
    L3-1 deployed: 0fe4cead0, deployed to staging 22:29:33; the paper
    booted onto it at 22:28. Smoke at 22:55 (read-only, counts only):
    - liveness ok, 0 overdue schedules;
    - 22 management runs, all completed, none showing an L3-1 refusal
      text in its events;
    - 2 story opens, both with kind "assertion": 100% kinded, no
      forecasts yet.
    Open hypothesis, not yet a finding: story opens fell from 24 per 89
    min to 2 per 25 min. Feeds held at about one disposition per minute
    and captures at about one per minute. The trace shows no refusal, so
    the likelier cause is the news cycle (more existing stories to feed).
    Re-measure over a full hour against the hour before.
    Re-measured 23:38 from the management inbox's task final reports
    (task_outcome.dispositions), which count every capture's disposition,
    over two equal 69-minute windows:
    - before (21:20-22:29): 65 captures arrived, 11 opens, 29 feeds;
    - after (22:29-23:38): 58 captures arrived, 7 opens, 35 feeds.
    Throughput held (40 against 42 dispositions), and 11 against 7 opens
    is within Poisson noise. The dip is not supported; there is no
    evidence L3-1 harms intake. All 8 new opens carry kind "assertion";
    there are no forecasts yet.
    (The earlier texture-inbox count undercounts older windows: it only
    sees inboxes in the newest runs.)
    Next observation: the kinded share and the forecast share over a
    day, which decides when kind becomes required.
    Witness (a) precondition: met, 2026-10-15 ~23:55 UTC.
    - The tool is cmd/ledger-witness (internal/ledgerwitness, tests W1-W6
      written first). It walks back from the head along previous_head over
      the host's event and payload artifacts, matching projected
      choir.commitment_record objects by record_id. It is read-only: no
      SQL, no guest, capped, and anything unproven is reported missing.
    - Run on Node B at nice 19, idle IO. Head: from GET
      /api/computers/{id}/self-development/head, sequence 704145. It
      walked 14,255 events in 2.0 s.
      - capture-arrival:rss:thehill_news:6aadebd4… is at sequence 689891
        (event 605e10fa…).
      - Story-opening Cast cell:53fac871…:cast:tray-3 (kind assertion) is
        at sequence 689953 (event e318e279…).
    - So the arrival precedes the Cast on one chain. That is correct for
      an opening: the capture that opened a story cannot settle that
      story's hypothesis, and the witness shows it.

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
   - The ordering witness (see finish.artifact) has two parts: the
     paper's event-chain `Sequence` for the commitment and the
     `CaptureArrival`, plus a first-creation witness on the Store B path
     for the capture itself. It is never `InboxSeq`, and never a
     timestamp. It must show the commitment preceding the earliest
     capture a settlement cites. If not, or if the order is unprovable, the
     settlement is recorded as not admissible for scoring. The settlement
     record keeps both sequences and event heads.
   - A deadline passing is not a contradiction: a forecast past its
     `resolves_by` with no settlement is counted as overdue.
   - Recaptured or syndicated copies cannot launder evidence that was
     already known.
3. **L3-3, scorers and the views.**
   - Score records per scorer, pinned to the model version, against the
     committed version. Every eligible settlement gets the self-score
     (non-governing) and at least two independent scorers (Jev and one
     other model first), with coverage denominators.
   - Inline outcomes stop being averaged in as `protocol:<inbox>` scores,
     without breaking the desk calibration digest injector. Disagreement
     is kept.
   - Per-desk and per-kind views with denominators.
   - Per-scorer track records against an owner-audited sample.
   - The spine's per-source analytics.

The off-host ledger copy is L3's entry criterion and an L0 parallel item;
it is tracked there, not built here.
