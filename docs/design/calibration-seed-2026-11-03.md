# The calibration seed: what exists, what to test, in what order (2026-11-03)

Status: green (design; the experiment plan for station S6 of
[`choir-selfdev-apps-feature-complete-2026-11-03.md`](../definitions/choir-selfdev-apps-feature-complete-2026-11-03.md)).

Owner, 2026-11-03: "a calibration seed, where we show the model in addition
to its system prompt and REPL ... Maybe it would be in a prompt injected in
on model initiation, or maybe it would just be data accessible in the REPL.
We have to empirically test this ... History of predictions and scores ...
there's the model level, there's the desk level, there's the system level
... make sure that we're not running too many experiments at once."

## Owner rulings (2026-11-03 01:5x)

"on, but we need a better experiment. and i think its most important for
eng desk"; and "consider other ideas as well. this is an ongoing research
program." Chosen, all together:
- engineering predicts outcomes past Freeze (verify passes, the judge
  accepts, the owner approves, rounds to approval), each settled by the
  event;
- predictions are mandatory per round, stated before the capsule starts;
- the model arms are compared by calibration and resolution beside what
  they deliver;
- a within-desk A/B: half of rounds see their digest, split by a hash with
  a per-computer secret salt.
`calibration.feedback` stops being a switch (no switches, 2026-11-03): the
digest is on, and the A/B split is the experiment. The program keeps
adding ideas; each is one experiment per desk at a time.

## Engineering is the test bed, not the scope (owner, 2026-11-03)

"we may/should expand the calibration seed to other desks besides eng. the
eng test is to get evidence to figure out the right way (eg should it be
injected as an initial prompt? or just optionally accessible in the repl?
what information is useful? how does this vary across configurations?
etc." The engineering experiment answers four design questions, then the
seed widens to management, research, Texture and the app desks:
1. Delivery: injected at run start against data the desk may read in the
   REPL (and whether and when it reads it).
2. Content: which information changes behaviour (aggregate bins, per claim
   type, per model, recent misses with their why, discrepancies).
3. Configuration: how the answer varies by model, desk, and computer.
4. Cost: tokens and latency per run.
Each arm is one variable at a time; results go to this file before any
other desk gets the seed.

## The bridge to progressive delivery (owner, 2026-11-03)

"calibration in particular is the bridge between the 2 missions." A change
that rolls out, platform release or self-development candidate, carries its
author's precommitments about its effect; the ring walk (soak, health,
rollback) and the approval path settle them. Progressive delivery is the
settlement mechanism, self-development the source of predictions, and the
settled record is the engineering digest. The ring controller's history
must therefore record, per ring, what a settlement needs: the release, the
prediction ids it carries, the metrics read, and the outcome.

## The bridge, specified (lane F, 2026-11-03)

Two kinds of change carry predictions, and they settle in different places.

**A self-development candidate** (made on one computer, published to the
Library). Engineering's rule precommits (`types/commitment_rules.go`) are
already staged at Cast and carried in the Library release record
(`release.Precommitment`: record id, kind, question, p(yes) in thousandths,
settled, resolve record id). Four rule kinds are added, each settled by an
event the computer already records, so the runtime writes the resolve
record and no scorer judges:
- `judge_accepts`: the judge's verdict on this operation's candidate;
- `owner_approves`: the approve or reject act on the operation (FP0's act
  on the test arms counts, and is marked as delegated so it can be sliced
  out);
- `approved_within_rounds` (N): the request reaches an approved candidate
  within N Casts;
- `adopted_elsewhere` (days): another computer installs the Library
  release within the window (settles late; unresolved at expiry is "no").
Management's Cast makes `verify_pass`, `judge_accepts` and
`owner_approves` mandatory (a Cast without them is refused with the
reason), so every round predicts. These records feed the engineering
digest on that computer; `calibrationDeskOfThread` gains engineering.

**As built (2026-11-04, cuts 1 and 2).** Management's Cast to engineering
must carry `"predictions": {"verify_pass": p, "judge_accepts": p,
"owner_approves": p, "freeze_minutes": {...}, "approval_minutes": {...}}`.
`approved_within_rounds` is optional. A Cast without the mandatory claims is
refused with a literal example. The runtime derives one rule precommit per
claim from the committed Cast record, with id `<cast>:predict:<kind>`. The
records name management's agent and model as their author and the assignment
as their subject, and they count on engineering's calibration record. The
definitions:
- `judge_accepts`: management's Verify is the independent judgment today.
  It is yes when the operation reaches `awaiting_approval`, and no when it
  fails. It shares one event with `verify_pass` until the judge leaves
  Verify. Management predicts its own verdict, so this claim can be
  self-fulfilling. Read it with that in mind.
- `owner_approves`: yes on an approve act, or on an apply or failed apply
  that follows an approval. No on a reject, an abandon, a failed judgment,
  or a round with no candidate. The settlement names `decision_actor` and
  `delegated`, which is true when the actor is not the computer's owner.
- `freeze_minutes` and `approval_minutes` (owner, 2026-11-04: "clock time is
  something that agents should be precommitting to and getting scored on"):
  a point estimate, a low-high interval, and p(inside). Minutes run from the
  Cast record's `committed_at`. They end at the fresh Freeze's commit time,
  or at the approve act's record time (the same timestamps the round receipt
  reads). An event that never happens settles "no", censored. The
  readout reports coverage beside the log ratio of estimate to actual and
  the interval's log width.
- `approved_within_rounds` (N): **within N Freezes**, counted from the
  Cast, on the trajectory the Cast serves. Each self-development operation
  that froze a bundle counts once: a reused engineer's re-Freeze opens a
  new operation revision and counts as one Freeze, and a later round's
  Freezes count too. It settles yes at the first approval when the count
  is at most N. It settles no once the count passes N without an approval,
  or when the request is abandoned. Counting Casts would mean nothing once a
  rejection reuses the engineer instead of casting again.
- The held-out set and the arms hash under an HMAC with the computer's
  secret salt, which is derived from the guest key. Engineering is the
  only push desk. Its digest-arm rounds read their digest and its
  control-arm rounds do not. Each round's records carry `calibration_arm`.
  `calibration.feedback` is deleted.
- Not built yet: `adopted_elsewhere`, the `Predicts:` trailers, release-walk
  weighting, the pull (REPL-only) arm, and a fresh "minutes to re-Freeze"
  stated at each reuse of the engineer.

**A platform release** (a commit range on main that walks the rings). The
author, a builder agent or the orchestrator, states its predictions in the
commit message as `Predicts:` trailers, one per claim:
`Predicts: ring_clean R3 p=0.9` (the walk passes ring R3 without a breach),
`Predicts: rollback_at R0 p=0.05`, `Predicts: metric <name> <op> <value> p=0.7`
for a named health metric the soak reads. CI collects the trailers of the
deploy's commit range into the release manifest; vmctl copies their ids
into the walk's `prediction_ids`; the ring-closed, breach and walk-complete
lines carry the readings that settle them. A settler (a script over the
history, phase 1) writes one resolve line per prediction beside the
history. These are platform authors' records, not a computer desk's: they
do not enter any computer's digest. They are scored per author kind and
model, and reported in the morning letter and the ring history, so the
orchestrator's own calibration about deploys becomes visible.

**What is deferred.** Calibration weighting how fast a release walks (a
well-calibrated author's release soaking shorter) waits until the settled
record exists and has n; it is named here so the history keeps what it
needs. The within-desk A/B and the delivery, content, configuration and
cost questions above run on the self-development records only.

## What the repo already has

- **The idea is on the roadmap.** `roadmap-2026-10-31.md` stage 3: the desk
  seed goes (`problems/desk-runs-seeded-from-prior-run-2026-10-31.md`, owner:
  "Each desk should start with system prompt and repl only"), and "in its
  place a desk may get a small calibration seed, built from its history of
  precommitment records and their scores."
- **The mechanism is built and off.** Commit 978522bc0 added self-scores,
  independent settlement records and an in-context calibration digest
  (`internal/agentcore/calibration_feedback.go`,
  `internal/types/calibration_digest.go`, `internal/calibration/`), behind
  the computer setting `calibration.feedback`, default false. The digest is
  aggregate only (bins of stated p against the share that held, Brier and
  ECE by claim type and by model), never a per-item verdict. It is appended
  as a user turn after history, so the cached prefix holds, and is also in
  `choir.Pack()`. 6c47ab868 added the settlement writer and the measurement
  script.
- **The experiment is designed and was never run.**
  `archive/reports/icrl-calibration-experiment-2026-10-14.md`: arms by a
  deterministic hash of the desk thread, concurrent control, primary
  outcome ECE difference with a bootstrap interval, harm stops (Brier worse
  by 0.03, token use up 10%), no early stop for success, "inconclusive is
  reported as inconclusive". Baseline then: management stated mean p 0.63
  against 0.79 held over 559 resolved hypotheses (underconfident), Brier
  0.116.
- **Owner rulings that bound it.** `precommitment-records.md` 4.7: a desk
  sees its own settled record when it predicts again, but independent
  scorers' judgments stay out of the acting context (or are sampled and
  delayed); automatic pack injection stays off until held-out evidence
  supports it (H2). 2026-10-24: engineering sees aggregates of its own
  calibration with a 10% held-out set. 5.3: calibrate per question type;
  report calibration with accuracy and discrimination, since a desk can get
  calibrated by becoming vague.
- **The gaps.** Engineering is absent from the digest
  (`calibrationDeskOfThread` maps only management, Texture and research),
  and a capsule has no pack (`tasks-precommit-icrl-2026-10-24.md` 1.5, 1.6).
  The held-out hash uses a fixed public prefix, so a desk that reads the
  source can compute it; the 10-24 design moves it to a per-computer salt.
  The data cutover moved the tape, so the 10-14 baseline must be recomputed
  on the new store before it is trusted.

## How it fits

The seed is the read side of precommitment records. PSA and the records
write the predictions; settlement scores them; the seed is what a desk sees
of that history when it predicts again. Without the seed, records are an
audit trail. With it, they are the learning loop the owner called "the core
scientific innovation of Choir."

The three levels are three slices of one record set, not three mechanisms.
The model level is records filtered by pinned model id; the desk level by
desk thread; the system level is the computer's whole record. The digest
already slices by claim type and model inside a desk. Two uses differ in
kind and should not be mixed:

- **Self-calibration**: a desk reads its own record to adjust its stated
  probabilities. This is the seed.
- **Routing by record**: management, or the pool, reads other agents' and
  models' records to choose who gets the work. That is an allocation
  policy, measured by outcomes of the work, not by the reader's
  calibration. It belongs to the pool arm later, as its own experiment.

Pull against push is the owner's open question. Pull (data in the REPL,
read on demand) changes nothing unless the desk chooses to read it, and
whether it reads is itself a measurable behavior. Push (injected at run
start) is the 10-14 digest arm. They are two treatments against one
control.

## The sequence

One experiment at a time on any one desk, and no experiment on a computer
whose model or prompt is changing for another reason.

1. **Remove the prior-run seed first.** It is a defect the owner ruled on,
   not an experiment, and it confounds any seed test: a run that inherits
   the last run's conversation already carries a crude, unscored history.
   Land it ring by ring with the usual proof, then hold a baseline window
   with no other change.
2. **Recompute the baseline** on the new store with the existing
   measurement script: per desk, claim type and model, n, Brier, ECE,
   resolution, and the self-favor gap. If management's records are too
   few since the cutover, keep collecting; do not lower the sample size.
3. **Run the experiment, engineering first (owner), push arm and
   within-desk A/B.** Management has the volume (hundreds of settled hypotheses), and
   the design, harm stops and analysis are already written. Two arms
   within each computer by the hash, so every computer is its own control
   and the model comparison across computers is not confounded. Move the
   hash to the per-computer salt before it starts.
4. **Then the pull arm** (REPL-only), as a second experiment on the same
   desk after the first reads out, or as a third arm only if the sample
   allows it. Record whether and when the desk reads it.
5. **Engineering's claims, widened first (owner: engineering most important).** It already stages rule-settled precommits
   (`tests_pass`, `freeze_within`, `paths_confined`, `verify_pass`,
   `completes`, `visual_captured`; `internal/agentcore/engineering_precommit_rules.go`),
   settled by the runtime at Freeze and Verify, but they are optional, the
   Cast carries no acceptance criteria, `RulePrecommitSayDo` has no caller,
   and nothing past Freeze (the judge, owner approval, apply) scores against
   them. Its claims need widening first: what an
   engineering run predicts (that verify passes, that the judge accepts,
   that the owner approves, rounds to approval) and how each settles. That
   is the PSA work in S6; the self-development arms supply the records.
   At a few rounds a day per arm, engineering takes weeks to reach the
   minimum n, so it starts collecting now and is tested later.
6. **Routing by record** in the pool arm, after the seed reads out.

## What would fool us

- Calibration improves because the desk turns vague: report resolution
  and accuracy beside ECE, per claim type.
- The scorer drifts or is gamed: keep the held-out set and scorer audits;
  self-scores never govern.
- Two changes in one window: the sequence above, and a note in each arm's
  ledger of every prompt or model change during a run.
- Too small a sample: inconclusive is the honest answer, not a lean.
